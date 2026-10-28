package main

import (
	"math"

	"golib"
)

const (
	facingRight uint8 = iota
	facingDownRight
	facingDown
	facingDownLeft
	facingLeft
	facingUpLeft
	facingUp
	facingUpRight
)

// facingFromMovement quantizes the flat isometric projection, not world axes.
func facingFromMovement(dx, dy float64) uint8 {
	screenX := (dx - dy) * float64(unitW) / 2
	screenY := (dx + dy) * float64(unitH) / 2
	sector := int(math.Round(math.Atan2(screenY, screenX) / (math.Pi / 4)))
	return uint8((sector + 8) % 8)
}

func facingAngle(facing uint8) float32 {
	return float32(facing%8) * 45
}

// groundFacingBasis turns the stored screen heading back into a world-space
// yaw. The stored octants are still screen-facing so old saves keep working.
func groundFacingBasis(facing uint8) (forward, side golib.Vector2) {
	angle := float64(facingAngle(facing)) * math.Pi / 180
	screenForward := golib.Vector2{
		X: float32(math.Cos(angle)),
		Y: float32(math.Sin(angle)),
	}
	forward = golib.Vector2{
		X: screenForward.X/unitW + screenForward.Y/unitH,
		Y: screenForward.Y/unitH - screenForward.X/unitW,
	}.Normalize()
	side = golib.Vector2{X: -forward.Y, Y: forward.X}
	return forward, side
}

// projectGroundDelta projects a flat-world offset without applying the
// region origin or its relief. The unit's center already has both applied.
func projectGroundDelta(delta golib.Vector2) golib.Vector2 {
	return golib.Vector2{
		X: (delta.X - delta.Y) * unitW / 2,
		Y: (delta.X + delta.Y) * unitH / 2,
	}
}

// groundFacingPoint rotates a unit footprint around the vertical world axis
// and projects it onto the ground instead of rotating it in screen space.
func groundFacingPoint(
	center golib.Vector2,
	forwardDistance, sideDistance, size float32,
	facing uint8,
) golib.Vector2 {
	forward, side := groundFacingBasis(facing)
	worldSize := size / unitW
	delta := forward.Scale(forwardDistance * worldSize).Add(
		side.Scale(sideDistance * worldSize),
	)
	return center.Add(projectGroundDelta(delta))
}
