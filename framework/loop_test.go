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

// TestAdvanceUnlessPaused checks a game with Config.PauseUnfocused: while its
// window doesn't have the player's attention it runs no updates, and it comes
// back where it was instead of running through the time it waited.
func TestAdvanceUnlessPaused(t *testing.T) {
	var c clock
	if updates := c.advanceUnlessPaused(updateStep, false); updates != 1 {
		t.Errorf("a normal frame ran %d updates, want 1", updates)
	}
	// Ten seconds go by while the player works in another program.
	if updates := c.advanceUnlessPaused(10, true); updates != 0 {
		t.Errorf("a paused frame ran %d updates, want 0", updates)
	}
	if c.pending != 0 {
		t.Errorf("a paused frame left %v seconds to catch up on, want none", c.pending)
	}
	// Back in the game: one update for one frame, not a burst.
	if updates := c.advanceUnlessPaused(updateStep, false); updates != 1 {
		t.Errorf("the frame after the pause ran %d updates, want 1", updates)
	}
	// A paused frame drops even the time gathered before it.
	c.pending = updateStep * 3
	if updates := c.advanceUnlessPaused(0, true); updates != 0 || c.pending != 0 {
		t.Errorf("a paused frame ran %d updates and left %v seconds, want 0 and 0", updates, c.pending)
	}
}
