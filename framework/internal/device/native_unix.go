//go:build !js && !windows

package device

import "github.com/ebitengine/purego"

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
}

func librarySymbol(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}

// openLibraryBeside opens a library by its path; the libraries it needs are
// found beside it through its rpath.
func openLibraryBeside(path string) (uintptr, error) {
	return openLibrary(path)
}
