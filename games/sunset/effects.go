package main

import (
	"math"

	"golib"
)

// How the effects look and move; the rules don't need them.
const (
	sprayEvery    = 0.02 // seconds between droplets of spray at full speed
	sprayLife     = 0.9  // seconds a puff lasts
	sparkLife     = 0.7  // seconds the sparks of a collected crystal fly
	sparkCount    = 14
	flashLife     = 0.45 // seconds the light of a collected crystal lasts
	popupLife     = 1.1  // seconds "+1" floats over a collected crystal
	gravity       = 9.8  // m/s²
	sprayMinSpeed = 2    // m/s: slower, the skirt makes no spray or wake
	wakeEvery     = 0.06 // seconds between points of the wake
	wakeLife      = 2.6  // seconds the wake lasts
	wakeSpread    = 1.2  // m/s the wake's two lines move apart
	wakeWidth     = 0.2  // meters: a line of the wake, new; it widens to three times that
)

// wakePoint is a point of the wake: where the two sides of the skirt were,
// spreading apart and fading.
type wakePoint struct {
	left, right golib.Vector3
	out         golib.Vector3 // the way right spreads; left spreads the other way
	age         float32
	strength    float32 // 0 to 1, from the speed it was made at
}

// particle is a puff of spray or a spark: something small that flies, falls
// and fades.
type particle struct {
	position, velocity golib.Vector3
	age, life          float32 // seconds
	size               float32 // meters
	spark, golden      bool
}

// flash is the light of a collected crystal, and the points it gave,
// floating up.
type flash struct {
	position golib.Vector3
	age      float32
	points   int
	golden   bool
}

// effects are the spray, sparks and flashes: only for the eye, moved in
// Update and drawn in Draw.
type effects struct {
	particles []particle
	flashes   []flash
	wake      []wakePoint
	spray     float32 // seconds to the next droplet
	wakeTimer float32 // seconds to the next point of the wake
}

// update moves every effect on, and makes spray from the hovercraft's skirt
// while it moves.
func (e *effects) update(c craft, dt float32) {
	speed := flat(c.velocity).Length()
	if speed > sprayMinSpeed {
		e.spray -= dt * speed / maxSpeed
		for e.spray <= 0 {
			e.spray += sprayEvery
			e.puff(c, speed)
		}
		e.wakeTimer -= dt
		if e.wakeTimer <= 0 {
			e.wakeTimer += wakeEvery
			turn := golib.Rotation{Yaw: c.yaw}
			back := c.position.Add(turn.Forward().Scale(-1.3)).Add(golib.Vector3{Y: 0.03})
			e.wake = append(e.wake, wakePoint{
				left: back.Add(turn.Right().Scale(-0.9)), right: back.Add(turn.Right().Scale(0.9)),
				out: turn.Right(), strength: min(1, speed/(maxSpeed*0.6)),
			})
		}
	}
	wake := e.wake[:0]
	for _, w := range e.wake {
		w.age += dt
		if w.age < wakeLife {
			w.left = w.left.Add(w.out.Scale(-wakeSpread * dt))
			w.right = w.right.Add(w.out.Scale(wakeSpread * dt))
			wake = append(wake, w)
		}
	}
	e.wake = wake
	kept := e.particles[:0]
	for _, p := range e.particles {
		p.age += dt
		if p.age >= p.life {
			continue
		}
		p.velocity.Y -= gravity * dt * 0.6
		if p.spark {
			p.velocity = p.velocity.Scale(1 - 2*dt)
		}
		p.position = p.position.Add(p.velocity.Scale(dt))
		if p.position.Y < 0.05 && !p.spark {
			p.position.Y, p.velocity.Y = 0.05, 0
		}
		kept = append(kept, p)
	}
	e.particles = kept
	flashes := e.flashes[:0]
	for _, f := range e.flashes {
		f.age += dt
		if f.age < popupLife {
			flashes = append(flashes, f)
		}
	}
	e.flashes = flashes
}

// puff throws a little spray from one side of the skirt's back.
func (e *effects) puff(c craft, speed float32) {
	turn := golib.Rotation{Yaw: c.yaw}
	side := float32(1)
	if golib.RandomInt(0, 1) == 0 {
		side = -1
	}
	at := c.position.Add(turn.Right().Scale(side * golib.RandomFloat(0.6, 1.1))).
		Add(turn.Forward().Scale(golib.RandomFloat(-1.5, -0.6))).Add(golib.Vector3{Y: 0.1})
	out := turn.Right().Scale(side * golib.RandomFloat(0.8, 2.2)).
		Add(c.velocity.Scale(-0.15)).
		Add(turn.Forward().Scale(golib.RandomFloat(-1, 1))).
		Add(golib.Vector3{Y: golib.RandomFloat(0.5, 3) * speed / maxSpeed})
	e.particles = append(e.particles, particle{
		position: at, velocity: out,
		life: sprayLife * golib.RandomFloat(0.7, 1.2), size: golib.RandomFloat(0.05, 0.14),
	})
}

// burst throws sparks out of a collected crystal and lights it up.
func (e *effects) burst(k crystal) {
	for i := range sparkCount {
		angle := float64(i)*2*math.Pi/sparkCount + float64(golib.RandomFloat(-0.2, 0.2))
		out := golib.Vector3{
			X: float32(math.Cos(angle)) * golib.RandomFloat(3, 6),
			Y: golib.RandomFloat(2, 6),
			Z: float32(math.Sin(angle)) * golib.RandomFloat(3, 6),
		}
		e.particles = append(e.particles, particle{
			position: k.position, velocity: out, life: sparkLife * golib.RandomFloat(0.7, 1.2),
			size: golib.RandomFloat(0.12, 0.22), spark: true, golden: k.golden,
		})
	}
	e.flashes = append(e.flashes, flash{position: k.position, points: k.points(), golden: k.golden})
}

// draw draws the wake, the spray and the sparks, and the light of each flash.
func (e *effects) draw(screen *golib.Screen, time float32) {
	for i := 1; i < len(e.wake); i++ {
		a, b := e.wake[i-1], e.wake[i]
		if b.age-a.age > wakeEvery*1.5 || b.age-a.age < -wakeEvery*1.5 {
			continue // the hovercraft stopped between them
		}
		left := 1 - b.age/wakeLife
		fade := foam[min(foamFades-1, int(left*b.strength*foamFades))]
		width := wakeWidth * (1 + 2*b.age/wakeLife)
		foamPatch(screen, a.left, b.left, width, fade)
		foamPatch(screen, a.right, b.right, width, fade)
	}
	for _, p := range e.particles {
		left := 1 - p.age/p.life
		if p.spark {
			material := sparkCyan
			if p.golden {
				material = sparkGold
			}
			size := p.size * left
			screen.DrawMeshWith(crystalMesh, golib.Transform{Position: p.position,
				Rotation: golib.Rotation{Yaw: time*400 + p.size*1000, Pitch: p.size * 900},
				Scale:    golib.Vector3{X: size * 2, Y: size * 2, Z: size * 2}}, material)
			continue
		}
		size := p.size * golib.EaseOut(left) // a droplet shrinks away
		screen.DrawMeshWith(sphereMesh, golib.Transform{Position: p.position, Scale: golib.Vector3{X: size, Y: size, Z: size}}, droplet)
	}
	for _, f := range e.flashes {
		if f.age > flashLife {
			continue
		}
		color := crystalColor
		if f.golden {
			color = goldenColor
		}
		screen.AddLight(golib.Light{Position: f.position, Color: color, Intensity: 12 * (1 - f.age/flashLife), Range: 12})
	}
}

// foamPatch draws a flat oval of foam on the water, from one point of the
// wake to the next and a little past both, so the patches run together.
func foamPatch(screen *golib.Screen, from, to golib.Vector3, width float32, material *golib.Material) {
	along := to.Sub(from)
	length := along.Length()
	if length < 0.001 {
		return
	}
	screen.DrawMeshWith(sphereMesh, golib.Transform{
		Position: from.Add(along.Scale(0.5)),
		Rotation: golib.Rotation{Yaw: golib.RotationTowards(along).Yaw},
		Scale:    golib.Vector3{X: width, Y: 0.02, Z: length * 1.8},
	}, material)
}
