package main

import (
	"golib"
)

type titleScene struct {
	app      *session
	selected int
	manual   bool
}

func newTitleScene(a *session) *titleScene { return &titleScene{app: a} }
func (s *titleScene) buttons() []button {
	title := "BEGIN OPERATIONS"
	if s.app.progress.Deliveries > 0 {
		title = "CONTINUE OPERATIONS"
	}
	return []button{
		buttonAt(58, 427, 354, 63, title, ""),
		buttonAt(58, 505, 210, 48, "FLIGHT MANUAL", ""),
		buttonAt(282, 505, 160, 48, "SETTINGS", ""),
		buttonAt(1120, 30, 120, 42, "QUIT", ""),
	}
}
func (s *titleScene) Update(in *golib.Input, dt float32) {
	s.app.update(in, dt)
	if s.manual {
		b := []button{buttonAt(464, 563, 352, 52, "READY TO FLY", "")}
		if menuInput(in, b, &s.selected) >= 0 || back(in) {
			s.manual = false
			s.selected = 0
		}
		return
	}
	switch menuInput(in, s.buttons(), &s.selected) {
	case 0:
		golib.SwitchScene(newControlScene(s.app))
	case 1:
		s.manual = true
		s.selected = 0
	case 2:
		golib.SwitchScene(&settingsScene{title: s})
	case 3:
		golib.Quit()
	}
}
func (s *titleScene) Draw(screen *golib.Screen) {
	a := s.app
	drawSky(screen, a, 0, 0)
	drawPlanet(screen, 932, 296, 214, 0)
	// Orbit arcs and the floating ship form the title's hero illustration.
	screen.DrawCircleOutline(932, 296, 259, 0.7, fade(cyan, 0.14))
	screen.DrawCircleOutline(932, 296, 282, 0.6, fade(cyan, 0.08))
	for i := 0; i < 4; i++ {
		yy := 540 + float32(i)*19
		screen.DrawLine(727, yy, 1167, yy+16, 1, fade(cyan, 0.05))
	}
	drawShip(screen, golib.Vector2{X: 846, Y: 441 + sin(a.time*0.8)*8}, -16, 3.8, a.progress.Selected, 0.35, a.time)
	badge(screen, "INDEPENDENT LUNAR OPERATIONS", 58, 101, cyan)
	wordmark(screen, 60, 168, 83, white)
	text(screen, "F  R  O  N  T  I  E  R", 60, 279, 25, cyan)
	text(screen, "A small ship. An entire moon of possibility.", 60, 344, 20, white)
	text(screen, "Master the descent. Build your future.", 60, 377, 18, muted)
	for i, b := range s.buttons() {
		drawButton(screen, b, s.selected == i)
	}
	text(screen, "FIVE REGIONS  /  THREE SHIPS  /  YOUR FRONTIER", 60, 589, 14, muted)
	badge(screen, ships[a.progress.Selected].Name+" / ORBITAL INSERTION", 852, 605, cyan)
	footer(screen, a)
	if s.manual {
		drawManual(screen)
	}
}
func drawManual(screen *golib.Screen) {
	rect(screen, 0, 0, 1280, 720, fade(ink, 0.88))
	box(screen, golib.Rectangle{X: 237, Y: 100, Width: 806, Height: 538})
	badge(screen, "PILOT ORIENTATION / 01", 273, 129, cyan)
	text(screen, "THE ART OF ARRIVING", 273, 176, 32, white)
	rows := [][2]string{
		{"01 / STEER", "A / D or arrows rotate. W / Space fires the engine."},
		{"02 / CONTROL DRIFT", "Hold S / Down / Shift to level out and brake sideways."},
		{"03 / BURN EARLY", "Gravity never stops. Brake before the landing zone."},
		{"04 / TOUCH DOWN", "Both feet on the lit pad. Green V, H and tilt indicators."},
		{"05 / BUILD A BUSINESS", "Buy upgrades, ships and colonies. Every delivery pays."},
	}
	for i, row := range rows {
		y := float32(235 + i*57)
		text(screen, row[0], 273, y, 18, cyan)
		text(screen, row[1], 273, y+25, 16, muted)
	}
	text(screen, "GAMEPAD: stick = turn, A = engine, X = stabilize, Start = pause.", 273, 530, 14, muted)
	drawButton(screen, buttonAt(464, 563, 352, 52, "READY TO FLY", ""), true)
}

type pauseScene struct {
	play     *playScene
	selected int
}

func (s *pauseScene) buttons() []button {
	return []button{buttonAt(442, 334, 396, 58, "RESUME FLIGHT", ""), buttonAt(442, 407, 396, 58, "RETRY CONTRACT", ""), buttonAt(442, 480, 396, 58, "RETURN TO OPERATIONS", "")}
}
func (s *pauseScene) Update(in *golib.Input, dt float32) {
	s.play.app.update(in, dt)
	if pausePressed(in) {
		golib.SwitchScene(s.play)
		return
	}
	switch menuInput(in, s.buttons(), &s.selected) {
	case 0:
		golib.SwitchScene(s.play)
	case 1:
		golib.SwitchScene(newPlayScene(s.play.app, s.play.world.contract))
	case 2:
		golib.SwitchScene(newControlScene(s.play.app))
	}
}
func (s *pauseScene) Draw(screen *golib.Screen) {
	s.play.Draw(screen)
	rect(screen, 0, 0, 1280, 720, fade(ink, 0.86))
	box(screen, golib.Rectangle{X: 399, Y: 190, Width: 482, Height: 380})
	center(screen, "FLIGHT SUSPENDED", 640, 226, 30, white)
	center(screen, "The moon can wait.", 640, 277, 18, muted)
	for i, b := range s.buttons() {
		drawButton(screen, b, s.selected == i)
	}
}

type debriefScene struct {
	play     *playScene
	result   settlement
	selected int
}

func (s *debriefScene) buttons() []button {
	if s.play.world.state == landed {
		return []button{buttonAt(384, 562, 512, 57, "RETURN TO OPERATIONS", "")}
	}
	return []button{buttonAt(384, 498, 512, 57, "RETRY / NO CHARGE", ""), buttonAt(384, 567, 512, 48, "RETURN TO OPERATIONS", "")}
}
func (s *debriefScene) Update(in *golib.Input, dt float32) {
	s.play.app.update(in, dt)
	action := menuInput(in, s.buttons(), &s.selected)
	if action < 0 {
		return
	}
	if s.play.world.state == crashed && action == 0 {
		golib.SwitchScene(newPlayScene(s.play.app, s.play.world.contract))
		return
	}
	golib.SwitchScene(newControlScene(s.play.app))
}
func (s *debriefScene) Draw(screen *golib.Screen) {
	s.play.Draw(screen)
	rect(screen, 0, 0, 1280, 720, fade(ink, 0.86))
	box(screen, golib.Rectangle{X: 332, Y: 104, Width: 616, Height: 537})
	r := s.result
	if s.play.world.state == landed {
		badge(screen, "CONTRACT COMPLETE", 374, 133, cyan)
		text(screen, "A GOOD DAY ON THE MOON.", 374, 180, 26, white)
		text(screen, r.Message, 374, 221, 16, muted)
		screen.DrawCircleOutline(851, 289, 38, 1, cyan)
		center(screen, r.Grade, 851, 267, 44, cyan)
		rows := []struct {
			name   string
			amount int
		}{
			{"CONTRACT PAYMENT", r.Contract}, {"LANDING PRECISION", r.Precision},
			{"FUEL EFFICIENCY", r.Fuel}, {"EXPRESS BONUS", r.Express}, {"COLONY DIVIDENDS", r.Dividends},
		}
		for i, row := range rows {
			y := float32(276 + i*39)
			text(screen, row.name, 374, y, 16, muted)
			right(screen, "+"+money(row.amount), 770, y, 20, white)
		}
		screen.DrawLine(374, 484, 906, 484, 1, lineColor)
		text(screen, "TOTAL EARNED", 374, 512, 18, cyan)
		right(screen, "+"+money(r.Total)+" CR", 906, 503, 32, cyan)
	} else {
		badge(screen, "VEHICLE RECOVERED / CARGO LOST", 374, 136, orange)
		text(screen, "THE MOON IS UNFORGIVING.", 374, 192, 26, white)
		drawShip(screen, golib.Vector2{X: 640, Y: 304}, -12, 2.2, s.play.app.progress.Selected, 0, 0)
		center(screen, "FULL INSURANCE / YOUR SHIP IS SAFE", 640, 369, 18, cyan)
		// Failure explanations are split to fit the debrief.
		lines := wrapText(r.Message, 58)
		for i, line := range lines {
			center(screen, line, 640, 413+float32(i)*24, 16, muted)
		}
	}
	for i, b := range s.buttons() {
		drawButton(screen, b, s.selected == i)
	}
	footer(screen, s.play.app)
}
func wrapText(v string, limit int) []string {
	var rows []string
	for len(v) > limit {
		at := limit
		for at > 0 && v[at] != ' ' {
			at--
		}
		if at == 0 {
			at = limit
		}
		rows = append(rows, v[:at])
		v = v[at:]
		if len(v) > 0 && v[0] == ' ' {
			v = v[1:]
		}
	}
	return append(rows, v)
}
