package main

import (
	"fmt"
	"math"
	"strings"

	"golib"
)

// The background music. GoLib plays music only from files (golib.NewMusic),
// and this game has none, so it plays a tune written here, one note at a time:
// every note is a short sound made by golib.NewSound, and a jukebox starts
// each on its step. Sounds can't loop or be stopped, so turning the music off
// lets the notes already playing ring out.
//
// To play a music file instead, such as assets/music/theme.ogg: in session.go,
// make it once with golib.NewMusic("music/theme.ogg"), and in session.update
// call its Play when the music is on and its Pause when it's off, in place of
// s.music.update. Write where the file came from, and its license, in
// assets/ATTRIBUTION.md.

// Tuning for the tune.
const (
	tuneStep   = 2.0 / 15 // seconds per step, a sixteenth note: 112.5 beats per minute
	leadVolume = 0.09     // from 0 to 1, under the sound effects
	bassVolume = 0.13
	tickVolume = 0.03
)

// tune is the music: an original melody in A minor over a bass, with a soft
// tick on the off-beats. It loops after 16 bars, about 34 seconds.
var tune = []voice{
	{wave: golib.WaveSquare, duty: 0.3, volume: leadVolume, score: `
		E5 - - - A4 - C5 - E5 - D5 - C5 - - -
		A4 - - - F4 - A4 - C5 - B4 - A4 - - -
		G4 - - - C5 - E5 - G5 - F5 - E5 - D5 -
		D5 - - - B4 - - - G4 - - - . . . .
		E5 - - - A4 - C5 - E5 - F5 - E5 - D5 -
		C5 - - - A4 - C5 - F5 - E5 - C5 - - -
		D5 - - - B4 - D5 - G5 - F5 - D5 - B4 -
		A4 - - - - - - - . . . . . . . .
		. . A4 - C5 - A4 - . . C5 - F5 - E5 -
		D5 - - - . . B4 - D5 - G4 - - - . .
		E5 - - - B4 - - - G4 - B4 - E5 - D5 -
		C5 - - - A4 - - - E4 - - - . . . .
		F4 - A4 - C5 - F5 - E5 - - - C5 - - -
		D5 - - - G4 - B4 - D5 - - - G5 - - -
		G#4 - - - B4 - - - E5 - - - D5 - - -
		B4 - - - G#4 - - - E4 - - - . . . .`},
	{wave: golib.WaveTriangle, volume: bassVolume, score: bassLine(
		"A2", "F2", "C3", "G2", "A2", "F2", "G2", "A2",
		"F2", "G2", "E2", "A2", "F2", "G2", "E2", "E2",
	)},
	{wave: golib.WaveNoise, volume: tickVolume, score: strings.Repeat(". . x . . . x . ", 32)},
}

// bassLine returns a bass part with one bar for each root note: the root and
// the note an octave above it, in turns, on every other step.
func bassLine(roots ...string) string {
	var bars []string
	for _, root := range roots {
		octave := root[:len(root)-1] + string(root[len(root)-1]+1)
		bars = append(bars, strings.Repeat(root+" - "+octave+" - ", 4))
	}
	return strings.Join(bars, "\n")
}

// voice is one part of the tune, and the sound its notes make.
type voice struct {
	wave   golib.Waveform
	duty   float32 // square waves only
	volume float32
	// score is the part, one step per word, separated by spaces or lines: a
	// note name, such as A4, C#5 or Bb3, starts a note; - holds the note one
	// more step; . is a rest; x is a tick, for a noise voice.
	score string
}

// Ticks are short, whatever the score says.
const (
	tickLength    = 0.03 // seconds
	tickFrequency = 7000 // Hz; for noise, how fine the hiss is
)

// jukebox plays a tune: it keeps time, and starts the notes of each step.
type jukebox struct {
	steps [][]*golib.Sound // the notes each step starts
	next  int              // the step to play next
	wait  float32          // seconds until then
}

// newJukebox reads the voices and makes the sound of every note in them, once.
func newJukebox(voices []voice) (*jukebox, error) {
	j := &jukebox{}
	sounds := map[golib.SoundSpec]*golib.Sound{}
	for v, part := range voices {
		words := strings.Fields(part.score)
		if v == 0 {
			j.steps = make([][]*golib.Sound, len(words))
		}
		if len(words) != len(j.steps) || len(words)%16 != 0 {
			return nil, fmt.Errorf("music: voice %d has %d steps; want whole bars of 16, as many as voice 0's %d", v+1, len(words), len(j.steps))
		}
		for step, word := range words {
			if word == "-" || word == "." {
				continue
			}
			spec := golib.SoundSpec{Wave: part.wave, Duty: part.duty, Volume: part.volume, Attack: 0.004}
			if word == "x" {
				spec.Frequency, spec.Duration, spec.Release = tickFrequency, tickLength, tickLength*0.8
			} else {
				frequency, err := noteFrequency(word)
				if err != nil {
					return nil, fmt.Errorf("music: voice %d, step %d: %w", v+1, step+1, err)
				}
				held := 1
				for step+held < len(words) && words[step+held] == "-" {
					held++
				}
				// A little silence before the next note keeps repeated notes apart.
				spec.Frequency = frequency
				spec.Duration = float32(held)*tuneStep - 0.02
				spec.Release = min(0.08, spec.Duration/2)
				if held >= 4 && part.wave == golib.WaveSquare {
					spec.Vibrato, spec.VibratoRate = frequency*0.006, 5.5 // long notes waver a little
				}
			}
			if sounds[spec] == nil {
				sounds[spec] = golib.NewSound(spec)
			}
			j.steps[step] = append(j.steps[step], sounds[spec])
		}
	}
	if len(j.steps) == 0 {
		return nil, fmt.Errorf("music: the tune has no steps")
	}
	return j, nil
}

// update keeps the tune going: it starts the notes of each step when its
// time comes. Every scene calls it once per update, through session.update.
// While the music is off the tune waits, and carries on from there.
func (j *jukebox) update(on bool, dt float32) {
	if !on {
		return
	}
	for j.wait < 0.001 { // not 0: adding up sixtieths of a second isn't exact
		for _, note := range j.steps[j.next] {
			note.Play()
		}
		j.next = (j.next + 1) % len(j.steps)
		j.wait += tuneStep
	}
	j.wait -= dt
}

// semitones are the note letters' places in an octave, from C.
var semitones = map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}

// noteFrequency returns the pitch of a note name such as A4, C#5 or Bb3, in
// Hz. A4 is 440 Hz; octaves start at C.
func noteFrequency(name string) (float32, error) {
	bad := fmt.Errorf("%q isn't a note: write a letter from A to G, then # or b if needed, then an octave from 0 to 8, such as A4 or C#5", name)
	if len(name) < 2 {
		return 0, bad
	}
	semitone, ok := semitones[name[0]]
	if !ok {
		return 0, bad
	}
	rest := name[1:]
	switch rest[0] {
	case '#':
		semitone++
		rest = rest[1:]
	case 'b':
		semitone--
		rest = rest[1:]
	}
	if len(rest) != 1 || rest[0] < '0' || rest[0] > '8' {
		return 0, bad
	}
	octave := int(rest[0] - '0')
	midi := 12*(octave+1) + semitone // A4 is 69
	return float32(440 * math.Pow(2, float64(midi-69)/12)), nil
}
