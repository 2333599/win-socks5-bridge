//go:build !windows && !linux

package main

import (
	"errors"
	"net"
)

type unsupportedTUNRoutePlan struct{}

func newTUNRoutePlan(net.IP) (tunRoutePlan, error) {
	return nil, errors.New("TUN 全流量模式仅支持 Windows 和 Linux")
}
func (*unsupportedTUNRoutePlan) Prepare() error         { return errors.New("unsupported platform") }
func (*unsupportedTUNRoutePlan) Activate() error        { return errors.New("unsupported platform") }
func (*unsupportedTUNRoutePlan) Stop() error            { return nil }
func (*unsupportedTUNRoutePlan) DeviceName() string     { return tunInterfaceName }
func (*unsupportedTUNRoutePlan) ProxyInterface() string { return "" }
func (*unsupportedTUNRoutePlan) RoutingMark() int       { return 0 }
