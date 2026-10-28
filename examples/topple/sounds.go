package main

import "golib"

// The game's sounds, made in code. The scenes play them from what the world
// says happened.
var (
	// woodSound is a crate knocking into something: a short, hollow thud.
	woodSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 220, Slide: -900, Duration: 0.09,
		Attack: 0.001, Release: 0.07, Volume: 0.55,
	})
	// woodSoftSound is a gentler knock, for a small bump: PlayAt has no
	// volume of its own, so the soft hits are a sound of their own.
	woodSoftSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 260, Slide: -900, Duration: 0.07,
		Attack: 0.001, Release: 0.06, Volume: 0.22,
	})
	// barrelSound is a barrel's deeper, rounder knock.
	barrelSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSine, Frequency: 140, Slide: -400, Duration: 0.14,
		Attack: 0.001, Release: 0.1, Volume: 0.6,
	})
	// ballSound is a ball smacking into something: a burst of noise.
	ballSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 900, Slide: -3000, Duration: 0.06,
		Attack: 0.001, Release: 0.05, Volume: 0.35,
	})
	// throwSound is the whoosh of a throw.
	throwSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 300, Slide: 2500, Duration: 0.16,
		Attack: 0.03, Release: 0.1, Volume: 0.18,
	})
	fellSound    = golib.Pickup()  // a target came off its platform
	clearSound   = golib.PowerUp() // the level is cleared
	confirmSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSquare, Frequency: 660, Duration: 0.07, Volume: 0.2, Duty: 0.3})
)

// softHit is the strength below which a crate's knock is the soft one.
const softHit = 0.35

// playEvents plays what happened in an update, from where it happened.
func playEvents(happened events, camera *golib.Camera3D) {
	if happened.threw {
		throwSound.PlayWith(1, golib.RandomFloat(0.9, 1.15))
	}
	for _, hit := range happened.impacts {
		sound := woodSound
		switch {
		case hit.ball:
			sound = ballSound
		case hit.barrel:
			sound = barrelSound
		case hit.strength < softHit:
			sound = woodSoftSound
		}
		sound.PlayAt(hit.position, camera)
	}
	if len(happened.fell) > 0 {
		fellSound.PlayWith(0.5, golib.RandomFloat(0.95, 1.1))
	}
	if happened.cleared {
		clearSound.Play()
	}
}
