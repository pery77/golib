package golib

import (
	"fmt"
	"strings"

	"golib/internal/device"
)

// Screen is the surface a game draws on. Run passes it to Game.Draw; use it
// only there. Coordinates are in pixels, from the top-left corner, with Y
// growing downwards. Later drawing covers earlier drawing.
type Screen struct {
	width, height float32
	time          float32 // seconds of game time, for animated map tiles

	// camera is the camera SetCamera set in this Draw, or nil, and view the
	// part of the world it showed then.
	camera *Camera
	view   Rectangle
	blend  BlendMode // how SetBlendMode mixes colors in this Draw
}

// BlendMode says how the colors a game draws mix with what is on the screen
// under them.
type BlendMode int

// Blend modes.
const (
	// BlendNormal covers what is underneath, the more so the less
	// see-through the color is. The default.
	BlendNormal BlendMode = iota

	// BlendAdd adds the color to what is underneath, so that overlapping
	// shapes grow brighter and nothing gets darker: explosions, flames,
	// sparks, lasers and other glows. Black adds nothing.
	BlendAdd
)

// Width returns the screen width in pixels.
func (s *Screen) Width() float32 {
	return s.width
}

// Height returns the screen height in pixels.
func (s *Screen) Height() float32 {
	return s.height
}

// Clear fills the whole screen with color. Call it at the start of Draw.
func (s *Screen) Clear(color Color) {
	device.Clear(color)
}

// DrawRectangle fills rect with color.
func (s *Screen) DrawRectangle(rect Rectangle, color Color) {
	device.DrawRectangle(device.Rectangle(rect), color)
}

// DrawCircle fills the circle centered at x, y with color.
func (s *Screen) DrawCircle(x, y, radius float32, color Color) {
	device.DrawCircle(x, y, radius, color)
}

// DrawLine draws a straight line from x1, y1 to x2, y2, thickness pixels wide.
func (s *Screen) DrawLine(x1, y1, x2, y2, thickness float32, color Color) {
	device.DrawLine(x1, y1, x2, y2, thickness, color)
}

// DrawTriangle fills the triangle with corners x1, y1, x2, y2 and x3, y3. The
// corners can come in any order.
func (s *Screen) DrawTriangle(x1, y1, x2, y2, x3, y3 float32, color Color) {
	device.DrawTriangle(x1, y1, x2, y2, x3, y3, color)
}

// DrawRectangleOutline draws the edges of rect, thickness pixels wide, inside
// it.
func (s *Screen) DrawRectangleOutline(rect Rectangle, thickness float32, color Color) {
	device.DrawRectangleOutline(device.Rectangle(rect), thickness, color)
}

// DrawCircleOutline draws the edge of the circle centered at x, y, thickness
// pixels wide, inside it: a ring.
func (s *Screen) DrawCircleOutline(x, y, radius, thickness float32, color Color) {
	device.DrawRing(x, y, max(0, radius-thickness), radius, color)
}

// DrawPolygon fills the shape whose corners are points, in order, clockwise or
// not. It fills triangles from the shape's middle, the average of its
// corners, to each side, so every corner must be in sight from there, as in
// ships, rocks and stars. For a shape that turns, turn its corners first:
//
//	corners := make([]golib.Vector2, len(shipShape))
//	for i, corner := range shipShape {
//		corners[i] = corner.Rotate(ship.angle).Add(ship.position)
//	}
//	screen.DrawPolygon(corners, shipColor)
func (s *Screen) DrawPolygon(points []Vector2, color Color) {
	if len(points) < 3 {
		return
	}
	var middle Vector2
	for _, p := range points {
		middle = middle.Add(p)
	}
	middle = middle.Scale(1 / float32(len(points)))
	for i, p := range points {
		next := points[(i+1)%len(points)]
		s.DrawTriangle(middle.X, middle.Y, p.X, p.Y, next.X, next.Y, color)
	}
}

// DrawPolygonOutline draws the sides of the shape whose corners are points,
// from the last corner back to the first, thickness pixels wide, with round
// corners.
func (s *Screen) DrawPolygonOutline(points []Vector2, thickness float32, color Color) {
	for i, p := range points {
		next := points[(i+1)%len(points)]
		s.DrawLine(p.X, p.Y, next.X, next.Y, thickness, color)
		if thickness >= 2 {
			device.DrawCircle(p.X, p.Y, thickness/2, color)
		}
	}
}

// DrawText draws text with its top-left corner at x, y, size pixels high, in
// GoLib's built-in pixel font unless options give a [Font]. Text is drawn at
// whole pixels, rounding x and y, so that its letters stay sharp. A line break
// starts a new line, 2 pixels below the last.
//
// With options, x can be the middle or the end of each line instead of its
// start:
//
//	screen.DrawText("Game over", screen.Width()/2, 300, 40, golib.White, golib.TextOptions{Align: golib.AlignCenter})
func (s *Screen) DrawText(text string, x, y, size float32, color Color, options ...TextOptions) {
	font, spacing := textFont("DrawText", text, size, options)
	align := AlignLeft
	if len(options) > 0 {
		align = options[0].Align
	}
	if align == AlignLeft {
		device.DrawText(font, text, wholePixel(x), wholePixel(y), size, spacing, color)
		return
	}
	if align != AlignCenter && align != AlignRight {
		reportError(fmt.Errorf("golib: Screen.DrawText got TextOptions.Align %d: use AlignLeft, AlignCenter or AlignRight", align))
		return
	}
	for i, line := range strings.Split(text, "\n") {
		left := x - device.TextWidth(font, line, size, spacing)
		if align == AlignCenter {
			left = (x + left) / 2
		}
		top := y + float32(i)*(size+textLineGap)
		device.DrawText(font, line, wholePixel(left), wholePixel(top), size, spacing, color)
	}
}

// TextWidth returns the width, in pixels, of text drawn by DrawText at size
// with the same options, or of its longest line. Use it to fit text in a box,
// or to put something right after it:
//
//	x := 20 + screen.TextWidth(label, 20) + 8 // after the label and a gap
func (s *Screen) TextWidth(text string, size float32, options ...TextOptions) float32 {
	font, spacing := textFont("TextWidth", text, size, options)
	return device.TextWidth(font, text, size, spacing)
}

// SetCamera makes the drawing that follows show the world through camera:
// positions and sizes are in the world, and the camera decides which part of
// it is on the screen. SetCamera(nil) goes back to screen pixels, for scores
// and menus drawn over the world. Every Draw starts without a camera. The
// camera's view is taken when SetCamera is called, so call it again after
// changing the camera.
func (s *Screen) SetCamera(camera *Camera) {
	if camera == nil {
		if s.camera != nil {
			device.EndCamera()
			s.camera = nil
		}
		return
	}
	if camera.width != s.width || camera.height != s.height {
		reportError(fmt.Errorf("golib: Screen.SetCamera got a camera for a %g by %g screen, but the screen is %g by %g: pass the screen's size to NewCamera", camera.width, camera.height, s.width, s.height))
	}
	center := camera.Center()
	s.camera, s.view = camera, camera.View()
	device.BeginCamera(s.width/2, s.height/2, center.X, center.Y, camera.zoom())
}

// SetBlendMode changes how the drawing that follows mixes with what is
// already on the screen: BlendAdd makes glows brighter where they overlap,
// and BlendNormal, the default, covers. Every Draw starts with BlendNormal:
//
//	screen.SetBlendMode(golib.BlendAdd)
//	for _, spark := range s.sparks {
//		screen.DrawCircle(spark.X, spark.Y, 3, sparkColor)
//	}
//	screen.SetBlendMode(golib.BlendNormal)
func (s *Screen) SetBlendMode(mode BlendMode) {
	if mode != BlendNormal && mode != BlendAdd {
		reportError(fmt.Errorf("golib: Screen.SetBlendMode got %d: use BlendNormal or BlendAdd", mode))
		return
	}
	if mode == s.blend {
		return
	}
	if mode == BlendAdd {
		device.BeginBlendAdd()
	} else {
		device.EndBlend()
	}
	s.blend = mode
}

// endDraw goes back to drawing in screen pixels, in the normal blend mode.
// Run calls it after each Draw, in case the game left either set.
func (s *Screen) endDraw() {
	if s.camera != nil {
		device.EndCamera()
		s.camera = nil
	}
	if s.blend != BlendNormal {
		device.EndBlend()
		s.blend = BlendNormal
	}
}

// visible returns the part of the drawing's coordinates that is on the
// screen: the camera's view, or the whole screen without a camera.
func (s *Screen) visible() Rectangle {
	if s.camera == nil {
		return Rectangle{Width: s.width, Height: s.height}
	}
	return s.view
}
