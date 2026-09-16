// Package golib is a small framework for making games in Go, on top of raylib.
//
// A game is a type that implements [Game]. [Run] opens the window and calls the
// game's Update and Draw methods until the window closes or the game calls
// [Quit]. This one moves a square with the arrow keys:
//
//	package main
//
//	import (
//		"log"
//
//		"golib"
//	)
//
//	type game struct {
//		x float32 // pixels from the left edge
//	}
//
//	func (g *game) Update(input *golib.Input, dt float32) {
//		if input.KeyDown(golib.KeyRight) {
//			g.x += 200 * dt // 200 pixels per second
//		}
//		if input.KeyDown(golib.KeyLeft) {
//			g.x -= 200 * dt
//		}
//	}
//
//	func (g *game) Draw(screen *golib.Screen) {
//		screen.Clear(golib.RayWhite)
//		screen.DrawRectangle(golib.Rectangle{X: g.x, Y: 300, Width: 40, Height: 40}, golib.Maroon)
//	}
//
//	func main() {
//		if err := golib.Run(&game{}, golib.Config{Title: "Square"}); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// games/platformer in the GoLib template is a complete example, and README.md
// in this package's folder is a guide to the whole package, by task.
//
// # Time
//
// Run calls Update 60 times per second of game time, always with dt = 1/60,
// and Draw once per frame, whatever the display's refresh rate. Base all
// timing on dt, never on the wall clock, so the game plays the same on every
// machine and in screenshots.
//
// # Screenshots
//
// golib shot runs a game in a hidden window for a number of frames and saves
// screenshots of chosen frames. Run handles this by itself, so games need no
// code for it.
//
// # Random numbers
//
// [RandomInt] and [RandomFloat] give different numbers on every run, except
// under golib shot, which starts them from the same seed so that screenshots
// repeat. Use them rather than math/rand, and call [SetRandomSeed] at the
// start of tests that use them.
//
// # Window, fullscreen and post-processing
//
// The game draws on a screen of Config.Width by Config.Height pixels, which Run
// scales to fit the window, so games never deal with the window's size.
// [SetFullscreen] switches to fullscreen and back. [SetPostProcess] runs
// shaders made with [NewShader] over the whole picture, for effects such as
// scanlines or a glow.
//
// # Sprites
//
// [NewSprite] reads a picture from the game's assets folder: a PNG image, or
// an Aseprite file with its frames and the animations its tags describe.
// [NewSpriteSheet] cuts a PNG image into a grid of frames. [Screen.DrawSprite]
// draws a frame, and an [Animation] says which frame to show as time passes.
//
// # Fonts
//
// [Screen.DrawText] writes in a built-in pixel font, or in a TrueType or
// OpenType font read with [NewFont] and passed in [TextOptions].
//
// # Maps
//
// [NewMap] reads a level made in Tiled. [Screen.DrawMap] draws it, and
// [Map.TilesIn], [Map.Objects] and the [Properties] Tiled gives tiles and
// objects tell the game what is where.
//
// # Sound and music
//
// [NewSound] makes a sound effect from a [SoundSpec], so games need no sound
// files: [Laser], [Explosion], [Pickup], [Jump], [Hurt] and [PowerUp] are
// ready-made recipes to start from. [NewSoundFile] plays a sound effect from a
// file in the game's assets folder, [NewMusic] streams music from one, and
// [SetVolume] sets how loud everything is. golib shot runs without a sound
// device, so screenshots stay silent.
//
// # Quitting
//
// The game ends when the player closes the window or the game calls [Quit].
// No key quits on its own, not even Esc.
//
// # Scenes
//
// A game with several screens, such as a title, the game itself, a pause
// screen and a game over screen, makes each one a scene: a value that
// implements [Game]. Run starts with the scene it is given, and [SwitchScene]
// moves to another one.
//
// # Assets
//
// A game keeps the files it loads in its assets folder and reads them with
// [ReadAsset]. Debug builds (golib build, run, shot and test) read them from
// disk. golib dist builds carry the assets inside the executable, so they
// read a copy embedded with [EmbedAssets].
//
// raylib stays reachable: import github.com/gen2brain/raylib-go/raylib for
// anything GoLib doesn't cover.
package golib

import (
	"errors"
	"fmt"
	"os"

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
//
// Width and Height are the size of the screen the game draws on, which never
// changes. The window opens at that size, or with PixelArt at the largest
// whole multiple of it that fits the monitor, and the player can resize it or go
// fullscreen: Run scales the screen to fit, with black bars where the shapes
// differ, and reports mouse positions in screen pixels.
type Config struct {
	Title      string // Window title. Default: "GoLib".
	Width      int    // Screen width in pixels. Default: 1280.
	Height     int    // Screen height in pixels. Default: 720.
	Fullscreen bool   // Start in fullscreen (see SetFullscreen). Default: in a window.

	// PixelArt scales the screen by whole numbers only, without smoothing,
	// so every pixel stays square and sharp, and opens the window as many
	// times larger than the screen as fits the monitor. Use it with a small
	// screen, such as 320 by 180. Default: smooth scaling to any size.
	PixelArt bool
}

// Game is the interface every GoLib game implements.
type Game interface {
	// Update advances the game by dt seconds. Read input and change the game
	// state here. dt is always 1/60: Run calls Update 60 times per second of
	// game time.
	Update(input *Input, dt float32)

	// Draw draws the current state on screen. It must not change the game
	// state. Run calls it once per frame, after zero, one or more updates.
	Draw(screen *Screen)
}

// Run opens the window described by config and runs game until the window is
// closed or the game calls Quit, calling game.Update 60 times per second and
// game.Draw once per frame. Call it once, from main.
//
// When golib shot starts the game, Run instead renders the requested frames in
// a hidden window, saves them as PNG files and returns.
//
// Run returns an error if game is nil, config is invalid, the window can't be
// opened, a file can't be read or a screenshot can't be saved. When the
// console can't show that error, Run also shows it, or a panic in the game, in
// a message box: a golib dist build on Windows has no console, and a debug
// build started from Explorer has a console window of its own, which closes
// as soon as the game ends.
func Run(game Game, config Config) (err error) {
	if distBuild || ownConsole() {
		title := config.Title
		if title == "" {
			title = defaultTitle
		}
		defer reportInDialog(title, &err)
	}
	// Forget requests made before Run started.
	quitRequested.Store(false)
	takeSceneRequest()
	if game == nil {
		return errors.New("golib.Run: game is nil: pass a value that implements golib.Game")
	}
	// A mistake made before Run, such as asking a sprite in a package
	// variable for an animation it doesn't have, stops the game now.
	if err := takeError(); err != nil {
		return err
	}
	config, err = config.resolve()
	if err != nil {
		return err
	}
	fullscreenWanted.Store(config.Fullscreen)
	plan, err := shotPlanFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	if plan != nil {
		return runShots(game, config, plan)
	}
	return runWindow(game, config)
}

// runWindow is the normal game loop: fixed-step updates, then one draw per
// frame, post-processed and scaled to fit the window.
func runWindow(game Game, config Config) error {
	if err := openWindow(config, false); err != nil {
		return err
	}
	defer rl.CloseWindow()
	rl.SetTargetFPS(targetFPS)
	audio.open()
	defer audio.close()
	render := newRenderer(config)
	defer render.close()

	screenWidth, screenHeight := float32(config.Width), float32(config.Height)
	screen := &Screen{width: screenWidth, height: screenHeight}
	var (
		gameClock clock
		queue     inputQueue
		input     Input
		display   window
		updates   int // run so far, for the time uniform of shaders
	)
	scene := game
	last := rl.GetTime()
	for !rl.WindowShouldClose() {
		display.apply()
		fit := fitScreen(screenWidth, screenHeight, float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()), config.PixelArt)

		now := rl.GetTime()
		queue.readKeyboard(raylibKeyDown, raylibKeyPressed)
		mouse := rl.GetMousePosition()
		mouseX, mouseY := toScreen(mouse.X, mouse.Y, fit, screenWidth, screenHeight)
		queue.readMouse(mouseX, mouseY, rl.GetMouseWheelMove(), raylibMouseDown, raylibMousePressed)
		queue.readGamepads(raylibGamepadFrame)
		frameUpdates := gameClock.advance(now - last)
		var (
			quit bool
			err  error
		)
		scene, quit, err = runUpdates(scene, &input, queue.next, frameUpdates)
		if err != nil || quit {
			return err
		}
		updates += frameUpdates
		last = now

		if err := audio.updateMusic(); err != nil {
			return err
		}
		screen.time = float32(updates) * updateStep
		if err := render.drawScene(scene, screen); err != nil {
			return err
		}
		if err := render.present(nil, fit, float32(updates)*updateStep); err != nil {
			return err
		}
	}
	return nil
}

// runUpdates runs updates updates of scene. Before each one, fill sets the
// input it sees. After each one, runUpdates switches to the scene passed to
// SwitchScene, if any, and stops if the game called Quit: no update runs after
// that. It returns the scene to draw and whether the game quit.
func runUpdates(scene Game, input *Input, fill func(*Input), updates int) (Game, bool, error) {
	for ; updates > 0; updates-- {
		fill(input)
		scene.Update(input, updateStep)
		if err := takeError(); err != nil {
			return scene, false, err
		}
		if quitRequested.Load() {
			return scene, true, nil
		}
		if next, ok := takeSceneRequest(); ok {
			if next == nil {
				return scene, false, errors.New("golib.SwitchScene: the next scene is nil: pass a value that implements golib.Game")
			}
			scene = next
		}
	}
	return scene, false, nil
}

// openWindow opens the game window, hidden if asked to.
func openWindow(config Config, hidden bool) error {
	rl.SetTraceLogLevel(rl.LogWarning)
	if hidden {
		rl.SetConfigFlags(rl.FlagWindowHidden)
	} else {
		// Run scales the screen to fit, so the player may resize the window.
		rl.SetConfigFlags(rl.FlagWindowResizable)
	}
	rl.InitWindow(int32(config.Width), int32(config.Height), config.Title)
	if !rl.IsWindowReady() {
		return fmt.Errorf("golib.Run: could not open a %dx%d window: see the raylib warnings above", config.Width, config.Height)
	}
	// raylib closes the window when Esc is pressed. GoLib leaves every key to
	// the game, which calls Quit to end.
	rl.SetExitKey(rl.KeyNull)
	if !hidden {
		rl.SetWindowMinSize(max(config.Width/4, 1), max(config.Height/4, 1))
		if config.PixelArt {
			enlargeWindow(config.Width, config.Height)
		}
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
