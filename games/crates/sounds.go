package main

import "golib"

// The game's sound effects. GoLib makes them when they first play, so the game
// ships no recorded sounds. The level-complete fanfare comes from settings
// made for jfxr (https://jfxr.frozenfractal.com), which can open
// assets/sounds/complete.jfxr to change it; the rest are recipes. play.go and
// scenes.go play them. The music is in music.go.
var (
	// stepSound is a soft tap for each step, quiet enough to hear a hundred times.
	stepSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 190, Slide: -300, Duration: 0.05,
		Attack: 0.002, Release: 0.04, Volume: 0.16,
	})

	// pushSound is a crate scraping across the floor.
	pushSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 420, Slide: -500, Duration: 0.13,
		Attack: 0.005, Release: 0.08, Volume: 0.26,
	})

	// landSound is a bright blip for a crate that lands on a goal.
	landSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 660, Slide: 1400, Duration: 0.13,
		Attack: 0.002, Release: 0.08, Volume: 0.24, Duty: 0.25,
	})

	// bumpSound is a dull knock for walking into a wall or a stuck crate, and
	// for picking a locked level.
	bumpSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 110, Slide: -200, Duration: 0.07,
		Attack: 0.002, Release: 0.05, Volume: 0.18,
	})

	// undoSound is a short falling note for taking a step back.
	undoSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 620, Slide: -1600, Duration: 0.08,
		Attack: 0.002, Release: 0.05, Volume: 0.22,
	})

	// restartSound is a falling hiss for starting a level over.
	restartSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 1800, Slide: -5000, Duration: 0.3,
		Attack: 0.01, Release: 0.2, Volume: 0.2,
	})

	// completeSound is a rising three-note fanfare for a finished level.
	completeSound = golib.NewSoundFile("sounds/complete.jfxr")

	// menuMoveSound and menuSelectSound are the menus' ticks.
	menuMoveSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 900, Duration: 0.035,
		Attack: 0.001, Release: 0.025, Volume: 0.12, Duty: 0.25,
	})
	menuSelectSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 620, Slide: 1600, Duration: 0.09,
		Attack: 0.001, Release: 0.06, Volume: 0.18, Duty: 0.25,
	})
)
