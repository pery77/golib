package main

import (
	"fmt"
	"math"

	"golib"
)

// The game's scenes. golib.Run starts with the title, and golib.SwitchScene
// moves between them.
//
//	title --Enter/Space/A--> play --Esc/P/Start--> pause --Esc/P/Start--> back to the same play
//	  |                       |                       '------Q/B------> title
//	  |                       '--last heart--> game over --Enter/A--> a new play
//	  '--Esc--> quit                            (game over: Esc or B for the title)

// record is what the game remembers between runs.
type record struct {
	BestScore int
	BestWave  int
}

const (
	recordName   = "record"
	bannerTime   = 2.2 // seconds a wave's banner shows
	whackShake   = 0.06
	hurtShake    = 0.18
	titleSway    = 12  // degrees the title's camera sways to either side
	titleNod     = 4.5 // seconds between the title astronaut's nods
	gameOverSpin = 10
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
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// titleScene shows the game's name and controls over the courtyard, where
// the astronaut waits with its stick, while the camera goes slowly round.
type titleScene struct {
	world  *world
	camera *golib.Camera3D
	record record
	nod    golib.Timer // the astronaut nods every so often
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord(), nod: golib.NewRepeatingTimer(titleNod)}
	s.place()
	return s
}

// place puts the camera in front of the astronaut, swaying a little from
// side to side, with the astronaut on the right of the screen and room for
// the text on the left: seen from the front, the screen's left is -X.
func (s *titleScene) place() {
	sway := titleSway * float32(math.Sin(float64(s.world.time)*0.4))
	s.camera.Orbit(playerStart.Add(golib.Vector3{X: -0.75, Y: 0.5}), 180+sway, -8, 2.7)
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
	// Only the time and the animations move on: no wave comes to the title.
	s.world.time += dt
	s.world.animate(dt)
	if s.nod.Tick(dt) {
		s.world.player.animator.PlayWith(animNod, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true})
	}
	s.place()
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	screen.DrawRectangle(golib.Rectangle{Width: 700, Height: screen.Height()}, golib.Color{R: 20, G: 16, B: 40, A: 140})
	const x = 56
	drawText(screen, "RUMBLE", x, 60, 120, accentColor)
	drawText(screen, "Red astronauts are pouring into the courtyard.", x, 200, 20, textColor)
	drawText(screen, "You have a stick. Three whacks knock one out.", x, 228, 20, textColor)
	drawText(screen, "Clear a wave and a bigger one comes in.", x, 256, 20, textColor)
	lines := []string{
		"Move: arrows, WASD, d-pad or left stick",
		"  (tilt the stick gently, or hold Shift, to walk)",
		"Swing: J, X, left click, or X or RB (even running)",
		"Jump: Space or A     Camera: Q and E or right stick",
		"Pause: Esc, P or Start     Fullscreen: F11",
	}
	for i, line := range lines {
		drawText(screen, line, x, 320+float32(i)*30, 20, textColor)
	}
	drawText(screen, "Press Enter, Space or A to fight", x, 500, 30, accentColor)
	if s.record.BestScore > 0 {
		drawText(screen, fmt.Sprintf("Best score %d, wave %d", s.record.BestScore, s.record.BestWave), x, 556, 20, textColor)
	}
	drawText(screen, "Esc to quit", x, screen.Height()-50, 20, golib.WithOpacity(textColor, 0.7))
}

// playScene is the game itself. Update turns the keys and the sticks into
// what the player wants, advances the world, and keeps the camera behind
// the player; Draw draws it all.
type playScene struct {
	world        *world
	camera       *golib.Camera3D
	record       record
	waveBanner   golib.Timer // runs while "Wave n" shows
	clearBanner  golib.Timer // runs while "Wave n cleared" shows
	clearedWave  int
	clearedBonus int
}

func newPlayScene(r record) *playScene {
	s := &playScene{world: newWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight), record: r}
	s.camera.Heading = s.world.player.yaw
	s.camera.Lag = cameraLag
	follow(s.camera, &s.world.player, 1.0/60)
	s.camera.Snap() // ready to draw before the first update
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	s.camera.Heading += cameraTurn(input) * dt
	want := readIntent(input, s.camera)
	if !s.world.player.alive() {
		want = intent{}
	}
	happened := s.world.step(want, dt)
	s.react(happened)
	follow(s.camera, &s.world.player, dt)
	s.camera.Update(dt)
	s.waveBanner.Tick(dt)
	s.clearBanner.Tick(dt)
	if s.world.over {
		s.saveRecord()
		golib.SwitchScene(newGameOverScene(s))
	}
}

// react plays the sounds and starts the effects for what happened in the world.
func (s *playScene) react(happened events) {
	if happened.jumped {
		jumpSound.PlayWith(0.5, golib.RandomFloat(0.95, 1.08))
	}
	if happened.landed > 4 {
		landSound.PlayWith(golib.Clamp(happened.landed/10, 0.3, 1), golib.RandomFloat(0.9, 1.1))
	}
	if happened.swung {
		swingSound.PlayWith(1, golib.RandomFloat(0.9, 1.15))
	}
	if happened.whacks > 0 {
		whackSound.PlayWith(1, golib.RandomFloat(0.9, 1.12))
		s.camera.Shake(whackShake, 0.12)
	}
	if happened.knockouts > 0 {
		knockoutSound.PlayWith(1, golib.RandomFloat(0.95, 1.05))
	}
	if happened.hurt {
		hurtSound.PlayWith(0.8, golib.RandomFloat(0.95, 1.05))
		s.camera.Shake(hurtShake, 0.3)
	}
	if happened.died {
		gameOverSound.Play()
	}
	if happened.waveStarted {
		startSound.Play()
		s.waveBanner.Start(bannerTime)
	}
	if happened.waveCleared {
		waveSound.Play()
		s.clearBanner.Start(bannerTime)
		s.clearedWave, s.clearedBonus = s.world.wave, waveBonus*s.world.wave
	}
}

// saveRecord keeps the best score and the furthest wave.
func (s *playScene) saveRecord() {
	w := s.world
	if w.score <= s.record.BestScore && w.wave <= s.record.BestWave {
		return
	}
	s.record.BestScore = max(s.record.BestScore, w.score)
	s.record.BestWave = max(s.record.BestWave, w.wave)
	golib.SaveData(recordName, s.record) // a failed save only loses the record
}

// bannerOpacity fades a banner in quickly and out slowly over its timer.
func bannerOpacity(t golib.Timer) float32 {
	if !t.Running() {
		return 0
	}
	shown := t.Progress()
	return golib.Clamp(min(shown*8, (1-shown)*3), 0, 1)
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	drawHUD(screen, s.world, max(s.record.BestScore, s.world.score))
	if s.clearBanner.Running() {
		drawBanner(screen, fmt.Sprintf("Wave %d cleared!", s.clearedWave), fmt.Sprintf("+%d", s.clearedBonus), bannerOpacity(s.clearBanner))
	} else if s.waveBanner.Running() {
		drawBanner(screen, fmt.Sprintf("Wave %d", s.world.wave), fmt.Sprintf("%d coming in", waveSize(s.world.wave)), bannerOpacity(s.waveBanner))
	}
	if !s.world.player.alive() {
		drawBanner(screen, "Knocked out!", "", 1)
	}
	drawText(screen, "Esc: pause", screen.Width()/2, screen.Height()-34, 20, golib.WithOpacity(textColor, 0.7), centered)
	if !golib.WindowFocused() {
		drawText(screen, "Paused: click the window to carry on", screen.Width()/2, screen.Height()-70, 20, textColor, centered)
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

// gameOverScene shows the score and the wave reached over the courtyard,
// where the enemies wander about the fallen astronaut.
type gameOverScene struct {
	finished *playScene
	camera   *golib.Camera3D
	yaw      float32
}

func newGameOverScene(finished *playScene) *gameOverScene {
	s := &gameOverScene{finished: finished, camera: golib.NewCamera3D(screenWidth, screenHeight), yaw: finished.camera.Heading}
	s.place()
	return s
}

func (s *gameOverScene) place() {
	s.camera.Orbit(s.finished.world.player.position.Add(golib.Vector3{Y: 0.5}), s.yaw, -30, 9)
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
	s.finished.world.step(intent{}, dt) // the enemies wander on
	s.yaw += gameOverSpin * dt
	s.place()
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.finished.world, s.camera)
	w := s.finished.world
	best := fmt.Sprintf("Best score %d, wave %d", s.finished.record.BestScore, s.finished.record.BestWave)
	if w.score >= s.finished.record.BestScore && w.score > 0 {
		best = "A new best score!"
	}
	drawMessage(screen, "Game over", fmt.Sprintf("Score %d", w.score), fmt.Sprintf("You reached wave %d", w.wave), best, "",
		"Enter, Space or A to fight again", "Esc or B for the title")
}
