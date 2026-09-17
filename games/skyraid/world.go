package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels. Enemy kinds are in
// enemies.go and waves in waves.go.
const (
	// The arena, in pixels. The screen shows 1280 by 720 of it.
	worldWidth  = 3600
	worldHeight = 2400

	shipSpeed        = 340  // pixels per second, at full tilt
	shipResponse     = 9    // per second: how quickly the ship reaches the speed it is steered to
	shipRadius       = 15   // pixels, for hits
	shipHull         = 5    // hits the ship takes before the game is over
	hurtTime         = 1.2  // seconds the ship can't be hurt after a hit
	fireCooldown     = 0.11 // seconds between shots while fire is held
	shotSpeed        = 950  // pixels per second
	shotLifetime     = 0.85 // seconds
	shotRadius       = 4    // pixels
	shotSpread       = 0.03 // radians a shot may stray from the aim, either way
	dashSpeed        = 1000 // pixels per second, while dashing
	dashTime         = 0.18 // seconds a dash lasts, untouchable
	dashCooldown     = 1.1  // seconds from the start of a dash to the next one
	repairRadius     = 16   // pixels
	repairLifetime   = 10   // seconds a repair kit stays
	repairBlinkAfter = 7    // seconds after which a repair kit blinks, about to go
	pickupReach      = 10   // pixels added to both radii when picking a repair kit up

	traumaDecay     = 1.8  // shake lost per second; shake goes from 0 to 1
	traumaHurt      = 0.55 // shake added when the ship is hit
	traumaExplosion = 0.18 // shake added by an enemy's explosion, times its size
	overDelay       = 1.6  // seconds from the ship's end to the game over screen
	particleDrag    = 2.5  // per second
	blastTime       = 0.35 // seconds an explosion's flash and ring last
)

// ship is the player's ship.
type ship struct {
	x, y         float32 // center, in the arena's pixels
	vx, vy       float32 // pixels per second
	angle        float32 // radians where it aims; 0 is right, growing clockwise
	hull         int
	alive        bool
	cooldown     float32 // seconds until it can fire again
	hurt         float32 // seconds left of not being hurt after a hit
	dashLeft     float32 // seconds left of the current dash; 0 when not dashing
	dashCharge   float32 // seconds until the next dash, counting down from dashCooldown
	dashX, dashY float32 // unit direction of the current dash
	thrusting    bool    // steered this update, for the engine flame
	flash        float32 // seconds left of the muzzle flash
	flameFlicker float32 // seconds of thrust, to animate the flame
	hitThisWave  bool    // hurt since the wave began, which loses the flawless bonus
}

// untouchable reports whether hits pass through the ship right now.
func (s *ship) untouchable() bool {
	return s.hurt > 0 || s.dashLeft > 0
}

// shot is a bullet, the ship's or an enemy's.
type shot struct {
	x, y   float32
	vx, vy float32
	radius float32
	life   float32 // seconds left
}

// particle is a spark, a trail dot or an explosion's blast. It only looks.
type particle struct {
	x, y     float32
	vx, vy   float32
	size     float32 // radius in pixels at the start; a dot shrinks, a blast grows
	life     float32 // seconds left
	lifetime float32 // seconds it lived at the start
	color    golib.Color
	blast    bool // a flash and a ring that grows, instead of a dot
}

// repairKit restores one hull point when the ship flies over it.
type repairKit struct {
	x, y float32
	life float32 // seconds left
}

// controls is what the player asks for in one update. play.go reads them from
// the keyboard, the mouse and gamepad 0.
type controls struct {
	moveX, moveY float32 // from -1 to 1 each; longer than 1 is cut to 1
	aimX, aimY   float32 // where to aim, as a direction of any length; 0, 0 keeps the aim
	fire         bool    // fire while held
	dash         bool    // start a dash, in this update only
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so the tests can play it directly. It does play the
// sounds of sounds.go, which stay silent in tests and golib shot.
type world struct {
	ship       ship
	enemies    []enemy
	warps      []warp
	shots      []shot // the ship's
	enemyShots []shot
	particles  []particle
	repairs    []repairKit
	score      int
	wave       int     // the wave being fought, or just cleared; 0 before the first
	queue      []int   // enemy kinds of this wave still to warp in
	spawnTimer float32 // seconds until the next group warps in
	breakLeft  float32 // seconds left of the break before the next wave; 0 while fighting
	lastBonus  int     // flawless bonus of the wave just cleared, for the banner
	kills      int
	trauma     float32 // screen shake, from 0 to 1
	time       float32 // seconds since the world began
	over       bool    // true once the ship is gone
	overTime   float32 // seconds since the ship was destroyed
}

// newWorld returns a new game: the ship in the middle of the arena, and the
// first wave about to start.
func newWorld() world {
	w := world{breakLeft: firstBreak}
	w.ship = ship{x: worldWidth / 2, y: worldHeight / 2, angle: -math.Pi / 2, hull: shipHull, alive: true}
	return w
}

// step advances the world by dt seconds with the player's controls.
func (w *world) step(c controls, dt float32) {
	w.time += dt
	w.trauma = max(0, w.trauma-traumaDecay*dt)
	if w.over {
		w.overTime += dt
	}
	w.moveShip(c, dt)
	if c.fire {
		w.fire()
	}
	w.updateWaves(dt)
	w.moveEnemies(dt)
	w.moveShots(dt)
	w.moveParticles(dt)
	w.updateRepairs(dt)
	w.hitEnemies()
	w.hitShip()
}

// moveShip steers, dashes and cools down the ship.
func (w *world) moveShip(c controls, dt float32) {
	s := &w.ship
	if !s.alive {
		return
	}
	s.cooldown = max(0, s.cooldown-dt)
	s.hurt = max(0, s.hurt-dt)
	s.flash = max(0, s.flash-dt)
	s.dashCharge = max(0, s.dashCharge-dt)

	moveX, moveY := limit(c.moveX, c.moveY, 1)
	s.thrusting = moveX != 0 || moveY != 0
	if s.thrusting {
		s.flameFlicker += dt
	}

	switch {
	case c.aimX != 0 || c.aimY != 0:
		s.angle = angleOf(c.aimX, c.aimY)
	case s.thrusting && !c.fire:
		// With nothing to aim with, the ship looks where it flies, so Space
		// alone fires straight ahead.
		s.angle = angleOf(moveX, moveY)
	}

	if c.dash && s.dashCharge == 0 {
		dx, dy := normalize(moveX, moveY)
		if dx == 0 && dy == 0 {
			dx, dy = direction(s.angle)
		}
		s.dashX, s.dashY = dx, dy
		s.dashLeft = dashTime
		s.dashCharge = dashCooldown
		dashSound.Play()
	}

	if s.dashLeft > 0 {
		s.dashLeft = max(0, s.dashLeft-dt)
		s.vx, s.vy = s.dashX*dashSpeed, s.dashY*dashSpeed
		w.trail(s.x, s.y, dashTrailColor)
		if s.dashLeft == 0 {
			// Leave the dash at cruising speed, not at dash speed.
			s.vx, s.vy = s.dashX*shipSpeed, s.dashY*shipSpeed
		}
	} else {
		s.vx = approach(s.vx, moveX*shipSpeed, shipResponse, dt)
		s.vy = approach(s.vy, moveY*shipSpeed, shipResponse, dt)
	}

	s.x = clamp(s.x+s.vx*dt, shipRadius, worldWidth-shipRadius)
	s.y = clamp(s.y+s.vy*dt, shipRadius, worldHeight-shipRadius)
	if s.thrusting && int(w.time*60)%3 == 0 {
		// Exhaust puffs, behind the ship as it flies.
		mx, my := normalize(moveX, moveY)
		w.particles = append(w.particles, particle{
			x: s.x - mx*14, y: s.y - my*14,
			vx: -mx*80 + golib.RandomFloat(-20, 20), vy: -my*80 + golib.RandomFloat(-20, 20),
			size: golib.RandomFloat(2, 3.5), life: 0.35, lifetime: 0.35, color: flameColor,
		})
	}
}

// fire shoots from the ship's nose where it aims, unless the ship is gone or
// still cooling down.
func (w *world) fire() {
	s := &w.ship
	if !s.alive || s.cooldown > 0 {
		return
	}
	angle := s.angle + golib.RandomFloat(-shotSpread, shotSpread)
	dx, dy := direction(angle)
	w.shots = append(w.shots, shot{
		x: s.x + dx*shipNose, y: s.y + dy*shipNose,
		vx: dx * shotSpeed, vy: dy * shotSpeed,
		radius: shotRadius, life: shotLifetime,
	})
	s.cooldown = fireCooldown
	s.flash = 0.05
	shotSound.Play()
}

// moveShots flies every bullet, and drops those that ran out of time or left
// the arena.
func (w *world) moveShots(dt float32) {
	w.shots = flyShots(w.shots, dt)
	w.enemyShots = flyShots(w.enemyShots, dt)
}

func flyShots(shots []shot, dt float32) []shot {
	kept := shots[:0]
	for _, b := range shots {
		b.life -= dt
		b.x += b.vx * dt
		b.y += b.vy * dt
		if b.life <= 0 || b.x < 0 || b.y < 0 || b.x > worldWidth || b.y > worldHeight {
			continue
		}
		kept = append(kept, b)
	}
	return kept
}

func (w *world) moveParticles(dt float32) {
	kept := w.particles[:0]
	keep := max(0, 1-particleDrag*dt)
	for _, p := range w.particles {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		p.vx *= keep
		p.vy *= keep
		kept = append(kept, p)
	}
	w.particles = kept
}

// updateRepairs ages the repair kits and lets the ship pick them up.
func (w *world) updateRepairs(dt float32) {
	s := &w.ship
	kept := w.repairs[:0]
	for _, r := range w.repairs {
		r.life -= dt
		if r.life <= 0 {
			continue
		}
		if s.alive && circlesTouch(s.x, s.y, shipRadius+pickupReach, r.x, r.y, repairRadius) {
			s.hull = min(shipHull, s.hull+1)
			w.burst(r.x, r.y, 14, 160, repairColor)
			repairSound.Play()
			continue
		}
		kept = append(kept, r)
	}
	w.repairs = kept
}

// hitShip hurts the ship with every enemy bullet or enemy that touches it.
func (w *world) hitShip() {
	s := &w.ship
	if !s.alive {
		return
	}
	kept := w.enemyShots[:0]
	for _, b := range w.enemyShots {
		if !s.untouchable() && circlesTouch(s.x, s.y, shipRadius, b.x, b.y, b.radius) {
			w.hurtShip(b.vx, b.vy)
			continue
		}
		kept = append(kept, b)
	}
	w.enemyShots = kept

	for i := 0; i < len(w.enemies); i++ {
		e := &w.enemies[i]
		kind := enemyKinds[e.kind]
		if !circlesTouch(s.x, s.y, shipRadius, e.x, e.y, kind.radius) {
			continue
		}
		if kind.rams {
			// A scout that rams the ship is destroyed, dash or not.
			w.destroyEnemy(i, false)
			i--
		} else {
			// Bigger ships push the ship away.
			dx, dy := normalize(s.x-e.x, s.y-e.y)
			s.vx, s.vy = dx*shipSpeed, dy*shipSpeed
		}
		if !s.untouchable() {
			w.hurtShip(s.x-e.x, s.y-e.y)
		}
		if !s.alive {
			return
		}
	}
}

// hurtShip takes a hull point from the ship, hit from the direction dx, dy.
func (w *world) hurtShip(dx, dy float32) {
	s := &w.ship
	s.hull--
	s.hitThisWave = true
	s.hurt = hurtTime
	w.shake(traumaHurt)
	nx, ny := normalize(dx, dy)
	s.vx += nx * 220
	s.vy += ny * 220
	w.burst(s.x, s.y, 12, 200, shipColor)
	w.blast(s.x, s.y, shipRadius, warnColor)
	if s.hull > 0 {
		hurtSound.Play()
		return
	}
	s.alive = false
	w.over = true
	w.burst(s.x, s.y, 60, 420, flameColor)
	w.burst(s.x, s.y, 30, 260, shipColor)
	w.blast(s.x, s.y, 50, shotColor)
	w.shake(1)
	shipLostSound.Play()
}

// shake adds screen shake, up to the most there is.
func (w *world) shake(amount float32) {
	w.trauma = min(1, w.trauma+amount)
}

// burst throws count sparks of a color out from x, y, up to speed pixels per
// second.
func (w *world) burst(x, y float32, count int, speed float32, color golib.Color) {
	for range count {
		dx, dy := direction(golib.RandomFloat(0, 2*math.Pi))
		v := golib.RandomFloat(0.2, 1) * speed
		life := golib.RandomFloat(0.3, 0.8)
		w.particles = append(w.particles, particle{
			x: x, y: y, vx: dx * v, vy: dy * v,
			size: golib.RandomFloat(1.5, 4), life: life, lifetime: life, color: color,
		})
	}
}

// blast adds an explosion's flash and ring, radius pixels at the start.
func (w *world) blast(x, y, radius float32, color golib.Color) {
	w.particles = append(w.particles, particle{x: x, y: y, size: radius, life: blastTime, lifetime: blastTime, color: color, blast: true})
}

// trail leaves one fading dot at x, y.
func (w *world) trail(x, y float32, color golib.Color) {
	w.particles = append(w.particles, particle{x: x, y: y, size: 9, life: 0.25, lifetime: 0.25, color: color})
}
