//go:build !windows

package golib

// showErrorDialog does nothing outside Windows. There, a dist build writes
// errors to the terminal it was started from, like a debug build.
func showErrorDialog(title, message string) {}

// ownConsole is false outside Windows, where a file manager starts a program
// without opening a terminal window for it.
func ownConsole() bool { return false }
