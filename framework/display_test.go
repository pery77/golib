package golib

import (
	"testing"

	"golib/internal/device"
)

func TestFitScreen(t *testing.T) {
	tests := []struct {
		name                      string
		screenWidth, screenHeight float32
		windowWidth, windowHeight float32
		pixelArt                  bool
		want                      device.Rectangle
	}{
		{name: "same size", screenWidth: 1280, screenHeight: 720, windowWidth: 1280, windowHeight: 720,
			want: device.Rectangle{X: 0, Y: 0, Width: 1280, Height: 720}},
		{name: "wider window: bars left and right", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 720,
			want: device.Rectangle{X: 320, Y: 0, Width: 1280, Height: 720}},
		{name: "taller window: bars above and below", screenWidth: 1280, screenHeight: 720, windowWidth: 1280, windowHeight: 1000,
			want: device.Rectangle{X: 0, Y: 140, Width: 1280, Height: 720}},
		{name: "scaled up smoothly", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 1080,
			want: device.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}},
		{name: "pixel art keeps a whole scale", screenWidth: 1280, screenHeight: 720, windowWidth: 1920, windowHeight: 1080, pixelArt: true,
			want: device.Rectangle{X: 320, Y: 180, Width: 1280, Height: 720}},
		{name: "small pixel art screen", screenWidth: 320, screenHeight: 180, windowWidth: 1366, windowHeight: 768, pixelArt: true,
			want: device.Rectangle{X: 43, Y: 24, Width: 1280, Height: 720}},
	}
	for _, tt := range tests {
		got := fitScreen(tt.screenWidth, tt.screenHeight, tt.windowWidth, tt.windowHeight, tt.pixelArt)
		if got != tt.want {
			t.Errorf("%s: fitScreen() = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestToScreen(t *testing.T) {
	letterboxed := device.Rectangle{X: 320, Y: 0, Width: 1280, Height: 720}
	scaled := device.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	tests := []struct {
		name         string
		x, y         float32
		fit          device.Rectangle
		wantX, wantY float32
	}{
		{name: "top-left corner of a letterboxed screen", x: 320, y: 0, fit: letterboxed, wantX: 0, wantY: 0},
		{name: "center of a letterboxed screen", x: 960, y: 360, fit: letterboxed, wantX: 640, wantY: 360},
		{name: "on the bar left of the screen", x: 0, y: 0, fit: letterboxed, wantX: -320, wantY: 0},
		{name: "center of a scaled screen", x: 960, y: 540, fit: scaled, wantX: 640, wantY: 360},
		{name: "minimized window", x: 5, y: 6, fit: device.Rectangle{}, wantX: 5, wantY: 6},
	}
	for _, tt := range tests {
		x, y := toScreen(tt.x, tt.y, tt.fit, 1280, 720)
		if x != tt.wantX || y != tt.wantY {
			t.Errorf("%s: toScreen(%v, %v) = %v, %v, want %v, %v", tt.name, tt.x, tt.y, x, y, tt.wantX, tt.wantY)
		}
	}
}

func TestScreenInWindow(t *testing.T) {
	tests := []struct {
		name                      string
		config                    Config
		windowWidth, windowHeight float32
		wantWidth, wantHeight     float32
		want                      device.Rectangle
	}{
		{name: "without FillWindow the screen keeps its size", config: Config{Width: 1280, Height: 720},
			windowWidth: 1920, windowHeight: 1200, wantWidth: 1280, wantHeight: 720,
			want: device.Rectangle{X: 0, Y: 60, Width: 1920, Height: 1080}},
		{name: "a 16:10 monitor makes it taller", config: Config{Width: 1280, Height: 720, FillWindow: true},
			windowWidth: 1920, windowHeight: 1200, wantWidth: 1280, wantHeight: 800,
			want: device.Rectangle{Width: 1920, Height: 1200}},
		{name: "an ultrawide monitor makes it wider", config: Config{Width: 1280, Height: 720, FillWindow: true},
			windowWidth: 3440, windowHeight: 1440, wantWidth: 1720, wantHeight: 720,
			want: device.Rectangle{Width: 3440, Height: 1440}},
		{name: "an uneven shape rounds up and cuts under a pixel", config: Config{Width: 1280, Height: 720, FillWindow: true},
			windowWidth: 2560, windowHeight: 1080, wantWidth: 1707, wantHeight: 720,
			want: device.Rectangle{X: -1, Width: 2560.5, Height: 1080}},
		{name: "pixel art keeps a whole scale", config: Config{Width: 320, Height: 180, FillWindow: true, PixelArt: true},
			windowWidth: 1366, windowHeight: 768, wantWidth: 342, wantHeight: 192,
			want: device.Rectangle{X: -1, Width: 1368, Height: 768}},
		{name: "a minimized window keeps the Config's size", config: Config{Width: 1280, Height: 720, FillWindow: true},
			windowWidth: 0, windowHeight: 0, wantWidth: 1280, wantHeight: 720,
			want: device.Rectangle{}},
		{
			name:        "WindowScale follows the drawing area",
			config:      Config{Width: 1280, Height: 720, WindowScale: 2},
			windowWidth: 1920, windowHeight: 1200,
			wantWidth: 960, wantHeight: 600,
			want: device.Rectangle{Width: 1920, Height: 1200},
		},
		{
			name: "WindowScale takes precedence over FillWindow",
			config: Config{
				Width: 1280, Height: 720, WindowScale: 2, FillWindow: true,
			},
			windowWidth: 1920, windowHeight: 1200,
			wantWidth: 960, wantHeight: 600,
			want: device.Rectangle{Width: 1920, Height: 1200},
		},
		{
			name: "the threshold keeps native pixels at 1024",
			config: Config{
				Width: 1280, Height: 720, WindowScale: 2,
				WindowScaleMinWidth: 1025, PixelArt: true, FillWindow: true,
			},
			windowWidth: 1024, windowHeight: 768,
			wantWidth: 1024, wantHeight: 768,
			want: device.Rectangle{Width: 1024, Height: 768},
		},
		{
			name: "the threshold restores retro pixels above 1024",
			config: Config{
				Width: 1280, Height: 720, WindowScale: 2,
				WindowScaleMinWidth: 1025, PixelArt: true, FillWindow: true,
			},
			windowWidth: 1025, windowHeight: 768,
			wantWidth: 512, wantHeight: 384,
			want: device.Rectangle{Width: 1024, Height: 768},
		},
	}
	for _, tt := range tests {
		width, height, fit := screenInWindow(tt.config, tt.windowWidth, tt.windowHeight)
		if width != tt.wantWidth || height != tt.wantHeight || fit != tt.want {
			t.Errorf("%s: screenInWindow() = %v, %v, %+v, want %v, %v, %+v", tt.name, width, height, fit, tt.wantWidth, tt.wantHeight, tt.want)
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

func TestWindowFit(t *testing.T) {
	tests := []struct {
		width, height, monitorWidth, monitorHeight, wantWidth, wantHeight int
	}{
		{1280, 720, 1920, 1080, 1280, 720},  // fits
		{1920, 1080, 1920, 1080, 1728, 972}, // the monitor's size: nine tenths of it
		{1920, 1080, 1366, 768, 1228, 691},  // larger than the monitor
		{1000, 1200, 1920, 1080, 810, 972},  // the height decides
		{1920, 1080, 0, 0, 1920, 1080},      // no monitor
	}
	for _, tt := range tests {
		width, height := windowFit(tt.width, tt.height, tt.monitorWidth, tt.monitorHeight)
		if width != tt.wantWidth || height != tt.wantHeight {
			t.Errorf("windowFit(%d, %d, %d, %d) = %d, %d, want %d, %d", tt.width, tt.height, tt.monitorWidth, tt.monitorHeight, width, height, tt.wantWidth, tt.wantHeight)
		}
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

func TestWindowScreenSize(t *testing.T) {
	config := Config{Width: 1280, Height: 720, WindowScale: 2}
	for _, tt := range []struct {
		windowWidth, windowHeight int
		wantWidth, wantHeight     int
	}{
		{1280, 720, 640, 360},
		{1024, 600, 512, 300},
		{1365, 767, 682, 383},
		{0, 0, 1, 1},
	} {
		width, height := windowScreenSize(
			config, tt.windowWidth, tt.windowHeight,
		)
		if width != tt.wantWidth || height != tt.wantHeight {
			t.Errorf("window %dx%d: screen = %dx%d, want %dx%d",
				tt.windowWidth, tt.windowHeight, width, height,
				tt.wantWidth, tt.wantHeight)
		}
	}
	config.WindowScaleMinWidth = 1025
	for _, tt := range []struct {
		windowWidth, windowHeight int
		wantWidth, wantHeight     int
	}{
		{800, 600, 800, 600},
		{1024, 768, 1024, 768},
		{1025, 768, 512, 384},
		{1280, 720, 640, 360},
		{1024, 768, 1024, 768},
		{0, 0, 1, 1},
	} {
		width, height := windowScreenSize(
			config, tt.windowWidth, tt.windowHeight,
		)
		fit := fitScreen(float32(width), float32(height),
			float32(tt.windowWidth), float32(tt.windowHeight), true)
		if width != tt.wantWidth || height != tt.wantHeight {
			t.Errorf("threshold window %dx%d: screen = %dx%d, want %dx%d",
				tt.windowWidth, tt.windowHeight, width, height,
				tt.wantWidth, tt.wantHeight)
		}
		if tt.windowWidth == 1024 && fit.Width != 1024 {
			t.Errorf("native screen is still enlarged: fit = %+v", fit)
		}
		if tt.windowWidth == 1025 && fit.Width != 1024 {
			t.Errorf("retro screen is not enlarged 2x: fit = %+v", fit)
		}
	}
	config.WindowScale = 0
	width, height := windowScreenSize(config, 1024, 600)
	if width != 1280 || height != 720 {
		t.Errorf("fixed screen = %dx%d, want 1280x720", width, height)
	}
}

// A game in fullscreen stays there. The web backend reports a player leaving
// fullscreen, which a browser lets them do, and the game's own idea of it
// follows; on the desktop nothing but the game takes it away, so a frame must
// never drop it by itself.
func TestFullscreenIsKeptFrameAfterFrame(t *testing.T) {
	openTestWindow(t, 64, 64)
	t.Cleanup(func() { SetFullscreen(false) })

	// A frame changes the window, so it runs on the main thread (see
	// onMainThread).
	var display window
	frame := func() { onMainThread(func() { display.apply(false) }) }
	SetFullscreen(true)
	frame()
	if !IsFullscreen() {
		t.Fatal("the frame that switched to fullscreen left the game windowed")
	}
	for i := range 3 {
		frame()
		if !IsFullscreen() {
			t.Fatalf("frame %d dropped the fullscreen the game asked for", i+2)
		}
	}

	// And the game can leave it again.
	SetFullscreen(false)
	frame()
	if IsFullscreen() {
		t.Error("the game asked to leave fullscreen and stayed in it")
	}
}
