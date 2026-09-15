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
