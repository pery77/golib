package main

import "golib"

// Menu buttons, in pixels.
const (
	buttonWidth  = 136
	buttonHeight = 14
	buttonGap    = 3
)

// menu is a column of buttons across the middle of the screen, for the title,
// pause and level-complete scenes. It works with the arrows, WASD, the d-pad,
// the left stick, the mouse wheel and the pointer; Enter, Space, the A button
// or a click picks the selected button.
type menu struct {
	labels     []string
	top        float32 // y of the first button's top
	selected   int
	directions directionReader
	pointer    pointer
}

func newMenu(top float32, labels ...string) menu {
	return menu{labels: labels, top: top}
}

// bounds returns where button i is on the screen.
func (m *menu) bounds(i int) golib.Rectangle {
	return golib.Rectangle{
		X:      (screenWidth - buttonWidth) / 2,
		Y:      m.top + float32(i)*(buttonHeight+buttonGap),
		Width:  buttonWidth,
		Height: buttonHeight,
	}
}

// update moves the selection, and returns the button picked in this update,
// or -1.
func (m *menu) update(input *golib.Input) int {
	step := 0
	switch m.directions.pressed(input) {
	case up:
		step = -1
	case down:
		step = 1
	}
	if wheel := input.MouseWheel(); wheel > 0 {
		step = -1
	} else if wheel < 0 {
		step = 1
	}
	if moved := max(0, min(m.selected+step, len(m.labels)-1)); moved != m.selected {
		m.selected = moved
		menuMoveSound.Play()
	}

	// The pointer picks the button under it when it moves or clicks, so a
	// resting pointer doesn't fight the keys.
	x, y, moved := m.pointer.read(input)
	pointed := -1
	for i := range m.labels {
		if m.bounds(i).Contains(x, y) {
			pointed = i
		}
	}
	clicked := input.MousePressed(golib.MouseLeft) && pointed >= 0
	if pointed >= 0 && pointed != m.selected && (moved || clicked) {
		m.selected = pointed
		menuMoveSound.Play()
	}

	if clicked || confirmPressed(input) {
		menuSelectSound.Play()
		return m.selected
	}
	return -1
}

func (m *menu) draw(screen *golib.Screen) {
	for i, label := range m.labels {
		bounds := m.bounds(i)
		color := buttonColor
		if i == m.selected {
			color = buttonSelectedColor
		}
		screen.DrawRectangle(bounds, color)
		const size = 10
		x := bounds.X + (bounds.Width-screen.TextWidth(label, size))/2
		drawShadowed(screen, label, x, bounds.Y+(bounds.Height-size)/2, size, textColor)
	}
}
