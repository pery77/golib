package golib

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"golib/internal/device"
)

// fullscreenWanted is the state SetFullscreen asks for. Run applies it at the
// start of every frame.
var fullscreenWanted atomic.Bool

// SetFullscreen switches the game between a window and fullscreen, from the
// next frame. Fullscreen covers the monitor the window is on without changing
// its resolution. On macOS it is the system's own fullscreen, the one the
// window's green button enters: the window slides into a space of its own,
// without the menu bar or the Dock, and the player can leave it with that
// button too. There the switch waits until no key or mouse button is held,
// such as the Enter that asked for it, because macOS loses a release that
// comes while the window slides. The screen keeps its size unless the game
// requests Config.WindowScale or Config.FillWindow; Run scales it to fit the
// window, with black bars where the shapes differ.
// Call it from Update, for example on F11 or Alt+Enter:
//
//	altEnter := input.KeyDown(golib.KeyLeftAlt) && input.KeyPressed(golib.KeyEnter)
//	if input.KeyPressed(golib.KeyF11) || altEnter {
//		golib.SetFullscreen(!golib.IsFullscreen())
//	}
//
// No key switches by itself. To start in fullscreen, set Config.Fullscreen.
// Screenshots from golib shot ignore fullscreen.
//
// In a browser, fullscreen comes only after a key, a click or a touch: on the
// one the game read this call on, as a settings screen does, it comes at
// once; otherwise, as with Config.Fullscreen, at the next one. Leaving is at
// once. The player can leave it themselves with Esc or a phone's gesture;
// IsFullscreen follows them when they do, there and on macOS, so this call
// switches it back on. On a phone, that key, click or touch is a tap, so a
// game meant for one needs something to tap.
func SetFullscreen(on bool) {
	fullscreenWanted.Store(on)
}

// IsFullscreen reports whether the game is in fullscreen, or will be from the
// next frame. In a browser it turns false by itself when the player leaves
// fullscreen with Esc or a phone's gesture, and on macOS when they leave it
// with the window's green button.
func IsFullscreen() bool {
	return fullscreenWanted.Load()
}

// mouseHiddenWanted is whether SetMouseVisible asks to hide the mouse pointer.
// Run applies it at the start of every frame.
var mouseHiddenWanted atomic.Bool

// SetMouseVisible shows or hides the mouse pointer over the game's window,
// from the next frame, such as to draw a crosshair of the game's own in its
// place: the system's pointer, or the sprite SetMouseSprite draws instead.
// The pointer still moves, and [Input.MousePosition] still reports it. It
// shows again outside the window.
func SetMouseVisible(visible bool) {
	mouseHiddenWanted.Store(!visible)
}

// mouseSprite is what SetMouseSprite draws in place of the mouse pointer: a
// nil sprite for the system's own pointer.
var mouseSprite struct {
	sync.Mutex
	sprite     *Sprite
	frame      int
	tipX, tipY float32
}

// SetMouseSprite draws frame number frame of sprite in place of the system's
// mouse pointer, from the next frame, with its pixel x, y, counted from the
// frame's top-left corner, where the pointer points: 0, 0 for an arrow whose
// tip is that corner. nil goes back to the system's pointer. Call it again
// with another frame to change the pointer, such as to a hand over a button.
//
// The sprite goes over everything, post-processing included, at the
// window's resolution, so it moves as smoothly as the system's pointer, and
// as many whole times larger as the screen is, so pixel art stays sharp. It
// doesn't show while the pointer is outside the window, while
// SetMouseVisible hides it, or while the player plays with a gamepad or with
// fingers (see PlayingWithGamepad and PlayingWithTouch): moving the mouse
// brings it back. A frame the sprite doesn't have stops Run with an error.
func SetMouseSprite(sprite *Sprite, frame int, x, y float32) {
	if sprite != nil {
		count, err := sprite.frameCount()
		if err != nil {
			return // reported
		}
		if frame < 0 || frame >= count {
			reportError(fmt.Errorf("golib.SetMouseSprite got frame %d of %s, which has %d frame(s), numbered from 0 to %d", frame, sprite.call(), count, count-1))
			return
		}
	}
	mouseSprite.Lock()
	defer mouseSprite.Unlock()
	mouseSprite.sprite, mouseSprite.frame, mouseSprite.tipX, mouseSprite.tipY = sprite, frame, x, y
}

// mouseSpriteSet reports whether SetMouseSprite replaced the system's mouse
// pointer, which is then hidden over the window.
func mouseSpriteSet() bool {
	mouseSprite.Lock()
	defer mouseSprite.Unlock()
	return mouseSprite.sprite != nil
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
// puts the window where it was, at the size it had. On macOS it is the
// system's own fullscreen instead, which puts the window back by itself.
type window struct {
	fullscreen          bool // what is applied now
	x, y, width, height int  // the window before it went fullscreen
	mouseHidden         bool // the mouse pointer is hidden now
	moveTo              int  // the monitor SetMonitor asked for, plus one, still to move to; 0 for none
}

// apply switches the window to match fullscreenWanted, and shows or hides the
// mouse pointer to match mouseHiddenWanted. held is whether a key or a mouse
// button was down when the input was last read.
func (w *window) apply(held bool) {
	w.placeWindow(held)
	if hide := mouseHiddenWanted.Load() || mouseSpriteSet(); hide != w.mouseHidden {
		device.SetCursorVisible(!hide)
		w.mouseHidden = hide
	}
	// A player who leaves fullscreen themselves, which a browser lets them do
	// with Esc or a phone's gesture, and macOS with the window's green button,
	// is not put back into it: the game's idea of fullscreen follows the
	// machine's, so the next SetFullscreen is a real change again and
	// IsFullscreen keeps telling the truth.
	if w.fullscreen && device.FullscreenLost() {
		w.fullscreen = false
		fullscreenWanted.Store(false)
	}
	want := fullscreenWanted.Load()
	if want == w.fullscreen || w.moveTo != 0 {
		return // a move to another monitor comes first
	}
	// Where the system has a fullscreen of its own that games should use,
	// macOS's, the backend switches it; elsewhere golib covers the monitor.
	// macOS drops every key and mouse event while its fullscreen slides in or
	// out, so a key released meanwhile, such as the Enter that asked for the
	// switch, would stay down until pressed again: the switch waits until
	// nothing is held.
	if device.HasSystemFullscreen() {
		if !held {
			device.SetSystemFullscreen(want)
			w.fullscreen = want
		}
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

// screenInWindow returns the size of the screen a game draws on in a window of
// windowWidth by windowHeight pixels, and where it goes in the window. It is
// Config.Width by Config.Height, fitted with fitScreen, unless the game has
// Config.FillWindow: then it grows wider or taller, at the scale fitScreen
// would use, until it covers the window. A minimized window, with no room,
// keeps the Config's size. Config.WindowScale takes precedence over FillWindow.
func screenInWindow(config Config, windowWidth, windowHeight float32) (width, height float32, fit device.Rectangle) {
	width, height = float32(config.Width), float32(config.Height)
	if config.WindowScale != 0 {
		if windowWidth > 0 && windowHeight > 0 {
			w, h := windowScreenSize(config, int(windowWidth), int(windowHeight))
			width, height = float32(w), float32(h)
		}
		return width, height, fitScreen(
			width, height, windowWidth, windowHeight, config.PixelArt,
		)
	}
	if !config.FillWindow || windowWidth <= 0 || windowHeight <= 0 {
		return width, height, fitScreen(width, height, windowWidth, windowHeight, config.PixelArt)
	}
	scale := min(windowWidth/width, windowHeight/height)
	if config.PixelArt {
		scale = max(1, float32(math.Floor(float64(scale))))
	}
	// Rounded up, so the screen covers the window: a thousandth of a pixel
	// less keeps float error from adding a whole column.
	cover := func(window, least float32) float32 {
		return max(least, float32(math.Ceil(float64(window/scale)-0.001)))
	}
	width, height = cover(windowWidth, width), cover(windowHeight, height)
	return width, height, device.Rectangle{
		X:      float32(math.Floor(float64(windowWidth-width*scale) / 2)),
		Y:      float32(math.Floor(float64(windowHeight-height*scale) / 2)),
		Width:  width * scale,
		Height: height * scale,
	}
}

// screenForWindow separates a high-density framebuffer's physical pixels
// from the logical window coordinates used by presentation and the mouse.
func screenForWindow(config Config, windowWidth, windowHeight, framebufferWidth, framebufferHeight int) (width, height float32, fit device.Rectangle) {
	if !config.HighDPI || framebufferWidth <= 0 || framebufferHeight <= 0 || windowWidth <= 0 || windowHeight <= 0 {
		return screenInWindow(config, float32(windowWidth), float32(windowHeight))
	}
	width, height, fit = screenInWindow(config, float32(framebufferWidth), float32(framebufferHeight))
	// Choose whole pixel-art scales and FillWindow's rounding in physical
	// pixels first, then convert the resulting rectangle to desktop points.
	x, y := float32(windowWidth)/float32(framebufferWidth), float32(windowHeight)/float32(framebufferHeight)
	fit.X, fit.Width = fit.X*x, fit.Width*x
	fit.Y, fit.Height = fit.Y*y, fit.Height*y
	return width, height, fit
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

// shrinkWindow makes a window of width by height pixels smaller, keeping its
// shape, when it doesn't fit in most of the monitor, and centers it there.
// raylib centers a new window on the monitor's space without the taskbar, but
// with unsigned sizes, so a window larger than that space would open far off
// every monitor, at a y of 32767 on Windows.
func shrinkWindow(width, height int) {
	cornerX, cornerY, monitorWidth, monitorHeight := device.MonitorBounds()
	fitWidth, fitHeight := windowFit(width, height, monitorWidth, monitorHeight)
	if fitWidth == width && fitHeight == height {
		return
	}
	device.SetWindowSize(fitWidth, fitHeight)
	device.SetWindowPosition(cornerX+(monitorWidth-fitWidth)/2, cornerY+(monitorHeight-fitHeight)/2)
}

// windowFit returns the size of a window of width by height pixels made to fit
// in nine tenths of a monitor, which leaves room for the title bar and the
// taskbar: the same size if it fits, or else smaller, with the same shape.
func windowFit(width, height, monitorWidth, monitorHeight int) (int, int) {
	roomWidth, roomHeight := monitorWidth*9/10, monitorHeight*9/10
	if width <= 0 || height <= 0 || roomWidth <= 0 || roomHeight <= 0 || (width <= roomWidth && height <= roomHeight) {
		return width, height
	}
	if width*roomHeight <= height*roomWidth {
		return max(1, width*roomHeight/height), roomHeight // the height decides
	}
	return roomWidth, max(1, height*roomWidth/width)
}

// windowScale returns how many times a screen fits in four fifths of a
// monitor, as a whole number, and at least 1.
func windowScale(screenWidth, screenHeight, monitorWidth, monitorHeight int) int {
	if screenWidth <= 0 || screenHeight <= 0 {
		return 1
	}
	return max(1, min(monitorWidth*4/5/screenWidth, monitorHeight*4/5/screenHeight))
}
