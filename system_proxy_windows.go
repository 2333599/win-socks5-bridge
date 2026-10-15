//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

const keyQueryValue = 0x0001

var regQueryValueExW = advapi32.NewProc("RegQueryValueExW")

type savedRegistryValue struct {
	found bool
	typ   uint32
	data  []byte
}

func setSystemProxy(endpoint string) (func() error, error) {
	keyPath, _ := syscall.UTF16PtrFromString(internetSettingsKey)
	var key syscall.Handle
	access := uintptr(keySetValue | keyQueryValue)
	status, _, _ := regOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, access, uintptr(unsafe.Pointer(&key)))
	if status != 0 {
		return nil, fmt.Errorf("open Internet Settings: %w", syscall.Errno(status))
	}
	defer regCloseKey.Call(uintptr(key))

	snapshot := make(map[string]savedRegistryValue, 3)
	for _, name := range []string{"ProxyEnable", "ProxyServer", "ProxyOverride"} {
		v, err := queryProxyValue(key, name)
		if err != nil {
			return nil, err
		}
		snapshot[name] = v
	}
	proxyText, _ := syscall.UTF16FromString(endpoint)
	proxyBytes := make([]byte, len(proxyText)*2)
	for i, v := range proxyText {
		binary.LittleEndian.PutUint16(proxyBytes[i*2:], v)
	}
	enabled := make([]byte, 4)
	binary.LittleEndian.PutUint32(enabled, 1)
	if err := writeProxyValue(key, "ProxyServer", 1, proxyBytes); err != nil {
		return nil, err
	}
	if err := writeProxyValue(key, "ProxyEnable", regDWORD, enabled); err != nil {
		_ = restoreProxyRegistry(key, snapshot)
		return nil, err
	}
	refreshWinINet()

	return func() error {
		path, _ := syscall.UTF16PtrFromString(internetSettingsKey)
		var restoreKey syscall.Handle
		status, _, _ := regOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, access, uintptr(unsafe.Pointer(&restoreKey)))
		if status != 0 {
			return fmt.Errorf("open Internet Settings to restore proxy: %w", syscall.Errno(status))
		}
		defer regCloseKey.Call(uintptr(restoreKey))
		err := restoreProxyRegistry(restoreKey, snapshot)
		refreshWinINet()
		return err
	}, nil
}

func queryProxyValue(key syscall.Handle, name string) (savedRegistryValue, error) {
	valueName, _ := syscall.UTF16PtrFromString(name)
	var typ, size uint32
	status, _, _ := regQueryValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	if status == errorFileNotFound {
		return savedRegistryValue{}, nil
	}
	if status != 0 {
		return savedRegistryValue{}, fmt.Errorf("read %s: %w", name, syscall.Errno(status))
	}
	data := make([]byte, size)
	if size == 0 {
		return savedRegistryValue{found: true, typ: typ, data: data}, nil
	}
	status, _, _ = regQueryValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return savedRegistryValue{}, fmt.Errorf("read %s: %w", name, syscall.Errno(status))
	}
	return savedRegistryValue{found: true, typ: typ, data: data}, nil
}

func writeProxyValue(key syscall.Handle, name string, typ uint32, data []byte) error {
	valueName, _ := syscall.UTF16PtrFromString(name)
	var ptr uintptr
	if len(data) > 0 {
		ptr = uintptr(unsafe.Pointer(&data[0]))
	}
	status, _, _ := regSetValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)), 0, uintptr(typ), ptr, uintptr(len(data)))
	if status != 0 {
		return fmt.Errorf("write %s: %w", name, syscall.Errno(status))
	}
	return nil
}

func restoreProxyRegistry(key syscall.Handle, snapshot map[string]savedRegistryValue) error {
	for _, name := range []string{"ProxyEnable", "ProxyServer", "ProxyOverride"} {
		value := snapshot[name]
		if value.found {
			if err := writeProxyValue(key, name, value.typ, value.data); err != nil {
				return err
			}
			continue
		}
		valueName, _ := syscall.UTF16PtrFromString(name)
		status, _, _ := regDeleteValueW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)))
		if status != 0 && status != errorFileNotFound {
			return fmt.Errorf("delete %s: %w", name, syscall.Errno(status))
		}
	}
	return nil
}
