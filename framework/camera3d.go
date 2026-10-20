package golib

import (
	"fmt"
	"math"
)

// Camera3D is the eye a 3D world is seen through. Make it once, with the
// screen's size, place it in Update, and draw the world through it in Draw:
//
//	camera := golib.NewCamera3D(screenWidth, screenHeight)
//	camera.Lag = 0.15
//
//	// In Update: behind the player, a little above, looking down at it.
//	s.camera.Orbit(s.player.Position, s.player.Rotation.Yaw, -20, 6)
//	s.camera.Update(dt)
//
//	// In Draw:
//	screen.Begin3D(s.camera)
//	screen.DrawMesh(s.box, golib.Transform{Position: s.player.Position}, golib.Red)
//	screen.End3D()
//
// Position and Target say where the camera should be. Without Lag the view is
// there at once, and Update is needed only for Shake.
type Camera3D struct {
	// Position is where the eye is, and Target the point it looks at.
	Position Vector3
	Target   Vector3

	// FieldOfView is how many degrees the camera sees from the bottom of the
	// screen to the top: 60 is usual, less zooms in. 0 counts as 60. With
	// Orthographic, it is instead how many meters of the world the screen
	// shows from bottom to top.
	FieldOfView float32

	// Orthographic draws without perspective: far things are as large as
	// near ones, as in an isometric or a top-down game.
	Orthographic bool

	// Near and Far are the closest and the farthest distances the camera
	// draws. 0 counts as 0.1 and 1000 meters.
	Near, Far float32

	// Lag smooths the view's movement: it closes about two thirds of the way
	// to Position and Target in Lag seconds. 0 follows them exactly.
	Lag float32

	width, height float32 // the screen's size
	started       bool    // Update has placed the view since Lag was set
	eye, look     Vector3 // the view, as Lag lets it follow Position and Target
	shakeStrength float32
	shakeLength   float32
	shakeLeft     float32
	shakeOffset   Vector3
}

// NewCamera3D returns a camera for a screen width by height pixels, the size
// in [Config]. It starts 8 meters back from 0, 0, 0 and 4 meters up, looking
// at it towards +Z, the way the zero [Rotation] faces.
func NewCamera3D(width, height float32) *Camera3D {
	if width <= 0 || height <= 0 {
		reportError(fmt.Errorf("golib.NewCamera3D: the screen size is %g by %g: pass the screen's width and height, as in Config", width, height))
		width, height = defaultWidth, defaultHeight
	}
	return &Camera3D{Position: Vector3{Y: 4, Z: -8}, FieldOfView: 60, width: width, height: height}
}

// Orbit places the camera distance meters from center, looking at it, facing
// yaw and pitch as a [Rotation] does: the yaw a player faces puts the camera
// behind them, and a negative pitch looks down from above.
func (c *Camera3D) Orbit(center Vector3, yaw, pitch, distance float32) {
	c.Target = center
	c.Position = center.Sub(Rotation{Yaw: yaw, Pitch: pitch}.Forward().Scale(distance))
}

// Look places the eye at position, looking yaw and pitch degrees as a
// [Rotation] does: the camera of a first-person game.
func (c *Camera3D) Look(position Vector3, yaw, pitch float32) {
	c.Position = position
	c.Target = position.Add(Rotation{Yaw: yaw, Pitch: pitch}.Forward())
}

// Update moves the view towards Position and Target, by Lag, and moves the
// shake on. Call it once in every Update of the scene that has the camera,
// after placing it.
func (c *Camera3D) Update(dt float32) {
	if c.Lag > 0 && c.started {
		keep := float32(math.Exp(float64(-dt / c.Lag)))
		c.eye = c.Position.Lerp(c.eye, keep)
		c.look = c.Target.Lerp(c.look, keep)
	} else {
		c.eye, c.look = c.Position, c.Target
	}
	c.started = c.Lag > 0

	c.shakeOffset = Vector3{}
	if c.shakeLeft > 0 {
		c.shakeLeft = max(0, c.shakeLeft-dt)
		reach := c.shakeStrength * c.shakeLeft / c.shakeLength
		c.shakeOffset = Vector3{X: RandomFloat(-reach, reach), Y: RandomFloat(-reach, reach), Z: RandomFloat(-reach, reach)}
	}
}

// Snap puts the view on Position and Target at once, without Lag: at the
// start of a level, or after the player moves to another place.
func (c *Camera3D) Snap() {
	c.eye, c.look = c.Position, c.Target
	c.started = c.Lag > 0
}

// Shake shakes the view for seconds, by up to strength meters at first,
// fading out to none, such as for an explosion. A shake replaces one that
// would move the view less. Call it from Update, which moves it on.
func (c *Camera3D) Shake(strength, seconds float32) {
	if strength <= 0 || seconds <= 0 {
		return
	}
	if c.shakeLeft > 0 && c.shakeStrength*c.shakeLeft/c.shakeLength > strength {
		return
	}
	c.shakeStrength, c.shakeLength, c.shakeLeft = strength, seconds, seconds
}

// view returns where the eye is and what it looks at, as the camera draws
// them: following Position and Target by Lag, and shaken.
func (c *Camera3D) view() (eye, look Vector3) {
	eye, look = c.Position, c.Target
	if c.Lag > 0 && c.started {
		eye, look = c.eye, c.look
	}
	return eye.Add(c.shakeOffset), look.Add(c.shakeOffset)
}

// Forward returns the direction the camera looks, length 1.
func (c *Camera3D) Forward() Vector3 {
	eye, look := c.view()
	return look.Sub(eye).Normalize()
}

// Right returns the direction to the right of the screen, flat on the
// ground, length 1. With Forward flattened the same way, it turns a stick or
// WASD into a walk that follows the camera:
//
//	forward := s.camera.Forward()
//	forward.Y = 0
//	move := forward.Normalize().Scale(-stickY).Add(s.camera.Right().Scale(stickX))
func (c *Camera3D) Right() Vector3 {
	forward := c.Forward()
	right := Vector3{X: -forward.Z, Z: forward.X}.Normalize()
	if right == (Vector3{}) {
		return Vector3{X: 1} // looking straight down or up
	}
	return right
}

// matrices returns the view and the projection the camera draws with.
func (c *Camera3D) matrices() (view, projection matrix) {
	eye, look := c.view()
	near, far := c.Near, c.Far
	if near <= 0 {
		near = 0.1
	}
	if far <= near {
		far = max(1000, near*2)
	}
	fov := c.FieldOfView
	aspect := c.width / c.height
	if c.Orthographic {
		if fov <= 0 {
			fov = 10
		}
		projection = orthographicMatrix(fov, aspect, near, far)
	} else {
		if fov <= 0 || fov >= 180 {
			fov = 60
		}
		projection = perspectiveMatrix(fov, aspect, near, far)
	}
	return lookAtMatrix(eye, look), projection
}

// ToScreen returns where point, in the world, is on the screen, in pixels,
// to draw something over it, such as a name over a character's head. It
// returns false when the point is behind the camera, where nothing should
// be drawn for it.
func (c *Camera3D) ToScreen(point Vector3) (Vector2, bool) {
	view, projection := c.matrices()
	m := projection.mul(view)
	x := m[0]*point.X + m[4]*point.Y + m[8]*point.Z + m[12]
	y := m[1]*point.X + m[5]*point.Y + m[9]*point.Z + m[13]
	w := m[3]*point.X + m[7]*point.Y + m[11]*point.Z + m[15]
	if w <= 0 {
		return Vector2{}, false
	}
	return Vector2{X: (x/w + 1) / 2 * c.width, Y: (1 - y/w) / 2 * c.height}, true
}

// Ray returns the line from the camera through x, y on the screen, such as
// where the mouse points: where it starts, and its direction, length 1. With
// it, a game finds what the player clicks on.
//
//	origin, direction := s.camera.Ray(input.MousePosition())
func (c *Camera3D) Ray(x, y float32) (origin, direction Vector3) {
	view, projection := c.matrices()
	inverse := projection.mul(view).inverse()
	clipX := x/c.width*2 - 1
	clipY := 1 - y/c.height*2
	near := inverse.transformPoint(Vector3{X: clipX, Y: clipY, Z: -1})
	far := inverse.transformPoint(Vector3{X: clipX, Y: clipY, Z: 1})
	return near, far.Sub(near).Normalize()
}
