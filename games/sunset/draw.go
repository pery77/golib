package main

import (
	"math"

	"golib"
)

// How the world looks; the rules don't need these.
const (
	sunStrength   = 7    // the HDRI's sky lights everything softly: the sun must be strong to stand out from it
	exposure      = 0.7  // the whole picture
	bloom         = 0.15 // how strongly the crystals and the sun's glints glow
	shadowReach   = 70   // meters from the camera the sun casts shadows
	waterSize     = 3000 // meters: the water plane, out to the horizon
	buoyCount     = 28   // buoys in the ring around the lagoon
	buoyRing      = 59   // meters from the middle: out of the hovercraft's reach
	crystalSpin   = 90   // degrees per second
	crystalBob    = 0.18 // meters up and down
	crystalGrowIn = 0.5  // seconds a new crystal takes to grow to its size
	crystalScale  = 1.5  // the crystal mesh is a meter high: this many meters
	hoverHeight   = 0.18 // meters between the water and the skirt
	fanSpeed      = 1400 // degrees per second, at full throttle
)

// drawWorld draws the lagoon in 3D through camera: the sky, the water, the
// islets, poles and buoys, the crystals, the hovercraft and the effects.
func drawWorld(screen *golib.Screen, w *world, fx *effects, camera *golib.Camera3D) {
	screen.Clear(golib.Black)
	screen.Begin3D(camera)
	screen.SetEnvironment(golib.Environment{
		HDRI: sunsetSky, SkyBackground: true,
		SunDirection: sunDirection, SunColor: sunColor, SunStrength: sunStrength,
		Exposure: exposure, Bloom: bloom, ShadowDistance: shadowReach,
	})
	screen.DrawMeshWith(planeMesh, golib.Transform{Scale: golib.Vector3{X: waterSize, Z: waterSize}}, water)
	for i, o := range obstacles {
		if o.islet {
			drawIslet(screen, o, i)
		} else {
			drawPoles(screen, o, i)
		}
	}
	drawBuoys(screen, w.time)
	for _, k := range w.crystals {
		drawCrystal(screen, k, w.time)
	}
	drawCraft(screen, w.craft, w.time)
	fx.draw(screen, w.time)
	screen.End3D()
}

// drawIslet draws an islet: a dome of rock in a ring of sand, with grass on
// top and a few pines, which cast long shadows across it in the low sun. The
// largest has a lantern.
func drawIslet(screen *golib.Screen, o obstacle, index int) {
	r := o.radius
	at := o.position
	screen.DrawMeshWith(sphereMesh, golib.Transform{Position: at, Scale: golib.Vector3{X: 2*r + 2.4, Y: 0.5, Z: 2*r + 2.4}}, sand)
	screen.DrawMeshWith(sphereMesh, golib.Transform{Position: at.Add(golib.Vector3{Y: -0.2}), Scale: golib.Vector3{X: 2 * r, Y: 3.2, Z: 2 * r}}, rock)
	screen.DrawMeshWith(sphereMesh, golib.Transform{Position: at.Add(golib.Vector3{Y: 0.55}), Scale: golib.Vector3{X: 1.6 * r, Y: 2.4, Z: 1.6 * r}}, grass)
	// Boulders round the edge, and pines on top, placed from the islet's
	// index so they stay put.
	for i := range 5 {
		angle := float64(index*37+i*71) * math.Pi / 180
		spot := at.Add(golib.Vector3{X: (r + 0.3) * float32(math.Cos(angle)), Y: 0.1, Z: (r + 0.3) * float32(math.Sin(angle))})
		size := 0.8 + 0.25*float32(i%3)
		screen.DrawMeshWith(sphereMesh, golib.Transform{Position: spot, Rotation: golib.Rotation{Yaw: float32(i * 40)}, Scale: golib.Vector3{X: size * 1.4, Y: size, Z: size}}, rock)
	}
	pines := 1 + int(r/2.5)
	for i := range pines {
		angle := float64(index*53+i*137) * math.Pi / 180
		reach := r * 0.45 * float32(i%2+1) / 2
		base := at.Add(golib.Vector3{X: reach * float32(math.Cos(angle)), Y: 1.4, Z: reach * float32(math.Sin(angle))})
		height := 4 + float32((index+i)%3)
		screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: base.Add(golib.Vector3{Y: height * 0.25}), Scale: golib.Vector3{X: 0.3, Y: height * 0.5, Z: 0.3}}, bark)
		screen.DrawMeshWith(coneMesh, golib.Transform{Position: base.Add(golib.Vector3{Y: height * 0.55}), Scale: golib.Vector3{X: 2.2, Y: height * 0.55, Z: 2.2}}, pine)
		screen.DrawMeshWith(coneMesh, golib.Transform{Position: base.Add(golib.Vector3{Y: height * 0.85}), Scale: golib.Vector3{X: 1.6, Y: height * 0.45, Z: 1.6}}, pine)
	}
	if index == 1 {
		post := at.Add(golib.Vector3{X: -r * 0.5, Y: 1.6, Z: r * 0.5})
		screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: post.Add(golib.Vector3{Y: 1}), Scale: golib.Vector3{X: 0.15, Y: 2, Z: 0.15}}, darkMetal)
		screen.DrawMeshWith(sphereMesh, golib.Transform{Position: post.Add(golib.Vector3{Y: 2.2}), Scale: golib.Vector3{X: 0.45, Y: 0.45, Z: 0.45}}, lantern)
		screen.AddLight(golib.Light{Position: post.Add(golib.Vector3{Y: 2.2}), Color: lanternColor, Intensity: 6, Range: 14})
	}
}

// drawPoles draws a Venetian mooring post: three wooden poles leaning
// together, bound with an iron band.
func drawPoles(screen *golib.Screen, o obstacle, index int) {
	const height = 5
	for i := range 3 {
		angle := float32(index*29 + i*120)
		turn := golib.Rotation{Yaw: angle}
		foot := o.position.Add(turn.Forward().Scale(0.32))
		lean := golib.Rotation{Yaw: angle, Pitch: 4} // the tops lean in
		center := foot.Add(lean.Up().Scale(height/2 - 1))
		screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: center, Rotation: lean, Scale: golib.Vector3{X: 0.26, Y: height, Z: 0.26}}, wood)
	}
	screen.DrawMeshWith(cylinderMesh, golib.Transform{Position: o.position.Add(golib.Vector3{Y: 2.6}), Scale: golib.Vector3{X: 0.9, Y: 0.18, Z: 0.9}}, darkMetal)
}

// drawBuoys draws the ring of red and white buoys around the lagoon, bobbing,
// with lamps that blink in turn.
func drawBuoys(screen *golib.Screen, time float32) {
	for i := range buoyCount {
		angle := float64(i) * 2 * math.Pi / buoyCount
		bob := 0.08 * float32(math.Sin(float64(time)*1.6+float64(i)))
		at := golib.Vector3{X: buoyRing * float32(math.Cos(angle)), Y: bob, Z: buoyRing * float32(math.Sin(angle))}
		tilt := golib.Rotation{Pitch: 4 * float32(math.Sin(float64(time)*1.1+float64(i)*2)), Roll: 4 * float32(math.Cos(float64(time)*1.3+float64(i)))}
		body := redPaint
		if i%2 == 1 {
			body = whitePaint
		}
		place := golib.Transform{Position: at, Rotation: tilt}
		screen.DrawMeshWith(cylinderMesh, place.Child(golib.Transform{Position: golib.Vector3{Y: 0.25}, Scale: golib.Vector3{X: 1, Y: 0.7, Z: 1}}), body)
		screen.DrawMeshWith(coneMesh, place.Child(golib.Transform{Position: golib.Vector3{Y: 0.95}, Scale: golib.Vector3{X: 0.8, Y: 0.7, Z: 0.8}}), body)
		if int(time*1.5+float32(i)*0.37)%3 == 0 {
			screen.DrawMeshWith(sphereMesh, place.Child(golib.Transform{Position: golib.Vector3{Y: 1.4}, Scale: golib.Vector3{X: 0.18, Y: 0.18, Z: 0.18}}), lantern)
		}
	}
}

// drawCrystal draws a crystal spinning and bobbing over the water, with a
// lamp in it that lights the water and the hovercraft around it. A golden
// crystal blinks in its last three seconds.
func drawCrystal(screen *golib.Screen, k crystal, time float32) {
	t := time + k.phase
	if k.golden && k.life.Left() < 3 && math.Mod(float64(k.life.Left()), 0.4) < 0.15 {
		return
	}
	grow := golib.EaseOut(min(1, k.age/crystalGrowIn))
	at := k.position.Add(golib.Vector3{Y: crystalBob * float32(math.Sin(float64(t)*2.2))})
	material, halo, color, size := crystalGlow, crystalHalo, crystalColor, float32(crystalScale)
	if k.golden {
		material, halo, color, size = goldenGlow, goldenHalo, goldenColor, crystalScale*1.25
	}
	size *= grow
	screen.DrawMeshWith(crystalMesh, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: t * crystalSpin}, Scale: golib.Vector3{X: size, Y: size, Z: size}}, material)
	glow := size * (0.9 + 0.08*sinf(t*4))
	screen.DrawMeshWith(sphereMesh, golib.Transform{Position: at.Add(golib.Vector3{Y: 0.05 * size}), Scale: golib.Vector3{X: glow, Y: glow * 1.2, Z: glow}}, halo)
	// Two small shards beside it, turning the other way.
	for i, side := range []float32{-1, 1} {
		turn := golib.Rotation{Yaw: -t*crystalSpin*1.5 + float32(i)*180}
		shard := at.Add(turn.Forward().Scale(0.55)).Add(golib.Vector3{Y: side * 0.15})
		screen.DrawMeshWith(crystalMesh, golib.Transform{Position: shard, Rotation: golib.Rotation{Yaw: t * 200, Roll: side * 20}, Scale: golib.Vector3{X: size * 0.35, Y: size * 0.35, Z: size * 0.35}}, material)
	}
	screen.AddLight(golib.Light{Position: at.Add(golib.Vector3{Y: -0.4}), Color: color, Intensity: 3 * grow, Range: 8})
}

// drawCraft draws the hovercraft: a steel hull on a rubber skirt, orange
// side pods, a glass canopy over the pilot's helmet, and a fan at the back,
// leaning into turns and lifting its nose under thrust.
func drawCraft(screen *golib.Screen, c craft, time float32) {
	speed := flat(c.velocity).Length()
	lean := c.steer * 9 * min(1, speed/6)
	pitch := c.throttle * 3
	bob := 0.05 * float32(math.Sin(float64(time)*3.1))
	body := golib.Transform{
		Position: c.position.Add(golib.Vector3{Y: hoverHeight + bob}),
		Rotation: golib.Rotation{Yaw: c.yaw, Pitch: pitch, Roll: lean + 1.5*float32(math.Sin(float64(time)*2.3))},
	}
	part := func(mesh *golib.Mesh, material *golib.Material, at, scale golib.Vector3, turn golib.Rotation) {
		screen.DrawMeshWith(mesh, body.Child(golib.Transform{Position: at, Rotation: turn, Scale: scale}), material)
	}
	none := golib.Rotation{}
	// The skirt, an oval of rubber, and the steel deck on it.
	part(cylinderMesh, rubber, golib.Vector3{Y: 0.22}, golib.Vector3{X: 2.1, Y: 0.44, Z: 3.1}, none)
	part(boxMesh, steel, golib.Vector3{Y: 0.58}, golib.Vector3{X: 1.6, Y: 0.32, Z: 2.5}, none)
	// The bow, a steel wedge sloping down to the front.
	part(boxMesh, steel, golib.Vector3{Y: 0.55, Z: 1.35}, golib.Vector3{X: 1.5, Y: 0.12, Z: 0.7}, golib.Rotation{Pitch: -14})
	// Orange side pods, and a stripe across the bow.
	for _, side := range []float32{-1, 1} {
		part(boxMesh, steel, golib.Vector3{X: side * 0.86, Y: 0.72, Z: -0.1}, golib.Vector3{X: 0.36, Y: 0.42, Z: 2.3}, none)
		part(boxMesh, paint, golib.Vector3{X: side * 0.87, Y: 0.8, Z: -0.1}, golib.Vector3{X: 0.38, Y: 0.1, Z: 2.32}, none)
		part(sphereMesh, headlight, golib.Vector3{X: side * 0.86, Y: 0.74, Z: 1.06}, golib.Vector3{X: 0.18, Y: 0.14, Z: 0.08}, none)
		part(boxMesh, taillight, golib.Vector3{X: side * 0.86, Y: 0.76, Z: -1.26}, golib.Vector3{X: 0.24, Y: 0.1, Z: 0.04}, none)
	}
	part(boxMesh, paint, golib.Vector3{Y: 0.76, Z: 0.95}, golib.Vector3{X: 1.4, Y: 0.06, Z: 0.22}, none)
	// The pilot's helmet under the glass canopy.
	part(sphereMesh, helmet, golib.Vector3{Y: 1.02, Z: 0.05}, golib.Vector3{X: 0.38, Y: 0.4, Z: 0.4}, none)
	part(sphereMesh, glass, golib.Vector3{Y: 0.86, Z: 0.15}, golib.Vector3{X: 1.0, Y: 0.75, Z: 1.4}, none)
	// The fan: a steel ring on two struts, blades spinning inside.
	fanAt := golib.Vector3{Y: 1.25, Z: -0.95}
	upright := golib.Rotation{Pitch: 90} // a cylinder stood on its side, facing back
	part(cylinderMesh, steel, fanAt, golib.Vector3{X: 1.45, Y: 0.3, Z: 1.45}, upright)
	part(cylinderMesh, darkMetal, fanAt.Add(golib.Vector3{Z: -0.02}), golib.Vector3{X: 1.25, Y: 0.32, Z: 1.25}, upright)
	for _, side := range []float32{-1, 1} {
		part(boxMesh, darkMetal, golib.Vector3{X: side * 0.45, Y: 0.85, Z: -0.95}, golib.Vector3{X: 0.08, Y: 0.6, Z: 0.12}, none)
	}
	spin := time * (fanSpeed*0.3 + fanSpeed*0.7*max(0, c.throttle))
	for i := range 3 {
		part(boxMesh, rubber, fanAt.Add(golib.Vector3{Z: -0.2}), golib.Vector3{X: 1.15, Y: 0.12, Z: 0.04}, golib.Rotation{Roll: spin + float32(i)*60})
	}
	part(sphereMesh, paint, fanAt.Add(golib.Vector3{Z: -0.24}), golib.Vector3{X: 0.22, Y: 0.22, Z: 0.22}, none)
	// Rudders behind the fan, turned with the steer.
	for _, side := range []float32{-1, 1} {
		part(boxMesh, paint, golib.Vector3{X: side * 0.3, Y: 1.25, Z: -1.32}, golib.Vector3{X: 0.05, Y: 1.1, Z: 0.35}, golib.Rotation{Yaw: -c.steer * 20})
	}
	screen.AddLight(golib.Light{
		Position: body.Child(golib.Transform{Position: golib.Vector3{Y: 0.8, Z: 1.3}}).Position,
		Color:    golib.Color{R: 255, G: 236, B: 200, A: 255}, Intensity: 4, Range: 18,
		Direction: golib.Rotation{Yaw: c.yaw, Pitch: -12}.Forward(), Cone: 50,
	})
}

// sinf is math.Sin for float32s.
func sinf(x float32) float32 {
	return float32(math.Sin(float64(x)))
}
