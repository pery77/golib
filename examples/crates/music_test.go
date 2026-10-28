package main

import (
	"strconv"
	"strings"
	"testing"
)

// tuneBeats returns how many beats the notes of a voice last. GoLib checks the
// notes themselves, and golib.Run reports a mistake in them, so this only
// counts lengths.
func tuneBeats(t *testing.T, notes string) float64 {
	t.Helper()
	total := 0.0
	for _, item := range strings.Fields(notes) {
		beats := 1.0
		if _, length, found := strings.Cut(item, "/"); found {
			value, err := strconv.ParseFloat(length, 64)
			if err != nil {
				t.Fatalf("the note %q doesn't say how many beats it lasts: %v", item, err)
			}
			beats = value
		}
		total += beats
	}
	return total
}

// The melody, the bass and the tick must all last the same number of beats:
// the longest voice sets the length of the tune, and a short one would fall
// out of step with the others every time it loops.
func TestTuneVoicesLineUp(t *testing.T) {
	const beats = 16 * 8 // 16 bars of 8 beats
	if len(themeSpec.Voices) != 3 {
		t.Fatalf("the tune has %d voices, want the melody, the bass and the tick", len(themeSpec.Voices))
	}
	for i, voice := range themeSpec.Voices {
		if got := tuneBeats(t, voice.Notes); got != beats {
			t.Errorf("voice %d lasts %g beats, want %d", i+1, got, beats)
		}
	}
}

func TestStickDirections(t *testing.T) {
	tests := []struct {
		x, y float32
		want direction
	}{
		{0, 0, noDirection},
		{0.3, -0.4, noDirection},
		{-0.9, 0.2, left},
		{0.7, 0.6, right},
		{0.2, -0.8, up},
		{-0.5, 0.9, down},
	}
	for _, tt := range tests {
		if got := stickToDirection(tt.x, tt.y); got != tt.want {
			t.Errorf("stickToDirection(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
		}
	}
}
