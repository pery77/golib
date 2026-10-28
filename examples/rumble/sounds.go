package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened.
var (
	jumpSound = golib.Jump()
	hurtSound = golib.Hurt()
	waveSound = golib.PowerUp()

	// swingSound is the swish of the stick through the air.
	swingSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 2400, Slide: -5000, Duration: 0.14,
		Attack: 0.03, Release: 0.08, Volume: 0.18,
	})

	// whackSound is the stick landing: a short hollow knock.
	whackSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 260, Slide: -900, Duration: 0.1,
		Release: 0.07, Volume: 0.55,
	})

	// knockoutSound is a falling boing for an enemy that goes down.
	knockoutSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 520, Slide: -700, Duration: 0.45,
		Release: 0.25, Volume: 0.3, Duty: 0.3, Vibrato: 30, VibratoRate: 18,
	})

	// landSound is a soft thump for landing from a jump.
	landSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 180, Slide: -120, Duration: 0.08,
		Release: 0.06, Volume: 0.22,
	})

	// gameOverSound is a low falling buzz.
	gameOverSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 220, Slide: -160, Duration: 0.9,
		Release: 0.4, Volume: 0.3, Duty: 0.3,
	})

	// startSound is a short blip for menus and a new wave.
	startSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 660, Slide: 600, Duration: 0.12, Volume: 0.35,
	})
)
