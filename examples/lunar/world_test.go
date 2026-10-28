package main

import (
	"golib"
	"testing"
)

const dt = 1.0 / 60

func testWorld(t *testing.T) *world {
	t.Helper()
	if err := loadRegions(); err != nil {
		t.Fatal(err)
	}
	return newWorld(newCampaign(), contract{})
}
func TestGravityAndEngine(t *testing.T) {
	w := testWorld(t)
	start := w.pos.Y
	for range 60 {
		w.step(controls{}, dt)
	}
	if w.pos.Y <= start || w.vel.Y < 13 {
		t.Fatal("freefall did not accelerate downwards")
	}
	w = testWorld(t)
	for range 60 {
		w.step(controls{thrust: 1}, dt)
	}
	if w.vel.Y >= 0 || w.fuel >= w.spec.Fuel {
		t.Fatal("engine failed to lift or consume fuel")
	}
}
func TestLandingEnvelope(t *testing.T) {
	cases := []struct {
		name          string
		vx, vy, angle float32
		state         int
	}{
		{"safe", 0, 8, 0, landed}, {"fast", 0, 80, 0, crashed}, {"drifting", 45, 5, 0, crashed}, {"tilted", 0, 5, 20, crashed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := testWorld(t)
			pad := w.pad()
			w.pos = golib.Vector2{X: pad.Center().X, Y: pad.Y - footY - 0.01}
			w.vel = golib.Vector2{X: c.vx, Y: c.vy}
			w.angle = c.angle
			w.step(controls{}, dt)
			if w.state != c.state {
				t.Fatalf("got state %d, want %d", w.state, c.state)
			}
		})
	}
}
func TestFuelEmptyCannotProduceThrust(t *testing.T) {
	w := testWorld(t)
	w.fuel = 0
	w.step(controls{thrust: 1, stabilize: true}, dt)
	if w.thrust != 0 || w.stabilizing || w.vel.Y <= 0 {
		t.Fatal("empty tank produced control authority")
	}
}
func TestSettlementCannotPayTwice(t *testing.T) {
	w := testWorld(t)
	p := newCampaign()
	w.state = landed
	w.landingScore = 95
	r := p.settle(w)
	before := p.Credits
	if r.Total <= 0 || p.Deliveries != 1 || p.Perfect != 1 {
		t.Fatal("missing reward")
	}
	p.settle(w)
	if p.Credits != before || p.Deliveries != 1 {
		t.Fatal("duplicate reward")
	}
}
func TestTransactions(t *testing.T) {
	p := newCampaign()
	if p.buyShip(2) || p.buyBase(4) {
		t.Fatal("unaffordable/locked purchase succeeded")
	}
	if !p.buyUpgrade(0) || p.Credits != 50 {
		t.Fatal("first upgrade transaction failed")
	}
	p.Credits = 50000
	p.Deliveries = 14
	for i := 0; i < 5; i++ {
		if !p.buyBase(i) {
			t.Fatal("base purchase failed")
		}
	}
	if p.colonies() != 5 || p.dividend() != 1250 {
		t.Fatalf("network did not grow: %d", p.dividend())
	}
	if !p.buyShip(2) || p.stats().Cargo != 1.8 {
		t.Fatal("ship purchase did not change capabilities")
	}
}
