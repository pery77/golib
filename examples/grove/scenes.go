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
//	  |                       '--flag or time up--> end --Enter/A--> a new play
//	  '--Esc--> quit                                 (end: Esc or B for the title)

// record is what the game remembers between runs.
type record struct {
	BestScore int
	BestCoins int
}

const (
	recordName  = "record"
	bannerTime  = 2.6 // seconds the "Reach the flag" banner shows
	titleSpin   = 5   // degrees per second the title's camera goes round the island
	endSwing    = 1.6 // seconds the end's camera takes to swing round to the astronaut's front
	endSway     = 14  // degrees it then sways to either side
	splashShake = 0.12
	punchShake  = 0.05
	breakShake  = 0.14
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

// titleScene shows the game's name and controls over the island, where the
// astronaut waits at the start, while the camera goes slowly round.
type titleScene struct {
	island *island
	world  *world
	camera *golib.Camera3D
	record record
	yaw    float32
}

func newTitleScene(l *island) *titleScene {
	s := &titleScene{island: l, world: newWorld(l), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord(), yaw: 205}
	s.world.clockRun = false
	s.place()
	return s
}

// place puts the camera high over the island, looking down at its middle.
func (s *titleScene) place() {
	s.camera.Orbit(golib.Vector3{Y: 1, Z: 2}, s.yaw, -32, 26)
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) {
		golib.Quit()
		return
	}
	if confirmed(input) {
		startSound.Play()
		golib.SwitchScene(newPlayScene(s.island, s.record))
		return
	}
	s.world.step(intent{}, dt) // the coins spin and the astronaut idles
	s.yaw += titleSpin * dt
	s.place()
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	screen.DrawRectangle(golib.Rectangle{Width: 640, Height: screen.Height()}, golib.WithOpacity(overlayColor, 0.75))
	const x = 56
	drawText(screen, "GROVE", x, 56, 120, accentColor)
	drawText(screen, "Cross the island to the flag before the clock runs out.", x, 196, 20, textColor)
	drawText(screen, "Coins add time and points; punch rocks open for more.", x, 224, 20, textColor)
	drawText(screen, "Fall in the sea and you lose three seconds.", x, 252, 20, textColor)
	lines := []string{
		"Walk: arrows, WASD, d-pad or left stick",
		"Sprint: hold Shift, or B or RT",
		"Jump: Space or A",
		"Punch: J, K, left click or X",
		"Camera: Q and E or right stick",
		"Pause: Esc, P or Start     Fullscreen: F11",
	}
	for i, line := range lines {
		drawText(screen, line, x, 306+float32(i)*30, 20, textColor)
	}
	drawText(screen, "Press Enter, Space or A to start", x, 516, 30, accentColor)
	if s.record.BestScore > 0 {
		drawText(screen, fmt.Sprintf("Best score %d (%d coins)", s.record.BestScore, s.record.BestCoins), x, 566, 20, textColor)
	}
	if version := golib.GameVersion(); version != "" {
		drawText(screen, "v"+version, x, screen.Height()-50, 20, golib.WithOpacity(textColor, 0.6))
	}
	drawText(screen, "Esc to quit", 600, screen.Height()-50, 20, golib.WithOpacity(textColor, 0.7), rightJusted)
}

// playScene is the game itself. Update turns the keys and the sticks into
// what the player wants, advances the world, and keeps the camera behind
// the astronaut; Draw draws it all.
type playScene struct {
	island *island
	world  *world
	camera *golib.Camera3D
	record record
	hud    hud
	banner golib.Timer // runs while "Reach the flag!" shows
}

func newPlayScene(l *island, r record) *playScene {
	s := &playScene{island: l, world: newWorld(l), camera: golib.NewCamera3D(screenWidth, screenHeight), record: r, banner: golib.NewTimer(bannerTime)}
	s.camera.Heading = s.world.player.yaw
	s.camera.Lag = cameraLag
	follow(s.camera, &s.world.player, l, 1.0/60)
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
	clock := s.world.clock
	happened := s.world.step(readIntent(input, s.camera), dt)
	s.react(happened, s.world.clock-clock+dt)
	follow(s.camera, &s.world.player, s.island, dt)
	s.camera.Update(dt)
	s.banner.Tick(dt)
	s.hud.coinPop.Tick(dt)
	s.hud.timePop.Tick(dt)
	if s.world.outcome != running {
		golib.SwitchScene(newEndScene(s))
	}
}

// react plays the sounds and starts the effects for what happened in the
// world; changed is how far the clock moved besides counting down.
func (s *playScene) react(happened events, changed float32) {
	if happened.jumped {
		jumpSound.PlayWith(0.5, golib.RandomFloat(0.95, 1.08))
	}
	if happened.landed > 5 {
		landSound.PlayWith(golib.Clamp(happened.landed/12, 0.3, 1), golib.RandomFloat(0.9, 1.1))
	}
	if happened.coins > 0 {
		coinSound.PlayWith(0.8, golib.RandomFloat(0.95, 1.12))
		s.hud.coinPop.Start(0.3)
	}
	if happened.punched {
		punchSound.PlayWith(1, golib.RandomFloat(0.9, 1.15))
	}
	if happened.rockHit {
		knockSound.PlayWith(1, golib.RandomFloat(0.9, 1.12))
		s.camera.Shake(punchShake, 0.12)
	}
	if happened.rockBroke {
		breakSound.PlayWith(0.7, golib.RandomFloat(0.95, 1.1))
		s.camera.Shake(breakShake, 0.25)
	}
	if happened.splashed {
		splashSound.Play()
		s.camera.Shake(splashShake, 0.3)
	}
	if happened.tick {
		tickSound.Play()
	}
	if happened.timeUp {
		timeUpSound.Play()
	}
	if happened.won {
		winSound.Play()
	}
	if changed > 0.01 || changed < -0.01 {
		s.hud.timePop.Start(0.9)
		s.hud.timeBy = changed
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, s.camera)
	drawHUD(screen, s.world, s.camera, &s.hud)
	if s.banner.Running() {
		fade := golib.Clamp(min(s.banner.Progress()*8, (1-s.banner.Progress())*3), 0, 1)
		drawBanner(screen, "Reach the flag!", "Coins add time. Punch rocks open for more.", fade)
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
		golib.SwitchScene(newTitleScene(s.paused.island))
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	drawMessage(screen, "Paused", accentColor, "Esc, P or Start to carry on", "Q or B to go back to the title")
}

// endScene shows how the run ended, over the island, with the camera swung
// round to the island's side of the astronaut: cheering at the flag, turned
// to it, or fallen where the clock ran out.
type endScene struct {
	finished *playScene
	camera   *golib.Camera3D
	swing    golib.Timer // runs while the camera swings round to the front
	fromYaw  float32     // where the camera was behind the astronaut
	toYaw    float32     // in front of it
	newBest  bool
}

func newEndScene(finished *playScene) *endScene {
	w := finished.world
	s := &endScene{
		finished: finished, camera: golib.NewCamera3D(screenWidth, screenHeight),
		swing: golib.NewTimer(endSwing), fromYaw: finished.camera.Heading, toYaw: w.inwardYaw() + 180,
	}
	if w.outcome == won && w.score() > finished.record.BestScore {
		s.newBest = true
		finished.record.BestScore, finished.record.BestCoins = w.score(), w.taken
		golib.SaveData(recordName, finished.record) // a failed save only loses the record
	}
	s.place()
	return s
}

// place swings the camera from behind the astronaut round to its front, and
// down closer to it, then sways a little from side to side.
func (s *endScene) place() {
	w := s.finished.world
	t := golib.EaseInOut(s.swing.Progress())
	yaw := s.fromYaw + angleBetween(s.fromYaw, s.toYaw)*t + endSway*float32(math.Sin(float64(w.time*0.5)))*t
	endPitch, endDistance := float32(-10), float32(3.4) // level with the astronaut cheering
	if w.outcome == timeUp {
		endPitch, endDistance = -40, 3.0 // looking down at it lying in the grass
	}
	pitch := golib.Lerp(cameraPitch, endPitch, t)
	distance := golib.Lerp(cameraDistance, endDistance, t)
	s.camera.Orbit(w.player.position.Add(golib.Vector3{Y: golib.Lerp(cameraLookUp, 0.2, t)}), yaw, pitch, distance)
}

func (s *endScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadB) {
		golib.SwitchScene(newTitleScene(s.finished.island))
		return
	}
	if confirmed(input) && !s.swing.Running() {
		startSound.Play()
		golib.SwitchScene(newPlayScene(s.finished.island, s.finished.record))
		return
	}
	s.finished.world.step(intent{}, dt) // the coins spin on, the astronaut cheers
	s.swing.Tick(dt)
	s.place()
}

func (s *endScene) Draw(screen *golib.Screen) {
	w := s.finished.world
	drawWorld(screen, w, s.camera)
	if w.outcome == won {
		best := fmt.Sprintf("Best score %d", s.finished.record.BestScore)
		if s.newBest {
			best = "A new best score!"
		}
		drawMessage(screen, "You made it!", accentColor,
			fmt.Sprintf("%d coins x %d  +  %s left x %d a second", w.taken, coinScore, clockText(w.clock), secondScore),
			fmt.Sprintf("Score %d     %s", w.score(), best),
			"Enter, Space or A to run again     Esc or B for the title")
		return
	}
	drawMessage(screen, "Time's up!", warningColor,
		fmt.Sprintf("You picked up %d coins, %d from the flag.", w.taken, int(w.player.position.Distance(w.island.flag)+0.5)),
		"Coins add time: pick them up on the way.",
		"Enter, Space or A to try again     Esc or B for the title")
}
