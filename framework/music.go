package golib

import (
	"fmt"
	"path"
	"slices"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// musicFormats are the file types raylib streams, by extension. Impulse Tracker
// (.it) is not among them.
var musicFormats = []string{".ogg", ".mp3", ".wav", ".qoa", ".xm", ".mod"}

// Music is a piece of music in the game's assets folder. raylib streams it a
// little at a time while the game runs, so a long track costs little memory,
// and it loops until it is stopped.
//
//	var theme = golib.NewMusic("music/theme.xm") // games/<game>/assets/music/theme.xm
//
//	func (s *playScene) Update(input *golib.Input, dt float32) {
//		theme.Play() // starts the music, and keeps it going
//		if input.KeyPressed(golib.KeyF3) {
//			theme.Pause() // or Play again to carry on, Stop to go back to the start
//		}
//	}
//
// The file is one of .ogg, .mp3, .wav, .qoa, .xm or .mod. Tracker modules (.xm
// and .mod) are a few dozen kilobytes, which makes them a good fit for a golib
// dist build, where the assets folder ships inside the executable.
//
// The music is read the first time it plays, so a game can make it before Run
// opens the window. Nothing plays when there is no sound device, so
// screenshots from golib shot stay silent.
type Music struct {
	name    string
	data    []byte // raylib streams from this, so it has to stay reachable
	stream  rl.Music
	loaded  bool
	tracked bool
	playing bool
	paused  bool
	volume  float32
	err     error
}

// NewMusic returns the music in the game's assets folder named name, which is
// relative to that folder and uses forward slashes, as in [ReadAsset]. Run
// stops with an error the first time it plays if the file is missing or is not
// one of the formats [Music] lists.
func NewMusic(name string) *Music {
	return &Music{name: name, volume: 1}
}

// Play starts the music, or carries on after Pause. It is safe to call in every
// update: music that is already playing keeps playing.
func (m *Music) Play() {
	if !audio.isReady() {
		return
	}
	audio.trackMusic(m)
	if !m.load() {
		return
	}
	switch {
	case m.paused:
		rl.ResumeMusicStream(m.stream)
		m.paused = false
	case !m.playing:
		rl.PlayMusicStream(m.stream)
		m.playing = true
	}
}

// Pause holds the music where it is. Play carries on from there.
func (m *Music) Pause() {
	if !m.playing || m.paused {
		return
	}
	rl.PauseMusicStream(m.stream)
	m.paused = true
}

// Stop ends the music. Play starts it again from the beginning.
func (m *Music) Stop() {
	if !m.playing {
		return
	}
	rl.StopMusicStream(m.stream)
	m.playing, m.paused = false, false
}

// Playing reports whether the music is playing now: false while it is paused,
// stopped, or waiting for a sound device.
func (m *Music) Playing() bool {
	return m.playing && !m.paused
}

// SetVolume sets how loud this music is, from 0 (silent) to 1 (full), under the
// volume [SetVolume] sets for everything. Use it to keep music under the sound
// effects. It works before the music plays.
func (m *Music) SetVolume(volume float32) {
	m.volume = max(0, min(volume, 1))
	if m.loaded {
		rl.SetMusicVolume(m.stream, m.volume)
	}
}

// load reads the music the first time it plays, and reports whether it is
// ready. A failure is kept in m.err, which Run reports, and is not tried again.
func (m *Music) load() bool {
	if m.loaded || m.err != nil {
		return m.loaded
	}
	format := strings.ToLower(path.Ext(m.name))
	if !slices.Contains(musicFormats, format) {
		m.err = fmt.Errorf("golib.NewMusic(%q): GoLib cannot play %q files: use one of %s", m.name, format, strings.Join(musicFormats, ", "))
		return false
	}
	data, err := ReadAsset(m.name)
	if err != nil {
		m.err = fmt.Errorf("golib.NewMusic(%q): %w", m.name, err)
		return false
	}
	stream := rl.LoadMusicStreamFromMemory(format, data, int32(len(data)))
	if !rl.IsMusicValid(stream) {
		m.err = fmt.Errorf("golib.NewMusic(%q): raylib could not read the music: see the raylib warnings above", m.name)
		return false
	}
	stream.Looping = true
	m.data, m.stream, m.loaded = data, stream, true
	rl.SetMusicVolume(stream, m.volume)
	return true
}

// unload frees the music, so that it is read again if a game runs again.
func (m *Music) unload() {
	if m.loaded {
		rl.StopMusicStream(m.stream)
		rl.UnloadMusicStream(m.stream)
	}
	m.data, m.loaded, m.tracked, m.playing, m.paused, m.err = nil, false, false, false, false, nil
}
