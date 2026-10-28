// Command soundgen makes the game's ambience loops as WAV files:
//
//	go run ./tools/soundgen wind.wav oilbed.wav
//	go run ./tools/soundgen mineral-ring.wav
//	go run ./tools/soundgen --tink mineral-tink.wav
//
// Wind and oil are lowpassed noise, crossfaded into themselves. The
// mineral ring wanders at irregular intervals and ends on a whole wave.
// ffmpeg can turn the noise beds into the OGG files the game reads:
//
//	ffmpeg -i wind.wav -c:a libvorbis -q:a 4 wind-loop.ogg
package main

import (
	"encoding/binary"
	"math"
	"math/rand"
	"os"
)

const rate = 22050

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--alert" {
		write(os.Args[2], alert())
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--tink" {
		write(os.Args[2], mineralTink())
		return
	}
	if len(os.Args) == 2 {
		write(os.Args[1], mineralRing())
		return
	}
	write(os.Args[1], wind())
	write(os.Args[2], oilBed())
}

func alert() []float64 {
	out := make([]float64, int(1.5*rate))
	hornNote(out, 0, 0.25, 110, 0.75)
	hornNote(out, 0.28, 0.25, 146.83, 0.8)
	hornNote(out, 0.58, 0.78, 196, 0.85)
	hornNote(out, 0.58, 0.78, 98, 0.65)
	hornNote(out, 0.58, 0.78, 146.83, 0.35)
	normalize(out, 0.75)
	return out
}

func hornNote(out []float64, start, duration, hz, level float64) {
	for i := int(start * rate); i < len(out); i++ {
		elapsed := float64(i)/rate - start
		if elapsed >= duration {
			break
		}
		attack := math.Min(1, elapsed/0.035)
		release := math.Min(1, (duration-elapsed)/0.18)
		breath := 0.93 + 0.07*math.Sin(2*math.Pi*5*elapsed)
		phase := 2 * math.Pi * hz * elapsed
		out[i] += level * attack * release * breath * (math.Sin(phase) + 0.55*math.Sin(2*phase) +
			0.3*math.Sin(3*phase) + 0.12*math.Sin(4*phase))
	}
}

func mineralRing() []float64 {
	const seconds = 8.0
	const frequency = 320.0
	rng := rand.New(rand.NewSource(71))
	times := []float64{0}
	pitches := []float64{0}
	for at := 0.0; ; {
		next := at + 0.25 + rng.Float64()*0.6
		if next >= seconds-0.5 {
			break
		}
		at = next
		times = append(times, at)
		pitches = append(pitches, rng.Float64()-0.5)
	}
	times = append(times, seconds)
	pitches = append(pitches, 0)

	n := int(seconds * rate)
	hz := make([]float64, n)
	cycles := 0.0
	segment := 0
	for i := range hz {
		at := float64(i) / rate
		for at >= times[segment+1] {
			segment++
		}
		span := (at - times[segment]) /
			(times[segment+1] - times[segment])
		blend := span * span * (3 - 2*span)
		pitch := pitches[segment] +
			(pitches[segment+1]-pitches[segment])*blend
		hz[i] = frequency * math.Exp2(pitch/12)
		cycles += hz[i] / rate
	}

	correction := math.Round(cycles) / cycles
	out := make([]float64, n)
	phase := 0.0
	for i := range out {
		out[i] = 0.4 * math.Sin(2*math.Pi*phase)
		phase += hz[i] * correction / rate
	}
	return out
}

func mineralTink() []float64 {
	const seconds = 0.8
	const frequency = 3800.0
	out := make([]float64, int(seconds*rate))
	for i := range out {
		at := float64(i) / rate
		attack := math.Min(1, at/0.004)
		release := math.Min(1, (seconds-at)/0.16)
		decay := math.Exp(-3.8 * at)
		modulation := 0.25 + 0.75*(0.5+0.5*
			math.Sin(2*math.Pi*6*at))
		phase := 2 * math.Pi * frequency * at
		metal := math.Sin(phase) + 0.24*math.Sin(2*phase) +
			0.08*math.Sin(3*phase)
		out[i] = attack * release * decay * modulation * metal
	}
	normalize(out, 0.75)
	return out
}

// lowpass is a one-pole filter at fc hertz.
func lowpass(x []float64, fc float64) []float64 {
	a := 1 - math.Exp(-2*math.Pi*fc/rate)
	y := make([]float64, len(x))
	for i := 1; i < len(x); i++ {
		y[i] = y[i-1] + a*(x[i]-y[i-1])
	}
	return y
}

// loopBed renders seconds of a leaky noise bed, lowpassed, its level
// wandering on its own noise, then crossfades the tail into the head so
// the loop never clicks.
func loopBed(seed int64, seconds float64, leak float64, fc float64, wander float64, depth float64, peak float64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	n := int(seconds * rate)
	fade := int(1.5 * rate)

	bed := make([]float64, n+fade)
	v := 0.0
	wanderState := 0.0
	for i := range bed {
		v = v*leak + (rng.Float64()*2 - 1)
		wanderState = wanderState*0.9997 + (rng.Float64()*2-1)*0.0004
		bed[i] = v * (1 + wander*wanderState)
	}
	bed = lowpass(bed, fc)

	out := make([]float64, n)
	for i := 0; i < fade; i++ {
		w := float64(i) / float64(fade)
		out[i] = bed[i]*w + bed[n+i]*(1-w)
	}
	copy(out[fade:], bed[fade:n])

	m := 0.0
	for _, s := range out {
		if c := math.Abs(s); c > m {
			m = c
		}
	}
	g := peak / m
	for i := range out {
		out[i] *= g * depth
	}
	return out
}

// bedNoise renders seconds of leaky noise lowpassed at fc hertz, its
// tail crossfaded into its head, so it loops with no seam.
func bedNoise(seed int64, seconds float64, leak float64, fc float64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	n := int(seconds * rate)
	fade := int(1.5 * rate)
	raw := make([]float64, n+fade)
	v := 0.0
	for i := range raw {
		v = v*leak + (rng.Float64()*2 - 1)
		raw[i] = v
	}
	raw = lowpass(raw, fc)
	out := make([]float64, n)
	for i := 0; i < fade; i++ {
		w := float64(i) / float64(fade)
		out[i] = raw[i]*w + raw[n+i]*(1-w)
	}
	copy(out[fade:], raw[fade:n])
	return out
}

// bandpass keeps the noise between low and high hertz: the difference of
// two lowpasses.
func bandpass(x []float64, low, high float64) []float64 {
	top := lowpass(x, high)
	bottom := lowpass(top, low)
	out := make([]float64, len(x))
	for i := range out {
		out[i] = top[i] - bottom[i]
	}
	return out
}

// wind is three layers the same gust drives: a deep rumble always there,
// an air that swells with it and a whistle only the strongest gusts
// sing. The gust itself is a loop, so the whole is.
func wind() []float64 {
	seconds := 10.0
	gust := bedNoise(5, seconds, 0.99965, 40)
	lo, hi := 0.0, 0.0
	for _, v := range gust {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	for i, v := range gust {
		g := (v - lo) / (hi - lo)
		gust[i] = g * g * g
	}
	rumble := bedNoise(11, seconds, 0.995, 130)
	air := bedNoise(17, seconds, 0.985, 800)
	// A second pole, so the air's tail doesn't hiss to the top of the
	// band: wind hisses a little, not everywhere.
	air = lowpass(air, 900)
	whistle := bandpass(bedNoise(29, seconds, 0.97, 1400), 500, 1100)

	out := make([]float64, len(gust))
	for i, g := range gust {
		out[i] = rumble[i]*(0.40+0.60*g) +
			air[i]*(0.12+0.88*g) +
			whistle[i]*0.55*g
	}
	normalize(out, 0.85)
	return out
}

func normalize(x []float64, peak float64) {
	m := 0.0
	for _, v := range x {
		if c := math.Abs(v); c > m {
			m = c
		}
	}
	if m == 0 {
		return
	}
	for i := range x {
		x[i] *= peak / m
	}
}

func oilBed() []float64 {
	return loopBed(47, 10, 0.996, 240, 1.1, 1, 0.8)
}

func write(name string, x []float64) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	data := make([]byte, 44+len(x)*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+len(x)*2))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(x)*2))
	for i, v := range x {
		if v > 1 {
			v = 1
		}
		if v < -1 {
			v = -1
		}
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(int16(v*32767)))
	}
	if _, err := f.Write(data); err != nil {
		panic(err)
	}
}
