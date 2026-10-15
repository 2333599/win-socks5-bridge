//go:build linux

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

const linuxTUNTable = "555"
const linuxTUNMark = 0x22b
const linuxTUNRulePriority = "20220"

type linuxTUNRoutePlan struct {
	ipv4       bool
	ipv6       bool
	ready      bool
	configured bool
}

func newTUNRoutePlan(net.IP) (tunRoutePlan, error) { return &linuxTUNRoutePlan{}, nil }

func (p *linuxTUNRoutePlan) ProxyInterface() string { return "" }
func (p *linuxTUNRoutePlan) RoutingMark() int       { return linuxTUNMark }
func (p *linuxTUNRoutePlan) DeviceName() string     { return tunInterfaceName }

func (p *linuxTUNRoutePlan) Prepare() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN 全流量模式需要 root 权限")
	}
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("TUN 设备 /dev/net/tun 不可用: %w", err)
	}
	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("找不到 ip 命令（iproute2）")
	}
	for _, family := range [][]string{{"-4"}, {"-6"}} {
		if output, err := exec.Command("ip", append(append([]string{}, family...), "route", "show", "table", linuxTUNTable)...).Output(); err == nil && len(bytes.TrimSpace(output)) != 0 {
			return fmt.Errorf("路由表 %s 已被占用，TUN 模式不会覆盖现有路由", linuxTUNTable)
		}
	}
	if output, err := exec.Command("ip", "rule", "show").Output(); err == nil && bytes.Contains(output, []byte(linuxTUNRulePriority+":")) {
		return fmt.Errorf("策略路由优先级 %s 已被占用", linuxTUNRulePriority)
	}
	var err error
	p.ipv4, err = p.cloneRoutes("-4")
	if err != nil {
		_ = runIP("-4", "route", "flush", "table", linuxTUNTable)
		return err
	}
	p.ipv6, err = p.cloneRoutes("-6")
	if err != nil {
		_ = runIP("-6", "route", "flush", "table", linuxTUNTable)
		_ = p.Stop()
		return err
	}
	if !p.ipv4 {
		_ = p.Stop()
		return fmt.Errorf("没有 IPv4 路由，无法启用 TUN")
	}
	p.ready = true
	return nil
}

func (p *linuxTUNRoutePlan) cloneRoutes(family string) (bool, error) {
	out, err := exec.Command("ip", family, "route", "show", "table", "main").Output()
	if err != nil {
		return false, fmt.Errorf("读取 %s 主路由表: %w", family, err)
	}
	lines := bytes.Split(out, []byte{'\n'})
	count := 0
	found := false
	for _, raw := range lines {
		fields := strings.Fields(string(raw))
		if len(fields) == 0 {
			continue
		}
		found = true
		if fields[0] == "default" {
			continue // TUN default is installed after the device exists.
		}
		filtered := fields[:0]
		for _, f := range fields {
			if f != "linkdown" {
				filtered = append(filtered, f)
			}
		}
		args := append([]string{family, "route", "replace"}, filtered...)
		args = append(args, "table", linuxTUNTable)
		if output, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			return false, fmt.Errorf("复制本地路由到 TUN 表失败 (%s): %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
		}
		count++
	}
	return found || count > 0 || family == "-4", nil
}

func (p *linuxTUNRoutePlan) Activate() error {
	if !p.ready {
		return fmt.Errorf("TUN 路由未准备好")
	}
	steps := [][]string{
		{"addr", "add", "198.18.0.1/15", "dev", tunInterfaceName},
		{"link", "set", "dev", tunInterfaceName, "up"},
		{"route", "replace", "default", "dev", tunInterfaceName, "metric", "1", "table", linuxTUNTable},
	}
	for i, args := range steps {
		if err := runIP(args...); err != nil {
			_ = p.Stop()
			return err
		}
		if i == 0 {
			p.configured = true
		}
	}
	if p.ipv6 {
		for _, args := range [][]string{
			{"-6", "addr", "add", "fd00:198:18::1/64", "dev", tunInterfaceName},
			{"-6", "route", "replace", "default", "dev", tunInterfaceName, "metric", "1", "table", linuxTUNTable},
		} {
			if err := runIP(args...); err != nil {
				_ = p.Stop()
				return err
			}
		}
	}
	if err := runIP("rule", "add", "priority", linuxTUNRulePriority, "not", "fwmark", fmt.Sprintf("0x%x", linuxTUNMark), "table", linuxTUNTable); err != nil {
		_ = p.Stop()
		return err
	}
	if p.ipv6 {
		if err := runIP("-6", "rule", "add", "priority", linuxTUNRulePriority, "not", "fwmark", fmt.Sprintf("0x%x", linuxTUNMark), "table", linuxTUNTable); err != nil {
			_ = p.Stop()
			return err
		}
	}
	p.ready = false
	return nil
}

func (p *linuxTUNRoutePlan) Stop() error {
	var first error
	try := func(args ...string) {
		if err := runIP(args...); err != nil && first == nil {
			first = err
		}
	}
	try("rule", "del", "priority", linuxTUNRulePriority)
	if p.ipv6 {
		try("-6", "rule", "del", "priority", linuxTUNRulePriority)
		try("-6", "route", "flush", "table", linuxTUNTable)
	}
	try("-4", "route", "flush", "table", linuxTUNTable)
	if p.configured {
		if p.ipv6 {
			try("-6", "addr", "del", "fd00:198:18::1/64", "dev", tunInterfaceName)
		}
		try("-4", "addr", "del", "198.18.0.1/15", "dev", tunInterfaceName)
		try("link", "set", "dev", tunInterfaceName, "down")
	}
	p.ready = false
	p.configured = false
	return first
}

func runIP(args ...string) error {
	output, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}
