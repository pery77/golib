package golib

// The small numbers game code writes again and again: moving a value towards
// another one, keeping one inside its limits, and the curves that make a
// movement start or stop gently instead of snapping.

// Lerp returns the number a share t of the way from a to b: a when t is 0, b
// when t is 1, and halfway when t is 0.5. Outside 0 to 1 it carries on past
// the ends, so clamp t first when that matters. With a curve, the movement
// starts or stops gently:
//
//	x := golib.Lerp(doorClosedX, doorOpenX, golib.EaseInOut(s.open.Progress()))
//
// [Vector2.Lerp] does the same for a point.
func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// Clamp returns value kept inside low to high: low when it is lower, high
// when it is higher, and value itself in between. The two swap if they come
// the wrong way round.
//
//	s.player.X = golib.Clamp(s.player.X, 0, screen.Width()-playerWidth)
func Clamp(value, low, high float32) float32 {
	if low > high {
		low, high = high, low
	}
	return max(low, min(value, high))
}

// EaseIn turns a share of the way from 0 to 1 into one that starts slowly and
// speeds up, for something setting off. Shares outside 0 to 1 are clamped.
func EaseIn(t float32) float32 {
	t = Clamp(t, 0, 1)
	return t * t
}

// EaseOut turns a share of the way from 0 to 1 into one that starts fast and
// slows down, for something coming to rest. It is the usual choice for menus
// and pop-ups. Shares outside 0 to 1 are clamped.
func EaseOut(t float32) float32 {
	t = Clamp(t, 0, 1)
	return 1 - (1-t)*(1-t)
}

// EaseInOut turns a share of the way from 0 to 1 into one that starts slowly,
// speeds up in the middle and slows down again, for a camera or a door that
// should not snap. Shares outside 0 to 1 are clamped.
func EaseInOut(t float32) float32 {
	t = Clamp(t, 0, 1)
	return t * t * (3 - 2*t)
}
