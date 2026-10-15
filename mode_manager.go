package main

import (
	"errors"
	"sync"
)

const (
	modeApplication = "单应用 SOCKS5"
	modeSystem      = "系统代理"
	modeTUN         = "TUN 全流量代理"
)

type modeManager struct {
	mu      sync.Mutex
	mode    string
	socks   *proxyManager
	http    *httpProxyManager
	restore func() error
	tunStop func() error
	address string
}

func (m *modeManager) Start(mode, address, upstreamURL string) error {
	m.mu.Lock()
	if m.mode != "" {
		m.mu.Unlock()
		return errors.New("代理已运行，请先停止当前模式")
	}
	m.mu.Unlock()

	switch mode {
	case modeApplication:
		if m.socks == nil {
			m.socks = newProxyManager()
		}
		if err := m.socks.Start(address, upstreamURL); err != nil {
			return err
		}
		m.set(mode, m.socks.Address())
	case modeSystem:
		if address == "" {
			address = "127.0.0.1:8080"
		}
		if m.http == nil {
			m.http = &httpProxyManager{}
		}
		if err := m.http.Start(address, upstreamURL); err != nil {
			return err
		}
		restore, err := setSystemProxy(m.http.Address())
		if err != nil {
			m.http.Stop()
			return err
		}
		m.restore = restore
		m.set(mode, m.http.Address())
	case modeTUN:
		stop, err := startTUNProxy(upstreamURL)
		if err != nil {
			return err
		}
		m.tunStop = stop
		m.set(mode, "TUN 默认路由")
	default:
		return errors.New("未知代理模式")
	}
	return nil
}

func (m *modeManager) set(mode, address string) {
	m.mu.Lock()
	m.mode, m.address = mode, address
	m.mu.Unlock()
}

func (m *modeManager) Stop() error {
	m.mu.Lock()
	mode, restore, tunStop := m.mode, m.restore, m.tunStop
	m.mode, m.address, m.restore, m.tunStop = "", "", nil, nil
	m.mu.Unlock()
	var err error
	switch mode {
	case modeApplication:
		if m.socks != nil {
			m.socks.Stop()
		}
	case modeSystem:
		if m.http != nil {
			m.http.Stop()
		}
		if restore != nil {
			err = restore()
		}
	case modeTUN:
		if tunStop != nil {
			err = tunStop()
		}
	}
	return err
}

func (m *modeManager) Status() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode, m.address
}
