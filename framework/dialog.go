package golib

import (
	"fmt"
	"runtime/debug"
	"strings"

	"golib/internal/startup"
)

// crashStackLines limits the stack trace in a crash dialog, so it fits on
// screen.
const crashStackLines = 30

// reportInDialog shows Run's error, or a panic in the game, in a message box,
// then lets the panic continue. Run defers it when the console can't show the
// error: a golib dist build on Windows has none, and a debug build started
// from Explorer has one that closes as soon as the game ends, so nothing else
// would tell the player what went wrong. It must be deferred directly, or
// recover returns nil.
func reportInDialog(title string, err *error) {
	if r := recover(); r != nil {
		startup.ShowError(title, crashMessage(r, debug.Stack()))
		panic(r)
	}
	if *err != nil {
		startup.ShowError(title, (*err).Error())
	}
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
