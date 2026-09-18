package golib

import (
	"slices"
	"strconv"
	"strings"

	"golib/internal/device"
)

// Key is a key on the keyboard. Letters and digits are named after their
// position on a US keyboard.
type Key int32

// Keyboard keys.
const (
	KeySpace     Key = device.KeySpace
	KeyEnter     Key = device.KeyEnter
	KeyEscape    Key = device.KeyEscape
	KeyTab       Key = device.KeyTab
	KeyBackspace Key = device.KeyBackspace

	KeyLeft  Key = device.KeyLeft
	KeyRight Key = device.KeyRight
	KeyUp    Key = device.KeyUp
	KeyDown  Key = device.KeyDown

	KeyLeftShift    Key = device.KeyLeftShift
	KeyRightShift   Key = device.KeyRightShift
	KeyLeftControl  Key = device.KeyLeftControl
	KeyRightControl Key = device.KeyRightControl
	KeyLeftAlt      Key = device.KeyLeftAlt
	KeyRightAlt     Key = device.KeyRightAlt

	KeyA Key = device.KeyA
	KeyB Key = device.KeyB
	KeyC Key = device.KeyC
	KeyD Key = device.KeyD
	KeyE Key = device.KeyE
	KeyF Key = device.KeyF
	KeyG Key = device.KeyG
	KeyH Key = device.KeyH
	KeyI Key = device.KeyI
	KeyJ Key = device.KeyJ
	KeyK Key = device.KeyK
	KeyL Key = device.KeyL
	KeyM Key = device.KeyM
	KeyN Key = device.KeyN
	KeyO Key = device.KeyO
	KeyP Key = device.KeyP
	KeyQ Key = device.KeyQ
	KeyR Key = device.KeyR
	KeyS Key = device.KeyS
	KeyT Key = device.KeyT
	KeyU Key = device.KeyU
	KeyV Key = device.KeyV
	KeyW Key = device.KeyW
	KeyX Key = device.KeyX
	KeyY Key = device.KeyY
	KeyZ Key = device.KeyZ

	KeyZero  Key = device.KeyZero
	KeyOne   Key = device.KeyOne
	KeyTwo   Key = device.KeyTwo
	KeyThree Key = device.KeyThree
	KeyFour  Key = device.KeyFour
	KeyFive  Key = device.KeyFive
	KeySix   Key = device.KeySix
	KeySeven Key = device.KeySeven
	KeyEight Key = device.KeyEight
	KeyNine  Key = device.KeyNine

	KeyF1  Key = device.KeyF1
	KeyF2  Key = device.KeyF2
	KeyF3  Key = device.KeyF3
	KeyF4  Key = device.KeyF4
	KeyF5  Key = device.KeyF5
	KeyF6  Key = device.KeyF6
	KeyF7  Key = device.KeyF7
	KeyF8  Key = device.KeyF8
	KeyF9  Key = device.KeyF9
	KeyF10 Key = device.KeyF10
	KeyF11 Key = device.KeyF11
	KeyF12 Key = device.KeyF12
)

// keyCount is one more than the highest key code, so key codes can
// index arrays.
const keyCount = device.KeyCount

// keyNames names every Key constant the way golib shot --input spells it: the
// constant's name without the Key prefix.
var keyNames = func() map[Key]string {
	names := map[Key]string{
		KeySpace: "Space", KeyEnter: "Enter", KeyEscape: "Escape", KeyTab: "Tab", KeyBackspace: "Backspace",
		KeyLeft: "Left", KeyRight: "Right", KeyUp: "Up", KeyDown: "Down",
		KeyLeftShift: "LeftShift", KeyRightShift: "RightShift", KeyLeftControl: "LeftControl",
		KeyRightControl: "RightControl", KeyLeftAlt: "LeftAlt", KeyRightAlt: "RightAlt",
	}
	for key := KeyA; key <= KeyZ; key++ {
		names[key] = string(rune('A' + (key - KeyA)))
	}
	for i, name := range []string{"Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine"} {
		names[KeyZero+Key(i)] = name
	}
	for i := range 12 {
		names[KeyF1+Key(i)] = "F" + strconv.Itoa(i+1)
	}
	return names
}()

// polledKeys lists every Key constant: Run reads their state each frame.
var polledKeys = func() []Key {
	keys := make([]Key, 0, len(keyNames))
	for key := range keyNames {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}()

// byName looks name up in names, such as keyNames, in any letter case:
// byName(keyNames, "enter") returns KeyEnter.
func byName[T comparable](names map[T]string, name string) (T, bool) {
	for value, valueName := range names {
		if strings.EqualFold(valueName, name) {
			return value, true
		}
	}
	var none T
	return none, false
}

// sortedNames returns the names in names, sorted.
func sortedNames[T comparable](names map[T]string) []string {
	list := make([]string, 0, len(names))
	for _, name := range names {
		list = append(list, name)
	}
	slices.Sort(list)
	return list
}
