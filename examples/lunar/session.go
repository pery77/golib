package main

import "golib"

type session struct {
	progress    campaign
	stars       []star
	time        float32
	notice      string
	noticeTimer golib.Timer
}

func newSession() *session {
	s := &session{progress: newCampaign(), stars: makeStars()}
	if _, err := golib.LoadData("campaign", &s.progress); err != nil {
		s.progress = newCampaign()
		s.tell("Save could not be read. Starting a new local campaign.")
	}
	s.progress.normalize()
	return s
}
func (s *session) tell(message string) { s.notice = message; s.noticeTimer.Start(7) }
func (s *session) save() {
	if err := golib.SaveData("campaign", s.progress); err != nil {
		s.tell("Progress could not be saved. Keep this session open.")
	}
}
func (s *session) applySettings() {
	if s.progress.EffectsOff {
		golib.SetPostProcess()
	} else {
		golib.SetPostProcess(cinema)
	}
	if s.progress.Muted {
		golib.SetVolume(0)
	} else {
		golib.SetVolume(0.65)
	}
	theme.SetVolume(0.22)
}
func (s *session) update(in *golib.Input, dt float32) {
	s.time += dt
	if s.noticeTimer.Tick(dt) {
		s.notice = ""
	}
	alt := in.KeyDown(golib.KeyLeftAlt) || in.KeyDown(golib.KeyRightAlt)
	if in.KeyPressed(golib.KeyF11) || (alt && in.KeyPressed(golib.KeyEnter)) {
		golib.SetFullscreen(!golib.IsFullscreen())
	}
	changed := false
	if in.KeyPressed(golib.KeyF2) {
		s.progress.EffectsOff = !s.progress.EffectsOff
		changed = true
	}
	if in.KeyPressed(golib.KeyF5) {
		s.reloadCinema()
	}
	if in.KeyPressed(golib.KeyM) {
		s.progress.Muted = !s.progress.Muted
		changed = true
	}
	if changed {
		s.applySettings()
		s.save()
	}
	theme.Play()
}
func confirm(in *golib.Input) bool {
	return (in.KeyPressed(golib.KeyEnter) && !in.KeyDown(golib.KeyLeftAlt) && !in.KeyDown(golib.KeyRightAlt)) || in.GamepadPressed(0, golib.GamepadA)
}
func back(in *golib.Input) bool {
	return in.KeyPressed(golib.KeyEscape) || in.GamepadPressed(0, golib.GamepadB)
}
func pausePressed(in *golib.Input) bool {
	return in.KeyPressed(golib.KeyEscape) || in.KeyPressed(golib.KeyP) || in.GamepadPressed(0, golib.GamepadStart)
}
