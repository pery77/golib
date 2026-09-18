//go:build !js

package device

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The contract fixes the key, button and color numbers in device.go, so that
// every backend reports the same key and draws the same red. The raylib
// backend passes them straight to raylib, so they have to be raylib's own.
// These tests fail if raylib ever moves one, which would otherwise show up as
// a game reading the wrong key only on the desktop.

func TestKeyCodesAreRaylibs(t *testing.T) {
	keys := map[string][2]int32{
		"KeySpace": {KeySpace, rl.KeySpace}, "KeyEnter": {KeyEnter, rl.KeyEnter},
		"KeyEscape": {KeyEscape, rl.KeyEscape}, "KeyTab": {KeyTab, rl.KeyTab},
		"KeyBackspace": {KeyBackspace, rl.KeyBackspace},
		"KeyLeft":      {KeyLeft, rl.KeyLeft}, "KeyRight": {KeyRight, rl.KeyRight},
		"KeyUp": {KeyUp, rl.KeyUp}, "KeyDown": {KeyDown, rl.KeyDown},
		"KeyLeftShift": {KeyLeftShift, rl.KeyLeftShift}, "KeyRightShift": {KeyRightShift, rl.KeyRightShift},
		"KeyLeftControl": {KeyLeftControl, rl.KeyLeftControl}, "KeyRightControl": {KeyRightControl, rl.KeyRightControl},
		"KeyLeftAlt": {KeyLeftAlt, rl.KeyLeftAlt}, "KeyRightAlt": {KeyRightAlt, rl.KeyRightAlt},
		"KeyA": {KeyA, rl.KeyA}, "KeyM": {KeyM, rl.KeyM}, "KeyZ": {KeyZ, rl.KeyZ},
		"KeyZero": {KeyZero, rl.KeyZero}, "KeyFive": {KeyFive, rl.KeyFive}, "KeyNine": {KeyNine, rl.KeyNine},
		"KeyF1": {KeyF1, rl.KeyF1}, "KeyF11": {KeyF11, rl.KeyF11}, "KeyF12": {KeyF12, rl.KeyF12},
		"KeyCount": {KeyCount, rl.KeyKbMenu + 1},
	}
	for name, pair := range keys {
		if pair[0] != pair[1] {
			t.Errorf("%s is %d in the contract and %d in raylib: change device.go to raylib's number, or games read the wrong key on one backend", name, pair[0], pair[1])
		}
	}
}

func TestButtonCodesAreRaylibs(t *testing.T) {
	buttons := map[string][2]int32{
		"MouseLeft": {MouseLeft, int32(rl.MouseButtonLeft)}, "MouseRight": {MouseRight, int32(rl.MouseButtonRight)},
		"MouseMiddle": {MouseMiddle, int32(rl.MouseButtonMiddle)},
		"GamepadUp":   {GamepadUp, rl.GamepadButtonLeftFaceUp}, "GamepadRight": {GamepadRight, rl.GamepadButtonLeftFaceRight},
		"GamepadDown": {GamepadDown, rl.GamepadButtonLeftFaceDown}, "GamepadLeft": {GamepadLeft, rl.GamepadButtonLeftFaceLeft},
		"GamepadY": {GamepadY, rl.GamepadButtonRightFaceUp}, "GamepadB": {GamepadB, rl.GamepadButtonRightFaceRight},
		"GamepadA": {GamepadA, rl.GamepadButtonRightFaceDown}, "GamepadX": {GamepadX, rl.GamepadButtonRightFaceLeft},
		"GamepadLeftBumper":   {GamepadLeftBumper, rl.GamepadButtonLeftTrigger1},
		"GamepadLeftTrigger":  {GamepadLeftTrigger, rl.GamepadButtonLeftTrigger2},
		"GamepadRightBumper":  {GamepadRightBumper, rl.GamepadButtonRightTrigger1},
		"GamepadRightTrigger": {GamepadRightTrigger, rl.GamepadButtonRightTrigger2},
		"GamepadBack":         {GamepadBack, rl.GamepadButtonMiddleLeft}, "GamepadStart": {GamepadStart, rl.GamepadButtonMiddleRight},
		"GamepadLeftStickButton":  {GamepadLeftStickButton, rl.GamepadButtonLeftThumb},
		"GamepadRightStickButton": {GamepadRightStickButton, rl.GamepadButtonRightThumb},
	}
	for name, pair := range buttons {
		if pair[0] != pair[1] {
			t.Errorf("%s is %d in the contract and %d in raylib: change device.go to raylib's number", name, pair[0], pair[1])
		}
	}
}

func TestPaletteIsRaylibs(t *testing.T) {
	colors := map[string][2]Color{
		"LightGray": {LightGray, rl.LightGray}, "Gray": {Gray, rl.Gray}, "DarkGray": {DarkGray, rl.DarkGray},
		"Yellow": {Yellow, rl.Yellow}, "Gold": {Gold, rl.Gold}, "Orange": {Orange, rl.Orange},
		"Pink": {Pink, rl.Pink}, "Red": {Red, rl.Red}, "Maroon": {Maroon, rl.Maroon},
		"Green": {Green, rl.Green}, "Lime": {Lime, rl.Lime}, "DarkGreen": {DarkGreen, rl.DarkGreen},
		"SkyBlue": {SkyBlue, rl.SkyBlue}, "Blue": {Blue, rl.Blue}, "DarkBlue": {DarkBlue, rl.DarkBlue},
		"Purple": {Purple, rl.Purple}, "Violet": {Violet, rl.Violet}, "DarkPurple": {DarkPurple, rl.DarkPurple},
		"Beige": {Beige, rl.Beige}, "Brown": {Brown, rl.Brown}, "DarkBrown": {DarkBrown, rl.DarkBrown},
		"White": {White, rl.White}, "Black": {Black, rl.Black}, "Blank": {Blank, rl.Blank},
		"Magenta": {Magenta, rl.Magenta}, "RayWhite": {RayWhite, rl.RayWhite},
	}
	for name, pair := range colors {
		if pair[0] != pair[1] {
			t.Errorf("%s is %v in the contract and %v in raylib: change device.go to raylib's color", name, pair[0], pair[1])
		}
	}
}
