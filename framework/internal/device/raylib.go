//go:build !js

package device

import (
	"image"
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The value types the backend passes around. On raylib they are raylib's own,
// so nothing is converted; another backend defines structs with the same
// fields.
type (
	// Color is an RGBA color with 8 bits per channel.
	Color = rl.Color

	// Vector2 is a point or a direction on the screen.
	Vector2 = rl.Vector2

	// Rectangle is an area of the screen, from its top-left corner.
	Rectangle = rl.Rectangle

	// Texture is a picture on the graphics card. Portable fields: ID, Width
	// and Height.
	Texture = rl.Texture2D

	// Target is a texture the game can draw into.
	Target = rl.RenderTexture2D

	// Shader is a compiled post-processing shader. Portable field: ID.
	Shader = rl.Shader

	// Font is a font drawn at one size, with the letters it was asked for.
	Font = rl.Font

	// Wave is the samples of a sound effect, before it is ready to play.
	Wave = rl.Wave

	// Sound is a sound effect ready to play. Portable field: FrameCount.
	Sound = rl.Sound

	// Music is a stream of sound the backend feeds while it plays.
	Music = rl.Music
)

// ShowsErrors says that this backend leaves error messages to the console and
// to golib/internal/startup, which shows them in a message box on Windows
// when nobody would see the console.
const ShowsErrors = false

// ShowError has nothing to show: see ShowsErrors.
func ShowError(title, message string) {}

// SavesToDisk says that a game here has a folder of its own to save high
// scores, settings and progress in.
const SavesToDisk = true

// OpenWindow opens the game window, hidden when hidden is true and resizable
// otherwise, and reports whether it opened. No key closes it: Run decides when
// the game ends.
func OpenWindow(width, height int, title string, hidden bool) bool {
	rl.SetTraceLogLevel(rl.LogWarning)
	if hidden {
		rl.SetConfigFlags(rl.FlagWindowHidden)
	} else {
		rl.SetConfigFlags(rl.FlagWindowResizable)
	}
	rl.InitWindow(int32(width), int32(height), title)
	if !rl.IsWindowReady() {
		return false
	}
	// raylib closes the window when Esc is pressed. GoLib leaves every key to
	// the game, which calls Quit to end.
	rl.SetExitKey(rl.KeyNull)
	return true
}

// CloseWindow closes the game window.
func CloseWindow() {
	rl.CloseWindow()
}

// WindowShouldClose reports whether the player closed the window.
func WindowShouldClose() bool {
	return rl.WindowShouldClose()
}

// WindowFocused reports whether the window has the player's attention.
func WindowFocused() bool {
	return rl.IsWindowFocused()
}

// SetTargetFPS asks the backend to draw fps frames per second at most.
func SetTargetFPS(fps int) {
	rl.SetTargetFPS(int32(fps))
}

// Time returns the seconds since the window opened.
func Time() float64 {
	return rl.GetTime()
}

// WindowSize returns the size of the window's drawing area, in pixels.
func WindowSize() (width, height int) {
	return rl.GetScreenWidth(), rl.GetScreenHeight()
}

// WindowPosition returns the window's top-left corner on the desktop.
func WindowPosition() (x, y int) {
	position := rl.GetWindowPosition()
	return int(position.X), int(position.Y)
}

// SetWindowSize resizes the window.
func SetWindowSize(width, height int) {
	rl.SetWindowSize(width, height)
}

// SetWindowPosition moves the window's top-left corner on the desktop.
func SetWindowPosition(x, y int) {
	rl.SetWindowPosition(x, y)
}

// SetWindowMinSize sets the smallest size the player can drag the window to.
func SetWindowMinSize(width, height int) {
	rl.SetWindowMinSize(width, height)
}

// SetWindowBorder shows or hides the window's title bar and border:
// fullscreen is a borderless window covering the monitor.
func SetWindowBorder(on bool) {
	if on {
		rl.ClearWindowState(rl.FlagWindowUndecorated)
	} else {
		rl.SetWindowState(rl.FlagWindowUndecorated)
	}
}

// MonitorBounds returns the position and size of the monitor the window is on.
func MonitorBounds() (x, y, width, height int) {
	monitor := rl.GetCurrentMonitor()
	corner := rl.GetMonitorPosition(monitor)
	return int(corner.X), int(corner.Y), rl.GetMonitorWidth(monitor), rl.GetMonitorHeight(monitor)
}

// SetCursorVisible shows or hides the mouse pointer over the window.
func SetCursorVisible(visible bool) {
	if visible {
		rl.ShowCursor()
	} else {
		rl.HideCursor()
	}
}

// SavePicture writes what was drawn into target to a PNG file, scaled by whole
// numbers when scale is above 1, and reports whether it was written. It is how
// golib shot saves a frame.
func SavePicture(target Target, path string, scale int) bool {
	picture := rl.LoadImageFromTexture(target.Texture)
	defer rl.UnloadImage(picture)
	rl.ImageFlipVertical(picture) // render textures are stored bottom row first
	if scale > 1 {
		rl.ImageResizeNN(picture, picture.Width*int32(scale), picture.Height*int32(scale))
	}
	// Drop alpha, which blending leaves below 255 at soft edges.
	rl.ImageFormat(picture, rl.UncompressedR8g8b8)
	return rl.ExportImage(*picture, path)
}

// CursorVisible reports whether the mouse pointer shows over the window.
func CursorVisible() bool {
	return !rl.IsCursorHidden()
}

// ReadTarget returns what was drawn into target as a picture, its top row
// first. It is how screenshots and the framework's own tests look at what a
// game drew.
func ReadTarget(target Target) *image.NRGBA {
	picture := rl.LoadImageFromTexture(target.Texture)
	defer rl.UnloadImage(picture)
	rl.ImageFlipVertical(picture) // render textures are stored bottom row first
	width, height := int(picture.Width), int(picture.Height)
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := rl.GetImageColor(*picture, int32(x), int32(y))
			result.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
	}
	return result
}
