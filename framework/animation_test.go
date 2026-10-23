package golib

import (
	"math"
	"strings"
	"testing"

	"golib/internal/gltf"
)

// barGLB makes a model as Blender would export a rigged one: an unlit white
// strip two meters high and 0.4 wide, facing +Z, bound to two bones, Root at
// its foot and Tip a meter up, with a marker, Hand, half a meter up Tip; a
// shape key, Wide, that spreads its top a meter each way; and animations:
// Bend turns Tip a quarter turn around +Z in a second, Wave a quarter turn
// the other way, Walk moves Root 2 meters along +Z in a second, and Grow
// takes Wide from 0 to 1.
func barGLB() []byte {
	b := gltf.NewBuilder()
	material := b.Add("materials", map[string]any{
		"pbrMetallicRoughness": map[string]any{"baseColorFactor": []float32{1, 1, 1, 1}},
		"extensions":           map[string]any{"KHR_materials_unlit": map[string]any{}},
	})
	mesh := b.Add("meshes", map[string]any{
		"name": "Bar",
		"primitives": []any{map[string]any{
			"attributes": map[string]any{
				"POSITION":  b.Floats([]float32{-0.2, 0, 0, 0.2, 0, 0, -0.2, 1, 0, 0.2, 1, 0, -0.2, 2, 0, 0.2, 2, 0}, 3),
				"NORMAL":    b.Floats([]float32{0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1}, 3),
				"JOINTS_0":  b.Joints([]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}),
				"WEIGHTS_0": b.Floats([]float32{1, 0, 0, 0, 1, 0, 0, 0, 0.5, 0.5, 0, 0, 0.5, 0.5, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}, 4),
			},
			"indices":  b.Indices([]uint16{0, 1, 3, 0, 3, 2, 2, 3, 5, 2, 5, 4}),
			"material": material,
			"targets":  []any{map[string]any{"POSITION": b.Floats([]float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -1, 0, 0, 1, 0, 0}, 3)}},
		}},
		"weights": []float32{0},
		"extras":  map[string]any{"targetNames": []any{"Wide"}},
	})
	hand := b.Add("nodes", map[string]any{"name": "Hand", "translation": []float32{0, 0.5, 0}})
	tip := b.Add("nodes", map[string]any{"name": "Tip", "translation": []float32{0, 1, 0}, "children": []any{hand}})
	root := b.Add("nodes", map[string]any{"name": "Root", "children": []any{tip}})
	body := b.Add("nodes", map[string]any{"name": "Body", "mesh": mesh, "skin": 0})
	b.Add("skins", map[string]any{"joints": []any{root, tip}, "inverseBindMatrices": b.Matrices([]float32{
		1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1,
		1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, -1, 0, 1,
	})})
	b.Root(b.Add("nodes", map[string]any{"name": "Armature", "children": []any{root, body}}))

	times := b.Floats([]float32{0, 1}, 1)
	half := float32(math.Sqrt(0.5))
	animation := func(name string, node int, path string, values []float32, comps int) {
		b.Add("animations", map[string]any{
			"name":     name,
			"channels": []any{map[string]any{"sampler": 0, "target": map[string]any{"node": node, "path": path}}},
			"samplers": []any{map[string]any{"input": times, "output": b.Floats(values, comps)}},
		})
	}
	animation("Bend", tip, "rotation", []float32{0, 0, 0, 1, 0, 0, half, half}, 4)
	animation("Wave", tip, "rotation", []float32{0, 0, 0, 1, 0, 0, -half, half}, 4)
	animation("Walk", root, "translation", []float32{0, 0, 0, 0, 0, 2}, 3)
	animation("Grow", body, "weights", []float32{0, 1}, 1)
	return b.GLB()
}

func within(a, b, by float32) bool {
	return math.Abs(float64(a-b)) <= float64(by)
}

func TestAnimatorPlays(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	bar := NewModel("models/bar.glb")
	if err := bar.Err(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(bar.Animations(), " "); got != "Bend Wave Walk Grow" {
		t.Errorf("Animations() = %s, want the file's, in its order", got)
	}
	lowest, upmost := bar.Bounds()
	if !near3(lowest, Vector3{X: -0.2}) || !near3(upmost, Vector3{X: 0.2, Y: 2}) {
		t.Errorf("Bounds() = %v, %v, want the strip at rest, bent by its bones", lowest, upmost)
	}
	if _, ok := bar.Marker("Hand"); !ok {
		t.Error("the marker under a bone isn't among the markers")
	}

	a := NewAnimator(bar)
	if got := a.Bone("Hand").Position; !near3(got, Vector3{Y: 1.5}) {
		t.Errorf("at rest the hand is at %v, want 0, 1.5, 0", got)
	}
	// The first animation shows at once; half a second in, a turn of 45°.
	a.Play("Bend")
	a.Update(0.5)
	if got := a.Bone("Hand").Position; !near3(got, Vector3{X: -0.3536, Y: 1.3536}) {
		t.Errorf("half way through Bend the hand is at %v, want turned 45° left around the tip", got)
	}
	if a.Playing() != "Bend" || !within(a.Time(), 0.5, 1e-5) {
		t.Errorf("Playing %q at %v, want Bend at 0.5", a.Playing(), a.Time())
	}
	// It loops.
	a.Update(0.75)
	if !within(a.Time(), 0.25, 1e-5) {
		t.Errorf("Bend looped to %v, want 0.25", a.Time())
	}
	// Playing it again changes nothing.
	a.Play("Bend")
	if !within(a.Time(), 0.25, 1e-5) {
		t.Errorf("playing Bend again moved it to %v", a.Time())
	}

	once := NewAnimator(bar)
	once.PlayWith("Bend", AnimationOptions{Once: true})
	once.Update(0.6)
	if once.Finished() {
		t.Error("Finished before its end")
	}
	once.Update(0.6)
	if !once.Finished() || !within(once.Time(), 1, 1e-5) {
		t.Errorf("played once, at %v, finished %v: want held at its end, 1", once.Time(), once.Finished())
	}
	if got := once.Bone("Hand").Position; !near3(got, Vector3{X: -0.5, Y: 1}) {
		t.Errorf("at the end of Bend the hand is at %v, want a quarter turn left: -0.5, 1, 0", got)
	}
	if got := once.Bone("Hand").Rotation; !within(got.Roll, -90, 0.01) && !within(got.Roll, 90, 0.01) {
		t.Errorf("at the end of Bend the hand is turned %+v, want a quarter turn around Z", got)
	}

	// A fade blends one into the next: half way, half each.
	once.Play("Wave")
	once.Update(0.1)
	hand := once.Bone("Hand").Position.Sub(Vector3{Y: 1})
	angle := math.Atan2(float64(-hand.X), float64(hand.Y)) * 180 / math.Pi
	if !within(float32(angle), (90-9)/2, 2) {
		t.Errorf("half way from Bend's end into Wave, the tip turned %.1f°, want about %.1f", angle, (90.0-9)/2)
	}
	once.Update(0.2)
	hand = once.Bone("Hand").Position.Sub(Vector3{Y: 1})
	if angle := math.Atan2(float64(-hand.X), float64(hand.Y)) * 180 / math.Pi; !within(float32(angle), -27, 1) {
		t.Errorf("faded into Wave, the tip turned %.1f°, want Wave's -27° at 0.3 s", angle)
	}

	// Then plays on once the first ends.
	then := NewAnimator(bar)
	then.PlayWith("Bend", AnimationOptions{Once: true, Then: "Walk"})
	then.Update(1.05)
	if then.Playing() != "Walk" {
		t.Errorf("after Bend played once, Playing is %q, want Walk", then.Playing())
	}

	// Stop("") fades back to the rest pose.
	then.Stop("")
	then.Update(0.5)
	if got := then.Bone("Hand").Position; !near3(got, Vector3{Y: 1.5}) || then.Playing() != "" {
		t.Errorf("stopped, the hand is at %v, playing %q, want at rest", got, then.Playing())
	}
}

func TestAnimatorRootMotionAndLayers(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	bar := NewModel("models/bar.glb")

	walker := NewAnimator(bar)
	walker.Play("Walk")
	walker.Update(0.5)
	if got := walker.Bone("Root").Position; !near3(got, Vector3{Z: 1}) || walker.Moved() != (Vector3{}) {
		t.Errorf("without root motion the root walks to %v and Moved is %v, want 0, 0, 1 and nothing", got, walker.Moved())
	}

	walker = NewAnimator(bar)
	walker.RootMotion = true
	walker.Play("Walk")
	walker.Update(0.5)
	if got := walker.Moved(); !near3(got, Vector3{Z: 1}) {
		t.Errorf("Moved = %v, want the meter the root walked", got)
	}
	if got := walker.Bone("Root").Position; !near3(got, Vector3{}) {
		t.Errorf("with root motion the root is drawn at %v, want kept where the model is", got)
	}
	// Across the loop's end, the walk goes on forward.
	walker.Update(0.75)
	if got := walker.Moved(); !near3(got, Vector3{Z: 1.5}) {
		t.Errorf("Moved across the loop = %v, want 1.5 forward", got)
	}

	// A layer on Tip waves while the root walks.
	layered := NewAnimator(bar)
	layered.Play("Walk")
	layered.PlayWith("Wave", AnimationOptions{Bone: "Tip"})
	// Half faded in, a tenth of a second into Wave: half of its 9°.
	layered.Update(0.1)
	hand := layered.Bone("Hand").Position
	if !within(hand.Z, 0.2, 1e-4) || !within(hand.X, 0.5*float32(math.Sin(4.5*math.Pi/180)), 0.002) {
		t.Errorf("waving the tip while walking, the hand is at %v, want walked 0.2 m and turned 4.5° right", hand)
	}
	layered.Update(0.65)
	hand = layered.Bone("Hand").Position
	if !within(hand.Z, 1.5, 1e-4) || !within(hand.X, 0.5*float32(math.Sin(67.5*math.Pi/180)), 0.002) {
		t.Errorf("faded in, the hand is at %v, want walked 1.5 m and turned Wave's 67.5° at 0.75 s", hand)
	}
	layered.Stop("Tip")
	layered.Update(0.3)
	if hand = layered.Bone("Hand").Position; !within(hand.X, 0, 1e-4) {
		t.Errorf("the layer stopped, the hand is at %v, want straight above the root again", hand)
	}
}

func TestAnimatorMistakes(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	a := NewAnimator(NewModel("models/bar.glb"))
	takeError()
	for _, c := range []struct {
		do   func()
		want string
	}{
		{func() { a.Play("Run") }, `has no animation called "Run"; it has "Bend", "Wave", "Walk", "Grow"`},
		{func() { a.PlayWith("Bend", AnimationOptions{Bone: "Spine"}) }, `has no bone called "Spine"; it has "Root", "Tip"`},
		{func() { a.Bone("Foot") }, `no bone or marker called "Foot"`},
		{func() { a.SetShapeKey("Smile", 1) }, `has no shape key called "Smile"; it has "Wide"`},
	} {
		c.do()
		if err := takeError(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v, want it to say %s", err, c.want)
		}
	}
}

func TestAnimatedInAWindow(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Y: 1, Z: 4}, Vector3{Y: 1}
	bar := NewModel("models/bar.glb")
	at := func(p Vector3) (int, int) {
		s, _ := camera.ToScreen(p)
		return int(s.X), int(s.Y)
	}
	draw := func(animators ...*Animator) func() {
		return func() {
			screen.Clear(Black)
			screen.Begin3D(camera)
			for i, a := range animators {
				screen.DrawAnimated(a, Transform{Position: Vector3{X: float32(i) * 3}}, White)
			}
			screen.End3D()
		}
	}

	rest := NewAnimator(bar)
	still := capture(draw(rest))
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	x, y := at(Vector3{Y: 1.8})
	if c := still.NRGBAAt(x, y); c.R < 200 {
		t.Errorf("at rest the bar's top is %v, want white", c)
	}
	x, y = at(Vector3{X: -0.7, Y: 1})
	if c := still.NRGBAAt(x, y); c.R > 5 {
		t.Errorf("at rest left of the bar is %v, want nothing", c)
	}

	// Bent, its top half lies to the left; a second one, drawn in the same
	// draw at rest, stays straight.
	bent := NewAnimator(bar)
	bent.PlayWith("Bend", AnimationOptions{Once: true})
	bent.Update(1)
	camera.Position, camera.Target = Vector3{X: 1.5, Y: 1, Z: 6}, Vector3{X: 1.5, Y: 1}
	both := capture(draw(bent, rest))
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	x, y = at(Vector3{X: -0.7, Y: 1})
	if c := both.NRGBAAt(x, y); c.R < 200 {
		t.Errorf("bent, the bar's top half is %v left of its middle, want white", c)
	}
	x, y = at(Vector3{Y: 1.8})
	if c := both.NRGBAAt(x, y); c.R > 5 {
		t.Errorf("bent, above its middle is %v, want nothing", c)
	}
	x, y = at(Vector3{X: 3, Y: 1.8})
	if c := both.NRGBAAt(x, y); c.R < 200 {
		t.Errorf("the second bar, at rest, is %v at its top, want white: each copy keeps its own pose", c)
	}

	// The shape key spreads the top.
	wide := NewAnimator(bar)
	wide.SetShapeKey("Wide", 1)
	camera.Position, camera.Target = Vector3{Y: 1, Z: 4}, Vector3{Y: 1}
	spread := capture(draw(wide))
	x, y = at(Vector3{X: 0.7, Y: 1.95})
	if c := spread.NRGBAAt(x, y); c.R < 200 {
		t.Errorf("with Wide, the top's right is %v, want white", c)
	}
	if c := still.NRGBAAt(x, y); c.R > 5 {
		t.Errorf("without Wide, the top's right is %v, want nothing", c)
	}

	// Through DrawModel, a skinned model is at rest.
	model := capture(func() {
		screen.Clear(Black)
		screen.Begin3D(camera)
		screen.DrawModel(bar, Transform{}, White)
		screen.End3D()
	})
	x, y = at(Vector3{Y: 1.8})
	if c := model.NRGBAAt(x, y); c.R < 200 {
		t.Errorf("drawn with DrawModel, the bar's top is %v, want white, at rest", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelSampling(t *testing.T) {
	// Cubic: from 0 to 1 with flat tangents, a smooth step, half way at
	// the middle and a quarter of the way rather sooner than linear.
	cubic := animationChannel{path: pathTranslation, cubic: true, width: 1, times: []float32{0, 1}, values: []float32{0, 0, 0, 0, 1, 0}}
	var out [1]float32
	cubic.sample(0.5, out[:])
	if !within(out[0], 0.5, 1e-6) {
		t.Errorf("cubic at its middle = %v, want 0.5", out[0])
	}
	cubic.sample(0.25, out[:])
	if !within(out[0], 0.15625, 1e-6) {
		t.Errorf("cubic at a quarter = %v, want 0.15625", out[0])
	}
	step := animationChannel{path: pathTranslation, step: true, width: 1, times: []float32{0, 1, 2}, values: []float32{3, 5, 7}}
	step.sample(1.9, out[:])
	if out[0] != 5 {
		t.Errorf("step at 1.9 = %v, want the value of the keyframe at 1", out[0])
	}
	step.sample(9, out[:])
	if out[0] != 7 {
		t.Errorf("step past its end = %v, want its last value", out[0])
	}
}

func TestShapeKeysPlayOnALayer(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	a := NewAnimator(NewModel("models/bar.glb"))
	a.Play("Walk")
	a.PlayWith("Grow", AnimationOptions{Bone: "Tip"})
	a.Update(0.5)
	body := a.model.nodeIndex("Body")
	if got := a.posedNow().weights[body][0]; !within(got, 0.5, 1e-4) {
		t.Errorf("Grow played on a layer over Walk gives Wide %v, want 0.5", got)
	}
	if got := a.Bone("Root").Position; !within(got.Z, 1, 1e-4) {
		t.Errorf("under the layer, the root walked to %v, want 1 along Z", got)
	}
}

func TestAnimatorLayersAndLengths(t *testing.T) {
	useAssets(t, map[string][]byte{"models/bar.glb": barGLB()})
	bar := NewModel("models/bar.glb")
	if got := strings.Join(bar.Bones(), " "); got != "Root Tip" {
		t.Errorf("Bones() = %s, want the skin's joints", got)
	}
	if got := bar.AnimationLength("Bend"); got != 1 {
		t.Errorf("AnimationLength(Bend) = %v, want 1", got)
	}
	if got := bar.AnimationLength("Run"); got != 0 {
		t.Errorf("AnimationLength of a missing animation = %v, want 0", got)
	}

	// Two layers on the same bone's tree: the one played last goes over.
	a := NewAnimator(bar)
	a.PlayWith("Bend", AnimationOptions{Bone: "Tip"})
	a.PlayWith("Wave", AnimationOptions{Bone: "Root"})
	a.Update(0.5)
	if got := a.Bone("Hand").Position; got.X <= 0 {
		t.Errorf("Wave, played last, should turn the hand right, but it is at %v", got)
	}
	a.PlayWith("Bend", AnimationOptions{Bone: "Tip", Restart: true})
	a.Update(0.3)
	if got := a.Bone("Hand").Position; got.X >= 0 {
		t.Errorf("Bend, played again, should go over Wave and turn the hand left, but it is at %v", got)
	}
	if a.PlayingOn("Tip") != "Bend" || !within(a.TimeOn("Tip"), 0.3, 1e-5) {
		t.Errorf("on Tip: %q at %v, want Bend at 0.3", a.PlayingOn("Tip"), a.TimeOn("Tip"))
	}

	// A layer played once is over when it has faded out.
	once := NewAnimator(bar)
	once.PlayWith("Bend", AnimationOptions{Bone: "Tip", Once: true})
	once.Update(0.5)
	if once.PlayingOn("Tip") != "Bend" {
		t.Errorf("half way, PlayingOn(Tip) = %q, want Bend", once.PlayingOn("Tip"))
	}
	once.Update(0.6)
	if once.PlayingOn("Tip") != "" {
		t.Errorf("at its end, PlayingOn(Tip) = %q, want nothing, as the layer fades out", once.PlayingOn("Tip"))
	}
}

func TestSeeThroughShowsOnlyTheFront(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: 4}, Vector3{}
	// Two squares one behind the other, both facing the camera: red in
	// front, blue behind.
	square := func(z float32, c Color) []Vertex {
		return []Vertex{
			{Position: Vector3{X: -1, Y: -1, Z: z}, Color: c}, {Position: Vector3{X: 1, Y: -1, Z: z}, Color: c},
			{Position: Vector3{X: 1, Y: 1, Z: z}, Color: c}, {Position: Vector3{X: -1, Y: 1, Z: z}, Color: c},
		}
	}
	both := NewMesh(append(square(0, Red), square(-1, Blue)...), []int{0, 1, 2, 0, 2, 3, 4, 5, 6, 4, 6, 7})
	front := NewMesh(square(0, Red), []int{0, 1, 2, 0, 2, 3})
	draw := func(mesh *Mesh) func() {
		return func() {
			screen.Clear(Black)
			screen.Begin3D(camera)
			screen.SetEnvironment(Environment{NoShadows: true})
			screen.DrawMesh(mesh, Transform{}, Color{R: 255, G: 255, B: 255, A: 128})
			screen.End3D()
		}
	}
	got, want := capture(draw(both)).NRGBAAt(32, 32), capture(draw(front)).NRGBAAt(32, 32)
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("a half see-through mesh is %v in the middle, want %v, its front square alone, without the blue one inside it", got, want)
	}
}
