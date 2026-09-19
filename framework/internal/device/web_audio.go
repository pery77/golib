//go:build js

package device

import (
	"fmt"
	"slices"
	"strings"
	"syscall/js"
)

// Sound on the web goes through Web Audio. The synthesizers above this
// package already turn a SoundSpec, a .jfxr file or a tune into the bytes of
// a WAV file, and a sound file is read as it is, so all this has to do is
// hand those bytes to the browser and play what comes back.
//
// The browser decodes them asynchronously, and the contract is synchronous,
// so decode blocks the game until the browser answers. That is allowed here:
// when every goroutine is waiting, Go hands the thread back to the page,
// which then finishes the decoding and wakes the game. A sound is decoded
// once, the first time it plays, so a game that makes its sounds before Run
// or plays each one early never waits during play.
//
// Browsers refuse to make a sound until the player has pressed a key or
// clicked. web.js starts the sound device at the first of either, so a game
// that plays a sound on its title screen is heard from the first press.

// OpenAudio starts the sound device and reports whether it is ready.
func OpenAudio() bool {
	return js_().Call("openAudio").Bool()
}

// CloseAudio closes the sound device.
func CloseAudio() {
	js_().Call("closeAudio")
}

// SetMasterVolume sets how loud everything is, from 0 to 1.
func SetMasterVolume(volume float32) {
	js_().Call("setMasterVolume", float64(volume))
}

// browserFormats are the sound files every browser decodes, in the order to
// prefer them: .ogg is the smallest for a long piece of music. The rest of
// what GoLib reads on the desktop, .qoa for sounds and .xm and .mod for
// music, browsers have never heard of.
var browserFormats = []string{".ogg", ".mp3", ".wav"}

// PlayableFormats returns the sound file formats this machine decodes, in
// the order to prefer them when there is a choice. Package golib plays a
// file beside the one a game asked for when that one's format is missing
// here, so a game whose music is .xm still plays its .ogg in a browser.
func PlayableFormats() []string {
	return slices.Clone(browserFormats)
}

// NewWave decodes the samples of a sound file held in memory, or says why the
// browser couldn't. format is the file's extension, such as ".wav". It waits
// for the browser to finish decoding.
func NewWave(format string, data []byte) (Wave, error) {
	if err := decodable(format); err != nil {
		return Wave{}, err
	}
	id := decodeSound(data)
	if id < 0 {
		return Wave{}, fmt.Errorf("the browser could not decode this %s file", format)
	}
	return Wave{ID: uint32(id)}, nil
}

// decodable says whether a browser reads this kind of file, and what to do
// when it doesn't.
func decodable(format string) error {
	if slices.Contains(browserFormats, format) {
		return nil
	}
	return fmt.Errorf("a browser cannot play %s files: save the sound as %s, or play the game on the desktop",
		format, strings.Join(browserFormats, ", "))
}

// decodeSound hands the bytes of a sound file to the browser and waits for
// the decoded sound, whose number it returns, or -1 when the browser could
// not read it.
func decodeSound(data []byte) int {
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	done := make(chan int, 1)
	var answer js.Func
	answer = js.FuncOf(func(_ js.Value, args []js.Value) any {
		answer.Release()
		done <- args[0].Int()
		return nil
	})
	js_().Call("decodeSound", bytes, answer)
	return <-done
}

// UnloadWave frees the decoded samples.
func UnloadWave(wave Wave) {
	js_().Call("unloadWave", int(wave.ID))
}

// NewSound makes a sound ready to play from a wave, and reports whether it
// is ready.
func NewSound(wave Wave) (Sound, bool) {
	made := js_().Call("newSound", int(wave.ID))
	id := made.Index(0).Int()
	if id < 0 {
		return Sound{}, false
	}
	return Sound{ID: uint32(id), FrameCount: uint32(made.Index(1).Int())}, true
}

// NewSoundAlias returns another voice of a sound, which plays the same
// samples on its own, so the sound can overlap itself.
func NewSoundAlias(sound Sound) Sound {
	made := js_().Call("newSoundAlias", int(sound.ID))
	return Sound{ID: uint32(made.Index(0).Int()), FrameCount: uint32(made.Index(1).Int())}
}

// UnloadSound frees a sound made by NewSound.
func UnloadSound(sound Sound) {
	js_().Call("unloadSound", int(sound.ID))
}

// UnloadSoundAlias frees a voice made by NewSoundAlias.
func UnloadSoundAlias(sound Sound) {
	js_().Call("unloadSound", int(sound.ID))
}

// PlaySound plays a sound from its start, over any copy already playing.
func PlaySound(sound Sound) {
	js_().Call("playSound", int(sound.ID))
}

// StopSound silences a sound.
func StopSound(sound Sound) {
	js_().Call("stopSound", int(sound.ID))
}

// SetSoundVolume sets how loud a sound plays, from 0 to 1.
func SetSoundVolume(sound Sound, volume float32) {
	js_().Call("setSoundVolume", int(sound.ID), float64(volume))
}

// SetSoundPitch sets how high a sound plays: 1 as it is, 2 an octave higher
// and half as long.
func SetSoundPitch(sound Sound, pitch float32) {
	js_().Call("setSoundPitch", int(sound.ID), float64(pitch))
}

// SoundPlaying reports whether a sound is playing now.
func SoundPlaying(sound Sound) bool {
	return js_().Call("soundPlaying", int(sound.ID)).Bool()
}

// NewMusic opens a stream of sound from a file held in memory, repeating
// with no gap when looping, or says why the browser couldn't read it.
// Package golib loops a sound this way as well as playing music.
func NewMusic(format string, data []byte, looping bool) (Music, error) {
	if err := decodable(format); err != nil {
		return Music{}, err
	}
	id := decodeSound(data)
	if id < 0 {
		return Music{}, fmt.Errorf("the browser could not decode this %s file", format)
	}
	made := js_().Call("newMusic", id, looping).Int()
	if made < 0 {
		return Music{}, fmt.Errorf("the browser could not hold this %s file to play it", format)
	}
	return Music{ID: uint32(made)}, nil
}

// PlayMusic starts a music from its beginning.
func PlayMusic(music Music) {
	js_().Call("playMusic", int(music.ID))
}

// PauseMusic stops a music where it is, for ResumeMusic to carry on.
func PauseMusic(music Music) {
	js_().Call("pauseMusic", int(music.ID))
}

// ResumeMusic carries on a paused music.
func ResumeMusic(music Music) {
	js_().Call("resumeMusic", int(music.ID))
}

// StopMusic stops a music and goes back to its beginning.
func StopMusic(music Music) {
	js_().Call("stopMusic", int(music.ID))
}

// UpdateMusic has nothing to do: the browser plays a whole sound by itself,
// where raylib has to be fed its samples every frame.
func UpdateMusic(music Music) {}

// SetMusicVolume sets how loud a music plays, from 0 to 1.
func SetMusicVolume(music Music, volume float32) {
	js_().Call("setMusicVolume", int(music.ID), float64(volume))
}

// MusicPlaying reports whether a music is playing now.
func MusicPlaying(music Music) bool {
	return js_().Call("musicPlaying", int(music.ID)).Bool()
}

// UnloadMusic frees a music.
func UnloadMusic(music Music) {
	js_().Call("unloadMusic", int(music.ID))
}
