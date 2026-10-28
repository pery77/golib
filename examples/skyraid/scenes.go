package main

import (
	"fmt"
	"math"

	"golib"
)

// The game's scenes. Each one is a golib.Game: golib.Run starts with the title,
// and golib.SwitchScene moves between them.
//
//	title --Enter/A/click--> play --Esc/P/Start--> pause --Esc/P/Start--> back to the same play
//	  |                        |                     '------Q/Back------> title
//	  |                        '--ship destroyed--> game over --Enter/A/click--> a new play
//	  |                                                '-----Esc/B-----> title
//	  '--Esc--> quit
//
// In every scene, F11 or Alt+Enter switches fullscreen, F2 or the Y button
// turns the screen effects (effects.go) on and off, F3 or the X button turns
// the music (music.go) on and off, and F5 reads the shaders from the assets
// folder again, to tune them without leaving the game.

// Scene tuning.
const (
	titlePanSpeed = 3.8 // degrees per second the title's view circles the arena
	titlePanWidth = 600 // pixels the title's view drifts to each side of the arena's middle
	titlePanTall  = 400 // pixels it drifts up and down
	gameOverWait  = 0.6 // seconds before the game over screen takes a key, so firing doesn't skip it
	panelHeight   = 200 // pixels, for a panel with two lines; each more line adds 30
)

// session is what every scene shares while the game runs: the best result so
// far, the space behind the arena, the screen effects and the music. GoLib
// can't save files yet, so the best score is gone when the game closes.
type session struct {
	bestScore int
	bestWave  int
	newBest   bool // the last game beat the best score
	backdrop  *backdrop
	effects   *effects
	music     *golib.Music
	musicOn   bool
	musicNote string // why there is no music, for the screen; empty when there is
}

// newSession returns a session with a new backdrop, the screen effects on and
// the music playing, if the assets folder has a tune GoLib can play.
func newSession() *session {
	s := &session{backdrop: newBackdrop(), effects: newEffects(), musicOn: true}
	s.music, s.musicNote = findMusic()
	return s
}

// notes are the lines the screen shows about what the game couldn't load.
func (s *session) notes() []string {
	return []string{s.effects.err, s.musicNote}
}

// record keeps a finished game's result if it is the best so far.
func (s *session) record(w *world) {
	s.newBest = w.score > s.bestScore
	if s.newBest {
		s.bestScore = w.score
	}
	s.bestWave = max(s.bestWave, w.wave)
}

// handleKeys reads the keys every scene shares: F11 or Alt+Enter switch
// fullscreen, F2 or the Y button turn the screen effects on and off, F3 or the
// X button turn the music on and off, and F5 reads the shaders again.
func (s *session) handleKeys(input *golib.Input) {
	if input.KeyPressed(golib.KeyF11) || (altDown(input) && input.KeyPressed(golib.KeyEnter)) {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	if input.KeyPressed(golib.KeyF2) || input.GamepadPressed(0, golib.GamepadY) {
		s.effects.setOn(!s.effects.on)
	}
	if input.KeyPressed(golib.KeyF3) || input.GamepadPressed(0, golib.GamepadX) {
		s.musicOn = !s.musicOn
	}
	if input.KeyPressed(golib.KeyF5) {
		s.effects.reload()
	}
	s.playMusic()
}

// playMusic keeps the music going, or holds it while it is off. Both calls are
// safe in every update: the first Play starts the music, once golib.Run has
// opened the sound device.
func (s *session) playMusic() {
	if s.music == nil {
		return
	}
	if s.musicOn {
		s.music.Play()
		return
	}
	s.music.Pause()
}

func altDown(input *golib.Input) bool {
	return input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
}

// confirmed reports whether the player pressed Enter, A or Start, or clicked.
// Alt+Enter doesn't count: it switches fullscreen.
func confirmed(input *golib.Input) bool {
	return (input.KeyPressed(golib.KeyEnter) && !altDown(input)) ||
		input.MousePressed(golib.MouseLeft) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// titleScene shows the game's name and controls over the arena, drifting by.
type titleScene struct {
	session *session
	camera  *golib.Camera
	time    float32 // seconds on the title, for the drift and the pulse
}

func newTitleScene(s *session) *titleScene {
	t := &titleScene{session: s, camera: golib.NewCamera(screenWidth, screenHeight)}
	t.camera.Bounds = arena
	t.camera.Target = titleTarget(0)
	t.camera.Snap() // no lag: the view is exactly where the drift puts it
	return t
}

// titleTarget returns the middle of the title's view after time seconds: a
// slow circle around the middle of the arena.
func titleTarget(time float32) golib.Vector2 {
	turn := golib.Vector2FromAngle(time * titlePanSpeed)
	return arena.Center().Add(golib.Vector2{X: titlePanWidth * turn.X, Y: titlePanTall * turn.Y})
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	s.session.handleKeys(input)
	golib.SetMouseVisible(true)
	s.session.effects.set(0, 0) // no ship on the title: nothing glitches
	s.time += dt
	s.camera.Target = titleTarget(s.time)
	s.camera.Update(dt)
	if confirmed(input) {
		golib.SwitchScene(newPlayScene(s.session))
		return
	}
	// No key quits by itself, not even Esc: the game calls golib.Quit when it
	// wants to end.
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadBack) {
		golib.Quit()
	}
}

func (s *titleScene) Draw(screen *golib.Screen) {
	s.session.backdrop.draw(screen, s.camera)
	screen.SetCamera(s.camera)
	drawGrid(screen, s.camera.View())
	screen.SetCamera(nil) // the title's text and ships are in screen pixels
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, withAlpha(overlayColor, 0.5))

	// A ship on the title, turning slowly.
	rock := 14 * float32(math.Sin(float64(s.time))) // degrees it sways
	drawShape(screen, shipShape, golib.Vector2{X: screenWidth / 2, Y: 250}, -90+rock, shipDarkColor, shipColor)
	drawShape(screen, scoutShape, golib.Vector2{X: screenWidth/2 - 220, Y: 210}, 17, darker(enemyColors[scout], 0.35), enemyColors[scout])
	drawShape(screen, gunshipShape, golib.Vector2{X: screenWidth/2 + 230, Y: 200}, 163, darker(enemyColors[gunship], 0.35), enemyColors[gunship])

	drawCentered(screen, "SKY RAID", 70, 100, titleColor)
	drawCentered(screen, "Waves of enemy ships are hunting you. Survive, and shoot them all.", 300, 20, textColor)

	lines := []string{
		"Fly: WASD, d-pad or left stick",
		"Aim: mouse, arrow keys or right stick",
		"Fire: left click, Space, arrow keys, A or right trigger",
		"Dash through bullets: Shift, right click, B or left bumper",
		"Pause: Esc, P or Start     Fullscreen: F11 or Alt+Enter",
		"Screen effects: F2     Music: F3",
	}
	for i, line := range lines {
		drawCentered(screen, line, 350+float32(i)*30, 20, dimTextColor)
	}

	pulse := 0.6 + 0.4*float32(math.Sin(float64(s.time)*4))
	drawCentered(screen, "Press Enter, A or click to start", 540, 30, withAlpha(titleColor, pulse))
	if s.session.bestScore > 0 {
		drawCentered(screen, fmt.Sprintf("Best score %d, wave %d", s.session.bestScore, s.session.bestWave), 600, 20, textColor)
	}
	drawCentered(screen, "Esc: quit", 670, 20, dimTextColor)
	drawNotes(screen, s.session.notes()...)
}

// pauseScene freezes a play scene and shows a message over it. Resuming
// switches back to that same play scene, so the game carries on where it
// stopped.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	s.paused.session.handleKeys(input)
	golib.SetMouseVisible(true)
	s.paused.applyEffects()
	if pausePressed(input) {
		golib.SwitchScene(s.paused)
		return
	}
	if input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadBack) {
		golib.SwitchScene(newTitleScene(s.paused.session))
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawPanel(screen, "PAUSED",
		"Esc, P or Start to resume",
		"Q or Back to quit to the title",
	)
}

// gameOverScene shows the final score while the arena keeps moving.
type gameOverScene struct {
	finished *playScene
	time     float32 // seconds on this screen
}

func (s *gameOverScene) Update(input *golib.Input, dt float32) {
	p := s.finished
	p.session.handleKeys(input)
	golib.SetMouseVisible(true)
	s.time += dt
	p.world.step(controls{}, dt)
	p.followCamera(dt)
	p.applyEffects()
	if s.time < gameOverWait {
		return
	}
	if confirmed(input) {
		golib.SwitchScene(newPlayScene(p.session))
		return
	}
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(p.session))
	}
}

func (s *gameOverScene) Draw(screen *golib.Screen) {
	p := s.finished
	drawWorld(screen, &p.world, p.session.backdrop, p.camera)
	best := fmt.Sprintf("Best %d", p.session.bestScore)
	if p.session.newBest {
		best = "New best score!"
	}
	drawPanel(screen, "GAME OVER",
		fmt.Sprintf("Score %d     Wave %d     Enemies destroyed %d", p.world.score, p.world.wave, p.world.kills),
		best,
		"",
		"Enter, A or click to play again",
		"Esc or B for the title",
	)
}

// drawPanel darkens the screen and shows a heading and a few lines in the
// middle.
func drawPanel(screen *golib.Screen, heading string, lines ...string) {
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, overlayColor)
	height := float32(panelHeight + 30*max(0, len(lines)-2))
	top := (screenHeight - height) / 2
	panel := golib.Rectangle{X: screenWidth/2 - 360, Y: top, Width: 720, Height: height}
	screen.DrawRectangle(panel, panelColor)
	screen.DrawRectangleOutline(panel, 2, withAlpha(titleColor, 0.6))
	drawCentered(screen, heading, top+30, 60, titleColor)
	for i, line := range lines {
		drawCentered(screen, line, top+110+float32(i)*30, 20, textColor)
	}
}
