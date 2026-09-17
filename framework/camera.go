package golib

import (
	"fmt"
	"math"
)

// Camera shows part of a world larger than the screen, such as a level that
// scrolls. Make it once, with the screen's size, point it at what it follows
// in Update, and draw the world through it in Draw:
//
//	camera := golib.NewCamera(screenWidth, screenHeight)
//	camera.Bounds = golib.Rectangle{Width: level.Width(), Height: level.Height()}
//	camera.Lag = 0.2
//
//	// In Update:
//	s.camera.Target = s.player.Position
//	s.camera.Update(dt)
//
//	// In Draw:
//	screen.SetCamera(s.camera) // positions are in the world from here on
//	screen.DrawMap(level, 0, 0)
//	screen.DrawSprite(hero, frame, s.player.Position.X, s.player.Position.Y)
//	screen.SetCamera(nil) // screen pixels again, for the score
//	screen.DrawText(score, 4, 4, 10, golib.White)
//
// The view is centered on whole screen pixels, so that pixel art, maps and
// text stay sharp as it moves.
type Camera struct {
	// Target is the point in the world to keep in the middle of the screen.
	// Update moves the view towards it.
	Target Vector2

	// Lag smooths the view's movement: the view closes about two thirds of
	// the distance to Target in Lag seconds. 0 keeps Target exactly in the
	// middle.
	Lag float32

	// Zoom is how many screen pixels one pixel of the world covers: 2 shows
	// everything twice as large, and 0.5 shows twice as much of the world.
	// 0 counts as 1.
	Zoom float32

	// Bounds is the part of the world the view stays inside, such as the
	// whole level, so the screen never shows what lies beyond it. Where the
	// view is larger than Bounds, it is centered on Bounds. The zero
	// Rectangle sets no limit.
	Bounds Rectangle

	width, height float32 // the screen's size
	center        Vector2 // the middle of the view, before shaking
	shakeStrength float32 // how far the view moves at most at the start of the shake
	shakeLength   float32 // seconds the whole shake lasts
	shakeLeft     float32 // seconds of shaking left
	shakeOffset   Vector2 // how far the shake moves the view in this update
}

// NewCamera returns a camera for a screen width by height pixels, the size in
// [Config]. It starts with Target, and the view, in the middle of the screen,
// so it shows the world from 0, 0 as if there were no camera.
func NewCamera(width, height float32) *Camera {
	if width <= 0 || height <= 0 {
		reportError(fmt.Errorf("golib.NewCamera: the screen size is %g by %g: pass the screen's width and height, as in Config", width, height))
		width, height = defaultWidth, defaultHeight
	}
	middle := Vector2{X: width / 2, Y: height / 2}
	return &Camera{Target: middle, width: width, height: height, center: middle}
}

// Update moves the view towards Target, keeps it inside Bounds and moves the
// shake on. Call it once in every Update of the scene that has the camera,
// after setting Target.
func (c *Camera) Update(dt float32) {
	if c.Lag > 0 {
		// The share of the way left after dt, so that the view closes
		// 1 - 1/e, about two thirds, of the distance in Lag seconds.
		keep := float32(math.Exp(float64(-dt / c.Lag)))
		c.center = c.Target.Lerp(c.center, keep)
	} else {
		c.center = c.Target
	}
	c.center = c.bounded(c.center)

	c.shakeOffset = Vector2{}
	if c.shakeLeft > 0 {
		c.shakeLeft = max(0, c.shakeLeft-dt)
		reach := c.shakeStrength * c.shakeLeft / c.shakeLength
		c.shakeOffset = Vector2{X: RandomFloat(-reach, reach), Y: RandomFloat(-reach, reach)}
	}
}

// Snap puts the view on Target at once, inside Bounds, without Lag: at the
// start of a level, or after the player moves to another place.
func (c *Camera) Snap() {
	c.center = c.bounded(c.Target)
}

// Shake shakes the view for seconds, by up to strength pixels of the world at
// first, fading out to none, such as for an explosion. A shake replaces one
// that would move the view less, so a game that keeps a "how shaken is it"
// value can call Shake in every update, with the strength and the time that
// value asks for, and the view follows it. Call it from Update: the shake
// moves the view by random amounts, which golib shot repeats.
//
// [Camera.Center] and [Camera.View] include the shake, so whatever a game
// places from them, such as a parallax background, shakes with the view.
func (c *Camera) Shake(strength, seconds float32) {
	if strength <= 0 || seconds <= 0 {
		return
	}
	if c.shakeLeft > 0 && c.shakeStrength*c.shakeLeft/c.shakeLength > strength {
		return
	}
	c.shakeStrength, c.shakeLength, c.shakeLeft = strength, seconds, seconds
}

// Center returns the point of the world in the middle of the screen, as the
// camera draws it: moved by the shake, and put on a whole screen pixel.
func (c *Camera) Center() Vector2 {
	zoom := c.zoom()
	center := c.center.Add(c.shakeOffset)
	// A world pixel lands on a whole screen pixel when the view's left and
	// top edges do.
	return Vector2{
		X: (wholePixel(center.X*zoom-c.width/2) + c.width/2) / zoom,
		Y: (wholePixel(center.Y*zoom-c.height/2) + c.height/2) / zoom,
	}
}

// View returns the part of the world on the screen: draw only what overlaps
// it, or use it to place things just outside it.
func (c *Camera) View() Rectangle {
	zoom := c.zoom()
	center := c.Center()
	width, height := c.width/zoom, c.height/zoom
	return Rectangle{X: center.X - width/2, Y: center.Y - height/2, Width: width, Height: height}
}

// ToWorld returns the point of the world at x, y on the screen, such as
// where the mouse points: camera.ToWorld(input.MousePosition()).
func (c *Camera) ToWorld(x, y float32) Vector2 {
	zoom := c.zoom()
	center := c.Center()
	return Vector2{X: center.X + (x-c.width/2)/zoom, Y: center.Y + (y-c.height/2)/zoom}
}

// ToScreen returns where point, in the world, is on the screen, to draw
// something over it in screen pixels, such as an arrow to an enemy off the
// screen. The result can be outside the screen.
func (c *Camera) ToScreen(point Vector2) Vector2 {
	zoom := c.zoom()
	center := c.Center()
	return Vector2{X: (point.X-center.X)*zoom + c.width/2, Y: (point.Y-center.Y)*zoom + c.height/2}
}

// zoom returns Zoom, with 0 and less counting as 1.
func (c *Camera) zoom() float32 {
	if c.Zoom <= 0 {
		return 1
	}
	return c.Zoom
}

// bounded returns center moved so that the view around it stays inside
// Bounds.
func (c *Camera) bounded(center Vector2) Vector2 {
	if c.Bounds == (Rectangle{}) {
		return center
	}
	zoom := c.zoom()
	return Vector2{
		X: boundedAxis(center.X, c.width/zoom, c.Bounds.X, c.Bounds.Width),
		Y: boundedAxis(center.Y, c.height/zoom, c.Bounds.Y, c.Bounds.Height),
	}
}

// boundedAxis keeps a view of size around middle inside the range from start
// that is length long, or centers it there when it is larger.
func boundedAxis(middle, size, start, length float32) float32 {
	if size >= length {
		return start + length/2
	}
	return min(max(middle, start+size/2), start+length-size/2)
}
