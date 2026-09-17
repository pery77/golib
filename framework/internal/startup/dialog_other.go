//go:build !windows

package startup

// ShowError does nothing outside Windows. There, a dist build writes errors
// to the terminal it was started from, like a debug build.
func ShowError(title, message string) {}

// ErrorsUnseen is false outside Windows, where a file manager starts a
// program without opening a terminal window for it.
func ErrorsUnseen() bool { return false }
