package golib

import "sync"

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
