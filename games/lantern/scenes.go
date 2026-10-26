package main

import (
	"fmt"

	"golib"
)

// toggleFullscreen switches fullscreen on F11 or Alt+Enter, in every scene.
func toggleFullscreen(input *golib.Input) {
	altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) && input.KeyPressed(golib.KeyEnter)
	if input.KeyPressed(golib.KeyF11) || altEnter {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
}

// startPressed is Enter, Space, or the gamepad's A or Start.
func startPressed(input *golib.Input) bool {
	altDown := input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)
	return (input.KeyPressed(golib.KeyEnter) && !altDown) || input.KeyPressed(golib.KeySpace) ||
		input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// titleScene shows the flooded hall, turning slowly, under the game's name
// and its controls.
type titleScene struct {
	world  *world
	camera *golib.Camera3D
	angle  float32
}

func newTitleScene() *titleScene {
	s := &titleScene{world: newWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight)}
	s.camera.FieldOfView = cameraView
	s.aim()
	return s
}

// aim puts the camera in the flooded hall, looking across the pool.
func (s *titleScene) aim() {
	s.camera.Orbit(golib.Vector3{Y: 1.2, Z: 20}, 180+s.angle, -12, 6.5)
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadBack) {
		golib.Quit()
		return
	}
	if startPressed(input) {
		golib.SwitchScene(newPlayScene())
		return
	}
	s.world.time += dt
	s.angle += 6 * dt
	s.aim()
	burnTorches(s.world, dt)
}

func (s *titleScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.Black)
	drawWorld(screen, s.world, s.camera, false)
	screen.DrawRectangle(golib.Rectangle{Y: 24, Width: screenWidth, Height: 150}, golib.Color{A: 140})
	drawCentered(screen, "LANTERN", 40, 80, golib.Color{R: 255, G: 210, B: 140, A: 255})
	drawCentered(screen, "A night in a flooded temple. Take the six gems to open the gate.", 132, 20, golib.RayWhite)
	screen.DrawRectangle(golib.Rectangle{Y: 600, Width: screenWidth, Height: 100}, golib.Color{A: 140})
	drawCentered(screen, "Up/Down or W/S: walk    Left/Right or A/D: turn    Q/E: step aside    Esc: pause    F11: fullscreen", 614, 20, golib.LightGray)
	drawCentered(screen, "Press Enter, Space or A to begin", 652, 30, golib.Gold)
}

// playScene is the game: the explorer in the temple.
type playScene struct {
	world  *world
	camera *golib.Camera3D
	note   string // what just happened, shown for a moment
	noteAt golib.Timer
}

func newPlayScene() *playScene {
	w := newWorld()
	s := &playScene{world: w, camera: newCamera(w)}
	s.say("Six gems lie in the temple. Take them all to open the gate.")
	return s
}

func (s *playScene) say(note string) {
	s.note = note
	s.noteAt = golib.NewTimer(3.5)
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	w := s.world
	want := steer(readControls(input), s.camera, dt)
	ev := w.step(want, dt)
	follow(s.camera, w, dt)
	s.noteAt.Tick(dt)

	if ev.took >= 0 {
		g := w.gems[ev.took]
		glimmers[g.color].Emit(g.position, golib.Vector3{Y: 1}, 50)
		pickupSound.PlayWith(1, 0.9+0.08*float32(w.gemsTaken()))
		left := len(w.gems) - w.gemsTaken()
		switch left {
		case 0:
		case 1:
			s.say("One gem left.")
		default:
			s.say(fmt.Sprintf("%d gems left.", left))
		}
	}
	if ev.gateOpens {
		gateSound.Play()
		s.camera.Shake(0.06, gateOpenTime)
		s.say("The gate sinks into the floor. Find the way out.")
	}
	if w.player.wading && w.player.body.Moved.Length() > 0.01 && golib.RandomFloat(0, 1) < 0.25 {
		at := w.player.body.Position
		at.Y = waterLevel + 0.02
		ripples.Emit(at, golib.Vector3{X: golib.RandomFloat(-1, 1), Z: golib.RandomFloat(-1, 1)}, 1)
	}
	burnTorches(w, dt)
	if ev.escaped {
		escapeSound.Play()
		golib.SwitchScene(&wonScene{played: s})
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.Black)
	drawWorld(screen, s.world, s.camera, true)
	s.drawHUD(screen)
}

func (s *playScene) drawHUD(screen *golib.Screen) {
	w := s.world
	screen.DrawRectangle(golib.Rectangle{X: 16, Y: 16, Width: 40 + 34*float32(len(w.gems)), Height: 48}, golib.Color{A: 140})
	for i, g := range w.gems {
		x, y := float32(44+34*i), float32(40)
		if g.taken {
			screen.DrawCircle(x, y, 12, gemColors[g.color])
		}
		screen.DrawCircleOutline(x, y, 12, 2, golib.Color{R: 200, G: 200, B: 200, A: 200})
	}
	if s.noteAt.Running() {
		drawCentered(screen, s.note, screenHeight-70, 26, golib.Color{R: 255, G: 230, B: 190, A: 255})
	}
}

// pauseScene freezes the game under a message.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || startPressed(input) {
		golib.SwitchScene(s.paused)
		return
	}
	if input.KeyPressed(golib.KeyT) || input.GamepadPressed(0, golib.GamepadBack) {
		golib.SwitchScene(newTitleScene())
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, golib.Color{A: 150})
	drawCentered(screen, "Paused", 280, 64, golib.RayWhite)
	drawCentered(screen, "Esc or Enter to go on, T for the title", 370, 24, golib.LightGray)
}

// wonScene shows the temple behind the explorer as they leave.
type wonScene struct {
	played *playScene
}

func (s *wonScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	s.played.world.time += dt
	burnTorches(s.played.world, dt)
	if startPressed(input) || input.KeyPressed(golib.KeyEscape) {
		golib.SwitchScene(newTitleScene())
	}
}

func (s *wonScene) Draw(screen *golib.Screen) {
	s.played.Draw(screen)
	screen.DrawRectangle(golib.Rectangle{Y: 230, Width: screenWidth, Height: 220}, golib.Color{A: 160})
	drawCentered(screen, "You escaped the flooded temple", 260, 52, golib.Color{R: 255, G: 210, B: 140, A: 255})
	minutes, seconds := int(s.played.world.escapedAt)/60, int(s.played.world.escapedAt)%60
	drawCentered(screen, fmt.Sprintf("in %d:%02d", minutes, seconds), 340, 28, golib.RayWhite)
	drawCentered(screen, "Press Enter to return to the title", 390, 24, golib.Gold)
}

// drawCentered writes text centered across the screen.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	screen.DrawText(text, screenWidth/2, y, size, color, golib.TextOptions{Align: golib.AlignCenter})
}
