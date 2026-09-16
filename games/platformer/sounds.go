package main

import "golib"

// The game's sounds. GoLib makes them in code when they first play, so the game
// ships no sound files. These four are ready-made recipes; golib.SoundSpec
// makes one from scratch, as games/asteroids does. world.go plays them.
var (
	jumpSound = golib.Jump()
	coinSound = golib.Pickup()
	fallSound = golib.Hurt()
	winSound  = golib.PowerUp()
)
