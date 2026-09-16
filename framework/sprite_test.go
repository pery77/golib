package golib

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useAssets makes the working directory, for the rest of the test, a folder
// whose assets folder holds files, by name.
func useAssets(t *testing.T, files map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		path := filepath.Join(dir, "assets", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	takeError() // start clean
}

// pngFile encodes a width by height image whose pixel x, y has red x and
// green y.
func pngFile(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// wantError checks that the mistake waiting for Run contains want.
func wantError(t *testing.T, want string) {
	t.Helper()
	err := takeError()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("reported error = %v, want one containing %q", err, want)
	}
}

func TestSpriteFromPNG(t *testing.T) {
	useAssets(t, map[string][]byte{"sprites/tree.png": pngFile(t, 3, 2)})
	tree := NewSprite("sprites/tree.png")
	if tree.Width() != 3 || tree.Height() != 2 || tree.Frames() != 1 {
		t.Errorf("tree is %v by %v with %d frames, want 3 by 2 with 1", tree.Width(), tree.Height(), tree.Frames())
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}
	if got := tree.pixels.NRGBAAt(2, 1); got != (color.NRGBA{R: 2, G: 1, A: 255}) {
		t.Errorf("pixel 2, 1 = %v", got)
	}
	if _, place, err := tree.frameTexture(1); err == nil || !strings.Contains(err.Error(), `frame 1 of golib.NewSprite("sprites/tree.png"), which has 1 frame(s), numbered from 0 to 0`) {
		t.Errorf("frame 1: place %v, error %v", place, err)
	}
}

func TestSpriteSheet(t *testing.T) {
	useAssets(t, map[string][]byte{"hero.png": pngFile(t, 8, 4)})
	hero := NewSpriteSheet("hero.png", 2, 2)
	if hero.Width() != 2 || hero.Height() != 2 || hero.Frames() != 8 {
		t.Fatalf("hero is %v by %v with %d frames, want 2 by 2 with 8", hero.Width(), hero.Height(), hero.Frames())
	}
	// Frame 5 is the second one on the second row.
	if want := image.Rect(2, 2, 4, 4); hero.frames[5] != want {
		t.Errorf("frame 5 is at %v, want %v", hero.frames[5], want)
	}
	for _, frame := range []int{-1, 8} {
		if _, _, err := hero.frameTexture(frame); err == nil || !strings.Contains(err.Error(), "which has 8 frame(s), numbered from 0 to 7") {
			t.Errorf("frame %d: error %v", frame, err)
		}
	}
	hero.Animation("run")
	wantError(t, `golib.NewSpriteSheet("hero.png", 2, 2) has no animation named "run": PNG files have only the animation named ""`)
	if all := hero.Animation(""); len(all.Frames) != 8 || all.Frames[7] != 7 || all.Frame(0.25) != 2 {
		t.Errorf(`Animation("") = %+v`, all)
	}
}

func TestSpriteMistakes(t *testing.T) {
	useAssets(t, map[string][]byte{
		"hero.png":        pngFile(t, 8, 4),
		"broken.png":      []byte("not a png"),
		"slime.aseprite":  []byte("an aseprite file"),
		"notes/hero.jpeg": []byte("a jpeg"),
	})
	tests := []struct {
		sprite *Sprite
		want   string
	}{
		{NewSprite("missing.png"), `golib.NewSprite("missing.png"): golib.ReadAsset: assets/missing.png not found`},
		{NewSprite("broken.png"), `golib.NewSprite("broken.png"): not a PNG image GoLib can read`},
		{NewSprite("notes/hero.jpeg"), `GoLib reads sprites from .png, .aseprite and .ase files, not ".jpeg"`},
		{NewSpriteSheet("hero.png", 3, 2), "the image is 8 by 4 pixels, which isn't a whole number of 3 by 2 frames"},
		{NewSpriteSheet("hero.png", 0, 2), `golib.NewSpriteSheet("hero.png", 0, 2): frames must be at least 1 by 1 pixels`},
		{NewSpriteSheet("hero.png", -2, -2), "frames must be at least 1 by 1 pixels"},
		{NewSpriteSheet("slime.aseprite", 2, 2), "golib.NewSpriteSheet reads PNG files only"},
	}
	for _, tt := range tests {
		if tt.sprite.Width() != 0 || tt.sprite.Frames() != 0 {
			t.Errorf("%s: has a size or frames", tt.sprite.call())
		}
		wantError(t, tt.want)
		// The mistake is reported again every time the sprite is used.
		tt.sprite.Height()
		wantError(t, tt.want)
	}
}

func TestAnimationFrame(t *testing.T) {
	run := Animation{Frames: []int{4, 5, 6}, FrameTime: 0.1}
	uneven := Animation{Frames: []int{1, 2, 3}, Durations: []float32{0.1, 0.3, 0.1}}
	once := Animation{Frames: []int{7, 8}, Once: true}
	tests := []struct {
		name      string
		animation Animation
		time      float32
		want      int
	}{
		{"start", run, 0, 4},
		{"before the start", run, -1, 4},
		{"first frame", run, 0.05, 4},
		{"second frame", run, 0.1, 5},
		{"third frame", run, 0.25, 6},
		{"loops", run, 0.3, 4},
		{"loops again", run, 0.75, 5},
		{"uneven: second frame", uneven, 0.39, 2},
		{"uneven: third frame", uneven, 0.41, 3},
		{"default frame time", once, 0.15, 8},
		{"once: stays on the last frame", once, 5, 8},
		{"no frames", Animation{}, 1, 0},
		{"no time", Animation{Frames: []int{3, 4}, Durations: []float32{0, 0}}, 1, 3},
	}
	for _, tt := range tests {
		if got := tt.animation.Frame(tt.time); got != tt.want {
			t.Errorf("%s: Frame(%v) = %d, want %d", tt.name, tt.time, got, tt.want)
		}
	}
	if d := uneven.Duration(); d < 0.4999 || d > 0.5001 {
		t.Errorf("uneven lasts %v, want 0.5", d)
	}
	if once.Finished(0.19) || !once.Finished(0.2) {
		t.Error("once should finish after 0.2 seconds")
	}
	if err := takeError(); err != nil {
		t.Errorf("reported error: %v", err)
	}

	Animation{Frames: []int{1, 2}, Durations: []float32{0.1}}.Frame(0.5)
	wantError(t, "golib.Animation has 1 Durations for 2 Frames")
}

func TestToNRGBA(t *testing.T) {
	paletted := image.NewPaletted(image.Rect(5, 5, 7, 6), color.Palette{color.Transparent, color.NRGBA{R: 200, A: 255}})
	paletted.SetColorIndex(6, 5, 1)
	pixels := toNRGBA(paletted)
	if pixels.Rect != image.Rect(0, 0, 2, 1) || pixels.NRGBAAt(1, 0) != (color.NRGBA{R: 200, A: 255}) || pixels.NRGBAAt(0, 0).A != 0 {
		t.Errorf("toNRGBA = %v %v", pixels.Rect, pixels.Pix)
	}
}
