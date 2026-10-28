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

// moduleFormats are the tracker modules among them: a few dozen kilobytes for
// a whole tune, and what no browser plays.
var moduleFormats = []string{".xm", ".mod"}

// docFormats are the files that belong in the music folder without being
// music, such as where the tune came from.
var docFormats = []string{".md", ".txt"}

// findMusic returns the tune in the game's music folder, with its volume set,
// or nothing and a line saying why, for the screen. An empty folder is no
// music and no complaint.
func findMusic() (*golib.Music, string) {
	names, err := golib.ListAssets(musicFolder)
	if err != nil {
		return nil, err.Error()
	}
	name, note := chooseMusic(names)
	if name == "" {
		return nil, note
	}
	music := golib.NewMusic(name)
	music.SetVolume(musicVolume)
	return music, note
}

// chooseMusic picks the file to name out of what the music folder holds, and
// otherwise a line saying why there is none.
//
// One tune can sit in that folder twice: a tracker module, and a render of it
// in a format browsers play, under the same name beside it, because no browser
// plays modules. They are one tune, and the module is the one to name: GoLib
// plays the render itself where the module cannot be played, in a web build,
// and the module everywhere else, which is the one to have there.
func chooseMusic(names []string) (name, note string) {
	var unplayable []string
	for _, candidate := range names {
		format := strings.ToLower(filepath.Ext(candidate))
		switch {
		case slices.Contains(musicFormats, format):
			if standsInForModule(candidate, names) {
				continue // the module beside it is the tune to name
			}
			return candidate, ""
		case !slices.Contains(docFormats, format):
			unplayable = append(unplayable, candidate)
		}
	}
	if len(unplayable) > 0 {
		return "", fmt.Sprintf("No music: GoLib cannot play %s. It plays %s.",
			strings.Join(unplayable, ", "), strings.Join(musicFormats, ", "))
	}
	return "", ""
}

// standsInForModule reports whether name is in the folder for a browser's
// sake: a render of the tracker module of the same name beside it.
func standsInForModule(name string, names []string) bool {
	if isModule(name) {
		return false
	}
	for _, other := range names {
		if isModule(other) && strings.EqualFold(withoutFormat(other), withoutFormat(name)) {
			return true
		}
	}
	return false
}

func isModule(name string) bool {
	return slices.Contains(moduleFormats, strings.ToLower(filepath.Ext(name)))
}

func withoutFormat(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
