package main

import (
	"fmt"

	"golib"
)

// The game's scenes. golib.Run starts with the title, and golib.SwitchScene
// moves between them.
//
//	title --Enter/Space/A--> play --Esc/P/Start--> pause --Esc/P/Start--> back to the same play
//	  |                       |                       '------Q/B------> title
//	  |                       |--the flag--> won --Enter/A--> a new play
//	  |                       '--last life--> game over --Enter/A--> a new play
//	  '--Esc--> quit            (won and game over: Esc or B for the title)

// record is what the game remembers between runs.
type record struct {
	BestTime  float32 // seconds to the flag; 0 before the first win
	MostCoins int     // the most coins in a won game
}

const (
	recordName   = "record"
	endingDelay  = 1.4 // seconds the game goes on showing after the flag or the last life
	respawnBlink = 1.0 // seconds the character blinks after starting again
)

func loadRecord() record {
	var r record
	golib.LoadData(recordName, &r) // nothing saved, or unreadable: start from nothing
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

// overview places camera high over the level, facing yaw, for the title and
// the end screens.
func overview(camera *golib.Camera3D, yaw float32) {
	middle := theLevel.start.Add(theLevel.goal).Scale(0.5)
	camera.Orbit(middle, yaw, -32, 34)
}

// titleScene shows the game's name and controls over the level, which turns
// slowly.
type titleScene struct {
	world  *world
	camera *golib.Camera3D
	record record
	yaw    float32
	time   float32
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(theLevel), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord(), yaw: 35}
	overview(s.camera, s.yaw)
	return s
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
	s.time += dt
	s.yaw += 6 * dt
	overview(s.camera, s.yaw)
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera, look{time: s.time})
	screen.DrawRectangle(golib.Rectangle{Width: screen.Width(), Height: screen.Height()}, golib.Color{R: 10, G: 20, B: 50, A: 100})
	x := screen.Width() / 2
	drawText(screen, "HOPPER", x, 80, 100, accentColor, centered)
	drawText(screen, "Hop across the floating blocks, pick up the coins and reach the flag.", x, 210, 20, textColor, centered)
	drawText(screen, "Three lives: a fall costs one, and you start again at the last checkpoint.", x, 240, 20, textColor, centered)
	drawText(screen, "Run: arrows, WASD, d-pad or left stick", x, 310, 20, textColor, centered)
	drawText(screen, "Jump: Space or A (hold it to jump higher)", x, 340, 20, textColor, centered)
	drawText(screen, "Camera: Q and E or right stick      Pause: Esc, P or Start      Fullscreen: F11", x, 370, 20, textColor, centered)
	drawText(screen, "Press Enter, Space or A to play", x, 460, 30, accentColor, centered)
	if s.record.BestTime > 0 {
		drawText(screen, fmt.Sprintf("Best time: %s   Most coins: %d / %d", formatTime(s.record.BestTime), s.record.MostCoins, len(theLevel.coins)), x, 520, 20, textColor, centered)
	}
	drawText(screen, "Esc to quit", x, screen.Height()-40, 20, golib.WithOpacity(textColor, 0.7), centered)
}

// playScene is the game itself. Update turns the keys and the sticks into
// what the player wants, advances the world, and moves the camera behind the
// character; Draw draws it all.
type playScene struct {
	world     *world
	camera    *golib.Camera3D
	cameraYaw float32       // degrees, as golib.Camera3D.Orbit takes it: 0 looks towards +Z
	looksAt   golib.Vector3 // the point the camera orbits, ahead of the character
	record    record
	look      look
	blink     golib.Timer // runs after starting again
	ending    golib.Timer // runs between the flag or the last life and the end screen
	newBest   bool
}

func newPlayScene(r record) *playScene {
	s := &playScene{world: newWorld(theLevel), camera: golib.NewCamera3D(screenWidth, screenHeight), record: r}
	s.look.animator = golib.NewAnimator(characterModel)
	s.cameraYaw = theLevel.startYaw
	s.camera.Lag = cameraLag
	s.placeCamera()
	s.camera.Snap() // ready to draw before the first update
	return s
}

// placeCamera puts the camera behind the character, looking a little ahead
// of it and above its feet. While it falls, the camera stays where it was
// and watches it go.
func (s *playScene) placeCamera() {
	if !s.world.falling() {
		ahead := golib.Rotation{Yaw: s.cameraYaw}.Forward().Scale(cameraAhead)
		s.looksAt = s.world.player.position.Add(ahead).Add(golib.Vector3{Y: cameraLookUp})
	}
	s.camera.Orbit(s.looksAt, s.cameraYaw, cameraPitch, cameraDistance)
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	s.look.time += dt
	if s.ending.Running() {
		if s.ending.Tick(dt) {
			if s.world.won {
				golib.SwitchScene(newWonScene(s))
			} else {
				golib.SwitchScene(newGameOverScene(s))
			}
			return
		}
		if s.world.won {
			s.cameraYaw += 40 * dt // swing round to the character's face
			s.placeCamera()
		}
		animate(s.look.animator, s.world.player, s.world.won, dt)
		s.camera.Update(dt)
		return
	}
	if input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}

	s.cameraYaw += cameraTurn(input) * dt
	happened := s.world.step(readIntent(input, s.cameraYaw), dt)
	s.react(happened)

	p := s.world.player
	animate(s.look.animator, p, false, dt)
	if !s.world.falling() {
		s.cameraYaw = followYaw(s.cameraYaw, p.velocity, dt)
	}
	s.placeCamera()
	if happened.lostLife && !happened.gameOver {
		s.cameraYaw = s.world.respawnYaw
		s.placeCamera()
		s.camera.Snap() // the character starts again: no swoop there
	}
	s.camera.Update(dt)
	s.blink.Tick(dt)
	s.look.blink = s.blink.Running() && int(s.blink.Left()/blinkInterval)%2 == 0
	s.look.hideAll = s.world.over
}

// react plays the sounds and starts the effects for what happened in the world.
func (s *playScene) react(happened events) {
	if happened.jumped {
		jumpSound.PlayWith(0.6, golib.RandomFloat(0.95, 1.08))
	}
	if happened.landed > 6 {
		landSound.PlayWith(golib.Clamp(happened.landed/15, 0.3, 1), golib.RandomFloat(0.9, 1.1))
	}
	if happened.coins > 0 {
		coinSound.PlayWith(1, golib.RandomFloat(0.95, 1.1))
	}
	if happened.checkpoint {
		checkpointSound.Play()
	}
	if happened.fell {
		fallSound.Play()
		s.blink.Start(respawnBlink)
	}
	if happened.gameOver {
		gameOverSound.Play()
		s.ending.Start(endingDelay)
	}
	if happened.won {
		winSound.Play()
		s.ending.Start(endingDelay)
		s.saveRecord()
	}
}

// saveRecord keeps the best time and the most coins after a win.
func (s *playScene) saveRecord() {
	w := s.world
	if s.record.BestTime == 0 || w.time < s.record.BestTime {
		s.record.BestTime = w.time
		s.newBest = true
	}
	s.record.MostCoins = max(s.record.MostCoins, w.collected)
	golib.SaveData(recordName, s.record) // a failed save only loses the record
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera, s.look)
	drawHUD(screen, s.world)
	x := screen.Width() / 2
	if s.world.won {
		drawText(screen, "You made it!", x, 150, 60, accentColor, centered)
	} else if s.world.falling() && !s.world.over {
		drawText(screen, "Whoops!", x, 150, 40, textColor, centered)
	}
	drawText(screen, "Esc: pause", screen.Width()-20, 20, 20, golib.WithOpacity(textColor, 0.8), golib.TextOptions{Align: golib.AlignRight})
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

// endScene is what the won and game over scenes share: the finished game,
// seen from above, turning slowly, and the keys to play again.
type endScene struct {
	finished *playScene
	camera   *golib.Camera3D
	yaw      float32
}

func newEndScene(finished *playScene) endScene {
	s := endScene{finished: finished, camera: golib.NewCamera3D(screenWidth, screenHeight), yaw: finished.cameraYaw}
	overview(s.camera, s.yaw)
	return s
}

func (s *endScene) update(input *golib.Input, dt float32) {
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
	s.finished.look.time += dt
	s.finished.look.animator.Update(dt)
	s.yaw += 6 * dt
	overview(s.camera, s.yaw)
}

// wonScene shows the time and the coins after reaching the flag.
type wonScene struct{ endScene }

func newWonScene(finished *playScene) *wonScene { return &wonScene{newEndScene(finished)} }

func (s *wonScene) Update(input *golib.Input, dt float32) { s.update(input, dt) }

func (s *wonScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.finished.world, s.camera, s.finished.look)
	w := s.finished.world
	best := fmt.Sprintf("Best time: %s", formatTime(s.finished.record.BestTime))
	if s.finished.newBest {
		best = "A new best time!"
	}
	coins := fmt.Sprintf("Coins: %d / %d", w.collected, len(w.level.coins))
	if w.collected == len(w.level.coins) {
		coins += "  Every one!"
	}
	drawMessage(screen, "You reached the flag!", "Time: "+formatTime(w.time), coins, best, "",
		"Enter, Space or A to play again", "Esc or B for the title")
}

// gameOverScene shows how far the game went after the last life.
type gameOverScene struct{ endScene }

func newGameOverScene(finished *playScene) *gameOverScene {
	return &gameOverScene{newEndScene(finished)}
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) { s.update(input, dt) }

func (s *gameOverScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.finished.world, s.camera, s.finished.look)
	w := s.finished.world
	drawMessage(screen, "Game over", fmt.Sprintf("Coins: %d / %d", w.collected, len(w.level.coins)), "Time: "+formatTime(w.time), "",
		"Enter, Space or A to try again", "Esc or B for the title")
}
