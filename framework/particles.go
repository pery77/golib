package golib

import (
	"bytes"
	"cmp"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"

	"golib/internal/device"
)

// Particles are many small things that fly, change and are gone: fire,
// smoke, sparks, dust, rain, magic. Each is a picture that always faces the
// camera, moved by GoLib, and all of one Particles are drawn at once. Make
// them once, emit as things happen, and update and draw them every frame:
//
//	var sparks = golib.NewParticles(golib.ParticleSpec{
//		Color: golib.Yellow, EndColor: golib.Red, Size: 0.08, Life: 0.6,
//		Speed: 4, Spread: 60, Gravity: golib.Vector3{Y: -9.81}, Glow: true,
//	})
//
//	// When the sword hits:
//	sparks.Emit(hit.Position, hit.Normal, 30)
//
//	// In Update:
//	sparks.Update(dt)
//
//	// In Draw, between Begin3D and End3D:
//	screen.DrawParticles(sparks)
//
// They are drawn after everything solid, lit by nothing: their colors are
// as given. Random numbers come from golib's, so golib shot repeats them.
type Particles struct {
	spec      ParticleSpec
	material  *material3D
	particles []particle
}

// ParticleSpec says how the particles of a Particles look and move. Every
// field left at zero has a working default.
type ParticleSpec struct {
	// Texture is the picture of each particle: a white one is tinted by
	// Color. Nil is a soft round dot.
	Texture *Texture

	// Color is a particle's color as it is born, and EndColor as it dies,
	// blended between along its life; their opacity fades it. The zero
	// Color is white, and the zero EndColor is Color faded to nothing.
	Color, EndColor Color

	// Brightness multiplies the colors, past white for a glow that blooms:
	// 3 for sparks that dazzle. 0 counts as 1.
	Brightness float32

	// Size is how many meters across a particle is as it is born, 0.2 when
	// zero, and EndSize as it dies, Size when zero.
	Size, EndSize float32

	// Life is how many seconds a particle lives: 1 when zero. Each lives
	// between three quarters and five quarters of it.
	Life float32

	// Speed is how fast a particle leaves, in meters a second, along the
	// direction Emit gives, turned by up to Spread degrees: 0 straight, 180
	// every way.
	Speed, Spread float32

	// Gravity pulls the particles every second: golib.Vector3{Y: -9.81}
	// for sparks that fall, golib.Vector3{Y: 1} for smoke that rises.
	Gravity Vector3

	// Drag slows them: how much of their speed they lose in a second, from
	// 0, none, to 1.
	Drag float32

	// Glow adds their light to what is behind them, as fire, sparks and
	// magic do, so they need no order. Without it they cover what is
	// behind them, as smoke and dust do, the farthest drawn first.
	Glow bool
}

// particle is one of a Particles.
type particle struct {
	position, velocity Vector3
	age, life          float32
}

// maxParticles is how many particles one Particles keeps: emitting more
// replaces the oldest.
const maxParticles = 10000

// NewParticles returns particles that look and move as spec says, none of
// them born yet.
func NewParticles(spec ParticleSpec) *Particles {
	if spec.Size <= 0 {
		spec.Size = 0.2
	}
	if spec.EndSize <= 0 {
		spec.EndSize = spec.Size
	}
	if spec.Life <= 0 {
		spec.Life = 1
	}
	if spec.Brightness <= 0 {
		spec.Brightness = 1
	}
	if spec.Color == (Color{}) {
		spec.Color = White
	}
	if spec.EndColor == (Color{}) {
		spec.EndColor = Color{R: spec.Color.R, G: spec.Color.G, B: spec.Color.B}
	}
	b := spec.Brightness
	return &Particles{spec: spec, material: &material3D{
		baseColor: [4]float32{b, b, b, 1}, unlit: true, alpha: alphaBlend, twoSided: true,
	}}
}

// Emit makes count particles at position, leaving along direction, turned
// by up to the spec's Spread; a zero direction is up.
func (p *Particles) Emit(position, direction Vector3, count int) {
	if p == nil {
		reportError(errors.New("golib: Particles.Emit on nil Particles: make them with golib.NewParticles"))
		return
	}
	direction = direction.Normalize()
	if direction == (Vector3{}) {
		direction = Vector3{Y: 1}
	}
	// Two ways across direction, to turn it by the spread.
	across := direction.Cross(Vector3{Y: 1})
	if across.Length() < 0.01 {
		across = direction.Cross(Vector3{X: 1})
	}
	across = across.Normalize()
	other := direction.Cross(across)
	spread := float64(min(p.spec.Spread, 180)) * math.Pi / 180
	for range count {
		// A way within the cone, every part of it as likely.
		cosine := 1 - float64(RandomFloat(0, 1))*(1-math.Cos(spread))
		sine := math.Sqrt(max(0, 1-cosine*cosine))
		turn := float64(RandomFloat(0, 2*math.Pi))
		way := direction.Scale(float32(cosine)).
			Add(across.Scale(float32(sine * math.Cos(turn)))).
			Add(other.Scale(float32(sine * math.Sin(turn))))
		q := particle{
			position: position,
			velocity: way.Scale(p.spec.Speed),
			life:     p.spec.Life * RandomFloat(0.75, 1.25),
		}
		if len(p.particles) >= maxParticles {
			p.particles = p.particles[1:]
		}
		p.particles = append(p.particles, q)
	}
}

// Update moves the particles on by dt seconds, and takes out those whose
// life is over.
func (p *Particles) Update(dt float32) {
	if p == nil {
		return
	}
	keep := 1 - min(max(p.spec.Drag, 0), 1)*dt
	alive := p.particles[:0]
	for _, q := range p.particles {
		q.age += dt
		if q.age >= q.life {
			continue
		}
		q.velocity = q.velocity.Add(p.spec.Gravity.Scale(dt)).Scale(keep)
		q.position = q.position.Add(q.velocity.Scale(dt))
		alive = append(alive, q)
	}
	p.particles = alive
}

// Count returns how many particles are alive.
func (p *Particles) Count() int {
	if p == nil {
		return 0
	}
	return len(p.particles)
}

// Clear takes every particle out at once.
func (p *Particles) Clear() {
	if p != nil {
		p.particles = p.particles[:0]
	}
}

// particleDraw is a Particles as a pass draws it: one instanced draw.
type particleDraw struct {
	material  *material3D
	instances []float32
	glow      bool
}

// DrawParticles draws the particles, each facing the camera, after
// everything solid in the pass.
func (s *Screen) DrawParticles(p *Particles) {
	if !s.in3D("DrawParticles") {
		return
	}
	if p == nil {
		reportError(errors.New("golib: Screen.DrawParticles got nil Particles: make them with golib.NewParticles"))
		return
	}
	if len(p.particles) == 0 {
		return
	}
	material := p.material
	if t := p.spec.Texture; t != nil {
		if err := t.prepare(); err != nil {
			reportError(err)
			return
		}
		material.baseTexture = &t.texture
	} else {
		material.baseTexture = softDot
	}
	material.baseTexture.upload()
	particleQuad.upload()

	camera := s.pass.camera
	eye, _ := camera.view()
	right := camera.Right()
	forward := camera.Forward()
	up := right.Cross(forward).Normalize()
	order := p.particles
	if !p.spec.Glow {
		// What covers is drawn farthest first.
		order = slices.Clone(order)
		slices.SortFunc(order, func(a, b particle) int {
			return cmp.Compare(b.position.Distance(eye), a.position.Distance(eye))
		})
	}
	start, end := p.spec.Color, p.spec.EndColor
	instances := make([]float32, 0, len(order)*device.InstanceFloats)
	for _, q := range order {
		t := q.age / q.life
		size := Lerp(p.spec.Size, p.spec.EndSize, t)
		r, u, f := right.Scale(size), up.Scale(size), forward.Scale(-size)
		tint := colorFloats(lerpColor(start, end, t))
		instances = append(instances,
			r.X, r.Y, r.Z, 0, u.X, u.Y, u.Z, 0, f.X, f.Y, f.Z, 0,
			q.position.X, q.position.Y, q.position.Z, 1,
			tint[0], tint[1], tint[2], tint[3])
	}
	s.pass.particles = append(s.pass.particles, particleDraw{material: material, instances: instances, glow: p.spec.Glow})
}

// lerpColor blends a into b by t, opacity too.
func lerpColor(a, b Color, t float32) Color {
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(Lerp(float32(x), float32(y), t)))) }
	return Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}

// particleQuad is the square every particle is drawn as, a meter across,
// facing +Z.
var particleQuad = func() *Mesh {
	m := &Mesh{twoSided: true}
	n := Vector3{Z: 1}
	m.addVertex(Vector3{X: -0.5, Y: -0.5}, n, 0, 1)
	m.addVertex(Vector3{X: 0.5, Y: -0.5}, n, 1, 1)
	m.addVertex(Vector3{X: 0.5, Y: 0.5}, n, 1, 0)
	m.addVertex(Vector3{X: -0.5, Y: 0.5}, n, 0, 0)
	m.addQuad(0, 1, 2, 3)
	return m
}()

// softDot is the picture of a particle without a texture: a white dot,
// opaque in its middle and fading to nothing at its edge.
var softDot = func() *texture3D {
	const size = 64
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dx, dy := (float64(x)+0.5)/size*2-1, (float64(y)+0.5)/size*2-1
			d := math.Min(1, math.Sqrt(dx*dx+dy*dy))
			a := 1 - d*d*(3-2*d) // smooth
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(math.Round(a * 255))})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return &texture3D{data: b.Bytes(), options: device.TextureOptions{ClampS: true, ClampT: true}}
}()
