package main

import (
	"testing"

	"golib"
)

// TestTheSoundFilesAreThere checks that every sound file the game plays is in
// the assets folder. A renamed or deleted file would otherwise show up only
// when that sound first plays, which ends the game in the middle of a wave.
func TestTheSoundFilesAreThere(t *testing.T) {
	if len(soundFiles) == 0 {
		t.Fatal("no sound comes from a file: soundFile should have collected them")
	}
	for _, name := range soundFiles {
		if _, err := golib.ReadAsset(name); err != nil {
			t.Error(err)
		}
	}
}
