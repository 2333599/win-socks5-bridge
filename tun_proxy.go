package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	tunengine "github.com/xjasonlyu/tun2socks/v2/engine"
)

type tunRoutePlan interface {
	Prepare() error
	Activate() error
	Stop() error
	DeviceName() string
	ProxyInterface() string
	RoutingMark() int
}

const tunInterfaceName = "s5tun0"

func startTUNProxy(upstreamURL string) (func() error, error) {
	up, err := parseUpstream(upstreamURL)
	if err != nil {
		return nil, err
	}
	proxyURL, endpoint, err := resolveSOCKSProxy(upstreamURL, up)
	if err != nil {
		return nil, err
	}
	plan, err := newTUNRoutePlan(endpoint)
	if err != nil {
		return nil, err
	}
	if err := plan.Prepare(); err != nil {
		return nil, err
	}
	key := &tunengine.Key{
		Proxy:     proxyURL,
		Device:    "tun://" + plan.DeviceName(),
		MTU:       1500,
		LogLevel:  "warn",
		Mark:      plan.RoutingMark(),
		Interface: plan.ProxyInterface(),
	}
	tunengine.Insert(key)
	tunengine.Start()
	if err := plan.Activate(); err != nil {
		_ = plan.Stop()
		tunengine.Stop()
		return nil, fmt.Errorf("configure TUN routes: %w", err)
	}
	return func() error {
		err := plan.Stop()
		tunengine.Stop()
		return err
	}, nil
}

func resolveSOCKSProxy(raw string, up upstream) (string, net.IP, error) {
	host, portText, err := net.SplitHostPort(up.address)
	if err != nil {
		return "", nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", nil, fmt.Errorf("resolve upstream host: %w", err)
		}
		if len(ips) == 0 {
			return "", nil, fmt.Errorf("upstream hostname has no IP address")
		}
		ip = ips[0]
		for _, candidate := range ips {
			if candidate.To4() != nil {
				ip = candidate
				break
			}
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, err
	}
	// Resolve the SOCKS server before default routes are changed and pin the
	// endpoint so the tunnel never recursively tries to proxy its own transport.
	u.Scheme = "socks5"
	u.Host = net.JoinHostPort(ip.String(), strconv.Itoa(port))
	return u.String(), ip, nil
}
