package golib

import (
	"strings"
	"syscall"
	"unsafe"
)

// showErrorDialog shows message in a Windows message box with an error icon,
// and waits until the player closes it. Ctrl+C in the box copies the text.
func showErrorDialog(title, message string) {
	const mbIconError = 0x10
	// UTF16PtrFromString rejects NUL characters, so drop them.
	titlePtr, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(title, "\x00", ""))
	messagePtr, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(message, "\x00", ""))
	messageBox := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}

// ownConsole reports whether the game is the only program in its console
// window. Windows opens a console window of its own for a debug build started
// from Explorer, and closes it as soon as the game ends, errors and all. golib
// run, golib shot, F5 and a terminal share their console with the game, and a
// dist build has none.
func ownConsole() bool {
	// GetConsoleProcessList returns how many programs share the console, 0
	// when there is none. It needs room for at least one process ID.
	var ids [2]uint32
	getConsoleProcessList := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	count, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return count == 1
}
