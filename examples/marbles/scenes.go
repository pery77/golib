package main

import (
	"fmt"

	"golib"
)

// The game's scenes. golib.Run starts with the title, and golib.SwitchScene
// moves between them.
//
//	title --Enter/A--> play --Esc/Start--> pause --Esc/Start--> back to the same play
//	  |                 |                    '------Q/B------> title
//	  |                 '--last life--> game over --Enter/A--> a new play
//	  |                                      '----Esc/B----> title
//	  '--Esc--> quit

// record is what the game remembers between runs.
type record struct {
	Best int
}

const (
	recordName    = "record"
	gameOverDelay = 1.2 // seconds the game goes on showing after the last life, before the game over screen
	bannerTime    = 1.6 // seconds "Faster!" shows
)

func loadRecord() record {
	var r record
	golib.LoadData(recordName, &r) // nothing saved, or unreadable: start from 0
	return r
}

// toggleFullscreen switches fullscreen on F11 or Alt+Enter, in every scene.
func toggleFullscreen(input *golib.Input) {
	altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) && input.KeyPressed(golib.KeyEnter)
	if input.KeyPressed(golib.KeyF11) || altEnter {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
}

// confirmed says whether the player pressed a key or button that starts or accepts.
func confirmed(input *golib.Input) bool {
	altDown := input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
	return (input.KeyPressed(golib.KeyEnter) && !altDown) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart) ||
		input.MousePressed(golib.MouseLeft)
}

// titleScene shows the game's name and controls over the platform, which
// turns slowly.
type titleScene struct {
	world  *world
	camera *golib.Camera3D
	record record
	yaw    float32
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord(), yaw: 30}
	s.placeCamera()
	s.camera.Snap()
	return s
}

func (s *titleScene) placeCamera() {
	overview(s.camera, s.yaw)
}

// overview places camera high over the platform, facing yaw, for the title
// and the game over screens.
func overview(camera *golib.Camera3D, yaw float32) {
	camera.Orbit(golib.Vector3{Y: 5}, yaw, -30, 36)
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) {
		golib.Quit()
		return
	}
	if confirmed(input) {
		startSound.Play()
		golib.SwitchScene(newPlayScene(s.record))
		return
	}
	s.world.time += dt // the gems bob and spin
	s.yaw += 8 * dt
	s.placeCamera()
	s.camera.Update(dt)
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, golib.Color{R: 10, G: 20, B: 50, A: 110})
	x := screen.Width() / 2
	drawText(screen, "MARBLES", x, 90, 100, accentColor, centered)
	drawText(screen, "Roll into the gems. Keep away from the red cubes, and from the edge.", x, 220, 20, textColor, centered)
	drawText(screen, "Every 5 gems the cubes get faster. Three lives.", x, 250, 20, textColor, centered)
	drawText(screen, "Roll: arrows, WASD, d-pad or left stick", x, 320, 20, textColor, centered)
	drawText(screen, "Turn the camera: right stick", x, 350, 20, textColor, centered)
	drawText(screen, "Pause: Esc, P or Start      Fullscreen: F11", x, 380, 20, textColor, centered)
	drawText(screen, "Press Enter, Space or A to play", x, 470, 30, accentColor, centered)
	if s.record.Best > 0 {
		drawText(screen, fmt.Sprintf("Best: %d", s.record.Best), x, 530, 20, textColor, centered)
	}
	drawText(screen, "Esc to quit", x, screen.Height()-40, 20, golib.WithOpacity(textColor, 0.7), centered)
}

// playScene is the game itself. Update turns the keys and the stick into a
// direction from where the camera looks, advances the world, and moves the
// camera behind the marble; Draw draws it all.
type playScene struct {
	world     *world
	camera    *golib.Camera3D
	cameraYaw float32       // degrees, as golib.Camera3D.Orbit takes it: 0 looks towards +Z
	looksAt   golib.Vector3 // the point the camera orbits, ahead of the marble
	record    record
	banner    golib.Timer // "Faster!" shows while it runs
	ending    golib.Timer // runs between the last life and the game over screen
}

func newPlayScene(r record) *playScene {
	s := &playScene{world: newWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight), record: r}
	s.camera.Lag = cameraLag
	s.placeCamera()
	s.camera.Snap() // ready to draw before the first update
	return s
}

// placeCamera puts the camera behind the marble, looking a little ahead of
// it, so the player sees where they roll. While the marble falls, the camera
// stays where it was and watches it go.
func (s *playScene) placeCamera() {
	if !s.world.marble.falling {
		ahead := golib.Rotation{Yaw: s.cameraYaw}.Forward().Scale(cameraAhead)
		s.looksAt = s.world.marble.position.Add(ahead)
	}
	s.camera.Orbit(s.looksAt, s.cameraYaw, cameraPitch, cameraDistance)
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if s.ending.Running() {
		s.world.step(golib.Vector3{}, dt)
		if s.ending.Tick(dt) {
			golib.SwitchScene(newGameOverScene(s))
		}
		return
	}
	if input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}

	// Keys and the d-pad roll at full tilt; the stick is analog, so tilted
	// halfway it pushes half as hard. The stick's y grows downwards.
	right, down := input.GamepadLeftStick(0)
	forward := -down
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		right = -1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		right = 1
	}
	if input.KeyDown(golib.KeyUp) || input.KeyDown(golib.KeyW) || input.GamepadDown(0, golib.GamepadUp) {
		forward = 1
	}
	if input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown) {
		forward = -1
	}
	turn, _ := input.GamepadRightStick(0)
	s.cameraYaw -= turn * cameraTurnSpeed * dt // a larger yaw turns left

	happened := s.world.step(rollDirection(s.cameraYaw, forward, right), dt)
	s.react(happened)

	if !s.world.marble.falling {
		s.cameraYaw = followYaw(s.cameraYaw, s.world.marble.velocity, dt)
	}
	s.placeCamera()
	if happened.lostLife && !happened.gameOver {
		s.camera.Snap() // the marble starts again in the middle: no swoop there
	}
	s.camera.Update(dt)
	s.banner.Tick(dt)
}

// react plays the sounds and the effects for what happened in the world.
func (s *playScene) react(happened events) {
	if happened.collected > 0 {
		gemSound.PlayWith(1, golib.RandomFloat(0.95, 1.1))
	}
	if happened.levelUp {
		levelUpSound.Play()
		s.banner.Start(bannerTime)
	}
	if happened.fell {
		fallSound.Play()
	}
	if happened.hit {
		hitSound.Play()
		s.camera.Shake(0.3, 0.4)
	}
	if happened.gameOver {
		gameOverSound.Play()
		s.ending.Start(gameOverDelay)
		if s.world.score > s.record.Best {
			s.record.Best = s.world.score
			golib.SaveData(recordName, s.record) // a failed save only loses the best score
		}
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	drawHUD(screen, s.world, s.record.Best)
	x := screen.Width() / 2
	if s.banner.Running() {
		size := golib.Lerp(40, 60, golib.EaseOut(min(1, s.banner.Progress()*4)))
		drawText(screen, "Faster!", x, 150, float32(int(size/10)*10), accentColor, centered)
	}
	if s.world.grace.Running() && !s.world.over {
		drawText(screen, "Go!", x, 150, 60, textColor, centered)
	}
	if s.world.marble.falling && !s.world.over {
		drawText(screen, "Over the edge!", x, 150, 40, textColor, centered)
	}
	hint := "Esc: pause"
	drawText(screen, hint, screen.Width()-20, 20, 20, golib.WithOpacity(textColor, 0.8), golib.TextOptions{Align: golib.AlignRight})
	if !golib.WindowFocused() {
		drawText(screen, "Paused: click the window to carry on", x, screen.Height()-40, 20, textColor, centered)
	}
}

// pauseScene freezes a play scene and shows a message over it.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(s.paused)
		return
	}
	if input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawMessage(screen, "Paused", "Esc, P or Start to carry on", "Q or B to go back to the title")
}

// gameOverScene shows the score over the finished game.
type gameOverScene struct {
	finished *playScene
	newBest  bool
	camera   *golib.Camera3D // turns slowly over the platform
	yaw      float32
}

func newGameOverScene(finished *playScene) *gameOverScene {
	s := &gameOverScene{
		finished: finished,
		newBest:  finished.world.score > 0 && finished.world.score == finished.record.Best,
		camera:   golib.NewCamera3D(screenWidth, screenHeight),
		yaw:      finished.cameraYaw,
	}
	overview(s.camera, s.yaw)
	return s
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
		return
	}
	if confirmed(input) {
		startSound.Play()
		golib.SwitchScene(newPlayScene(s.finished.record))
		return
	}
	s.finished.world.time += dt // the gems go on spinning
	s.yaw += 8 * dt
	overview(s.camera, s.yaw)
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.finished.world, s.camera)
	best := fmt.Sprintf("Best: %d", s.finished.record.Best)
	if s.newBest {
		best = "A new best!"
	}
	drawMessage(screen, "Game over", fmt.Sprintf("Score: %d", s.finished.world.score), best, "",
		"Enter, Space or A to play again", "Esc or B for the title")
}
