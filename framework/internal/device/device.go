// Package device is the line between GoLib and the machine it draws, sounds
// and reads input on.
//
// Everything in package golib above this line is plain Go: the game loop, the
// Aseprite and Tiled readers, the sound synthesizers, the camera, the vectors
// and the timers. Everything below it is one backend, chosen when the game is
// built:
//
//	raylib*.go   //go:build !js   raylib on Windows, Linux and macOS
//	web*.go      //go:build js    WebGL 2 and Web Audio in a browser
//
// Package golib must not import raylib itself. A test checks that, so the
// contract stays the only way down, and a backend can be added without
// touching the code above it.
//
// # The contract
//
// A backend provides:
//
//   - The value types: Color, Vector2, Rectangle, and handles for a texture,
//     a render target, a shader, a font, a sound, a wave and a music. Only
//     their ID, Width and Height fields are portable; nothing above this
//     package reads anything else.
//   - The functions in raylib.go, raylib_draw.go, raylib_audio.go and
//     raylib_input.go, with the same behavior.
//
// The numbers below are the same on every backend, so a game reads the same
// key wherever it runs. They are GLFW's key codes, which raylib passes on;
// raylib_test.go fails if raylib ever moves one.
//
// A backend that can't do something does the harmless thing rather than
// stopping the game: no sound device means silence, and an unknown key is
// never down.
package device

import "strings"

// The key codes GoLib passes on to games.
const (
	KeySpace     = 32
	KeyEnter     = 257
	KeyEscape    = 256
	KeyTab       = 258
	KeyBackspace = 259

	KeyLeft  = 263
	KeyRight = 262
	KeyUp    = 265
	KeyDown  = 264

	KeyLeftShift    = 340
	KeyRightShift   = 344
	KeyLeftControl  = 341
	KeyRightControl = 345
	KeyLeftAlt      = 342
	KeyRightAlt     = 346

	// The letters and digits are the ASCII codes of their capitals and
	// figures, so they follow each other.
	KeyA = 65
	KeyB = KeyA + 1
	KeyC = KeyA + 2
	KeyD = KeyA + 3
	KeyE = KeyA + 4
	KeyF = KeyA + 5
	KeyG = KeyA + 6
	KeyH = KeyA + 7
	KeyI = KeyA + 8
	KeyJ = KeyA + 9
	KeyK = KeyA + 10
	KeyL = KeyA + 11
	KeyM = KeyA + 12
	KeyN = KeyA + 13
	KeyO = KeyA + 14
	KeyP = KeyA + 15
	KeyQ = KeyA + 16
	KeyR = KeyA + 17
	KeyS = KeyA + 18
	KeyT = KeyA + 19
	KeyU = KeyA + 20
	KeyV = KeyA + 21
	KeyW = KeyA + 22
	KeyX = KeyA + 23
	KeyY = KeyA + 24
	KeyZ = KeyA + 25

	KeyZero  = 48
	KeyOne   = KeyZero + 1
	KeyTwo   = KeyZero + 2
	KeyThree = KeyZero + 3
	KeyFour  = KeyZero + 4
	KeyFive  = KeyZero + 5
	KeySix   = KeyZero + 6
	KeySeven = KeyZero + 7
	KeyEight = KeyZero + 8
	KeyNine  = KeyZero + 9

	KeyF1  = 290
	KeyF2  = KeyF1 + 1
	KeyF3  = KeyF1 + 2
	KeyF4  = KeyF1 + 3
	KeyF5  = KeyF1 + 4
	KeyF6  = KeyF1 + 5
	KeyF7  = KeyF1 + 6
	KeyF8  = KeyF1 + 7
	KeyF9  = KeyF1 + 8
	KeyF10 = KeyF1 + 9
	KeyF11 = KeyF1 + 10
	KeyF12 = KeyF1 + 11
)

// KeyCount is one more than the highest key code, so key codes can index
// arrays. The highest is the menu key, which GoLib doesn't name.
const KeyCount = 348 + 1

// The mouse buttons.
const (
	MouseLeft   = 0
	MouseRight  = 1
	MouseMiddle = 2
)

// TouchPoint is one finger on a touch screen, as a backend reports it.
type TouchPoint struct {
	ID   int     // the same number while that finger stays down
	X, Y float32 // where the finger is, in window pixels, as the mouse is
	New  bool    // the finger landed since the last frame
}

// MaxTouches is how many fingers at once a backend reports, so touches can
// travel in an array. More fingers than that are ignored, in the order they
// landed: no game needs a ninth.
const MaxTouches = 8

// The gamepad buttons, named after an Xbox controller.
const (
	GamepadUp    = 1 // the d-pad
	GamepadRight = 2
	GamepadDown  = 3
	GamepadLeft  = 4

	GamepadY = 5
	GamepadB = 6
	GamepadA = 7
	GamepadX = 8

	GamepadLeftBumper   = 9
	GamepadLeftTrigger  = 10
	GamepadRightBumper  = 11
	GamepadRightTrigger = 12

	GamepadBack  = 13 // 14 is the middle button, which GoLib doesn't name
	GamepadStart = 15

	GamepadLeftStickButton  = 16
	GamepadRightStickButton = 17
)

// The colors of raylib's palette, which GoLib passes on to games.
var (
	LightGray  = Color{R: 200, G: 200, B: 200, A: 255}
	Gray       = Color{R: 130, G: 130, B: 130, A: 255}
	DarkGray   = Color{R: 80, G: 80, B: 80, A: 255}
	Yellow     = Color{R: 253, G: 249, B: 0, A: 255}
	Gold       = Color{R: 255, G: 203, B: 0, A: 255}
	Orange     = Color{R: 255, G: 161, B: 0, A: 255}
	Pink       = Color{R: 255, G: 109, B: 194, A: 255}
	Red        = Color{R: 230, G: 41, B: 55, A: 255}
	Maroon     = Color{R: 190, G: 33, B: 55, A: 255}
	Green      = Color{R: 0, G: 228, B: 48, A: 255}
	Lime       = Color{R: 0, G: 158, B: 47, A: 255}
	DarkGreen  = Color{R: 0, G: 117, B: 44, A: 255}
	SkyBlue    = Color{R: 102, G: 191, B: 255, A: 255}
	Blue       = Color{R: 0, G: 121, B: 241, A: 255}
	DarkBlue   = Color{R: 0, G: 82, B: 172, A: 255}
	Purple     = Color{R: 200, G: 122, B: 255, A: 255}
	Violet     = Color{R: 135, G: 60, B: 190, A: 255}
	DarkPurple = Color{R: 112, G: 31, B: 126, A: 255}
	Beige      = Color{R: 211, G: 176, B: 131, A: 255}
	Brown      = Color{R: 127, G: 106, B: 79, A: 255}
	DarkBrown  = Color{R: 76, G: 63, B: 47, A: 255}
	White      = Color{R: 255, G: 255, B: 255, A: 255}
	Black      = Color{R: 0, G: 0, B: 0, A: 255}
	Blank      = Color{R: 0, G: 0, B: 0, A: 0}
	Magenta    = Color{R: 255, G: 0, B: 255, A: 255}
	RayWhite   = Color{R: 245, G: 245, B: 245, A: 255}
)

// ESShader returns a fragment shader written for desktop OpenGL as one a
// backend on OpenGL ES can compile, which is what browsers have. The two
// differ in their first lines: the version, and the precision every ES
// shader has to declare. What comes after is left alone, so a shader that
// mixes whole numbers into float arithmetic, which ES refuses, still fails
// to compile and says so.
func ESShader(source string) string {
	const version = "#version 300 es"
	const precision = "precision highp float;"
	rest := strings.TrimLeft(source, " \t\r\n")
	if strings.HasPrefix(rest, "#version") {
		if line := strings.IndexByte(rest, '\n'); line >= 0 {
			rest = rest[line+1:]
		} else {
			rest = ""
		}
	}
	return version + "\n" + precision + "\n" + rest
}
