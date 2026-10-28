package main

import (
	"fmt"

	"golib"
)

// The game's scenes. golib.Run starts with the title, and golib.SwitchScene
// moves between them.
//
//	title --Enter/click/A--> play level 1 --cleared--> cleared --Enter/click/A--> play level 2 ... --> won --> title
//	play --Esc/P/Start--> pause --Esc/P/Start--> the same play
//	                        '--R/Y--> the level again   '--Q/B--> title

// record is what the game remembers between runs.
type record struct {
	BestTotal int   // fewest balls for all the yards in one game; 0 before the first finish
	BestLevel []int // fewest balls for each level; 0 where it hasn't been cleared
	Reached   int   // the furthest level opened, from 0: the title can start there
}

const (
	recordName   = "record"
	clearedDelay = 1.2 // seconds of cheering before the cleared screen
	titleSpin    = 8   // degrees per second the title's camera goes round
)

func loadRecord() record {
	var r record
	golib.LoadData(recordName, &r) // nothing saved, or unreadable: start from nothing
	return r
}

// run is one game, from the first level to the last.
type run struct {
	levels []*level
	first  int   // the level it started at: only a game from the first counts for BestTotal
	balls  []int // balls used on each level cleared so far
	record record
	saved  string // what happened to the record, to show
}

// total is the balls used so far.
func (r *run) total() int {
	n := 0
	for _, b := range r.balls {
		n += b
	}
	return n
}

// finishLevel keeps a cleared level's balls, and the record when it is beaten.
func (r *run) finishLevel(n, balls int) {
	r.balls = append(r.balls, balls)
	for len(r.record.BestLevel) <= n {
		r.record.BestLevel = append(r.record.BestLevel, 0)
	}
	changed := false
	if best := r.record.BestLevel[n]; best == 0 || balls < best {
		r.record.BestLevel[n] = balls
		changed = true
	}
	if n+1 < len(r.levels) && n+1 > r.record.Reached {
		r.record.Reached = n + 1
		changed = true
	}
	if r.first == 0 && len(r.balls) == len(r.levels) {
		if r.record.BestTotal == 0 || r.total() < r.record.BestTotal {
			r.record.BestTotal = r.total()
			changed = true
		}
	}
	if changed {
		if err := golib.SaveData(recordName, r.record); err != nil {
			r.saved = "Your best score couldn't be saved"
		}
	}
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
		input.MousePressed(golib.MouseLeft) || input.GamepadPressed(0, golib.GamepadA) || input.GamepadPressed(0, golib.GamepadStart)
}

// pausePressed says whether the player asked to pause, or to go back from a pause.
func pausePressed(input *golib.Input) bool {
	return input.KeyPressed(golib.KeyEscape) || input.KeyPressed(golib.KeyP) || input.GamepadPressed(0, golib.GamepadStart)
}

// titleScene shows the game's name and controls over the first level,
// while the camera goes slowly round.
type titleScene struct {
	levels []*level
	world  *world
	camera *golib.Camera3D
	record record
	yaw    float32
}

func newTitleScene(levels []*level) *titleScene {
	s := &titleScene{levels: levels, world: newWorld(levels, 0), camera: golib.NewCamera3D(screenWidth, screenHeight), record: loadRecord(), yaw: 200}
	golib.SetMouseVisible(true)
	s.place()
	return s
}

func (s *titleScene) place() {
	s.camera.Orbit(s.world.focus().Add(golib.Vector3{Y: 1}), s.yaw, -22, 13)
}

func (s *titleScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadBack) {
		golib.Quit()
		return
	}
	if confirmed(input) {
		confirmSound.Play()
		golib.SwitchScene(newPlayScene(&run{levels: s.levels, record: s.record}, 0))
		return
	}
	for n, key := range []golib.Key{golib.KeyOne, golib.KeyTwo, golib.KeyThree} {
		if input.KeyPressed(key) && n <= s.record.Reached && n < len(s.levels) {
			confirmSound.Play()
			golib.SwitchScene(newPlayScene(&run{levels: s.levels, record: s.record, first: n}, n))
			return
		}
	}
	s.yaw += titleSpin * dt
	s.world.step(intent{}, dt) // the stacks settle as the title shows
	s.place()
}

func (s *titleScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, look{camera: s.camera})
	screen.DrawRectangle(golib.Rectangle{Width: 560, Height: screen.Height()}, panelColor)
	const x = 56
	drawText(screen, "TOPPLE", x, 60, 110, accentColor)
	lines := []string{
		"Knock every crate and barrel off",
		"the platforms, with as few balls",
		"as you can. Three yards to clear.",
	}
	for i, line := range lines {
		drawText(screen, line, x, 200+float32(i)*30, 22, textColor)
	}
	controls := [][2]string{
		{"Mouse", "aim"},
		{"Click, Space", "throw"},
		{"Arrows, WASD", "walk, push crates"},
		{"Esc", "pause"},
		{"F11", "fullscreen"},
	}
	for i, c := range controls {
		y := 330 + float32(i)*30
		drawText(screen, c[0], x, y, 20, accentColor)
		drawText(screen, c[1], x+200, y, 20, textColor)
	}
	if golib.PlayingWithGamepad() {
		drawText(screen, "Gamepad: left stick walks, right stick aims, A throws", x, 490, 16, textColor)
	}
	if s.record.BestTotal > 0 {
		drawText(screen, fmt.Sprintf("Best: %d balls for all three", s.record.BestTotal), x, 530, 22, textColor)
	}
	if s.record.Reached > 0 {
		drawText(screen, fmt.Sprintf("1 to %d: start at that yard", min(s.record.Reached+1, len(s.levels))), x, 562, 20, textColor)
	}
	if int(s.world.time*2)%2 == 0 {
		drawText(screen, "Click or press Enter to start", x, 610, 26, accentColor)
	}
}

// playScene is a level being played.
type playScene struct {
	run      *run
	world    *world
	view     *view
	aim      aimer
	animator *golib.Animator
	arc      []golib.Vector3
	popups   []popup
	cheering float32 // seconds since the level was cleared
}

func newPlayScene(r *run, n int) *playScene {
	w := newWorld(r.levels, n)
	s := &playScene{run: r, world: w, view: newView(w), aim: newAimer(), animator: golib.NewAnimator(hero)}
	s.arc = arcOf(w, controls{}.intent(s.view.camera, s.aim))
	golib.SetMouseVisible(false)
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	if pausePressed(input) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	c := readControls(input)
	s.play(c, dt)
	if s.world.cleared && s.cheering >= clearedDelay {
		s.run.finishLevel(s.world.number, s.world.thrown)
		golib.SwitchScene(newClearedScene(s))
	}
}

// play moves the level on by one update with the controls c.
func (s *playScene) play(c controls, dt float32) {
	s.aim.update(c, dt)
	want := c.intent(s.view.camera, s.aim)
	happened := s.world.step(want, dt)
	playEvents(happened, s.view.camera)
	s.popups = agePopups(s.popups, dt)
	for _, at := range happened.fell {
		s.popups = append(s.popups, popup{at: at})
	}
	s.view.follow(s.world, dt)
	s.arc = arcOf(s.world, c.intent(s.view.camera, s.aim))
	if s.world.cleared {
		s.cheering += dt
	}
	s.pose(dt)
}

// pose picks what the player's animation plays from what it does.
func (s *playScene) pose(dt float32) {
	p := &s.world.player
	switch {
	case s.world.cleared:
		s.animator.Play("emote-yes")
	case p.throwing > 0.3:
		s.animator.PlayWith("attack-melee-right", golib.AnimationOptions{Once: true, Restart: true, Speed: 1.6, Then: "idle"})
	case p.throwing > 0:
	case p.walking:
		s.animator.PlayWith("walk", golib.AnimationOptions{Speed: 1.4})
	default:
		s.animator.Play("idle")
	}
	s.animator.Update(dt)
}

func (s *playScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.world, look{camera: s.view.camera, animator: s.animator, arc: s.arc})
	drawPopups(screen, s.popups, s.view.camera)
	drawHUD(screen, s.world, s.run)
	if s.world.cleared {
		drawCentered(screen, "CLEARED!", screenWidth/2, 250, 80, accentColor)
		return
	}
	drawCrosshair(screen, s.aim.x, s.aim.y)
}

// pauseScene freezes a level and shows the choices over it.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	switch {
	case pausePressed(input):
		golib.SwitchScene(s.paused)
	case input.KeyPressed(golib.KeyR) || input.GamepadPressed(0, golib.GamepadY):
		golib.SwitchScene(newPlayScene(s.paused.run, s.paused.world.number))
	case input.KeyPressed(golib.KeyQ) || input.GamepadPressed(0, golib.GamepadB):
		golib.SwitchScene(newTitleScene(s.paused.run.levels))
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen)
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, panelColor)
	drawCentered(screen, "PAUSED", screenWidth/2, 220, 72, accentColor)
	drawCentered(screen, "Esc: carry on     R: try this level again     Q: back to the title", screenWidth/2, 340, 24, textColor)
}

// clearedScene shows how a level went, over it, before the next one.
type clearedScene struct {
	played *playScene
	time   float32
}

func newClearedScene(played *playScene) *clearedScene {
	golib.SetMouseVisible(true)
	return &clearedScene{played: played}
}

func (s *clearedScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	s.time += dt
	s.played.world.step(intent{}, dt)
	s.played.pose(dt)
	if s.time < 0.4 || !confirmed(input) {
		return
	}
	confirmSound.Play()
	r := s.played.run
	next := s.played.world.number + 1
	if next >= len(r.levels) {
		golib.SwitchScene(&wonScene{run: r, behind: s.played})
		return
	}
	golib.SwitchScene(newPlayScene(r, next))
}

func (s *clearedScene) Draw(screen *golib.Screen) {
	w := s.played.world
	drawWorld(screen, w, look{camera: s.played.view.camera, animator: s.played.animator})
	panel := golib.Rectangle{X: screenWidth/2 - 300, Y: 150, Width: 600, Height: 380}
	screen.DrawRectangle(panel, panelColor)
	drawCentered(screen, fmt.Sprintf("%s cleared!", w.level.Name), screenWidth/2, 180, 44, accentColor)
	drawStars(screen, screenWidth/2, 240, 64, stars(w.thrown, w.level.Par))
	drawCentered(screen, fmt.Sprintf("Balls: %d   (par %d)", w.thrown, w.level.Par), screenWidth/2, 330, 28, textColor)
	r := s.played.run
	if best := r.record.BestLevel[w.number]; best > 0 {
		drawCentered(screen, fmt.Sprintf("Best for this yard: %d", best), screenWidth/2, 375, 22, textColor)
	}
	next := "Click or press Enter for the next yard"
	if w.number+1 >= len(r.levels) {
		next = "Click or press Enter"
	}
	drawCentered(screen, next, screenWidth/2, 470, 24, accentColor)
}

// wonScene sums up the whole game.
type wonScene struct {
	run    *run
	behind *playScene
	time   float32
}

func (s *wonScene) Update(input *golib.Input, dt float32) {
	toggleFullscreen(input)
	s.time += dt
	s.behind.world.step(intent{}, dt)
	s.behind.pose(dt)
	if s.time > 0.4 && confirmed(input) {
		golib.SwitchScene(newTitleScene(s.run.levels))
	}
}

func (s *wonScene) Draw(screen *golib.Screen) {
	drawWorld(screen, s.behind.world, look{camera: s.behind.view.camera, animator: s.behind.animator})
	screen.DrawRectangle(golib.Rectangle{Width: screenWidth, Height: screenHeight}, panelColor)
	drawCentered(screen, "EVERY YARD CLEARED", screenWidth/2, 110, 60, accentColor)
	for i, balls := range s.run.balls {
		l := s.run.levels[s.run.first+i]
		y := 230 + float32(i)*44
		drawText(screen, l.Name, 300, y, 28, textColor)
		drawText(screen, fmt.Sprintf("%d balls (par %d)", balls, l.Par), 600, y, 28, textColor)
		drawStars(screen, 940, y-2, 30, stars(balls, l.Par))
	}
	y := 230 + float32(len(s.run.balls))*44 + 30
	drawCentered(screen, fmt.Sprintf("Total: %d balls", s.run.total()), screenWidth/2, y, 40, accentColor)
	switch {
	case s.run.first > 0:
		drawCentered(screen, "Start from the first yard for a best total", screenWidth/2, y+56, 24, textColor)
	case s.run.record.BestTotal == s.run.total():
		drawCentered(screen, "A new best!", screenWidth/2, y+56, 28, textColor)
	default:
		drawCentered(screen, fmt.Sprintf("Best: %d balls", s.run.record.BestTotal), screenWidth/2, y+56, 28, textColor)
	}
	if s.run.saved != "" {
		drawCentered(screen, s.run.saved, screenWidth/2, y+100, 20, textColor)
	}
	drawCentered(screen, "Click or press Enter for the title", screenWidth/2, 640, 24, accentColor)
}
