package golib

import "testing"

func TestSetRandomSeedRepeatsNumbers(t *testing.T) {
	draw := func() [5]int {
		var numbers [5]int
		for i := range numbers {
			numbers[i] = RandomInt(1, 1000)
		}
		return numbers
	}
	SetRandomSeed(42)
	first := draw()
	SetRandomSeed(42)
	if second := draw(); second != first {
		t.Errorf("the same seed gave %v, then %v", first, second)
	}
}

func TestRandomIntStaysInRange(t *testing.T) {
	SetRandomSeed(7)
	seen := map[int]bool{}
	for range 1000 {
		n := RandomInt(3, 5)
		if n < 3 || n > 5 {
			t.Fatalf("RandomInt(3, 5) = %d", n)
		}
		seen[n] = true
	}
	if len(seen) != 3 {
		t.Errorf("RandomInt(3, 5) gave only %v in 1000 draws, want 3, 4 and 5", seen)
	}
	if n := RandomInt(5, 3); n < 3 || n > 5 {
		t.Errorf("RandomInt(5, 3) = %d, want the bounds to swap", n)
	}
	if n := RandomInt(7, 7); n != 7 {
		t.Errorf("RandomInt(7, 7) = %d", n)
	}
}

func TestRandomFloatStaysInRange(t *testing.T) {
	SetRandomSeed(7)
	for range 1000 {
		if f := RandomFloat(2, 4); f < 2 || f >= 4 {
			t.Fatalf("RandomFloat(2, 4) = %v", f)
		}
	}
}

func TestStartSeed(t *testing.T) {
	shot := func(key string) string {
		if key == shotFramesEnv {
			return "60"
		}
		return ""
	}
	if got := startSeed(shot); got != shotSeed {
		t.Errorf("startSeed() under golib shot = %d, want %d", got, shotSeed)
	}
	normal := func(string) string { return "" }
	if startSeed(normal) == startSeed(normal) {
		t.Error("two normal runs got the same seed")
	}
}
