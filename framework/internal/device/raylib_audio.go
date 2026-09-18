//go:build !js

package device

import (
	"errors"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// OpenAudio starts the sound device and reports whether it is ready. A game
// on a machine with no sound plays on in silence.
func OpenAudio() bool {
	rl.InitAudioDevice()
	return rl.IsAudioDeviceReady()
}

// CloseAudio closes the sound device.
func CloseAudio() {
	rl.CloseAudioDevice()
}

// SetMasterVolume sets how loud everything is, from 0 to 1.
func SetMasterVolume(volume float32) {
	rl.SetMasterVolume(volume)
}

// NewWave reads the samples of a sound file held in memory, or says why it
// couldn't. format is the file's extension, such as ".wav".
func NewWave(format string, data []byte) (Wave, error) {
	// Without the window, raylib would otherwise print a line for each file.
	rl.SetTraceLogLevel(rl.LogWarning)
	wave := rl.LoadWaveFromMemory(format, data, int32(len(data)))
	if !rl.IsWaveValid(wave) {
		return Wave{}, errors.New("raylib could not read it: see the raylib warnings above")
	}
	return wave, nil
}

// UnloadWave frees the samples NewWave read.
func UnloadWave(wave Wave) {
	rl.UnloadWave(wave)
}

// NewSound makes a sound ready to play from a wave, and reports whether it
// is ready. It needs the sound device.
func NewSound(wave Wave) (Sound, bool) {
	sound := rl.LoadSoundFromWave(wave)
	if sound.FrameCount == 0 {
		return Sound{}, false
	}
	return sound, true
}

// NewSoundAlias returns another voice of a sound, which plays the same
// samples on its own, so the sound can overlap itself.
func NewSoundAlias(sound Sound) Sound {
	return rl.LoadSoundAlias(sound)
}

// UnloadSound frees a sound made by NewSound.
func UnloadSound(sound Sound) {
	rl.UnloadSound(sound)
}

// UnloadSoundAlias frees a voice made by NewSoundAlias.
func UnloadSoundAlias(sound Sound) {
	rl.UnloadSoundAlias(sound)
}

// PlaySound plays a sound from its start, over any copy already playing.
func PlaySound(sound Sound) {
	rl.PlaySound(sound)
}

// StopSound silences a sound.
func StopSound(sound Sound) {
	rl.StopSound(sound)
}

// SetSoundVolume sets how loud a sound plays, from 0 to 1.
func SetSoundVolume(sound Sound, volume float32) {
	rl.SetSoundVolume(sound, volume)
}

// SetSoundPitch sets how high a sound plays: 1 as it is, 2 an octave higher.
func SetSoundPitch(sound Sound, pitch float32) {
	rl.SetSoundPitch(sound, pitch)
}

// NewMusic opens a stream of sound from a file held in memory, repeating with
// no gap when looping, or says why it couldn't. format is the file's
// extension, such as ".ogg". The data has to stay reachable while the music
// plays: the backend streams from it.
func NewMusic(format string, data []byte, looping bool) (Music, error) {
	stream := rl.LoadMusicStreamFromMemory(format, data, int32(len(data)))
	if !rl.IsMusicValid(stream) {
		return Music{}, errors.New("raylib could not read it: see the raylib warnings above")
	}
	stream.Looping = looping
	return stream, nil
}

// PlayMusic starts a music from its beginning.
func PlayMusic(music Music) {
	rl.PlayMusicStream(music)
}

// PauseMusic stops a music where it is, for ResumeMusic to carry on.
func PauseMusic(music Music) {
	rl.PauseMusicStream(music)
}

// ResumeMusic carries on a paused music.
func ResumeMusic(music Music) {
	rl.ResumeMusicStream(music)
}

// StopMusic stops a music and goes back to its beginning.
func StopMusic(music Music) {
	rl.StopMusicStream(music)
}

// UpdateMusic gives a playing music the samples it needs for this frame.
// Without it every frame, music stops after a few seconds.
func UpdateMusic(music Music) {
	rl.UpdateMusicStream(music)
}

// SetMusicVolume sets how loud a music plays, from 0 to 1.
func SetMusicVolume(music Music, volume float32) {
	rl.SetMusicVolume(music, volume)
}

// MusicPlaying reports whether a music is playing now.
func MusicPlaying(music Music) bool {
	return rl.IsMusicStreamPlaying(music)
}

// UnloadMusic frees a music.
func UnloadMusic(music Music) {
	rl.UnloadMusicStream(music)
}

// SoundPlaying reports whether a sound is playing now.
func SoundPlaying(sound Sound) bool {
	return rl.IsSoundPlaying(sound)
}
