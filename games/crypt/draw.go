package main

import (
	"math"

	"golib"
)

// How the torches and the keys look and light the crypt.
const (
	torchIntensity = 2.6  // a torch's light, as golib.Light's Intensity
	torchRange     = 9.5  // meters a torch lights
	flameHeight    = 0.32 // meters
	keyLift        = 0.42 // meters above the floor a key floats
	keySpin        = 120  // degrees per second
)

// environment is a night in the crypt: a faint moon through the open roof,
// a dark sky, fog that swallows the far halls, and a glow on the flames.
var environment = golib.Environment{
	SunColor: moonColor, SunStrength: 0.32, SunDirection: golib.Vector3{X: 0.35, Y: -1, Z: 0.55},
	Sky: skyColor, Horizon: horizonColor, Ground: groundColor, SkyBackground: true,
	FogColor: fogColor, FogDistance: 42, ShadowDistance: 30, Bloom: 0.7, Exposure: 1.3,
}

// drawWorld draws the crypt and everyone in it through camera, with the
// crate under the mouse pointer, hovered, lit up (-1 for none).
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D, hovered int) {
	screen.Clear(horizonColor)
	screen.Begin3D(camera)
	screen.SetEnvironment(environment)
	for i, at := range w.level.torches {
		screen.AddLight(golib.Light{Position: at.Add(golib.Vector3{Y: 0.25}), Color: torchColor, Intensity: torchIntensity * flicker(w.time, i), Range: torchRange})
	}
	screen.AddLight(golib.Light{Position: w.level.exit.Add(golib.Vector3{Y: 1.2}), Color: exitColor, Intensity: 2.5, Range: 7})
	for _, k := range w.keys {
		if !k.taken {
			screen.AddLight(golib.Light{Position: keyPlace(w, k).Position, Color: keyLightColor, Intensity: 0.6, Range: 3})
		}
	}

	screen.DrawModel(levelModel, golib.Transform{}, golib.White)
	drawTorches(screen, w)
	drawDoors(screen, w)
	for i, c := range w.crates {
		if c.broken {
			continue
		}
		look := crateWood
		if i == hovered {
			look = crateLit
		}
		screen.DrawMeshWith(boxMesh, golib.Transform{Position: c.spec.center, Scale: c.spec.size}, look)
	}
	drawLift(screen, w)
	for _, k := range w.keys {
		if !k.taken {
			drawKey(screen, keyPlace(w, k))
		}
	}
	drawExit(screen, w)
	for _, d := range w.debris {
		look := debrisWood
		if d.stone {
			look = debrisStone
		}
		screen.DrawMeshWith(boxMesh, golib.Transform{Position: d.position, Rotation: d.turn, Scale: golib.Vector3{X: d.size, Y: d.size * 0.35, Z: d.size * 0.6}}, look)
	}
	screen.DrawAnimated(w.player.animator, golib.Transform{Position: w.player.body.Position, Rotation: golib.Rotation{Yaw: w.player.yaw}}, golib.White)
	for _, g := range w.guards {
		screen.DrawAnimated(g.animator, golib.Transform{Position: g.body.Position, Rotation: golib.Rotation{Yaw: g.yaw}}, guardTint)
	}
	screen.End3D()
}

// flicker is how bright torch i burns at time t, around 1: a few waves of
// its own, so no two torches flicker together.
func flicker(t float32, i int) float32 {
	at := float64(t) + float64(i)*1.37
	return float32(0.86 + 0.08*math.Sin(at*11.3) + 0.05*math.Sin(at*23.7+1) + 0.03*math.Sin(at*41.1+2))
}

// drawTorches draws a flickering flame over each bracket.
func drawTorches(screen *golib.Screen, w *world) {
	for i, at := range w.level.torches {
		f := flicker(w.time, i)
		height := flameHeight * (0.75 + 0.35*f)
		sway := float32(math.Sin(float64(w.time)*7+float64(i))) * 6
		base := golib.Transform{Position: at, Rotation: golib.Rotation{Roll: sway, Yaw: float32(i) * 40}}
		// A teardrop: a round bottom under a cone that licks upwards.
		screen.DrawMeshWith(sphereMesh, base.Child(golib.Transform{Position: golib.Vector3{Y: 0.07}, Scale: golib.Vector3{X: 0.17, Y: 0.17, Z: 0.17}}), flame)
		screen.DrawMeshWith(coneMesh, base.Child(golib.Transform{Position: golib.Vector3{Y: 0.07 + height/2}, Scale: golib.Vector3{X: 0.16, Y: height, Z: 0.16}}), flame)
		screen.DrawMeshWith(sphereMesh, base.Child(golib.Transform{Position: golib.Vector3{Y: 0.06}, Scale: golib.Vector3{X: 0.09, Y: 0.12, Z: 0.09}}), flameCore)
	}
}

// drawDoors draws the locked doors, and the opened ones sinking away.
func drawDoors(screen *golib.Screen, w *world) {
	for _, d := range w.doors {
		if d.open && !d.gone.Running() {
			continue
		}
		place := golib.Transform{Position: d.spec.center, Scale: d.spec.size}
		if d.open {
			left := 1 - golib.EaseIn(d.gone.Progress())
			place.Scale.Y = d.spec.size.Y * left
			place.Position.Y = d.spec.center.Y - d.spec.size.Y*(1-left)/2
		}
		screen.DrawMeshWith(boxMesh, place, doorWood)
	}
}

// drawLift draws the stone lift where it is, with an iron rim.
func drawLift(screen *golib.Screen, w *world) {
	l := &w.lift
	screen.DrawMeshWith(boxMesh, l.transform(), liftStone)
	rim := l.transform()
	rim.Position.Y = l.top() - 0.06
	rim.Scale = golib.Vector3{X: l.spec.size.X + 0.06, Y: 0.06, Z: l.spec.size.Z + 0.06}
	screen.DrawMeshWith(boxMesh, rim, liftRim)
}

// keyPlace is where a key floats and how it is turned now: spinning and
// bobbing over where it lies, and hopping out of its crate at first.
func keyPlace(w *world, k *key) golib.Transform {
	bob := 0.06 * float32(math.Sin(float64(w.time)*2.5))
	hop := float32(0)
	if k.fromBox && k.popped < 0.6 {
		t := k.popped / 0.6
		hop = 1.2 * t * (1 - t) * 4 * 0.5
	}
	return golib.Transform{
		Position: k.position.Add(golib.Vector3{Y: keyLift + bob + hop}),
		Rotation: golib.Rotation{Yaw: w.time * keySpin},
	}
}

// drawKey draws a golden key standing up, at place: a round bow, a shaft
// and two teeth.
func drawKey(screen *golib.Screen, place golib.Transform) {
	screen.DrawMeshWith(cylinderMesh, place.Child(golib.Transform{Position: golib.Vector3{Y: 0.13}, Rotation: golib.Rotation{Pitch: 90}, Scale: golib.Vector3{X: 0.2, Y: 0.05, Z: 0.2}}), keyGold)
	screen.DrawMeshWith(boxMesh, place.Child(golib.Transform{Position: golib.Vector3{Y: -0.06}, Scale: golib.Vector3{X: 0.05, Y: 0.3, Z: 0.05}}), keyGold)
	screen.DrawMeshWith(boxMesh, place.Child(golib.Transform{Position: golib.Vector3{X: 0.05, Y: -0.17}, Scale: golib.Vector3{X: 0.09, Y: 0.04, Z: 0.05}}), keyGold)
	screen.DrawMeshWith(boxMesh, place.Child(golib.Transform{Position: golib.Vector3{X: 0.05, Y: -0.09}, Scale: golib.Vector3{X: 0.07, Y: 0.04, Z: 0.05}}), keyGold)
}

// drawExit draws the way out: a shimmering doorway of light in the far
// wall, over a glowing ring on the floor.
func drawExit(screen *golib.Screen, w *world) {
	at := w.level.exit
	screen.DrawMeshWith(boxMesh, golib.Transform{Position: golib.Vector3{X: at.X, Y: at.Y + 1.3, Z: at.Z + 0.78}, Scale: golib.Vector3{X: 1.5, Y: 2.6, Z: 0.04}}, exitGlow)
	screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: at.Add(golib.Vector3{Y: 0.01}), Scale: golib.Vector3{X: 1.4, Y: 0.02, Z: 1.4}}, exitFloor)
}
