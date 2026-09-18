//go:build js

// The web backend: WebGL 2 and, from stage 2, Web Audio, reached through
// web.js, the JavaScript half of this backend. See device.go for the
// contract, and docs/roadmap.md for what each stage covers.
//
// Drawing does not cross into JavaScript one shape at a time, which would
// cost more than the drawing: package golib writes its shapes into a buffer
// of numbers, and a frame hands that whole buffer over at once. web_draw.go
// writes it and web.js reads it.

package device

import (
	"errors"
	"fmt"
	"syscall/js"
)

// The value types. They hold the same fields as raylib's, so package golib
// reads Width, Height and ID the same way on both backends.
type (
	// Color is an RGBA color with 8 bits per channel.
	Color struct{ R, G, B, A uint8 }

	// Vector2 is a point or a direction on the screen.
	Vector2 struct{ X, Y float32 }

	// Rectangle is an area of the screen, from its top-left corner.
	Rectangle struct{ X, Y, Width, Height float32 }

	// Texture is a picture on the graphics card.
	Texture struct {
		ID            uint32
		Width, Height int32
	}

	// Target is a texture the game can draw into.
	Target struct {
		ID      uint32
		Texture Texture
	}

	// Shader is a compiled post-processing shader.
	Shader struct{ ID uint32 }

	// Font is a font drawn at one size, with the letters it was asked for.
	Font struct{ ID uint32 }

	// Wave is the samples of a sound effect, before it is ready to play.
	Wave struct{ ID uint32 }

	// Sound is a sound effect ready to play.
	Sound struct {
		ID         uint32
		FrameCount uint32
	}

	// Music is a stream of sound the backend feeds while it plays.
	Music struct{ ID uint32 }
)

// glue is web.js, which the page loads before the game. It is looked up once,
// the first time it is needed, so that package initialization doesn't depend
// on the order the page loads its files in.
var glue js.Value

func js_() js.Value {
	if glue.IsUndefined() || glue.IsNull() {
		glue = js.Global().Get("golib")
	}
	return glue
}

// frame is what the page told the game about the frame it is drawing now.
// readFrame fills it once a frame, so nothing below asks JavaScript again.
var frame struct {
	time          float64 // seconds since the page opened
	width, height int     // the canvas, in pixels
	focused       bool
	closed        bool
}

// ShowsErrors says that this backend shows error messages itself: nobody
// playing in a browser looks at its console, so a game that stops says why on
// the page instead of leaving it black.
const ShowsErrors = true

// ShowError puts a message over the game, where the player will see it.
func ShowError(title, message string) {
	js_().Call("showError", title, message)
}

// HasSaveStore says that this backend keeps saved data itself: a page has no
// folder to write in, but the browser keeps a store of its own for the
// address the game is served from, which survives the page being closed.
//
// The store holds text, and it is small, a few megabytes for the whole
// address, which is what golib.SaveData is for anyway. A player in a private
// window, or one who clears their browsing data, starts fresh; so does the
// same game served from another address.
const HasSaveStore = true

// storeKey names a game's saved data in the browser's store, apart from
// anything else the page keeps there.
const storeKey = "golib.save."

// SaveToStore keeps data under name until the player clears it.
func SaveToStore(name string, data []byte) error {
	store := js.Global().Get("localStorage")
	if !store.Truthy() {
		return errors.New("this browser keeps no store for the game to save in")
	}
	// A store full, or turned off, throws instead of returning.
	var thrown error
	func() {
		defer func() {
			if r := recover(); r != nil {
				thrown = fmt.Errorf("the browser would not keep it: %v", r)
			}
		}()
		store.Call("setItem", storeKey+name, string(data))
	}()
	return thrown
}

// LoadFromStore returns what SaveToStore kept under name, and whether
// anything was kept.
func LoadFromStore(name string) ([]byte, bool, error) {
	store := js.Global().Get("localStorage")
	if !store.Truthy() {
		return nil, false, nil
	}
	kept := store.Call("getItem", storeKey+name)
	if !kept.Truthy() {
		return nil, false, nil
	}
	return []byte(kept.String()), true, nil
}

// DeleteFromStore removes what was kept under name.
func DeleteFromStore(name string) error {
	store := js.Global().Get("localStorage")
	if store.Truthy() {
		store.Call("removeItem", storeKey+name)
	}
	return nil
}

// OpenWindow prepares the canvas for a game of width by height pixels and
// reports whether it is ready. hidden has no meaning in a browser: a page is
// shown or it isn't, and golib shot doesn't run here.
func OpenWindow(width, height int, title string, hidden bool) bool {
	if !js_().Truthy() {
		return false
	}
	js_().Call("open", width, height, title)
	readFrame()
	return true
}

// CloseWindow ends the game's hold on the canvas.
func CloseWindow() {
	js_().Call("close")
}

// WindowShouldClose reports whether the page is going away. It also takes the
// picture of input and of the frame that the rest of this frame reads, so
// that one frame sees one state throughout.
func WindowShouldClose() bool {
	readInput()
	readFrame()
	return frame.closed
}

// readFrame reads the time, the canvas size and the focus for this frame.
func readFrame() {
	state := js_().Call("frame")
	frame.time = state.Index(0).Float()
	frame.width = state.Index(1).Int()
	frame.height = state.Index(2).Int()
	frame.focused = state.Index(3).Bool()
	frame.closed = state.Index(4).Bool()
}

// WindowFocused reports whether the page has the player's attention.
func WindowFocused() bool {
	return frame.focused
}

// SetTargetFPS is nothing here: the browser decides when to draw, and the
// game waits for it in EndFrame.
func SetTargetFPS(fps int) {}

// Time returns the seconds since the page opened, the same value for the
// whole frame, so a game's clock doesn't drift within one frame.
func Time() float64 {
	return frame.time
}

// WindowSize returns the size of the canvas, in pixels.
func WindowSize() (width, height int) {
	return frame.width, frame.height
}

// WindowPosition is 0, 0: a page doesn't choose where it sits.
func WindowPosition() (x, y int) {
	return 0, 0
}

// SetWindowSize is nothing here: the canvas follows the page.
func SetWindowSize(width, height int) {}

// SetWindowPosition is nothing here.
func SetWindowPosition(x, y int) {}

// SetWindowMinSize is nothing here.
func SetWindowMinSize(width, height int) {}

// SetWindowBorder asks the browser for fullscreen, without a border, or for
// the page again. Browsers only allow it while handling a key or a click, so
// web.js remembers the request and makes it at the next one.
func SetWindowBorder(on bool) {
	js_().Call("setFullscreen", !on)
}

// MonitorBounds returns the canvas as the monitor: a page has no desktop
// around it to measure.
func MonitorBounds() (x, y, width, height int) {
	return 0, 0, frame.width, frame.height
}

// SetCursorVisible shows or hides the mouse pointer over the canvas.
func SetCursorVisible(visible bool) {
	js_().Call("setCursorVisible", visible)
}

// SavePicture cannot write files from a page. golib shot doesn't run here;
// stage 3 gives the CLI its own way to take screenshots of a web build.
func SavePicture(target Target, path string, scale int) bool {
	return false
}

// CursorVisible reports whether the mouse pointer shows over the canvas.
func CursorVisible() bool {
	return js_().Call("cursorVisible").Bool()
}
