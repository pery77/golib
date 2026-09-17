package golib

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The files in testdata/jfxr come from make.go, which runs jfxr's own
// synthesizer in a browser: each <name>.wav is the sound jfxr exports for
// <name>.jfxr. The preset-<name>.jfxr files are sounds from jfxr's presets,
// saved as jfxr saves them.
func TestJfxrMatchesJfxr(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "jfxr", "*.jfxr"))
	if err != nil || len(names) < 20 {
		t.Fatalf("found %d .jfxr files in testdata/jfxr, want 20 or more (%v)", len(names), err)
	}
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(strings.TrimSuffix(name, ".jfxr") + ".wav")
		if err != nil {
			t.Fatal(err)
		}
		sound, err := parseJfxr(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		got := wav(sound.samples())
		if bytes.Equal(got, want) {
			continue
		}
		if len(got) != len(want) || !bytes.Equal(got[:44], want[:44]) {
			t.Errorf("%s: GoLib made a %d-byte WAV file, jfxr a %d-byte one, with other headers", name, len(got), len(want))
			continue
		}
		differ, first, most := 0, -1, 0
		for i := 44; i < len(got); i += 2 {
			a := int(int16(binary.LittleEndian.Uint16(got[i:])))
			b := int(int16(binary.LittleEndian.Uint16(want[i:])))
			if a != b {
				differ++
				if first < 0 {
					first = (i - 44) / 2
				}
				most = max(most, abs(a-b))
			}
		}
		t.Errorf("%s: %d of %d samples differ from jfxr's, from sample %d, by up to %d", name, differ, (len(got)-44)/2, first, most)
	}
}

func TestJfxrSettings(t *testing.T) {
	sound, err := parseJfxr([]byte(`{"sustain": 0.25}`))
	if err != nil {
		t.Fatal(err)
	}
	if sound.frequency != 500 || sound.waveform != jfxrSine || !sound.normalization || !sound.interpolateNoise ||
		sound.bitCrush != 16 || sound.lowPassCutoff != 22050 || sound.amplification != 100 || sound.compression != 1 {
		t.Errorf("defaults = %+v, want jfxr's", sound)
	}
	if got := len(sound.samples()); got != 11025 {
		t.Errorf("a quarter of a second has %d samples, want 11025", got)
	}

	sound, err = parseJfxr([]byte(`{"_version": 1, "_name": "Hit", "_locked": ["frequency"], "sampleRate": 8000,
		"decay": 9, "frequency": 1, "harmonics": 2.5, "harmonicsFalloff": -1, "waveform": "pinknoise",
		"normalization": false, "interpolateNoise": false}`))
	if err != nil {
		t.Fatal(err)
	}
	if sound.decay != 5 || sound.frequency != 10 || sound.harmonics != 3 || sound.harmonicsFalloff != 0 || sound.sampleRate != 44100 {
		t.Errorf("decay %v, frequency %v, harmonics %v, falloff %v, sample rate %v: want 5, 10, 3, 0 and 44100, kept in range as jfxr keeps them",
			sound.decay, sound.frequency, sound.harmonics, sound.harmonicsFalloff, sound.sampleRate)
	}
	if sound.waveform != jfxrPinkNoise || sound.normalization || sound.interpolateNoise {
		t.Errorf("waveform %v, normalization %v, noise interpolation %v: want pink noise, off and off", sound.waveform, sound.normalization, sound.interpolateNoise)
	}
	if sound.cycleRate != 1.0/5 {
		t.Errorf("a 5-second sound repeats %v times a second, want 0.2", sound.cycleRate)
	}
}

func TestJfxrMistakes(t *testing.T) {
	tests := map[string]string{
		`[1, 2]`:                                   "the file isn't a jfxr sound, which is a JSON object: json: cannot unmarshal array",
		`{"sustain": 0.1`:                          "the file isn't a jfxr sound, which is a JSON object: unexpected end of JSON input",
		`null`:                                     "the file isn't a jfxr sound, which is a JSON object: it holds null",
		`{"_version": 2, "sustain": 1}`:            "a newer jfxr saved the sound, in file version 2, and GoLib reads up to version 1",
		`{"_version": "1", "sustain": 1}`:          "_version is text, want a number",
		`{"volume": 0.5, "sustain": 1}`:            `jfxr has no setting called "volume" (framework/README.md lists them)`,
		`{"sustain": "1"}`:                         "sustain is text, want a number",
		`{"sustain": 1, "harmonics": []}`:          "harmonics is a list, want a number",
		`{"sustain": 1, "waveform": "saw"}`:        `waveform is "saw", want one of sine, triangle, sawtooth, square, tangent, whistle, breaker, whitenoise, pinknoise, brownnoise`,
		`{"sustain": 1, "waveform": 3}`:            "waveform is 3, want one of sine,",
		`{"sustain": 1, "normalization": 1}`:       "normalization is a number, want true or false",
		`{"sustain": 1, "interpolateNoise": null}`: "interpolateNoise is null, want true or false",
		`{"_name": "Nothing"}`:                     "the sound lasts 0 seconds: give it an attack, a sustain or a decay in jfxr",
		`{"attack": -2, "decay": 0}`:               "the sound lasts 0 seconds",
	}
	for file, want := range tests {
		_, err := parseJfxr([]byte(file))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseJfxr(%s) = %v, want an error containing %q", file, err, want)
		}
	}
}

func TestJsRound(t *testing.T) {
	for _, test := range []struct{ in, want float64 }{
		{2.5, 3}, {-2.5, -2}, {2.4, 2}, {-2.6, -3}, {0.49999999999999994, 0},
		{-0.5, 0}, {-0.5000000000000001, -1}, {1e300, 1e300}, {-7, -7},
	} {
		if got := jsRound(test.in); got != test.want {
			t.Errorf("jsRound(%v) = %v, want %v", test.in, got, test.want)
		}
	}
	if got := jsRound(math.NaN()); !math.IsNaN(got) {
		t.Errorf("jsRound(NaN) = %v, want NaN", got)
	}
}

func TestJfxrRandom(t *testing.T) {
	// The first numbers of jfxr's Random(0x3cf78ba3).uint32(), from jfxr's
	// random.js.
	r := newJfxrRandom(0x3cf78ba3)
	for i, want := range []float64{91492987, 1477143755, 3546181110} {
		if got := r.next(); got != want {
			t.Errorf("number %d = %v, want %v", i, got, want)
		}
	}
}
