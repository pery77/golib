package golib

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Tune limits. A tune is made in memory, at soundSampleRate, so a long one
// costs memory: two minutes take about 10 MB.
const (
	tuneMaxSeconds = 120
	tuneMaxVoices  = 8
	tuneBeatGap    = 0.04 // seconds of silence at the end of a note by default, so notes are told apart
)

// TuneSpec is the recipe for music made from notes, without a music file:
// how fast it goes, and the voices that play together.
//
//	var theme = golib.NewTune(golib.TuneSpec{
//		Tempo: 132,
//		Voices: []golib.Voice{
//			{Notes: "c4 e4 g4 e4 f4 a4 c5 a4"},                                  // the melody
//			{Wave: golib.WaveTriangle, Volume: 0.4, Notes: "c2/2 c2/2 f2/2 f2/2"}, // the bass
//		},
//	})
type TuneSpec struct {
	// Tempo is the speed, in beats per minute, from 20 to 400. A beat is
	// what a note without a length lasts, so a tune written in eighth notes
	// at 110 beats per minute has a Tempo of 220. Default: 120.
	Tempo float32

	// Voices are the lines that play together, at most 8. A tune needs one.
	Voices []Voice
}

// Voice is one line of a [TuneSpec]: a sound, and the notes it plays.
type Voice struct {
	// Wave is the voice's sound, as in [SoundSpec]. Default: WaveSquare.
	Wave Waveform

	// Volume is how loud this voice is, from 0 to 1, before the tune's own
	// volume. Default: 0.5.
	Volume float32

	// Duty shapes a square wave, from 0.05 to 0.95, as in [SoundSpec].
	// Default: 0.5.
	Duty float32

	// Vibrato wobbles the pitch by this many Hz, as in [SoundSpec], and
	// VibratoRate says how many times a second. Default: no wobble, 12 times
	// a second.
	Vibrato     float32
	VibratoRate float32

	// Gap is the silence at the end of every note, in seconds, so that two of
	// the same note in a row are heard as two. Default: 0.04. Use 0 for
	// notes that run into each other, and more for a short, clipped sound,
	// such as a drum: a note is never shorter than half its length.
	Gap float32

	// Notes are the notes to play, separated by spaces, each of them a letter
	// from a to g, an optional # or b, and the octave, such as "c4", "f#3" or
	// "eb5"; capital letters work too. Middle C is c4, and octaves go from 0
	// to 8. A dot is a silence, and a dash holds the note before it for
	// another beat. "/" and a number give a length in beats: "c4/2" lasts two
	// beats and "c4/0.5" half a beat.
	//
	//	"c4 e4 g4 c5 - - . g4/0.5 e4/0.5 c4/2"
	//
	// Every voice of a tune should add up to the same number of beats, or the
	// short ones end in silence and the loop falls out of step. Counting the
	// beats of each voice is worth a test in the game.
	Notes string
}

// NewTune returns music made from the notes in spec, for a game that has no
// music file. It plays like music read from a file, and loops:
//
//	theme.Play()  // in Update, such as when the play scene starts
//	theme.Pause() // while the game is paused
//
// The tune is made the first time it plays, so a game can create it before
// Run opens the window, as a package variable. Making it takes a moment, a
// few tens of milliseconds for a tune of half a minute, so start it on a
// title screen rather than in the middle of the action. A mistake in the
// notes, or a tune longer than two minutes, stops Run with a message, and
// [Music.Err] returns it, in tests too:
//
//	var themeSpec = golib.TuneSpec{...} // keep the recipe to test it
//	var theme = golib.NewTune(themeSpec)
func NewTune(spec TuneSpec) *Music {
	tune := spec
	return &Music{name: "the tune", tune: &tune, volume: 1}
}

// samples renders the tune: every voice's notes, mixed together.
func (spec TuneSpec) samples() ([]int16, error) {
	tempo := spec.Tempo
	if tempo == 0 {
		tempo = 120
	}
	if tempo < 20 || tempo > 400 {
		return nil, fmt.Errorf("golib.NewTune: the tempo is %g beats per minute: use 20 to 400", tempo)
	}
	if len(spec.Voices) == 0 {
		return nil, fmt.Errorf("golib.NewTune: the tune has no voices: give it at least one, with its notes")
	}
	if len(spec.Voices) > tuneMaxVoices {
		return nil, fmt.Errorf("golib.NewTune: the tune has %d voices: use at most %d", len(spec.Voices), tuneMaxVoices)
	}
	beat := 60 / float64(tempo)

	var mixed []int16
	for i, voice := range spec.Voices {
		notes, err := parseNotes(voice.Notes)
		if err != nil {
			return nil, fmt.Errorf("golib.NewTune: voice %d: %w", i+1, err)
		}
		if len(notes) == 0 {
			return nil, fmt.Errorf("golib.NewTune: voice %d has no notes", i+1)
		}
		volume := voice.Volume
		if volume == 0 {
			volume = 0.5
		}
		line, err := voice.render(notes, beat, volume)
		if err != nil {
			return nil, fmt.Errorf("golib.NewTune: voice %d: %w", i+1, err)
		}
		if len(line) > len(mixed) {
			// The longest voice sets the tune's length; shorter ones end in
			// silence.
			mixed = append(mixed, make([]int16, len(line)-len(mixed))...)
		}
		for j, sample := range line {
			mixed[j] = clipSample(int(mixed[j]) + int(sample))
		}
	}
	return mixed, nil
}

// render returns the samples of one voice: each note in turn, made by the
// same synthesizer as the sound effects.
func (v Voice) render(notes []note, beat float64, volume float32) ([]int16, error) {
	var samples []int16
	for _, n := range notes {
		length := n.beats * beat
		if float64(len(samples))/soundSampleRate+length > tuneMaxSeconds {
			return nil, fmt.Errorf("the tune is longer than %d seconds: make it shorter, and let it loop", tuneMaxSeconds)
		}
		count := int(length * soundSampleRate)
		if n.frequency == 0 { // a silence
			samples = append(samples, make([]int16, count)...)
			continue
		}
		// The note stops a little before the next one starts, so that two of
		// the same note in a row are heard as two.
		gap := float64(tuneBeatGap)
		if v.Gap != 0 {
			gap = float64(max(0, v.Gap))
		}
		sound := min(length, max(length-gap, length/2))
		spec := SoundSpec{
			Wave:        v.Wave,
			Frequency:   float32(n.frequency),
			Duration:    float32(sound),
			Attack:      0.005,
			Release:     float32(min(0.05, sound/2)),
			Volume:      volume,
			Duty:        v.Duty,
			Vibrato:     v.Vibrato,
			VibratoRate: v.VibratoRate,
		}
		played := spec.samples()
		if len(played) > count {
			played = played[:count]
		}
		samples = append(samples, played...)
		samples = append(samples, make([]int16, count-len(played))...)
	}
	return samples, nil
}

// note is one note of a voice: how many beats it lasts, and the pitch it
// sounds at, or 0 for a silence.
type note struct {
	frequency float64
	beats     float64
}

// noteSteps gives each note letter its place in an octave, in semitones from
// C.
var noteSteps = map[byte]int{'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11}

// parseNotes turns the notes of a voice into pitches and lengths.
func parseNotes(notes string) ([]note, error) {
	var parsed []note
	for _, item := range strings.Fields(notes) {
		text, beats := item, 1.0
		if name, length, found := strings.Cut(item, "/"); found {
			value, err := strconv.ParseFloat(length, 64)
			if err != nil || value <= 0 || value > 64 {
				return nil, fmt.Errorf("%q: after / write how many beats the note lasts, such as %q or %q", item, name+"/2", name+"/0.5")
			}
			text, beats = name, value
		}
		if text == "-" {
			if len(parsed) == 0 {
				return nil, fmt.Errorf("%q: a dash holds the note before it, so it can't come first", item)
			}
			parsed[len(parsed)-1].beats += beats
			continue
		}
		if text == "." {
			parsed = append(parsed, note{beats: beats})
			continue
		}
		frequency, err := noteFrequency(text)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", item, err)
		}
		parsed = append(parsed, note{frequency: frequency, beats: beats})
	}
	return parsed, nil
}

// noteFrequency returns the pitch of a note such as "c4", "f#3" or "eb5", in
// Hz, with a4 at 440 Hz.
func noteFrequency(text string) (float64, error) {
	invalid := fmt.Errorf(`write a note as a letter from a to g, an optional # or b, and its octave, such as "c4", "f#3" or "eb5", "." for a silence, or "-" to hold the note before it`)
	if len(text) < 2 {
		return 0, invalid

	}
	step, found := noteSteps[text[0]|0x20] // lowercase
	if !found {
		return 0, invalid
	}
	rest := text[1:]
	switch rest[0] {
	case '#':
		step++
		rest = rest[1:]
	case 'b', 'B':
		step--
		rest = rest[1:]
	}
	octave, err := strconv.Atoi(rest)
	if err != nil || octave < 0 || octave > 8 {
		return 0, invalid
	}
	// a4 is 440 Hz, and an octave doubles the pitch. Steps count from c0.
	semitones := step + 12*octave - (noteSteps['a'] + 12*4)
	return 440 * math.Pow(2, float64(semitones)/12), nil
}

// clipSample keeps a mixed sample within what a 16-bit sample holds.
func clipSample(value int) int16 {
	return int16(max(math.MinInt16, min(value, math.MaxInt16)))
}
