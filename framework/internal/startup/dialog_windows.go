package startup

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
)

// ShowError shows message in a Windows message box with an error icon, and
// waits until the player closes it. Ctrl+C in the box copies the text.
func ShowError(title, message string) {
	const mbIconError = 0x10
	titlePtr := utf16Ptr(title)
	messagePtr := utf16Ptr(message)
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}

// ErrorsUnseen reports whether nobody would see what the program writes to
// its console. A golib dist build has no console. Windows opens a console
// window of its own for a debug build started from Explorer, and closes it as
// soon as the game ends, errors and all. golib run, golib shot, go test, F5
// and a terminal share their console with the game.
func ErrorsUnseen() bool {
	// GetConsoleProcessList returns how many programs share the console, 0
	// when there is none. It needs room for at least one process ID.
	var ids [2]uint32
	count, _, _ := kernel32.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return count <= 1
}

// utf16Ptr returns s as a NUL-terminated UTF-16 string, without the NUL
// characters s has, which would end it early.
func utf16Ptr(s string) *uint16 {
	text := make([]rune, 0, len(s)+1)
	for _, r := range s {
		if r != 0 {
			text = append(text, r)
		}
	}
	ptr, _ := syscall.UTF16PtrFromString(string(text))
	return ptr
}
