package main

import "golib"

// The game's sounds. GoLib makes them when they first play, so the game ships
// no recorded sounds. Most are ready-made recipes; the slash is made from
// scratch with golib.SoundSpec, and the chest from settings made in jfxr
// (https://jfxr.frozenfractal.com), which can open assets/sounds/chest.jfxr
// to change it. world.go plays them.
var (
	jumpSound  = golib.Jump()
	chestSound = golib.NewSoundFile("sounds/chest.jfxr") // a rising arpeggio
	fallSound  = golib.Hurt()
	hitSound   = golib.Explosion()
	winSound   = golib.PowerUp()

	// slashSound is a short hiss that falls, for the hero's slash.
	slashSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 2600, Slide: -9000, Duration: 0.14,
		Attack: 0.004, Release: 0.1, Volume: 0.22,
	})
)
