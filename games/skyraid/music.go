package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"golib"
)

// The music. Nothing in the code names the file: the game plays whatever it
// finds in the music folder of its assets, so the tune can be swapped without
// building the game, the way the shaders in effects.go can.
const (
	musicFolder = "music"
	musicVolume = 0.1 // under the sound effects, which play at full volume
)

// musicFormats are the files golib.NewMusic plays, in lower case. Impulse
// Tracker (.it) is not among them, and nothing in GoLib converts one: open it
// in OpenMPT (<https://openmpt.org>) and save it as .xm, or render it to .ogg.
var musicFormats = []string{".ogg", ".mp3", ".wav", ".qoa", ".xm", ".mod"}

// docFormats are the files that belong in the music folder without being
// music, such as where the tune came from.
var docFormats = []string{".md", ".txt"}

// findMusic returns the first file in the game's music folder that GoLib can
// play, with its volume set, or nothing and a line saying why, for the screen.
// An empty folder is no music and no complaint.
func findMusic() (*golib.Music, string) {
	names, err := golib.ListAssets(musicFolder)
	if err != nil {
		return nil, err.Error()
	}
	var unplayable []string
	for _, name := range names {
		format := strings.ToLower(filepath.Ext(name))
		switch {
		case slices.Contains(musicFormats, format):
			music := golib.NewMusic(name)
			music.SetVolume(musicVolume)
			return music, ""
		case !slices.Contains(docFormats, format):
			unplayable = append(unplayable, name)
		}
	}
	if len(unplayable) > 0 {
		return nil, fmt.Sprintf("No music: GoLib cannot play %s. It plays %s.",
			strings.Join(unplayable, ", "), strings.Join(musicFormats, ", "))
	}
	return nil, ""
}
