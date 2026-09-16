package golib

import (
	"encoding/binary"
	"fmt"
	"math"
	"path"
	"slices"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Waveform is the shape of a sound.
type Waveform int

// Waveforms, from the harshest to the softest.
const (
	WaveSquare   Waveform = iota // buzzy, like an 8-bit console
	WaveSaw                      // harsh and bright
	WaveTriangle                 // softer, a bit like a flute
	WaveSine                     // pure and smooth
	WaveNoise                    // hiss, for explosions and hits
)

const (
	soundSampleRate = 44100 // samples per second
	soundVoices     = 4     // copies of one sound that can play at the same time
)

// SoundSpec is the recipe for a sound effect: GoLib makes the sound from it, so
// games need no sound files. Fields left at their zero value get the default in
// their comment, so SoundSpec{} is a short beep.
//
//	shot := golib.NewSound(golib.SoundSpec{
//		Wave:      golib.WaveSquare,
//		Frequency: 1000,
//		Slide:     -4000, // the pitch falls: pew
//		Duration:  0.12,
//		Duty:      0.2,
//	})
//
// [Laser], [Explosion], [Pickup], [Jump], [Hurt] and [PowerUp] are ready-made
// recipes to start from.
type SoundSpec struct {
	Wave        Waveform // Shape of the sound. Default: WaveSquare.
	Frequency   float32  // Hz the sound starts at. Default: 440.
	Slide       float32  // Hz added every second; negative falls. Default: 0.
	Duration    float32  // Seconds the sound lasts. Default: 0.25.
	Attack      float32  // Seconds fading in, so the start doesn't click. Default: 0.005.
	Release     float32  // Seconds fading out at the end. Default: 0.05.
	Volume      float32  // Loudness, from 0 to 1. Default: 0.5.
	Duty        float32  // Part of each square wave that is high, from 0 to 1: 0.5 is even, 0.2 thin and nasal. Default: 0.5.
	Vibrato     float32  // Hz the pitch wobbles up and down. Default: 0, no wobble.
	VibratoRate float32  // Wobbles per second. Default: 12.
}

// resolve returns the recipe with defaults applied and values kept in range.
func (spec SoundSpec) resolve() SoundSpec {
	if spec.Frequency <= 0 {
		spec.Frequency = 440
	}
	if spec.Duration <= 0 {
		spec.Duration = 0.25
	}
	if spec.Attack == 0 {
		spec.Attack = 0.005
	}
	if spec.Release == 0 {
		spec.Release = 0.05
	}
	if spec.Volume == 0 {
		spec.Volume = 0.5
	}
	if spec.Duty == 0 {
		spec.Duty = 0.5
	}
	if spec.VibratoRate == 0 {
		spec.VibratoRate = 12
	}
	spec.Duration = min(spec.Duration, 10)
	spec.Volume = max(0, min(spec.Volume, 1))
	spec.Duty = max(0.05, min(spec.Duty, 0.95))
	spec.Attack = max(0, min(spec.Attack, spec.Duration))
	spec.Release = max(0, min(spec.Release, spec.Duration))
	return spec
}

// samples renders the sound as 16-bit samples, one channel, at
// soundSampleRate. The same recipe always gives the same samples, hiss
// included.
func (spec SoundSpec) samples() []int16 {
	spec = spec.resolve()
	samples := make([]int16, int(spec.Duration*soundSampleRate))
	step := 1 / float64(soundSampleRate)
	phase := 0.0
	noise := 0.0
	noiseState := uint32(0x2545f491) // a fixed seed keeps the hiss the same every time
	for i := range samples {
		seconds := float64(i) * step

		frequency := float64(spec.Frequency) + float64(spec.Slide)*seconds
		if spec.Vibrato != 0 {
			frequency += float64(spec.Vibrato) * math.Sin(2*math.Pi*float64(spec.VibratoRate)*seconds)
		}
		frequency = max(20, min(frequency, soundSampleRate/2))

		phase += frequency * step
		for phase >= 1 {
			phase--
			// Noise holds one random value per wave, so its hiss follows the frequency.
			noiseState = noiseState*1664525 + 1013904223
			noise = float64(noiseState>>8)/float64(1<<24)*2 - 1
		}

		samples[i] = int16(spec.shape(phase, noise) * spec.envelope(seconds) * float64(spec.Volume) * math.MaxInt16)
	}
	return samples
}

// shape returns the wave's value, from -1 to 1, at a point in its cycle.
func (spec SoundSpec) shape(phase, noise float64) float64 {
	switch spec.Wave {
	case WaveSaw:
		return 2*phase - 1
	case WaveTriangle:
		return 4*math.Abs(phase-0.5) - 1
	case WaveSine:
		return math.Sin(2 * math.Pi * phase)
	case WaveNoise:
		return noise
	default: // WaveSquare
		if phase < float64(spec.Duty) {
			return 1
		}
		return -1
	}
}

// envelope returns the loudness, from 0 to 1, at a moment of the sound: it
// fades in over Attack and out over Release.
func (spec SoundSpec) envelope(seconds float64) float64 {
	loudness := 1.0
	if spec.Attack > 0 && seconds < float64(spec.Attack) {
		loudness = seconds / float64(spec.Attack)
	}
	if left := float64(spec.Duration) - seconds; spec.Release > 0 && left < float64(spec.Release) {
		loudness = min(loudness, max(0, left)/float64(spec.Release))
	}
	return loudness
}

// wav wraps samples in a WAV file, which is what raylib reads from memory.
func wav(samples []int16) []byte {
	const headerSize = 44
	data := make([]byte, 0, headerSize+2*len(samples))
	putText := func(text string) { data = append(data, text...) }
	putUint32 := func(value uint32) { data = binary.LittleEndian.AppendUint32(data, value) }
	putUint16 := func(value uint16) { data = binary.LittleEndian.AppendUint16(data, value) }

	putText("RIFF")
	putUint32(uint32(headerSize - 8 + 2*len(samples)))
	putText("WAVE")
	putText("fmt ")
	putUint32(16)                  // the size of this block
	putUint16(1)                   // uncompressed samples
	putUint16(1)                   // one channel
	putUint32(soundSampleRate)     // samples per second
	putUint32(soundSampleRate * 2) // bytes per second
	putUint16(2)                   // bytes per sample, all channels
	putUint16(16)                  // bits per sample
	putText("data")
	putUint32(uint32(2 * len(samples)))
	for _, sample := range samples {
		putUint16(uint16(sample))
	}
	return data
}

// soundFormats are the file types raylib reads sound effects from, by
// extension.
var soundFormats = []string{".wav", ".ogg", ".mp3", ".qoa"}

// Sound is a sound effect, ready to play: made in code with NewSound or one of
// the ready-made recipes, or read from a file with NewSoundFile. Keep it in
// the game's state, and play it as often as needed.
type Sound struct {
	spec   SoundSpec
	name   string // the file NewSoundFile was given; "" for a sound made in code
	volume float32
	read   bool // the file has been read, successfully or not
	err    error
	voices []rl.Sound // the same sound several times over, so it can overlap itself
	next   int
}

// NewSound returns the sound effect spec describes. It is made the first time
// it plays, so games can create their sounds before Run opens the window.
func NewSound(spec SoundSpec) *Sound {
	return &Sound{spec: spec.resolve(), volume: 1}
}

// NewSoundFile returns the sound effect in the game's assets folder named
// name, which is relative to that folder and uses forward slashes, as in
// [ReadAsset]. The file is a .wav, .ogg, .mp3 or .qoa file, which the sound
// keeps whole in memory: use [NewMusic] for long tracks.
//
//	var coin = golib.NewSoundFile("sounds/coin.wav") // games/<game>/assets/sounds/coin.wav
//
// The file is read the first time the sound plays, even when there is no
// sound device, so a file that is missing or can't be read stops Run with an
// error under golib shot too.
func NewSoundFile(name string) *Sound {
	return &Sound{name: name, volume: 1}
}

// Play plays the sound, over any copy of it that is already playing. Up to
// four copies sound at once; the fifth replaces the oldest.
//
// Nothing is heard when there is no sound device, so screenshots from golib
// shot stay silent.
func (s *Sound) Play() {
	if !s.load() {
		return
	}
	rl.PlaySound(s.voices[s.next])
	s.next = (s.next + 1) % len(s.voices)
}

// SetVolume sets how loud this sound is, from 0 (silent) to 1 (full), under
// the volume [SetVolume] sets for everything. Use it to balance sound files
// that are louder than the rest. It works before the sound plays.
func (s *Sound) SetVolume(volume float32) {
	s.volume = max(0, min(volume, 1))
	for _, voice := range s.voices {
		rl.SetSoundVolume(voice, s.volume)
	}
}

// load makes or reads the sound the first time it plays, and reports whether
// it is ready to play, which needs the sound device. Without one, a sound
// file is still read once, so that a mistake in it shows. A mistake is
// reported to Run every time the sound plays.
func (s *Sound) load() bool {
	if len(s.voices) > 0 {
		return true
	}
	if s.err != nil {
		reportError(s.err)
		return false
	}
	ready := audio.isReady()
	if !ready && (s.name == "" || s.read) {
		return false
	}
	s.read = true
	wave, err := s.wave()
	if err != nil {
		s.err = err
		reportError(err)
		return false
	}
	defer rl.UnloadWave(wave)
	if !ready {
		return false
	}
	first := rl.LoadSoundFromWave(wave)
	if first.FrameCount == 0 {
		return false
	}
	s.voices = append(s.voices, first)
	for range soundVoices - 1 {
		s.voices = append(s.voices, rl.LoadSoundAlias(first))
	}
	for _, voice := range s.voices {
		rl.SetSoundVolume(voice, s.volume)
	}
	audio.track(s)
	return true
}

// wave returns the sound's samples, made from its spec or read from its file.
func (s *Sound) wave() (rl.Wave, error) {
	if s.name == "" {
		file := wav(s.spec.samples())
		return rl.LoadWaveFromMemory(".wav", file, int32(len(file))), nil
	}
	format := strings.ToLower(path.Ext(s.name))
	if !slices.Contains(soundFormats, format) {
		return rl.Wave{}, fmt.Errorf("golib.NewSoundFile(%q): GoLib plays sound effects from %s files, not %q ones: convert the sound to one of those", s.name, strings.Join(soundFormats, ", "), format)
	}
	data, err := ReadAsset(s.name)
	if err != nil {
		return rl.Wave{}, fmt.Errorf("golib.NewSoundFile(%q): %w", s.name, err)
	}
	// Without the window, raylib would otherwise print a line for each file.
	rl.SetTraceLogLevel(rl.LogWarning)
	wave := rl.LoadWaveFromMemory(format, data, int32(len(data)))
	if !rl.IsWaveValid(wave) {
		return rl.Wave{}, fmt.Errorf("golib.NewSoundFile(%q): raylib could not read the sound: see the raylib warnings above", s.name)
	}
	return wave, nil
}

// unload frees the sound, so that it is made or read again if a game runs
// again.
func (s *Sound) unload() {
	for i, voice := range s.voices {
		if i == 0 {
			rl.UnloadSound(voice)
		} else {
			rl.UnloadSoundAlias(voice)
		}
	}
	s.voices, s.next, s.read, s.err = nil, 0, false, nil
}

// audioDevice is the sound device Run opens while a game plays. golib shot
// leaves it closed, so screenshots are silent and fast.
type audioDevice struct {
	sync.Mutex
	ready  bool
	volume float32
	sounds []*Sound // made while the device was open
	music  []*Music // played while the device was open
}

var audio = audioDevice{volume: 1}

// SetVolume sets how loud every sound is, from 0 (silent) to 1 (full). It
// works before Run too, so a game can start quiet.
func SetVolume(volume float32) {
	volume = max(0, min(volume, 1))
	audio.Lock()
	defer audio.Unlock()
	audio.volume = volume
	if audio.ready {
		rl.SetMasterVolume(volume)
	}
}

// open starts the sound device. A game that plays nothing doesn't notice it.
func (a *audioDevice) open() {
	rl.InitAudioDevice()
	a.Lock()
	defer a.Unlock()
	a.ready = rl.IsAudioDeviceReady()
	if a.ready {
		rl.SetMasterVolume(a.volume)
	}
}

// close frees every sound and music that was made and closes the device.
func (a *audioDevice) close() {
	a.Lock()
	sounds, music, ready := a.sounds, a.music, a.ready
	a.sounds, a.music, a.ready = nil, nil, false
	a.Unlock()
	for _, sound := range sounds {
		sound.unload()
	}
	for _, track := range music {
		track.unload()
	}
	if ready {
		rl.CloseAudioDevice()
	}
}

func (a *audioDevice) isReady() bool {
	a.Lock()
	defer a.Unlock()
	return a.ready
}

// track remembers a sound, so close can free it.
func (a *audioDevice) track(sound *Sound) {
	a.Lock()
	defer a.Unlock()
	a.sounds = append(a.sounds, sound)
}

// trackMusic remembers a music the first time it is played, so that Run keeps
// it fed and reports what went wrong if it can't be read.
func (a *audioDevice) trackMusic(music *Music) {
	if music.tracked {
		return
	}
	music.tracked = true
	a.Lock()
	defer a.Unlock()
	a.music = append(a.music, music)
}

// updateMusic gives every playing music the samples it needs for this frame,
// and reports the first music that could not be read. Run calls it once a
// frame: without it, music stops after a few seconds.
func (a *audioDevice) updateMusic() error {
	a.Lock()
	music := a.music
	a.Unlock()
	for _, track := range music {
		if track.err != nil {
			return track.err
		}
		if track.Playing() {
			rl.UpdateMusicStream(track.stream)
		}
	}
	return nil
}

// Laser returns a falling zap, for shots.
func Laser() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveSquare, Frequency: 1040, Slide: -5200, Duration: 0.14,
		Attack: 0.001, Release: 0.1, Volume: 0.35, Duty: 0.18, Vibrato: 18, VibratoRate: 35,
	})
}

// Explosion returns a low burst of noise, for things breaking apart.
func Explosion() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveNoise, Frequency: 950, Slide: -620, Duration: 0.5,
		Attack: 0.001, Release: 0.45, Volume: 0.5, Vibrato: 8, VibratoRate: 9,
	})
}

// Pickup returns a bright blip that rises, for coins and collecting.
func Pickup() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveSquare, Frequency: 1320, Slide: 900, Duration: 0.12,
		Attack: 0.001, Release: 0.08, Volume: 0.3,
	})
}

// Jump returns a soft rising note, for jumps.
func Jump() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveTriangle, Frequency: 220, Slide: 760, Duration: 0.16,
		Attack: 0.001, Release: 0.1, Volume: 0.4, Vibrato: 8, VibratoRate: 18,
	})
}

// Hurt returns a harsh falling note, for taking damage.
func Hurt() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveSaw, Frequency: 420, Slide: -900, Duration: 0.25,
		Attack: 0.001, Release: 0.2, Volume: 0.45,
	})
}

// PowerUp returns a rising fanfare, for upgrades and extra lives.
func PowerUp() *Sound {
	return NewSound(SoundSpec{
		Wave: WaveSquare, Frequency: 440, Slide: 540, Duration: 0.35,
		Attack: 0.002, Release: 0.12, Volume: 0.35, Duty: 0.25,
	})
}
