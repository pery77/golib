//go:build windows

package device

import (
	"syscall"
	"unsafe"
)

func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	return uintptr(h), err
}

func librarySymbol(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}

// openLibraryBeside opens a library by its path, with the libraries it needs
// looked for in its own folder first (LOAD_WITH_ALTERED_SEARCH_PATH).
func openLibraryBeside(path string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	const loadWithAlteredSearchPath = 0x00000008
	h, _, callErr := loadLibraryEx.Call(uintptr(unsafe.Pointer(p)), 0, loadWithAlteredSearchPath)
	if h == 0 {
		return 0, callErr
	}
	return h, nil
}

var loadLibraryEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")
