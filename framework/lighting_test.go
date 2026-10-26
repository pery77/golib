package golib

import (
	"strings"
	"testing"
)

func TestEnvironmentDefaults(t *testing.T) {
	zero := Environment{}.uniforms()
	if zero.sun != defaultSun || zero.sky != defaultSky || zero.ground != defaultGround || zero.exposure != 1 {
		t.Errorf("the zero Environment gives %+v, want GoLib's defaults", zero)
	}
	if zero.fog[3] != 0 || zero.background {
		t.Error("the zero Environment has fog or a sky background")
	}
	night := Environment{SunColor: Black, Exposure: 2, FogDistance: 50}.uniforms()
	if night.sun != [4]float32{0, 0, 0, 1} {
		t.Errorf("SunColor Black gives a sun of %v, want none", night.sun)
	}
	if night.exposure != 2 || night.fog[3] != 50 || night.fog[0] != night.horizon[0] {
		t.Errorf("fog %v, exposure %v: want 50 meters of fog in the horizon's color, and exposure 2", night.fog, night.exposure)
	}
	strong := Environment{SunStrength: 2}.uniforms()
	if strong.sun[0] != defaultSun[0]*2 {
		t.Errorf("SunStrength 2 gives a sun of %v, want twice the default", strong.sun)
	}
}

func TestLightsKeepTheNearest(t *testing.T) {
	var lights []Light
	for i := range 12 {
		lights = append(lights, Light{Position: Vector3{X: float32(i)}})
	}
	u := lightsFor(lights, Vector3{X: 20})
	if u.count != maxLights {
		t.Fatalf("%d lights kept, want %d", u.count, maxLights)
	}
	for i := 0; i < int(u.count); i++ {
		if x := u.positions[i*4]; x < 4 {
			t.Errorf("light at X %v kept, but 8 others are nearer to the camera at X 20", x)
		}
	}
	spot := lightsFor([]Light{{Direction: Vector3{Y: -2}, Cone: 60, Intensity: 3, Range: 5}}, Vector3{})
	if spot.positions[3] != 5 || spot.colors[0] != 3 || spot.directions[1] != -1 {
		t.Errorf("a spotlight became %v, %v, %v", spot.positions, spot.colors, spot.directions)
	}
	if outer := spot.colors[3]; outer < 0.86 || outer > 0.87 {
		t.Errorf("a 60 degree cone's edge is at cosine %v, want cos(30) = 0.866", outer)
	}
	if point := lightsFor([]Light{{}}, Vector3{}); point.colors[3] != -2 || point.positions[3] != 10 {
		t.Errorf("a zero Light became %v, %v, want a point light reaching 10 meters", point.positions, point.colors)
	}
}

func TestLightingInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	box := NewBoxMesh()
	brightness := func(draw func()) int {
		picture := capture(func() {
			screen.Begin3D(camera)
			draw()
			screen.DrawMesh(box, Transform{Scale: Vector3{X: 2, Y: 2, Z: 0.2}}, Gray)
			screen.End3D()
		})
		c := picture.NRGBAAt(32, 32)
		return int(c.R) + int(c.G) + int(c.B)
	}
	night := Environment{SunColor: Black, Sky: Color{R: 10, G: 10, B: 10, A: 255}, Ground: Color{R: 10, G: 10, B: 10, A: 255}}
	dark := brightness(func() { screen.SetEnvironment(night) })
	lit := brightness(func() {
		screen.SetEnvironment(night)
		screen.AddLight(Light{Position: Vector3{Z: -1.5}, Intensity: 2})
	})
	if dark > 30 || lit < dark+100 {
		t.Errorf("a grey wall is %d bright at night and %d with a lamp before it, want dark, then lit", dark, lit)
	}

	// The sky shows where nothing is drawn, and fog fades what is far.
	sky := capture(func() {
		screen.Begin3D(camera)
		screen.SetEnvironment(Environment{Sky: Blue, Horizon: Blue, Ground: Blue, SkyBackground: true})
		screen.DrawMesh(box, Transform{}, Red)
		screen.End3D()
	})
	if c := sky.NRGBAAt(2, 2); c.B < 150 || c.R > 40 || c.A != 255 {
		t.Errorf("the corner is %v, want the blue sky", c)
	}
	if c := sky.NRGBAAt(32, 32); c.R < c.B {
		t.Errorf("the middle is %v, want the red box in front of the sky", c)
	}
	far := Vector3{Z: 60}
	fogged := capture(func() {
		screen.Begin3D(camera)
		screen.SetEnvironment(Environment{FogColor: White, FogDistance: 30})
		screen.DrawMesh(box, Transform{Position: far, Scale: Vector3{X: 80, Y: 80, Z: 1}}, Black)
		screen.End3D()
	})
	if c := fogged.NRGBAAt(32, 32); c.R < 200 {
		t.Errorf("a black wall twice the fog's distance away is %v, want gone in the white fog", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}

	capture(func() { screen.AddLight(Light{}) })
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "Begin3D") {
		t.Errorf("AddLight outside a pass reported %v", err)
	}
}

func TestCascadesCoverTheView(t *testing.T) {
	camera := NewCamera3D(800, 600)
	c := cascadesFor(camera, Vector3{X: 0.3, Y: -1}.Normalize(), 50)
	for k := 1; k < shadowCascades; k++ {
		if c.splits[k] <= c.splits[k-1] || c.texels[k] < c.texels[k-1] {
			t.Errorf("slice %d ends at %v with texels of %v, after %v and %v: want each farther and coarser", k, c.splits[k], c.texels[k], c.splits[k-1], c.texels[k-1])
		}
	}
	if c.splits[shadowCascades-1] < 49.9 || c.splits[shadowCascades-1] > 50.1 {
		t.Errorf("the last slice ends at %v, want the shadow distance, 50", c.splits[shadowCascades-1])
	}
	// What the camera looks at is inside the nearest slice's view.
	p := c.viewProjections[0].transformPoint(camera.Target)
	if p.X < -1 || p.X > 1 || p.Y < -1 || p.Y > 1 || p.Z < -1 || p.Z > 1 {
		t.Errorf("the camera's target is at %v in the first slice, want inside it", p)
	}
}

func TestShadowsInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 6, Z: -0.01}, Vector3{} // looking down
	ground, box := NewPlaneMesh(), NewBoxMesh()
	scene := func(environment Environment) func() {
		return func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(environment)
			screen.DrawMesh(ground, Transform{Scale: Vector3{X: 20, Z: 20}}, White)
			screen.DrawMesh(box, Transform{Position: Vector3{Y: 2}, Scale: Vector3{X: 0.6, Y: 0.2, Z: 0.6}}, White)
			screen.End3D()
		}
	}
	// A sun from the side casts the box's shadow 2 meters across the
	// ground, where the camera, looking down, sees it beside the box.
	slant := Vector3{X: 1, Y: -1}
	shaded := capture(scene(Environment{SunDirection: slant}))
	plain := capture(scene(Environment{SunDirection: slant, NoShadows: true}))
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	brightness := func(x, y int, img func(int, int) (uint8, uint8, uint8)) int {
		r, g, b := img(x, y)
		return int(r) + int(g) + int(b)
	}
	edge := func(x, y int) (uint8, uint8, uint8) { c := shaded.NRGBAAt(x, y); return c.R, c.G, c.B }
	flat := func(x, y int) (uint8, uint8, uint8) { c := plain.NRGBAAt(x, y); return c.R, c.G, c.B }
	// The far corner is in sun either way.
	if brightness(2, 2, edge) < brightness(2, 2, flat)-12 {
		t.Errorf("the far corner is %d with shadows and %d without, want both in full sun", brightness(2, 2, edge), brightness(2, 2, flat))
	}
	dark := 0
	for x := 0; x < 64; x++ {
		if brightness(x, 32, edge) < brightness(x, 32, flat)-60 {
			dark++
		}
	}
	if dark == 0 {
		t.Error("no pixel across the middle is darker with shadows: the box casts none")
	}
}

// At night, a lamp over a box casts its shadow on the ground under it, a
// point light and a spotlight alike, and only when it asks to.
func TestLampShadowsInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 6, Z: -0.01}, Vector3{} // looking down
	ground, box := NewPlaneMesh(), NewBoxMesh()
	night := Environment{SunColor: Black, Sky: Black, Ground: Black, NoBloom: true}
	middle := func(lamp Light) int {
		picture := capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(night)
			screen.AddLight(lamp)
			screen.DrawMesh(ground, Transform{Scale: Vector3{X: 20, Z: 20}}, White)
			// A slab between the lamp and the ground, beside the camera.
			screen.DrawMesh(box, Transform{Position: Vector3{X: 1.5, Y: 1}, Scale: Vector3{X: 1, Y: 0.1, Z: 1}}, White)
			screen.End3D()
		})
		// The ground right under the camera, where the slab's shadow from
		// the lamp falls, between X -1 and 1.
		c := picture.NRGBAAt(32, 32)
		return int(c.R) + int(c.G) + int(c.B)
	}
	for _, kind := range []struct {
		name string
		lamp Light
	}{
		{"point", Light{Position: Vector3{X: 3, Y: 2}, Intensity: 6, Range: 20}},
		{"spot", Light{Position: Vector3{X: 3, Y: 2}, Intensity: 6, Range: 20, Direction: Vector3{X: -1, Y: -0.8}, Cone: 120}},
	} {
		lit := middle(kind.lamp)
		kind.lamp.Shadows = true
		shaded := middle(kind.lamp)
		if lit < 100 {
			t.Errorf("%s: the ground the lamp lights is %d bright, too dark to see a shadow", kind.name, lit)
		}
		if shaded > lit/3 {
			t.Errorf("%s: the ground behind the slab from the lamp is %d bright with Shadows and %d without, want in shadow", kind.name, shaded, lit)
		}
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
