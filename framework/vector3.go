package golib

import "math"

// Vector3 is a point, or a direction, in a 3D world: X and Z across the
// ground, Y up. One unit is a meter, as in Blender and glTF. Its methods return
// a new vector and leave v as it is, as [Vector2]'s do:
//
//	velocity = velocity.Add(golib.Vector3{Y: -gravity * dt})
//	position = position.Add(velocity.Scale(dt))
//
// The world is right-handed: seen from above, with X to the right, Z points
// towards the viewer. A model's front faces +Z, as in glTF; [Rotation] turns
// it.
type Vector3 struct {
	X, Y, Z float32
}

// Add returns v moved by other.
func (v Vector3) Add(other Vector3) Vector3 {
	return Vector3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

// Sub returns v minus other: the way from other to v, when both are points.
func (v Vector3) Sub(other Vector3) Vector3 {
	return Vector3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

// Scale returns v with every part multiplied by factor.
func (v Vector3) Scale(factor float32) Vector3 {
	return Vector3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

// Length returns how long v is.
func (v Vector3) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Distance returns how far the point v is from the point other.
func (v Vector3) Distance(other Vector3) float32 {
	return v.Sub(other).Length()
}

// Dot returns the dot product of v and other: positive when they point the
// same way, 0 when they are at right angles, negative when they point apart.
func (v Vector3) Dot(other Vector3) float32 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the vector at right angles to both v and other, as long as
// the area of the parallelogram they make: the up of a surface whose sides
// go along v and then other, counterclockwise as seen from that up.
func (v Vector3) Cross(other Vector3) Vector3 {
	return Vector3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

// Normalize returns the vector of length 1 that points the same way as v, or
// the zero vector when v is zero. Use it to turn a direction into a speed:
// direction.Normalize().Scale(speed).
func (v Vector3) Normalize() Vector3 {
	length := v.Length()
	if length == 0 {
		return Vector3{}
	}
	return v.Scale(1 / length)
}

// ClampLength returns v, shortened to max if it is longer, pointing the same
// way. Use it so that moving diagonally isn't faster than moving straight, or
// to cap a speed.
func (v Vector3) ClampLength(max float32) Vector3 {
	length := v.Length()
	if length <= max || length == 0 {
		return v
	}
	return v.Scale(max / length)
}

// MoveTowards returns the point v moved towards target by at most
// maxDistance, stopping at target.
func (v Vector3) MoveTowards(target Vector3, maxDistance float32) Vector3 {
	way := target.Sub(v)
	distance := way.Length()
	if distance <= maxDistance || distance == 0 {
		return target
	}
	return v.Add(way.Scale(maxDistance / distance))
}

// Lerp returns the point a share t of the way from v to target: v when t is
// 0, target when t is 1, and halfway when t is 0.5.
func (v Vector3) Lerp(target Vector3, t float32) Vector3 {
	return Vector3{X: v.X + (target.X-v.X)*t, Y: v.Y + (target.Y-v.Y)*t, Z: v.Z + (target.Z-v.Z)*t}
}

// Rotate returns v turned by rotation, around 0, 0, 0: what a direction of a
// model becomes once the model is turned.
func (v Vector3) Rotate(rotation Rotation) Vector3 {
	return rotation.matrix().transformDirection(v)
}

// Rotation is how something is turned, in degrees, as a pilot would say it.
// The zero Rotation leaves a model as it was made, facing +Z.
//
//	ship.Rotation.Yaw += turnSpeed * dt          // steer
//	forward := ship.Rotation.Forward()           // the way the ship faces
//	ship.Position = ship.Position.Add(forward.Scale(speed * dt))
//
// The three turns happen in the order Roll, Pitch, Yaw, around the model's
// own front, its own sideways line and the world's up.
type Rotation struct {
	// Yaw turns left and right, around the world's up: 0 faces +Z, 90 faces
	// +X, 180 faces -Z and -90 faces -X. Seen from behind, from above, a
	// larger Yaw turns left, because facing +Z the left side is +X.
	Yaw float32

	// Pitch tilts the front up or down: 90 faces straight up, -90 straight
	// down.
	Pitch float32

	// Roll banks around the front: positive lowers the right side, as a
	// plane rolls right.
	Roll float32
}

// RotationTowards returns the Rotation that turns a model's front, +Z, to
// face direction: the Yaw and the Pitch, with no Roll. A zero direction
// returns the zero Rotation.
//
//	turret.Rotation = golib.RotationTowards(enemy.Position.Sub(turret.Position))
func RotationTowards(direction Vector3) Rotation {
	if direction == (Vector3{}) {
		return Rotation{}
	}
	ground := math.Hypot(float64(direction.X), float64(direction.Z))
	return Rotation{
		Yaw:   float32(math.Atan2(float64(direction.X), float64(direction.Z)) * 180 / math.Pi),
		Pitch: float32(math.Atan2(float64(direction.Y), ground) * 180 / math.Pi),
	}
}

// Forward returns the direction a model turned by r faces, length 1: its
// front, +Z, turned.
func (r Rotation) Forward() Vector3 {
	return Vector3{Z: 1}.Rotate(r)
}

// Right returns the direction of a model's right side once turned by r,
// length 1. Facing +Z, with Y up, the right side is -X.
func (r Rotation) Right() Vector3 {
	return Vector3{X: -1}.Rotate(r)
}

// Up returns the direction of a model's top once turned by r, length 1.
func (r Rotation) Up() Vector3 {
	return Vector3{Y: 1}.Rotate(r)
}

// Transform places something in the world: where it is, how it is turned,
// and how large it is. Scale multiplies each of the model's own sizes, and a
// part of it left at 0 counts as 1: a Transform with only a Position draws
// the model as it was made, and Scale{X: 40, Z: 40} stretches a plane over
// the ground. To hide something, don't draw it.
//
//	screen.DrawMesh(s.box, golib.Transform{Position: crate.Position, Scale: golib.Vector3{X: 2, Y: 1, Z: 1}}, golib.Brown)
type Transform struct {
	Position Vector3
	Rotation Rotation
	Scale    Vector3
}

// matrix returns the transform as a matrix: scaled, then turned, then moved.
func (t Transform) matrix() matrix {
	scale := t.Scale
	for _, part := range []*float32{&scale.X, &scale.Y, &scale.Z} {
		if *part == 0 {
			*part = 1
		}
	}
	m := t.Rotation.matrix()
	for i := 0; i < 3; i++ {
		m[i] *= scale.X
		m[4+i] *= scale.Y
		m[8+i] *= scale.Z
	}
	m[12], m[13], m[14] = t.Position.X, t.Position.Y, t.Position.Z
	return m
}
