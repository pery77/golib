//go:build raylib_no_embed && ffi_no_embed

package startup

import (
	"syscall"
	"unsafe"
)

// init loads the libraries as ffi and raylib-go do, in the same order: when a
// build tells them not to carry copies of their own, as GoLib's builds do,
// they load libffi-8.dll and raylib.dll from the executable's folder, the
// system's folders or the folders in PATH. When one doesn't load, init writes
// why to the console, shows it in a message box if nobody would see the
// console, and ends the program before they panic.
func init() {
	problems := ""
	for _, name := range []string{"libffi-8.dll", "raylib.dll"} {
		if _, err := syscall.LoadLibrary(name); err != nil {
			problems += "cannot load library " + name + ": " + err.Error() + "\n"
		}
	}
	if problems == "" {
		return
	}
	// A dist build has no console to write to, so this fails there.
	syscall.Write(syscall.Stderr, []byte(problems+
		"The game needs libffi-8.dll and raylib.dll next to its executable or in a folder in PATH: "+
		"golib build, run, shot and test put them there, and golib dist puts them in the game's folder.\n"))
	if ErrorsUnseen() {
		exe := programFile()
		name := exe
		if n := len(name) - len(".exe"); n > 0 && (name[n:] == ".exe" || name[n:] == ".EXE") {
			name = name[:n]
		}
		ShowError(name, name+" can't start.\n\n"+problems+"\n"+
			exe+" needs the files that came with it, libffi-8.dll and raylib.dll, in its own folder. "+
			"Start it from the folder it came in, or unzip the game again.")
	}
	syscall.Exit(1)
}

// programFile returns the file name of the running executable.
func programFile() string {
	path := make([]uint16, 32768) // the longest path Windows allows
	n, _, _ := kernel32.NewProc("GetModuleFileNameW").Call(0, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	name := syscall.UTF16ToString(path[:n])
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '\\' || name[i] == '/' {
			return name[i+1:]
		}
	}
	return name
}
