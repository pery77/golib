//go:build !windows

package golib

// showErrorDialog does nothing outside Windows. There, a dist build writes
// errors to the terminal it was started from, like a debug build.
func showErrorDialog(title, message string) {}
