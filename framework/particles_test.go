package golib

import (
	"testing"
)

func TestParticlesMoveAgeAndDie(t *testing.T) {
	SetRandomSeed(1)
	p := NewParticles(ParticleSpec{Speed: 2, Life: 1, Gravity: Vector3{Y: -1}})
	p.Emit(Vector3{X: 3}, Vector3{Y: 1}, 50)
	if p.Count() != 50 {
		t.Fatalf("%d particles after emitting 50", p.Count())
	}
	p.Update(0.5)
	for _, q := range p.particles {
		// Straight up at 2 m/s, slowed by gravity: about a meter up.
		if q.position.X != 3 || q.position.Y < 0.7 || q.position.Y > 1.1 {
			t.Fatalf("a particle sent straight up is at %v after half a second", q.position)
		}
	}
	p.Update(0.8) // past every life, which is at most 1.25 seconds
	if p.Count() != 0 {
		t.Errorf("%d particles still alive past their lives", p.Count())
	}

	// Spread 180 sends them every way, down too.
	every := NewParticles(ParticleSpec{Speed: 1, Spread: 180})
	every.Emit(Vector3{}, Vector3{Y: 1}, 400)
	down := 0
	for _, q := range every.particles {
		if q.velocity.Y < -0.5 {
			down++
		}
	}
	if down < 50 {
		t.Errorf("%d of 400 particles spread every way go down, want about a quarter", down)
	}
	every.Clear()
	if every.Count() != 0 {
		t.Error("Clear left particles")
	}

	// The oldest give way past the most one Particles keeps.
	many := NewParticles(ParticleSpec{})
	many.Emit(Vector3{}, Vector3{}, maxParticles+10)
	if many.Count() != maxParticles {
		t.Errorf("%d particles kept, want %d", many.Count(), maxParticles)
	}
}

// Glowing particles add their light to what is behind them; the others
// cover it.
func TestParticlesInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -3}, Vector3{}
	middle := func(background Color, spec ParticleSpec) int {
		p := NewParticles(spec)
		p.Emit(Vector3{}, Vector3{}, 1)
		c := capture(func() {
			screen.Clear(background)
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{NoBloom: true})
			screen.DrawParticles(p)
			screen.End3D()
		}).NRGBAAt(32, 32)
		return int(c.R) + int(c.G) + int(c.B)
	}
	if glow := middle(Black, ParticleSpec{Color: Orange, Size: 1, Glow: true}); glow < 150 {
		t.Errorf("a glowing particle over black is %d bright, want its orange", glow)
	}
	if cover := middle(White, ParticleSpec{Color: Black, Size: 1}); cover > 120 {
		t.Errorf("a black particle over white is %d bright, want it to cover the white", cover)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
