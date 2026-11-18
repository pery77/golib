package golib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"golib/internal/device"
	"golib/internal/startup"
)

// crashStackLines limits the stack trace in a crash dialog, so it fits on
// screen.
const crashStackLines = 30

// reportInDialog shows Run's error, or a panic in the game, where the player
// will see it, then lets the panic continue. Run defers it when the console
// can't show the error: a golib dist build on Windows has none, a debug build
// started from Explorer has one that closes as soon as the game ends, and
// nobody playing in a browser looks at one. It must be deferred directly, or
// recover returns nil.
func reportInDialog(title string, err *error) {
	if r := recover(); r != nil {
		message := crashMessage(r, debug.Stack())
		if crashKept != "" {
			message += "\n\nThis report is kept in " + crashKept
		}
		showError(title, message)
		panic(r)
	}
	if *err != nil {
		showError(title, (*err).Error())
	}
}

// showError shows a message the way this machine shows them: a message box on
// Windows, the page in a browser, and nothing where the console is enough.
func showError(title, message string) {
	startup.ShowError(title, message)
	device.ShowError(title, message)
}

// crashMessage describes a panic for a crash dialog, with the start of its
// stack trace.
func crashMessage(r any, stack []byte) string {
	lines := strings.Split(strings.TrimSpace(string(stack)), "\n")
	if len(lines) > crashStackLines {
		lines = append(lines[:crashStackLines], "...")
	}
	return fmt.Sprintf("The game crashed: %v\n\n%s", r, strings.Join(lines, "\n"))
}

// crashFile is the file a crash is kept in, in the folder where SaveData
// saves.
const crashFile = "crash.txt"

// crashKept is where the crash was kept, once keepCrash has written it.
var crashKept string

// keepCrash keeps a panic in the game in a file, crash.txt in the folder
// where SaveData saves, then lets the panic continue: a message box is gone
// once it is closed, and a player can send a file to the game's author. Each
// crash replaces the one before. Run defers it directly, or recover returns
// nil.
func keepCrash() {
	if r := recover(); r != nil {
		crashKept = writeCrash(r, debug.Stack(), time.Now())
		panic(r)
	}
}

// writeCrash writes a panic's report where saved data goes, and returns the
// file's path, or "" when nothing was written: saved data has no folder
// under go test and golib shot, nor in a browser, and a crash is no time to
// report a second error.
func writeCrash(r any, stack []byte, when time.Time) string {
	folder, place, err := saveFolder()
	if err != nil || place != saveInFolder {
		return ""
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(folder, crashFile)
	if err := os.WriteFile(path, []byte(crashReport(r, stack, when)), 0o644); err != nil {
		return ""
	}
	return path
}

// crashReport describes a panic for crash.txt: what the dialog says, with
// the whole stack trace, when it happened, and the game's version and the
// system it ran on, which the author asks for first.
func crashReport(r any, stack []byte, when time.Time) string {
	version := GameVersion()
	if version == "" {
		version = "unknown"
	}
	return fmt.Sprintf("The game crashed: %v\n\nWhen: %s\nVersion: %s\nSystem: %s/%s\n\n%s\n",
		r, when.Format("2006-01-02 15:04:05"), version, runtime.GOOS, runtime.GOARCH, strings.TrimSpace(string(stack)))
}
