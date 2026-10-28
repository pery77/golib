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
//	  |                 '--time up--> time up --Enter/A--> a new play
//	  |                                  '----Esc/B----> title
//	  '--Esc--> quit

// The camera behind the hovercraft.
const (
	cameraDistance = 8.5  // meters from the point it looks at
	cameraPitch    = -17  // degrees: negative looks down
	cameraLift     = 1.2  // meters over the hovercraft the camera looks at
	cameraLag      = 0.15 // seconds of smoothing
	cameraFar      = 1600 // meters: the water reaches the horizon
	timeUpDelay    = 1.4  // seconds the run goes on showing after the time is up
)

// record is what the game remembers between runs.
type record struct {
	Best int
}

const recordName = "record"

func loadRecord() record {
	var r record
	golib.LoadData(recordName, &r) // nothing saved, or unreadable: start from 0
	return r
}

func newCamera() *golib.Camera3D {
	camera := golib.NewCamera3D(screenWidth, screenHeight)
	camera.Far = cameraFar
	return camera
}

// showcase places camera to circle slowly around the hovercraft, low over
// the water, with the sunset behind it: for the title and the time-up screen.
func showcase(camera *golib.Camera3D, c craft, time float32) {
	yaw := sunsetYaw + 30 + 25*float32(sinf(time*0.15))
	camera.Orbit(c.position.Add(golib.Vector3{Y: 1.1}), yaw, -7, 7.5)
}

// titleScene shows the game's name and controls over the hovercraft idling
// in the sunset.
type titleScene struct {
	world   *world
	fx      effects
	camera  *golib.Camera3D
	record  record
	padType golib.GamepadType // for the buttons' names, when playing with a gamepad
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(), camera: newCamera(), record: loadRecord()}
	showcase(s.camera, s.world.craft, 0)
	s.camera.Snap()
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
	s.padType = input.GamepadType(0)
	s.world.time += dt // the crystals bob and spin; the clock waits
	s.fx.update(s.world.craft, dt)
	showcase(s.camera, s.world.craft, s.world.time)
	s.camera.Update(dt)
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, &s.fx, s.camera)
	x := screen.Width() / 2
	drawText(screen, "SUNSET LAGOON", x, 70, 80, accentColor, centered)
	drawText(screen, "Collect the glowing crystals before the sun goes down: 90 seconds.", x, 170, 20, textColor, centered)
	drawText(screen, "Golden ones are worth 3, but fade away. Mind the poles and the islets.", x, 200, 20, textColor, centered)

	lines := []string{
		"Thrust: Up or W      Brake and reverse: Down or S      Turn: Left and Right, or A and D",
		"Pause: Esc or P      Fullscreen: F11",
		"Press Enter or Space to start",
	}
	if golib.PlayingWithGamepad() {
		a, b, rt, lt, start := buttonNames(s.padType)
		lines = []string{
			fmt.Sprintf("Thrust: %s or %s      Brake and reverse: %s or %s      Turn: left stick or d-pad", a, rt, b, lt),
			"Pause: " + start,
			fmt.Sprintf("Press %s to start", a),
		}
	}
	y := screen.Height() - 170
	drawText(screen, lines[0], x, y, 20, textColor, centered)
	drawText(screen, lines[1], x, y+30, 20, textColor, centered)
	drawText(screen, lines[2], x, y+80, 30, accentColor, centered)
	if s.record.Best > 0 {
		drawText(screen, fmt.Sprintf("Best: %d", s.record.Best), x, 230, 20, accentColor, centered)
	}
}

// playScene is the game itself. Update turns the input into a throttle and
// a steer, advances the world and moves the camera behind the hovercraft;
// Draw draws it all.
type playScene struct {
	world  *world
	fx     effects
	camera *golib.Camera3D
	record record
	ending golib.Timer // runs between the time running out and the time-up screen
}

func newPlayScene(r record) *playScene {
	s := &playScene{world: newWorld(), camera: newCamera(), record: r}
	s.camera.Lag = cameraLag
	s.camera.Heading = s.world.craft.yaw
	s.placeCamera(1.0 / 60)
	s.camera.Snap() // ready to draw before the first update
	return s
}

// placeCamera keeps the camera behind the hovercraft's nose: Follow swings
// round behind the way it is given, and the nose is where the player steers,
// even while the hovercraft drifts sideways.
func (s *playScene) placeCamera(dt float32) {
	c := s.world.craft
	s.camera.Follow(c.position.Add(golib.Vector3{Y: cameraLift}), c.heading(), cameraDistance, cameraPitch, dt)
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if s.ending.Running() {
		s.world.step(0, 0, dt)
		s.fx.update(s.world.craft, dt)
		s.placeCamera(dt)
		s.camera.Update(dt)
		if s.ending.Tick(dt) {
			golib.SwitchScene(newTimeUpScene(s))
		}
		return
	}
	if pausePressed(input) {
		engineSound.Stop()
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	engineSound.Loop()
	throttle, steer := controls(input)
	happened := s.world.step(throttle, steer, dt)
	s.react(happened)
	s.fx.update(s.world.craft, dt)
	s.placeCamera(dt)
	s.camera.Update(dt)
}

// react plays the sounds and the effects for what happened in the world.
func (s *playScene) react(happened events) {
	for _, k := range happened.collected {
		s.fx.burst(k)
		if k.golden {
			goldenSound.Play()
		} else {
			crystalSound.PlayWith(1, golib.RandomFloat(0.95, 1.12))
		}
	}
	if len(happened.faded) > 0 {
		fadeSound.Play()
	}
	if happened.bumped > 2 {
		bumpSound.PlayWith(min(1, happened.bumped/10), golib.RandomFloat(0.9, 1.1))
		s.camera.Shake(min(0.4, happened.bumped/30), 0.3)
	}
	if happened.tick > 0 {
		tickSound.Play()
	}
	if happened.timeUp {
		engineSound.Stop()
		timeUpSound.Play()
		s.ending.Start(timeUpDelay)
		if s.world.score > s.record.Best {
			s.record.Best = s.world.score
			golib.SaveData(recordName, s.record) // a failed save only loses the best score
		}
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, &s.fx, s.camera)
	drawHUD(screen, s.world, &s.fx, s.camera, s.record.Best)
	if s.world.time < 1.5 {
		drawText(screen, "Go!", screen.Width()/2, 160, 60, accentColor, centered)
	}
	if s.world.over {
		drawText(screen, "Time!", screen.Width()/2, 160, 60, accentColor, centered)
	}
	if !golib.WindowFocused() {
		drawText(screen, "Paused: click the window to carry on", screen.Width()/2, screen.Height()-50, 20, textColor, centered)
	}
}

// pauseScene freezes a play scene and shows a message over it.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if pausePressed(input) {
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

// timeUpScene shows the score over the finished run, the camera circling the
// hovercraft.
type timeUpScene struct {
	finished *playScene
	newBest  bool
	camera   *golib.Camera3D
	padType  golib.GamepadType
}

func newTimeUpScene(finished *playScene) *timeUpScene {
	s := &timeUpScene{
		finished: finished,
		newBest:  finished.world.score > 0 && finished.world.score == finished.record.Best,
		camera:   newCamera(),
	}
	showcase(s.camera, finished.world.craft, finished.world.time)
	s.camera.Snap()
	return s
}

func (s *timeUpScene) Update(input *golib.Input, dt float32) {
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
	s.padType = input.GamepadType(0)
	w := s.finished.world
	w.step(0, 0, dt) // the hovercraft drifts to a stop; the crystals spin on
	s.finished.fx.update(w.craft, dt)
	showcase(s.camera, w.craft, w.time)
	s.camera.Update(dt)
}

func (s *timeUpScene) Draw(screen *golib.Screen) {
	w := s.finished.world
	drawWorld(screen, w, &s.finished.fx, s.camera)
	best := fmt.Sprintf("Best: %d", s.finished.record.Best)
	if s.newBest {
		best = "A new best!"
	}
	again := "Enter or Space to play again      Esc for the title"
	if golib.PlayingWithGamepad() {
		a, b, _, _, _ := buttonNames(s.padType)
		again = fmt.Sprintf("%s to play again      %s for the title", a, b)
	}
	drawMessage(screen, "Time's up", fmt.Sprintf("Crystals: %d", w.score), best, "", again)
}

// buttonNames are the bottom and right face buttons, the triggers and Start,
// as printed on the player's kind of gamepad.
func buttonNames(pad golib.GamepadType) (a, b, rightTrigger, leftTrigger, start string) {
	switch pad {
	case golib.GamepadTypePlayStation:
		return "Cross", "Circle", "R2", "L2", "Options"
	case golib.GamepadTypeNintendo:
		return "B", "A", "ZR", "ZL", "+"
	}
	return "A", "B", "RT", "LT", "Menu"
}
