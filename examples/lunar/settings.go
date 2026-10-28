package main

import "golib"

type settingsScene struct {
	title        *titleScene
	selected     int
	confirmReset bool
}

func (s *settingsScene) buttons() []button {
	if s.confirmReset {
		return []button{buttonAt(410, 434, 460, 55, "KEEP MY CAMPAIGN", ""), buttonAt(410, 506, 460, 55, "CONFIRM / NEW CAMPAIGN", "")}
	}
	p := s.title.app.progress
	fx, audio, full := "ON", "ON", "OFF"
	if p.EffectsOff {
		fx = "OFF"
	}
	if p.Muted {
		audio = "OFF"
	}
	if golib.IsFullscreen() {
		full = "ON"
	}
	return []button{
		buttonAt(410, 224, 460, 55, "POST-PROCESSING  /  "+fx, ""),
		buttonAt(410, 291, 460, 55, "SOUND & MUSIC  /  "+audio, ""),
		buttonAt(410, 358, 460, 55, "FULLSCREEN  /  "+full, ""),
		buttonAt(410, 425, 460, 55, "RESET CAMPAIGN", ""),
		buttonAt(410, 492, 460, 55, "BACK TO TITLE", ""),
	}
}
func (s *settingsScene) Update(in *golib.Input, dt float32) {
	a := s.title.app
	a.update(in, dt)
	if back(in) {
		if s.confirmReset {
			s.confirmReset = false
			s.selected = 0
			return
		}
		golib.SwitchScene(s.title)
		return
	}
	action := menuInput(in, s.buttons(), &s.selected)
	if action < 0 {
		return
	}
	if s.confirmReset {
		if action == 1 {
			muted, fx := a.progress.Muted, a.progress.EffectsOff
			a.progress = newCampaign()
			a.progress.Muted = muted
			a.progress.EffectsOff = fx
			a.tell("A new frontier awaits. Campaign reset.")
			a.save()
		}
		s.confirmReset = false
		s.selected = 0
		return
	}
	switch action {
	case 0:
		a.progress.EffectsOff = !a.progress.EffectsOff
	case 1:
		a.progress.Muted = !a.progress.Muted
	case 2:
		golib.SetFullscreen(!golib.IsFullscreen())
		return
	case 3:
		s.confirmReset = true
		s.selected = 0
		return
	case 4:
		golib.SwitchScene(s.title)
		return
	}
	a.applySettings()
	a.save()
}
func (s *settingsScene) Draw(screen *golib.Screen) {
	s.title.Draw(screen)
	rect(screen, 0, 0, 1280, 720, fade(ink, 0.92))
	box(screen, golib.Rectangle{X: 365, Y: 115, Width: 550, Height: 489})
	title := "FLIGHT PREFERENCES"
	if s.confirmReset {
		title = "START FROM THE BEGINNING?"
	}
	center(screen, title, 640, 159, 26, white)
	if s.confirmReset {
		center(screen, "Your credits, ships, upgrades and colonies", 640, 269, 18, muted)
		center(screen, "will be replaced by a fresh campaign.", 640, 302, 18, muted)
		center(screen, "This cannot be undone.", 640, 354, 20, orange)
	}
	for i, b := range s.buttons() {
		drawButton(screen, b, s.selected == i)
	}
	footer(screen, s.title.app)
}
