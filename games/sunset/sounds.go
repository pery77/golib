package main

import "golib"

// The game's sounds, made in code: the scenes play them when the world says
// something happened.
var (
	crystalSound = golib.Pickup()
	goldenSound  = golib.PowerUp()

	// bumpSound is a dull knock, for hitting a pole, an islet or the edge.
	bumpSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveNoise, Frequency: 180, Slide: -200, Duration: 0.18, Release: 0.12, Volume: 0.4,
	})

	// fadeSound is a soft falling note, for a golden crystal that fades away.
	fadeSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSine, Frequency: 900, Slide: -1200, Duration: 0.35, Release: 0.25, Volume: 0.25,
	})

	// tickSound is a short beep for each of the last ten seconds.
	tickSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 880, Duration: 0.08, Release: 0.04, Volume: 0.3,
	})

	// timeUpSound is a long low horn, for the end of the run.
	timeUpSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSaw, Frequency: 196, Slide: -20, Duration: 1.0, Attack: 0.03, Release: 0.5, Volume: 0.28, Vibrato: 3, VibratoRate: 6,
	})

	// startSound is a short blip for menus.
	startSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 660, Slide: 600, Duration: 0.12, Volume: 0.35,
	})

	// engineSound is the fan's hum, looped while the hovercraft is played: 20
	// whole waves at 80 Hz, with no fades, so it loops without a click.
	engineSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveTriangle, Frequency: 80, Duration: 0.25, Attack: -1, Release: -1, Volume: 0.18,
	})
)
