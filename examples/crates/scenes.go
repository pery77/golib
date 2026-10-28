package main

import (
	"fmt"

	"golib"
)

// The game's scenes besides play (play.go). Each one is a golib.Game: golib.Run
// starts with the title, and golib.SwitchScene moves between them.
//
//	title --Continue--> play --Esc/Start--> pause --Resume--> the same play
//	  |                  |                    |---Restart---> a new play
//	  |                  |                    |---Levels----> levels
//	  |                  |                    '---Title-----> title
//	  |                  '--last crate--> complete --Next--> play (next level), or the end after the last
//	  |                                     |------Replay--> a new play
//	  |                                     '------Levels--> levels
//	  |--Levels--> levels --Enter/click--> play
//	  |              '------Esc/B------> title
//	  '--Quit or Esc--> quit
//
// Every scene calls session.update first, for fullscreen and the music.

// Title screen buttons, as indexes into titleScene.menu.
const (
	continueButton = iota
	levelListButton
	titleMusicButton
	quitButton
)

// titleScene shows the game's name, its controls and a menu, over the
// warehouse.
type titleScene struct {
	session *session
	menu    menu
}

func newTitleScene(s *session) *titleScene {
	t := &titleScene{session: s, menu: newMenu(60, "", "Level list", "", "Quit")}
	t.setLabels()
	return t
}

// setLabels writes the buttons whose labels depend on the progress.
func (t *titleScene) setLabels() {
	next := t.session.nextLevel()
	t.menu.labels[continueButton] = fmt.Sprintf("Play level %d", next+1)
	if finished, _ := t.session.finishedLevels(); finished > 0 && finished < len(t.session.levels) {
		t.menu.labels[continueButton] = fmt.Sprintf("Continue: level %d", next+1)
	}
	t.menu.labels[titleMusicButton] = "Music: " + onOff(t.session.musicOn())
}

func (t *titleScene) Update(input *golib.Input, dt float32) {
	t.session.update(input)
	choice := t.menu.update(input)
	if input.GamepadPressed(0, golib.GamepadStart) {
		choice = t.menu.selected
	}
	// No key quits by itself, not even Esc: the game calls golib.Quit.
	switch {
	case input.KeyPressed(golib.KeyEscape) || choice == quitButton:
		golib.Quit()
		return
	case choice == continueButton:
		golib.SwitchScene(newPlayScene(t.session, t.session.nextLevel()))
		return
	case choice == levelListButton:
		golib.SwitchScene(newLevelsScene(t.session, t.session.nextLevel()))
		return
	case choice == titleMusicButton:
		t.session.toggleMusic()
	}
	t.setLabels()
}

func (t *titleScene) Draw(screen *golib.Screen) {
	drawWarehouse(screen)
	drawCentered(screen, "CRATES", 12, 30, highlightColor)
	drawCentered(screen, "Push every crate onto a goal", 44, 10, textColor)
	t.menu.draw(screen)
	if t.session.gamepad {
		drawCentered(screen, "D-pad: walk   B: undo   Y: restart", 132, 10, dimTextColor)
		drawCentered(screen, "Start: menu   Back: music   A: choose", 144, 10, dimTextColor)
	} else {
		drawCentered(screen, "Arrows or WASD: walk   Z: undo   R: restart", 132, 10, dimTextColor)
		drawCentered(screen, "Esc: menu   M: music   F11: fullscreen", 144, 10, dimTextColor)
	}
	if t.session.saveErr != nil {
		drawCentered(screen, "Progress can't be saved (see the console)", 164, 10, highlightColor)
	} else if finished, atPar := t.session.finishedLevels(); finished > 0 {
		status := fmt.Sprintf("%d of %d levels finished, %d at par", finished, len(t.session.levels), atPar)
		drawCentered(screen, status, 164, 10, textColor)
	}
}

// onOff turns a setting into the word a button shows.
func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// The level list: a grid of boxes, one per level.
const (
	levelColumns   = 5
	levelBoxWidth  = 48 // pixels
	levelBoxHeight = 38
	levelBoxGap    = 8
	levelsTop      = 34
)

// levelBox returns where the box of level i, counted from 0, is on the screen.
func levelBox(i int) golib.Rectangle {
	left := (screenWidth - (levelColumns*levelBoxWidth + (levelColumns-1)*levelBoxGap)) / 2
	return golib.Rectangle{
		X:      float32(left + i%levelColumns*(levelBoxWidth+levelBoxGap)),
		Y:      float32(levelsTop + i/levelColumns*(levelBoxHeight+levelBoxGap)),
		Width:  levelBoxWidth,
		Height: levelBoxHeight,
	}
}

// levelsScene lists the levels: finished ones show their best moves, and a
// star at par; locked ones are dimmed.
type levelsScene struct {
	session    *session
	selected   int
	directions directionReader
	pointer    pointer
}

func newLevelsScene(s *session, selected int) *levelsScene {
	return &levelsScene{session: s, selected: selected}
}

func (s *levelsScene) Update(input *golib.Input, dt float32) {
	s.session.update(input)
	count := len(s.session.levels)
	selected := s.selected
	switch s.directions.pressed(input) {
	case left:
		selected--
	case right:
		selected++
	case up:
		selected -= levelColumns
	case down:
		selected += levelColumns
	}
	if wheel := input.MouseWheel(); wheel != 0 {
		selected -= int(wheel)
	}
	// The pointer picks the level under it when it moves or clicks, so a
	// resting pointer doesn't fight the keys.
	x, y, moved := s.pointer.read(input)
	pointed := -1
	for i := range count {
		if levelBox(i).Contains(x, y) {
			pointed = i
		}
	}
	clicked := input.MousePressed(golib.MouseLeft) && pointed >= 0
	if pointed >= 0 && (moved || clicked) {
		selected = pointed
	}
	if selected = max(0, min(selected, count-1)); selected != s.selected {
		s.selected = selected
		menuMoveSound.Play()
	}

	switch {
	case backPressed(input):
		golib.SwitchScene(newTitleScene(s.session))
	case clicked || confirmPressed(input) || input.GamepadPressed(0, golib.GamepadStart):
		if !s.session.unlocked(s.selected) {
			bumpSound.Play()
			return
		}
		menuSelectSound.Play()
		golib.SwitchScene(newPlayScene(s.session, s.selected))
	}
}

func (s *levelsScene) Draw(screen *golib.Screen) {
	drawWarehouse(screen)
	drawCentered(screen, "Levels", 10, 20, highlightColor)
	for i, l := range s.session.levels {
		box := levelBox(i)
		unlocked := s.session.unlocked(i)
		color := buttonColor
		switch {
		case i == s.selected && !unlocked:
			color = lockedSelectedColor
		case i == s.selected:
			color = buttonSelectedColor
		case !unlocked:
			color = lockedColor
		}
		screen.DrawRectangle(box, color)
		numberColor := textColor
		if !unlocked {
			numberColor = dimTextColor
		}
		number := fmt.Sprint(i + 1)
		drawShadowed(screen, number, box.X+(box.Width-screen.TextWidth(number, 20))/2, box.Y+5, 20, numberColor)
		best, finished := s.session.progress.Best[l.name]
		switch {
		case finished:
			moves := fmt.Sprint(best)
			drawShadowed(screen, moves, box.X+(box.Width-screen.TextWidth(moves, 10))/2, box.Y+26, 10, textColor)
			if best <= l.par {
				drawStar(screen, box.X+box.Width-7, box.Y+7, 5, highlightColor)
			}
		case !unlocked:
			drawShadowed(screen, "locked", box.X+(box.Width-screen.TextWidth("locked", 10))/2, box.Y+26, 10, dimTextColor)
		}
	}

	l := s.session.levels[s.selected]
	info := fmt.Sprintf("%d. %s   Par %d", s.selected+1, l.title, l.par)
	if best, ok := s.session.progress.Best[l.name]; ok {
		info += fmt.Sprintf("   Best %d", best)
	}
	drawCentered(screen, info, 134, 10, textColor)
	if !s.session.unlocked(s.selected) {
		drawCentered(screen, fmt.Sprintf("Finish level %d to open it", s.selected), 148, 10, dimTextColor)
	}
	drawCentered(screen, "Enter or A: play   Esc or B: back", 164, 10, dimTextColor)
}

// Pause menu buttons, as indexes into pauseScene.menu.
const (
	resumeButton = iota
	restartButton
	pauseLevelsButton
	pauseMusicButton
	titleButton
)

// pauseScene freezes a play scene under a menu. Resuming switches back to
// that same play scene, so the level carries on where it stopped.
type pauseScene struct {
	paused *playScene
	menu   menu
}

func newPauseScene(paused *playScene) *pauseScene {
	p := &pauseScene{paused: paused, menu: newMenu(56, "Resume", "Restart level", "Level list", "", "Title screen")}
	p.setLabels()
	return p
}

func (p *pauseScene) setLabels() {
	p.menu.labels[pauseMusicButton] = "Music: " + onOff(p.paused.session.musicOn())
}

func (p *pauseScene) Update(input *golib.Input, dt float32) {
	s := p.paused.session
	s.update(input)
	choice := p.menu.update(input)
	switch {
	case pausePressed(input) || backPressed(input) || choice == resumeButton:
		golib.SwitchScene(p.paused)
		return
	case choice == restartButton:
		p.paused.restart()
		golib.SwitchScene(p.paused)
		return
	case choice == pauseLevelsButton:
		golib.SwitchScene(newLevelsScene(s, p.paused.level))
		return
	case choice == pauseMusicButton:
		s.toggleMusic()
	case choice == titleButton:
		golib.SwitchScene(newTitleScene(s))
		return
	}
	p.setLabels()
}

func (p *pauseScene) Draw(screen *golib.Screen) {
	p.paused.Draw(screen)
	drawPanel(screen, 18, 146)
	drawCentered(screen, "Paused", 26, 20, highlightColor)
	p.menu.draw(screen)
}

// Level-complete buttons, as indexes into completeScene.menu.
const (
	nextButton = iota
	replayButton
	completeLevelsButton
)

// completeScene shows the finished room under the result, and what to do
// next.
type completeScene struct {
	finished *playScene
	menu     menu
}

func newCompleteScene(finished *playScene) *completeScene {
	next := "Next level"
	if finished.level == len(finished.session.levels)-1 {
		next = "Finish"
	}
	return &completeScene{finished: finished, menu: newMenu(100, next, "Replay", "Level list")}
}

func (c *completeScene) Update(input *golib.Input, dt float32) {
	f := c.finished
	f.session.update(input)
	choice := c.menu.update(input)
	if input.GamepadPressed(0, golib.GamepadStart) {
		choice = c.menu.selected
	}
	switch {
	case choice == nextButton && f.level == len(f.session.levels)-1:
		golib.SwitchScene(&endScene{session: f.session})
	case choice == nextButton:
		golib.SwitchScene(newPlayScene(f.session, f.level+1))
	case choice == replayButton || restartPressed(input):
		golib.SwitchScene(newPlayScene(f.session, f.level))
	case choice == completeLevelsButton || backPressed(input):
		golib.SwitchScene(newLevelsScene(f.session, min(f.level+1, len(f.session.levels)-1)))
	}
}

func (c *completeScene) Draw(screen *golib.Screen) {
	f := c.finished
	f.Draw(screen)
	drawPanel(screen, 14, 156)
	drawCentered(screen, fmt.Sprintf("Level %d complete!", f.level+1), 22, 20, highlightColor)
	moves, par := f.world.moves, f.world.layout.par
	drawCentered(screen, fmt.Sprintf("%d moves, %d pushes   Par %d", moves, f.world.pushes, par), 48, 10, textColor)
	if moves <= par {
		drawStar(screen, screenWidth/2, 70, 8, highlightColor)
		drawCentered(screen, "At par!", 82, 10, highlightColor)
	} else {
		result := fmt.Sprintf("%d over par: replay for the star", moves-par)
		if f.best {
			drawCentered(screen, "New best!", 64, 10, highlightColor)
		}
		drawCentered(screen, result, 78, 10, textColor)
	}
	c.menu.draw(screen)
}

// endScene thanks the player after the last level.
type endScene struct {
	session *session
}

func (e *endScene) Update(input *golib.Input, dt float32) {
	e.session.update(input)
	if confirmPressed(input) || backPressed(input) || input.GamepadPressed(0, golib.GamepadStart) ||
		input.MousePressed(golib.MouseLeft) {
		menuSelectSound.Play()
		golib.SwitchScene(newTitleScene(e.session))
	}
}

func (e *endScene) Draw(screen *golib.Screen) {
	drawWarehouse(screen)
	finished, atPar := e.session.finishedLevels()
	drawCentered(screen, "Every crate is home!", 40, 20, highlightColor)
	drawCentered(screen, fmt.Sprintf("You finished %d of %d levels,", finished, len(e.session.levels)), 76, 10, textColor)
	drawCentered(screen, fmt.Sprintf("%d of them at par.", atPar), 90, 10, textColor)
	if atPar < len(e.session.levels) {
		drawCentered(screen, "The level list shows where a star is missing.", 112, 10, dimTextColor)
	}
	drawCentered(screen, "Thanks for playing!", 134, 10, textColor)
	drawCentered(screen, "Enter or A: back to the title", 160, 10, dimTextColor)
}
