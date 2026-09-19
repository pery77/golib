package main

import "golib"

// The controls for a phone or a tablet, where there is no keyboard and no
// gamepad. One build plays both ways: options.pads says whether they show and
// are read, which follows golib.PlayingWithTouch unless the player has chosen
// with F4. A player with a keyboard and a mouse never sees them until a finger
// lands on the game.
//
// The pads sit inside the screen, not against the window's edges, so they stay
// where the thumbs are however GoLib scales the screen to the phone, in
// landscape or in portrait.

// The colors of the on-screen controls: faint, so the game stays the thing you
// look at.
var (
	padColor     = golib.Color{R: 110, G: 225, B: 255, A: 40}
	padEdgeColor = golib.Color{R: 110, G: 225, B: 255, A: 110}
	padTextColor = golib.Color{R: 200, G: 230, B: 245, A: 200}
)

// The pads the game is played with: two to turn, one to thrust, one to fire,
// and one to pause.
var (
	turnLeftPad  = touchPad{area: golib.Rectangle{X: 40, Y: 470, Width: 150, Height: 190}, arrow: pointingLeft}
	turnRightPad = touchPad{area: golib.Rectangle{X: 210, Y: 470, Width: 150, Height: 190}, arrow: pointingRight}
	thrustPad    = touchPad{area: golib.Rectangle{X: 920, Y: 470, Width: 150, Height: 190}, arrow: pointingUp}
	firePad      = touchPad{area: golib.Rectangle{X: 1090, Y: 470, Width: 150, Height: 190}, label: "FIRE"}
	pausePad     = touchPad{area: golib.Rectangle{X: 565, Y: 610, Width: 150, Height: 70}, label: "PAUSE"}
)

// The buttons the screens between plays are worked with.
var (
	playPad       = touchPad{area: golib.Rectangle{X: 490, Y: 430, Width: 300, Height: 90}, label: "PLAY"}
	fullscreenPad = touchPad{area: golib.Rectangle{X: 490, Y: 540, Width: 300, Height: 70}, label: "FULLSCREEN"}
	resumePad     = touchPad{area: golib.Rectangle{X: 490, Y: 430, Width: 300, Height: 90}, label: "RESUME"}
	titlePad      = touchPad{area: golib.Rectangle{X: 490, Y: 540, Width: 300, Height: 70}, label: "TITLE"}
	againPad      = touchPad{area: golib.Rectangle{X: 490, Y: 430, Width: 300, Height: 90}, label: "PLAY AGAIN"}
)

// arrow is the sign drawn on a pad that has no word: an arrow says which way
// the ship turns, or that it thrusts, in any language and at a glance.
type arrow int

const (
	noArrow arrow = iota
	pointingLeft
	pointingRight
	pointingUp
)

// touchPad is one on-screen control: an area with a word or an arrow in it.
type touchPad struct {
	area  golib.Rectangle
	label string
	arrow arrow
}

// held reports whether a finger is on the pad, for a control the player holds,
// such as turning or thrusting.
func (p touchPad) held(input *golib.Input) bool {
	return input.TouchDownIn(p.area)
}

// tapped reports whether a finger landed on the pad since the previous update,
// for a one-off action, such as pausing or starting a play.
func (p touchPad) tapped(input *golib.Input) bool {
	return input.TouchPressedIn(p.area)
}

// draw shows the pad, with its word or its arrow in the middle.
func (p touchPad) draw(screen *golib.Screen) {
	screen.DrawRectangle(p.area, padColor)
	screen.DrawRectangleOutline(p.area, lineWidth, padEdgeColor)
	middle := p.area.Center()
	if p.arrow != noArrow {
		const point, base = 30, 22 // how far the arrow reaches from the middle
		switch p.arrow {
		case pointingLeft:
			screen.DrawTriangle(middle.X-point, middle.Y, middle.X+base, middle.Y-point, middle.X+base, middle.Y+point, padTextColor)
		case pointingRight:
			screen.DrawTriangle(middle.X+point, middle.Y, middle.X-base, middle.Y-point, middle.X-base, middle.Y+point, padTextColor)
		case pointingUp:
			screen.DrawTriangle(middle.X, middle.Y-point, middle.X-point, middle.Y+base, middle.X+point, middle.Y+base, padTextColor)
		}
		return
	}
	const size = 28
	screen.DrawText(p.label, middle.X, middle.Y-size/2, size, padTextColor, golib.TextOptions{Align: golib.AlignCenter})
}

// drawTouchPads shows the pads the play scene is played with. The caller has
// already decided that the game is being played with fingers.
func drawTouchPads(screen *golib.Screen) {
	drawTouchButtons(screen, turnLeftPad, turnRightPad, thrustPad, firePad, pausePad)
}

// drawTouchButtons shows the given pads.
func drawTouchButtons(screen *golib.Screen, pads ...touchPad) {
	for _, pad := range pads {
		pad.draw(screen)
	}
}

// touchChoice is what the player has said about the on-screen pads.
type touchChoice int

const (
	// touchAuto shows them while the game is played with fingers, and hides
	// them again at the first key, mouse move or gamepad button. It is what a
	// player gets without asking: fingers on a phone, keys on a computer, and
	// the same build for both.
	touchAuto touchChoice = iota
	touchAlways
	touchNever
)

// next is the choice F4 moves to.
func (c touchChoice) next() touchChoice {
	if c == touchNever {
		return touchAuto
	}
	return c + 1
}

// word is what the title screen calls the choice.
func (c touchChoice) word() string {
	switch c {
	case touchAlways:
		return "on"
	case touchNever:
		return "off"
	}
	return "auto"
}

// shows reports whether the pads show and are read.
func (c touchChoice) shows() bool {
	switch c {
	case touchAlways:
		return true
	case touchNever:
		return false
	}
	return golib.PlayingWithTouch()
}
