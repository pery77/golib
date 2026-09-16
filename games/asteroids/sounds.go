package main

import "golib"

// theme is the music, streamed from the game's assets folder. scenes.go starts
// it and F3 or the X button turns it off. assets/ATTRIBUTION.md says where it
// comes from.
var theme = golib.NewMusic("4_rndd!.xm")

// musicVolume keeps the theme under the sound effects, from 0 to 1.
const musicVolume = 0.20

// The game's sounds. GoLib makes them when they first play, from these recipes,
// so the game ships no sound files: change the numbers to change how it sounds.
// world.go plays them; see golib.SoundSpec for what each field does.
var (
	// shotSound is the ship's gun: a short zap whose pitch falls.
	shotSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 1200, Slide: -3000, Duration: 0.12,
		Attack: 0.001, Release: 0.09, Volume: 0.22, Duty: 0.2,
	})

	// rockSounds is one burst of noise per rock size: the bigger the rock, the
	// lower and longer it breaks.
	rockSounds = [...]*golib.Sound{
		smallRock: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveNoise, Frequency: 1100, Slide: -800, Duration: 0.2,
			Attack: 0.001, Release: 0.18, Volume: 0.3,
		}),
		mediumRock: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveNoise, Frequency: 700, Slide: -460, Duration: 0.32,
			Attack: 0.001, Release: 0.3, Volume: 0.38,
		}),
		bigRock: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveNoise, Frequency: 420, Slide: -260, Duration: 0.45,
			Attack: 0.001, Release: 0.42, Volume: 0.45,
		}),
	}

	// crashSound and waveSound are two of GoLib's ready-made recipes.
	crashSound = golib.Explosion()
	waveSound  = golib.PowerUp()
)
