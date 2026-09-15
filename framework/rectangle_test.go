package golib

import "testing"

func TestRectangleOverlaps(t *testing.T) {
	base := Rectangle{X: 0, Y: 0, Width: 10, Height: 10}
	tests := []struct {
		name  string
		other Rectangle
		want  bool
	}{
		{"partly overlapping", Rectangle{X: 5, Y: 5, Width: 10, Height: 10}, true},
		{"inside", Rectangle{X: 2, Y: 2, Width: 2, Height: 2}, true},
		{"touching the right edge", Rectangle{X: 10, Y: 0, Width: 10, Height: 10}, false},
		{"touching the bottom edge", Rectangle{X: 0, Y: 10, Width: 10, Height: 10}, false},
		{"apart", Rectangle{X: 20, Y: 20, Width: 5, Height: 5}, false},
	}
	for _, tt := range tests {
		if got := base.Overlaps(tt.other); got != tt.want {
			t.Errorf("%s: Overlaps = %v, want %v", tt.name, got, tt.want)
		}
		if got := tt.other.Overlaps(base); got != tt.want {
			t.Errorf("%s, reversed: Overlaps = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRectangleContains(t *testing.T) {
	r := Rectangle{X: 10, Y: 20, Width: 30, Height: 40}
	tests := []struct {
		name string
		x, y float32
		want bool
	}{
		{name: "inside", x: 25, y: 40, want: true},
		{name: "top-left corner", x: 10, y: 20, want: true},
		{name: "on the right edge", x: 40, y: 40, want: false},
		{name: "on the bottom edge", x: 25, y: 60, want: false},
		{name: "left of it", x: 9, y: 40, want: false},
		{name: "above it", x: 25, y: 19, want: false},
	}
	for _, tt := range tests {
		if got := r.Contains(tt.x, tt.y); got != tt.want {
			t.Errorf("%s: Contains(%v, %v) = %v, want %v", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
}
