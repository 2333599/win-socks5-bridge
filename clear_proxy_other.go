//go:build !windows

package main

import "errors"

func ClearProxy() error {
	return errors.New("-clear-proxy is available only on Windows")
}
