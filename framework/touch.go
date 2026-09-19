package golib

import (
	"sync/atomic"

	"golib/internal/device"
)

// maxTouches is how many fingers at once GoLib reports, so touches can travel
// in an array. A game that needs a ninth finger doesn't exist.
const maxTouches = device.MaxTouches

// Touch is one finger on a touch screen, as one update sees it. Read them with
// [Input.Touches].
type Touch struct {
	// ID is the same number for as long as that finger stays down, so a game
	// can follow one finger from update to update. A finger that lifts and
	// lands again is a new one, with a number that hasn't been used before.
	ID int

	// Position is where the finger is, in the pixel coordinates [Screen] draws
	// with, as [Input.MousePosition] is. A finger on the black bars around the
	// screen is outside it.
	Position Vector2

	// Pressed is whether the finger landed since the previous update. A finger
	// that lands and lifts between two updates is reported to one update, with
	// Pressed true, so a quick tap is never lost.
	Pressed bool
}

// playingWithTouch is whether the player is playing with fingers now. Run keeps
// it in step at the start of every frame, and golib shot sets it from its input
// script.
var playingWithTouch atomic.Bool

// PlayingWithTouch reports whether the player is playing with fingers, which is
// what decides whether a game draws its on-screen controls:
//
//	func (s *playScene) Draw(screen *golib.Screen) {
//		s.drawWorld(screen)
//		if golib.PlayingWithTouch() {
//			s.drawTouchControls(screen)
//		}
//	}
//
// It follows the player, so one build plays both ways: it starts true on a
// phone or a tablet, where a finger is how the machine is pointed at, and false
// everywhere else, including a computer with a touch screen and a mouse. A
// finger on the screen then turns it on, wherever the game runs, and the
// keyboard, the mouse or a gamepad turns it off again. Between the two it holds
// its answer, so the controls don't blink between taps.
//
// It is false on the desktop until the first finger, and no finger ever
// arrives: GoLib reads no touch screen there, and a touch screen on Windows
// moves the mouse pointer instead, so those games are played with the mouse.
// Under golib shot it is true when the --input script has Touch in it.
func PlayingWithTouch() bool {
	return playingWithTouch.Load()
}

// followTouchPlaying moves PlayingWithTouch to what the player last used. A
// frame in which they use nothing leaves it as it was.
func followTouchPlaying(fingers, keyboard, mouse, gamepad bool) {
	switch {
	case fingers:
		playingWithTouch.Store(true)
	case keyboard || mouse || gamepad:
		playingWithTouch.Store(false)
	}
}

// Touches returns the fingers on the screen now, oldest first. It is empty
// where there is no touch screen, so a game can read it alongside the keyboard
// without checking first:
//
//	for _, touch := range input.Touches() {
//		if touch.Pressed && fireButton.Contains(touch.Position.X, touch.Position.Y) {
//			s.fire()
//		}
//	}
//
// The oldest finger also moves the mouse pointer and holds [MouseLeft] down, so
// a tap works a menu written for a mouse without the game doing anything.
//
// The slice belongs to this update: read it in Update, and don't keep it.
func (in *Input) Touches() []Touch {
	return in.touches[:in.touchCount]
}

// TouchDownIn reports whether a finger is inside area, which is what an
// on-screen button held down asks, such as a thrust or a steering pad:
//
//	if input.TouchDownIn(leftPad) {
//		s.ship.turn(-turnSpeed)
//	}
func (in *Input) TouchDownIn(area Rectangle) bool {
	for _, touch := range in.Touches() {
		if area.Contains(touch.Position.X, touch.Position.Y) {
			return true
		}
	}
	return false
}

// TouchPressedIn reports whether a finger landed inside area since the previous
// update, which is what an on-screen button tapped asks, such as a jump or a
// menu item. Each landing is reported to exactly one update.
func (in *Input) TouchPressedIn(area Rectangle) bool {
	for _, touch := range in.Touches() {
		if touch.Pressed && area.Contains(touch.Position.X, touch.Position.Y) {
			return true
		}
	}
	return false
}

// deviceTouches returns the fingers the machine reports, in the pixel
// coordinates the game draws with, given the rectangle fitScreen chose.
func deviceTouches(into []Touch, fit device.Rectangle, screenWidth, screenHeight float32) []Touch {
	into = into[:0]
	for _, point := range device.TouchPoints() {
		if len(into) >= maxTouches {
			break
		}
		x, y := toScreen(point.X, point.Y, fit, screenWidth, screenHeight)
		into = append(into, Touch{
			ID:       point.ID,
			Position: Vector2{X: x, Y: y},
			Pressed:  point.New,
		})
	}
	return into
}
