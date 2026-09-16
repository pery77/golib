package golib

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestFitScreen(t *testing.T) {
	tests := []struct {
		name                      string
		screenWidth, screenHeight float32
		windowWidth, windowHeight float32
		pixelArt                  bool
		want                      rl.Rectangle
	}{
		{name: "same size", screenWidth: 1280, screenHeight: 720, windowWidth: 1280, windowHeight: 720,
			want: rl.Rectangle{X: 0, Y: 0, Width: 1280, Height: 720}},
		{name: "wider window: bars left and right", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 720,
			want: rl.Rectangle{X: 320, Y: 0, Width: 1280, Height: 720}},
		{name: "taller window: bars above and below", screenWidth: 1280, screenHeight: 720, windowWidth: 1280, windowHeight: 1000,
			want: rl.Rectangle{X: 0, Y: 140, Width: 1280, Height: 720}},
		{name: "scaled up smoothly", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 1080,
			want: rl.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}},
		{name: "pixel art keeps a whole scale", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 1080, pixelArt: true,
			want: rl.Rectangle{X: 320, Y: 180, Width: 1280, Height: 720}},
		{name: "small pixel art screen", screenWidth: 320, screenHeight: 180, windowWidth: 1366, windowHeight: 768, pixelArt: true,
			want: rl.Rectangle{X: 43, Y: 24, Width: 1280, Height: 720}},
	}
	for _, tt := range tests {
		got := fitScreen(tt.screenWidth, tt.screenHeight, tt.windowWidth, tt.windowHeight, tt.pixelArt)
		if got != tt.want {
			t.Errorf("%s: fitScreen() = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestToScreen(t *testing.T) {
	letterboxed := rl.Rectangle{X: 320, Y: 0, Width: 1280, Height: 720}
	scaled := rl.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	tests := []struct {
		name         string
		x, y         float32
		fit          rl.Rectangle
		wantX, wantY float32
	}{
		{name: "top-left corner of a letterboxed screen", x: 320, y: 0, fit: letterboxed, wantX: 0, wantY: 0},
		{name: "center of a letterboxed screen", x: 960, y: 360, fit: letterboxed, wantX: 640, wantY: 360},
		{name: "on the bar left of the screen", x: 0, y: 0, fit: letterboxed, wantX: -320, wantY: 0},
		{name: "center of a scaled screen", x: 960, y: 540, fit: scaled, wantX: 640, wantY: 360},
		{name: "minimized window", x: 5, y: 6, fit: rl.Rectangle{}, wantX: 5, wantY: 6},
	}
	for _, tt := range tests {
		x, y := toScreen(tt.x, tt.y, tt.fit, 1280, 720)
		if x != tt.wantX || y != tt.wantY {
			t.Errorf("%s: toScreen(%v, %v) = %v, %v, want %v, %v", tt.name, tt.x, tt.y, x, y, tt.wantX, tt.wantY)
		}
	}
}

func TestSetFullscreen(t *testing.T) {
	t.Cleanup(func() { SetFullscreen(false) })
	SetFullscreen(true)
	if !IsFullscreen() {
		t.Error("IsFullscreen() = false after SetFullscreen(true)")
	}
	SetFullscreen(false)
	if IsFullscreen() {
		t.Error("IsFullscreen() = true after SetFullscreen(false)")
	}
}

func TestWindowScale(t *testing.T) {
	tests := []struct {
		screenWidth, screenHeight, monitorWidth, monitorHeight, want int
	}{
		{320, 180, 1920, 1080, 4},  // 1280 by 720
		{320, 180, 2560, 1440, 6},  // 1920 by 1080
		{320, 240, 1920, 1080, 3},  // the height decides
		{1280, 720, 1920, 1080, 1}, // already large
		{1920, 1080, 1366, 768, 1}, // larger than the monitor
		{320, 180, 0, 0, 1},        // no monitor
		{0, 180, 1920, 1080, 1},    // no screen
	}
	for _, tt := range tests {
		if got := windowScale(tt.screenWidth, tt.screenHeight, tt.monitorWidth, tt.monitorHeight); got != tt.want {
			t.Errorf("windowScale(%d, %d, %d, %d) = %d, want %d", tt.screenWidth, tt.screenHeight, tt.monitorWidth, tt.monitorHeight, got, tt.want)
		}
	}
}
