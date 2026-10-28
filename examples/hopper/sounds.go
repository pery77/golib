package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened.
var (
	coinSound       = golib.Pickup()
	jumpSound       = golib.Jump()
	checkpointSound = golib.PowerUp()

	// landSound is a soft thump for landing after a fall.
	landSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 180, Slide: -120, Duration: 0.09,
		Release: 0.06, Volume: 0.25,
	})

	// fallSound is a whistle that drops, for falling off the level.
	fallSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSine, Frequency: 900, Slide: -900, Duration: 0.8,
		Attack: 0.01, Release: 0.3, Volume: 0.35, Vibrato: 12, VibratoRate: 9,
	})

	// winSound is a bright rising chime for reaching the flag.
	winSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 520, Slide: 900, Duration: 0.6,
		Release: 0.3, Volume: 0.3, Duty: 0.25, Vibrato: 6, VibratoRate: 14,
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
