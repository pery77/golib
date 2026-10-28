package main

import (
	"math"

	"golib"
)

// The meshes, made once: the courtyard is built from these, scaled, turned
// and placed by a golib.Transform.
var (
	boxMesh      = golib.NewBoxMesh()
	cylinderMesh = golib.NewCylinderMesh()
	coneMesh     = golib.NewConeMesh()
	planeMesh    = golib.NewPlaneMesh()
)

// The courtyard's shape, around the arena the rules keep everyone in.
const (
	wallThick   = 0.7  // meters
	wallHeight  = 1.7  // meters
	gateHalf    = 1.0  // meters: half the width of each gate
	pillarSize  = 0.85 // meters across a gate's pillars
	pillarTall  = 2.9  // meters
	towerRadius = 1.1  // meters: the round towers in the corners
	towerTall   = 3.6  // meters
	groundSize  = 400  // meters: the meadow out to the horizon
)

// The stick in the player's right hand: a cylinder, its grip at the end of
// the arm, pointing up out of the fist and leaning a little outwards. The
// arm's bone sits at the shoulder, and the arm runs along its -X, so the hand
// is a quarter of a meter along it; the bone's Y runs up the fist.
var (
	stickLength = float32(0.62) // meters
	stickGrip   = golib.Transform{
		Position: golib.Vector3{X: -0.25, Y: -0.03},
		Rotation: golib.Rotation{Pitch: 20},
	}
)

// The materials, made once.
var (
	lawn       = &golib.Material{Color: grassColor, Roughness: 0.95, Shader: lawnShader}
	stone      = &golib.Material{Color: stoneColor, Roughness: 0.9}
	darkStone  = &golib.Material{Color: darkStoneColor, Roughness: 0.9}
	paving     = &golib.Material{Color: pavingColor, Roughness: 0.85}
	roof       = &golib.Material{Color: roofColor, Roughness: 0.6}
	doorWood   = &golib.Material{Color: doorColor, Roughness: 0.8}
	stickWood  = &golib.Material{Color: stickColor, Roughness: 0.7}
	wellWater  = &golib.Material{Color: waterColor, Roughness: 0.08, Shader: rippleShader}
	bannerRed  = &golib.Material{Color: bannerColor, Roughness: 0.7, TwoSided: true}
	ironBands  = &golib.Material{Color: ironColor, Metallic: 1, Roughness: 0.45}
	torchFlame = &golib.Material{Color: golib.Black, Emission: flameColor, EmissionStrength: 4}
)

// lawnShader mows stripes into the courtyard's lawn and lets the meadow
// outside grow wilder: a darker green that varies in patches.
const lawnShader = `
void surface(inout Surface s) {
    vec2 p = s.position.xz;
    float inside = step(abs(p.x), 11.0) * step(abs(p.y), 11.0);
    float stripe = step(0.5, fract(p.x / 2.2));
    float patches = sin(p.x * 0.9 + sin(p.y * 0.7) * 2.0) * sin(p.y * 1.1 + sin(p.x * 0.5) * 2.0);
    float meadow = 0.78 + 0.1 * patches;
    float mown = 0.9 + 0.12 * stripe + 0.03 * patches;
    s.color.rgb *= mix(meadow, mown, inside);
}`

// rippleShader stirs the well's water a little.
const rippleShader = `
void surface(inout Surface s) {
    float r = length(s.position.xz);
    s.normal = normalize(s.normal + 0.06 * vec3(sin(r * 14.0 - s.time * 3.0), 0.0, cos(r * 11.0 - s.time * 2.0)));
}`

// The sun, the sky and the shadows: a bright afternoon.
var environment = golib.Environment{
	SunDirection: golib.Vector3{X: -0.6, Y: -1, Z: -0.45}, SunStrength: 1.15,
	Sky: skyColor, Horizon: horizonColor, Ground: meadowColor, SkyBackground: true,
	FogDistance: 160, ShadowDistance: 40, Bloom: 0.25,
}

// drawWorld draws the courtyard and everyone in it through camera.
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D) {
	screen.Clear(horizonColor)
	screen.Begin3D(camera)
	screen.SetEnvironment(environment)
	drawCourtyard(screen, w.time)
	drawPlayer(screen, &w.player)
	for _, e := range w.enemies {
		drawEnemy(screen, e)
	}
	screen.End3D()
}

// drawPlayer draws the astronaut and the stick in its right hand, blinking
// pink for a moment after a punch.
func drawPlayer(screen *golib.Screen, p *player) {
	tint := golib.White
	if p.hurt.Running() && p.alive() && int(p.hurt.Left()*12)%2 == 0 {
		tint = hurtTint
	}
	body := golib.Transform{Position: p.position, Rotation: golib.Rotation{Yaw: p.yaw}}
	screen.DrawAnimated(p.animator, body, tint)
	hand := body.Child(p.animator.Bone(boneRightArm)).Child(stickGrip)
	// The cylinder stands on its middle: move it along its length so the
	// grip end is in the fist, with a little sticking out below.
	stick := hand.Child(golib.Transform{
		Position: golib.Vector3{Y: stickLength/2 - 0.08},
		Scale:    golib.Vector3{X: 0.06, Y: stickLength, Z: 0.06},
	})
	screen.DrawMeshWith(cylinderMesh, stick, stickWood)
}

// drawEnemy draws a red astronaut: gold for a moment when whacked, and
// fading away once it is down.
func drawEnemy(screen *golib.Screen, e *enemy) {
	tint := enemyTint
	if e.flash.Running() {
		tint = whackedTint
	}
	tint = golib.WithOpacity(tint, e.opacity())
	screen.DrawAnimated(e.animator, golib.Transform{Position: e.position, Rotation: golib.Rotation{Yaw: e.yaw}}, tint)
}

// drawCourtyard draws the lawn, the walls with their gates and towers, the
// well in the middle, and the grass, flowers and trees around.
func drawCourtyard(screen *golib.Screen, time float32) {
	screen.DrawMeshWith(planeMesh, golib.Transform{Scale: golib.Vector3{X: groundSize, Z: groundSize}}, lawn)

	// Paths of flagstones from each gate to the well, and a ring round it.
	for _, gate := range gates {
		in := gate.Normalize()
		middle := in.Scale((arenaHalf + wellRadius) / 2)
		long := float32(arenaHalf - wellRadius)
		size := golib.Vector3{X: 1.3, Y: 0.04, Z: long}
		if in.X != 0 {
			size = golib.Vector3{X: long, Y: 0.04, Z: 1.3}
		}
		screen.DrawMeshWith(boxMesh, golib.Transform{Position: middle, Scale: size}, paving)
	}
	screen.DrawMeshWith(cylinderMesh, golib.Transform{Scale: golib.Vector3{X: 2*wellRadius + 1.6, Y: 0.05, Z: 2*wellRadius + 1.6}}, paving)

	drawWalls(screen)
	drawWell(screen)
	drawGreenery(screen)
	drawTorches(screen, time)
}

// drawWalls draws the four walls, each in two halves either side of its
// gate, with battlements on top, the gates' pillars and doors, and a round
// tower with a pointed roof in each corner.
func drawWalls(screen *golib.Screen) {
	const half = arenaHalf + wallThick/2 // the walls' middle line
	segment := float32(arenaHalf + wallThick - gateHalf)
	for side := range 4 {
		turn := golib.Transform{Rotation: golib.Rotation{Yaw: float32(side) * 90}}
		for _, along := range []float32{-1, 1} {
			mid := gateHalf + segment/2
			wall := turn.Child(golib.Transform{
				Position: golib.Vector3{X: along * mid, Y: wallHeight / 2, Z: half},
				Scale:    golib.Vector3{X: segment, Y: wallHeight, Z: wallThick},
			})
			screen.DrawMeshWith(boxMesh, wall, stone)
			// Battlements: a merlon every 1.1 meters.
			for x := float32(gateHalf + 0.6); x < arenaHalf; x += 1.1 {
				merlon := turn.Child(golib.Transform{
					Position: golib.Vector3{X: along * x, Y: wallHeight + 0.2, Z: half},
					Scale:    golib.Vector3{X: 0.5, Y: 0.4, Z: wallThick},
				})
				screen.DrawMeshWith(boxMesh, merlon, stone)
			}
			// The gate's pillar on this side, with a banner.
			pillar := turn.Child(golib.Transform{
				Position: golib.Vector3{X: along * (gateHalf + pillarSize/2), Y: pillarTall / 2, Z: half},
				Scale:    golib.Vector3{X: pillarSize, Y: pillarTall, Z: wallThick + 0.3},
			})
			screen.DrawMeshWith(boxMesh, pillar, darkStone)
			banner := turn.Child(golib.Transform{
				Position: golib.Vector3{X: along * (gateHalf + pillarSize/2), Y: pillarTall - 0.85, Z: half - (wallThick+0.3)/2 - 0.02},
				Scale:    golib.Vector3{X: 0.5, Y: 1.1, Z: 0.02},
			})
			screen.DrawMeshWith(boxMesh, banner, bannerRed)
		}
		// The lintel over the gate, and its closed door of planks.
		lintel := turn.Child(golib.Transform{
			Position: golib.Vector3{Y: pillarTall - 0.3, Z: half},
			Scale:    golib.Vector3{X: 2 * gateHalf, Y: 0.6, Z: wallThick + 0.3},
		})
		screen.DrawMeshWith(boxMesh, lintel, darkStone)
		door := turn.Child(golib.Transform{
			Position: golib.Vector3{Y: (pillarTall - 0.6) / 2, Z: half + 0.1},
			Scale:    golib.Vector3{X: 2 * gateHalf, Y: pillarTall - 0.6, Z: 0.2},
		})
		screen.DrawMeshWith(boxMesh, door, doorWood)
		for _, y := range []float32{0.5, 1.7} {
			band := turn.Child(golib.Transform{
				Position: golib.Vector3{Y: y, Z: half - 0.02},
				Scale:    golib.Vector3{X: 2 * gateHalf, Y: 0.12, Z: 0.06},
			})
			screen.DrawMeshWith(boxMesh, band, ironBands)
		}
		// The tower at this side's corner.
		corner := turn.Child(golib.Transform{Position: golib.Vector3{X: half, Z: half}})
		screen.DrawMeshWith(cylinderMesh, corner.Child(golib.Transform{
			Position: golib.Vector3{Y: towerTall / 2},
			Scale:    golib.Vector3{X: 2 * towerRadius, Y: towerTall, Z: 2 * towerRadius},
		}), stone)
		screen.DrawMeshWith(cylinderMesh, corner.Child(golib.Transform{
			Position: golib.Vector3{Y: towerTall + 0.15},
			Scale:    golib.Vector3{X: 2*towerRadius + 0.3, Y: 0.3, Z: 2*towerRadius + 0.3},
		}), darkStone)
		screen.DrawMeshWith(coneMesh, corner.Child(golib.Transform{
			Position: golib.Vector3{Y: towerTall + 1.2},
			Scale:    golib.Vector3{X: 2*towerRadius + 0.5, Y: 1.9, Z: 2*towerRadius + 0.5},
		}), roof)
	}
}

// drawWell draws the well in the middle: a ring of stone round still water,
// two posts and a little roof.
func drawWell(screen *golib.Screen) {
	screen.DrawMeshWith(cylinderMesh, golib.Transform{
		Position: golib.Vector3{Y: 0.35},
		Scale:    golib.Vector3{X: 2 * wellRadius, Y: 0.7, Z: 2 * wellRadius},
	}, stone)
	screen.DrawMeshWith(cylinderMesh, golib.Transform{
		Position: golib.Vector3{Y: 0.71},
		Scale:    golib.Vector3{X: 2*wellRadius - 0.35, Y: 0.02, Z: 2*wellRadius - 0.35},
	}, wellWater)
	for _, x := range []float32{-1, 1} {
		screen.DrawMeshWith(boxMesh, golib.Transform{
			Position: golib.Vector3{X: x * (wellRadius - 0.1), Y: 1.2},
			Scale:    golib.Vector3{X: 0.14, Y: 1.4, Z: 0.14},
		}, doorWood)
	}
	screen.DrawMeshWith(boxMesh, golib.Transform{
		Position: golib.Vector3{Y: 1.75},
		Scale:    golib.Vector3{X: 2 * wellRadius, Y: 0.08, Z: 0.08},
	}, doorWood)
	for _, z := range []float32{-1, 1} {
		screen.DrawMeshWith(boxMesh, golib.Transform{
			Position: golib.Vector3{Y: 2.05, Z: z * 0.38},
			Rotation: golib.Rotation{Pitch: z * 35},
			Scale:    golib.Vector3{X: 2*wellRadius + 0.4, Y: 0.06, Z: 0.95},
		}, roof)
	}
}

// greenery is where the kit's grass, flowers, rocks and trees stand, worked
// out once from fixed numbers, so the courtyard looks the same every game.
type planted struct {
	model *golib.Model
	place golib.Transform
}

var greenery = plantGreenery()

func plantGreenery() []planted {
	var out []planted
	// A cheap scatter that needs no random numbers: the golden angle.
	for i := range 70 {
		angle := float64(i) * 2.39996
		reach := 2.6 + float32(math.Mod(float64(i)*0.618, 1))*7.8
		at := golib.Vector3{X: reach * float32(math.Cos(angle)), Z: reach * float32(math.Sin(angle))}
		if abs(at.X) < 1.0 || abs(at.Z) < 1.0 { // keep the paths clear
			continue
		}
		model := grassModel
		switch i % 5 {
		case 1, 3:
			model = flowersModel
		case 4:
			if i%10 == 9 {
				model = rocksModel
			}
		}
		out = append(out, planted{model, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: float32(i * 47)}}})
	}
	// Trees round the outside of the walls.
	for i := range 36 {
		angle := float64(i)*2*math.Pi/36 + 0.07*float64(i%3)
		reach := float32(17 + 5*math.Mod(float64(i)*0.618, 1) + 3*float64(i%2))
		at := golib.Vector3{X: reach * float32(math.Cos(angle)), Z: reach * float32(math.Sin(angle))}
		model := treeModel
		if i%3 != 0 {
			model = pineModel
		}
		size := 2.2 + float32(i%4)*0.35
		out = append(out, planted{model, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: float32(i * 71)}, Scale: golib.Vector3{X: size, Y: size, Z: size}}})
	}
	return out
}

func drawGreenery(screen *golib.Screen) {
	for _, p := range greenery {
		screen.DrawModel(p.model, p.place, golib.White)
	}
}

// drawTorches draws a flame on each gate's pillars, flickering, inside the
// courtyard.
func drawTorches(screen *golib.Screen, time float32) {
	const half = arenaHalf + wallThick/2
	for side := range 4 {
		turn := golib.Transform{Rotation: golib.Rotation{Yaw: float32(side) * 90}}
		for i, along := range []float32{-1, 1} {
			flicker := 1 + 0.12*float32(math.Sin(float64(time*17+float32(side*3+i))))
			at := turn.Child(golib.Transform{
				Position: golib.Vector3{X: along * (gateHalf + pillarSize/2), Y: 2.0, Z: half - (wallThick+0.3)/2 - 0.12},
				Scale:    golib.Vector3{X: 0.13 * flicker, Y: 0.22 * flicker, Z: 0.13 * flicker},
			})
			screen.DrawMeshWith(coneMesh, at, torchFlame)
		}
	}
}
