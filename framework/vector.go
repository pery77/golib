package golib

import "math"

// Vector2 is a point, or a direction, in pixels, with Y growing downwards as
// on the screen. Its methods return a new vector and leave v as it is:
//
//	velocity = velocity.Add(direction.Scale(acceleration * dt)).ClampLength(maxSpeed)
//	position = position.Add(velocity.Scale(dt))
//
// Angles are in degrees, clockwise from pointing right, as in
// [DrawOptions].Rotation: 90 points down the screen.
type Vector2 struct {
	X, Y float32
}

// Vector2FromAngle returns the vector of length 1 that points at angle
// degrees: 0 is right, 90 is down, 180 is left and -90 is up.
func Vector2FromAngle(degrees float32) Vector2 {
	radians := float64(degrees) * math.Pi / 180
	return Vector2{X: float32(math.Cos(radians)), Y: float32(math.Sin(radians))}
}

// Add returns v moved by other.
func (v Vector2) Add(other Vector2) Vector2 {
	return Vector2{X: v.X + other.X, Y: v.Y + other.Y}
}

// Sub returns v minus other: the way from other to v, when both are points.
func (v Vector2) Sub(other Vector2) Vector2 {
	return Vector2{X: v.X - other.X, Y: v.Y - other.Y}
}

// Scale returns v with both parts multiplied by factor.
func (v Vector2) Scale(factor float32) Vector2 {
	return Vector2{X: v.X * factor, Y: v.Y * factor}
}

// Length returns how long v is.
func (v Vector2) Length() float32 {
	return float32(math.Hypot(float64(v.X), float64(v.Y)))
}

// Distance returns how far the point v is from the point other.
func (v Vector2) Distance(other Vector2) float32 {
	return v.Sub(other).Length()
}

// Dot returns the dot product of v and other: positive when they point the
// same way, 0 when they are at right angles, negative when they point apart.
func (v Vector2) Dot(other Vector2) float32 {
	return v.X*other.X + v.Y*other.Y
}

// Normalize returns the vector of length 1 that points the same way as v, or
// the zero vector when v is zero. Use it to turn a direction into a speed:
// direction.Normalize().Scale(speed).
func (v Vector2) Normalize() Vector2 {
	length := v.Length()
	if length == 0 {
		return Vector2{}
	}
	return v.Scale(1 / length)
}

// ClampLength returns v, shortened to max if it is longer, pointing the same
// way. Use it so that moving diagonally isn't faster than moving straight, or
// to cap a speed.
func (v Vector2) ClampLength(max float32) Vector2 {
	length := v.Length()
	if length <= max || length == 0 {
		return v
	}
	return v.Scale(max / length)
}

// Angle returns the direction v points at, in degrees from -180 to 180: 0 is
// right, 90 is down, -90 is up. The zero vector points at 0.
func (v Vector2) Angle() float32 {
	return float32(math.Atan2(float64(v.Y), float64(v.X)) * 180 / math.Pi)
}

// Rotate returns v turned clockwise by degrees, around 0, 0.
func (v Vector2) Rotate(degrees float32) Vector2 {
	radians := float64(degrees) * math.Pi / 180
	sin, cos := float32(math.Sin(radians)), float32(math.Cos(radians))
	return Vector2{X: v.X*cos - v.Y*sin, Y: v.X*sin + v.Y*cos}
}

// MoveTowards returns the point v moved towards target by at most
// maxDistance, stopping at target: an enemy chasing the player moves
// enemy.MoveTowards(player, speed*dt).
func (v Vector2) MoveTowards(target Vector2, maxDistance float32) Vector2 {
	way := target.Sub(v)
	distance := way.Length()
	if distance <= maxDistance || distance == 0 {
		return target
	}
	return v.Add(way.Scale(maxDistance / distance))
}

// Lerp returns the point a share t of the way from v to target: v when t is
// 0, target when t is 1, and halfway when t is 0.5.
func (v Vector2) Lerp(target Vector2, t float32) Vector2 {
	return Vector2{X: v.X + (target.X-v.X)*t, Y: v.Y + (target.Y-v.Y)*t}
}
