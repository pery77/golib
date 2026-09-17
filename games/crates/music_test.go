package main

import (
	"math"
	"testing"

	"golib"
)

func TestTuneIsWellFormed(t *testing.T) {
	j, err := newJukebox(tune)
	if err != nil {
		t.Fatal(err)
	}
	if len(j.steps) != 16*16 {
		t.Errorf("the tune has %d steps, want 16 bars of 16", len(j.steps))
	}
	if len(j.steps[0]) == 0 {
		t.Error("the tune starts with silence")
	}
}

func TestNoteFrequency(t *testing.T) {
	tests := []struct {
		name string
		want float64
	}{
		{"A4", 440},
		{"A3", 220},
		{"C4", 261.63},
		{"C#5", 554.37},
		{"Bb3", 233.08},
		{"G#4", 415.30},
	}
	for _, tt := range tests {
		got, err := noteFrequency(tt.name)
		if err != nil || math.Abs(float64(got)-tt.want) > 0.01 {
			t.Errorf("noteFrequency(%q) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "H4", "A", "A#", "A9", "C#10", "x"} {
		if _, err := noteFrequency(bad); err == nil {
			t.Errorf("noteFrequency(%q) didn't fail", bad)
		}
	}
}

func TestBadScores(t *testing.T) {
	for _, voices := range [][]voice{
		{{score: "A4 - - -"}},                                                  // not a whole bar
		{{score: "A4 - - - - - - - - - - - - - - Q"}},                          // not a note
		{{score: sixteen("A4")}, {score: sixteen("A4") + " " + sixteen("A4")}}, // voices of different lengths
	} {
		if _, err := newJukebox(voices); err == nil {
			t.Errorf("newJukebox(%+v) didn't fail", voices)
		}
	}
}

// sixteen returns one bar of a note held for the whole bar.
func sixteen(note string) string {
	bar := note
	for range 15 {
		bar += " -"
	}
	return bar
}

func TestJukeboxKeepsTime(t *testing.T) {
	j, err := newJukebox([]voice{{wave: golib.WaveSine, score: sixteen("A4")}})
	if err != nil {
		t.Fatal(err)
	}
	// A step is 8 updates: after 80 updates, 10 steps have started.
	for range 80 {
		j.update(true, dt)
	}
	if j.next != 10 {
		t.Errorf("after 80 updates the tune is at step %d, want 10", j.next)
	}
	// Off, it waits.
	for range 80 {
		j.update(false, dt)
	}
	if j.next != 10 {
		t.Errorf("with the music off the tune moved to step %d", j.next)
	}
	// It loops after its last step.
	for range 6 * 8 {
		j.update(true, dt)
	}
	if j.next != 0 {
		t.Errorf("after 16 steps the tune is at step %d, want 0", j.next)
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
