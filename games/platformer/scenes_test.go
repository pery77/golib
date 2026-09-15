package main

import (
	"testing"

	"golib"
)

func TestButtonAt(t *testing.T) {
	buttons := []menuButton{
		{label: "Play", bounds: golib.Rectangle{X: 10, Y: 10, Width: 100, Height: 40}},
		{label: "Quit", bounds: golib.Rectangle{X: 10, Y: 60, Width: 100, Height: 40}},
	}
	tests := []struct {
		name string
		x, y float32
		want int
	}{
		{name: "on the first button", x: 50, y: 30, want: 0},
		{name: "on the second button", x: 50, y: 80, want: 1},
		{name: "between the buttons", x: 50, y: 55, want: -1},
		{name: "just right of a button", x: 110, y: 30, want: -1},
	}
	for _, tt := range tests {
		if got := buttonAt(buttons, tt.x, tt.y); got != tt.want {
			t.Errorf("%s: buttonAt(%v, %v) = %d, want %d", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
}

func TestNewCloudsRepeatWithTheSameSeed(t *testing.T) {
	golib.SetRandomSeed(1)
	first := newClouds()
	golib.SetRandomSeed(1)
	second := newClouds()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("cloud %d differs with the same seed: %+v, then %+v", i, first[i], second[i])
		}
	}
}

func TestMoveSelection(t *testing.T) {
	tests := []struct {
		selected, step, want int
	}{
		{selected: 0, step: 1, want: 1},
		{selected: 1, step: 1, want: 1},
		{selected: 1, step: -1, want: 0},
		{selected: 0, step: -1, want: 0},
		{selected: 1, step: 0, want: 1},
	}
	for _, tt := range tests {
		if got := moveSelection(tt.selected, tt.step, 2); got != tt.want {
			t.Errorf("moveSelection(%d, %d, 2) = %d, want %d", tt.selected, tt.step, got, tt.want)
		}
	}
}
