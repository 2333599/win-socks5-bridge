package main

import (
	"errors"
	"net"
	"sync"
)

// proxyManager owns the local listener and active sessions so the UI can stop
// the proxy without terminating the whole application.
type proxyManager struct {
	mu       sync.Mutex
	listener net.Listener
	clients  map[net.Conn]struct{}
}

func newProxyManager() *proxyManager {
	return &proxyManager{clients: make(map[net.Conn]struct{})}
}

func (p *proxyManager) Start(listenAddress, upstreamURL string) error {
	up, err := parseUpstream(upstreamURL)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.listener != nil {
		p.mu.Unlock()
		_ = ln.Close()
		return errors.New("proxy is already running")
	}
	p.listener = ln
	p.mu.Unlock()
	go p.acceptLoop(ln, up)
	return nil
}

func (p *proxyManager) acceptLoop(ln net.Listener, up upstream) {
	for {
		client, err := ln.Accept()
		if err != nil {
			p.mu.Lock()
			if p.listener == ln {
				p.listener = nil
			}
			p.mu.Unlock()
			return
		}

		p.mu.Lock()
		if p.listener != ln {
			p.mu.Unlock()
			_ = client.Close()
			return
		}
		p.clients[client] = struct{}{}
		p.mu.Unlock()

		go func() {
			serve(client, up)
			p.mu.Lock()
			delete(p.clients, client)
			p.mu.Unlock()
		}()
	}
}

func (p *proxyManager) Stop() {
	p.mu.Lock()
	ln := p.listener
	p.listener = nil
	clients := make([]net.Conn, 0, len(p.clients))
	for c := range p.clients {
		clients = append(clients, c)
	}
	p.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range clients {
		_ = c.Close()
	}
}

func (p *proxyManager) Address() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listener == nil {
		return ""
	}
	return p.listener.Addr().String()
}
