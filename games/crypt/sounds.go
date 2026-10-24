package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened. The guards' footsteps, the torches, the lift and a
// guard's shriek are heard from where they are, with Sound.PlayAt and
// Sound.LoopAt.
var (
	keySound        = golib.Pickup()
	caughtSound     = golib.Hurt()
	winSound        = golib.PowerUp()
	checkpointSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveTriangle, Frequency: 520, Slide: 400, Duration: 0.35, Release: 0.25, Volume: 0.3, Vibrato: 8, VibratoRate: 9})

	// guardStepSound is a bony clack on stone.
	guardStepSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 900, Slide: -2400, Duration: 0.07, Attack: 0.001, Release: 0.05, Volume: 0.55})

	// playerStepSound is the astronaut's soft boot.
	playerStepSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 260, Slide: -300, Duration: 0.06, Release: 0.05, Volume: 0.12})

	// crackleSounds are a torch's pops, three of them so they don't repeat.
	crackleSounds = []*golib.Sound{
		golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 3000, Slide: -8000, Duration: 0.03, Attack: 0.001, Release: 0.02, Volume: 0.35}),
		golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 2200, Slide: -5000, Duration: 0.045, Attack: 0.001, Release: 0.03, Volume: 0.3}),
		golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 4200, Slide: -9000, Duration: 0.02, Attack: 0.001, Release: 0.015, Volume: 0.3}),
	}

	// fireSound is the soft roar of the torch nearest the player, looped:
	// noise, with no fades, so it loops without a click.
	fireSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 180, Duration: 1, Attack: -1, Release: -1, Volume: 0.12})

	// liftSound is the stone lift grinding, looped while it moves.
	liftSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 90, Duration: 1, Attack: -1, Release: -1, Volume: 0.35})

	// liftStopSound is the lift settling at a floor.
	liftStopSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 120, Slide: -200, Duration: 0.25, Release: 0.2, Volume: 0.5})

	// punchSound is the fist going through the air.
	punchSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 2400, Slide: -5000, Duration: 0.12, Attack: 0.02, Release: 0.08, Volume: 0.15})

	// smashSound is a crate splintering.
	smashSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 700, Slide: -1500, Duration: 0.3, Attack: 0.001, Release: 0.22, Volume: 0.6})

	// doorSound is a door crumbling to dust, low and long.
	doorSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 220, Slide: -180, Duration: 0.9, Attack: 0.02, Release: 0.6, Volume: 0.55})

	// lockedSound is a locked door rattling.
	lockedSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSquare, Frequency: 110, Duration: 0.18, Release: 0.1, Volume: 0.25, Duty: 0.3, Vibrato: 40, VibratoRate: 30})

	// shriekSound is a guard that has seen the player: a thin rising wail.
	shriekSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSaw, Frequency: 500, Slide: 900, Duration: 0.4, Attack: 0.03, Release: 0.25, Volume: 0.25, Vibrato: 30, VibratoRate: 14})

	// startSound is a short blip for menus.
	startSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveTriangle, Frequency: 660, Slide: 600, Duration: 0.12, Volume: 0.35})

	// overSound is a low falling buzz, for the last heart.
	overSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSquare, Frequency: 220, Slide: -160, Duration: 0.9, Release: 0.4, Volume: 0.3, Duty: 0.3})
)
