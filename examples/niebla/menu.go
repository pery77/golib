package main

import (
	"fmt"
	"log"

	"golib"
)

type menuAction int

const (
	playButton menuAction = iota
	continueButton
	newButton
	loadButton
	quitButton
)

// The buttons' shape, on the screen's middle line.
const (
	menuButtonWidth  = 200
	menuButtonHeight = 38
	menuButtonStep   = 52
)

// menuScene is the title screen.
type menuScene struct {
	buttons  []menuButton
	selected int
	saves    []saveInfo
	warning  string
}

// newMenuScene lays the menu out and puts the monitor filters on, the
// same ones the play scene runs under.
func newMenuScene() *menuScene {
	setFilters(true)
	s := &menuScene{}
	if db != nil {
		var err error
		s.saves, err = db.listSaves(player)
		if err != nil {
			log.Print(err)
			s.warning = "Could not list saved games."
		}
	}
	activeResize = s.resize
	s.resize(screenWidth, screenHeight)
	return s
}

func (s *menuScene) resize(width, height int) {
	s.buttons = []menuButton{
		{label: "Play", action: playButton},
		{label: "Quit", action: quitButton},
	}
	if len(s.saves) > 0 {
		s.buttons = []menuButton{
			{label: "Continue", action: continueButton},
			{label: "New", action: newButton},
			{label: "Load", action: loadButton},
			{label: "Quit", action: quitButton},
		}
	}
	x := (width - menuButtonWidth) / 2
	mid := float32(height) / 2
	buttonY := mid + 30
	if height < 360 {
		buttonY = mid + 12
	}
	step := menuButtonStep
	if len(s.buttons) > 2 && height < 360 {
		step = 42
	}
	blockHeight := (len(s.buttons)-1)*step + menuButtonHeight
	buttonY = min(buttonY, float32(height-32-blockHeight))
	for i := range s.buttons {
		s.buttons[i].bounds = golib.Rectangle{
			X: float32(x), Y: buttonY + float32(i*step),
			Width: menuButtonWidth, Height: menuButtonHeight,
		}
	}
}

// Update moves the selection with the up and down keys, the d-pad, the
// wheel or the pointer, and confirms it with Enter, Space, A, Start or a
// click. Esc quits, as the only screen where it does.
func (s *menuScene) Update(input *golib.Input, dt float32) {
	if menuBack(input) {
		golib.Quit()
		return
	}
	var confirm bool
	s.selected, confirm = updateMenuSelection(input, s.buttons, s.selected)
	if !confirm {
		return
	}
	switch s.buttons[s.selected].action {
	case quitButton:
		golib.Quit()
	case playButton:
		s.play()
	case continueButton:
		slot := latestSave(s.saves).Slot
		if err := loadGame(slot); err != nil {
			log.Print(err)
			s.warning = "Could not load Save " + slot + "."
		}
	case newButton:
		s.start(nil)
	case loadButton:
		golib.SwitchScene(newLoadScene(s.saves))
	}
}

func menuBack(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) ||
		input.GamepadPressed(0, golib.GamepadBack)
}

func updateMenuSelection(
	input *golib.Input, buttons []menuButton, selected int,
) (int, bool) {
	step := 0
	if input.KeyPressed(golib.KeyUp) || input.KeyPressed(golib.KeyW) ||
		input.GamepadPressed(0, golib.GamepadUp) || input.MouseWheel() > 0 {
		step--
	}
	if input.KeyPressed(golib.KeyDown) || input.KeyPressed(golib.KeyS) ||
		input.GamepadPressed(0, golib.GamepadDown) || input.MouseWheel() < 0 {
		step++
	}
	selected = moveSelection(selected, step, len(buttons))

	x, y := input.MousePosition()
	pointed := buttonAt(buttons, x, y)
	clicked := input.MousePressed(golib.MouseLeft) && pointed >= 0
	// The pointer picks the button under it only when it moves or clicks,
	// so a resting pointer doesn't fight the keys.
	if pointed >= 0 && (input.MouseMoved() || clicked) {
		selected = pointed
	}

	confirm := clicked || input.KeyPressed(golib.KeyEnter) ||
		input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA) ||
		input.GamepadPressed(0, golib.GamepadStart)
	return selected, confirm
}

func (s *menuScene) play() {
	state, _, err := resumeState()
	if err != nil {
		log.Print(err)
		s.warning = "Could not load the saved game."
		return
	}
	s.start(state)
}

func (s *menuScene) start(state *State) {
	slot := regionSlot
	if db != nil {
		var err error
		slot, err = db.nextSlot(player)
		if err != nil {
			log.Print(err)
			s.warning = "Could not create a new save slot."
			return
		}
	}
	startGame(slot, state)
}

func startGame(slot string, state *State) {
	play := newPlayScene(state)
	play.slot = slot
	golib.SwitchScene(play)
}

func loadGame(slot string) error {
	if db == nil {
		return fmt.Errorf("loading Save %s: saving is off", slot)
	}
	state, found, err := db.loadState(player, slot)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Save %s was not found", slot)
	}
	startGame(slot, state)
	return nil
}

// Draw paints the fog, the game's name, the player's number and the
// menu. It reads and never changes anything.
func (s *menuScene) Draw(screen *golib.Screen) {
	screen.Clear(fogColor)
	titleY := float32(screenHeight)/2 - 118
	playerY := float32(screenHeight)/2 - 20
	warningY := float32(screenHeight)/2 + 6
	titleSize := float32(64)
	if len(s.buttons) > 2 {
		buttonY := s.buttons[0].bounds.Y
		titleY = max(6, buttonY-148)
		playerY, warningY = buttonY-50, buttonY-24
		if screenHeight < 360 {
			titleSize = 32
		}
	}
	drawCentered(screen, "niebla", titleY, titleSize, textColor)
	drawCentered(screen, "player "+playerDisplay(player), playerY, 16, textColor)
	warning := saveWarning
	if s.warning != "" {
		warning = s.warning
	}
	if warning != "" {
		drawCentered(screen, warning, warningY, 11, groundColor)
	}
	for i, button := range s.buttons {
		button.draw(screen, i == s.selected)
	}
	help := "Enter or click to choose, up, down or the wheel to move, Esc quits"
	if screenWidth < 600 {
		help = "Enter / click: choose   Esc: quit"
	}
	drawCentered(screen, help, float32(screenHeight)-18, 13, groundColor)
}

// drawCentered draws text centered on the screen's width.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	x := (float32(screenWidth) - screen.TextWidth(text, size, uiText)) / 2
	screen.DrawText(text, x, y, size, color, uiText)
}

// menuButton is one on-screen button of the menu.
type menuButton struct {
	label  string
	bounds golib.Rectangle
	action menuAction
}

// buttonAt returns the index of the button holding the point x, y, or -1.
func buttonAt(buttons []menuButton, x, y float32) int {
	for i, button := range buttons {
		if button.bounds.Contains(x, y) {
			return i
		}
	}
	return -1
}

// moveSelection moves the selected button index by step, staying within
// count buttons.
func moveSelection(selected, step, count int) int {
	return max(0, min(selected+step, count-1))
}

// draw paints the button, lit when it is the selected one.
func (b menuButton) draw(screen *golib.Screen, selected bool) {
	fill := buttonColor
	if selected {
		fill = buttonHoverColor
	}
	screen.DrawRectangle(b.bounds, fill)
	screen.DrawRectangleOutline(b.bounds, 2, buttonEdgeColor)
	const size = 16
	x := b.bounds.X + (b.bounds.Width-screen.TextWidth(b.label, size, uiText))/2
	y := b.bounds.Y + (b.bounds.Height-size)/2
	screen.DrawText(b.label, x, y, size, panelTextColor, uiText)
}
