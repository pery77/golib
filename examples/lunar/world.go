package main

import (
	"fmt"
	"golib"
	"math"
)

const (
	flying = iota
	landed
	crashed
)
const (
	footX    float32 = 19
	footY    float32 = 23
	fuelBurn float32 = 2.1
)

type landscape struct {
	terrain []golib.Vector2
	pads    [3]golib.Rectangle
	spawns  [3]golib.Vector2
	width   float32
}

var maps = [...]*golib.Map{
	golib.NewMap("maps/01-tranquility.tmx"),
	golib.NewMap("maps/02-glass.tmx"),
	golib.NewMap("maps/03-cinder.tmx"),
	golib.NewMap("maps/04-ion.tmx"),
	golib.NewMap("maps/05-polar.tmx"),
}
var landscapes [5]landscape

func loadRegions() error {
	for i, m := range maps {
		if err := m.Err(); err != nil {
			return err
		}
		l := landscape{width: m.Width()}
		for _, o := range m.Objects("terrain") {
			for _, v := range o.Points {
				l.terrain = append(l.terrain, v.Add(golib.Vector2{X: o.X, Y: o.Y}))
			}
		}
		for j := 0; j < 3; j++ {
			foundPad, foundSpawn := false, false
			for _, o := range m.Objects("navigation") {
				if o.Name == fmt.Sprintf("pad-%d", j) {
					l.pads[j] = o.Rectangle
					foundPad = true
				}
				if o.Name == fmt.Sprintf("spawn-%d", j) {
					l.spawns[j] = golib.Vector2{X: o.X, Y: o.Y}
					foundSpawn = true
				}
			}
			if !foundPad || !foundSpawn || l.pads[j].Width < 60 {
				return fmt.Errorf("region %d: invalid pad or insertion %d", i, j)
			}
		}
		if len(l.terrain) < 2 {
			return fmt.Errorf("region %d: missing terrain polyline", i)
		}
		for j := 1; j < len(l.terrain); j++ {
			if l.terrain[j].X <= l.terrain[j-1].X {
				return fmt.Errorf("region %d: terrain must run left to right", i)
			}
		}
		landscapes[i] = l
	}
	return nil
}

func (l landscape) height(x float32) float32 {
	for i := 1; i < len(l.terrain); i++ {
		a, b := l.terrain[i-1], l.terrain[i]
		if x <= b.X {
			return golib.Lerp(a.Y, b.Y, golib.Clamp((x-a.X)/(b.X-a.X), 0, 1))
		}
	}
	return l.terrain[len(l.terrain)-1].Y
}

type controls struct {
	turn, thrust float32
	stabilize    bool
}
type world struct {
	contract                              contract
	land                                  landscape
	spec                                  shipSpec
	pos, vel                              golib.Vector2
	angle, angular, fuel, elapsed, thrust float32
	stabilizing                           bool
	state, landingScore                   int
	reason                                string
	settled                               bool
}

func newWorld(p campaign, c contract) *world {
	return &world{contract: c, land: landscapes[c.Region], spec: p.stats(),
		pos: landscapes[c.Region].spawns[c.Kind], fuel: p.stats().Fuel}
}
func (w *world) pad() golib.Rectangle { return w.land.pads[w.contract.Kind] }
func (w *world) altitude() float32    { return max(0, (w.land.height(w.pos.X)-w.pos.Y-footY)/4) }
func abs(v float32) float32           { return float32(math.Abs(float64(v))) }
func sin(v float32) float32           { return float32(math.Sin(float64(v))) }
func (w *world) descentLimit() float32 {
	limit := w.spec.Landing
	if w.contract.Kind == 1 {
		limit *= 0.78
	}
	return limit
}
func (w *world) driftLimit() float32 { return w.descentLimit() * 0.68 }
func (w *world) feet() (golib.Vector2, golib.Vector2) {
	return golib.Vector2{X: -footX, Y: footY}.Rotate(w.angle).Add(w.pos),
		golib.Vector2{X: footX, Y: footY}.Rotate(w.angle).Add(w.pos)
}
func (w *world) fail(reason string) {
	w.state = crashed
	w.reason = reason
	w.thrust = 0
	w.stabilizing = false
}

func (w *world) step(c controls, dt float32) {
	if w.state != flying {
		return
	}
	w.elapsed += dt
	w.thrust = 0
	w.stabilizing = c.stabilize && w.fuel > 0
	desiredTurn := golib.Clamp(c.turn, -1, 1) * w.spec.Turn
	if w.stabilizing {
		desiredTurn = golib.Clamp(-w.angle*5, -w.spec.Turn, w.spec.Turn)
		w.vel.X *= max(0, 1-dt*1.9)
		w.fuel = max(0, w.fuel-dt*0.85)
	}
	w.angular = golib.Lerp(w.angular, desiredTurn, min(1, dt*7))
	w.angle += w.angular * dt
	if w.angle > 180 {
		w.angle -= 360
	}
	if w.angle < -180 {
		w.angle += 360
	}
	thrust := golib.Clamp(c.thrust, 0, 1)
	if w.fuel > 0 {
		// Consume the final partial frame of fuel without producing free thrust.
		thrust = min(thrust, w.fuel/(dt*fuelBurn))
		w.thrust = thrust
		w.fuel = max(0, w.fuel-thrust*fuelBurn*dt)
	}
	direction := golib.Vector2{Y: -1}.Rotate(w.angle)
	acceleration := direction.Scale(w.spec.Thrust * w.thrust)
	acceleration.Y += regions[w.contract.Region].Gravity
	acceleration.X += regions[w.contract.Region].Wind * sin(w.elapsed*0.7)
	w.vel = w.vel.Add(acceleration.Scale(dt))
	// Bound velocity and substep collision checks to prevent tunneling.
	w.vel = w.vel.ClampLength(240)
	distance := w.vel.Length() * dt
	steps := max(1, int(math.Ceil(float64(distance/3))))
	for range steps {
		w.pos = w.pos.Add(w.vel.Scale(dt / float32(steps)))
		if w.pos.X < 25 || w.pos.X > w.land.width-25 || w.pos.Y < -450 {
			w.fail("Flight boundary exceeded. Follow the destination beacon.")
			return
		}
		w.checkContact()
		if w.state != flying {
			return
		}
	}
}
func (w *world) checkContact() {
	left, right := w.feet()
	pad := w.pad()
	// A pad can be touched only while both landing feet fit over its surface.
	within := min(left.X, right.X) >= pad.X && max(left.X, right.X) <= pad.X+pad.Width
	if within && max(left.Y, right.Y) >= pad.Y && w.pos.Y < pad.Y {
		if abs(w.angle) > 12 {
			w.fail("Hull was tilted. Hold STABILIZE before touchdown.")
			return
		}
		if abs(w.vel.X) > w.driftLimit() {
			w.fail("Too much lateral drift. Brake with STABILIZE.")
			return
		}
		if w.vel.Y > w.descentLimit() || w.vel.Y < -4 {
			w.fail("Impact was too fast. Burn earlier to slow your descent.")
			return
		}
		w.landingScore = int(golib.Clamp(100-abs(w.vel.Y)/w.descentLimit()*38-
			abs(w.vel.X)/w.driftLimit()*20-abs(w.angle)*1.5-
			abs(w.pos.X-pad.Center().X)/pad.Width*20, 0, 100))
		w.pos.Y = pad.Y - footY
		w.angle = 0
		w.vel = golib.Vector2{}
		w.state = landed
		w.thrust = 0
		w.stabilizing = false
		return
	}
	// Feet and hull probes share the visual ship's geometry.
	probes := []golib.Vector2{left, right}
	for _, v := range []golib.Vector2{{X: -13, Y: -12}, {X: 13, Y: -12}, {Y: -24}, {Y: 14}} {
		probes = append(probes, v.Rotate(w.angle).Add(w.pos))
	}
	for _, v := range probes {
		if v.Y >= w.land.height(v.X) {
			w.fail("Terrain contact. Land only on the illuminated destination pad.")
			return
		}
	}
}
