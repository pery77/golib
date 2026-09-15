package main

import (
	"math"

	"golib"
)

// Tuning: the numbers that define how the game feels.
const (
	turnSpeed        = 4.2  // radians per second
	thrustPower      = 480  // pixels per second gained every second of thrust
	maxShipSpeed     = 460  // pixels per second
	shipDrag         = 0.5  // share of the ship's speed lost every second while coasting
	shipRadius       = 14   // pixels, for crashes
	shipNose         = 18   // pixels from the ship's center to its nose, where bullets start
	bulletSpeed      = 680  // pixels per second, on top of the ship's speed
	bulletLifetime   = 0.85 // seconds
	fireCooldown     = 0.17 // seconds between shots while fire is held
	maxBullets       = 8    // bullets in flight at once
	respawnDelay     = 1.5  // seconds from a crash to the next ship
	invulnerableTime = 2.5  // seconds a new ship can't crash
	startLives       = 3
	firstWaveRocks   = 4  // big rocks in the first wave; every wave adds one
	rockSpeedMin     = 35 // pixels per second, for big rocks; smaller rocks are faster
	rockSpeedMax     = 95
	rockCorners      = 11  // corners in a rock's outline
	particleLifetime = 0.7 // seconds, at most
)

// World size in pixels. It is also the size of the screen (see main.go).
const (
	worldWidth  = 1280
	worldHeight = 720
)

// Rock sizes. A hit splits a rock into two of the next smaller size.
const (
	smallRock = iota + 1
	mediumRock
	bigRock
)

var (
	rockRadius  = [...]float32{smallRock: 14, mediumRock: 26, bigRock: 48} // pixels
	rockPoints  = [...]int{smallRock: 100, mediumRock: 50, bigRock: 20}
	rockSpeedUp = [...]float32{smallRock: 2, mediumRock: 1.5, bigRock: 1} // speed compared with a big rock
)

// ship is the player's ship.
type ship struct {
	x, y         float32 // center, in pixels
	vx, vy       float32 // pixels per second
	angle        float32 // radians, clockwise from pointing up
	alive        bool
	thrusting    bool    // for drawing the flame
	cooldown     float32 // seconds until the ship can fire again
	invulnerable float32 // seconds until the ship can crash
}

// facing returns the unit vector the ship points along.
func (s ship) facing() (float32, float32) {
	return float32(math.Sin(float64(s.angle))), -float32(math.Cos(float64(s.angle)))
}

// rock drifts and spins until a bullet or the ship hits it.
type rock struct {
	x, y   float32 // center, in pixels
	vx, vy float32 // pixels per second
	size   int     // smallRock, mediumRock or bigRock
	angle  float32 // radians, for drawing
	spin   float32 // radians per second
	shape  [rockCorners]float32
}

func (r rock) radius() float32 {
	return rockRadius[r.size]
}

// bullet flies straight until it hits a rock or its time runs out.
type bullet struct {
	x, y   float32
	vx, vy float32
	life   float32 // seconds left
}

// particle is a spark from an explosion.
type particle struct {
	x, y   float32
	vx, vy float32
	life   float32 // seconds left
}

// controls is what the player asks for in one update. playScene reads them
// from the keyboard and the gamepad.
type controls struct {
	turn   float32 // from -1 (left) to 1 (right)
	thrust bool
	fire   bool
}

// world is the whole game state and its rules. It knows nothing about the
// input or the screen, so world_test.go can play it directly.
type world struct {
	ship      ship
	rocks     []rock
	bullets   []bullet
	particles []particle
	score     int
	lives     int
	wave      int
	respawn   float32 // seconds until the next ship, while there is none
	over      bool    // true once the last ship has crashed
	time      float32 // seconds since the world began, for animations
}

// newWorld returns a new game: a ship in the middle and the first wave.
func newWorld() world {
	w := world{lives: startLives}
	w.spawnShip()
	w.nextWave()
	return w
}

// step advances the world by dt seconds with the player's controls. Once the
// game is over, the rocks and sparks keep moving.
func (w *world) step(c controls, dt float32) {
	w.time += dt
	w.moveRocks(dt)
	w.moveParticles(dt)
	if w.over {
		return
	}
	w.moveShip(c, dt)
	w.moveBullets(dt)
	if c.fire {
		w.fire()
	}
	w.shootRocks()
	w.crashShip()
	if len(w.rocks) == 0 {
		w.nextWave()
	}
}

// spawnShip puts a new ship in the middle, unable to crash for a while.
func (w *world) spawnShip() {
	w.ship = ship{x: worldWidth / 2, y: worldHeight / 2, alive: true, invulnerable: invulnerableTime}
}

// nextWave starts the next wave: one more big rock than the last, on a ring
// around the middle so that none starts on the ship.
func (w *world) nextWave() {
	w.wave++
	for range firstWaveRocks + w.wave - 1 {
		angle := float64(golib.RandomFloat(0, 2*math.Pi))
		distance := golib.RandomFloat(260, 420)
		x := wrap(worldWidth/2+distance*float32(math.Cos(angle)), worldWidth)
		y := wrap(worldHeight/2+distance*float32(math.Sin(angle)), worldHeight)
		w.rocks = append(w.rocks, newRock(x, y, bigRock))
	}
}

// newRock returns a rock of the given size at x, y, drifting and spinning in a
// random direction, with a random outline.
func newRock(x, y float32, size int) rock {
	direction := float64(golib.RandomFloat(0, 2*math.Pi))
	speed := golib.RandomFloat(rockSpeedMin, rockSpeedMax) * rockSpeedUp[size]
	r := rock{
		x:    x,
		y:    y,
		vx:   speed * float32(math.Cos(direction)),
		vy:   speed * float32(math.Sin(direction)),
		size: size,
		spin: golib.RandomFloat(-1.2, 1.2),
	}
	for i := range r.shape {
		r.shape[i] = golib.RandomFloat(0.72, 1.12)
	}
	return r
}

func (w *world) moveShip(c controls, dt float32) {
	s := &w.ship
	if !s.alive {
		w.respawn -= dt
		if w.respawn <= 0 {
			w.spawnShip()
		}
		return
	}
	s.angle += c.turn * turnSpeed * dt
	s.thrusting = c.thrust
	if c.thrust {
		fx, fy := s.facing()
		s.vx += fx * thrustPower * dt
		s.vy += fy * thrustPower * dt
	} else {
		keep := max(0, 1-shipDrag*dt)
		s.vx *= keep
		s.vy *= keep
	}
	if speed := length(s.vx, s.vy); speed > maxShipSpeed {
		s.vx *= maxShipSpeed / speed
		s.vy *= maxShipSpeed / speed
	}
	s.x = wrap(s.x+s.vx*dt, worldWidth)
	s.y = wrap(s.y+s.vy*dt, worldHeight)
	s.cooldown = max(0, s.cooldown-dt)
	s.invulnerable = max(0, s.invulnerable-dt)
}

// fire shoots a bullet from the ship's nose, unless the ship is gone, still
// cooling down or has too many bullets in flight.
func (w *world) fire() {
	s := &w.ship
	if !s.alive || s.cooldown > 0 || len(w.bullets) >= maxBullets {
		return
	}
	fx, fy := s.facing()
	w.bullets = append(w.bullets, bullet{
		x:    s.x + fx*shipNose,
		y:    s.y + fy*shipNose,
		vx:   s.vx + fx*bulletSpeed,
		vy:   s.vy + fy*bulletSpeed,
		life: bulletLifetime,
	})
	s.cooldown = fireCooldown
}

func (w *world) moveBullets(dt float32) {
	kept := w.bullets[:0]
	for _, b := range w.bullets {
		b.life -= dt
		if b.life <= 0 {
			continue
		}
		b.x = wrap(b.x+b.vx*dt, worldWidth)
		b.y = wrap(b.y+b.vy*dt, worldHeight)
		kept = append(kept, b)
	}
	w.bullets = kept
}

func (w *world) moveRocks(dt float32) {
	for i := range w.rocks {
		r := &w.rocks[i]
		r.x = wrap(r.x+r.vx*dt, worldWidth)
		r.y = wrap(r.y+r.vy*dt, worldHeight)
		r.angle += r.spin * dt
	}
}

func (w *world) moveParticles(dt float32) {
	kept := w.particles[:0]
	for _, p := range w.particles {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		kept = append(kept, p)
	}
	w.particles = kept
}

// shootRocks breaks every rock a bullet is inside, and scores it.
func (w *world) shootRocks() {
	for bi := 0; bi < len(w.bullets); bi++ {
		b := w.bullets[bi]
		for ri, r := range w.rocks {
			if distanceSquared(b.x, b.y, r.x, r.y) < r.radius()*r.radius() {
				w.score += rockPoints[r.size]
				w.breakRock(ri)
				w.bullets = append(w.bullets[:bi], w.bullets[bi+1:]...)
				bi--
				break
			}
		}
	}
}

// crashShip ends the ship if it touches a rock, unless it is invulnerable.
func (w *world) crashShip() {
	s := &w.ship
	if !s.alive || s.invulnerable > 0 {
		return
	}
	for i, r := range w.rocks {
		reach := r.radius() + shipRadius
		if distanceSquared(s.x, s.y, r.x, r.y) < reach*reach {
			w.breakRock(i)
			w.burst(s.x, s.y, 30)
			s.alive = false
			w.lives--
			if w.lives == 0 {
				w.over = true
			} else {
				w.respawn = respawnDelay
			}
			return
		}
	}
}

// breakRock splits rock number i into two smaller rocks, or removes it when it
// is already small, with a burst of sparks.
func (w *world) breakRock(i int) {
	r := w.rocks[i]
	w.rocks = append(w.rocks[:i], w.rocks[i+1:]...)
	w.burst(r.x, r.y, 8+4*r.size)
	if r.size > smallRock {
		w.rocks = append(w.rocks, newRock(r.x, r.y, r.size-1), newRock(r.x, r.y, r.size-1))
	}
}

// burst throws count sparks out from x, y.
func (w *world) burst(x, y float32, count int) {
	for range count {
		direction := float64(golib.RandomFloat(0, 2*math.Pi))
		speed := golib.RandomFloat(40, 220)
		w.particles = append(w.particles, particle{
			x:    x,
			y:    y,
			vx:   speed * float32(math.Cos(direction)),
			vy:   speed * float32(math.Sin(direction)),
			life: golib.RandomFloat(0.3, 1) * particleLifetime,
		})
	}
}

// wrap returns value moved into 0 to size, so what leaves one edge of the world
// comes back on the other.
func wrap(value, size float32) float32 {
	value = float32(math.Mod(float64(value), float64(size)))
	if value < 0 {
		value += size
	}
	return value
}

func distanceSquared(x1, y1, x2, y2 float32) float32 {
	dx, dy := x2-x1, y2-y1
	return dx*dx + dy*dy
}

func length(x, y float32) float32 {
	return float32(math.Hypot(float64(x), float64(y)))
}
