package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func requireLoopback(listenAddress string) error {
	host, _, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return fmt.Errorf("invalid local proxy address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("system proxy must listen on a loopback address (127.0.0.1 or ::1)")
	}
	return nil
}

// httpProxyManager exposes an HTTP/CONNECT proxy that dials destinations via
// the configured upstream SOCKS5 server. It is used by OS system-proxy mode.
type httpProxyManager struct {
	mu     sync.Mutex
	server *http.Server
	ln     net.Listener
	addr   string
}

func (p *httpProxyManager) Start(listenAddress, upstreamURL string) error {
	up, err := parseUpstream(upstreamURL)
	if err != nil {
		return err
	}
	if err := requireLoopback(listenAddress); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.server != nil {
		p.mu.Unlock()
		_ = ln.Close()
		return fmt.Errorf("HTTP proxy is already running")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveHTTPProxy(w, r, up)
	})
	p.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	p.ln, p.addr = ln, ln.Addr().String()
	server := p.server
	p.mu.Unlock()
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP proxy stopped: %v", err)
		}
	}()
	return nil
}

func (p *httpProxyManager) Stop() {
	p.mu.Lock()
	server := p.server
	p.server, p.ln, p.addr = nil, nil, ""
	p.mu.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

func (p *httpProxyManager) Address() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

func serveHTTPProxy(w http.ResponseWriter, r *http.Request, up upstream) {
	if r.Method == http.MethodConnect {
		serveConnectProxy(w, r, up)
		return
	}
	host := r.URL.Host
	if host == "" {
		host = r.Host
	}
	if host == "" {
		http.Error(w, "proxy request has no destination", http.StatusBadRequest)
		return
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		if strings.Contains(host, ":") {
			http.Error(w, "invalid destination", http.StatusBadRequest)
			return
		}
		host = net.JoinHostPort(host, "80")
	}
	remote, err := dialUpstream(up, host)
	if err != nil {
		http.Error(w, "upstream SOCKS5 connection failed", http.StatusBadGateway)
		return
	}
	defer remote.Close()
	r.Header.Del("Proxy-Authorization")
	r.Header.Del("Proxy-Connection")
	r.Header.Set("Connection", "close")
	r.Close = true
	if err := r.Write(remote); err != nil {
		http.Error(w, "failed to forward request", http.StatusBadGateway)
		return
	}
	response, err := http.ReadResponse(bufio.NewReader(remote), r)
	if err != nil {
		http.Error(w, "invalid response from upstream", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for key, values := range response.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if _, err := io.Copy(w, response.Body); err != nil {
		log.Printf("write proxy response: %v", err)
	}
}

func isHopByHopHeader(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func serveConnectProxy(w http.ResponseWriter, r *http.Request, up upstream) {
	destination := r.Host
	if _, _, err := net.SplitHostPort(destination); err != nil {
		http.Error(w, "CONNECT requires host:port", http.StatusBadRequest)
		return
	}
	remote, err := dialUpstream(up, destination)
	if err != nil {
		http.Error(w, "upstream SOCKS5 connection failed", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = remote.Close()
		http.Error(w, "HTTP hijacking is unavailable", http.StatusInternalServerError)
		return
	}
	client, rw, err := hijacker.Hijack()
	if err != nil {
		_ = remote.Close()
		return
	}
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = client.Close()
		_ = remote.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		_ = client.Close()
		_ = remote.Close()
		return
	}
	go func() {
		_, _ = io.Copy(remote, rw)
		closeWrite(remote)
	}()
	_, _ = io.Copy(client, remote)
	_ = client.Close()
	_ = remote.Close()
}

func closeWrite(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}
