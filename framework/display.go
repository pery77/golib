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

// window switches the game window between windowed and fullscreen, and
// remembers where the window was. Fullscreen is a window without borders that
// covers the monitor, so the monitor keeps its resolution, and switching back
// puts the window where it was, at the size it had.
type window struct {
	fullscreen          bool // what is applied now
	x, y, width, height int  // the window before it went fullscreen
}

// apply switches the window to match fullscreenWanted.
func (w *window) apply() {
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
