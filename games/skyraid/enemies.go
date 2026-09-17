package main

import (
	"math"

	"golib"
)

// Enemy kinds, as indexes into enemyKinds.
const (
	scout = iota
	gunship
	heavy
)

// enemyKind is what every enemy of one kind shares. Speeds and fire rates
// grow with the wave (see waves.go).
type enemyKind struct {
	name       string
	radius     float32 // pixels, for hits
	hull       int     // hits it takes
	speed      float32 // pixels per second, in wave 1
	response   float32 // per second: how quickly it reaches the speed it steers to
	rams       bool    // flies straight at the ship, and breaks when it hits it
	keepAway   float32 // pixels it tries to keep from the ship; 0 for none
	fireEvery  float32 // seconds between volleys, in wave 1
	volley     int     // bullets per volley, in a fan
	fan        float32 // radians between the bullets of a volley
	shotSpeed  float32 // pixels per second
	shotRadius float32 // pixels
	points     int     // score, times the wave number
	dropChance float32 // chance of leaving a repair kit, from 0 to 1
	size       float32 // how big its explosion is: sparks, shake and sound
}

var enemyKinds = [...]enemyKind{
	scout: {
		name: "scout", radius: 13, hull: 1, speed: 235, response: 3.2, rams: true,
		fireEvery: 2.6, volley: 1, shotSpeed: 330, shotRadius: 5,
		points: 100, dropChance: 0.05, size: 1,
	},
	gunship: {
		name: "gunship", radius: 20, hull: 3, speed: 165, response: 2.5, keepAway: 340,
		fireEvery: 2.0, volley: 3, fan: 0.22, shotSpeed: 360, shotRadius: 6,
		points: 250, dropChance: 0.12, size: 1.6,
	},
	heavy: {
		name: "heavy", radius: 32, hull: 12, speed: 90, response: 1.5, keepAway: 460,
		fireEvery: 2.8, volley: 7, fan: 0.16, shotSpeed: 250, shotRadius: 8,
		points: 600, dropChance: 0.4, size: 2.6,
	},
}

// More enemy tuning.
const (
	fireRange      = 720  // pixels: enemies farther from the ship hold fire
	firstShotDelay = 1.2  // seconds, at least, before a new enemy fires
	enemyFlashTime = 0.08 // seconds an enemy flashes when hit
	personalSpace  = 18   // pixels enemies keep between each other's edges
	enemyShotLife  = 3.2  // seconds an enemy bullet flies
	warpTime       = 0.9  // seconds from the warp ring to the enemy
	strafeFactor   = 0.8  // share of its speed that an enemy at its distance circles with
)

// enemy is one enemy ship.
type enemy struct {
	kind     int
	x, y     float32 // center
	vx, vy   float32 // pixels per second
	angle    float32 // radians it faces; it always faces the ship
	hull     int
	cooldown float32 // seconds until its next volley
	flash    float32 // seconds left of the hit flash
	circle   float32 // -1 or 1: which way it circles the ship
}

// warp is an enemy about to arrive: a ring that shrinks for warpTime.
type warp struct {
	kind int
	x, y float32
	left float32 // seconds until the enemy arrives
}

// newEnemy returns an enemy of a kind at x, y, facing the ship.
func (w *world) newEnemy(kind int, x, y float32) enemy {
	k := enemyKinds[kind]
	circle := float32(1)
	if golib.RandomInt(0, 1) == 0 {
		circle = -1
	}
	return enemy{
		kind:     kind,
		x:        x,
		y:        y,
		angle:    angleOf(w.ship.x-x, w.ship.y-y),
		hull:     k.hull,
		cooldown: firstShotDelay + golib.RandomFloat(0, k.fireEvery),
		circle:   circle,
	}
}

// moveEnemies brings in the enemies whose warp has ended, then steers and
// fires every enemy.
func (w *world) moveEnemies(dt float32) {
	kept := w.warps[:0]
	for _, wp := range w.warps {
		wp.left -= dt
		if wp.left > 0 {
			kept = append(kept, wp)
			continue
		}
		w.enemies = append(w.enemies, w.newEnemy(wp.kind, wp.x, wp.y))
		w.burst(wp.x, wp.y, 10, 140, warpColor)
	}
	w.warps = kept

	speedScale := waveSpeedScale(w.wave)
	fireScale := waveFireScale(w.wave)
	for i := range w.enemies {
		e := &w.enemies[i]
		k := enemyKinds[e.kind]
		e.flash = max(0, e.flash-dt)

		wantX, wantY := w.steer(i)
		speed := k.speed * speedScale
		e.vx = approach(e.vx, wantX*speed, k.response, dt)
		e.vy = approach(e.vy, wantY*speed, k.response, dt)
		e.x = clamp(e.x+e.vx*dt, k.radius, worldWidth-k.radius)
		e.y = clamp(e.y+e.vy*dt, k.radius, worldHeight-k.radius)

		if !w.ship.alive {
			continue
		}
		e.angle = angleOf(w.ship.x-e.x, w.ship.y-e.y)
		e.cooldown -= dt * fireScale
		if e.cooldown <= 0 {
			e.cooldown += k.fireEvery
			if distanceSquared(e.x, e.y, w.ship.x, w.ship.y) < fireRange*fireRange {
				w.enemyFire(e)
			}
		}
	}
}

// steer returns the direction enemy i wants to fly in, at most 1 long: at the
// ship, away from it or around it, and away from enemies too close to it.
func (w *world) steer(i int) (float32, float32) {
	e := &w.enemies[i]
	k := enemyKinds[e.kind]
	var x, y float32
	if w.ship.alive {
		toX, toY := w.ship.x-e.x, w.ship.y-e.y
		distance := length(toX, toY)
		toX, toY = normalize(toX, toY)
		switch {
		case k.keepAway == 0 || distance > k.keepAway*1.25:
			x, y = toX, toY
		case distance < k.keepAway*0.8:
			x, y = -toX, -toY
		default:
			// About at its distance: circle the ship, drifting back to the
			// distance it likes.
			pull := (distance - k.keepAway) / k.keepAway
			x = -toY*e.circle*strafeFactor + toX*pull*2
			y = toX*e.circle*strafeFactor + toY*pull*2
		}
	} else {
		// No ship to chase: drift around.
		x, y = direction(w.time*0.3 + float32(i))
		x, y = x*0.3, y*0.3
	}

	for j := range w.enemies {
		if j == i {
			continue
		}
		o := &w.enemies[j]
		reach := k.radius + enemyKinds[o.kind].radius + personalSpace
		dx, dy := e.x-o.x, e.y-o.y
		d2 := dx*dx + dy*dy
		if d2 >= reach*reach || d2 == 0 {
			continue
		}
		d := float32(math.Sqrt(float64(d2)))
		push := (reach - d) / reach * 2
		x += dx / d * push
		y += dy / d * push
	}
	return limit(x, y, 1)
}

// enemyFire shoots a volley from an enemy at the ship: one bullet, or a fan
// of them centered on the ship.
func (w *world) enemyFire(e *enemy) {
	k := enemyKinds[e.kind]
	first := e.angle - k.fan*float32(k.volley-1)/2
	for n := range k.volley {
		dx, dy := direction(first + k.fan*float32(n))
		w.enemyShots = append(w.enemyShots, shot{
			x: e.x + dx*k.radius, y: e.y + dy*k.radius,
			vx: dx * k.shotSpeed, vy: dy * k.shotSpeed,
			radius: k.shotRadius, life: enemyShotLife,
		})
	}
	enemyShotSounds[e.kind].Play()
}

// hitEnemies damages every enemy a bullet of the ship touches.
func (w *world) hitEnemies() {
	kept := w.shots[:0]
	for _, b := range w.shots {
		hit := false
		for i := range w.enemies {
			e := &w.enemies[i]
			k := enemyKinds[e.kind]
			if !circlesTouch(b.x, b.y, b.radius, e.x, e.y, k.radius) {
				continue
			}
			hit = true
			e.hull--
			e.flash = enemyFlashTime
			// A hit nudges the enemy back.
			e.vx += b.vx * 0.08 / k.size
			e.vy += b.vy * 0.08 / k.size
			if e.hull <= 0 {
				w.destroyEnemy(i, true)
			} else {
				w.burst(b.x, b.y, 4, 120, sparkColor)
				hitSound.Play()
			}
			break
		}
		if !hit {
			kept = append(kept, b)
		}
	}
	w.shots = kept
}

// destroyEnemy blows up enemy i and removes it. scored is false when the
// enemy broke by ramming the ship, which still counts, for half the points.
func (w *world) destroyEnemy(i int, scored bool) {
	e := w.enemies[i]
	k := enemyKinds[e.kind]
	w.enemies = append(w.enemies[:i], w.enemies[i+1:]...)
	points := k.points * max(1, w.wave)
	if !scored {
		points /= 2
	}
	w.score += points
	w.kills++
	w.burst(e.x, e.y, int(18*k.size), 260*k.size, enemyColors[e.kind])
	w.burst(e.x, e.y, int(10*k.size), 180*k.size, flameColor)
	w.blast(e.x, e.y, k.radius, sparkColor)
	w.shake(traumaExplosion * k.size)
	if k.size > 2 {
		bigExplosionSound.Play()
	} else {
		explosionSound.Play()
	}
	if golib.RandomFloat(0, 1) < k.dropChance {
		w.repairs = append(w.repairs, repairKit{x: e.x, y: e.y, life: repairLifetime})
	}
}
