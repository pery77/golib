package golib

import "testing"

func TestClockKeepsGameSpeedAtAnyFrameRate(t *testing.T) {
	for _, fps := range []float64{30, 59.94, 60, 75, 144, 240} {
		var c clock
		updates := 0
		frames := int(fps * 10) // ten seconds
		for i := 0; i < frames; i++ {
			updates += c.advance(1 / fps)
		}
		if updates < 599 || updates > 601 {
			t.Errorf("%v fps for 10 s: %d updates, want 600 (+-1)", fps, updates)
		}
	}
}

func TestClockRunsOneUpdatePerFrameAt60FPS(t *testing.T) {
	var c clock
	// Real frame times jitter around 1/60 s but average exactly 1/60 s.
	jitter := []float64{-0.0007, 0.0006, -0.0001, -0.0009, 0.0008, 0.0003}
	for i := 0; i < 600; i++ {
		if got := c.advance(updateStep + jitter[i%len(jitter)]); got != 1 {
			t.Fatalf("frame %d: %d updates, want 1", i, got)
		}
	}
}

func TestClockSkipsLongPauses(t *testing.T) {
	var c clock
	if got := c.advance(2); got != maxUpdatesPerFrame {
		t.Errorf("after a 2 s frame: %d updates, want %d", got, maxUpdatesPerFrame)
	}
	if got := c.advance(updateStep); got != 1 {
		t.Errorf("next normal frame: %d updates, want 1 (the backlog must be dropped)", got)
	}
}

func TestClockIgnoresNegativeTime(t *testing.T) {
	var c clock
	if got := c.advance(-1); got != 0 {
		t.Errorf("advance(-1) = %d, want 0", got)
	}
}
