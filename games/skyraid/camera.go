package main

import "golib"

// GoLib has no camera yet (it is planned for M6), so the game keeps its own:
// a position in the arena that every drawing call subtracts.

// Camera tuning.
const (
	cameraLead     = 110 // pixels the camera looks ahead, where the ship aims
	cameraResponse = 7   // per second: how quickly the camera catches up
	shakeMax       = 14  // pixels the screen moves at the strongest shake
)

// camera is the part of the arena the screen shows.
type camera struct {
	x, y           float32 // the arena point at the screen's top-left corner, without shake
	shakeX, shakeY float32 // pixels the picture is moved by, for screen shake
}

// newCamera returns a camera centered on the ship.
func newCamera(w *world) camera {
	c := camera{}
	c.x, c.y = cameraGoal(w)
	return c
}

// cameraGoal returns where the camera wants to be: the ship in the middle,
// a little ahead of where it aims, without showing outside the arena.
func cameraGoal(w *world) (float32, float32) {
	ax, ay := direction(w.ship.angle)
	x := w.ship.x + ax*cameraLead - screenWidth/2
	y := w.ship.y + ay*cameraLead - screenHeight/2
	return clamp(x, 0, worldWidth-screenWidth), clamp(y, 0, worldHeight-screenHeight)
}

// follow eases the camera towards its goal and shakes it as much as the world
// asks. Call it from Update, after the world's step: it uses random numbers.
func (c *camera) follow(w *world, dt float32) {
	goalX, goalY := cameraGoal(w)
	c.x = approach(c.x, goalX, cameraResponse, dt)
	c.y = approach(c.y, goalY, cameraResponse, dt)
	// Shake grows with the square of trauma, so small hits barely move it.
	strength := w.trauma * w.trauma * shakeMax
	c.shakeX = golib.RandomFloat(-1, 1) * strength
	c.shakeY = golib.RandomFloat(-1, 1) * strength
}

// toScreen returns where the arena point x, y is on the screen.
func (c camera) toScreen(x, y float32) (float32, float32) {
	return x - c.x + c.shakeX, y - c.y + c.shakeY
}

// toWorld returns the arena point under the screen point x, y, such as the
// mouse pointer.
func (c camera) toWorld(x, y float32) (float32, float32) {
	return x + c.x - c.shakeX, y + c.y - c.shakeY
}

// sees reports whether a circle in the arena is at least partly on the
// screen, so drawing can skip what isn't.
func (c camera) sees(x, y, radius float32) bool {
	sx, sy := c.toScreen(x, y)
	return sx+radius >= 0 && sx-radius <= screenWidth && sy+radius >= 0 && sy-radius <= screenHeight
}

// edgePoint returns where an arrow to the arena point x, y goes when the
// point is off the screen: on a frame inset pixels inside the screen's edge,
// on the line from the screen's middle to the point. ok is false when the
// point is on the screen.
func (c camera) edgePoint(x, y, inset float32) (ex, ey float32, ok bool) {
	if c.sees(x, y, 0) {
		return 0, 0, false
	}
	sx, sy := c.toScreen(x, y)
	midX, midY := float32(screenWidth)/2, float32(screenHeight)/2
	dx, dy := sx-midX, sy-midY
	halfW, halfH := midX-inset, midY-inset
	// Scale the line down until it touches the frame.
	scale := min(halfW/max(abs(dx), 0.001), halfH/max(abs(dy), 0.001))
	return midX + dx*scale, midY + dy*scale, true
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
