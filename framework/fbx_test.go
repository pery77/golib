package golib

import (
	"math"
	"os"
	"strings"
	"testing"

	"golib/internal/gltf"
)

// TestFBXMatchesGLTF reads one scene Blender exported both ways, a tube
// bent by three bones with a shape key (made by
// internal/fbx/testdata/tube.py), and checks that the FBX file poses as the
// glTF one does. Blender's glTF export starts an action a frame later than
// its FBX export, at 1/30 of a second.
func TestFBXMatchesGLTF(t *testing.T) {
	read := func(name string) []byte {
		b, err := os.ReadFile("internal/fbx/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	useAssets(t, map[string][]byte{"models/tube.fbx": read("tube.fbx"), "models/tube.glb": read("tube.glb")})
	f, g := NewModel("models/tube.fbx"), NewModel("models/tube.glb")
	if err := f.Err(); err != nil {
		t.Fatal(err)
	}
	fl, fu := f.Bounds()
	gl, gu := g.Bounds()
	if !near3(fl, gl) || !near3(fu, gu) {
		t.Errorf("the FBX tube's bounds are %v to %v, the glTF one's %v to %v", fl, fu, gl, gu)
	}
	if got := strings.Join(f.Bones(), " "); got != "Base Middle Top" {
		t.Errorf("the FBX tube's bones are %s", got)
	}
	if !strings.Contains(strings.Join(f.Animations(), " "), "Sway") {
		t.Errorf("the FBX tube's animations are %v, want Sway, as Blender names the action", f.Animations())
	}
	for _, at := range []float32{0.3, 0.5, 1.2} {
		a, b := NewAnimator(f), NewAnimator(g)
		a.Play("Sway")
		b.Play("Sway")
		a.Update(at)
		b.Update(at + 1.0/30)
		for _, bone := range []string{"Middle", "Top"} {
			if pa, pb := a.Bone(bone).Position, b.Bone(bone).Position; pa.Distance(pb) > 2e-3 {
				t.Errorf("%v s into Sway, %s is at %v from the FBX file and %v from the glTF one", at, bone, pa, pb)
			}
		}
	}
	// The shape key, read with its name; Blender's FBX export puts its action
	// in Sway's take.
	a := NewAnimator(f)
	a.SetShapeKey("Bulge", 1)
	if err := takeError(); err != nil {
		t.Errorf("SetShapeKey on the FBX tube: %v", err)
	}
}

// kickGLB is a file of one animation alone, as Mixamo gives them: the bar's
// skeleton, Root and Tip, without the bar, and Tip turned a quarter turn.
func kickGLB(tipName string) []byte {
	b := gltf.NewBuilder()
	tip := b.Add("nodes", map[string]any{"name": tipName, "translation": []float32{0, 1, 0}})
	root := b.Add("nodes", map[string]any{"name": "Root", "children": []any{tip}})
	b.Root(root)
	half := float32(math.Sqrt(0.5))
	b.Add("animations", map[string]any{
		"name":     "mixamo.com",
		"channels": []any{map[string]any{"sampler": 0, "target": map[string]any{"node": tip, "path": "rotation"}}},
		"samplers": []any{map[string]any{"input": b.Floats([]float32{0, 1}, 1), "output": b.Floats([]float32{0, 0, 0, 1, 0, 0, half, half}, 4)}},
	})
	return b.GLB()
}

func TestAddAnimations(t *testing.T) {
	useAssets(t, map[string][]byte{
		"models/bar.glb": barGLB(), "models/kick.glb": kickGLB("Tip"), "models/other.glb": kickGLB("Head"),
	})
	bar := NewModel("models/bar.glb").AddAnimations("models/kick.glb")
	if err := bar.Err(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(bar.Animations(), " "); got != "Bend Wave Walk Grow kick" {
		t.Errorf("Animations() = %s, want the file's animation named after the file, last", got)
	}
	a := NewAnimator(bar)
	a.PlayWith("kick", AnimationOptions{Once: true})
	a.Update(1)
	if got := a.Bone("Hand").Position; !near3(got, Vector3{X: -0.5, Y: 1}) {
		t.Errorf("at the end of kick the hand is at %v, want turned a quarter left, as Bend turns it", got)
	}
	if err := NewModel("models/bar.glb").AddAnimations("models/other.glb").Err(); err == nil || !strings.Contains(err.Error(), `moves no bone of the model: its bones are named "Head"`) {
		t.Errorf("an animation of other bones gave %v", err)
	}
}
