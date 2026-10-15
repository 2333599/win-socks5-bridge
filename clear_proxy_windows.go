//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser   = 0x80000001
	keySetValue       = 0x0002
	regDWORD          = 4
	errorFileNotFound = 2
)

var (
	advapi32        = syscall.NewLazyDLL("advapi32.dll")
	regOpenKeyExW   = advapi32.NewProc("RegOpenKeyExW")
	regSetValueExW  = advapi32.NewProc("RegSetValueExW")
	regDeleteValueW = advapi32.NewProc("RegDeleteValueW")
	regCloseKey     = advapi32.NewProc("RegCloseKey")
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// ClearProxy disables the current user's Windows manual proxy and removes its
// server/exception values. It leaves automatic proxy discovery and PAC scripts
// untouched.
func ClearProxy() error {
	keyPath, _ := syscall.UTF16PtrFromString(internetSettingsKey)
	var key syscall.Handle
	status, _, _ := regOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, keySetValue, uintptr(unsafe.Pointer(&key)))
	if status != 0 {
		return fmt.Errorf("open current-user Internet Settings registry key: %w", syscall.Errno(status))
	}
	defer regCloseKey.Call(uintptr(key))

	name, _ := syscall.UTF16PtrFromString("ProxyEnable")
	var disabled uint32
	status, _, _ = regSetValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0, regDWORD, uintptr(unsafe.Pointer(&disabled)), unsafe.Sizeof(disabled))
	if status != 0 {
		return fmt.Errorf("disable ProxyEnable: %w", syscall.Errno(status))
	}
	for _, value := range []string{"ProxyServer", "ProxyOverride"} {
		valueName, _ := syscall.UTF16PtrFromString(value)
		status, _, _ = regDeleteValueW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)))
		// ERROR_FILE_NOT_FOUND means it was already cleared.
		if status != 0 && status != errorFileNotFound {
			return fmt.Errorf("delete %s: %w", value, syscall.Errno(status))
		}
	}
	refreshWinINet()
	return nil
}
func refreshWinINet() {
	proc := syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")
	// INTERNET_OPTION_SETTINGS_CHANGED and INTERNET_OPTION_REFRESH.
	_, _, _ = proc.Call(0, 39, 0, 0)
	_, _, _ = proc.Call(0, 37, 0, 0)
}
