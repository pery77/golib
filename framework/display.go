package golib

import (
	"math"
	"sync/atomic"

	"golib/internal/device"
)

// fullscreenWanted is the state SetFullscreen asks for. Run applies it at the
// start of every frame.
var fullscreenWanted atomic.Bool

// SetFullscreen switches the game between a window and fullscreen, from the
// next frame. Fullscreen covers the monitor the window is on without changing
// its resolution. The screen games draw on keeps its size either way: Run
// scales it to fit, with black bars where the shapes differ. Call it from
// Update, for example on F11 or Alt+Enter:
//
//	altEnter := input.KeyDown(golib.KeyLeftAlt) && input.KeyPressed(golib.KeyEnter)
//	if input.KeyPressed(golib.KeyF11) || altEnter {
//		golib.SetFullscreen(!golib.IsFullscreen())
//	}
//
// No key switches by itself. To start in fullscreen, set Config.Fullscreen.
// Screenshots from golib shot ignore fullscreen.
//
// In a browser it happens at the next key, click or touch, which is when a
// browser allows it, and the player can leave it themselves with Esc or a
// phone's gesture; IsFullscreen follows them when they do, so this call switches
// it back on. On a phone, that key, click or touch is a tap, so a game meant for
// one needs something to tap.
func SetFullscreen(on bool) {
	fullscreenWanted.Store(on)
}

// IsFullscreen reports whether the game is in fullscreen, or will be from the
// next frame. In a browser it turns false by itself when the player leaves
// fullscreen with Esc or a phone's gesture.
func IsFullscreen() bool {
	return fullscreenWanted.Load()
}

// mouseHiddenWanted is whether SetMouseVisible asks to hide the mouse pointer.
// Run applies it at the start of every frame.
var mouseHiddenWanted atomic.Bool

// SetMouseVisible shows or hides the mouse pointer over the game's window,
// from the next frame, such as to draw a crosshair of the game's own in its
// place. The pointer still moves, and [Input.MousePosition] still reports it.
// It shows again outside the window.
func SetMouseVisible(visible bool) {
	mouseHiddenWanted.Store(!visible)
}

// IsMouseVisible reports whether the mouse pointer shows over the game's
// window, or will from the next frame.
func IsMouseVisible() bool {
	return !mouseHiddenWanted.Load()
}

// windowUnfocused is whether the game's window has lost the player's
// attention. Run keeps it in step at the start of every frame. It stays false
// where there is no window to focus, under golib shot and in tests, so game
// code takes the same path there.
var windowUnfocused atomic.Bool

// WindowFocused reports whether the game's window has the player's attention.
// It is false while they work in another program, so a game can quieten its
// music or draw a sign over itself; with Config.PauseUnfocused, Run stops
// updating the game meanwhile, and Draw keeps running:
//
//	func (s *playScene) Draw(screen *golib.Screen) {
//		s.drawWorld(screen)
//		if !golib.WindowFocused() {
//			screen.DrawText("Paused", 640, 360, golib.TextOptions{Align: golib.AlignCenter})
//		}
//	}
//
// Under golib shot and in tests it is always true.
func WindowFocused() bool {
	return !windowUnfocused.Load()
}

// window switches the game window between windowed and fullscreen, and
// remembers where the window was. Fullscreen is a window without borders that
// covers the monitor, so the monitor keeps its resolution, and switching back
// puts the window where it was, at the size it had.
type window struct {
	fullscreen          bool // what is applied now
	x, y, width, height int  // the window before it went fullscreen
	mouseHidden         bool // the mouse pointer is hidden now
}

// apply switches the window to match fullscreenWanted, and shows or hides the
// mouse pointer to match mouseHiddenWanted.
func (w *window) apply() {
	if hide := mouseHiddenWanted.Load(); hide != w.mouseHidden {
		device.SetCursorVisible(!hide)
		w.mouseHidden = hide
	}
	// A player who leaves fullscreen themselves, which a browser lets them do
	// with Esc or a phone's gesture, is not put back into it: the game's idea
	// of fullscreen follows the machine's, so the next SetFullscreen is a real
	// change again and IsFullscreen keeps telling the truth.
	if w.fullscreen && device.FullscreenLost() {
		w.fullscreen = false
		fullscreenWanted.Store(false)
	}
	want := fullscreenWanted.Load()
	if want == w.fullscreen {
		return
	}
	if want {
		w.x, w.y = device.WindowPosition()
		w.width, w.height = device.WindowSize()
		monitorX, monitorY, monitorWidth, monitorHeight := device.MonitorBounds()
		device.SetWindowBorder(false)
		device.SetWindowPosition(monitorX, monitorY)
		device.SetWindowSize(monitorWidth, monitorHeight)
	} else {
		device.SetWindowBorder(true)
		device.SetWindowSize(w.width, w.height)
		device.SetWindowPosition(w.x, w.y)
	}
	w.fullscreen = want
}

// fitScreen returns where a screen of screenWidth by screenHeight pixels goes
// in a window of windowWidth by windowHeight pixels: as large as it fits,
// centered, keeping its shape. With pixelArt, the scale is a whole number, at
// least 1, so every pixel stays square and sharp.
func fitScreen(screenWidth, screenHeight, windowWidth, windowHeight float32, pixelArt bool) device.Rectangle {
	scale := min(windowWidth/screenWidth, windowHeight/screenHeight)
	if pixelArt {
		scale = max(1, float32(math.Floor(float64(scale))))
	}
	width, height := screenWidth*scale, screenHeight*scale
	return device.Rectangle{
		X:      float32(math.Floor(float64(windowWidth-width) / 2)),
		Y:      float32(math.Floor(float64(windowHeight-height) / 2)),
		Width:  width,
		Height: height,
	}
}

// toScreen converts a point from window pixels to screen pixels, the
// coordinates games draw with, given the rectangle fitScreen chose. A window
// with no room for the screen, such as a minimized one, leaves the point as it
// is.
func toScreen(x, y float32, fit device.Rectangle, screenWidth, screenHeight float32) (float32, float32) {
	if fit.Width <= 0 || fit.Height <= 0 {
		return x, y
	}
	return (x - fit.X) * screenWidth / fit.Width, (y - fit.Y) * screenHeight / fit.Height
}

// enlargeWindow makes the window of a pixel art game a whole number of times
// the size of its screen, as large as fits in most of the monitor, and
// centers it there. A small screen, such as 320 by 180, would otherwise open
// a tiny window.
func enlargeWindow(screenWidth, screenHeight int) {
	cornerX, cornerY, monitorWidth, monitorHeight := device.MonitorBounds()
	scale := windowScale(screenWidth, screenHeight, monitorWidth, monitorHeight)
	if scale <= 1 {
		return
	}
	width, height := screenWidth*scale, screenHeight*scale
	device.SetWindowSize(width, height)
	device.SetWindowPosition(cornerX+(monitorWidth-width)/2, cornerY+(monitorHeight-height)/2)
}

// windowScale returns how many times a screen fits in four fifths of a
// monitor, as a whole number, and at least 1.
func windowScale(screenWidth, screenHeight, monitorWidth, monitorHeight int) int {
	if screenWidth <= 0 || screenHeight <= 0 {
		return 1
	}
	return max(1, min(monitorWidth*4/5/screenWidth, monitorHeight*4/5/screenHeight))
}
