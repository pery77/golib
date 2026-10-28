package main

import "golib"

// The game's sounds. GoLib makes them when they first play: the ones the
// player hears most from files in assets/sounds/, which open in jfxr
// (https://jfxr.frozenfractal.com) to change them, and the rest from recipes
// in code. world.go, enemies.go and waves.go play them.
//
// A file made in jfxr comes out far louder than a sound made from a recipe, so
// every one of them is turned down here, with atVolume, instead of changing
// the file: leave the file as jfxr saved it and balance it in this one place.
var (
	// shotSound is the ship's gun. It fires nine times a second, so it plays
	// quietly.
	shotSound = soundFile("sounds/laser.jfxr", 0.25)

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
	hitSound = soundFile("sounds/hitsound.jfxr", 0.4)

	explosionSound    = soundFile("sounds/explosion.jfxr", 0.4) // a scout or gunship breaking apart
	bigExplosionSound = golib.Explosion()                       // a heavy breaking apart

	// dashSound is a quick rising whoosh.
	dashSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 600, Slide: 5000, Duration: 0.18,
		Attack: 0.01, Release: 0.12, Volume: 0.2,
	})

	// warpSound is a group of enemies arriving somewhere in the arena.
	warpSound        = soundFile("sounds/warpsound.jfxr", 0.4)
	hurtSound        = soundFile("sounds/playerhit.jfxr", 0.6)        // the ship is hit
	repairSound      = soundFile("sounds/powerup.jfxr", 0.5)          // a repair kit is picked up
	shipLostSound    = soundFile("sounds/playerexplosion.jfxr", 0.45) // the ship's end
	waveClearedSound = soundFile("sounds/wavecleared.jfxr", 0.4)      // the last enemy of a wave is gone
)

// soundFiles is every file the sounds above are made from, for the test that
// checks they are all still there.
var soundFiles []string

// soundFile makes a sound from a file in the game's assets folder, as loud as
// volume says, from 0 to 1, and remembers its name. Every sound says up there
// how it sits against the rest: the volumes come from how loud jfxr saved each
// file, not from listening, so tune them by ear.
func soundFile(name string, volume float32) *golib.Sound {
	soundFiles = append(soundFiles, name)
	sound := golib.NewSoundFile(name)
	sound.SetVolume(volume)
	return sound
}
