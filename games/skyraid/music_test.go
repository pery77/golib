package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golib"
)

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

// The music folder holds the tracker module and the render of it that a
// browser plays. The module is the one the game names: GoLib puts the render
// in its place where modules don't play, so naming the render instead would
// give every desktop build the larger file for nothing.
func TestTheModuleIsTheTuneWhenBothAreThere(t *testing.T) {
	names, err := golib.ListAssets(musicFolder)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(names, isModule) {
		t.Skip("this game's music is not a tracker module")
	}
	name, note := chooseMusic(names)
	if note != "" {
		t.Fatal(note)
	}
	if !isModule(name) {
		t.Errorf("the game plays %q out of %v, want the tracker module: the render beside it is only for a browser", name, names)
	}
}

func TestChooseMusic(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
		note  string // part of the line for the screen, empty when there is none
	}{
		{
			name:  "a module and its render are one tune, and the module is it",
			files: []string{"theme.ogg", "theme.xm"},
			want:  "theme.xm",
		},
		{
			name:  "whatever letter case they are written in",
			files: []string{"Theme.OGG", "Theme.XM"},
			want:  "Theme.XM",
		},
		{
			name:  "a game whose music is an .ogg of its own plays it",
			files: []string{"theme.ogg"},
			want:  "theme.ogg",
		},
		{
			name:  "a render of another tune is not this one's",
			files: []string{"menu.ogg", "theme.xm"},
			want:  "menu.ogg",
		},
		{
			name:  "notes about the tune are not the tune",
			files: []string{"WHERE-IT-CAME-FROM.md", "theme.mod"},
			want:  "theme.mod",
		},
		{
			name:  "a format GoLib cannot read says so",
			files: []string{"theme.it"},
			note:  "cannot play theme.it",
		},
		{name: "an empty folder is no music and no complaint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, note := chooseMusic(tt.files)
			if name != tt.want {
				t.Errorf("chooseMusic(%v) plays %q, want %q", tt.files, name, tt.want)
			}
			if tt.note == "" && note != "" {
				t.Errorf("chooseMusic(%v) says %q, want nothing", tt.files, note)
			}
			if tt.note != "" && !strings.Contains(note, tt.note) {
				t.Errorf("chooseMusic(%v) says %q, want a line holding %q", tt.files, note, tt.note)
			}
		})
	}
}

// The formats the game knows are the ones GoLib plays, and the modules are
// among them: a list that drifts would quietly drop a tune.
func TestMusicFormatsHoldTheModules(t *testing.T) {
	for _, format := range moduleFormats {
		if !slices.Contains(musicFormats, format) {
			t.Errorf("%s is a module format but not one of the music formats %v", format, musicFormats)
		}
		if strings.ToLower(filepath.Ext("tune"+format)) != format {
			t.Errorf("%q is not a file format", format)
		}
	}
}
