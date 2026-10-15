//go:build !windows && !linux

package main

import "errors"

func setSystemProxy(string) (func() error, error) {
	return nil, errors.New("system proxy mode is supported only on Windows and GNOME Linux")
}
