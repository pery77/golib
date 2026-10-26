//go:build !js

package device

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// macOS needs four things of the raylib backend that Windows and Linux
// don't. The first two, about the window, were found on a Retina iMac, 2560
// by 1440 points on a 5120 by 2880 screen:
//
//   - While the window opens, and golib.Run enlarges it, AppKit can measure
//     it in Retina pixels, twice its size in points, and raylib keeps that
//     size although it draws in points. The screen then shows twice too
//     large, only its bottom-left quarter in the window, until the player
//     resizes the window. MeasureWindow makes raylib measure it again.
//   - A borderless window the size of the monitor, GoLib's fullscreen
//     elsewhere, sits below the menu bar here, so its bottom falls off the
//     monitor. macOS has a fullscreen of its own, the one the window's green
//     button enters: a space of its own, without the menu bar or the Dock,
//     that Cmd+Tab leaves like any other. SetSystemFullscreen uses it.
//   - A debug build is a bare executable, which the Dock shows with the
//     generic icon for programs, and raylib's SetWindowIcon does nothing on
//     macOS. SetAppIcon gives the Dock the game's icon.
//   - macOS measures windows and monitors in points, two pixels across on a
//     Retina screen. DisplayScale says how many.
//
// They ask AppKit through purego's Objective-C calls, as raylib-go reaches
// raylib through purego.

// The Objective-C messages the backend sends, registered once.
var (
	selContentView             = objc.RegisterName("contentView")
	selFrame                   = objc.RegisterName("frame")
	selConvertRectToBacking    = objc.RegisterName("convertRectToBacking:")
	selBackingScaleFactor      = objc.RegisterName("backingScaleFactor")
	selStyleMask               = objc.RegisterName("styleMask")
	selToggleFullScreen        = objc.RegisterName("toggleFullScreen:")
	selSharedApplication       = objc.RegisterName("sharedApplication")
	selSetApplicationIconImage = objc.RegisterName("setApplicationIconImage:")
	selDataWithBytesLength     = objc.RegisterName("dataWithBytes:length:")
	selAlloc                   = objc.RegisterName("alloc")
	selInitWithData            = objc.RegisterName("initWithData:")
)

// nsWindowStyleMaskFullScreen is the bit of an NSWindow's styleMask that is
// on while the window is in macOS's fullscreen.
const nsWindowStyleMaskFullScreen = 1 << 14

// nsRect is AppKit's NSRect.
type nsRect struct{ X, Y, Width, Height float64 }

// windowMeasured is whether MeasureWindow has seen raylib keep the window's
// own size. It checks no more after that: the wrong size only comes while
// the window opens.
var windowMeasured bool

// MeasureWindow makes raylib measure the window again while the size it keeps
// isn't the window's own, which happens on macOS as the window opens (see the
// top of this file). It waits for AppKit to measure the window in points,
// since raylib would measure it wrong again before that, and for the window
// to leave macOS's fullscreen, whose own resize measures it again. Run calls
// it at the start of every frame.
func MeasureWindow() {
	// With the high-density flag, raylib intentionally keeps separate
	// logical window and physical framebuffer sizes. Do not collapse them.
	if windowHighDPI {
		return
	}
	if windowMeasured || inSystemFullscreen() {
		return
	}
	view := nsWindow().Send(selContentView)
	points := objc.Send[nsRect](view, selFrame)
	pixels := objc.Send[nsRect](view, selConvertRectToBacking, points)
	if pixels.Width != points.Width || pixels.Height != points.Height {
		return // AppKit still counts Retina pixels
	}
	width, height := int(points.Width), int(points.Height)
	if rl.GetScreenWidth() == width && rl.GetScreenHeight() == height &&
		rl.GetRenderWidth() == width && rl.GetRenderHeight() == height {
		windowMeasured = true
		return
	}
	// What the player's own resize does: GLFW measures the window again at
	// every new size, one point wider and then the size it has.
	rl.SetWindowSize(width+1, height)
	rl.SetWindowSize(width, height)
}

// HasSystemFullscreen reports true: golib uses macOS's own fullscreen
// instead of a borderless window (see the top of this file). A hidden window,
// as in the framework's tests, keeps the borderless one.
func HasSystemFullscreen() bool {
	return !rl.IsWindowHidden()
}

// SetSystemFullscreen enters or leaves macOS's own fullscreen. The window
// slides into place over the next frames, and every key and mouse event
// meanwhile is lost, releases too: golib only asks while nothing is held.
func SetSystemFullscreen(on bool) {
	if on != inSystemFullscreen() {
		nsWindow().Send(selToggleFullScreen, objc.ID(0))
	}
}

// FullscreenLost reports whether the window is out of macOS's fullscreen
// while golib had put it there: the player can leave it themselves, with the
// window's green button.
func FullscreenLost() bool {
	return !rl.IsWindowHidden() && !inSystemFullscreen()
}

// SetAppIcon shows png, a PNG image, as the game's icon in the Dock while it
// runs. golib gives it the game's icon.png in debug builds, which are bare
// executables that the Dock shows with the generic icon for programs; the
// app golib dist makes carries its own icon. An image macOS can't read
// leaves the icon as it was.
func SetAppIcon(png []byte) {
	if len(png) == 0 {
		return
	}
	data := objc.ID(objc.GetClass("NSData")).Send(selDataWithBytesLength, unsafe.Pointer(&png[0]), uint(len(png)))
	image := objc.ID(objc.GetClass("NSImage")).Send(selAlloc).Send(selInitWithData, data)
	if image == 0 {
		return
	}
	objc.ID(objc.GetClass("NSApplication")).Send(selSharedApplication).Send(selSetApplicationIconImage, image)
}

// DisplayScale returns how many of the screen's pixels, across, a point is
// where the window is: 2 on a Retina screen, 1 on others. It asks the window
// for its backingScaleFactor, the screen's, and not the content view to
// convert to its backing: raylib draws in points, so after the window has
// been in macOS's fullscreen, the view's backing counts one pixel a point,
// even on a Retina screen.
func DisplayScale() float32 {
	if scale := objc.Send[float64](nsWindow(), selBackingScaleFactor); scale > 0 {
		return float32(scale)
	}
	return 1
}

// inSystemFullscreen reports whether the window is in macOS's fullscreen, or
// sliding into it.
func inSystemFullscreen() bool {
	return objc.Send[uint](nsWindow(), selStyleMask)&nsWindowStyleMaskFullScreen != 0
}

// nsWindow returns the game's window as AppKit's NSWindow.
func nsWindow() objc.ID {
	return objc.ID(uintptr(rl.GetWindowHandle()))
}
