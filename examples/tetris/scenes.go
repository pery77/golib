package main

import (
	"fmt"

	"golib"
)

// The game's scenes. Each one is a golib.Game: golib.Run starts with the title,
// and golib.SwitchScene moves between them.
//
//	title --Play--> play --Esc/Start--> pause --Esc/Start--> back to the same play
//	  |              |                    '------Q/B------> title
//	  |              '--no room left--> game over --Enter/A--> a new play
//	  |                                '------Esc/B----> title
//	  '--Quit or Esc--> quit
//
// Every scene reads the keyboard and gamepad 0; the title also reads the mouse.

// options are the player's settings, shared by every scene.
type options struct {
	music bool // whether the theme is playing
}

func newOptions() *options {
	return &options{music: true}
}

// handleKeys reads the keys every scene shares: fullscreen and music on or off,
// then keeps the theme playing or paused to match.
func (o *options) handleKeys(input *golib.Input) {
	altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) && input.KeyPressed(golib.KeyEnter)
	if input.KeyPressed(golib.KeyF11) || altEnter {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	if input.KeyPressed(golib.KeyM) || input.GamepadPressed(0, golib.GamepadY) {
		o.music = !o.music
	}
	if o.music {
		theme.Play()
	} else {
		theme.Pause()
	}
}

// records are the scores kept between runs. JSON saves only exported fields, so
// their names start with a capital letter.
type records struct {
	Best int // the highest score ever reached
	Last int // the score of the last game, so the title can show it
}

// newRecords loads the saved records, or starts empty when there are none or
// they can't be read.
func newRecords() *records {
	r := &records{}
	if _, err := golib.LoadData("records", r); err != nil {
		// A file edited by hand could do this: carry on with a fresh record.
		*r = records{}
	}
	return r
}

// noteGame records the score of a finished game and saves it, so the title and
// the game over screen show it.
func (r *records) noteGame(score int) {
	r.Last = score
	if score > r.Best {
		r.Best = score
	}
	// Save when the game ends, not in every update.
	golib.SaveData("records", r)
}

// Title screen buttons, as indexes into titleScene.buttons.
const (
	playButton = iota
	quitButton
)

// titleScene shows the game's name, its controls and a menu with two buttons,
// over falling blocks.
type titleScene struct {
	options  *options
	records  *records
	buttons  []menuButton
	selected int    // the highlighted button
	gamepad  string // name of gamepad 0, or "" when none is connected
	time     float32
}

func newTitleScene(o *options, r *records) *titleScene {
	const width, height = 260, 52
	x := float32(screenWidth-width) / 2
	return &titleScene{
		options: o,
		records: r,
		buttons: []menuButton{
			playButton: {label: "Play", bounds: golib.Rectangle{X: x, Y: 470, Width: width, Height: height}},
			quitButton: {label: "Quit", bounds: golib.Rectangle{X: x, Y: 534, Width: width, Height: height}},
		},
		selected: playButton,
	}
}

// Update moves the selection with the Up and Down arrows, the d-pad, the mouse
// wheel or the pointer, and confirms it with Enter, A, Start or a click.
func (s *titleScene) Update(input *golib.Input, dt float32) {
	s.time += dt
	s.options.handleKeys(input)
	s.gamepad = input.GamepadName(0)

	step := 0
	if input.KeyPressed(golib.KeyUp) || input.KeyPressed(golib.KeyW) || input.GamepadPressed(0, golib.GamepadUp) || input.MouseWheel() > 0 {
		step--
	}
	if input.KeyPressed(golib.KeyDown) || input.KeyPressed(golib.KeyS) || input.GamepadPressed(0, golib.GamepadDown) || input.MouseWheel() < 0 {
		step++
	}
	s.selected = moveSelection(s.selected, step, len(s.buttons))

	x, y := input.MousePosition()
	pointed := buttonAt(s.buttons, x, y)
	clicked := input.MousePressed(golib.MouseLeft) && pointed >= 0
	if pointed >= 0 && (input.MouseMoved() || clicked) {
		s.selected = pointed
	}

	confirm := clicked || input.KeyPressed(golib.KeyEnter) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
	// No key quits by itself, not even Esc: the game calls golib.Quit when it
	// wants to end.
	if input.KeyPressed(golib.KeyEscape) || (confirm && s.selected == quitButton) {
		golib.Quit()
	} else if confirm && s.selected == playButton {
		golib.SwitchScene(newPlayScene(s.options, s.records))
	}
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawFallingBackdrop(screen, s.time)
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, golib.WithOpacity(backgroundColor, 0.72))

	drawCentered(screen, "TETRIS", 120, 96, accentColor)
	drawCentered(screen, "Stack the falling blocks", 250, 30, textColor)
	drawCentered(screen, "Fill a row to clear it. Don't reach the top.", 292, 24, dimTextColor)

	drawCentered(screen, "Move: Left/Right or A/D, d-pad or stick", 350, 22, textColor)
	drawCentered(screen, "Turn: Up/X (clockwise), Z (anticlockwise)", 380, 22, textColor)
	drawCentered(screen, "Drop: Down or S (soft), Space (hard)", 410, 22, textColor)
	drawCentered(screen, "Hold: C or Shift        Pause: Esc or Start", 440, 22, textColor)

	for i, button := range s.buttons {
		button.draw(screen, i == s.selected)
	}
	if s.records.Best > 0 {
		drawCentered(screen, fmt.Sprintf("Best %d", s.records.Best), 610, 26, accentColor)
	}
	if s.gamepad != "" {
		drawCentered(screen, "Gamepad: "+s.gamepad, 640, 18, dimTextColor)
	}
	settings := "F11 or Alt+Enter: fullscreen     M or Y: music " + onOff(s.options.music) + "     Esc: quit"
	drawCentered(screen, settings, 680, 18, dimTextColor)
}

// onOff turns a setting into the word the title screen shows.
func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// pauseScene freezes a play scene and shows a message over it. Resuming
// switches back to that same play scene, so the game carries on where it
// stopped.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	s.paused.options.handleKeys(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(s.paused)
	}
	if input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(s.paused.options, s.paused.records))
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawMessage(screen, "PAUSED", "Esc or Start to resume", "Q or B to quit to the title")
}

// gameOverScene shows the finished board under a message, with the score and
// whether it beat the best.
type gameOverScene struct {
	finished *playScene
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) {
	s.finished.options.handleKeys(input)
	if input.KeyPressed(golib.KeyEnter) || input.GamepadPressed(0, golib.GamepadA) {
		golib.SwitchScene(newPlayScene(s.finished.options, s.finished.records))
	}
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(s.finished.options, s.finished.records))
	}
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	s.finished.Draw(screen)
	best := s.finished.world.score >= s.finished.records.Best && s.finished.world.score > 0
	heading := "GAME OVER"
	if best {
		heading = "NEW BEST!"
	}
	drawMessage(screen, heading,
		fmt.Sprintf("Score %d     Lines %d     Level %d", s.finished.world.score, s.finished.world.lines, s.finished.world.level),
		"Enter or A to play again, Esc or B for the title")
}

// menuButton is an on-screen button of a menu.
type menuButton struct {
	label  string
	bounds golib.Rectangle
}

// buttonAt returns the index of the button under the point x, y, or -1.
func buttonAt(buttons []menuButton, x, y float32) int {
	for i, button := range buttons {
		if button.bounds.Contains(x, y) {
			return i
		}
	}
	return -1
}

// moveSelection moves the selected button index by step, staying within count
// buttons.
func moveSelection(selected, step, count int) int {
	return max(0, min(selected+step, count-1))
}

func (b menuButton) draw(screen *golib.Screen, selected bool) {
	color := panelColor
	if selected {
		color = accentColor
	}
	screen.DrawRectangle(b.bounds, color)
	screen.DrawRectangleOutline(b.bounds, 2, textColor)
	const size = 30
	labelColor := textColor
	if selected {
		labelColor = backgroundColor
	}
	x := b.bounds.X + (b.bounds.Width-screen.TextWidth(b.label, size))/2
	y := b.bounds.Y + (b.bounds.Height-size)/2
	screen.DrawText(b.label, x, y, size, labelColor)
}
