package main

import (
	"math"

	"golib"
)

// How things move for the eye; the rules don't need these.
const (
	coinSpinSpeed = 160  // degrees per second
	coinBob       = 0.07 // meters up and down
	rockShake     = 0.06 // meters a punched rock shakes
	seaSize       = 600  // meters: the sea out to the horizon
)

// The meshes and materials, made once.
var (
	planeMesh = golib.NewPlaneMesh()
	sea       = &golib.Material{Color: seaColor, Roughness: 0.12, Shader: waveShader}
	foam      = &golib.Material{Color: golib.WithOpacity(golib.White, 0.55), Roughness: 0.4}
	ringMesh  = golib.NewCylinderMesh()
)

// waveShader stirs the sea's surface, and pales it to foam close to the
// island's shore: the sea is darker away from the middle.
const waveShader = `
void surface(inout Surface s) {
    vec2 p = s.position.xz;
    s.normal = normalize(s.normal + 0.07 * vec3(sin(p.x * 1.7 + s.time * 1.3) + sin(p.y * 0.9 - s.time), 0.0,
                                                 cos(p.y * 1.5 + s.time * 1.1) + cos(p.x * 0.7 + s.time * 0.8)));
    float far = smoothstep(14.0, 60.0, length(p));
    s.color.rgb *= mix(1.15, 0.8, far);
}`

// The sun, the sky and the shadows: a bright, clear morning, the sun high on
// the left as the astronaut sets off, so the trees' shadows fall across the
// grass.
var environment = golib.Environment{
	SunDirection: golib.Vector3{X: -0.55, Y: -1, Z: 0.35}, SunStrength: 1.2,
	Sky: skyColor, Horizon: horizonColor, Ground: seaBedColor, SkyBackground: true,
	FogDistance: 220, ShadowDistance: 45, Bloom: 0.3,
}

// drawWorld draws the island and everything on it through camera.
func drawWorld(screen *golib.Screen, w *world, camera *golib.Camera3D) {
	screen.Clear(horizonColor)
	screen.Begin3D(camera)
	screen.SetEnvironment(environment)
	screen.DrawMeshWith(planeMesh, golib.Transform{Position: golib.Vector3{Y: seaLevel}, Scale: golib.Vector3{X: seaSize, Z: seaSize}}, sea)
	drawIsland(screen, w.island)
	drawRocks(screen, w)
	drawCoins(screen, w)
	drawFlag(screen, w)
	drawPlayer(screen, &w.player)
	screen.End3D()
}

// drawIsland draws the grass blocks, stacked as high as each cell is, the
// trees and the flowers.
func drawIsland(screen *golib.Screen, l *island) {
	for row := range l.rows {
		for col := range l.cols {
			height := l.heights[row][col]
			at := l.center(row, col)
			for k := range height {
				screen.DrawModel(blockModel, golib.Transform{Position: at.Add(golib.Vector3{Y: float32(k) * l.blockHeight})}, golib.White)
			}
		}
	}
	for _, t := range l.trees {
		model := treeModel
		if t.pine {
			model = pineModel
		}
		screen.DrawModel(model, golib.Transform{Position: t.position, Rotation: golib.Rotation{Yaw: t.yaw}, Scale: golib.Vector3{X: t.scale, Y: t.scale, Z: t.scale}}, golib.White)
	}
	for _, f := range l.flowers {
		screen.DrawModel(flowersModel, f, golib.White)
	}
}

// drawRocks draws the rocks that still stand, shaking and flashing for a
// moment after a punch, and smaller and more cracked the more punches they
// have taken; and the pieces of the broken ones.
func drawRocks(screen *golib.Screen, w *world) {
	for i, r := range w.rocks {
		if r.broken {
			continue
		}
		at := r.position
		tint := golib.White
		if r.shake.Running() {
			strength := rockShake * (1 - r.shake.Progress())
			wobble := float32(math.Sin(float64(w.time*70 + float32(i))))
			at = at.Add(golib.Vector3{X: wobble * strength, Z: -wobble * strength * 0.6})
			tint = hitTint
		}
		size := float32(rockScale) * (1 - 0.08*float32(r.hits))
		screen.DrawModel(rocksModel, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: float32(i * 67)}, Scale: golib.Vector3{X: size, Y: size, Z: size}}, tint)
	}
	for _, p := range w.pieces {
		size := 0.35 * (1 - p.life.Progress()*0.6)
		screen.DrawModel(rocksModel, golib.Transform{Position: p.position, Rotation: p.spin, Scale: golib.Vector3{X: size, Y: size, Z: size}}, golib.White)
	}
}

// drawCoins draws the coins not yet picked up, spinning and bobbing.
func drawCoins(screen *golib.Screen, w *world) {
	for i, c := range w.coins {
		if c.taken {
			continue
		}
		at := c.position
		if !c.flying {
			at.Y += coinBob * float32(math.Sin(float64(w.time*3+float32(i))))
		}
		at.Y -= 0.2 * coinScale // the coin's model stands on its edge: put its middle at the coin's position
		spin := w.time*coinSpinSpeed + float32(i*40)
		screen.DrawModel(coinModel, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: spin}, Scale: golib.Vector3{X: coinScale, Y: coinScale, Z: coinScale}}, golib.White)
	}
}

// drawFlag draws the flag at the far end, swaying a little in the wind.
func drawFlag(screen *golib.Screen, w *world) {
	sway := 6 * float32(math.Sin(float64(w.time*2.2)))
	screen.DrawModel(flagModel, golib.Transform{Position: w.island.flag, Rotation: golib.Rotation{Yaw: 200 + sway}, Scale: golib.Vector3{X: flagScale, Y: flagScale, Z: flagScale}}, golib.White)
}

// drawPlayer draws the astronaut, and the splash where it fell in the sea.
func drawPlayer(screen *golib.Screen, p *player) {
	body := golib.Transform{Position: p.position, Rotation: golib.Rotation{Yaw: p.yaw}}
	if !p.splash.Running() || p.position.Y > seaLevel-0.3 {
		screen.DrawAnimated(p.animator, body, golib.White)
	}
	if p.splash.Running() {
		// A ring of foam that spreads and fades where it went in.
		grow := golib.EaseOut(p.splash.Progress())
		size := 0.4 + 1.8*grow
		ring := golib.Transform{Position: golib.Vector3{X: p.position.X, Y: seaLevel + 0.02, Z: p.position.Z}, Scale: golib.Vector3{X: size, Y: 0.02, Z: size}}
		foam.Color = golib.WithOpacity(golib.White, 0.6*(1-grow))
		screen.DrawMeshWith(ringMesh, ring, foam)
	}
}
