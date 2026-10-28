package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Distances are in
// meters and times in seconds, as in golib's 3D: Y is up, and the level runs
// towards +Z.
const (
	runSpeed     = 5.5 // meters per second, at full tilt
	groundAccel  = 45  // meters per second per second: how fast running starts and stops
	airAccel     = 22  // the same in the air: less grip, but still steerable
	gravity      = 30  // meters per second per second
	jumpSpeed    = 10  // meters per second up: a full jump rises jumpSpeed²/2g, about 1.67 m
	jumpCutSpeed = 4.5 // meters per second: letting go of jump while rising slows the rise to this
	maxFallSpeed = 22  // meters per second
	coyoteTime   = 0.1 // seconds after running off an edge in which a jump still works
	jumpBuffer   = 0.12
	turnSpeed    = 720 // degrees per second the character turns to face where it runs

	playerRadius = 0.28 // meters: half the width of the character's box
	playerHeight = 0.85 // meters

	coinReach       = 0.6 // meters from the character's middle to a coin's to pick it up
	checkpointReach = 1.0 // meters, on the ground, from a checkpoint
	goalReach       = 0.9 // meters, on the ground, from the flag
	startLives      = 3
)

// player is the character.
type player struct {
	position golib.Vector3 // its feet
	velocity golib.Vector3 // meters per second
	yaw      float32       // where it faces, in degrees, as golib.Rotation's Yaw
	onGround bool
	airTime  float32     // seconds since it last stood on something
	buffered golib.Timer // a jump pressed just before landing, kept for a moment
	jumping  bool        // rising from a jump the player may still cut short
	groundY  float32     // the height it last stood at
}

// intent is what the player wants in one update, from the keys or the
// gamepad; the rules never see the input itself.
type intent struct {
	move     golib.Vector3 // a direction on the ground, at most length 1
	jump     bool          // jump was pressed in this update
	jumpHeld bool          // jump is held down
}

// events is what happened in one step, for the scene's sounds and effects.
type events struct {
	jumped     bool
	landed     float32 // the speed it landed at, in meters per second; 0 if it didn't land
	coins      int     // coins picked up
	checkpoint bool    // a new checkpoint was reached
	fell       bool    // it dropped below the level
	lostLife   bool
	gameOver   bool // the last life went
	won        bool // it reached the flag
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so world_test.go can play it directly.
type world struct {
	level      *level
	player     player
	taken      []bool // which of the level's coins have been picked up
	collected  int
	reached    []bool // which checkpoints have been reached
	respawnAt  golib.Vector3
	respawnYaw float32
	lives      int
	time       float32 // seconds since the start, while playing
	won, over  bool
}

// newWorld returns the world at the start of a game, on the level l.
func newWorld(l *level) *world {
	w := &world{
		level:      l,
		taken:      make([]bool, len(l.coins)),
		reached:    make([]bool, len(l.checkpoints)),
		respawnAt:  l.start,
		respawnYaw: l.startYaw,
		lives:      startLives,
	}
	w.respawn()
	return w
}

// respawn puts the character back where it starts again, at rest.
func (w *world) respawn() {
	w.player = player{position: w.respawnAt, yaw: w.respawnYaw, onGround: true, groundY: w.respawnAt.Y}
}

// playing says whether the game is still going: not won, not over.
func (w *world) playing() bool {
	return !w.won && !w.over
}

// falling says whether the character has dropped well below where it last
// stood, too far to land anywhere: the camera stops following it then.
func (w *world) falling() bool {
	return !w.player.onGround && w.player.position.Y < min(w.player.groundY, w.level.lowestTop)-2
}

// middle is the middle of the character's body, for picking things up.
func (p *player) middle() golib.Vector3 {
	return p.position.Add(golib.Vector3{Y: playerHeight / 2})
}

// step advances the world by dt seconds with what the player wants.
func (w *world) step(want intent, dt float32) events {
	var happened events
	if !w.playing() {
		return happened
	}
	w.time += dt
	p := &w.player

	// Running: the velocity on the ground moves towards where the player
	// wants to go, with less grip in the air.
	move := golib.Vector3{X: want.move.X, Z: want.move.Z}.ClampLength(1)
	target := move.Scale(runSpeed)
	accel := float32(groundAccel)
	if !p.onGround {
		accel = airAccel
	}
	flat := golib.Vector3{X: p.velocity.X, Z: p.velocity.Z}.MoveTowards(target, accel*dt)
	p.velocity.X, p.velocity.Z = flat.X, flat.Z
	if move.Length() > 0.1 {
		p.yaw = turnTowards(p.yaw, golib.RotationTowards(move).Yaw, turnSpeed*dt)
	}

	// Jumping, from the ground or just after leaving it, and a press just
	// before landing counts when it lands.
	if want.jump {
		p.buffered.Start(jumpBuffer)
	}
	if p.buffered.Running() && (p.onGround || p.airTime < coyoteTime) && !p.jumping {
		p.velocity.Y = jumpSpeed
		p.onGround, p.jumping = false, true
		p.airTime = coyoteTime // no second jump from the coyote time
		p.buffered.Stop()
		happened.jumped = true
	}
	p.buffered.Tick(dt)
	if p.jumping && !want.jumpHeld && p.velocity.Y > jumpCutSpeed {
		p.velocity.Y = jumpCutSpeed
	}
	if p.velocity.Y <= 0 {
		p.jumping = false
	}
	p.velocity.Y = max(p.velocity.Y-gravity*dt, -maxFallSpeed)

	// Moving, one axis at a time, out of any block it runs into.
	fallSpeed := -p.velocity.Y
	w.moveAcross(dt)
	wasOnGround := p.onGround
	w.moveUpDown(dt)
	if p.onGround {
		p.airTime = 0
		p.groundY = p.position.Y
		if !wasOnGround {
			happened.landed = fallSpeed
		}
	} else {
		p.airTime += dt
	}

	w.pickUp(&happened)
	if p.position.Y < w.level.killY {
		happened.fell, happened.lostLife = true, true
		w.lives--
		if w.lives <= 0 {
			w.lives = 0
			w.over = true
			happened.gameOver = true
		} else {
			w.respawn()
		}
	}
	return happened
}

// moveAcross moves the character along X, then Z, and stops it at the side
// of any block it runs into.
func (w *world) moveAcross(dt float32) {
	p := &w.player
	p.position.X += p.velocity.X * dt
	for _, b := range w.level.blocks {
		if w.overlaps(b) {
			if p.velocity.X > 0 {
				p.position.X = b.lowest.X - playerRadius
			} else if p.velocity.X < 0 {
				p.position.X = b.upmost.X + playerRadius
			}
			p.velocity.X = 0
		}
	}
	p.position.Z += p.velocity.Z * dt
	for _, b := range w.level.blocks {
		if w.overlaps(b) {
			if p.velocity.Z > 0 {
				p.position.Z = b.lowest.Z - playerRadius
			} else if p.velocity.Z < 0 {
				p.position.Z = b.upmost.Z + playerRadius
			}
			p.velocity.Z = 0
		}
	}
}

// moveUpDown moves the character along Y: it lands on the top of a block it
// falls into, and bumps its head on the bottom of one it jumps into.
func (w *world) moveUpDown(dt float32) {
	p := &w.player
	p.position.Y += p.velocity.Y * dt
	p.onGround = false
	for _, b := range w.level.blocks {
		if !w.overlaps(b) {
			continue
		}
		if p.velocity.Y <= 0 {
			p.position.Y = b.upmost.Y
			p.onGround = true
		} else {
			p.position.Y = b.lowest.Y - playerHeight
			p.jumping = false
		}
		p.velocity.Y = 0
	}
}

// overlaps says whether the character's box and b share some space. Boxes
// that only touch, such as feet on a block's top, don't overlap.
func (w *world) overlaps(b box) bool {
	p := w.player.position
	return p.X+playerRadius > b.lowest.X && p.X-playerRadius < b.upmost.X &&
		p.Z+playerRadius > b.lowest.Z && p.Z-playerRadius < b.upmost.Z &&
		p.Y+playerHeight > b.lowest.Y && p.Y < b.upmost.Y
}

// pickUp collects the coins the character touches, reaches checkpoints, and
// wins at the flag.
func (w *world) pickUp(happened *events) {
	p := &w.player
	middle := p.middle()
	for i, c := range w.level.coins {
		if !w.taken[i] && middle.Distance(c) < coinReach {
			w.taken[i] = true
			w.collected++
			happened.coins++
		}
	}
	for i, c := range w.level.checkpoints {
		if !w.reached[i] && near(p.position, c, checkpointReach) {
			w.reached[i] = true
			w.respawnAt, w.respawnYaw = c, p.yaw
			happened.checkpoint = true
		}
	}
	if near(p.position, w.level.goal, goalReach) {
		w.won = true
		happened.won = true
	}
}

// near says whether the feet at are within reach of a spot on the ground,
// measured across the ground, and not much above or below it.
func near(at, spot golib.Vector3, reach float32) bool {
	across := golib.Vector3{X: at.X - spot.X, Z: at.Z - spot.Z}.Length()
	return across < reach && at.Y > spot.Y-0.5 && at.Y < spot.Y+2
}

// turnTowards turns the yaw from towards to by at most maxTurn degrees, the
// shortest way round.
func turnTowards(from, to, maxTurn float32) float32 {
	turn := angleBetween(from, to)
	if float32(math.Abs(float64(turn))) <= maxTurn {
		return to
	}
	if turn < 0 {
		return from - maxTurn
	}
	return from + maxTurn
}

// angleBetween returns the shortest turn from one yaw to another, from -180 to
// 180 degrees.
func angleBetween(from, to float32) float32 {
	turn := float32(math.Mod(float64(to-from), 360))
	if turn > 180 {
		turn -= 360
	}
	if turn < -180 {
		turn += 360
	}
	return turn
}
