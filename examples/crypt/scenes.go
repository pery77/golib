package main

import (
	"fmt"
	"math"

	"golib"
)

// The game's scenes. golib.Run starts with the title, and golib.SwitchScene
// moves between them.
//
//	title --Enter/A--> play (new) --Esc/P/Start--> pause --Esc/P/Start--> back to the same play
//	  |  --C/Y-----> play (saved)                     '------Q/B------> title
//	  '--Esc--> quit     play --last heart--> game over --Enter/A--> play from the last checkpoint
//	                     play --way out--> won --Enter/A--> title

// record is the best time, kept between runs.
type record struct {
	BestTime float32 // seconds; 0 before the first escape
}

const (
	recordName    = "record"
	progressName  = "progress"
	titleYaw      = 235 // the way the title's camera looks: from the entry hall's north-east corner
	titleSway     = 18  // degrees it sways to either side
	titleSpin     = 6   // degrees per second the game over screen's camera goes round
	wonSway       = 25  // degrees the win screen's camera sways to either side
	wonDelay      = 1.6 // seconds of cheering before the win screen
	crackleRange  = 9   // meters: torches closer than this crackle
	crackleChance = 1.2 // pops per second from each torch close by
)

func loadRecord() record {
	var r record
	golib.LoadData(recordName, &r) // nothing saved, or unreadable: start from nothing
	return r
}

// loadProgress is the saved progress, and whether there is any.
func loadProgress() (progress, bool) {
	var p progress
	found, err := golib.LoadData(progressName, &p)
	if !found || err != nil || p.Checkpoint == 0 && p.KeysFound == 0 {
		return progress{}, false
	}
	return p, true
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
	return (input.KeyPressed(golib.KeyEnter) && !altDown) || input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// paused says whether the player asked to pause, or to go back from a pause.
func pausePressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart)
}

// stopLoops silences the sounds that loop while a game is played.
func stopLoops() {
	fireSound.Stop()
	liftSound.Stop()
}

// titleScene shows the game's name and controls over the entry hall, where
// the astronaut waits, while the camera goes slowly round.
type titleScene struct {
	world   *world
	camera  *golib.Camera3D
	record  record
	saved   progress
	canGoOn bool // there is saved progress to carry on from
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(theLevel, nil), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord()}
	s.camera.FieldOfView = cameraView
	s.saved, s.canGoOn = loadProgress()
	s.place()
	return s
}

// place puts the camera high in a corner of the entry hall, looking at the
// astronaut and the locked door, swaying slowly from side to side.
func (s *titleScene) place() {
	center := s.world.level.start.Add(golib.Vector3{Y: 0.6, Z: 1.5})
	sway := titleSway * float32(math.Sin(float64(s.world.time)*0.3))
	s.camera.Orbit(center, titleYaw+sway, -24, 5)
	s.camera.AvoidWalls(s.world.physics)
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) {
		golib.Quit()
		return
	}
	if s.canGoOn && (input.KeyPressed(golib.KeyC) || input.GamepadPressed(0, golib.GamepadY)) {
		startSound.Play()
		saved := s.saved
		golib.SwitchScene(newPlayScene(newWorld(theLevel, &saved), s.record))
		return
	}
	if confirmed(input) {
		startSound.Play()
		golib.DeleteData(progressName)
		golib.SwitchScene(newPlayScene(newWorld(theLevel, nil), s.record))
		return
	}
	s.world.time += dt
	s.world.animate(dt)
	s.place()
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera, -1)
	screen.DrawRectangle(golib.Rectangle{Width: 640, Height: screen.Height()}, golib.Color{R: 8, G: 6, B: 14, A: 170})
	const x = 56
	drawText(screen, "CRYPT", x, 56, 120, accentColor)
	story := []string{
		"Three keys lie hidden in the crypt: in crates,",
		"and up on the great hall's gallery. Each one",
		"opens a locked door. Find them all and reach",
		"the way out, past the skeleton guards.",
	}
	for i, line := range story {
		drawText(screen, line, x, 196+float32(i)*28, 20, textColor)
	}
	controls := []string{
		"Move: arrows, WASD, d-pad or left stick",
		"Break a crate: click it, or F, Space, X",
		"Camera: Q and E, or the right stick",
		"Pause: Esc, P or Start   Fullscreen: F11",
	}
	for i, line := range controls {
		drawText(screen, line, x, 340+float32(i)*30, 20, golib.WithOpacity(textColor, 0.85))
	}
	drawText(screen, "Press Enter or A to begin", x, 484, 30, accentColor)
	y := float32(534)
	if s.canGoOn {
		drawText(screen, fmt.Sprintf("C or Y: carry on from %s", roomName(s.saved.Checkpoint)), x, y, 20, textColor)
		y += 30
	}
	if s.record.BestTime > 0 {
		drawText(screen, "Best escape "+clock(s.record.BestTime), x, y, 20, textColor)
	}
	drawText(screen, "Esc to quit", x, screen.Height()-50, 20, golib.WithOpacity(textColor, 0.6))
}

// roomName is the name of the room of checkpoint i: "the great hall".
func roomName(i int) string {
	if i < 0 || i >= len(theLevel.checkpoints) {
		return "the start"
	}
	return "the " + theLevel.checkpoints[i].room
}

// playScene is the game itself. Update turns the keys, the mouse and the
// sticks into what the player wants, advances the world, keeps the camera
// behind the astronaut and plays the sounds; Draw draws it all.
type playScene struct {
	world   *world
	camera  *golib.Camera3D
	record  record
	hovered int // the crate under the mouse pointer, or -1
	prompts hudInput
	ending  golib.Timer // runs from reaching the way out to the win screen
}

func newPlayScene(w *world, r record) *playScene {
	return &playScene{world: w, camera: newFollowCamera(w), record: r, hovered: -1, prompts: hudInput{breakPrompt: "", pausePrompt: "Esc: pause"}}
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	s.prompts = prompts(input)
	if pausePressed(input) && !s.world.won {
		stopLoops()
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	s.advance(readControls(input), dt)
	switch {
	case s.world.over:
		stopLoops()
		golib.SwitchScene(newGameOverScene(s))
	case s.world.won && s.ending.Tick(dt):
		stopLoops()
		golib.SwitchScene(newWonScene(s))
	}
}

// advance plays one update of the game with the controls given: the
// camera turns, the world steps with what the player wants, the sounds play
// and the camera follows. The tests call it too, with controls of their own.
func (s *playScene) advance(c controls, dt float32) events {
	s.camera.Heading += c.turn * cameraTurnSpeed * dt
	happened := s.world.step(c.intent(s.camera, s.world), dt)
	s.react(happened)
	if happened.restarted {
		s.camera.Heading = s.world.player.yaw // behind the astronaut at the checkpoint, at once
		follow(s.camera, s.world, dt)
		s.camera.Snap()
	}
	follow(s.camera, s.world, dt)
	s.hovered = s.world.pickCrate(s.camera.Ray(c.mouseX, c.mouseY))
	if s.hovered >= 0 && !s.world.inReach(s.hovered) {
		s.hovered = -1 // only crates in reach light up
	}
	s.sounds(dt)
	return happened
}

// react plays the sounds and starts the effects for what happened in the world.
func (s *playScene) react(happened events) {
	if happened.playerStep {
		playerStepSound.PlayWith(1, golib.RandomFloat(0.85, 1.15))
	}
	for _, at := range happened.steps {
		guardStepSound.PlayAt(at, s.camera)
	}
	for _, at := range happened.spotted {
		shriekSound.PlayAt(at, s.camera)
	}
	if happened.punched {
		punchSound.PlayWith(1, golib.RandomFloat(0.9, 1.15))
	}
	for range happened.broke {
		smashSound.PlayWith(1, golib.RandomFloat(0.9, 1.1))
		s.camera.Shake(0.05, 0.15)
	}
	if happened.keyTaken {
		keySound.Play()
	}
	for range happened.opened {
		doorSound.Play()
		s.camera.Shake(0.08, 0.5)
	}
	if happened.caught {
		caughtSound.Play()
		s.camera.Shake(0.15, 0.4)
	}
	if happened.gameOver {
		overSound.Play()
		golib.DeleteData(progressName)
	}
	if happened.checkpoint {
		checkpointSound.Play()
		golib.SaveData(progressName, s.world.progress()) // a failed save only loses the checkpoint
	}
	if happened.won {
		winSound.Play()
		s.ending.Start(wonDelay)
		golib.DeleteData(progressName)
		if s.record.BestTime == 0 || s.world.time < s.record.BestTime {
			s.record.BestTime = s.world.time
			golib.SaveData(recordName, s.record) // a failed save only loses the record
		}
	}
	if happened.locked {
		lockedSound.Play()
	}
	lift := s.world.lift.transform().Position
	if happened.liftMoving {
		liftSound.LoopAt(lift, s.camera)
	} else {
		liftSound.Stop()
	}
	if happened.liftStopped {
		liftStopSound.PlayAt(lift, s.camera)
	}
}

// sounds keeps the torch nearest the astronaut roaring softly, where it is,
// and makes the torches close by crackle now and then.
func (s *playScene) sounds(dt float32) {
	feet := s.world.player.body.Position
	nearest, best := -1, float32(math.MaxFloat32)
	for i, at := range s.world.level.torches {
		d := at.Distance(feet)
		if d < best {
			nearest, best = i, d
		}
		if d < crackleRange && golib.RandomFloat(0, 1) < crackleChance*dt {
			crackleSounds[golib.RandomInt(0, len(crackleSounds)-1)].PlayAt(at, s.camera)
		}
	}
	if nearest >= 0 {
		fireSound.LoopAt(s.world.level.torches[nearest], s.camera)
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera, s.hovered)
	drawHUD(screen, s.world, s.camera, s.prompts)
	if s.world.player.caught.Running() && s.world.hearts > 0 {
		drawText(screen, "Grabbed!", screen.Width()/2, screen.Height()*0.3, 60, alertColor, centered)
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
	drawMessage(screen, "Paused", "Esc, P or Start to carry on", "Q or B to go back to the title (checkpoints are kept)")
}

// gameOverScene shows that the guards got the astronaut, over the crypt.
type gameOverScene struct {
	finished *playScene
	camera   *golib.Camera3D
	turn     float32
}

func newGameOverScene(finished *playScene) *gameOverScene {
	s := &gameOverScene{finished: finished, camera: golib.NewCamera3D(screenWidth, screenHeight), turn: finished.camera.Heading}
	s.camera.FieldOfView = cameraView
	s.place()
	return s
}

func (s *gameOverScene) place() {
	s.camera.Orbit(s.finished.world.player.body.Position.Add(golib.Vector3{Y: 0.4}), s.turn, -35, 4.5)
	s.camera.AvoidWalls(s.finished.world.physics)
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
		return
	}
	if confirmed(input) {
		startSound.Play()
		saved := s.finished.world.progress()
		saved.Hearts = startHearts
		golib.SwitchScene(newPlayScene(newWorld(theLevel, &saved), s.finished.record))
		return
	}
	s.finished.world.animate(dt)
	s.turn += titleSpin * dt
	s.place()
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.finished.world, s.camera, -1)
	drawMessage(screen, "The guards got you", fmt.Sprintf("Keys found: %d of %d", s.finished.world.keysFound, s.finished.world.level.keysInLevel()), "",
		"Enter or A to try again from "+roomName(s.finished.world.checkpoint), "Esc or B for the title")
}

// wonScene shows the astronaut cheering at the way out, with the time.
type wonScene struct {
	finished *playScene
	camera   *golib.Camera3D
	turn     float32 // the yaw the camera looks along
	swayTime float32 // seconds on the win screen
}

func newWonScene(finished *playScene) *wonScene {
	s := &wonScene{finished: finished, camera: golib.NewCamera3D(screenWidth, screenHeight), turn: finished.world.player.yaw}
	s.camera.FieldOfView = cameraView
	s.place()
	return s
}

// place looks at the astronaut from behind and above, cheering in front of
// the way out, swaying a little from side to side.
func (s *wonScene) place() {
	sway := wonSway * float32(math.Sin(float64(s.swayTime)*0.5))
	s.camera.Orbit(s.finished.world.player.body.Position.Add(golib.Vector3{Y: 0.6}), s.turn+sway, -18, 4)
	s.camera.AvoidWalls(s.finished.world.physics)
}

func (s *wonScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if confirmed(input) || input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene())
		return
	}
	s.finished.world.animate(dt) // the clock stopped at the way out
	s.swayTime += dt
	s.place()
}

func (s *wonScene) Draw(screen *golib.Screen) {
	w := s.finished.world
	drawWorld(screen, w, s.camera, -1)
	best := "Best escape " + clock(s.finished.record.BestTime)
	if s.finished.record.BestTime == w.time {
		best = "Your best escape yet!"
	}
	drawMessage(screen, "You escaped the crypt!", fmt.Sprintf("Time %s, with %d of %d hearts", clock(w.time), w.hearts, startHearts), best, "",
		"Enter or A for the title")
}
