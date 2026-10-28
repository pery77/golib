package main

import (
	"fmt"
	"golib"
)

var touchLeft = golib.Rectangle{X: 34, Y: 533, Width: 86, Height: 84}
var touchRight = golib.Rectangle{X: 132, Y: 533, Width: 86, Height: 84}
var touchStabilize = golib.Rectangle{X: 957, Y: 533, Width: 120, Height: 84}
var touchThrust = golib.Rectangle{X: 1091, Y: 513, Width: 154, Height: 104}
var pauseArea = golib.Rectangle{X: 1152, Y: 30, Width: 88, Height: 32}

type playScene struct {
	app       *session
	world     *world
	camera    *golib.Camera
	particles []particle
	autoZoom  bool
	zoom      float32
	finished  golib.Timer
	warning   golib.Timer
}

func newPlayScene(a *session, c contract) *playScene {
	w := newWorld(a.progress, c)
	s := &playScene{app: a, world: w, camera: golib.NewCamera(screenWidth, screenHeight), autoZoom: true, zoom: 0.9, warning: golib.NewRepeatingTimer(2)}
	s.camera.Lag = 0.3
	s.camera.Zoom = 0.9
	s.camera.Target = w.pos.Add(golib.Vector2{Y: 150})
	s.camera.Snap()
	return s
}
func (s *playScene) Update(in *golib.Input, dt float32) {
	s.app.update(in, dt)
	mx, my := in.MousePosition()
	if pausePressed(in) || (in.MousePressed(golib.MouseLeft) && pauseArea.Contains(mx, my)) {
		engineSound.Stop()
		golib.SwitchScene(&pauseScene{play: s})
		return
	}
	w := s.world
	if w.state != flying {
		s.updateParticles(dt)
		s.camera.Update(dt)
		if s.finished.Tick(dt) {
			result := s.app.progress.settle(w)
			s.app.save()
			golib.SwitchScene(&debriefScene{play: s, result: result})
		}
		return
	}
	c := controls{}
	c.turn, _ = in.GamepadLeftStick(0)
	if in.KeyDown(golib.KeyLeft) || in.KeyDown(golib.KeyA) || in.GamepadDown(0, golib.GamepadLeft) || in.TouchDownIn(touchLeft) {
		c.turn = -1
	}
	if in.KeyDown(golib.KeyRight) || in.KeyDown(golib.KeyD) || in.GamepadDown(0, golib.GamepadRight) || in.TouchDownIn(touchRight) {
		c.turn = 1
	}
	if in.KeyDown(golib.KeyW) || in.KeyDown(golib.KeyUp) || in.KeyDown(golib.KeySpace) || in.GamepadDown(0, golib.GamepadA) || in.GamepadDown(0, golib.GamepadRightTrigger) || in.TouchDownIn(touchThrust) {
		c.thrust = 1
	}
	c.stabilize = in.KeyDown(golib.KeyS) || in.KeyDown(golib.KeyDown) || in.KeyDown(golib.KeyLeftShift) || in.GamepadDown(0, golib.GamepadX) || in.TouchDownIn(touchStabilize)
	dz := in.MouseWheel() * 0.12
	if in.KeyDown(golib.KeyQ) || in.GamepadDown(0, golib.GamepadLeftBumper) {
		dz -= dt * 0.7
	}
	if in.KeyDown(golib.KeyE) || in.GamepadDown(0, golib.GamepadRightBumper) {
		dz += dt * 0.7
	}
	if dz != 0 {
		s.autoZoom = false
		s.zoom = golib.Clamp(s.zoom+dz, 0.5, 1.85)
	}
	if in.KeyPressed(golib.KeyC) || in.GamepadPressed(0, golib.GamepadY) {
		s.autoZoom = true
	}
	w.step(c, dt)
	if w.thrust > 0 {
		engineSound.Loop()
	} else {
		engineSound.Stop()
	}
	if w.fuel < w.spec.Fuel*0.2 && s.warning.Tick(dt) {
		alertSound.Play()
	}
	s.updateParticles(dt)
	if w.state != flying {
		engineSound.Stop()
		s.burst(w.state == landed)
		if w.state == landed {
			landedSound.PlayWith(0.6, 0.8)
			s.camera.Shake(3, 0.3)
		} else {
			crashSound.PlayWith(0.7, 0.75)
			s.camera.Shake(14, 0.7)
		}
		s.finished.Start(1.8)
	}
	if s.autoZoom {
		s.zoom = golib.Lerp(1.48, 0.82, golib.Clamp(w.altitude()/110, 0, 1))
	}
	s.camera.Zoom = golib.Lerp(s.camera.Zoom, s.zoom, min(1, dt*2))
	target := w.pos.Add(golib.Vector2{X: w.vel.X * 0.9, Y: golib.Clamp(w.altitude()*1.1, 35, 140)})
	s.camera.Target = target
	s.camera.Update(dt)
}
func (s *playScene) Draw(screen *golib.Screen) {
	w, a := s.world, s.app
	drawSky(screen, a, w.contract.Region, s.camera.Center().X)
	drawPlanet(screen, 965-s.camera.Center().X*0.045, 190-s.camera.Center().Y*0.015, 128, w.contract.Region)
	// Distant ridge: screen-space parallax.
	for i := 0; i < 14; i++ {
		x := float32(i)*130 - 100 - s.camera.Center().X*0.08
		y := 490 + sin(float32(i)*2.1)*55 - s.camera.Center().Y*0.05
		screen.DrawTriangle(x-110, 720, x+70, y, x+220, 720, rgb(18, 34, 47))
	}
	screen.SetCamera(s.camera)
	drawTerrain(screen, w, a.time, a.progress.Bases)
	s.drawParticles(screen)
	if w.state != crashed {
		drawShip(screen, w.pos, w.angle, 1, a.progress.Selected, w.thrust, a.time)
		if w.stabilizing {
			for _, x := range []float32{-17, 17} {
				p := shipPoint(w.pos, w.angle, 1, x, 0)
				screen.DrawCircle(p.X, p.Y, 3+sin(a.time*45), cyan)
			}
		}
	}
	screen.SetCamera(nil)
	s.drawHUD(screen)
	if a.notice != "" {
		box(screen, golib.Rectangle{X: 250, Y: 107, Width: 780, Height: 56})
		for i, line := range wrapText(a.notice, 78) {
			center(screen, line, 640, 117+float32(i)*20, 14, orange)
		}
	}
	if !golib.WindowFocused() {
		rect(screen, 0, 0, 1280, 720, fade(ink, 0.55))
		center(screen, "FLIGHT SUSPENDED / RETURN TO THE WINDOW", 640, 344, 24, white)
	}
}
func (s *playScene) drawHUD(screen *golib.Screen) {
	w := s.world
	rect(screen, 0, 0, 1280, 102, fade(ink, 0.76))
	text(screen, "SELENE", 36, 26, 24, white)
	text(screen, "FLIGHT SYSTEMS / LIVE", 36, 60, 14, cyan)
	screen.DrawLine(208, 25, 208, 77, 1, lineColor)
	text(screen, regions[w.contract.Region].Name, 232, 26, 24, white)
	text(screen, cargoNames[w.contract.Kind]+"  /  "+ships[s.app.progress.Selected].Name, 232, 62, 14, muted)
	right(screen, fmt.Sprintf("CONTRACT   %s CR", money(w.contract.reward(s.app.progress))), 1108, 28, 18, cyan)
	right(screen, fmt.Sprintf("T+ %02d:%02d", int(w.elapsed)/60, int(w.elapsed)%60), 1108, 61, 16, muted)
	drawButton(screen, button{r: pauseArea, title: "II"}, false)

	pad := w.pad()
	target := s.camera.ToScreen(golib.Vector2{X: pad.Center().X, Y: pad.Y - 55})
	tx, ty := golib.Clamp(target.X, 310, 1020), golib.Clamp(target.Y, 132, 525)
	if target.X != tx || target.Y != ty {
		screen.DrawCircleOutline(tx, ty, 14, 1, cyan)
		screen.DrawLine(tx-6, ty-3, tx, ty+4, 2, cyan)
		screen.DrawLine(tx, ty+4, tx+6, ty-3, 2, cyan)
	}
	if w.pos.Distance(pad.Center()) > 120 {
		center(screen, fmt.Sprintf("PAD %02d  /  %.0f M", w.contract.Kind+1, w.pos.Distance(pad.Center())/4), tx, ty-36, 14, cyan)
	}

	// Left telemetry remains readable while the world moves behind it.
	box(screen, golib.Rectangle{X: 30, Y: 126, Width: 190, Height: 243})
	text(screen, "RADAR ALTITUDE", 46, 144, 14, muted)
	text(screen, fmt.Sprintf("%03.0f", w.altitude()), 46, 171, 42, white)
	text(screen, "METERS", 155, 192, 12, muted)
	screen.DrawLine(46, 226, 204, 226, 1, lineColor)
	text(screen, "DESCENT", 46, 242, 14, muted)
	vc := cyan
	if w.vel.Y > w.descentLimit() {
		vc = orange
	}
	right(screen, fmt.Sprintf("%+.1f M/S", w.vel.Y/4), 204, 265, 22, vc)
	text(screen, "LATERAL", 46, 305, 14, muted)
	vc = cyan
	if abs(w.vel.X) > w.driftLimit() {
		vc = orange
	}
	right(screen, fmt.Sprintf("%+.1f M/S", w.vel.X/4), 204, 330, 22, vc)

	box(screen, golib.Rectangle{X: 1050, Y: 126, Width: 200, Height: 171})
	text(screen, "LANDING ENVELOPE", 1066, 144, 14, muted)
	checks := []struct {
		label string
		ok    bool
	}{
		{fmt.Sprintf("V  < %.1f M/S", w.descentLimit()/4), w.vel.Y <= w.descentLimit()},
		{fmt.Sprintf("H  < %.1f M/S", w.driftLimit()/4), abs(w.vel.X) <= w.driftLimit()},
		{"TILT < 12 DEG", abs(w.angle) <= 12},
	}
	for i, c := range checks {
		col := orange
		mark := "!"
		if c.ok {
			col = cyan
			mark = "+"
		}
		text(screen, mark, 1066, 176+float32(i)*33, 20, col)
		text(screen, c.label, 1090, 179+float32(i)*33, 14, col)
	}
	if w.contract.Kind == 2 {
		timeLeft := w.contract.deadline() - w.elapsed
		col := cyan
		if timeLeft < 10 {
			col = orange
		}
		text(screen, fmt.Sprintf("TIME BONUS  %.0f S", max(0, timeLeft)), 1055, 313, 16, col)
	}
	// Instrument rail.
	rect(screen, 0, 632, 1280, 88, fade(ink, 0.96))
	screen.DrawLine(30, 632, 1250, 632, 1, lineColor)
	text(screen, "PROPELLANT", 36, 649, 14, muted)
	fc := cyan
	if w.fuel/w.spec.Fuel < 0.22 {
		fc = orange
	}
	text(screen, fmt.Sprintf("%03.0f %%", 100*w.fuel/w.spec.Fuel), 36, 674, 26, fc)
	meter(screen, 155, 686, 160, w.fuel/w.spec.Fuel, fc)
	text(screen, "ATTITUDE", 353, 649, 14, muted)
	text(screen, fmt.Sprintf("%+.0f DEG", w.angle), 353, 679, 20, white)
	text(screen, "THRUST", 535, 649, 14, muted)
	meter(screen, 535, 685, 110, w.thrust, orange)
	zoomMode := "AUTO"
	if !s.autoZoom {
		zoomMode = "MANUAL"
	}
	text(screen, "OPTICS / "+zoomMode, 687, 649, 14, muted)
	text(screen, fmt.Sprintf("%.2f X", s.camera.Zoom), 687, 679, 20, cyan)
	// Whole-region navigation map.
	x, y, width := float32(934), float32(655), float32(290)
	for i := 1; i < len(w.land.terrain); i++ {
		a, b := w.land.terrain[i-1], w.land.terrain[i]
		screen.DrawLine(x+a.X/w.land.width*width, y+(a.Y-850)*0.09, x+b.X/w.land.width*width, y+(b.Y-850)*0.09, 1, muted)
	}
	px := x + pad.Center().X/w.land.width*width
	screen.DrawCircle(px, y+(pad.Y-850)*0.09, 3, cyan)
	shipX := x + w.pos.X/w.land.width*width
	screen.DrawTriangle(shipX-4, y-3, shipX+4, y-3, shipX, y-10, orange)
	text(screen, "SECTOR RADAR", 934, 695, 10, muted)

	if golib.PlayingWithTouch() {
		drawButton(screen, button{r: touchLeft, title: "<"}, false)
		drawButton(screen, button{r: touchRight, title: ">"}, false)
		drawButton(screen, button{r: touchStabilize, title: "STABILIZE"}, w.stabilizing)
		drawButton(screen, button{r: touchThrust, title: "THRUST"}, w.thrust > 0)
	} else {
		center(screen, "A / D  ROTATE     W / SPACE  THRUST     S  STABILIZE     Q / E  ZOOM     C  AUTO", 640, 603, 14, muted)
	}
	if w.elapsed < 8 && s.app.progress.Deliveries == 0 {
		box(screen, golib.Rectangle{X: 326, Y: 109, Width: 628, Height: 63})
		center(screen, "POINT TOWARD THE BEACON, THEN BURN", 640, 123, 20, white)
		center(screen, "Hold S to level out and brake drift. W controls your descent.", 640, 151, 14, muted)
	}
	if w.fuel <= 0 {
		center(screen, "FUEL DEPLETED / BALLISTIC DESCENT", 640, 213, 20, orange)
	}
}
