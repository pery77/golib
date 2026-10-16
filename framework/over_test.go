package golib

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"golib/internal/device"
)

// A smoothed texture's colors are multiplied by their opacity, so a
// see-through pixel blended with its neighbours weighs as much as shows of
// it, and the edges of a picture don't darken.
func TestPremultiplied(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	pixels.SetNRGBA(1, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	pixels.SetNRGBA(2, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 0})
	got := premultiplied(pixels)
	want := []byte{255, 255, 255, 255, 100, 50, 25, 128, 0, 0, 0, 0}
	if string(got) != string(want) {
		t.Errorf("premultiplied gives %v, want %v", got, want)
	}
	if pixels.Pix[4] != 200 {
		t.Error("premultiplied changed the sprite's own pixels")
	}
}

// A frame drawn at full resolution waits in screen pixels, unrounded, seen
// through the camera, with its tint premultiplied as its texture is.
func TestAFullResolutionFrameWaitsInScreenPixels(t *testing.T) {
	place := image.Rect(10, 0, 20, 8)
	options := DrawOptions{Scale: 0.5, OriginX: 4, FlipX: true, Tint: Color{R: 255, G: 128, B: 0, A: 128}}
	got := options.over(device.Texture{ID: 7}, place, 100.25, 50.5, nil)
	want := overDraw{
		texture: device.Texture{ID: 7},
		source:  device.Rectangle{X: 10, Y: 0, Width: -10, Height: 8},
		dest:    device.Rectangle{X: 100.25, Y: 50.5, Width: 5, Height: 4},
		origin:  device.Vector2{X: 2},
		tint:    Color{R: 128, G: 64, B: 0, A: 128},
	}
	if got != want {
		t.Errorf("the frame waits as %+v, want %+v", got, want)
	}

	camera := NewCamera(320, 180)
	camera.Zoom = 2
	camera.Target = Vector2{X: 1000, Y: 500}
	camera.Snap()
	seen := DrawOptions{}.over(device.Texture{}, place, 1010, 505, camera)
	if seen.dest != (device.Rectangle{X: 180, Y: 100, Width: 20, Height: 16}) {
		t.Errorf("through a camera zoomed twice the frame goes to %+v, want 180, 100 and twice its size", seen.dest)
	}
	if seen.tint != White {
		t.Errorf("with no tint the frame is tinted %v, want white", seen.tint)
	}
}

// The mouse sprite shows only while the player points with the mouse inside
// the window.
func TestTheMouseSpriteShowsWhileThePlayerPoints(t *testing.T) {
	pointer := newGeneratedSprite("a test pointer", func(s *Sprite) error {
		s.pixels = image.NewNRGBA(image.Rect(0, 0, 8, 4))
		s.width, s.height = 4, 4
		s.frames = gridFrames(2, 1, 4, 4, 0)
		s.animations = allFrames(2)
		return nil
	})
	t.Cleanup(func() {
		SetMouseSprite(nil, 0, 0, 0)
		mouseHiddenWanted.Store(false)
		playingWithGamepad.Store(false)
		playingWithTouch.Store(false)
	})
	if p := pointerAt(10, 20, true); p.sprite != nil || mouseSpriteSet() {
		t.Fatal("without SetMouseSprite there is a mouse sprite")
	}
	SetMouseSprite(pointer, 1, 2, 3)
	if !mouseSpriteSet() {
		t.Fatal("SetMouseSprite didn't replace the system's pointer")
	}
	want := pointerDraw{sprite: pointer, frame: 1, tipX: 2, tipY: 3, x: 10, y: 20}
	if p := pointerAt(10, 20, true); p != want {
		t.Errorf("the mouse sprite draws as %+v, want %+v", p, want)
	}
	for _, c := range []struct {
		why  string
		hide func()
		in   bool
	}{
		{"outside the window", func() {}, false},
		{"hidden by SetMouseVisible", func() { mouseHiddenWanted.Store(true) }, true},
		{"playing with a gamepad", func() { playingWithGamepad.Store(true) }, true},
		{"playing with fingers", func() { playingWithTouch.Store(true) }, true},
	} {
		c.hide()
		if p := pointerAt(10, 20, c.in); p.sprite != nil {
			t.Errorf("the mouse sprite shows %s", c.why)
		}
		mouseHiddenWanted.Store(false)
		playingWithGamepad.Store(false)
		playingWithTouch.Store(false)
	}

	SetMouseSprite(pointer, 2, 0, 0)
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "SetMouseSprite got frame 2") {
		t.Errorf("a frame the sprite doesn't have gives %v, want an error that names it", err)
	}
	if p := pointerAt(0, 0, true); p.frame != 1 {
		t.Errorf("a frame the sprite doesn't have replaced frame 1 with %d", p.frame)
	}
}

// The mouse sprite is as many whole times larger as the screen, with its tip
// on the pointer, at whole pixels of the window, between screen pixels too.
func TestTheMouseSpriteStaysSharp(t *testing.T) {
	fit := device.Rectangle{X: 40, Y: 0, Width: 1920, Height: 1080} // a 640 by 360 screen, three times larger
	p := pointerDraw{tipX: 1, tipY: 1, x: 100.5, y: 50}
	// 40 + 100.5*3 - 1*3 is 338.5.
	if got, want := p.dest(fit, 3, 9, 13), (device.Rectangle{X: 339, Y: 147, Width: 27, Height: 39}); got != want {
		t.Errorf("the pointer goes to %+v, want %+v", got, want)
	}
	if got := p.dest(device.Rectangle{Width: 1280, Height: 720}, 1.6, 9, 13); got.Width != 18 || got.Height != 26 {
		t.Errorf("at 1.6 window pixels a screen pixel the pointer is %g by %g, want twice its size", got.Width, got.Height)
	}
	if got := p.dest(device.Rectangle{Width: 320, Height: 180}, 0.5, 9, 13); got.Width != 9 {
		t.Errorf("in a window smaller than the screen the pointer is %g wide, want its own 9", got.Width)
	}
}

// golib shot draws the mouse sprite once its script has moved the mouse.
func TestAShotShowsTheMouseOnceItMoved(t *testing.T) {
	script, err := parseInputScript("Enter@1 Mouse@30:100,50")
	if err != nil {
		t.Fatal(err)
	}
	if script.mouseMovedBy(29) || !script.mouseMovedBy(30) || !script.mouseMovedBy(90) {
		t.Error("the shot's pointer shows before the script moves the mouse, or not after")
	}
	if none, _ := parseInputScript("Enter@1"); none.mouseMovedBy(100) {
		t.Error("a script that never moves the mouse shows the pointer")
	}
}

// Text drawn at full resolution waits in screen pixels, unrounded, seen
// through the camera, its size zoomed with it, instead of being drawn.
func TestFullResolutionTextWaitsInScreenPixels(t *testing.T) {
	var s Screen
	s.DrawText("Hola", 10.5, 20, 12, White, TextOptions{FullResolution: true, Align: AlignCenter})
	if len(s.over) != 1 || s.over[0].text == nil {
		t.Fatalf("the text waits as %+v, want one text", s.over)
	}
	want := overText{text: "Hola", x: 10.5, y: 20, size: 12, color: White, align: AlignCenter}
	if got := *s.over[0].text; got != want {
		t.Errorf("the text waits as %+v, want %+v", got, want)
	}

	camera := NewCamera(320, 180)
	camera.Zoom = 2
	camera.Target = Vector2{X: 1000, Y: 500}
	camera.Snap()
	s.camera = camera
	s.DrawText("Adiós", 1010, 505, 10, White, TextOptions{FullResolution: true})
	seen := *s.over[1].text
	if seen.x != 180 || seen.y != 100 || seen.size != 20 {
		t.Errorf("through a camera zoomed twice the text goes to %v, %v at size %v, want 180, 100 at 20", seen.x, seen.y, seen.size)
	}
}
