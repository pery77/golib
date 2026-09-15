package golib

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// crashStackLines limits the stack trace in a crash dialog, so it fits on
// screen.
const crashStackLines = 30

// reportInDialog shows Run's error, or a panic in the game, in a message box,
// then lets the panic continue. golib dist builds defer it in Run: on Windows
// they have no console, so nothing else would tell the player what went wrong.
// It must be deferred directly, or recover returns nil.
func reportInDialog(title string, err *error) {
	if r := recover(); r != nil {
		showErrorDialog(title, crashMessage(r, debug.Stack()))
		panic(r)
	}
	if *err != nil {
		showErrorDialog(title, (*err).Error())
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
