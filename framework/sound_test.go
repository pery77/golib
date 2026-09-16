package golib

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestSoundSpecDefaults(t *testing.T) {
	spec := SoundSpec{}.resolve()
	want := SoundSpec{Wave: WaveSquare, Frequency: 440, Duration: 0.25, Attack: 0.005, Release: 0.05, Volume: 0.5, Duty: 0.5, VibratoRate: 12}
	if spec != want {
		t.Errorf("SoundSpec{}.resolve() = %+v, want %+v", spec, want)
	}

	loud := SoundSpec{Volume: 4, Duty: 9, Attack: 5, Release: 5, Duration: 0.1}.resolve()
	if loud.Volume != 1 || loud.Duty != 0.95 || loud.Attack != 0.1 || loud.Release != 0.1 {
		t.Errorf("out-of-range values resolved to %+v, want them kept in range", loud)
	}
}

func TestSamplesLengthAndEnvelope(t *testing.T) {
	spec := SoundSpec{Wave: WaveSine, Frequency: 440, Duration: 0.5, Attack: 0.02, Release: 0.05, Volume: 0.5}
	samples := spec.samples()
	if want := 0.5 * soundSampleRate; len(samples) != int(want) {
		t.Fatalf("half a second is %d samples, want %v", len(samples), want)
	}

	if samples[0] != 0 {
		t.Errorf("the first sample is %d, want 0: the sound fades in", samples[0])
	}
	peak := 0
	for _, sample := range samples {
		peak = max(peak, int(sample), -int(sample))
	}
	if want := math.MaxInt16 / 2; peak > want+1 || peak < want-1200 {
		t.Errorf("peak = %d, want about %d: Volume 0.5 is half as loud as it gets", peak, want)
	}
	if last := samples[len(samples)-1]; last > 400 || last < -400 {
		t.Errorf("the last sample is %d, want near 0: the sound fades out", last)
	}
}

func TestSineHasTheRightFrequency(t *testing.T) {
	// Half a second of 440 Hz is 220 waves, and every wave crosses zero twice.
	samples := SoundSpec{Wave: WaveSine, Frequency: 440, Duration: 0.5, Attack: 0.001, Release: 0.001}.samples()
	crossings := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i-1] < 0) != (samples[i] < 0) {
			crossings++
		}
	}
	if crossings < 435 || crossings > 445 {
		t.Errorf("440 Hz crossed zero %d times in half a second, want about 440", crossings)
	}
}

func TestSquareFollowsItsDuty(t *testing.T) {
	samples := SoundSpec{Wave: WaveSquare, Frequency: 500, Duration: 0.4, Duty: 0.2, Attack: 0.001, Release: 0.001}.samples()
	high := 0
	for _, sample := range samples {
		if sample > 0 {
			high++
		}
	}
	if share := float64(high) / float64(len(samples)); share < 0.17 || share > 0.23 {
		t.Errorf("Duty 0.2 kept the wave high %.0f%% of the time, want about 20%%", share*100)
	}
}

func TestNoiseIsTheSameEveryTime(t *testing.T) {
	spec := SoundSpec{Wave: WaveNoise, Frequency: 900, Duration: 0.2}
	if first, second := spec.samples(), spec.samples(); !bytes.Equal(int16Bytes(first), int16Bytes(second)) {
		t.Error("the same noise recipe gave different samples")
	}
}

func TestEveryRecipeMakesSound(t *testing.T) {
	recipes := map[string]*Sound{
		"Laser": Laser(), "Explosion": Explosion(), "Pickup": Pickup(),
		"Jump": Jump(), "Hurt": Hurt(), "PowerUp": PowerUp(),
	}
	for name, sound := range recipes {
		samples := sound.spec.samples()
		if len(samples) < soundSampleRate/20 {
			t.Errorf("%s lasts %d samples, want at least a twentieth of a second", name, len(samples))
		}
		peak := 0
		for _, sample := range samples {
			peak = max(peak, int(sample), -int(sample))
		}
		if peak < 1000 {
			t.Errorf("%s peaks at %d, want something audible", name, peak)
		}
	}
}

func TestWav(t *testing.T) {
	samples := []int16{0, 1000, -1000, 32767}
	file := wav(samples)
	if len(file) != 44+2*len(samples) {
		t.Fatalf("the file is %d bytes, want %d", len(file), 44+2*len(samples))
	}
	if string(file[0:4]) != "RIFF" || string(file[8:12]) != "WAVE" || string(file[12:16]) != "fmt " || string(file[36:40]) != "data" {
		t.Fatalf("the file doesn't look like a WAV: %q", file[:40])
	}
	if got := binary.LittleEndian.Uint32(file[4:8]); got != uint32(36+2*len(samples)) {
		t.Errorf("the RIFF size is %d, want %d", got, 36+2*len(samples))
	}
	if got := binary.LittleEndian.Uint16(file[22:24]); got != 1 {
		t.Errorf("channels = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(file[24:28]); got != soundSampleRate {
		t.Errorf("sample rate = %d, want %d", got, soundSampleRate)
	}
	if got := binary.LittleEndian.Uint16(file[34:36]); got != 16 {
		t.Errorf("bits per sample = %d, want 16", got)
	}
	if got := binary.LittleEndian.Uint32(file[40:44]); got != uint32(2*len(samples)) {
		t.Errorf("the data size is %d, want %d", got, 2*len(samples))
	}
	if got := int16(binary.LittleEndian.Uint16(file[46:48])); got != 1000 {
		t.Errorf("the second sample is %d, want 1000", got)
	}
}

func TestPlayWithoutASoundDevice(t *testing.T) {
	// Tests and golib shot run with no sound device: playing must do nothing.
	if audio.isReady() {
		t.Skip("a sound device is open")
	}
	sound := NewSound(SoundSpec{})
	sound.Play()
	if len(sound.voices) != 0 {
		t.Error("Play made the sound without a device")
	}
	SetVolume(0.3)
	t.Cleanup(func() { SetVolume(1) })
}

// int16Bytes turns samples into bytes, so tests can compare them.
func int16Bytes(samples []int16) []byte {
	data := make([]byte, 0, 2*len(samples))
	for _, sample := range samples {
		data = binary.LittleEndian.AppendUint16(data, uint16(sample))
	}
	return data
}
