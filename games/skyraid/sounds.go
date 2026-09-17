package main

import "golib"

// The game's sounds. GoLib makes them when they first play: most from recipes
// in code, and the enemy explosion from assets/sounds/explosion.jfxr, which
// opens in jfxr (https://jfxr.frozenfractal.com) to change it. world.go,
// enemies.go and waves.go play them.
var (
	// shotSound is the ship's gun. It fires nine times a second, so it is
	// short and quiet.
	shotSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 1500, Slide: -5000, Duration: 0.07,
		Attack: 0.001, Release: 0.05, Volume: 0.1, Duty: 0.25,
	})

	// enemyShotSounds is one sound per enemy kind: the bigger the enemy,
	// the lower its gun.
	enemyShotSounds = [...]*golib.Sound{
		scout: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveTriangle, Frequency: 900, Slide: -1600, Duration: 0.12,
			Attack: 0.002, Release: 0.08, Volume: 0.16,
		}),
		gunship: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveSaw, Frequency: 520, Slide: -900, Duration: 0.16,
			Attack: 0.002, Release: 0.1, Volume: 0.14,
		}),
		heavy: golib.NewSound(golib.SoundSpec{
			Wave: golib.WaveSquare, Frequency: 180, Slide: -200, Duration: 0.3,
			Attack: 0.005, Release: 0.2, Volume: 0.2, Vibrato: 30, VibratoRate: 20,
		}),
	}

	// hitSound is a bullet that hits an enemy without destroying it.
	hitSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 3000, Slide: -6000, Duration: 0.05,
		Attack: 0.001, Release: 0.04, Volume: 0.14,
	})

	explosionSound    = golib.NewSoundFile("sounds/explosion.jfxr") // a scout or gunship breaking apart
	bigExplosionSound = golib.Explosion()                           // a heavy breaking apart

	// dashSound is a quick rising whoosh.
	dashSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 600, Slide: 5000, Duration: 0.18,
		Attack: 0.01, Release: 0.12, Volume: 0.2,
	})

	// warpSound is a group of enemies arriving somewhere in the arena.
	warpSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSine, Frequency: 200, Slide: 900, Duration: 0.5,
		Attack: 0.1, Release: 0.3, Volume: 0.18, Vibrato: 25, VibratoRate: 16,
	})

	hurtSound        = golib.Hurt()    // the ship is hit
	repairSound      = golib.Pickup()  // a repair kit is picked up
	waveClearedSound = golib.PowerUp() // the last enemy of a wave is gone

	// shipLostSound is the ship's end: a long falling growl.
	shipLostSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSaw, Frequency: 400, Slide: -350, Duration: 1.1,
		Attack: 0.005, Release: 0.8, Volume: 0.35, Vibrato: 20, VibratoRate: 9,
	})
)
