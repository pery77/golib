package main

import (
	"math"

	"golib"
)

// The sky: Poly Haven's Venice Sunset, with the waterfront's houses and
// paving painted out by sources/skies/lakesky.go, so only the sky and the
// open water towards the sunset are left. It lights the world and shows
// behind it.
var sunsetSky = golib.NewHDRI("skies/lake_sunset.hdr")

// The meshes, made once: every shape in the game is one of these, scaled,
// turned and placed by a golib.Transform.
var (
	boxMesh      = golib.NewBoxMesh()
	sphereMesh   = golib.NewSphereMesh()
	cylinderMesh = golib.NewCylinderMesh()
	coneMesh     = golib.NewConeMesh()
	planeMesh    = golib.NewPlaneMesh()
	crystalMesh  = newCrystalMesh()
)

// newCrystalMesh makes a crystal: a six-sided bipyramid, one meter high and
// half a meter across, its faces flat so each catches the light on its own.
// Every triangle has corners of its own, so NewMesh gives each its own
// normal instead of smoothing them together.
func newCrystalMesh() *golib.Mesh {
	const sides, radius, top, bottom = 6, 0.25, 0.6, -0.4
	ring := make([]golib.Vector3, sides)
	for i := range ring {
		angle := float64(i) * 2 * math.Pi / sides
		ring[i] = golib.Vector3{X: radius * float32(math.Cos(angle)), Z: radius * float32(math.Sin(angle))}
	}
	var vertices []golib.Vertex
	var indices []int
	triangle := func(a, b, c golib.Vector3) {
		n := len(vertices)
		vertices = append(vertices, golib.Vertex{Position: a}, golib.Vertex{Position: b}, golib.Vertex{Position: c})
		indices = append(indices, n, n+1, n+2)
	}
	apex, foot := golib.Vector3{Y: top}, golib.Vector3{Y: bottom}
	for i := range sides {
		next := ring[(i+1)%sides]
		triangle(apex, next, ring[i]) // counterclockwise seen from outside
		triangle(foot, ring[i], next)
	}
	return golib.NewMesh(vertices, indices)
}

// The materials, made once.
var (
	// water ripples its normals with a few slow waves, so the smooth surface
	// shimmers as it reflects the sky. The waves fade out far away, where
	// they would only flicker.
	water = &golib.Material{Color: waterColor, Roughness: 0.12, Shader: waterShader}

	steel      = &golib.Material{Color: steelColor, Metallic: 1, Roughness: 0.28}
	darkMetal  = &golib.Material{Color: darkSteel, Metallic: 1, Roughness: 0.4}
	paint      = &golib.Material{Color: paintColor, Metallic: 0.6, Roughness: 0.3}
	rubber     = &golib.Material{Color: rubberColor, Roughness: 0.9}
	glass      = &golib.Material{Color: glassColor, Roughness: 0.05}
	helmet     = &golib.Material{Color: helmetColor, Roughness: 0.35}
	headlight  = &golib.Material{Color: golib.Black, Emission: golib.Color{R: 255, G: 240, B: 210, A: 255}, EmissionStrength: 3}
	taillight  = &golib.Material{Color: golib.Black, Emission: golib.Red, EmissionStrength: 3}
	rock       = &golib.Material{Color: rockColor, Roughness: 0.9}
	sand       = &golib.Material{Color: sandColor, Roughness: 0.95}
	grass      = &golib.Material{Color: grassColor, Roughness: 0.9}
	pine       = &golib.Material{Color: pineColor, Roughness: 0.85}
	bark       = &golib.Material{Color: barkColor, Roughness: 0.9}
	wood       = &golib.Material{Color: poleColor, Roughness: 0.8}
	redPaint   = &golib.Material{Color: buoyRed, Roughness: 0.4}
	whitePaint = &golib.Material{Color: buoyWhite, Roughness: 0.4}
	lantern    = &golib.Material{Color: golib.Black, Emission: lanternColor, EmissionStrength: 4}

	// The crystals glow from inside, pulsing a little, each at its own
	// moment, from where it is.
	crystalGlow = &golib.Material{Color: golib.Color{R: 20, G: 60, B: 80, A: 255}, Roughness: 0.1, Metallic: 0.3,
		Emission: crystalColor, EmissionStrength: 6, Shader: pulseShader}
	goldenGlow = &golib.Material{Color: golib.Color{R: 90, G: 60, B: 10, A: 255}, Roughness: 0.1, Metallic: 0.6,
		Emission: goldenColor, EmissionStrength: 7, Shader: pulseShader}
	// droplet is the spray the skirt throws up: glowing a little, so it
	// stays white on the side away from the sun. (Unlit white comes out
	// grey under the Environment's Exposure.)
	droplet = &golib.Material{Color: golib.Black, Emission: sprayColor, EmissionStrength: 1.3}

	// foam is the wake, from see-through to almost solid: a Material has
	// one color, and DrawMeshWith takes no tint, so each step of the fade is
	// a material of its own.
	foam = foamMaterials()

	// The halos round the crystals: faint and see-through, glowing.
	crystalHalo = &golib.Material{Color: golib.Color{A: 40}, Emission: crystalColor, EmissionStrength: 1.2}
	goldenHalo  = &golib.Material{Color: golib.Color{A: 40}, Emission: goldenColor, EmissionStrength: 1.2}

	sparkCyan = &golib.Material{Color: golib.Black, Emission: crystalColor, EmissionStrength: 5}
	sparkGold = &golib.Material{Color: golib.Black, Emission: goldenColor, EmissionStrength: 5}
)

const waterShader = `
void surface(inout Surface s) {
    vec2 p = s.position.xz;
    float t = s.time;
    vec2 slope = vec2(0.0);
    slope += 0.050 * vec2(0.8, 0.6) * cos(dot(p, vec2(0.8, 0.6)) * 0.9 + t * 1.1);
    slope += 0.035 * vec2(-0.5, 0.86) * cos(dot(p, vec2(-0.5, 0.86)) * 1.7 + t * 1.6);
    slope += 0.020 * vec2(0.97, -0.24) * cos(dot(p, vec2(0.97, -0.24)) * 3.1 + t * 2.3);
    slope += 0.012 * vec2(-0.7, -0.7) * cos(dot(p, vec2(-0.7, -0.7)) * 5.3 + t * 3.1);
    float fade = 1.0 - smoothstep(60.0, 220.0, length(p));
    slope *= fade;
    s.normal = normalize(vec3(-slope.x, 1.0, -slope.y));
}`

const pulseShader = `
void surface(inout Surface s) {
    float pulse = 0.75 + 0.25 * sin(s.time * 4.0 + s.position.x * 0.7 + s.position.z * 0.5);
    s.emission *= pulse;
}`

// foamFades is how many steps the wake fades through.
const foamFades = 10

func foamMaterials() []*golib.Material {
	materials := make([]*golib.Material, foamFades)
	for i := range materials {
		opacity := float32(i+1) / foamFades
		materials[i] = &golib.Material{Color: golib.WithOpacity(golib.Black, 0.45*opacity), Emission: sprayColor, EmissionStrength: 1.2}
	}
	return materials
}
