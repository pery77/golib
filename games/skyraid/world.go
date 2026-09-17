package main

import "golib"

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
	shotSpread       = 1.7  // degrees a shot may stray from the aim, either way
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

// arena is the whole playing field, in pixels. It is the camera's Bounds, and
// bullets that leave it vanish.
var arena = golib.Rectangle{Width: worldWidth, Height: worldHeight}

// insideArena returns a point kept at least margin pixels inside the arena.
func insideArena(at golib.Vector2, margin float32) golib.Vector2 {
	return golib.Vector2{
		X: clamp(at.X, margin, worldWidth-margin),
		Y: clamp(at.Y, margin, worldHeight-margin),
	}
}

// ship is the player's ship.
type ship struct {
	position     golib.Vector2 // center, in the arena's pixels
	velocity     golib.Vector2 // pixels per second
	angle        float32       // degrees where it aims; 0 is right, growing clockwise
	hull         int
	alive        bool
	cooldown     float32       // seconds until it can fire again
	hurt         float32       // seconds left of not being hurt after a hit
	dashLeft     float32       // seconds left of the current dash; 0 when not dashing
	dashCharge   float32       // seconds until the next dash, counting down from dashCooldown
	dashWay      golib.Vector2 // unit direction of the current dash
	thrusting    bool          // steered this update, for the engine flame
	flash        float32       // seconds left of the muzzle flash
	flameFlicker float32       // seconds of thrust, to animate the flame
	hitThisWave  bool          // hurt since the wave began, which loses the flawless bonus
}

// untouchable reports whether hits pass through the ship right now.
func (s *ship) untouchable() bool {
	return s.hurt > 0 || s.dashLeft > 0
}

// shot is a bullet, the ship's or an enemy's.
type shot struct {
	position golib.Vector2
	velocity golib.Vector2
	radius   float32
	life     float32 // seconds left
}

// particle is a spark, a trail dot or an explosion's blast. It only looks.
type particle struct {
	position golib.Vector2
	velocity golib.Vector2
	size     float32 // radius in pixels at the start; a dot shrinks, a blast grows
	life     float32 // seconds left
	lifetime float32 // seconds it lived at the start
	color    golib.Color
	blast    bool // a flash and a ring that grows, instead of a dot
}

// repairKit restores one hull point when the ship flies over it.
type repairKit struct {
	position golib.Vector2
	life     float32 // seconds left
}

// controls is what the player asks for in one update. play.go reads them from
// the keyboard, the mouse and gamepad 0.
type controls struct {
	move golib.Vector2 // each part from -1 to 1; longer than 1 is cut to 1
	aim  golib.Vector2 // where to aim, of any length; the zero vector keeps the aim
	fire bool          // fire while held
	dash bool          // start a dash, in this update only
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
	w.ship = ship{position: arena.Center(), angle: -90, hull: shipHull, alive: true}
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

	move := c.move.ClampLength(1)
	s.thrusting = move != (golib.Vector2{})
	if s.thrusting {
		s.flameFlicker += dt
	}

	switch {
	case c.aim != (golib.Vector2{}):
		s.angle = c.aim.Angle()
	case s.thrusting && !c.fire:
		// With nothing to aim with, the ship looks where it flies, so Space
		// alone fires straight ahead.
		s.angle = move.Angle()
	}

	if c.dash && s.dashCharge == 0 {
		way := move.Normalize()
		if way == (golib.Vector2{}) {
			way = golib.Vector2FromAngle(s.angle)
		}
		s.dashWay = way
		s.dashLeft = dashTime
		s.dashCharge = dashCooldown
		dashSound.Play()
	}

	if s.dashLeft > 0 {
		s.dashLeft = max(0, s.dashLeft-dt)
		s.velocity = s.dashWay.Scale(dashSpeed)
		w.trail(s.position, dashTrailColor)
		if s.dashLeft == 0 {
			// Leave the dash at cruising speed, not at dash speed.
			s.velocity = s.dashWay.Scale(shipSpeed)
		}
	} else {
		s.velocity = s.velocity.Lerp(move.Scale(shipSpeed), min(1, shipResponse*dt))
	}

	s.position = insideArena(s.position.Add(s.velocity.Scale(dt)), shipRadius)
	if s.thrusting && int(w.time*60)%3 == 0 {
		// Exhaust puffs, behind the ship as it flies.
		back := move.Normalize().Scale(-1)
		w.particles = append(w.particles, particle{
			position: s.position.Add(back.Scale(14)),
			velocity: back.Scale(80).Add(golib.Vector2{X: golib.RandomFloat(-20, 20), Y: golib.RandomFloat(-20, 20)}),
			size:     golib.RandomFloat(2, 3.5), life: 0.35, lifetime: 0.35, color: flameColor,
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
	way := golib.Vector2FromAngle(s.angle + golib.RandomFloat(-shotSpread, shotSpread))
	w.shots = append(w.shots, shot{
		position: s.position.Add(way.Scale(shipNose)),
		velocity: way.Scale(shotSpeed),
		radius:   shotRadius,
		life:     shotLifetime,
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
		b.position = b.position.Add(b.velocity.Scale(dt))
		if b.life <= 0 || !arena.Contains(b.position.X, b.position.Y) {
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
		p.position = p.position.Add(p.velocity.Scale(dt))
		p.velocity = p.velocity.Scale(keep)
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
		if s.alive && s.position.Distance(r.position) < shipRadius+pickupReach+repairRadius {
			s.hull = min(shipHull, s.hull+1)
			w.burst(r.position, 14, 160, repairColor)
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
		if !s.untouchable() && s.position.Distance(b.position) < shipRadius+b.radius {
			w.hurtShip(b.velocity)
			continue
		}
		kept = append(kept, b)
	}
	w.enemyShots = kept

	for i := 0; i < len(w.enemies); i++ {
		e := &w.enemies[i]
		kind := enemyKinds[e.kind]
		if s.position.Distance(e.position) >= shipRadius+kind.radius {
			continue
		}
		away := s.position.Sub(e.position)
		if kind.rams {
			// A scout that rams the ship is destroyed, dash or not.
			w.destroyEnemy(i, false)
			i--
		} else {
			// Bigger ships push the ship away.
			s.velocity = away.Normalize().Scale(shipSpeed)
		}
		if !s.untouchable() {
			w.hurtShip(away)
		}
		if !s.alive {
			return
		}
	}
}

// hurtShip takes a hull point from the ship, knocked the way push points.
func (w *world) hurtShip(push golib.Vector2) {
	s := &w.ship
	s.hull--
	s.hitThisWave = true
	s.hurt = hurtTime
	w.shake(traumaHurt)
	s.velocity = s.velocity.Add(push.Normalize().Scale(220))
	w.burst(s.position, 12, 200, shipColor)
	w.blast(s.position, shipRadius, warnColor)
	if s.hull > 0 {
		hurtSound.Play()
		return
	}
	s.alive = false
	w.over = true
	w.burst(s.position, 60, 420, flameColor)
	w.burst(s.position, 30, 260, shipColor)
	w.blast(s.position, 50, shotColor)
	w.shake(1)
	shipLostSound.Play()
}

// shake adds screen shake, up to the most there is.
func (w *world) shake(amount float32) {
	w.trauma = min(1, w.trauma+amount)
}

// burst throws count sparks of a color out from at, up to speed pixels per
// second.
func (w *world) burst(at golib.Vector2, count int, speed float32, color golib.Color) {
	for range count {
		way := golib.Vector2FromAngle(golib.RandomFloat(0, 360))
		v := golib.RandomFloat(0.2, 1) * speed
		life := golib.RandomFloat(0.3, 0.8)
		w.particles = append(w.particles, particle{
			position: at, velocity: way.Scale(v),
			size: golib.RandomFloat(1.5, 4), life: life, lifetime: life, color: color,
		})
	}
}

// blast adds an explosion's flash and ring, radius pixels at the start.
func (w *world) blast(at golib.Vector2, radius float32, color golib.Color) {
	w.particles = append(w.particles, particle{position: at, size: radius, life: blastTime, lifetime: blastTime, color: color, blast: true})
}

// trail leaves one fading dot at a point.
func (w *world) trail(at golib.Vector2, color golib.Color) {
	w.particles = append(w.particles, particle{position: at, size: 9, life: 0.25, lifetime: 0.25, color: color})
}
