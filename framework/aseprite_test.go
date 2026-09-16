package golib

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The files in testdata/aseprite come from make.lua, which Aseprite runs: each
// .aseprite file, and a PNG of each of its frames as Aseprite draws them.

func readTestAseprite(t *testing.T, name string) *aseFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "aseprite", name+".aseprite"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := parseAseprite(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return file
}

// readReference returns frame of name as Aseprite draws it.
func readReference(t *testing.T, name string, frame int) *image.NRGBA {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "aseprite", fmt.Sprintf("%s-%d.png", name, frame)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return toNRGBA(img)
}

// comparePixels reports the pixels where got and want differ; fully
// transparent pixels match whatever their color.
func comparePixels(t *testing.T, label string, got, want *image.NRGBA) {
	t.Helper()
	if got.Rect.Size() != want.Rect.Size() {
		t.Errorf("%s: %v, want %v", label, got.Rect.Size(), want.Rect.Size())
		return
	}
	var wrong []string
	for y := range want.Rect.Dy() {
		for x := range want.Rect.Dx() {
			g, w := got.NRGBAAt(got.Rect.Min.X+x, got.Rect.Min.Y+y), want.NRGBAAt(want.Rect.Min.X+x, want.Rect.Min.Y+y)
			if g.A == 0 && w.A == 0 {
				continue
			}
			if g != w {
				wrong = append(wrong, fmt.Sprintf("%d,%d: %v, want %v", x, y, g, w))
			}
		}
	}
	if len(wrong) > 0 {
		t.Errorf("%s: %d pixels differ from Aseprite's, such as %s", label, len(wrong), strings.Join(wrong[:min(len(wrong), 4)], "; "))
	}
}

func TestAsepriteFramesMatchAseprite(t *testing.T) {
	for _, name := range []string{"blend", "layers", "indexed", "gray"} {
		file := readTestAseprite(t, name)
		for frame := range file.frames {
			comparePixels(t, fmt.Sprintf("%s frame %d", name, frame), file.render(frame), readReference(t, name, frame))
		}
	}
}

func TestAsepriteFile(t *testing.T) {
	file := readTestAseprite(t, "layers")
	if file.width != 16 || file.height != 16 || len(file.frames) != 6 {
		t.Errorf("layers is %d by %d with %d frames, want 16 by 16 with 6", file.width, file.height, len(file.frames))
	}
	var names []string
	for _, layer := range file.layers {
		names = append(names, fmt.Sprintf("%s@%d", layer.name, layer.parent))
	}
	want := []string{"Background@-1", "hidden@-1", "walker@-1", "group@-1", "inside@3", "hidden group@-1", "lost@5", "linked@-1", "under@-1"}
	if !slices.Equal(names, want) {
		t.Errorf("layers and their groups = %q, want %q", names, want)
	}
	if file.shows(1) || file.shows(6) || !file.shows(4) {
		t.Error("the hidden layer and the layer in the hidden group should not show; the one in the visible group should")
	}
	if cel := file.celOf(2, 7); cel == nil || cel.pixels == nil {
		t.Error("frame 2 should show the linked cel of frame 0")
	}
}

func TestAsepriteAnimations(t *testing.T) {
	animations := readTestAseprite(t, "layers").animations()
	durations := []float32{0.1, 0.25, 0.05, 0.1, 0.3, 0.1}
	tests := []struct {
		name   string
		frames []int
		once   bool
	}{
		{"", []int{0, 1, 2, 3, 4, 5}, false},
		{"walk", []int{0, 1, 2}, false}, // the first tag of that name
		{"back", []int{3, 2, 1, 3, 2, 1}, true},
		{"bounce", []int{0, 1, 2, 3, 2, 1}, false},
		{"bounce back", []int{5, 4, 3, 2, 3, 4, 5, 4, 3, 2}, true},
		{"still", []int{4}, false},
	}
	if len(animations) != len(tests) {
		t.Errorf("%d animations, want %d", len(animations), len(tests))
	}
	for _, tt := range tests {
		got, found := animations[tt.name]
		if !found {
			t.Errorf("no animation %q", tt.name)
			continue
		}
		if !slices.Equal(got.Frames, tt.frames) || got.Once != tt.once {
			t.Errorf("animation %q plays %v, once %v; want %v, once %v", tt.name, got.Frames, got.Once, tt.frames, tt.once)
		}
		for i, frame := range got.Frames {
			if got.Durations[i] != durations[frame] {
				t.Errorf("animation %q: frame %d lasts %v, want %v", tt.name, frame, got.Durations[i], durations[frame])
				break
			}
		}
	}
}

func TestAsepriteSprite(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "aseprite", "layers.aseprite"))
	if err != nil {
		t.Fatal(err)
	}
	var references []*image.NRGBA
	for i := range 6 {
		references = append(references, readReference(t, "layers", i))
	}
	useAssets(t, map[string][]byte{"slime.aseprite": data, "copy.ase": data})
	slime := NewSprite("slime.aseprite")
	if slime.Width() != 16 || slime.Height() != 16 || slime.Frames() != 6 {
		t.Fatalf("slime is %v by %v with %d frames, want 16 by 16 with 6", slime.Width(), slime.Height(), slime.Frames())
	}
	// Every frame is in the picture, with a gap around it.
	for i, place := range slime.frames {
		if place.Dx() != 16 || place.Dy() != 16 {
			t.Fatalf("frame %d is at %v", i, place)
		}
		comparePixels(t, fmt.Sprintf("frame %d in the picture", i), slime.pixels.SubImage(place).(*image.NRGBA), references[i])
	}
	if slime.frames[1].Min.X != 17 {
		t.Errorf("frame 1 starts at %v, want a one-pixel gap after frame 0", slime.frames[1].Min)
	}
	if bounce := slime.Animation("bounce"); !slices.Equal(bounce.Frames, []int{0, 1, 2, 3, 2, 1}) {
		t.Errorf("bounce = %v", bounce.Frames)
	}
	if all := slime.Animation(""); len(all.Frames) != 6 {
		t.Errorf("the animation of every frame has %v", all.Frames)
	}
	if err := takeError(); err != nil {
		t.Fatalf("reported error: %v", err)
	}
	// Changing an animation doesn't change the sprite's.
	walk := slime.Animation("walk")
	walk.Frames[0] = 5
	if slime.Animation("walk").Frames[0] != 0 {
		t.Error("changing a returned animation changed the sprite")
	}

	slime.Animation("jump")
	wantError(t, `golib.NewSprite("slime.aseprite") has no animation named "jump": it has "", "back", "bounce", "bounce back", "still", "walk"`)
	if NewSprite("copy.ase").Frames() != 6 {
		t.Error(".ase files should read as Aseprite files")
	}
}

func TestAsepriteMistakes(t *testing.T) {
	good, err := os.ReadFile(filepath.Join("testdata", "aseprite", "gray.aseprite"))
	if err != nil {
		t.Fatal(err)
	}
	pngData, err := os.ReadFile(filepath.Join("testdata", "aseprite", "gray-0.png"))
	if err != nil {
		t.Fatal(err)
	}
	wrongDepth := slices.Clone(good)
	wrongDepth[12] = 24
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "the file is cut short"},
		{"cut short", good[:len(good)-10], "the file is cut short"},
		{"a PNG", pngData, "not an Aseprite file"},
		{"wrong depth", wrongDepth, "24 bits per pixel"},
	}
	for _, tt := range tests {
		if _, err := parseAseprite(tt.data); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error %v, want one containing %q", tt.name, err, tt.want)
		}
	}
}

func TestBlendNormal(t *testing.T) {
	tests := []struct {
		backdrop, src rgba
		opacity       int
		want          rgba
	}{
		{rgba{10, 20, 30, 255}, rgba{200, 100, 50, 255}, 255, rgba{200, 100, 50, 255}},
		{rgba{}, rgba{200, 100, 50, 128}, 255, rgba{200, 100, 50, 128}},
		{rgba{10, 20, 30, 255}, rgba{200, 100, 50, 0}, 255, rgba{10, 20, 30, 255}},
		{rgba{0, 0, 0, 255}, rgba{255, 255, 255, 255}, 128, rgba{128, 128, 128, 255}},
	}
	for _, tt := range tests {
		if got := blend(tt.backdrop, tt.src, tt.opacity, blendNormal); got != tt.want {
			t.Errorf("blend(%v, %v, %d) = %v, want %v", tt.backdrop, tt.src, tt.opacity, got, tt.want)
		}
	}
	// Over nothing, every mode looks like normal.
	src := rgba{90, 180, 20, 200}
	for mode := uint16(blendNormal); mode <= blendDivide; mode++ {
		if got := blend(rgba{}, src, 255, mode); got != src {
			t.Errorf("mode %d over nothing = %v, want %v", mode, got, src)
		}
	}
	_ = color.NRGBA{}
}

// solid returns a 2 by 1 image of one color.
func solid(c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, c)
	img.SetNRGBA(1, 0, c)
	return img
}

// TestAsepriteGroups checks how a group's opacity counts: not at all, unless
// the file says groups are combined on their own first.
func TestAsepriteGroups(t *testing.T) {
	red, blue := color.NRGBA{200, 0, 0, 255}, color.NRGBA{0, 0, 200, 255}
	file := func(flags uint32) *aseFile {
		f := &aseFile{
			width: 2, height: 1, flags: flags,
			layers: []aseLayer{
				{name: "floor", flags: aseLayerVisible, opacity: 255},
				{name: "group", flags: aseLayerVisible, kind: aseLayerGroup, opacity: 128},
				{name: "inside", flags: aseLayerVisible, level: 1, opacity: 255},
			},
			frames: []aseFrame{{cels: []aseCel{
				{layer: 0, opacity: 255, pixels: solid(red), link: -1},
				{layer: 2, opacity: 255, pixels: solid(blue), link: -1},
			}}},
		}
		if err := f.check(); err != nil {
			t.Fatal(err)
		}
		return f
	}
	if got := file(aseLayerOpacityValid).render(0).NRGBAAt(0, 0); got != blue {
		t.Errorf("a group that isn't combined on its own: %v, want the blue layer as it is", got)
	}
	want := blend(rgba{200, 0, 0, 255}, rgba{0, 0, 200, 255}, 128, blendNormal)
	if got := file(aseLayerOpacityValid|aseGroupsComposite).render(0).NRGBAAt(1, 0); got != (color.NRGBA{uint8(want.r), uint8(want.g), uint8(want.b), uint8(want.a)}) {
		t.Errorf("a group combined on its own: %v, want the blue layer at half opacity, %v", got, want)
	}
}
