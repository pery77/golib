// This file is a Go version of the synthesizer in jfxr, the sound effect maker
// at https://jfxr.frozenfractal.com: lib/src in
// https://github.com/ttencate/jfxr, commit e971e4c, whose synthesizer last
// changed in 2019. It makes the same samples as jfxr from a .jfxr file.
//
// Copyright (c) 2014, Thomas ten Cate. All rights reserved.
//
// Unlike the rest of GoLib, this file is under jfxr's BSD 3-Clause license,
// in LICENSE-jfxr.txt next to it, which golib dist copies into every game's
// THIRD-PARTY-LICENSES.txt.

package golib

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
)

// The synthesizer follows jfxr's JavaScript step by step, down to the order
// of every sum and product, so that the sound matches jfxr's bit for bit:
//
//   - Samples are float32 between steps, as in jfxr's Float32Array, and
//     float64 within a step.
//   - float64(a*b) keeps Go from fusing a product with the sum around it,
//     which some processors do, and which JavaScript never does.
//   - pi is a variable, so that products such as 40*pi are rounded at run
//     time, as JavaScript rounds 40*Math.PI, not as exact constants.
//   - jsRound rounds halves up, as Math.round does.

const (
	jfxrSampleRate = 44100 // jfxr can't change it
	jfxrNyquist    = jfxrSampleRate / 2
	jfxrVersion    = 1 // the newest file version GoLib reads
)

var pi = math.Pi

// jfxrWaveforms are the waveforms of jfxr, as .jfxr files name them.
var jfxrWaveforms = []string{"sine", "triangle", "sawtooth", "square", "tangent", "whistle", "breaker", "whitenoise", "pinknoise", "brownnoise"}

// Indexes into jfxrWaveforms.
const (
	jfxrSine = iota
	jfxrTriangle
	jfxrSawtooth
	jfxrSquare
	jfxrTangent
	jfxrWhistle
	jfxrBreaker
	jfxrWhiteNoise
	jfxrPinkNoise
	jfxrBrownNoise
)

// jfxrSound is a sound effect as a .jfxr file describes it, in the units jfxr
// shows: seconds, Hz and percentages.
type jfxrSound struct {
	attack, sustain, sustainPunch, decay   float64
	tremoloDepth, tremoloFrequency         float64
	frequency, frequencySweep, deltaSweep  float64
	repeatFrequency                        float64
	jump1Onset, jump1Amount                float64
	jump2Onset, jump2Amount                float64
	harmonics, harmonicsFalloff            float64
	waveform                               int
	interpolateNoise                       bool
	vibratoDepth, vibratoFrequency         float64
	squareDuty, squareDutySweep            float64
	flangerOffset, flangerOffsetSweep      float64
	bitCrush, bitCrushSweep                float64
	lowPassCutoff, lowPassCutoffSweep      float64
	highPassCutoff, highPassCutoffSweep    float64
	compression, amplification, sampleRate float64
	normalization                          bool

	cycleRate float64 // repeatFrequency, or once per sound when it is slower
}

// jfxrNumber is a number setting of a .jfxr file: where it goes, and the range
// jfxr keeps it in.
type jfxrNumber struct {
	value     *float64
	low, high float64
	whole     bool // rounded to a whole number
}

// numbers returns the number settings of s by their names in .jfxr files,
// which jfxr's own names for them, in its Sound class, follow.
func (s *jfxrSound) numbers() map[string]jfxrNumber {
	return map[string]jfxrNumber{
		"sampleRate":           {value: &s.sampleRate, low: jfxrSampleRate, high: jfxrSampleRate},
		"attack":               {value: &s.attack, low: 0, high: 5},
		"sustain":              {value: &s.sustain, low: 0, high: 5},
		"sustainPunch":         {value: &s.sustainPunch, low: 0, high: 100},
		"decay":                {value: &s.decay, low: 0, high: 5},
		"tremoloDepth":         {value: &s.tremoloDepth, low: 0, high: 100},
		"tremoloFrequency":     {value: &s.tremoloFrequency, low: 0, high: 1000},
		"frequency":            {value: &s.frequency, low: 10, high: 10000},
		"frequencySweep":       {value: &s.frequencySweep, low: -10000, high: 10000},
		"frequencyDeltaSweep":  {value: &s.deltaSweep, low: -10000, high: 10000},
		"repeatFrequency":      {value: &s.repeatFrequency, low: 0, high: 100},
		"frequencyJump1Onset":  {value: &s.jump1Onset, low: 0, high: 100},
		"frequencyJump1Amount": {value: &s.jump1Amount, low: -100, high: 100},
		"frequencyJump2Onset":  {value: &s.jump2Onset, low: 0, high: 100},
		"frequencyJump2Amount": {value: &s.jump2Amount, low: -100, high: 100},
		"harmonics":            {value: &s.harmonics, low: 0, high: 5, whole: true},
		"harmonicsFalloff":     {value: &s.harmonicsFalloff, low: 0, high: 1},
		"vibratoDepth":         {value: &s.vibratoDepth, low: 0, high: 1000},
		"vibratoFrequency":     {value: &s.vibratoFrequency, low: 0, high: 1000},
		"squareDuty":           {value: &s.squareDuty, low: 0, high: 100},
		"squareDutySweep":      {value: &s.squareDutySweep, low: -100, high: 100},
		"flangerOffset":        {value: &s.flangerOffset, low: 0, high: 50},
		"flangerOffsetSweep":   {value: &s.flangerOffsetSweep, low: -50, high: 50},
		"bitCrush":             {value: &s.bitCrush, low: 1, high: 16},
		"bitCrushSweep":        {value: &s.bitCrushSweep, low: -16, high: 16},
		"lowPassCutoff":        {value: &s.lowPassCutoff, low: 0, high: jfxrNyquist},
		"lowPassCutoffSweep":   {value: &s.lowPassCutoffSweep, low: -jfxrNyquist, high: jfxrNyquist},
		"highPassCutoff":       {value: &s.highPassCutoff, low: 0, high: jfxrNyquist},
		"highPassCutoffSweep":  {value: &s.highPassCutoffSweep, low: -jfxrNyquist, high: jfxrNyquist},
		"compression":          {value: &s.compression, low: 0, high: 5},
		"amplification":        {value: &s.amplification, low: 0, high: 500},
	}
}

// parseJfxr reads a .jfxr file: settings it leaves out keep jfxr's defaults,
// and numbers out of range are kept in range, as jfxr does. Anything jfxr
// wouldn't have written is an error, so that mistakes in files written by
// hand show.
func parseJfxr(data []byte) (jfxrSound, error) {
	s := jfxrSound{
		sampleRate: jfxrSampleRate, tremoloFrequency: 10, frequency: 500,
		jump1Onset: 33, jump2Onset: 66, harmonicsFalloff: 0.5,
		interpolateNoise: true, vibratoFrequency: 10, squareDuty: 50,
		bitCrush: 16, lowPassCutoff: jfxrNyquist, compression: 1,
		normalization: true, amplification: 100,
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return s, fmt.Errorf("the file isn't a jfxr sound, which is a JSON object: %w", err)
	}
	if fields == nil {
		return s, errors.New("the file isn't a jfxr sound, which is a JSON object: it holds null")
	}
	numbers := s.numbers()
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		value := fields[name]
		switch name {
		case "_version":
			version, ok := value.(float64)
			if !ok {
				return s, fmt.Errorf("_version is %s, want a number", jsonKind(value))
			}
			if version > jfxrVersion {
				return s, fmt.Errorf("a newer jfxr saved the sound, in file version %v, and GoLib reads up to version %d: update GoLib, or save the sound from an older jfxr", version, jfxrVersion)
			}
		case "_name", "_locked":
			// What jfxr shows, not how the sound sounds.
		case "waveform":
			waveform, _ := value.(string)
			s.waveform = slices.Index(jfxrWaveforms, waveform)
			if s.waveform < 0 {
				return s, fmt.Errorf("waveform is %s, want one of %s", jsonText(value), strings.Join(jfxrWaveforms, ", "))
			}
		case "interpolateNoise", "normalization":
			on, ok := value.(bool)
			if !ok {
				return s, fmt.Errorf("%s is %s, want true or false", name, jsonKind(value))
			}
			if name == "normalization" {
				s.normalization = on
			} else {
				s.interpolateNoise = on
			}
		default:
			setting, found := numbers[name]
			if !found {
				return s, fmt.Errorf("jfxr has no setting called %q (framework/README.md lists them)", name)
			}
			number, ok := value.(float64)
			if !ok {
				return s, fmt.Errorf("%s is %s, want a number", name, jsonKind(value))
			}
			if setting.whole {
				number = jsRound(number)
			}
			*setting.value = jfxrClamp(setting.low, setting.high, number)
		}
	}
	if s.duration() <= 0 {
		return s, errors.New("the sound lasts 0 seconds: give it an attack, a sustain or a decay in jfxr")
	}
	s.cycleRate = max(s.repeatFrequency, 1/s.duration())
	return s, nil
}

// jsonKind names the kind of a JSON value, for error messages.
func jsonKind(value any) string {
	switch value.(type) {
	case float64:
		return "a number"
	case string:
		return "text"
	case bool:
		return "true or false"
	case nil:
		return "null"
	case []any:
		return "a list"
	default:
		return "an object"
	}
}

// jsonText returns a JSON value as a .jfxr file shows it, for error messages.
func jsonText(value any) string {
	text, _ := json.Marshal(value)
	return string(text)
}

// duration returns the length of the sound in seconds.
func (s *jfxrSound) duration() float64 {
	return s.attack + s.sustain + s.decay
}

// samples makes the sound as jfxr does, as the 16-bit samples of the WAV
// files jfxr exports. The same file always gives the same samples, noise
// included.
func (s *jfxrSound) samples() []int16 {
	buffer := make([]float32, max(1, int(math.Ceil(jfxrSampleRate*s.duration()))))
	s.generate(buffer)
	s.applyEnvelope(buffer)
	s.flange(buffer)
	s.crushBits(buffer)
	s.lowPass(buffer)
	s.highPass(buffer)
	s.compress(buffer)
	s.normalize(buffer)
	s.amplify(buffer)

	samples := make([]int16, len(buffer))
	for i, sample := range buffer {
		value := jsRound(float64(sample) * 0x8000)
		if !math.IsNaN(value) { // silence, normalized, is NaN: jfxr writes 0
			samples[i] = int16(jfxrClamp(-0x8000, 0x7fff, value))
		}
	}
	return samples
}

// generate fills buffer with the waveform at the sound's pitch, with its
// harmonics.
func (s *jfxrSound) generate(buffer []float32) {
	oscillators := make([]jfxrOscillator, int(s.harmonics)+1)
	amplitude, total := 1.0, 0.0
	for i := range oscillators {
		oscillators[i] = jfxrOscillator{sound: s, random: newJfxrRandom(0x3cf78ba3)}
		total += amplitude
		amplitude *= s.harmonicsFalloff
	}
	first := 1 / total

	phase := 0.0
	for i := range buffer {
		time := float64(i) / jfxrSampleRate
		phase = frac(phase + s.frequencyAt(time)/jfxrSampleRate)
		sample, amplitude := 0.0, first
		for h := range oscillators {
			sample += float64(amplitude * oscillators[h].sample(frac(phase*float64(h+1)), time))
			amplitude *= s.harmonicsFalloff
		}
		buffer[i] = float32(sample)
	}
}

// frequencyAt returns the pitch at a time, in Hz.
func (s *jfxrSound) frequencyAt(time float64) float64 {
	cycle := frac(time * s.cycleRate)
	frequency := s.frequency + float64(cycle*s.frequencySweep) + float64(cycle*cycle*s.deltaSweep)
	if cycle > s.jump1Onset/100 {
		frequency *= 1 + s.jump1Amount/100
	}
	if cycle > s.jump2Onset/100 {
		frequency *= 1 + s.jump2Amount/100
	}
	if s.vibratoDepth != 0 {
		// jfxr adds 1 Hz here too.
		frequency += 1 - float64(s.vibratoDepth*(0.5-float64(0.5*math.Sin(2*pi*time*s.vibratoFrequency))))
	}
	return max(0, frequency)
}

// squareDutyAt returns the part of each square wave that is high at a time,
// from 0 to 1.
func (s *jfxrSound) squareDutyAt(time float64) float64 {
	cycle := frac(time * s.cycleRate)
	return (s.squareDuty + float64(cycle*s.squareDutySweep)) / 100
}

// amplitudeAt returns the loudness at a time, from 0 to 2.
func (s *jfxrSound) amplitudeAt(time float64) float64 {
	amplitude := 0.0 // after the decay, which the last sample can reach
	switch {
	case time < s.attack:
		amplitude = time / s.attack
	case time < s.attack+s.sustain:
		amplitude = 1 + float64(s.sustainPunch/100*(1-(time-s.attack)/s.sustain))
	case time < s.attack+s.sustain+s.decay:
		amplitude = 1 - (time-s.attack-s.sustain)/s.decay
	}
	if s.tremoloDepth != 0 {
		amplitude *= 1 - float64(s.tremoloDepth/100*(0.5+float64(0.5*math.Cos(2*pi*time*s.tremoloFrequency))))
	}
	return amplitude
}

// applyEnvelope shapes the loudness over time: attack, sustain, decay and
// tremolo.
func (s *jfxrSound) applyEnvelope(buffer []float32) {
	if s.attack == 0 && s.sustainPunch == 0 && s.decay == 0 && s.tremoloDepth == 0 {
		return
	}
	for i, sample := range buffer {
		buffer[i] = float32(float64(sample) * s.amplitudeAt(float64(i)/jfxrSampleRate))
	}
}

// flange adds the sound to itself, a few milliseconds later.
func (s *jfxrSound) flange(buffer []float32) {
	if s.flangerOffset == 0 && s.flangerOffsetSweep == 0 {
		return
	}
	delayed := make([]float32, int(math.Ceil(jfxrSampleRate*0.1))) // up to 100 ms
	at := 0
	for i := range buffer {
		delayed[at] = buffer[i]
		offset := jsRound((s.flangerOffset + float64(float64(i)/float64(len(buffer))*s.flangerOffsetSweep)) / 1000 * jfxrSampleRate)
		offset = jfxrClamp(0, float64(len(delayed)-1), offset)
		buffer[i] = float32(float64(buffer[i]) + float64(delayed[(at-int(offset)+len(delayed))%len(delayed)]))
		at = (at + 1) % len(delayed)
	}
}

// crushBits keeps fewer levels of loudness. jfxr always does it, with 16 bits
// by default.
func (s *jfxrSound) crushBits(buffer []float32) {
	for i, sample := range buffer {
		bits := s.bitCrush + float64(float64(i)/float64(len(buffer))*s.bitCrushSweep)
		steps := math.Ldexp(1, int(jfxrClamp(1, 16, jsRound(bits))))
		buffer[i] = float32(-1 + 2*jsRound((0.5+0.5*float64(sample))*steps)/steps)
	}
}

// lowPass takes out the frequencies above the cutoff.
func (s *jfxrSound) lowPass(buffer []float32) {
	if s.lowPassCutoff >= jfxrNyquist && s.lowPassCutoff+s.lowPassCutoffSweep >= jfxrNyquist {
		return
	}
	previous := 0.0
	for i, sample := range buffer {
		cutoff := jfxrClamp(0, jfxrNyquist, s.lowPassCutoff+float64(float64(i)/float64(len(buffer))*s.lowPassCutoffSweep))
		cos := math.Cos(cutoff / jfxrSampleRate * pi)
		alpha := 1.0
		if cos > 0 {
			alpha = 1 - (1/cos - math.Sqrt(1/(cos*cos)-1))
		}
		previous = float64(alpha*float64(sample)) + float64((1-alpha)*previous)
		buffer[i] = float32(previous)
	}
}

// highPass takes out the frequencies below the cutoff.
func (s *jfxrSound) highPass(buffer []float32) {
	if s.highPassCutoff <= 0 && s.highPassCutoff+s.highPassCutoffSweep <= 0 {
		return
	}
	previousIn, previousOut := 0.0, 0.0
	for i, sample := range buffer {
		cutoff := jfxrClamp(0, jfxrNyquist, s.highPassCutoff+float64(float64(i)/float64(len(buffer))*s.highPassCutoffSweep))
		angle := cutoff / jfxrSampleRate * pi
		alpha := (1 - math.Sin(angle)) / math.Cos(angle)
		in := float64(sample)
		previousOut = alpha * (previousOut - previousIn + in)
		previousIn = in
		buffer[i] = float32(previousOut)
	}
}

// compress raises every sample to a power: below 1 makes the quiet parts
// louder.
func (s *jfxrSound) compress(buffer []float32) {
	if s.compression == 1 {
		return
	}
	for i, sample := range buffer {
		value := float64(sample)
		if value >= 0 {
			value = math.Pow(value, s.compression)
		} else {
			value = -math.Pow(-value, s.compression)
		}
		buffer[i] = float32(value)
	}
}

// normalize makes the loudest sample full volume.
func (s *jfxrSound) normalize(buffer []float32) {
	if !s.normalization {
		return
	}
	peak := 0.0
	for _, sample := range buffer {
		peak = max(peak, math.Abs(float64(sample)))
	}
	scale := 1 / peak
	for i, sample := range buffer {
		buffer[i] = float32(float64(sample) * scale)
	}
}

// amplify scales the sound by the amplification.
func (s *jfxrSound) amplify(buffer []float32) {
	scale := s.amplification / 100
	if scale == 1 {
		return
	}
	for i, sample := range buffer {
		buffer[i] = float32(float64(sample) * scale)
	}
}

// jfxrOscillator makes one harmonic of a sound's waveform.
type jfxrOscillator struct {
	sound *jfxrSound
	// Noise: its random numbers, the last two values it drew, and where in
	// the wave it was.
	random          jfxrRandom
	previous, value float64
	lastPhase       float64
	pink            [7]float64
}

// sample returns the waveform's value, about -1 to 1, at a point of its
// cycle, from 0 to 1, and a time.
func (o *jfxrOscillator) sample(phase, time float64) float64 {
	switch o.sound.waveform {
	case jfxrTriangle:
		if phase < 0.25 {
			return 4 * phase
		}
		if phase < 0.75 {
			return 2 - 4*phase
		}
		return -4 + 4*phase
	case jfxrSawtooth:
		if phase < 0.5 {
			return 2 * phase
		}
		return -2 + 2*phase
	case jfxrSquare:
		if phase < o.sound.squareDutyAt(time) {
			return 1
		}
		return -1
	case jfxrTangent:
		return jfxrClamp(-2, 2, 0.3*math.Tan(pi*phase)) // cut off, so normalizing works
	case jfxrWhistle:
		return float64(0.75*math.Sin(2*pi*phase)) + float64(0.25*math.Sin(40*pi*phase))
	case jfxrBreaker:
		p := frac(phase + math.Sqrt(0.75)) // starts where the wave crosses 0
		return -1 + float64(2*math.Abs(1-float64(p*p*2)))
	case jfxrWhiteNoise, jfxrPinkNoise, jfxrBrownNoise:
		return o.noise(phase)
	default: // jfxrSine
		return math.Sin(2 * pi * phase)
	}
}

// noise returns the noise at a point of the wave: a new random value twice
// per wave, so that the noise reaches the wave's frequency.
func (o *jfxrOscillator) noise(phase float64) float64 {
	phase = frac(phase * 2)
	if phase < o.lastPhase {
		o.previous = o.value
		white := o.random.uniform()
		switch o.sound.waveform {
		case jfxrPinkNoise:
			// Paul Kellet's method pk3, from
			// http://www.firstpr.com.au/dsp/pink-noise/.
			b := &o.pink
			b[0] = float64(0.99886*b[0]) + float64(white*0.0555179)
			b[1] = float64(0.99332*b[1]) + float64(white*0.0750759)
			b[2] = float64(0.96900*b[2]) + float64(white*0.1538520)
			b[3] = float64(0.86650*b[3]) + float64(white*0.3104856)
			b[4] = float64(0.55000*b[4]) + float64(white*0.5329522)
			b[5] = float64(-0.7616*b[5]) + float64(white*0.0168980)
			o.value = (b[0] + b[1] + b[2] + b[3] + b[4] + b[5] + b[6] + float64(white*0.5362)) / 7
			b[6] = white * 0.115926
		case jfxrBrownNoise:
			o.value = jfxrClamp(-1, 1, o.value+float64(0.1*white))
		default:
			o.value = white
		}
	}
	o.lastPhase = phase
	if o.sound.interpolateNoise {
		return float64((1-phase)*o.previous) + float64(phase*o.value)
	}
	return o.value
}

// jfxrRandom is jfxr's random number generator, an xorshift generator whose
// numbers JavaScript keeps in 32-bit integers.
type jfxrRandom struct {
	x, y, z, w int32
}

func newJfxrRandom(seed int32) jfxrRandom {
	r := jfxrRandom{x: seed, y: 362436069, z: 521288629, w: 88675123}
	for range 32 {
		r.next()
	}
	return r
}

// next returns the next number, from 0 to 2^32-1.
func (r *jfxrRandom) next() float64 {
	t := r.x ^ r.x<<11
	r.x, r.y, r.z = r.y, r.z, r.w
	r.w = r.w ^ int32(uint32(r.w)>>19) ^ (t ^ int32(uint32(t)>>8))
	return float64(r.w) + 0x80000000
}

// uniform returns a number from -1 to 1.
func (r *jfxrRandom) uniform() float64 {
	return -1 + 2*r.next()/0xffffffff
}

// frac returns the part of x after the point, from 0 up to 1.
func frac(x float64) float64 {
	return x - math.Floor(x)
}

// jfxrClamp keeps x from low to high. Unlike Go's min and max, it keeps -0 as
// it is, as jfxr's clamp does.
func jfxrClamp(low, high, x float64) float64 {
	if x < low {
		return low
	}
	if x > high {
		return high
	}
	return x
}

// jsRound rounds x as JavaScript's Math.round does: halves go up, so -2.5
// becomes -2.
func jsRound(x float64) float64 {
	rounded := math.Floor(x)
	if x-rounded >= 0.5 {
		rounded++
	}
	return rounded
}
