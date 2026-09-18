package main

import "golib"

// The game's music and sounds. GoLib makes them in code, so the game ships no
// sound files: change the numbers to change how they sound. play.go plays the
// sound effects when the world reports that something happened.

// theme is the background music, a loop made from notes because the game has no
// music file. It is the classic Tetris feel: a fast minor-key melody over a
// simple bass, with a tick on the beat. To use a real track instead, put an OGG
// or a tracker module in assets/ and change only this line to
// golib.NewMusic("music/theme.ogg").
var theme = golib.NewTune(themeSpec)

// themeSpec is the tune: every voice is sixteen beats long, so they loop in
// step. It is written in eighth notes, so its tempo is twice the beats per
// minute the ear hears.
var themeSpec = golib.TuneSpec{
	Tempo: 300, // eighth notes at 150 a minute
	Voices: []golib.Voice{
		{
			Wave: golib.WaveSquare, Duty: 0.5, Volume: 0.30,
			Notes: "e5 b4 c5 d5 c5 b4 a4 a4 c5 e5 d5 c5 b4 - c5 d5",
		},
		{
			Wave: golib.WaveTriangle, Volume: 0.30,
			Notes: "a2/2 e3/2 a2/2 e3/2 f2/2 c3/2 g2/2 g2/2",
		},
		{
			Wave: golib.WaveNoise, Volume: 0.05, Gap: 0.12,
			Notes: ". a7 . a7 . a7 . a7 . a7 . a7 . a7 . a7",
		},
	},
}

// musicVolume keeps the theme under the sound effects, from 0 to 1.
const musicVolume = 0.22

// The game's sound effects, each made from a golib.SoundSpec. Short, quiet
// blips for moving and turning; a lower thud for landing; a rising sweep for
// clearing lines; GoLib's PowerUp for a level-up, and Hurt for game over.
var (
	// moveSound is the small click when a piece slides sideways.
	moveSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 320, Slide: 220, Duration: 0.05,
		Attack: 0.001, Release: 0.04, Volume: 0.10, Duty: 0.3,
	})

	// rotateSound is a brighter click when a piece turns.
	rotateSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 520, Slide: 260, Duration: 0.06,
		Attack: 0.001, Release: 0.05, Volume: 0.11, Duty: 0.35,
	})

	// lockSound is the dull thud of a piece coming to rest.
	lockSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 320, Slide: -180, Duration: 0.09,
		Attack: 0.001, Release: 0.07, Volume: 0.18,
	})

	// holdSound is a two-tone blip when a piece goes to hold.
	holdSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 380, Slide: 700, Duration: 0.09,
		Attack: 0.001, Release: 0.07, Volume: 0.13, Duty: 0.4,
	})

	// clearSound is a bright rising sweep when lines clear.
	clearSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 500, Slide: 1900, Duration: 0.28,
		Attack: 0.002, Release: 0.22, Volume: 0.20, Duty: 0.3,
	})

	// tetrisSound is a bigger fanfare for four lines at once.
	tetrisSound = golib.PowerUp()

	// levelUpSound plays when the level rises.
	levelUpSound = golib.PowerUp()

	// gameOverSound plays when no piece fits any more.
	gameOverSound = golib.Hurt()
)

func init() {
	theme.SetVolume(musicVolume)
}
