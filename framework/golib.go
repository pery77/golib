// Package golib is a small framework for making games in Go, on top of raylib.
//
// A game is a type that implements [Game]. [Run] opens the window and calls the
// game's Update and Draw methods once per frame until the window closes:
//
//	package main
//
//	import (
//		"log"
//
//		"golib"
//	)
//
//	type game struct{}
//
//	func (g *game) Update(dt float32) {}
//
//	func (g *game) Draw(screen *golib.Screen) {
//		screen.Clear(golib.RayWhite)
//		screen.DrawText("Hello!", 40, 40, 40, golib.DarkGray)
//	}
//
//	func main() {
//		if err := golib.Run(&game{}, golib.Config{Title: "Hello"}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// raylib stays reachable: import github.com/gen2brain/raylib-go/raylib for
// anything GoLib doesn't cover.
package golib

import (
	"errors"
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	defaultTitle  = "GoLib"
	defaultWidth  = 1280
	defaultHeight = 720
	targetFPS     = 60
)

// Config describes the game window. Fields left at their zero value get the
// default shown in their comment.
type Config struct {
	Title  string // Window title. Default: "GoLib".
	Width  int    // Window width in pixels. Default: 1280.
	Height int    // Window height in pixels. Default: 720.
}

// Game is the interface every GoLib game implements.
type Game interface {
	// Update advances the game by dt, the time in seconds since the previous
	// frame. Read input and change the game state here.
	Update(dt float32)

	// Draw draws the current state on screen. It must not change the game state.
	Draw(screen *Screen)
}

// Run opens the window described by config, then calls game.Update and
// game.Draw every frame, aiming for 60 frames per second, until the window is
// closed or Esc is pressed. Call it once, from main.
//
// Run returns an error if game is nil, config is invalid or the window can't
// be opened.
func Run(game Game, config Config) error {
	if game == nil {
		return errors.New("golib.Run: game is nil: pass a value that implements golib.Game")
	}
	config, err := config.resolve()
	if err != nil {
		return err
	}

	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(int32(config.Width), int32(config.Height), config.Title)
	if !rl.IsWindowReady() {
		return fmt.Errorf("golib.Run: could not open a %dx%d window: see the raylib warnings above", config.Width, config.Height)
	}
	defer rl.CloseWindow()
	rl.SetTargetFPS(targetFPS)

	screen := &Screen{}
	for !rl.WindowShouldClose() {
		game.Update(rl.GetFrameTime())
		rl.BeginDrawing()
		game.Draw(screen)
		rl.EndDrawing()
	}
	return nil
}

// resolve returns the config with defaults applied, or an error if a field is
// invalid.
func (c Config) resolve() (Config, error) {
	if c.Title == "" {
		c.Title = defaultTitle
	}
	if c.Width == 0 {
		c.Width = defaultWidth
	}
	if c.Height == 0 {
		c.Height = defaultHeight
	}
	if c.Width < 0 || c.Height < 0 {
		return c, fmt.Errorf("golib.Run: invalid window size %dx%d: use positive sizes, or 0 for the default", c.Width, c.Height)
	}
	return c, nil
}
