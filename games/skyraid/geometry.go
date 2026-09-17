package main

import "math"

// Small float32 helpers for positions and directions. GoLib has no vector
// math yet (it is planned for M6), so the game keeps its own here.

// length returns the length of the vector x, y.
func length(x, y float32) float32 {
	return float32(math.Hypot(float64(x), float64(y)))
}

// normalize returns x, y scaled to length 1, or 0, 0 for a zero vector.
func normalize(x, y float32) (float32, float32) {
	l := length(x, y)
	if l == 0 {
		return 0, 0
	}
	return x / l, y / l
}

// limit returns x, y shortened to length at most max.
func limit(x, y, max float32) (float32, float32) {
	if l := length(x, y); l > max {
		return x * max / l, y * max / l
	}
	return x, y
}

// distanceSquared returns the squared distance between two points, which is
// enough to compare distances.
func distanceSquared(x1, y1, x2, y2 float32) float32 {
	dx, dy := x2-x1, y2-y1
	return dx*dx + dy*dy
}

// circlesTouch reports whether two circles overlap.
func circlesTouch(x1, y1, r1, x2, y2, r2 float32) bool {
	reach := r1 + r2
	return distanceSquared(x1, y1, x2, y2) < reach*reach
}

// direction returns the unit vector of an angle in radians. Angle 0 points
// right, and angles grow clockwise on the screen, because y grows downwards.
func direction(angle float32) (float32, float32) {
	return float32(math.Cos(float64(angle))), float32(math.Sin(float64(angle)))
}

// angleOf returns the angle of the vector x, y, as direction measures it.
func angleOf(x, y float32) float32 {
	return float32(math.Atan2(float64(y), float64(x)))
}

// rotate turns the point x, y around 0, 0 by angle radians, clockwise on the
// screen.
func rotate(x, y, angle float32) (float32, float32) {
	sin, cos := float32(math.Sin(float64(angle))), float32(math.Cos(float64(angle)))
	return x*cos - y*sin, x*sin + y*cos
}

// approach moves value towards target by the share rate*dt of the gap, which
// eases in smoothly at any rate. rate is per second.
func approach(value, target, rate, dt float32) float32 {
	return value + (target-value)*min(1, rate*dt)
}

// clamp returns value, kept between low and high.
func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}

// wrap returns value moved into 0 to size, for patterns that repeat.
func wrap(value, size float32) float32 {
	value = float32(math.Mod(float64(value), float64(size)))
	if value < 0 {
		value += size
	}
	return value
}
