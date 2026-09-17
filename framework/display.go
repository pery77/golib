package golib

import (
	"math"
	"sync/atomic"

	rl "github.com/gen2brain/raylib-go/raylib"
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
func SetFullscreen(on bool) {
	fullscreenWanted.Store(on)
}

// IsFullscreen reports whether the game is in fullscreen, or will be from the
// next frame.
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
		if hide {
			rl.HideCursor()
		} else {
			rl.ShowCursor()
		}
		w.mouseHidden = hide
	}
	want := fullscreenWanted.Load()
	if want == w.fullscreen {
		return
	}
	if want {
		position := rl.GetWindowPosition()
		w.x, w.y = int(position.X), int(position.Y)
		w.width, w.height = rl.GetScreenWidth(), rl.GetScreenHeight()
		monitor := rl.GetCurrentMonitor()
		monitorPosition := rl.GetMonitorPosition(monitor)
		rl.SetWindowState(rl.FlagWindowUndecorated)
		rl.SetWindowPosition(int(monitorPosition.X), int(monitorPosition.Y))
		rl.SetWindowSize(rl.GetMonitorWidth(monitor), rl.GetMonitorHeight(monitor))
	} else {
		rl.ClearWindowState(rl.FlagWindowUndecorated)
		rl.SetWindowSize(w.width, w.height)
		rl.SetWindowPosition(w.x, w.y)
	}
	w.fullscreen = want
}

// fitScreen returns where a screen of screenWidth by screenHeight pixels goes
// in a window of windowWidth by windowHeight pixels: as large as it fits,
// centered, keeping its shape. With pixelArt, the scale is a whole number, at
// least 1, so every pixel stays square and sharp.
func fitScreen(screenWidth, screenHeight, windowWidth, windowHeight float32, pixelArt bool) rl.Rectangle {
	scale := min(windowWidth/screenWidth, windowHeight/screenHeight)
	if pixelArt {
		scale = max(1, float32(math.Floor(float64(scale))))
	}
	width, height := screenWidth*scale, screenHeight*scale
	return rl.Rectangle{
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
func toScreen(x, y float32, fit rl.Rectangle, screenWidth, screenHeight float32) (float32, float32) {
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
	monitor := rl.GetCurrentMonitor()
	monitorWidth, monitorHeight := rl.GetMonitorWidth(monitor), rl.GetMonitorHeight(monitor)
	scale := windowScale(screenWidth, screenHeight, monitorWidth, monitorHeight)
	if scale <= 1 {
		return
	}
	width, height := screenWidth*scale, screenHeight*scale
	corner := rl.GetMonitorPosition(monitor)
	rl.SetWindowSize(width, height)
	rl.SetWindowPosition(int(corner.X)+(monitorWidth-width)/2, int(corner.Y)+(monitorHeight-height)/2)
}

// windowScale returns how many times a screen fits in four fifths of a
// monitor, as a whole number, and at least 1.
func windowScale(screenWidth, screenHeight, monitorWidth, monitorHeight int) int {
	if screenWidth <= 0 || screenHeight <= 0 {
		return 1
	}
	return max(1, min(monitorWidth*4/5/screenWidth, monitorHeight*4/5/screenHeight))
}
