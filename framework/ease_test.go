package golib

import "testing"

func TestLerpAndClamp(t *testing.T) {
	for _, tt := range []struct{ a, b, at, want float32 }{
		{0, 10, 0, 0}, {0, 10, 1, 10}, {0, 10, 0.5, 5}, {10, 0, 0.25, 7.5}, {0, 10, 2, 20},
	} {
		if got := Lerp(tt.a, tt.b, tt.at); got != tt.want {
			t.Errorf("Lerp(%v, %v, %v) = %v, want %v", tt.a, tt.b, tt.at, got, tt.want)
		}
	}
	for _, tt := range []struct{ value, low, high, want float32 }{
		{5, 0, 10, 5}, {-1, 0, 10, 0}, {11, 0, 10, 10}, {5, 10, 0, 5}, {-1, 10, 0, 0},
	} {
		if got := Clamp(tt.value, tt.low, tt.high); got != tt.want {
			t.Errorf("Clamp(%v, %v, %v) = %v, want %v", tt.value, tt.low, tt.high, got, tt.want)
		}
	}
}

func TestEasings(t *testing.T) {
	for name, ease := range map[string]func(float32) float32{"EaseIn": EaseIn, "EaseOut": EaseOut, "EaseInOut": EaseInOut} {
		if ease(0) != 0 || ease(1) != 1 {
			t.Errorf("%s: 0 gives %v and 1 gives %v, want 0 and 1", name, ease(0), ease(1))
		}
		if ease(-1) != 0 || ease(2) != 1 {
			t.Errorf("%s doesn't clamp: -1 gives %v and 2 gives %v", name, ease(-1), ease(2))
		}
		// Each one only goes forwards.
		last := float32(0)
		for step := range 11 {
			at := ease(float32(step) / 10)
			if at < last {
				t.Errorf("%s goes backwards at %v", name, float32(step)/10)
			}
			last = at
		}
	}
	// In starts slowly, out starts fast, and in-out is halfway at halfway.
	if EaseIn(0.5) >= 0.5 || EaseOut(0.5) <= 0.5 || EaseInOut(0.5) != 0.5 {
		t.Errorf("at halfway: in %v, out %v, in-out %v", EaseIn(0.5), EaseOut(0.5), EaseInOut(0.5))
	}
}
