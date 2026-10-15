// win-socks5-bridge exposes a local SOCKS5 proxy and relays TCP connections
// through an upstream SOCKS5 server.
package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	version5       = 5
	methodNone     = 0
	methodPassword = 2
	methodReject   = 255
	cmdConnect     = 1
	addrIPv4       = 1
	addrDomain     = 3
	addrIPv6       = 4
)

type upstream struct {
	address  string
	username string
	password string
}

func main() {
	// Launch the built-in browser UI when double-clicked or run without flags.
	if len(os.Args) == 1 {
		if err := runGUI(); err != nil {
			log.Fatal(err)
		}
		return
	}
	listen := flag.String("listen", "127.0.0.1:1080", "local SOCKS5 listen address (default: 127.0.0.1:1080)")
	server := flag.String("upstream", "", "upstream SOCKS5 URL, e.g. socks5://user:pass@host:1080 (required)")
	clearProxy := flag.Bool("clear-proxy", false, "disable the current user's Windows manual system proxy and exit")
	gui := flag.Bool("gui", false, "open the built-in browser interface")
	flag.Parse()
	if *gui {
		if err := runGUI(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *clearProxy {
		if err := ClearProxy(); err != nil {
			log.Fatalf("clear Windows proxy: %v", err)
		}
		log.Println("Windows manual proxy has been disabled and settings refreshed.")
		return
	}

	u, err := parseUpstream(*server)
	if err != nil {
		log.Fatalf("invalid -upstream: %v", err)
	}
	manager := newProxyManager()
	if err := manager.Start(*listen, *server); err != nil {
		log.Fatal(err)
	}
	log.Printf("Local SOCKS5 proxy listening on %s; upstream %s", manager.Address(), u.address)
	select {}
}

func parseUpstream(raw string) (upstream, error) {
	if strings.TrimSpace(raw) == "" {
		return upstream{}, errors.New("provide an upstream SOCKS5 URL")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "socks5" && u.Scheme != "socks5h") || u.Hostname() == "" {
		return upstream{}, errors.New("expected socks5://[user:pass@]host:port")
	}
	port := u.Port()
	if port == "" {
		port = "1080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return upstream{}, errors.New("invalid upstream port")
	}
	user, pass := "", ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
		if len(user) == 0 || len(user) > 255 || len(pass) > 255 {
			return upstream{}, errors.New("upstream username/password must be 1..255 / 0..255 bytes")
		}
	}
	return upstream{address: net.JoinHostPort(u.Hostname(), port), username: user, password: pass}, nil
}

func serve(client net.Conn, up upstream) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))
	h := make([]byte, 2)
	if _, err := io.ReadFull(client, h); err != nil || h[0] != version5 || h[1] == 0 {
		return
	}
	methods := make([]byte, int(h[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	if !hasMethod(methods, methodNone) {
		_, _ = client.Write([]byte{version5, methodReject})
		return
	}
	if _, err := client.Write([]byte{version5, methodNone}); err != nil {
		return
	}

	req := make([]byte, 4)
	if _, err := io.ReadFull(client, req); err != nil || req[0] != version5 {
		return
	}
	if req[1] != cmdConnect {
		_ = writeReply(client, 7)
		return
	}
	destination, err := readDestination(client, req[3])
	if err != nil {
		_ = writeReply(client, 8)
		return
	}

	remote, err := dialUpstream(up, destination)
	if err != nil {
		log.Printf("upstream connect to %s: %v", destination, err)
		_ = writeReply(client, 5)
		return
	}
	defer remote.Close()
	_ = client.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})
	if err := writeReply(client, 0); err != nil {
		return
	}
	pipe(client, remote)
}

func hasMethod(methods []byte, want byte) bool {
	for _, m := range methods {
		if m == want {
			return true
		}
	}
	return false
}

func readDestination(c net.Conn, atyp byte) (string, error) {
	var host string
	switch atyp {
	case addrIPv4:
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return "", err
		}
		host = net.IP(b).String()
	case addrIPv6:
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return "", err
		}
		host = net.IP(b).String()
	case addrDomain:
		l := []byte{0}
		if _, err := io.ReadFull(c, l); err != nil {
			return "", err
		}
		b := make([]byte, int(l[0]))
		if len(b) == 0 {
			return "", errors.New("empty domain")
		}
		if _, err := io.ReadFull(c, b); err != nil {
			return "", err
		}
		host = string(b)
	default:
		return "", errors.New("unsupported address type")
	}
	p := make([]byte, 2)
	if _, err := io.ReadFull(c, p); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(p)))), nil
}

func dialUpstream(up upstream, destination string) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", up.address, 15*time.Second)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (net.Conn, error) { c.Close(); return nil, err }
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	if up.username == "" {
		if _, err = c.Write([]byte{version5, 1, methodNone}); err != nil {
			return fail(err)
		}
	} else {
		if _, err = c.Write([]byte{version5, 2, methodNone, methodPassword}); err != nil {
			return fail(err)
		}
	}
	selection := make([]byte, 2)
	if _, err = io.ReadFull(c, selection); err != nil || selection[0] != version5 {
		return fail(errors.New("invalid upstream method response"))
	}
	if up.username == "" {
		if selection[1] != methodNone {
			return fail(errors.New("upstream requires authentication"))
		}
	} else {
		if selection[1] != methodPassword {
			return fail(errors.New("upstream does not accept username/password authentication"))
		}
		msg := []byte{1, byte(len(up.username))}
		msg = append(msg, up.username...)
		msg = append(msg, byte(len(up.password)))
		msg = append(msg, up.password...)
		if _, err = c.Write(msg); err != nil {
			return fail(err)
		}
		resp := make([]byte, 2)
		if _, err = io.ReadFull(c, resp); err != nil || resp[0] != 1 || resp[1] != 0 {
			return fail(errors.New("upstream authentication failed"))
		}
	}

	host, portText, err := net.SplitHostPort(destination)
	if err != nil {
		return fail(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fail(errors.New("invalid destination port"))
	}
	var req []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append([]byte{version5, cmdConnect, 0, addrIPv4}, ip4...)
		} else {
			req = append([]byte{version5, cmdConnect, 0, addrIPv6}, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return fail(errors.New("destination domain must be 1..255 bytes"))
		}
		req = append([]byte{version5, cmdConnect, 0, addrDomain, byte(len(host))}, host...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err = c.Write(req); err != nil {
		return fail(err)
	}
	resp := make([]byte, 4)
	if _, err = io.ReadFull(c, resp); err != nil || resp[0] != version5 || resp[1] != 0 {
		return fail(fmt.Errorf("upstream SOCKS5 connection rejected (reply %d)", resp[1]))
	}
	if err = discardBoundAddress(c, resp[3]); err != nil {
		return fail(err)
	}
	_ = c.SetDeadline(time.Time{})
	return c, nil
}

func discardBoundAddress(c net.Conn, atyp byte) error {
	var n int
	switch atyp {
	case addrIPv4:
		n = 4
	case addrIPv6:
		n = 16
	case addrDomain:
		b := []byte{0}
		if _, err := io.ReadFull(c, b); err != nil {
			return err
		}
		n = int(b[0])
	default:
		return errors.New("invalid upstream bound address")
	}
	_, err := io.CopyN(io.Discard, c, int64(n+2))
	return err
}

func writeReply(c net.Conn, code byte) error {
	return writeReplyAddr(c, code, []byte{version5, code, 0, addrIPv4, 0, 0, 0, 0, 0, 0})
}
func writeReplyAddr(c net.Conn, code byte, b []byte) error {
	b[1] = code
	_, err := c.Write(b)
	return err
}

func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(b, a)
		if tcp, ok := b.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(a, b)
		if tcp, ok := a.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
}
