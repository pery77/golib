package main

import "golib"

// The game's sounds, made in code.
var (
	pickupSound = golib.Pickup()
	escapeSound = golib.PowerUp()

	// gateSound is the gate grinding down into the floor: low noise that
	// falls.
	gateSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 160, Slide: -60, Duration: gateOpenTime,
		Attack: 0.2, Release: 0.6, Volume: 0.35,
	})
)
