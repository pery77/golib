package golib

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"

	"golib/internal/gltf"
)

// onePixelPNG returns a PNG of a single pixel of c.
func onePixelPNG(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// levelGLB makes a level as Blender would export it: a 2 meter square floor,
// facing +Z and unlit, painted with a one-pixel texture, inside a node moved
// up by 1, and two empties with custom properties.
func levelGLB(t *testing.T, paint color.NRGBA) []byte {
	b := gltf.NewBuilder()
	image := b.Add("images", map[string]any{"mimeType": "image/png", "bufferView": b.View(onePixelPNG(t, paint))})
	texture := b.Add("textures", map[string]any{"source": image, "sampler": b.Add("samplers", map[string]any{"magFilter": 9728})})
	material := b.Add("materials", map[string]any{
		"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": texture}},
		"extensions":           map[string]any{"KHR_materials_unlit": map[string]any{}},
	})
	mesh := b.Add("meshes", map[string]any{"primitives": []any{map[string]any{
		"attributes": map[string]any{
			"POSITION":   b.Floats([]float32{-1, -1, 0, 1, -1, 0, 1, 1, 0, -1, 1, 0}, 3),
			"NORMAL":     b.Floats([]float32{0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1}, 3),
			"TEXCOORD_0": b.Floats([]float32{0, 1, 1, 1, 1, 0, 0, 0}, 2),
		},
		"indices": b.Indices([]uint16{0, 1, 2, 0, 2, 3}), "material": material,
	}}})
	floor := b.Add("nodes", map[string]any{"name": "Floor", "mesh": mesh})
	start := b.Add("nodes", map[string]any{"name": "Start", "translation": []float32{0, 0, 3},
		"rotation": []float32{0, float32(math.Sin(math.Pi / 4)), 0, float32(math.Cos(math.Pi / 4))}})
	enemy := b.Add("nodes", map[string]any{"name": "Enemy", "translation": []float32{-2, 0, 0},
		"extras": map[string]any{"kind": "slime", "speed": 1.5}})
	bone := b.Add("nodes", map[string]any{"name": "Bone"})
	b.Add("skins", map[string]any{"joints": []any{bone}})
	b.Root(b.Add("nodes", map[string]any{"name": "Level", "translation": []float32{0, 1, 0}, "children": []any{floor, start, enemy, bone}}))
	return b.GLB()
}

func TestModelMarkersAndBounds(t *testing.T) {
	useAssets(t, map[string][]byte{"models/level.glb": levelGLB(t, color.NRGBA{G: 255, A: 255})})
	level := NewModel("models/level.glb")
	if err := level.Err(); err != nil {
		t.Fatal(err)
	}
	lowest, upmost := level.Bounds()
	if !near3(lowest, Vector3{X: -1, Y: 0}) || !near3(upmost, Vector3{X: 1, Y: 2}) {
		t.Errorf("Bounds = %v, %v, want the floor raised by its parent: -1, 0, 0 to 1, 2, 0", lowest, upmost)
	}
	markers := level.Markers()
	if len(markers) != 3 || markers[0].Name != "Level" || markers[1].Name != "Start" || markers[2].Name != "Enemy" {
		t.Fatalf("Markers = %+v, want the level, Start and Enemy, in the file's order", markers)
	}
	start, ok := level.Marker("Start")
	if !ok || !near3(start.Position, Vector3{Y: 1, Z: 3}) || math.Abs(float64(start.Rotation.Yaw-90)) > 0.01 || !near3(start.Scale, Vector3{X: 1, Y: 1, Z: 1}) {
		t.Errorf("Start = %+v, want at 0, 1, 3, facing yaw 90", start)
	}
	enemy, _ := level.Marker("Enemy")
	if enemy.Properties.String("kind") != "slime" || enemy.Properties.Float("speed") != 1.5 {
		t.Errorf("Enemy's properties = %v, want Blender's custom properties", enemy.Properties)
	}
	if _, ok := level.Marker("Exit"); ok {
		t.Error(`Marker("Exit") found a marker the file doesn't have`)
	}
}

func TestModelMistakes(t *testing.T) {
	useAssets(t, map[string][]byte{
		"models/squashed.glb": []byte(`{"asset":{"version":"2.0"},"extensionsRequired":["KHR_draco_mesh_compression"]}`),
		"models/empty.gltf":   []byte(`{"asset":{"version":"2.0"},"scenes":[{"nodes":[]}]}`),
	})
	for name, want := range map[string]string{
		"models/missing.glb":  `golib.NewModel("models/missing.glb"): golib.ReadAsset: assets/models/missing.glb not found`,
		"models/squashed.glb": "turn off Compression in Blender's glTF export",
		"models/empty.gltf":   "nothing in its scene",
	} {
		if err := NewModel(name).Err(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("NewModel(%q).Err() = %v, want it to say %q", name, err, want)
		}
	}
}

func TestRotationTurnAndChild(t *testing.T) {
	// A ball rolling towards +Z turns around +X: its top goes forward.
	rolled := Rotation{}.Turn(Vector3{Y: 1}.Cross(Vector3{Z: 1}), 90)
	if got := rolled.Up(); !near3(got, Vector3{Z: 1}) {
		t.Errorf("rolled a quarter turn towards +Z, the top points %v, want +Z", got)
	}
	// Turns pile up in any order the axes ask, which three angles can't
	// say on their own.
	r := Rotation{}
	for range 4 {
		r = r.Turn(Vector3{X: 1}, 90)
	}
	if !near3(r.Forward(), Vector3{Z: 1}) || !near3(r.Up(), Vector3{Y: 1}) {
		t.Errorf("four quarter turns came back facing %v, up %v, want where they started", r.Forward(), r.Up())
	}
	mixed := Rotation{Yaw: 30, Pitch: 20}.Turn(Vector3{X: 1, Y: 1}, 50)
	matrix := Rotation{Yaw: 30, Pitch: 20}.matrix()
	_ = matrix
	if got := mixed.matrix().transformDirection(Vector3{Z: 1}); !near3(got, mixed.Forward()) {
		t.Errorf("a turned Rotation's matrix and its Forward disagree: %v and %v", got, mixed.Forward())
	}

	tank := Transform{Position: Vector3{X: 5}, Rotation: Rotation{Yaw: 90}, Scale: Vector3{X: 2, Y: 2, Z: 2}}
	turret := tank.Child(Transform{Position: Vector3{Y: 1, Z: 1}, Rotation: Rotation{Pitch: 10}})
	// One meter up and one ahead, at the tank's scale of 2, ahead being +X.
	if !near3(turret.Position, Vector3{X: 7, Y: 2}) {
		t.Errorf("the turret is at %v, want 7, 2, 0", turret.Position)
	}
	if math.Abs(float64(turret.Rotation.Yaw-90)) > 0.01 || math.Abs(float64(turret.Rotation.Pitch-10)) > 0.01 || !near3(turret.Scale, Vector3{X: 2, Y: 2, Z: 2}) {
		t.Errorf("the turret is %+v, scale %v, want the tank's yaw with its own pitch, at the tank's scale", turret.Rotation, turret.Scale)
	}
}

func TestDecomposeUndoesTransform(t *testing.T) {
	for _, tr := range []Transform{
		{Position: Vector3{X: 1, Y: 2, Z: 3}, Rotation: Rotation{Yaw: 40, Pitch: -30, Roll: 20}, Scale: Vector3{X: 2, Y: 3, Z: 4}},
		{Rotation: Rotation{Yaw: -170, Pitch: 89, Roll: 0}},
		{Rotation: Rotation{Yaw: 10, Pitch: 0, Roll: 175}},
	} {
		position, rotation, scale := tr.matrix().decompose()
		again := Transform{Position: position, Rotation: rotation, Scale: scale}.matrix()
		want := tr.matrix()
		for i := range want {
			if math.Abs(float64(again[i]-want[i])) > 1e-3 {
				t.Errorf("%+v decomposed into %v, %+v, %v, which makes another matrix", tr, position, rotation, scale)
				break
			}
		}
	}
}

func TestFrustumSees(t *testing.T) {
	camera := NewCamera3D(800, 600)
	view, projection := camera.matrices()
	sight := frustumOf(projection.mul(view))
	for _, c := range []struct {
		center Vector3
		radius float32
		want   bool
	}{
		{Vector3{}, 1, true},              // what it looks at
		{Vector3{Y: 4, Z: -20}, 1, false}, // behind it
		{Vector3{X: 100}, 1, false},       // far to the side
		{Vector3{X: 100}, 200, true},      // far to the side, but large enough to reach in
		{Vector3{Z: 2000}, 1, false},      // past Far
	} {
		if got := sight.sees(c.center, c.radius); got != c.want {
			t.Errorf("sees a sphere at %v, radius %v: %v, want %v", c.center, c.radius, got, c.want)
		}
	}
}

func TestNewMesh(t *testing.T) {
	// A pyramid of four triangles sharing its top: the normals are worked
	// out, smooth, and the colors kept.
	vertices := []Vertex{
		{Position: Vector3{Y: 1}, Color: Red},
		{Position: Vector3{X: -1, Z: 1}}, {Position: Vector3{X: 1, Z: 1}},
		{Position: Vector3{X: 1, Z: -1}}, {Position: Vector3{X: -1, Z: -1}},
	}
	pyramid := NewMesh(vertices, []int{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 1})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	stride := 12
	top := pyramid.vertices[0:stride]
	if !near3(Vector3{X: top[3], Y: top[4], Z: top[5]}, Vector3{Y: 1}) {
		t.Errorf("the shared top's normal is %v, want straight up, the average of the four sides", top[3:6])
	}
	if top[8] < 0.7 || top[9] > 0.05 || top[11] != 1 {
		t.Errorf("the top's color is %v, want red in linear light, opaque", top[8:12])
	}
	if corner := pyramid.vertices[stride+8 : stride+12]; corner[0] != 1 || corner[3] != 1 {
		t.Errorf("a corner with no color is %v, want white", corner)
	}
	NewMesh(vertices, []int{0, 1})
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "three for each triangle") {
		t.Errorf("two indices reported %v", err)
	}
	NewMesh(vertices, []int{0, 1, 9})
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "vertex 9") {
		t.Errorf("a vertex that isn't there reported %v", err)
	}
}

func TestModelInAWindow(t *testing.T) {
	useAssets(t, map[string][]byte{"models/level.glb": levelGLB(t, color.NRGBA{G: 200, B: 40, A: 255})})
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 1, Z: 3}, Vector3{Y: 1}
	level := NewModel("models/level.glb")

	painted := capture(func() {
		screen.Begin3D(camera)
		screen.DrawModel(level, Transform{}, White)
		screen.End3D()
	})
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if c := painted.NRGBAAt(32, 32); c.R > 4 || c.G < 196 || c.G > 204 || c.B < 36 || c.B > 44 || c.A != 255 {
		t.Errorf("an unlit textured floor is %v in the middle, want its texture's 0, 200, 40, as it is", c)
	}
	tinted := capture(func() {
		screen.Begin3D(camera)
		screen.DrawModel(level, Transform{}, Color{R: 255, G: 128, B: 255, A: 255})
		screen.End3D()
	})
	if c := tinted.NRGBAAt(32, 32); c.G < 70 || c.G > 110 {
		t.Errorf("tinted half green, the floor is %v, want its green at about half its light", c)
	}

	// Many copies of a mesh are one draw; each keeps its own place and color.
	box := NewBoxMesh()
	camera.Position, camera.Target = Vector3{Z: -10}, Vector3{}
	many := capture(func() {
		screen.Begin3D(camera)
		for i := range 21 {
			color := Red
			if i == 10 {
				color = Blue
			}
			screen.DrawMesh(box, Transform{Position: Vector3{X: float32(i-10) * 1.5}}, color)
		}
		screen.End3D()
	})
	if c := many.NRGBAAt(32, 32); c.B < c.R {
		t.Errorf("the middle box of 21 is %v, want the one drawn blue", c)
	}
	if c := many.NRGBAAt(42, 32); c.R < c.B {
		t.Errorf("the box beside it is %v, want red", c)
	}

	// Shapes made in code draw too.
	cone := capture(func() {
		screen.Begin3D(camera)
		screen.DrawMesh(NewConeMesh(), Transform{Scale: Vector3{X: 4, Y: 4, Z: 4}}, Orange)
		screen.DrawMesh(NewCapsuleMesh(), Transform{Position: Vector3{X: -6}, Scale: Vector3{X: 4, Y: 4, Z: 4}}, Orange)
		screen.End3D()
	})
	if c := cone.NRGBAAt(32, 34); c.A == 0 {
		t.Error("the cone isn't drawn")
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}

// Hide hides an object and what is under it, wherever the model is drawn,
// until Show; a name the model doesn't have is reported.
func TestModelHideAndShow(t *testing.T) {
	useAssets(t, map[string][]byte{"models/level.glb": levelGLB(t, color.NRGBA{G: 255, A: 255})})
	level := NewModel("models/level.glb")
	if err := level.Err(); err != nil {
		t.Fatal(err)
	}
	floor := level.parts[0].node
	level.Hide("Level")
	if !level.partHidden(floor) {
		t.Error("hiding Level left the floor under it shown")
	}
	level.Show("Level")
	level.Hide("Floor")
	if !level.partHidden(floor) {
		t.Error("hiding Floor left it shown")
	}
	level.Show("Floor")
	if level.partHidden(floor) {
		t.Error("Show left the floor hidden")
	}
	takeError()
	level.Hide("Ceiling")
	if err := takeError(); err == nil || !strings.Contains(err.Error(), `Model.Hide("Ceiling")`) {
		t.Errorf("hiding a name the model doesn't have reported %v", err)
	}
}
