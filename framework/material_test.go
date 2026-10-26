package golib

import (
	"image/color"
	"strings"
	"testing"
)

func TestMaterialsInAWindow(t *testing.T) {
	useAssets(t, map[string][]byte{"textures/green.png": onePixelPNG(t, color.NRGBA{G: 200, A: 255})})
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	box := NewBoxMesh()
	dark := Environment{SunColor: Black, Sky: Black, Ground: Black, NoBloom: true}
	middle := func(environment Environment, material *Material) color.NRGBA {
		return capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(environment)
			screen.DrawMeshWith(box, Transform{Scale: Vector3{X: 2, Y: 2, Z: 2}}, material)
			screen.End3D()
		}).NRGBAAt(32, 32)
	}

	if c := middle(dark, &Material{Color: Red}); c.R > 10 {
		t.Errorf("a red box in the dark is %v, want black", c)
	}
	if c := middle(dark, &Material{Color: Black, Emission: Orange}); c.R < 200 || c.G < 120 {
		t.Errorf("an orange glowing box in the dark is %v, want orange", c)
	}
	if c := middle(dark, &Material{Color: Color{R: 10, G: 120, B: 230, A: 255}, Unlit: true}); c.R > 14 || c.G < 115 || c.G > 125 || c.B < 225 {
		t.Errorf("an unlit box is %v, want its color as it is, 10, 120, 230", c)
	}
	if c := middle(dark, &Material{Texture: NewTexture("textures/green.png"), Unlit: true, Tiling: 4}); c.G < 195 || c.R > 4 {
		t.Errorf("an unlit textured box is %v, want the texture's green", c)
	}
	painted := &Material{Unlit: true, Shader: `
void surface(inout Surface s) {
    s.color = vec4(0.0, 0.0, 0.5, 1.0);
}`}
	// 0.5 in linear light is 186 in sRGB: below what the tonemapper softens.
	if c := middle(dark, painted); c.B < 180 || c.B > 192 || c.R > 4 || c.G > 4 {
		t.Errorf("a box whose shader paints it blue is %v", c)
	}
	// A field changed shows at the next draw.
	painted.Shader = ""
	painted.Color = Color{R: 200, A: 255}
	if c := middle(dark, painted); c.R < 196 || c.R > 204 || c.B > 4 {
		t.Errorf("after its shader went and its color turned red, the box is %v", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}

	middle(dark, &Material{Shader: "void surface(inout Surface s) { s.color = nothing; }"})
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "Material's Shader doesn't compile") {
		t.Errorf("a shader that doesn't compile reported %v", err)
	}
	middle(dark, &Material{Texture: NewTexture("textures/missing.png")})
	if err := takeError(); err == nil || !strings.Contains(err.Error(), `golib.NewTexture("textures/missing.png")`) {
		t.Errorf("a missing texture reported %v", err)
	}
}

// DrawMeshWithTint shades one Material differently each draw, and fades it.
func TestDrawMeshWithTint(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{}
	white := &Material{Color: White, Unlit: true}
	middle := func(tint Color) color.NRGBA {
		return capture(func() {
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{NoBloom: true})
			screen.DrawMeshWithTint(NewBoxMesh(), Transform{Scale: Vector3{X: 2, Y: 2, Z: 2}}, white, tint)
			screen.End3D()
		}).NRGBAAt(32, 32)
	}
	if c := middle(Color{R: 200, G: 40, B: 40, A: 255}); c.R < 190 || c.G > 50 {
		t.Errorf("a white material tinted red is %v, want red", c)
	}
	if c := middle(Color{R: 255, G: 255, B: 255, A: 0}); c.A > 5 {
		t.Errorf("a white material tinted to nothing is %v, want see-through", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}
