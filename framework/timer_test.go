package golib

import "testing"

func TestTimerRunsOnce(t *testing.T) {
	timer := NewTimer(1)
	if !timer.Running() || timer.Left() != 1 || timer.Progress() != 0 {
		t.Fatalf("a new timer: running %v, %v left, progress %v", timer.Running(), timer.Left(), timer.Progress())
	}
	if timer.Tick(0.25) {
		t.Error("the timer finished after a quarter of its time")
	}
	if timer.Left() != 0.75 || timer.Progress() != 0.25 {
		t.Errorf("after a quarter: %v left, progress %v, want 0.75 and 0.25", timer.Left(), timer.Progress())
	}
	if !timer.Tick(0.75) {
		t.Error("the timer didn't finish when its time ran out")
	}
	if timer.Running() || timer.Left() != 0 || timer.Progress() != 1 {
		t.Errorf("a finished timer: running %v, %v left, progress %v", timer.Running(), timer.Left(), timer.Progress())
	}
	if timer.Tick(1) {
		t.Error("a finished timer finished again")
	}

	// Start runs it again, and Stop takes it back without finishing.
	timer.Start(2)
	if !timer.Running() || timer.Left() != 2 {
		t.Errorf("after Start: running %v, %v left", timer.Running(), timer.Left())
	}
	timer.Stop()
	if timer.Running() || timer.Tick(1) {
		t.Error("a stopped timer runs or finishes")
	}
	// The zero timer is stopped, and a timer given no time never finishes.
	var zero Timer
	none, negative := NewTimer(0), NewTimer(-1)
	if zero.Running() || zero.Tick(1) || none.Tick(1) || negative.Running() {
		t.Error("a timer with no time runs or finishes")
	}
}

func TestTimerRepeats(t *testing.T) {
	timer := NewRepeatingTimer(1)
	finishes := 0
	for range 8 {
		if timer.Tick(0.25) {
			finishes++
		}
	}
	if finishes != 2 {
		t.Errorf("a timer of 1 second finished %d times in 2 seconds, want 2", finishes)
	}
	if !timer.Running() {
		t.Error("a repeating timer stopped")
	}

	// An update that passes the end keeps the beat: the time past it counts
	// towards the next round.
	timer = NewRepeatingTimer(1)
	if !timer.Tick(2.5) {
		t.Fatal("the timer didn't finish after two and a half rounds")
	}
	if timer.Left() != 0.5 {
		t.Errorf("%v left after 2.5 seconds, want 0.5", timer.Left())
	}
	// Start gives a repeating timer a new round, and it keeps repeating.
	timer.Start(0.5)
	if !timer.Tick(0.5) || !timer.Tick(0.5) {
		t.Error("a repeating timer stopped after Start")
	}
}
