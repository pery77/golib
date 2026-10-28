package main

import (
	"math"
	"testing"
)

func TestMineralTinkHasAmplitudeModulation(t *testing.T) {
	samples := mineralTink()
	low := rms(samples, 0.115, 0.14)
	high := rms(samples, 0.195, 0.22)
	if low >= high*0.55 {
		t.Fatalf("tink pulse is too even: low %g, high %g", low, high)
	}
}

func rms(samples []float64, start, end float64) float64 {
	first := int(start * rate)
	last := int(end * rate)
	sum := 0.0
	for _, sample := range samples[first:last] {
		sum += sample * sample
	}
	return math.Sqrt(sum / float64(last-first))
}
