package golib

import (
	"slices"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Key is a key on the keyboard. Letters and digits are named after their
// position on a US keyboard.
type Key int32

// Keyboard keys.
const (
	KeySpace     Key = rl.KeySpace
	KeyEnter     Key = rl.KeyEnter
	KeyEscape    Key = rl.KeyEscape
	KeyTab       Key = rl.KeyTab
	KeyBackspace Key = rl.KeyBackspace

	KeyLeft  Key = rl.KeyLeft
	KeyRight Key = rl.KeyRight
	KeyUp    Key = rl.KeyUp
	KeyDown  Key = rl.KeyDown

	KeyLeftShift    Key = rl.KeyLeftShift
	KeyRightShift   Key = rl.KeyRightShift
	KeyLeftControl  Key = rl.KeyLeftControl
	KeyRightControl Key = rl.KeyRightControl
	KeyLeftAlt      Key = rl.KeyLeftAlt
	KeyRightAlt     Key = rl.KeyRightAlt

	KeyA Key = rl.KeyA
	KeyB Key = rl.KeyB
	KeyC Key = rl.KeyC
	KeyD Key = rl.KeyD
	KeyE Key = rl.KeyE
	KeyF Key = rl.KeyF
	KeyG Key = rl.KeyG
	KeyH Key = rl.KeyH
	KeyI Key = rl.KeyI
	KeyJ Key = rl.KeyJ
	KeyK Key = rl.KeyK
	KeyL Key = rl.KeyL
	KeyM Key = rl.KeyM
	KeyN Key = rl.KeyN
	KeyO Key = rl.KeyO
	KeyP Key = rl.KeyP
	KeyQ Key = rl.KeyQ
	KeyR Key = rl.KeyR
	KeyS Key = rl.KeyS
	KeyT Key = rl.KeyT
	KeyU Key = rl.KeyU
	KeyV Key = rl.KeyV
	KeyW Key = rl.KeyW
	KeyX Key = rl.KeyX
	KeyY Key = rl.KeyY
	KeyZ Key = rl.KeyZ

	KeyZero  Key = rl.KeyZero
	KeyOne   Key = rl.KeyOne
	KeyTwo   Key = rl.KeyTwo
	KeyThree Key = rl.KeyThree
	KeyFour  Key = rl.KeyFour
	KeyFive  Key = rl.KeyFive
	KeySix   Key = rl.KeySix
	KeySeven Key = rl.KeySeven
	KeyEight Key = rl.KeyEight
	KeyNine  Key = rl.KeyNine
)

// keyCount is one more than the highest raylib key code, so key codes can
// index arrays.
const keyCount = rl.KeyKbMenu + 1

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
