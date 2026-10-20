package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened.
var (
	gemSound     = golib.Pickup()
	hitSound     = golib.Hurt()
	levelUpSound = golib.PowerUp()

	// fallSound is a whistle that drops, for rolling off the edge.
	fallSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSine, Frequency: 900, Slide: -900, Duration: 0.8,
		Attack: 0.01, Release: 0.3, Volume: 0.35, Vibrato: 12, VibratoRate: 9,
	})

	// gameOverSound is a low falling buzz.
	gameOverSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 220, Slide: -160, Duration: 0.9,
		Release: 0.4, Volume: 0.3, Duty: 0.3,
	})

	// startSound is a short blip for menus.
	startSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 660, Slide: 600, Duration: 0.12, Volume: 0.35,
	})
)
