package main

import (
	"fmt"

	"golib"
)

// Horizontal auto-repeat: after the first sideways move, holding the key waits
// moveDelay seconds and then moves one cell every moveRepeat seconds, so a held
// key slides the piece steadily instead of jumping.
const (
	moveDelay  = 0.17 // seconds before the piece starts sliding
	moveRepeat = 0.05 // seconds between cells while held
)

// piece is the play scene: it turns the keyboard and gamepad into actions for
// the world (world.go), plays a sound and starts an effect for everything the
// world reports, and draws the game (draw.go).
type playScene struct {
	options *options
	records *records
	world   *world

	moveDir   int     // -1, 0 or 1: the way a held key slides the piece
	moveTimer float32 // seconds until the next repeated slide

	particles []particle
	time      float32

	// shake, flash and toast are the effects a clear, a level-up or a hard
	// drop sets off. shakeX and shakeY offset the board so it shudders.
	shakeTime      float32
	shakeStrength  float32
	shakeDuration  float32
	shakeX, shakeY float32
	flash          float32 // 0 to 1: a white wash over the board
	toast          string  // "TETRIS!", "LEVEL 2" and the like
	toastTimer     float32
}

func newPlayScene(o *options, r *records) *playScene {
	return &playScene{options: o, records: r, world: newWorld()}
}

// Update reads the input and advances the world. golib.Run calls it 60 times
// per second, always with dt = 1/60.
func (s *playScene) Update(input *golib.Input, dt float32) {
	s.time += dt
	s.options.handleKeys(input)

	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}

	a := s.readActions(input, dt)
	before := s.world.piece
	s.world.step(a, dt)
	s.playFellSound(before)
	s.handleEvents()
	s.updateEffects(dt)

	if s.world.phase == phaseGameOver {
		s.records.noteGame(s.world.score)
		golib.SwitchScene(&gameOverScene{finished: s})
	}
}

// readActions turns the keyboard, the d-pad and the stick into one update's
// actions for the world. The world never sees the input itself.
func (s *playScene) readActions(input *golib.Input, dt float32) actions {
	var a actions

	// Sideways: one cell at once when the direction changes, then auto-repeat
	// while the key or d-pad button stays held.
	dir := 0
	if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
		dir = -1
	}
	if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
		dir = 1
	}
	if dir == 0 {
		// The stick slides when pushed well past the dead zone.
		if x, _ := input.GamepadLeftStick(0); x > 0.45 {
			dir = 1
		} else if x < -0.45 {
			dir = -1
		}
	}
	switch {
	case dir != s.moveDir:
		s.moveDir = dir
		s.moveTimer = moveDelay
		a.moveX = dir
	case dir != 0:
		s.moveTimer -= dt
		if s.moveTimer <= 0 {
			s.moveTimer = moveRepeat
			a.moveX = dir
		}
	}

	// Pressed, not down: a held key turns or drops only once.
	a.rotateCW = input.KeyPressed(golib.KeyUp) || input.KeyPressed(golib.KeyX) ||
		input.GamepadPressed(0, golib.GamepadUp) || input.GamepadPressed(0, golib.GamepadA)
	a.rotateCCW = input.KeyPressed(golib.KeyZ) || input.GamepadPressed(0, golib.GamepadB)
	a.softDrop = input.KeyDown(golib.KeyDown) || input.KeyDown(golib.KeyS) || input.GamepadDown(0, golib.GamepadDown)
	a.hardDrop = input.KeyPressed(golib.KeySpace) || input.GamepadPressed(0, golib.GamepadX)
	a.hold = input.KeyPressed(golib.KeyC) || input.KeyPressed(golib.KeyLeftShift) ||
		input.KeyPressed(golib.KeyRightShift) || input.GamepadPressed(0, golib.GamepadLeftBumper)
	return a
}

// playFellSound plays a click when the piece slid or turned. It compares the
// piece before and after the step, so it plays only when something moved.
func (s *playScene) playFellSound(before activePiece) {
	if s.world.piece.kind != before.kind {
		return // the piece locked or was swapped: a clear or a hold sound covers it
	}
	if s.world.piece.rotation != before.rotation {
		rotateSound.Play()
	} else if s.world.piece.x != before.x {
		moveSound.Play()
	}
}

// handleEvents reacts to everything the world reported this update: a sound and,
// for a clear, particles, a shake and a toast.
func (s *playScene) handleEvents() {
	for _, e := range s.world.TakeEvents() {
		switch e.kind {
		case eventLock:
			lockSound.PlayWith(1, golib.RandomFloat(0.95, 1.08))
			s.addShake(2.5, 0.1)
		case eventHardDrop:
			lockSound.PlayWith(1, golib.RandomFloat(0.8, 0.9))
			s.addShake(3+float32(e.value)*0.6, 0.16)
		case eventHold:
			holdSound.Play()
		case eventClear:
			s.spawnClearParticles(e.rows)
			s.addShake(5+float32(e.value)*2, 0.22)
			s.toast = clearName(e.value)
			s.toastTimer = 1.1
			if e.value >= 4 {
				tetrisSound.Play()
				s.flash = 0.85
			} else {
				clearSound.PlayWith(1, 0.9+float32(e.value)*0.1)
				s.flash = 0.35
			}
		case eventLevelUp:
			levelUpSound.Play()
			s.toast = fmt.Sprintf("LEVEL %d", e.value)
			s.toastTimer = 1.4
		case eventGameOver:
			gameOverSound.Play()
			s.addShake(9, 0.5)
		}
	}
}

// clearName is what a line clear is called, as the toast shows it.
func clearName(lines int) string {
	switch lines {
	case 1:
		return "SINGLE"
	case 2:
		return "DOUBLE"
	case 3:
		return "TRIPLE"
	default:
		return "TETRIS!"
	}
}

// addShake starts a screen shake, keeping the stronger of the two when one is
// already running.
func (s *playScene) addShake(strength, duration float32) {
	if strength >= s.shakeStrength || s.shakeTime <= 0 {
		s.shakeStrength = strength
		s.shakeDuration = duration
		s.shakeTime = duration
	}
}

// spawnClearParticles throws sparks across every row that cleared.
func (s *playScene) spawnClearParticles(rows []int) {
	for _, row := range rows {
		y := float32(boardY + row*boardCell + boardCell/2)
		for x := 0; x < boardColumns; x++ {
			centerX := float32(boardX + x*boardCell + boardCell/2)
			for range 3 {
				color := pieceColors[golib.RandomInt(0, int(pieceKinds)-1)]
				s.particles = append(s.particles, particle{
					x:     centerX + golib.RandomFloat(-boardCell/2, boardCell/2),
					y:     y + golib.RandomFloat(-boardCell/2, boardCell/2),
					vx:    golib.RandomFloat(-190, 190),
					vy:    golib.RandomFloat(-320, -60),
					life:  golib.RandomFloat(0.45, 0.9),
					max:   0.9,
					color: color,
				})
			}
		}
	}
}

// updateEffects moves the particles and counts down the shake, the flash and
// the toast.
func (s *playScene) updateEffects(dt float32) {
	alive := s.particles[:0]
	for _, p := range s.particles {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.vy += 1100 * dt // gravity pulls the sparks down
		p.x += p.vx * dt
		p.y += p.vy * dt
		alive = append(alive, p)
	}
	s.particles = alive

	if s.shakeTime > 0 {
		s.shakeTime -= dt
		fade := max(s.shakeTime/s.shakeDuration, 0)
		s.shakeX = golib.RandomFloat(-s.shakeStrength, s.shakeStrength) * fade
		s.shakeY = golib.RandomFloat(-s.shakeStrength, s.shakeStrength) * fade
	} else {
		s.shakeX, s.shakeY = 0, 0
	}

	s.flash = max(s.flash-2.6*dt, 0)
	s.toastTimer = max(s.toastTimer-dt, 0)
}

// Draw draws the game: the board, the panels and the effects. It reads the
// state and never changes it.
func (s *playScene) Draw(screen *golib.Screen) {
	drawPlay(screen, s)
}
