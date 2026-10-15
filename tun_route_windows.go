//go:build windows

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

type windowsTUNRoutePlan struct {
	endpoint  net.IP
	physIf    net.Interface
	gateway   net.IP
	tunIf     net.Interface
	ipv6      bool
	addedHost bool
	addedV4   bool
	addedV6   bool
	activated bool
}

func newTUNRoutePlan(endpoint net.IP) (tunRoutePlan, error) {
	return &windowsTUNRoutePlan{endpoint: endpoint}, nil
}

func (p *windowsTUNRoutePlan) RoutingMark() int   { return 0 }
func (p *windowsTUNRoutePlan) DeviceName() string { return tunInterfaceName }
func (p *windowsTUNRoutePlan) ProxyInterface() string {
	return p.physIf.Name
}

func (p *windowsTUNRoutePlan) Prepare() error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("检查管理员权限失败: %w", err)
	}
	defer token.Close()
	if !token.IsElevated() {
		return fmt.Errorf("TUN 全流量模式需要以管理员身份运行程序")
	}
	if p.endpoint.To4() == nil {
		return fmt.Errorf("Windows TUN 模式当前需要 IPv4 SOCKS5 服务器地址")
	}
	output, err := exec.Command("route.exe", "PRINT", "-4").Output()
	if err != nil {
		return fmt.Errorf("读取 Windows IPv4 路由表: %w", err)
	}
	foundDefault := false
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" {
			continue
		}
		gateway, localIP := net.ParseIP(fields[2]), net.ParseIP(fields[3])
		if gateway == nil || localIP == nil || gateway.To4() == nil || localIP.To4() == nil {
			continue
		}
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		for _, iface := range interfaces {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				ip, _, _ := net.ParseCIDR(addr.String())
				if ip != nil && ip.Equal(localIP) {
					p.gateway, p.physIf = gateway, iface
					foundDefault = true
					break
				}
			}
			if foundDefault {
				break
			}
		}
		if foundDefault {
			break
		}
	}
	if routes, err := exec.Command("route.exe", "PRINT", "-6").Output(); err == nil {
		for _, line := range strings.Split(string(routes), "\n") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if field == "::/0" {
					p.ipv6 = true
					break
				}
			}
		}
	}
	if !foundDefault {
		return fmt.Errorf("找不到当前 IPv4 默认网关；TUN 模式未更改路由")
	}
	return nil
}

func (p *windowsTUNRoutePlan) Activate() error {
	if output, err := exec.Command("netsh.exe", "interface", "ipv4", "set", "address", "name="+tunInterfaceName, "source=static", "address=198.18.0.1", "mask=255.254.0.0", "gateway=none").CombinedOutput(); err != nil {
		return fmt.Errorf("配置 TUN IPv4 地址（需管理员权限）: %s", strings.TrimSpace(string(output)))
	}
	iface, err := net.InterfaceByName(tunInterfaceName)
	if err != nil {
		return fmt.Errorf("找不到 Wintun 接口 %s: %w", tunInterfaceName, err)
	}
	p.tunIf = *iface

	host := p.endpoint.To4().String()
	if output, err := exec.Command("route.exe", "ADD", host, "MASK", "255.255.255.255", p.gateway.String(), "METRIC", "1", "IF", fmt.Sprint(p.physIf.Index)).CombinedOutput(); err != nil {
		return fmt.Errorf("添加 SOCKS 服务器直连路由: %s", strings.TrimSpace(string(output)))
	}
	p.addedHost = true
	if output, err := exec.Command("route.exe", "ADD", "0.0.0.0", "MASK", "0.0.0.0", "198.18.0.1", "METRIC", "1", "IF", fmt.Sprint(p.tunIf.Index)).CombinedOutput(); err != nil {
		return fmt.Errorf("添加 TUN 默认路由（需管理员权限）: %s", strings.TrimSpace(string(output)))
	}
	p.addedV4 = true
	if p.ipv6 {
		if output, err := exec.Command("netsh.exe", "interface", "ipv6", "add", "address", "interface="+tunInterfaceName, "address=fd00:198:18::1/64", "store=active").CombinedOutput(); err != nil {
			return fmt.Errorf("配置 TUN IPv6 地址（需管理员权限）: %s", strings.TrimSpace(string(output)))
		}
		if output, err := exec.Command("netsh.exe", "interface", "ipv6", "add", "route", "prefix=::/0", "interface="+tunInterfaceName, "nexthop=::", "metric=1", "store=active").CombinedOutput(); err != nil {
			return fmt.Errorf("添加 TUN IPv6 默认路由: %s", strings.TrimSpace(string(output)))
		}
		p.addedV6 = true
	}
	p.activated = true
	return nil
}

func (p *windowsTUNRoutePlan) Stop() error {
	var first error
	remove := func(args ...string) {
		if output, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil && first == nil {
			first = fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
		}
	}
	if p.addedV6 {
		remove("netsh.exe", "interface", "ipv6", "delete", "route", "prefix=::/0", "interface="+tunInterfaceName, "nexthop=::", "store=active")
		p.addedV6 = false
	}
	if p.addedV4 {
		remove("route.exe", "DELETE", "0.0.0.0", "MASK", "0.0.0.0", "198.18.0.1", "IF", fmt.Sprint(p.tunIf.Index))
		p.addedV4 = false
	}
	if p.addedHost {
		remove("route.exe", "DELETE", p.endpoint.To4().String(), "MASK", "255.255.255.255", p.gateway.String(), "IF", fmt.Sprint(p.physIf.Index))
		p.addedHost = false
	}
	if p.ipv6 {
		_ = exec.Command("netsh.exe", "interface", "ipv6", "delete", "address", "interface="+tunInterfaceName, "address=fd00:198:18::1/64", "store=active").Run()
	}
	_ = exec.Command("netsh.exe", "interface", "ipv4", "set", "address", "name="+tunInterfaceName, "source=dhcp").Run()
	p.activated = false
	return first
}
