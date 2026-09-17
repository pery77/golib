package main

import "math"

// The little maths GoLib's golib.Vector2 and golib.Rectangle don't cover.
// Lengths, directions, angles, turning and distances are all in the
// framework: see framework/README.md, "Vectors, rectangles and collisions".

// clamp returns value, kept between low and high.
func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}

// abs returns value without its sign.
func abs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

// wrap returns value moved into 0 to size, for patterns that repeat.
func wrap(value, size float32) float32 {
	value = float32(math.Mod(float64(value), float64(size)))
	if value < 0 {
		value += size
	}
	return value
}
