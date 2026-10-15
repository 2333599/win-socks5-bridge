//go:build linux

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type gsettingsSnapshot struct {
	values map[string]string
}

func setSystemProxy(endpoint string) (func() error, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("gsettings"); err != nil {
		return nil, fmt.Errorf("system proxy mode currently requires GNOME gsettings")
	}
	snapshot := &gsettingsSnapshot{values: make(map[string]string)}
	keys := []string{
		"org.gnome.system.proxy mode",
		"org.gnome.system.proxy.http host",
		"org.gnome.system.proxy.http port",
		"org.gnome.system.proxy.https host",
		"org.gnome.system.proxy.https port",
		"org.gnome.system.proxy use-same-proxy",
	}
	for _, item := range keys {
		parts := strings.Fields(item)
		value, err := exec.Command("gsettings", "get", parts[0], parts[1]).Output()
		if err != nil {
			return nil, fmt.Errorf("read GNOME proxy setting %s: %w", item, err)
		}
		snapshot.values[item] = strings.TrimSpace(string(value))
	}
	changes := [][3]string{
		{"org.gnome.system.proxy.http", "host", quoteGVariant(host)},
		{"org.gnome.system.proxy.http", "port", port},
		{"org.gnome.system.proxy.https", "host", quoteGVariant(host)},
		{"org.gnome.system.proxy.https", "port", port},
		{"org.gnome.system.proxy", "use-same-proxy", "true"},
		{"org.gnome.system.proxy", "mode", "'manual'"},
	}
	for _, change := range changes {
		if output, err := exec.Command("gsettings", "set", change[0], change[1], change[2]).CombinedOutput(); err != nil {
			_ = snapshot.restore()
			return nil, fmt.Errorf("set GNOME proxy: %s", strings.TrimSpace(string(output)))
		}
	}
	return snapshot.restore, nil
}

func quoteGVariant(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'") + "'"
}

func (s *gsettingsSnapshot) restore() error {
	keys := []string{
		"org.gnome.system.proxy.http host",
		"org.gnome.system.proxy.http port",
		"org.gnome.system.proxy.https host",
		"org.gnome.system.proxy.https port",
		"org.gnome.system.proxy use-same-proxy",
		"org.gnome.system.proxy mode",
	}
	for _, item := range keys {
		parts := strings.Fields(item)
		if _, err := exec.Command("gsettings", "set", parts[0], parts[1], s.values[item]).CombinedOutput(); err != nil {
			return fmt.Errorf("restore GNOME proxy setting %s: %w", item, err)
		}
	}
	return nil
}
