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
//	  |              '--last chest--> won --Enter/A--> a new play
//	  |                                '----Esc/B----> title
//	  '--Quit or Esc--> quit
//
// Every scene reads the keyboard and gamepad 0; the title also reads the mouse.

// Title screen buttons, as indexes into titleScene.buttons.
const (
	playButton = iota
	quitButton
)

// titleScene shows the game's name, its controls and a menu with two buttons,
// over the level.
type titleScene struct {
	buttons  []menuButton
	selected int    // the highlighted button
	gamepad  string // name of gamepad 0, or "" when none is connected
}

func newTitleScene() *titleScene {
	const width, height = 72, 14
	x := float32(screenWidth-width) / 2
	return &titleScene{
		buttons: []menuButton{
			playButton: {label: "Play", bounds: golib.Rectangle{X: x, Y: 118, Width: width, Height: height}},
			quitButton: {label: "Quit", bounds: golib.Rectangle{X: x, Y: 136, Width: width, Height: height}},
		},
		selected: playButton,
	}
}

// Update moves the selection with the Up and Down arrows, the d-pad, the mouse
// wheel or the pointer, and confirms it with Enter, A, Start or a click.
func (s *titleScene) Update(input *golib.Input, dt float32) {
	s.gamepad = input.GamepadName(0)

	step := 0
	if input.KeyPressed(golib.KeyUp) || input.GamepadPressed(0, golib.GamepadUp) || input.MouseWheel() > 0 {
		step--
	}
	if input.KeyPressed(golib.KeyDown) || input.GamepadPressed(0, golib.GamepadDown) || input.MouseWheel() < 0 {
		step++
	}
	s.selected = moveSelection(s.selected, step, len(s.buttons))

	// The pointer picks the button under it when it moves or clicks, so a
	// resting pointer doesn't fight the keys, nor picks a button as the
	// menu opens under it.
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
		golib.SwitchScene(newPlayScene())
	}
}

func (s *titleScene) Draw(screen *golib.Screen) {
	// DrawMap draws every visible layer of the level, with its background
	// color and its chests, here with the level's bottom on the screen's.
	screen.Clear(skyColor)
	screen.DrawMap(level, 0, screenHeight-level.Height())
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, overlayColor)

	drawCentered(screen, "Platformer", 14, 30)
	drawCentered(screen, "Open every chest in the forest", 50, 10)
	drawCentered(screen, "Walk: arrows, A/D, d-pad or left stick", 66, 10)
	drawCentered(screen, "Jump: Space, Up, W or the A button", 78, 10)
	drawCentered(screen, "Slash: X, J or the X button", 90, 10)
	drawCentered(screen, "Pause: Esc or Start", 102, 10)
	for i, button := range s.buttons {
		button.draw(screen, i == s.selected)
	}
	drawCentered(screen, "Arrows, wheel or pointer; Enter, A or click", 156, 10)
	if s.gamepad != "" {
		drawCentered(screen, "Gamepad: "+s.gamepad, 168, 10)
	}
}

// playScene is the game itself. Update turns keys and gamepad buttons into
// actions for the world, which holds the rules (world.go), and Draw draws the
// world (draw.go).
type playScene struct {
	world  world
	clouds []cloud
	camera *golib.Camera // follows the player, inside the level
}

func newPlayScene() *playScene {
	s := &playScene{world: newWorld(), clouds: newClouds(), camera: golib.NewCamera(screenWidth, screenHeight)}
	s.camera.Bounds = golib.Rectangle{Width: s.world.width, Height: s.world.height}
	s.camera.Target = s.world.player.bounds().Center()
	s.camera.Snap() // ready to draw before the first update
	return s
}

// Update reads the keyboard and gamepad 0, and advances the world. golib.Run
// calls it 60 times per second, always with dt = 1/60.
func (s *playScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}

	var move float32
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		move--
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		move++
	}
	if move == 0 {
		// The stick is analog: tilted halfway, the player walks at half speed.
		move, _ = input.GamepadLeftStick(0)
	}
	// KeyPressed and GamepadPressed are true for a single update per press, so
	// holding the button down jumps or slashes only once.
	jump := input.KeyPressed(golib.KeySpace) || input.KeyPressed(golib.KeyUp) || input.KeyPressed(golib.KeyW) ||
		input.GamepadPressed(0, golib.GamepadA)
	slash := input.KeyPressed(golib.KeyX) || input.KeyPressed(golib.KeyJ) || input.GamepadPressed(0, golib.GamepadX)

	s.world.step(move, jump, slash, dt)
	s.camera.Target = s.world.player.bounds().Center()
	s.camera.Update(dt)
	if s.world.won {
		golib.SwitchScene(&wonScene{finished: s})
	}
}

// Draw draws the world, then the count of chests over it. It reads the state
// and never changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, &s.world, s.clouds, s.camera)
	chests := fmt.Sprintf("Chests: %d/%d", s.world.chestsOpened(), len(s.world.chests))
	drawShadowed(screen, chests, 4, 4, 10)
	const hint = "Esc: pause"
	drawShadowed(screen, hint, screenWidth-4, 4, 10, golib.TextOptions{Align: golib.AlignRight})
	if !golib.WindowFocused() {
		// Config.PauseUnfocused stops the updates meanwhile, so say so:
		// Draw still runs, and the level is standing still.
		drawShadowed(screen, "Paused: click the window to carry on", screenWidth/2, screenHeight-14, 10,
			golib.TextOptions{Align: golib.AlignCenter})
	}
}

// pauseScene freezes a play scene and shows a message over it. Resuming
// switches back to that same play scene, so the game carries on where it
// stopped.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(s.paused)
	}
	if input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawMessage(screen, "Paused", "Esc or Start to resume", "Q or B to quit to the title")
}

// wonScene shows the finished level under a victory message.
type wonScene struct {
	finished *playScene
}

func (s *wonScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyEnter) || input.GamepadPressed(0, golib.GamepadA) {
		golib.SwitchScene(newPlayScene())
	}
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
	}
}

func (s *wonScene) Draw(screen *golib.Screen) {
	s.finished.Draw(screen)
	drawMessage(screen, "You win!", "Enter or A to play again", "Esc or B for the title")
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
	color := buttonColor
	if selected {
		color = buttonSelectedColor
	}
	screen.DrawRectangle(b.bounds, color)
	const size = 10
	x := b.bounds.X + (b.bounds.Width-screen.TextWidth(b.label, size))/2
	y := b.bounds.Y + (b.bounds.Height-size)/2
	drawShadowed(screen, b.label, x, y, size)
}
