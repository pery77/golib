package main

import (
	"fmt"

	"golib"
)

// Tuning: how playing a level feels.
const (
	slideTime   = 0.08 // seconds the player, and a pushed crate, take to slide one cell
	repeatDelay = 0.25 // seconds a direction is held before the player keeps walking
	repeatTime  = 0.12 // seconds between steps while the direction stays held
	flashTime   = 0.35 // seconds a crate glows after landing on a goal
	solvedDelay = 0.7  // seconds between the last crate landing and the level-complete message
)

// playScene is a level being played. Update turns the keyboard and gamepad
// into steps for the world, which holds the rules (world.go), and Draw draws
// the room.
type playScene struct {
	session    *session
	level      int // index into session.levels
	world      world
	directions directionReader
	held       direction // the direction that keeps walking while held, or none
	repeat     float32   // seconds until the held direction takes its next step
	undoRepeat float32   // seconds until a held undo takes back the next step

	// The last step slides on the screen: the player from the cell from, and
	// the crate it pushed, if any, from crateFrom.
	slide     float32 // seconds left of the slide; 0 when still
	from      cell
	pushed    int // the crate sliding, or -1
	crateFrom cell

	flash      float32 // seconds left of the glow of the crate flashed
	flashed    int
	solvedTime float32 // seconds since the last crate landed
	best       bool    // the finish is the level's best
}

// newPlayScene starts level, counted from 0.
func newPlayScene(s *session, level int) *playScene {
	return &playScene{session: s, level: level, world: newWorld(s.levels[level]), pushed: -1, flashed: -1}
}

// Update reads the input and steps the world. golib.Run calls it 60 times per
// second, always with dt = 1/60.
func (s *playScene) Update(input *golib.Input, dt float32) {
	s.session.update(input)
	s.slide = max(0, s.slide-dt)
	s.flash = max(0, s.flash-dt)

	if s.world.solved() {
		s.solvedTime += dt
		if s.solvedTime >= solvedDelay {
			golib.SwitchScene(newCompleteScene(s))
		}
		return
	}
	// Read the directions in every update, so the stick's last direction stays
	// current even while undo is held.
	pressed := s.directions.pressed(input)
	switch {
	case pausePressed(input):
		golib.SwitchScene(newPauseScene(s))
		return
	case undoPressed(input):
		s.undoRepeat = repeatDelay
		s.undo(true)
		return
	case undoHeld(input):
		// Held, undo keeps taking steps back, like walking.
		s.undoRepeat -= dt
		if s.undoRepeat <= 0 {
			s.undoRepeat += repeatTime
			s.undo(false)
		}
		return
	case restartPressed(input):
		s.restart()
		return
	}

	// A press steps at once. Held, the direction waits a moment, then keeps
	// stepping, so a tap never walks two cells.
	switch {
	case pressed != noDirection:
		s.held, s.repeat = pressed, repeatDelay
		s.step(pressed, true)
	case s.held != noDirection && s.directions.held(input, s.held):
		s.repeat -= dt
		if s.repeat <= 0 {
			s.repeat += repeatTime
			s.step(s.held, false)
		}
	default:
		s.held = noDirection
	}
}

// step walks the player in direction d, and starts the slide and the sound
// that go with it. pressed says whether the key went down in this update, so
// that holding a direction against a wall knocks only once.
func (s *playScene) step(d direction, pressed bool) {
	from := s.world.player
	result := s.world.step(d)
	if !result.moved {
		if pressed {
			bumpSound.Play()
		}
		return
	}
	s.slide, s.from, s.pushed = slideTime, from, result.pushed
	if result.pushed >= 0 {
		s.crateFrom = s.world.crates[result.pushed].next(d.opposite())
	}
	switch {
	case result.landed:
		s.flash, s.flashed = flashTime, result.pushed
		landSound.Play()
	case result.pushed >= 0:
		pushSound.Play()
	default:
		stepSound.Play()
	}
	if s.world.solved() {
		s.solvedTime = 0
		s.best = s.session.finish(s.world.layout, s.world.moves)
		completeSound.Play()
	}
}

// undo takes back the last step, at once. pressed says whether the key went
// down in this update, so that holding undo with nothing left knocks only
// once.
func (s *playScene) undo(pressed bool) {
	if !s.world.undo() {
		if pressed {
			bumpSound.Play()
		}
		return
	}
	s.slide, s.flash, s.held = 0, 0, noDirection
	undoSound.Play()
}

// restart puts the level back as it started.
func (s *playScene) restart() {
	if len(s.world.history) == 0 {
		return
	}
	s.world = newWorld(s.world.layout)
	s.slide, s.flash, s.held = 0, 0, noDirection
	restartSound.Play()
}

// Draw draws the room, then the heading and the hints. It reads the state
// and never changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(backgroundColor)
	l := s.world.layout
	x, y := roomOrigin(l)
	screen.DrawMapLayer(levelMaps[s.level], floorLayer, x, y)

	// The last step slides from the old cells to the new ones.
	moved := easeOut(1 - s.slide/slideTime)
	for i, c := range s.world.crates {
		cx, cy := cellPosition(c, x, y)
		sliding := i == s.pushed && s.slide > 0
		if sliding {
			fromX, fromY := cellPosition(s.crateFrom, x, y)
			cx, cy = between(fromX, fromY, cx, cy, moved)
		}
		frame := crateFrame
		if l.at(c) == goal && !sliding {
			frame = crateOnGoalFrame
		}
		screen.DrawSprite(tiles, frame, cx, cy)
		if i == s.flashed && s.flash > 0 {
			glow := golib.Color{R: 255, G: 255, B: 220, A: uint8(200 * s.flash / flashTime)}
			screen.DrawRectangle(golib.Rectangle{X: cx + 1, Y: cy + 1, Width: tileSize - 2, Height: tileSize - 2}, glow)
		}
	}
	px, py := cellPosition(s.world.player, x, y)
	frames := playerFrames[s.world.facing]
	frame := frames[0]
	if s.slide > 0 {
		fromX, fromY := cellPosition(s.from, x, y)
		px, py = between(fromX, fromY, px, py, moved)
		frame = frames[1]
	}
	screen.DrawSprite(tiles, frame, px, py)

	drawShadowed(screen, fmt.Sprintf("%d. %s", s.level+1, l.title), 4, headingY, 10, textColor)
	moves := fmt.Sprintf("Moves %d", s.world.moves)
	par := fmt.Sprintf("  Par %d", l.par)
	drawRight(screen, par, screenWidth-4, headingY, 10, dimTextColor)
	movesColor := textColor
	if s.world.moves > l.par {
		movesColor = highlightColor // over par: still fine, but no star
	}
	drawRight(screen, moves, screenWidth-4-screen.TextWidth(par, 10), headingY, 10, movesColor)

	screen.DrawSprite(tiles, crateOnGoalFrame, 4, hintY-3)
	drawShadowed(screen, fmt.Sprintf("%d/%d", s.world.cratesOnGoals(), len(s.world.crates)), 23, hintY, 10, textColor)
	hints := "Z: undo  R: restart  Esc: menu"
	if s.session.gamepad {
		hints = "B: undo  Y: restart  Start: menu"
	}
	drawRight(screen, hints, screenWidth-4, hintY, 10, dimTextColor)
}
