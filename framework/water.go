package golib

// Water is a surface of water, a lake, a sea, a river or a pool, drawn by
// Screen.DrawWater as a flat square a meter across, on X and Z, placed and
// scaled by a Transform: golib.Transform{Position: golib.Vector3{Y: -0.3},
// Scale: golib.Vector3{X: 80, Z: 80}} for a lake around an island. Its waves
// move in the light; what is under it shows through, bent by them and fading
// into its color as it deepens, with foam where it meets the shore; and on it
// shine the sun and what it reflects: the pass itself, where what it
// reflects is on the screen, the reflection probes and the sky elsewhere.
// It is drawn after the solid things, over them, and before what is
// see-through.
type Water struct {
	// Color is the water's color deep down, where the bottom no longer
	// shows: a blue-green when zero.
	Color Color

	// Clarity is how many meters down the bottom still shows through it,
	// fading: 2 when zero. A muddy river is 0.5, a tropical sea 6.
	Clarity float32

	// Waves is how big its waves are: 1, as 0 counts, is a lake in a
	// breeze, 3 a rough sea. Still makes it a mirror.
	Waves float32
	Still bool

	// NoFoam leaves out the foam where it is shallow.
	NoFoam bool
}

// waterDraw is water as a pass draws it.
type waterDraw struct {
	place   matrix
	color   [4]float32
	clarity float32
	waves   float32
	foam    bool
}

// waterPlane is the square every water is drawn as, and waterMaterial what
// the lit program reads for it: the water program works out the rest.
var (
	waterPlane    = NewPlaneMesh()
	waterMaterial = &material3D{baseColor: [4]float32{1, 1, 1, 1}, roughness: 0.02, twoSided: true}
)

// DrawWater draws water, a square a meter across on X and Z placed and
// scaled by transform, between Begin3D and End3D.
func (s *Screen) DrawWater(water Water, transform Transform) {
	if !s.in3D("DrawWater") {
		return
	}
	color := water.Color
	if color == (Color{}) {
		color = Color{R: 30, G: 90, B: 100, A: 255}
	}
	clarity := water.Clarity
	if clarity <= 0 {
		clarity = 2
	}
	waves := water.Waves
	if waves <= 0 {
		waves = 1
	}
	if water.Still {
		waves = 0
	}
	if transform.Scale == (Vector3{}) {
		transform.Scale = Vector3{X: 1, Y: 1, Z: 1}
	}
	s.pass.waters = append(s.pass.waters, waterDraw{
		place: transform.matrix(), color: linearColor(color), clarity: clarity, waves: waves, foam: !water.NoFoam,
	})
}
