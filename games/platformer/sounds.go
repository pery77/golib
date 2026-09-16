package main

import "golib"

// The game's sounds. GoLib makes them in code when they first play, so the game
// ships no sound files. Most are ready-made recipes; the slash is made from
// scratch with golib.SoundSpec. world.go plays them.
var (
	jumpSound  = golib.Jump()
	chestSound = golib.Pickup()
	fallSound  = golib.Hurt()
	hitSound   = golib.Explosion()
	winSound   = golib.PowerUp()

	// slashSound is a short hiss that falls, for the hero's slash.
	slashSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 2600, Slide: -9000, Duration: 0.14,
		Attack: 0.004, Release: 0.1, Volume: 0.22,
	})
)
