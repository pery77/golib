//go:build js

package device

// Sound is stage 2 of the web build (see docs/roadmap.md). Until then a web
// build plays in silence: OpenAudio reports no sound device, and package
// golib takes the same path it takes under golib shot and in tests, where a
// game plays on without being heard.
//
// The one thing that still happens is reading a sound file, which package
// golib does even without a device so that a missing file is reported. Here
// that read stops at the file's bytes: the game's own checks still run, but a
// damaged .wav or .ogg isn't caught until Web Audio decodes it in stage 2.

// OpenAudio reports that there is no sound device yet.
func OpenAudio() bool {
	return false
}

// CloseAudio has nothing to close.
func CloseAudio() {}

// SetMasterVolume has nothing to make louder yet.
func SetMasterVolume(volume float32) {}

// NewWave accepts the bytes of a sound file without decoding them, so that a
// game with sounds runs in silence instead of stopping.
func NewWave(format string, data []byte) (Wave, bool) {
	return Wave{}, true
}

// UnloadWave has nothing to free.
func UnloadWave(wave Wave) {}

// NewSound reports that no sound can be made without a sound device. Package
// golib only calls it once OpenAudio has said yes.
func NewSound(wave Wave) (Sound, bool) {
	return Sound{}, false
}

// NewSoundAlias has no sound to copy.
func NewSoundAlias(sound Sound) Sound {
	return Sound{}
}

// UnloadSound has nothing to free.
func UnloadSound(sound Sound) {}

// UnloadSoundAlias has nothing to free.
func UnloadSoundAlias(sound Sound) {}

// PlaySound is silent.
func PlaySound(sound Sound) {}

// StopSound is silent.
func StopSound(sound Sound) {}

// SetSoundVolume is silent.
func SetSoundVolume(sound Sound, volume float32) {}

// SetSoundPitch is silent.
func SetSoundPitch(sound Sound, pitch float32) {}

// SoundPlaying is never playing.
func SoundPlaying(sound Sound) bool {
	return false
}

// NewMusic reports that no music can be opened without a sound device.
// Package golib only calls it once OpenAudio has said yes.
func NewMusic(format string, data []byte, looping bool) (Music, bool) {
	return Music{}, false
}

// PlayMusic is silent.
func PlayMusic(music Music) {}

// PauseMusic is silent.
func PauseMusic(music Music) {}

// ResumeMusic is silent.
func ResumeMusic(music Music) {}

// StopMusic is silent.
func StopMusic(music Music) {}

// UpdateMusic is silent.
func UpdateMusic(music Music) {}

// SetMusicVolume is silent.
func SetMusicVolume(music Music, volume float32) {}

// MusicPlaying is never playing.
func MusicPlaying(music Music) bool {
	return false
}

// UnloadMusic has nothing to free.
func UnloadMusic(music Music) {}
