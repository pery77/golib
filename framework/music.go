package golib

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"golib/internal/device"
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
	tune    *TuneSpec // notes to make the music from, instead of reading a file
	data    []byte    // raylib streams from this, so it has to stay reachable
	stream  device.Music
	loaded  bool
	tracked bool
	checked bool // a tune has been made once, to find mistakes without a sound device
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
		// Under golib shot and in tests there is no sound device, but a
		// mistake in a tune still stops Run.
		m.checkTune()
		return
	}
	audio.trackMusic(m)
	if !m.load() {
		return
	}
	switch {
	case m.paused:
		device.ResumeMusic(m.stream)
		m.paused = false
	case !m.playing:
		device.PlayMusic(m.stream)
		m.playing = true
	}
}

// Pause holds the music where it is. Play carries on from there.
func (m *Music) Pause() {
	if !m.playing || m.paused {
		return
	}
	device.PauseMusic(m.stream)
	m.paused = true
}

// Stop ends the music. Play starts it again from the beginning.
func (m *Music) Stop() {
	if !m.playing {
		return
	}
	device.StopMusic(m.stream)
	m.playing, m.paused = false, false
}

// Err returns what stopped the music from playing, or nil: a missing or
// unreadable file for [NewMusic], or a mistake in the notes for [NewTune].
// Playing music reports the same mistake to Run, so a game needs Err only to
// handle it itself, or to check its own tune in a test, where nothing plays:
//
//	if err := theme.Err(); err != nil {
//		t.Error(err)
//	}
//
// For a tune, Err makes it once, as playing it would; for a music file, it
// tells only what playing it has found so far, since the file is read when the
// music first plays, with a sound device.
func (m *Music) Err() error {
	if m.tune != nil && !m.checked {
		m.checked = true
		if _, err := m.tune.samples(); err != nil {
			m.err = err
		}
	}
	return m.err
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
		device.SetMusicVolume(m.stream, m.volume)
	}
}

// load reads the music the first time it plays, and reports whether it is
// ready. A failure is kept in m.err, which Run reports, and is not tried again.
func (m *Music) load() bool {
	if m.loaded || m.err != nil {
		return m.loaded
	}
	var data []byte
	format := ".wav"
	if m.tune != nil {
		samples, err := m.tune.samples()
		if err != nil {
			m.err = err
			return false
		}
		data = wav(samples)
	} else {
		format = strings.ToLower(path.Ext(m.name))
		if !slices.Contains(musicFormats, format) {
			m.err = fmt.Errorf("golib.NewMusic(%q): GoLib cannot play %q files: use one of %s", m.name, format, strings.Join(musicFormats, ", "))
			return false
		}
		read, err := ReadAsset(m.name)
		if err != nil {
			m.err = fmt.Errorf("golib.NewMusic(%q): %w", m.name, err)
			return false
		}
		data = read
	}
	stream, ok := device.NewMusic(format, data, true)
	if !ok {
		m.err = fmt.Errorf("golib: raylib could not play %s: see the raylib warnings above", m.describe())
		return false
	}
	m.data, m.stream, m.loaded = data, stream, true
	device.SetMusicVolume(stream, m.volume)
	return true
}

// checkTune makes the tune once, without a sound device, so that a mistake in
// its notes is reported even where nothing can be heard.
func (m *Music) checkTune() {
	if m.tune == nil {
		return
	}
	if !m.checked {
		m.checked = true
		if _, err := m.tune.samples(); err != nil {
			m.err = err
		}
	}
	if m.err != nil {
		reportError(m.err)
	}
}

// describe names the music in messages: its file, or a tune made in code.
func (m *Music) describe() string {
	if m.tune != nil {
		return "the tune made by golib.NewTune"
	}
	return fmt.Sprintf("golib.NewMusic(%q)", m.name)
}

// unload frees the music, so that it is read again if a game runs again.
func (m *Music) unload() {
	if m.loaded {
		device.StopMusic(m.stream)
		device.UnloadMusic(m.stream)
	}
	m.data, m.loaded, m.tracked, m.checked, m.playing, m.paused, m.err = nil, false, false, false, false, false, nil
}
