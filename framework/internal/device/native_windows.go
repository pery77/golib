//go:build windows

package device

import "syscall"

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func librarySymbol(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}
