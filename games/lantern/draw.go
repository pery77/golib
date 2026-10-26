package main

import (
	"math"

	"golib"
)

// The look of the night in the temple.
var (
	nightSky     = golib.Color{R: 14, G: 20, B: 40, A: 255}
	nightHorizon = golib.Color{R: 30, G: 38, B: 66, A: 255}
	hazeColor    = golib.Color{R: 190, G: 205, B: 255, A: 255}
	water        = golib.Water{Color: golib.Color{R: 10, G: 46, B: 52, A: 255}, Clarity: 1.4, Waves: 0.5}
)

const (
	haze        = 0.04 // light the air takes in a meter: a dusty, damp hall
	torchReach  = 10   // meters a torch's light reaches
	exposure    = 1.25
	bloom       = 0.7
	torchFlick  = 0.28 // how much of a torch's light flickers
	runeSize    = 1.5  // meters across
	gemSpin     = 70   // degrees per second
	gemBob      = 0.08 // meters up and down
	lanternGlow = 0.6  // the explorer's lantern, as a lamp
	gemGlow      = 0.5 // each gem's light
	gemGlowReach = 3   // meters
)

// flicker is how bright torch i is at time t, about 1: a few waves of
// different speeds, so no two torches flicker together.
func flicker(i int, t float32) float32 {
	p := float64(i) * 1.7
	x := float64(t)
	f := 0.5*math.Sin(x*11.3+p) + 0.3*math.Sin(x*17.9+2.3*p) + 0.2*math.Sin(x*5.1+0.7*p)
	return 1 + torchFlick*float32(f)
}

// drawWorld draws the temple, lit, and everything in it, through camera.
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D, showExplorer bool) {
	screen.Begin3D(camera)
	env := golib.Environment{
		Sky: nightSky, Horizon: nightHorizon, Ground: golib.Black, SkyBackground: true,
		Haze: haze, HazeColor: hazeColor, Exposure: exposure, Bloom: bloom,
	}
	if sun, ok := level.Sun(); ok {
		env.SunDirection, env.SunColor, env.SunStrength = sun.Direction, sun.Color, sun.Strength
	}
	screen.SetEnvironment(env)
	for i, lamp := range level.Lights() {
		lamp.Intensity *= flicker(i, w.time)
		// Only the 4 nearest lamps cast shadows, so the others would light
		// the next hall through its wall: a torch lights its own hall.
		lamp.Range = torchReach
		screen.AddLight(lamp)
	}
	if showExplorer {
		screen.AddLight(golib.Light{
			Position: lanternPosition(w), Color: golib.Color{R: 255, G: 190, B: 120, A: 255},
			Intensity: lanternGlow, Range: 4,
		})
	}
	// Each gem lights the air and the stone round it in its color.
	for _, g := range w.gems {
		if !g.taken {
			screen.AddLight(golib.Light{Position: g.position, Color: gemColors[g.color], Intensity: gemGlow, Range: gemGlowReach})
		}
	}
	screen.DrawModel(level, golib.Transform{}, golib.White)
	// The heart of each flame, solid, so the polished floor and the water
	// reflect it: particles are drawn after the reflections, and aren't in
	// them.
	for i, at := range w.torches {
		f := flicker(i, w.time)
		screen.DrawMeshWith(coneMesh, golib.Transform{
			Position: at.Add(golib.Vector3{Y: 0.11 * f}),
			Scale:    golib.Vector3{X: 0.09, Y: 0.22 * f, Z: 0.09},
		}, flameMaterial)
	}
	drawGate(screen, w)
	for i, g := range w.gems {
		if !g.taken {
			screen.DrawMeshWith(gemMesh, gemPlace(w, i), gemMaterials[g.color])
		}
	}
	if showExplorer {
		drawExplorer(screen, w)
	}
	for i, g := range w.gems {
		glow := w.runeGlow(g)
		if glow <= 0 {
			continue
		}
		c := gemColors[g.color]
		c.A = uint8(255 * glow)
		screen.DrawDecal(golib.Decal{Texture: runeImage, Color: c, Glow: true, Brightness: 2.5},
			golib.Transform{Position: g.floor, Rotation: golib.Rotation{Yaw: float32(i) * 37},
				Scale: golib.Vector3{X: runeSize, Y: 0.6, Z: runeSize}})
	}
	screen.DrawWater(water, golib.Transform{
		Position: golib.Vector3{X: (pool.x0 + pool.x1) / 2, Y: waterLevel, Z: (pool.z0 + pool.z1) / 2},
		Scale:    golib.Vector3{X: pool.x1 - pool.x0, Z: pool.z1 - pool.z0},
	})
	screen.DrawParticles(smoke)
	screen.DrawParticles(ripples)
	screen.DrawParticles(flames)
	screen.DrawParticles(sparks)
	for _, g := range glimmers {
		screen.DrawParticles(g)
	}
	screen.End3D()
}

// gemPlace is where gem i floats now, bobbing and turning.
func gemPlace(w *world, i int) golib.Transform {
	bob := gemBob * float32(math.Sin(float64(w.time*2.2+float32(i))))
	return golib.Transform{
		Position: w.gems[i].position.Add(golib.Vector3{Y: bob}),
		Rotation: golib.Rotation{Yaw: w.time*gemSpin + float32(i)*40},
	}
}

// drawGate draws the gate, sinking as it opens, with a socket for each gem
// that glows once the gem is taken.
func drawGate(screen *golib.Screen, w *world) {
	if w.gate.opening >= 1 {
		return
	}
	t := w.gate.transform()
	screen.DrawMeshWith(boxMesh, t, gateMaterial)
	front := t.Position.Z - w.gate.size.Z/2 - 0.02
	for i, g := range w.gems {
		x := t.Position.X + (float32(i)-2.5)*0.34
		at := golib.Vector3{X: x, Y: t.Position.Y + 0.6, Z: front}
		m := socketMaterial
		if g.taken {
			m = gemMaterials[g.color]
		}
		screen.DrawMeshWith(sphereMesh, golib.Transform{Position: at, Scale: golib.Vector3{X: 0.2, Y: 0.2, Z: 0.08}}, m)
	}
}

// lanternPosition is where the explorer's lantern hangs, at their right
// hand.
func lanternPosition(w *world) golib.Vector3 {
	p := w.player
	body := golib.Transform{Position: p.body.Position, Rotation: golib.Rotation{Yaw: p.yaw}}
	swing := 0.04 * float32(math.Sin(float64(p.stride)*3))
	return body.Child(golib.Transform{Position: golib.Vector3{X: -0.42, Y: 0.85 + swing, Z: 0.12}}).Position
}

// drawExplorer draws the hooded explorer from shapes: boots, a long cloak,
// a hood with a dark face in it, and a lantern in the right hand.
func drawExplorer(screen *golib.Screen, w *world) {
	p := w.player
	bob := 0.04 * float32(math.Abs(math.Sin(float64(p.stride)*3)))
	body := golib.Transform{Position: p.body.Position.Add(golib.Vector3{Y: bob}), Rotation: golib.Rotation{Yaw: p.yaw}}
	step := 0.12 * float32(math.Sin(float64(p.stride)*3))
	for _, side := range []float32{-1, 1} {
		boot := golib.Transform{Position: golib.Vector3{X: 0.12 * side, Y: 0.08, Z: step * side}, Scale: golib.Vector3{X: 0.14, Y: 0.16, Z: 0.26}}
		screen.DrawMeshWith(boxMesh, body.Child(boot), bootMaterial)
	}
	screen.DrawMeshWith(coneMesh, body.Child(golib.Transform{Position: golib.Vector3{Y: 0.68}, Scale: golib.Vector3{X: 0.72, Y: 1.2, Z: 0.6}}), cloakMaterial)
	screen.DrawMeshWith(capsuleMesh, body.Child(golib.Transform{Position: golib.Vector3{Y: 1.05}, Scale: golib.Vector3{X: 0.75, Y: 0.75, Z: 0.75}}), cloakMaterial)
	screen.DrawMeshWith(sphereMesh, body.Child(golib.Transform{Position: golib.Vector3{Y: 1.5}, Scale: golib.Vector3{X: 0.4, Y: 0.42, Z: 0.42}}), hoodMaterial)
	screen.DrawMeshWith(coneMesh, body.Child(golib.Transform{Position: golib.Vector3{Y: 1.76, Z: -0.08}, Rotation: golib.Rotation{Pitch: -25}, Scale: golib.Vector3{X: 0.3, Y: 0.32, Z: 0.3}}), hoodMaterial)
	screen.DrawMeshWith(sphereMesh, body.Child(golib.Transform{Position: golib.Vector3{Y: 1.47, Z: 0.12}, Scale: golib.Vector3{X: 0.26, Y: 0.28, Z: 0.2}}), faceMaterial)
	// The arm holding the lantern out, and the lantern.
	lantern := lanternPosition(w)
	screen.DrawMeshWith(capsuleMesh, body.Child(golib.Transform{Position: golib.Vector3{X: -0.36, Y: 1.05, Z: 0.08}, Rotation: golib.Rotation{Roll: -18}, Scale: golib.Vector3{X: 0.28, Y: 0.55, Z: 0.28}}), cloakMaterial)
	screen.DrawMeshWith(boxMesh, golib.Transform{Position: lantern.Add(golib.Vector3{Y: 0.13}), Rotation: golib.Rotation{Yaw: p.yaw}, Scale: golib.Vector3{X: 0.14, Y: 0.04, Z: 0.14}}, ironMaterial)
	screen.DrawMeshWith(boxMesh, golib.Transform{Position: lantern, Rotation: golib.Rotation{Yaw: p.yaw}, Scale: golib.Vector3{X: 0.1, Y: 0.18, Z: 0.1}}, lanternMaterial)
}

// burnTorches keeps the torches' fire burning: flames every update, smoke
// and sparks now and then. It changes only the particles, so the scene's
// Update calls it.
func burnTorches(w *world, dt float32) {
	for _, at := range w.torches {
		flames.Emit(at.Add(golib.Vector3{Y: 0.05}), golib.Vector3{Y: 1}, 2)
		if golib.RandomFloat(0, 1) < 0.18 {
			smoke.Emit(at.Add(golib.Vector3{Y: 0.35}), golib.Vector3{Y: 1}, 1)
		}
		if golib.RandomFloat(0, 1) < 0.04 {
			sparks.Emit(at.Add(golib.Vector3{Y: 0.1}), golib.Vector3{Y: 1}, 2)
		}
	}
	flames.Update(dt)
	smoke.Update(dt)
	sparks.Update(dt)
	for _, g := range glimmers {
		g.Update(dt)
	}
	ripples.Update(dt)
}
