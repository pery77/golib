package golib

import (
	"os"
	"runtime"
	"testing"

	"golib/internal/device"
)

type resizingGame struct {
	updates int
	width   int
	height  int
	resized bool
}

func (g *resizingGame) Update(_ *Input, _ float32) {
	g.updates++
	if g.updates == 1 {
		device.SetWindowSize(600, 400)
	}
	if (g.resized && g.updates > 4) || g.updates >= 120 {
		Quit()
	}
}

func (g *resizingGame) Draw(screen *Screen) {
	screen.Clear(Black)
	if screen.Width() == 300 && screen.Height() == 200 &&
		g.width == 300 && g.height == 200 {
		g.resized = true
	}
}

func TestScreenFollowsWindowResize(t *testing.T) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" &&
		os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no display to open a window on")
	}
	defer quitRequested.Store(false)
	game := &resizingGame{}
	config := Config{
		Title: "Resize test", Width: 400, Height: 300,
		WindowScale: 2,
		OnScreenResize: func(width, height int) {
			game.width, game.height = width, height
		},
	}
	// Run opens the window, so it runs on the main thread (see onMainThread).
	var err error
	onMainThread(func() { err = Run(game, config) })
	if err != nil {
		t.Fatal(err)
	}
	if !game.resized {
		t.Errorf("resize not seen in Draw: callback %dx%d after %d updates",
			game.width, game.height, game.updates)
	}
}
