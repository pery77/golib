package main

import (
	"strings"

	"golib"
)

// The background music: a tune made from notes with golib.NewTune, so the game
// ships no music file. It is an ordinary golib.Music, so session.go starts it
// and pauses it with the music switch (M, or the Back button).
//
// Notes sound thinner than a real track. To play a music file instead, such as
// assets/music/theme.ogg, make theme with golib.NewMusic("music/theme.ogg")
// and delete themeSpec, then write where the file came from, and its license,
// in assets/ATTRIBUTION.md.

// Tuning for the tune. A beat here is an eighth note, so 225 beats per minute
// is 112.5 in four-four time, and a bar is 8 beats.
const (
	tuneTempo  = 225  // beats per minute
	leadVolume = 0.09 // from 0 to 1, under the sound effects
	bassVolume = 0.13
	tickVolume = 0.03
)

// tickBar is one half bar of the tick: noise at the top of the range, which
// hisses finely, on the two off-beats. A quarter of a beat is about 30
// milliseconds of sound, which ticks instead of hissing.
const tickBar = "./1 a8/0.25 ./1.75 a8/0.25 ./0.75 "

// theme is the music. It loops on its own, for as long as it plays.
var theme = golib.NewTune(themeSpec)

// themeSpec is the tune: an original melody in A minor over a bass, with a
// soft tick on the off-beats. It is 16 bars, about 34 seconds. Each line of
// the melody is one bar; a note lasts one beat unless it says otherwise, so
// "e5/2" holds for two and "." rests for one. Every voice lasts 128 beats, so
// they line up each time round; music_test.go checks that.
var themeSpec = golib.TuneSpec{
	Tempo: tuneTempo,
	Voices: []golib.Voice{
		{Wave: golib.WaveSquare, Duty: 0.3, Volume: leadVolume, Notes: `
			e5/2 a4 c5 e5 d5 c5/2
			a4/2 f4 a4 c5 b4 a4/2
			g4/2 c5 e5 g5 f5 e5 d5
			d5/2 b4/2 g4/2 ./2
			e5/2 a4 c5 e5 f5 e5 d5
			c5/2 a4 c5 f5 e5 c5/2
			d5/2 b4 d5 g5 f5 d5 b4
			a4/4 ./4
			. a4 c5 a4 . c5 f5 e5
			d5/2 . b4 d5 g4/2 .
			e5/2 b4/2 g4 b4 e5 d5
			c5/2 a4/2 e4/2 ./2
			f4 a4 c5 f5 e5/2 c5/2
			d5/2 g4 b4 d5/2 g5/2
			g#4/2 b4/2 e5/2 d5/2
			b4/2 g#4/2 e4/2 ./2`},
		{Wave: golib.WaveTriangle, Volume: bassVolume, Notes: bassLine(
			"a2", "f2", "c3", "g2", "a2", "f2", "g2", "a2",
			"f2", "g2", "e2", "a2", "f2", "g2", "e2", "e2",
		)},
		{Wave: golib.WaveNoise, Volume: tickVolume, Notes: strings.Repeat(tickBar, 32)},
	},
}

// bassLine returns the bass: one bar for each root note, which walks between
// the root and the note an octave above it, one beat at a time.
func bassLine(roots ...string) string {
	var bars []string
	for _, root := range roots {
		octave := root[:len(root)-1] + string(root[len(root)-1]+1)
		bars = append(bars, strings.Repeat(root+" "+octave+" ", 4))
	}
	return strings.Join(bars, "\n")
}
