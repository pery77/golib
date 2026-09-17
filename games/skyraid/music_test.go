package main

import "testing"

// TestTheMusicPlays checks that the assets folder holds a tune GoLib can play,
// so that a file in a format it doesn't read, such as Impulse Tracker's .it,
// doesn't leave the game silent without anyone noticing.
func TestTheMusicPlays(t *testing.T) {
	music, note := findMusic()
	if note != "" {
		t.Fatal(note)
	}
	if music == nil {
		t.Fatalf("no music: put a file GoLib can play (%v) in assets/%s", musicFormats, musicFolder)
	}
}
