package golib

import (
	"os"
	"testing"
)

// macOS opens, draws in and closes windows only on a program's main thread,
// and go test runs every test on another one: a test that opened a window
// itself would stop the whole program. raylib-go keeps the main goroutine on
// the main thread (its init locks it there), so TestMain runs the tests in a
// goroutine of their own and keeps the main goroutine for the work
// onMainThread hands it, one piece at a time. Windows and Linux don't need
// it, and do the same, so the tests run alike everywhere.

// mainThread carries work to the main goroutine.
var mainThread = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case work := <-mainThread:
			work()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMainThread runs work on the main thread and waits for it. work must not
// call t.Fatal, t.Skip or anything else that ends a test: those only work
// from the test's own goroutine, so work returns what went wrong instead.
func onMainThread(work func()) {
	ran := make(chan struct{})
	mainThread <- func() {
		defer close(ran)
		work()
	}
	<-ran
}
