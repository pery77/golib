package main

import "golib"

var (
	engineSound = golib.NewSound(golib.SoundSpec{Wave: golib.WaveNoise, Frequency: 85, Duration: 0.6, Attack: -1, Release: -1, Volume: 0.16})
	clickSound  = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSine, Frequency: 700, Slide: 250, Duration: 0.08, Volume: 0.13})
	landedSound = golib.PowerUp()
	crashSound  = golib.Explosion()
	buySound    = golib.Pickup()
	alertSound  = golib.NewSound(golib.SoundSpec{Wave: golib.WaveSine, Frequency: 420, Duration: 0.12, Volume: 0.16})
)
var themeSpec = golib.TuneSpec{Tempo: 72, Voices: []golib.Voice{
	{Wave: golib.WaveSine, Volume: 0.28, Notes: "d3/8 bb2/8 f3/8 c3/8"},
	{Wave: golib.WaveTriangle, Volume: 0.10, Notes: "a4/2 ./2 d5/2 ./2 f5/2 ./2 e5/2 ./2 a4/2 ./2 c5/2 ./2 g4/2 ./2 a4/2 ./2"},
}}
var theme = golib.NewTune(themeSpec)
