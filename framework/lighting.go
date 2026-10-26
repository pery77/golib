package golib

import (
	"math"
	"slices"
)

// Environment is how a 3D pass is lit, and what shows behind it: the sun,
// the light of the sky, fog and how bright the picture is. Every field left
// at its zero value keeps GoLib's default, so a game sets only what it wants:
//
//	screen.Begin3D(s.camera)
//	screen.SetEnvironment(golib.Environment{
//		Sky: golib.SkyBlue, Horizon: golib.RayWhite, Ground: golib.DarkGreen, SkyBackground: true,
//		FogColor: golib.RayWhite, FogDistance: 120,
//	})
//
// Colors are given as everywhere in GoLib; the 3D lights in linear light and
// brings the picture back with a tonemapper that leaves a lit color as it is
// and softens only what is brighter than white.
type Environment struct {
	// SunDirection is the way the sunlight travels: straight down is
	// golib.Vector3{Y: -1}. Zero: high behind and to the right of a camera
	// facing +Z, or, with an HDRI, from its picture's sun.
	SunDirection Vector3

	// SunColor is the sunlight's color. The zero Color is a soft white, or,
	// with an HDRI, its picture's sun, as bright as it is there, or none
	// for a picture without one. Black, with A 255, is no sun at all, for a
	// night lit by lamps.
	SunColor Color

	// SunStrength multiplies the sunlight: 1, as 0 counts, shows a color
	// facing the sun as it is; 2 or 3 is a glaring desert.
	SunStrength float32

	// Sky and Ground are the soft light that comes from everywhere above and
	// below: the sky's, and the ground's bouncing the sun back. A surface's
	// side away from the sun is lit by them. Zero: greys that show such a
	// side at about half its brightness.
	Sky, Ground Color

	// SkyBackground draws a sky behind everything: Sky overhead, fading to
	// Horizon at eye level, and Ground below it. Without it, what Draw drew
	// before Begin3D shows behind the 3D, such as Screen.Clear's color.
	SkyBackground bool

	// Horizon is the sky's color at eye level, with SkyBackground. Zero: a
	// paler Sky.
	Horizon Color

	// HDRI lights the world as the place in the panorama would, with soft
	// light from all around and reflections in shiny things, in place of Sky
	// and Ground; with SkyBackground, it shows behind everything. See
	// [NewHDRI].
	HDRI *HDRI

	// HDRITurn turns the HDRI's panorama round, in degrees, as Yaw turns a
	// model: a larger one turns it left. Its sun turns with it. The middle
	// of the picture faces +Z before it is turned.
	HDRITurn float32

	// FogColor and FogDistance fade the world into fog: things FogDistance
	// meters away are gone in it, and the fog starts at a tenth of that. 0
	// is no fog. A FogColor like the background's hides where the world
	// ends.
	FogColor    Color
	FogDistance float32

	// Haze is air that catches the light, lit where the sun and the lamps
	// reach it and dark in their shadows, so light through a window shows
	// as shafts and a spotlight as a beam: how much of the light it takes
	// in a meter, 0.01 for a hazy day, 0.05 for a dusty hall, 0.2 for
	// smoke. 0 is none. HazeColor tints the light it gives back, white
	// when zero, and HazeHeight keeps it below a height, thinning out over
	// a meter or a quarter of the height above it, for mist on a lake or
	// smoke in a cellar: 0 is everywhere. Its shadows reach as far as
	// ShadowDistance. It covers what is solid, the sky and water, not what
	// is see-through or particles.
	Haze       float32
	HazeColor  Color
	HazeHeight float32

	// Exposure brightens or darkens the whole picture, as a camera's would:
	// 1, as 0 counts, is as it is; 2 is twice as bright.
	Exposure float32

	// The sun casts shadows, from everything solid onto everything, up to
	// ShadowDistance meters from the camera: 0 counts as 50. Shadows are
	// sharpest near the camera. NoShadows turns them off, for a game that
	// needs every frame it can get, or wants the flat look.
	ShadowDistance float32
	NoShadows      bool

	// Bloom is how strongly what is brighter than white glows: a glowing
	// material, the sun off something shiny, a lamp close to a wall. 0
	// counts as 0.5; ordinary lit colors never glow. NoBloom turns it off.
	Bloom   float32
	NoBloom bool

	// Blur softens the whole 3D picture by about this many pixels, as a
	// camera out of focus sees it: for a menu, a map or a pause screen over
	// a world that goes on behind it, which the 2D drawn after End3D stays
	// sharp over. 0 is none; 8 is soft, 32 strong. It comes in steps, each
	// twice the one before, and costs about as little as Bloom.
	Blur float32

	// Smooth surfaces, their roughness below 0.35, reflect what
	// the pass shows where they can see it on the screen, fading out as
	// they roughen, and the sky, the HDRI or the level's reflection probes
	// elsewhere: a wet street shows the lamps, a polished floor the walls.
	// They don't reflect each other, nor what is off the screen or behind
	// something. NoScreenReflections turns them off, for a game that needs
	// every frame it can get.
	NoScreenReflections bool
}

// SetEnvironment sets how the 3D pass begun last is lit, and what shows
// behind it. Every pass starts with the zero Environment, GoLib's defaults;
// call it after Begin3D, before End3D.
func (s *Screen) SetEnvironment(environment Environment) {
	if !s.in3D("SetEnvironment") {
		return
	}
	s.pass.environment = environment
}

// Light is a lamp in the 3D world: a point that lights all around it, or,
// given a Direction, a spotlight. Add lights to a pass with Screen.AddLight:
//
//	screen.AddLight(golib.Light{Position: torch.Position, Color: golib.Orange, Intensity: 3, Range: 8})
type Light struct {
	// Position is where the light is.
	Position Vector3

	// Color is the light's color. The zero Color is white.
	Color Color

	// Intensity is how bright it is: 1, as 0 counts, lights what faces it
	// from a meter away as brightly as the sun does; it fades with the
	// square of the distance, as real light does.
	Intensity float32

	// Range is how far it reaches, in meters, fading out before it: what is
	// farther isn't lit by it at all. 0 counts as 10.
	Range float32

	// Direction makes a spotlight that points this way, lighting a cone
	// Cone degrees across. Zero: a point light, lighting all around.
	Direction Vector3

	// Cone is a spotlight's width, in degrees, softening at its edge. 0
	// counts as 45.
	Cone float32

	// Shadows makes the lamp cast shadows from everything solid, as the sun
	// does. A spotlight's cost one more drawing of the world a frame, a
	// point light's six: a pass gives shadows to the 4 nearest lamps that
	// ask for them.
	Shadows bool
}

// maxLights is how many lights a pass uses: the nearest to the camera, when
// a pass adds more.
const maxLights = 8

// AddLight adds a light to the 3D pass begun last: it lights what the pass
// draws, all of it, whenever it is added before End3D. A pass uses the 8
// lights nearest to the camera, and leaves the rest out.
func (s *Screen) AddLight(light Light) {
	if !s.in3D("AddLight") {
		return
	}
	s.pass.lights = append(s.pass.lights, light)
}

// lightUniforms are a pass's lights as the lit shader reads them: four
// vec4 for each of up to maxLights.
type lightUniforms struct {
	count                         int32
	positions, colors, directions []float32
	chosen                        []Light // the lights used, in the shader's order
}

// lightsFor turns lights into what the lit shader reads, keeping the
// nearest to eye.
func lightsFor(lights []Light, eye Vector3) lightUniforms {
	if len(lights) > maxLights {
		lights = slices.Clone(lights)
		slices.SortStableFunc(lights, func(a, b Light) int {
			da, db := a.Position.Distance(eye), b.Position.Distance(eye)
			switch {
			case da < db:
				return -1
			case da > db:
				return 1
			}
			return 0
		})
		lights = lights[:maxLights]
	}
	var u lightUniforms
	u.count = int32(len(lights))
	u.chosen = lights
	for _, l := range lights {
		reach := l.Range
		if reach <= 0 {
			reach = 10
		}
		intensity := l.Intensity
		if intensity <= 0 {
			intensity = 1
		}
		color := [4]float32{1, 1, 1}
		if l.Color != (Color{}) {
			color = linearColor(l.Color)
		}
		u.positions = append(u.positions, l.Position.X, l.Position.Y, l.Position.Z, reach)
		u.colors = append(u.colors, color[0]*intensity, color[1]*intensity, color[2]*intensity, 0)
		// A point light's direction says -2: a cone wider than any angle.
		direction, inner, outer := Vector3{}, float32(-2), float32(-2)
		if d := l.Direction.Normalize(); d != (Vector3{}) {
			cone := l.Cone
			if cone <= 0 {
				cone = 45
			}
			half := float64(min(cone, 179)) / 2 * math.Pi / 180
			direction = d
			outer = float32(math.Cos(half))
			inner = float32(math.Cos(half * 0.8))
		}
		u.directions = append(u.directions, direction.X, direction.Y, direction.Z, inner)
		u.colors[len(u.colors)-1] = outer
	}
	return u
}

// environmentUniforms are an Environment, with its defaults, as the shaders
// read it: linear light.
type environmentUniforms struct {
	sunDirection      Vector3
	sun               [4]float32
	sky, ground       [4]float32
	horizon, skyTop   [4]float32 // the background's colors, linear
	skyBottom         [4]float32
	background        bool
	fog               [4]float32 // color, then its distance in w; 0 for none
	haze              [4]float32 // color, then how much it takes in a meter; 0 for none
	hazeHeight        float32
	exposure          float32
	shadows           bool
	shadowDistance    float32
	hdri              *HDRI
	hdriTurn          [4]float32 // the turn's cosine and sine, to turn a direction back onto the picture
	bloom             float32    // 0 for none
	blur              int        // halvings the picture is softened by; 0 for none
	screenReflections bool
}

// The default light: a sun high behind and to the right of a camera facing
// +Z, as NewCamera3D's does and a third-person camera behind a player with no
// Yaw does, and the soft light of the sky and the ground, in linear light. A
// surface facing the sun shows its color as it is; the side away from it, at
// about half its brightness.
var (
	defaultSunDirection = Vector3{X: 0.4, Y: -1, Z: 0.6}.Normalize()
	defaultSun          = [4]float32{0.9, 0.9, 0.9}
	defaultSky          = [4]float32{0.3, 0.3, 0.3}
	defaultGround       = [4]float32{0.12, 0.12, 0.12}
)

// ambientShare is how much of the Sky and Ground colors a game gives lights
// what the sun doesn't: the sky is far brighter to look at than the light it
// sheds.
const ambientShare = 0.4

func (e Environment) uniforms() environmentUniforms {
	u := environmentUniforms{
		sunDirection: defaultSunDirection, sun: defaultSun,
		sky: defaultSky, ground: defaultGround, exposure: 1,
		background: e.SkyBackground,
	}
	if h := e.HDRI; h != nil && h.Err() == nil {
		// The picture's sun, taken out of its light, shines from where it
		// was; a picture without one has no sun.
		turn := Rotation{Yaw: e.HDRITurn}
		u.sun = h.sun
		if h.toSun != (Vector3{}) {
			u.sunDirection = h.toSun.Rotate(turn).Scale(-1)
		}
		radians := float64(e.HDRITurn) * math.Pi / 180
		u.hdriTurn = [4]float32{float32(math.Cos(radians)), float32(math.Sin(radians))}
	}
	if d := e.SunDirection.Normalize(); d != (Vector3{}) {
		u.sunDirection = d
	}
	if e.SunColor != (Color{}) {
		u.sun = linearColor(e.SunColor)
	}
	if e.SunStrength > 0 {
		for i := range 3 {
			u.sun[i] *= e.SunStrength
		}
	}
	share := func(c Color) [4]float32 {
		l := linearColor(c)
		return [4]float32{l[0] * ambientShare, l[1] * ambientShare, l[2] * ambientShare}
	}
	if e.Sky != (Color{}) {
		u.sky = share(e.Sky)
	}
	if e.Ground != (Color{}) {
		u.ground = share(e.Ground)
	}
	top, bottom := linearColor(e.Sky), linearColor(e.Ground)
	if e.Sky == (Color{}) {
		top = linearColor(SkyBlue)
	}
	if e.Ground == (Color{}) {
		bottom = linearColor(DarkGray)
	}
	horizon := linearColor(e.Horizon)
	if e.Horizon == (Color{}) {
		for i := range 3 {
			horizon[i] = top[i] + (1-top[i])*0.6
		}
	}
	u.skyTop, u.horizon, u.skyBottom = top, horizon, bottom
	if e.FogDistance > 0 {
		fog := linearColor(e.FogColor)
		if e.FogColor == (Color{}) {
			fog = horizon
		}
		u.fog = [4]float32{fog[0], fog[1], fog[2], e.FogDistance}
	}
	if e.Haze > 0 {
		tint := [4]float32{1, 1, 1}
		if e.HazeColor != (Color{}) {
			tint = linearColor(e.HazeColor)
		}
		u.haze = [4]float32{tint[0], tint[1], tint[2], e.Haze}
		u.hazeHeight = max(e.HazeHeight, 0)
	}
	if e.Exposure > 0 {
		u.exposure = e.Exposure
	}
	u.hdri = e.HDRI
	if !e.NoBloom {
		u.bloom = 0.5
		if e.Bloom > 0 {
			u.bloom = e.Bloom
		}
	}
	u.blur = blurLevels(e.Blur)
	u.screenReflections = !e.NoScreenReflections
	u.shadows = !e.NoShadows && (u.sun[0] > 0 || u.sun[1] > 0 || u.sun[2] > 0)
	u.shadowDistance = 50
	if e.ShadowDistance > 0 {
		u.shadowDistance = e.ShadowDistance
	}
	return u
}
