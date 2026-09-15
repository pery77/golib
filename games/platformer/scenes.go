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
//	  |              '--last coin--> won --Enter/A--> a new play
//	  |                               '----Esc/B----> title
//	  '--Quit or Esc--> quit
//
// Every scene reads the keyboard and gamepad 0; the title also reads the mouse.

// Title screen buttons, as indexes into titleScene.buttons.
const (
	playButton = iota
	quitButton
)

// titleScene shows the game's name, its controls and a menu with two buttons.
type titleScene struct {
	clouds         []cloud
	buttons        []menuButton
	selected       int     // the highlighted button
	mouseX, mouseY float32 // the pointer in the previous update, to notice when it moves
	gamepad        string  // name of gamepad 0, or "" when none is connected
}

func newTitleScene() *titleScene {
	const width, height = 240, 56
	x := float32(levelWidth-width) / 2
	return &titleScene{
		clouds: newClouds(),
		buttons: []menuButton{
			playButton: {label: "Play", bounds: golib.Rectangle{X: x, Y: 450, Width: width, Height: height}},
			quitButton: {label: "Quit", bounds: golib.Rectangle{X: x, Y: 526, Width: width, Height: height}},
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
	// resting pointer doesn't fight the keys.
	x, y := input.MousePosition()
	pointed := buttonAt(s.buttons, x, y)
	clicked := input.MousePressed(golib.MouseLeft) && pointed >= 0
	if pointed >= 0 && (x != s.mouseX || y != s.mouseY || clicked) {
		s.selected = pointed
	}
	s.mouseX, s.mouseY = x, y

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
	screen.Clear(skyColor)
	drawClouds(screen, s.clouds)
	drawCentered(screen, "Platformer", 110, 80, textColor)
	drawCentered(screen, "Collect every coin", 210, 30, textColor)
	drawCentered(screen, "Walk: arrows, A/D, d-pad or left stick", 290, 24, textColor)
	drawCentered(screen, "Jump: Space, Up, W or the A button", 326, 24, textColor)
	drawCentered(screen, "Pause: Esc or Start", 362, 24, textColor)
	for i, button := range s.buttons {
		button.draw(screen, i == s.selected)
	}
	drawCentered(screen, "Choose with the arrows, d-pad, mouse wheel or pointer; confirm with Enter, A or a click", 620, 20, textColor)
	if s.gamepad != "" {
		drawCentered(screen, "Gamepad: "+s.gamepad, 656, 20, textColor)
	}
}

// playScene is the game itself. Update turns keys and gamepad buttons into
// actions for the world, which holds the rules (world.go), and Draw draws the
// world.
type playScene struct {
	world  world
	clouds []cloud
}

func newPlayScene() *playScene {
	return &playScene{world: newWorld(), clouds: newClouds()}
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
	// holding the button down jumps only once.
	jump := input.KeyPressed(golib.KeySpace) || input.KeyPressed(golib.KeyUp) || input.KeyPressed(golib.KeyW) ||
		input.GamepadPressed(0, golib.GamepadA)

	s.world.step(move, jump, dt)
	if s.world.won {
		golib.SwitchScene(&wonScene{finished: s})
	}
}

// Draw draws the world from back to front. It reads the state and never
// changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(skyColor)
	drawClouds(screen, s.clouds)
	for _, platform := range s.world.platforms {
		screen.DrawRectangle(platform, platformColor)
	}
	for _, c := range s.world.coins {
		if !c.collected {
			screen.DrawCircle(c.x, c.y, coinRadius, coinColor)
		}
	}
	screen.DrawRectangle(s.world.player.bounds(), playerColor)

	coins := fmt.Sprintf("Coins: %d/%d", s.world.coinsCollected(), len(s.world.coins))
	screen.DrawText(coins, 20, 20, 30, textColor)
	screen.DrawText("Esc or Start to pause", 20, 60, 20, textColor)
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
	drawMessage(screen, "Paused", "Esc or Start to resume, Q or B to quit to the title")
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
	drawMessage(screen, "You win!", "Enter or A to play again, Esc or B for the title")
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
	const size = 32
	x := b.bounds.X + (b.bounds.Width-screen.TextWidth(b.label, size))/2
	y := b.bounds.Y + (b.bounds.Height-size)/2
	screen.DrawText(b.label, x, y, size, textColor)
}

// cloud is a background decoration made of three circles.
type cloud struct {
	x, y   float32 // center of the middle circle, in pixels
	radius float32 // radius of the middle circle, in pixels
}

// newClouds scatters clouds across the upper sky. They are random, so each
// play looks a little different; golib shot always gets the same ones.
func newClouds() []cloud {
	clouds := make([]cloud, cloudCount)
	for i := range clouds {
		clouds[i] = cloud{
			x:      golib.RandomFloat(0, levelWidth),
			y:      golib.RandomFloat(100, 250),
			radius: golib.RandomFloat(20, 36),
		}
	}
	return clouds
}

func drawClouds(screen *golib.Screen, clouds []cloud) {
	for _, c := range clouds {
		screen.DrawCircle(c.x-c.radius, c.y+c.radius/3, c.radius*0.7, cloudColor)
		screen.DrawCircle(c.x+c.radius, c.y+c.radius/3, c.radius*0.7, cloudColor)
		screen.DrawCircle(c.x, c.y, c.radius, cloudColor)
	}
}

// drawMessage darkens the whole screen and shows a heading and a hint in the
// middle.
func drawMessage(screen *golib.Screen, heading, hint string) {
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, overlayColor)
	drawCentered(screen, heading, 280, 60, messageColor)
	drawCentered(screen, hint, 370, 30, messageColor)
}

// drawCentered draws text centered across the screen, with its top at y.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, (screen.Width()-screen.TextWidth(text, size))/2, y, size, color)
}
