package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened.
var (
	coinSound = golib.Pickup()
	jumpSound = golib.Jump()
	winSound  = golib.PowerUp()

	// breakSound is a rock bursting open.
	breakSound = golib.Explosion()

	// punchSound is the swish of a fist through the air.
	punchSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 2000, Slide: -4000, Duration: 0.12,
		Attack: 0.02, Release: 0.07, Volume: 0.16,
	})

	// knockSound is a fist on stone: a short hard knock.
	knockSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 180, Slide: -500, Duration: 0.09,
		Release: 0.06, Volume: 0.6,
	})

	// splashSound is falling in the sea.
	splashSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 900, Slide: -1500, Duration: 0.45,
		Attack: 0.01, Release: 0.35, Volume: 0.3,
	})

	// landSound is a soft thump for landing from a jump.
	landSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 180, Slide: -120, Duration: 0.08,
		Release: 0.06, Volume: 0.2,
	})

	// tickSound counts the last ten seconds down.
	tickSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 1320, Duration: 0.05, Release: 0.03, Volume: 0.18, Duty: 0.25,
	})

	// timeUpSound is a low falling buzz.
	timeUpSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 260, Slide: -180, Duration: 0.9,
		Release: 0.4, Volume: 0.3, Duty: 0.3,
	})

	// startSound is a short blip for menus.
	startSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 660, Slide: 600, Duration: 0.12, Volume: 0.35,
	})
)
