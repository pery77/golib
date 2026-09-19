package golib

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// pendingError is the first mistake found with an asset while the game runs,
// such as a missing file or a frame a sprite doesn't have. Update and Draw
// can't return errors, so the code that finds one keeps it here, and Run
// returns it after the update or the draw.
var pendingError struct {
	sync.Mutex
	err error
}

// reportError keeps err for Run to return, unless an earlier mistake is
// waiting already.
func reportError(err error) {
	pendingError.Lock()
	defer pendingError.Unlock()
	if pendingError.err == nil {
		pendingError.err = err
	}
}

// takeError returns the mistake waiting for Run, if any, and forgets it.
func takeError() error {
	pendingError.Lock()
	defer pendingError.Unlock()
	err := pendingError.err
	pendingError.err = nil
	return err
}

// warnings is where warnf writes. Tests read what GoLib said; nothing else
// touches it.
var warnings io.Writer = os.Stderr

// warnf prints a line about something the game can carry on without, such as
// music this machine cannot play. A mistake stops Run, through reportError; a
// warning only tells whoever is watching, on the console of a desktop build
// and in the developer tools of a browser.
func warnf(format string, args ...any) {
	fmt.Fprintf(warnings, format+"\n", args...)
}
