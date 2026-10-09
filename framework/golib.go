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
// machine and in screenshots. A world that must follow the wall clock, such as
// one that goes on while the player is in another program, reads it with
// [Now], which follows the updates under golib shot.
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
// By default the game draws on a screen of Config.Width by Config.Height
// pixels, which Run scales to fit the window. Config.WindowScale can instead
// keep the screen at a fraction of the window's drawing area; Config.FillWindow
// lets it take the window's shape instead, with no black bars.
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

	"golib/internal/device"
	"golib/internal/startup"
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
// Width and Height set the screen's size, fixed unless WindowScale or
// FillWindow is used.
// The window opens at that size, smaller with the same shape when it doesn't
// fit the monitor, or with PixelArt, unless WindowScale is set, at the largest
// whole multiple of it that fits the monitor. Run scales the screen to the window
// and reports mouse positions in screen pixels.
type Config struct {
	Title      string // Window title. Default: "GoLib".
	Width      int    // Screen width in pixels. Default: 1280.
	Height     int    // Screen height in pixels. Default: 720.
	Fullscreen bool   // Start in fullscreen (see SetFullscreen). Default: in a window.
	// WindowScale makes the screen follow the window's drawing area: each
	// screen pixel covers this many window pixels. For example, 2 renders at
	// half the window width and height, including after a resize. Zero keeps
	// Width and Height fixed unless FillWindow is set. Takes precedence over
	// FillWindow. Screenshots always use Width and Height.
	WindowScale int
	// WindowScaleMinWidth uses native resolution below this drawing-area
	// width, and WindowScale at or above it. Zero disables the threshold.
	// Ignored when WindowScale is zero, and for screenshots.
	WindowScaleMinWidth int
	// OnScreenResize is called before the first Update or Draw, and again
	// whenever WindowScale changes the screen size. A game can update its
	// layout and camera here. It is not called for screenshots.
	OnScreenResize func(width, height int)

	// PauseUnfocused stops the game while its window doesn't have the
	// player's attention, such as while they work in another program, and
	// carries on where it was when they come back. Run calls no update
	// meanwhile, and drops the time that passed, so the game never catches
	// up in a burst. Draw keeps running, so the window still shows
	// the game, and sounds and music play on: a game that wants silence
	// stops them itself, and [WindowFocused] tells it when to. Default: the
	// game keeps playing without the player.
	PauseUnfocused bool

	// PixelArt scales the screen by whole numbers only, without smoothing,
	// so every pixel stays square and sharp, and opens the window as many
	// times larger than the screen as fits the monitor. Use it with a small
	// screen, such as 320 by 180. Default: smooth scaling to any size.
	PixelArt bool

	// FillWindow makes the screen take the window's shape, so there are no
	// black bars, in a window or in fullscreen: Width by Height is the
	// smallest it gets, and it grows wider or taller to match the window,
	// scaled as it would be without the option. A 1280 by 720 screen is 1280
	// by 800 on a 1920 by 1200 monitor, and 1720 by 720 on an ultrawide one.
	// Screen.Width and Screen.Height say its size in each Draw: lay the game
	// out from them, not from the Config, and keep them for Update, which
	// runs before Draw. A camera used with Screen.SetCamera follows the
	// screen's size. With PixelArt, the scale stays a whole number and the
	// screen covers the window, cutting less than one of its pixels at the
	// edges. Screenshots from golib shot are Width by Height. Default: the
	// screen keeps its size and shape. Ignored when WindowScale is positive.
	FillWindow bool

	// FixedWindowSize stops the player resizing the window by its edges or
	// maximizing it, for a game that offers window sizes in its settings
	// with SetWindowSize. SetWindowSize and SetFullscreen still change it.
	// Ignored in a browser, where the canvas follows the page. Default: the
	// player resizes the window freely.
	FixedWindowSize bool

	// Antialias smooths the edges of what the game draws: circles, lines,
	// polygons and turned shapes get soft edges instead of steps. GoLib draws
	// the screen at twice its width and height and shrinks it back, blending
	// every four pixels into one, before the post-processing shaders and the
	// window get it, so sizes, cameras, text and shaders work as they do
	// without it. Text in a font from a file is drawn twice as large too, so it
	// stays sharp in a window larger than the screen. It costs four times the
	// pixels to draw, which a 2D game hardly notices. Ignored with PixelArt,
	// whose steps are its look. Default: off.
	Antialias bool
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
	if startup.ErrorsUnseen() || device.ShowsErrors {
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
	defer device.CloseWindow()
	windowOpen.Store(true)
	defer windowOpen.Store(false)
	showGameIcon()
	device.SetTargetFPS(targetFPS)
	lookAtMonitors()
	audio.open()
	defer audio.close()
	windowWidth, windowHeight := device.WindowSize()
	width, height, _ := screenInWindow(
		config, float32(windowWidth), float32(windowHeight),
	)
	renderConfig := config
	renderConfig.Width, renderConfig.Height = int(width), int(height)
	render := newRenderer(renderConfig)
	defer render.close()
	if config.WindowScale != 0 && config.OnScreenResize != nil {
		config.OnScreenResize(int(width), int(height))
	}

	screen := &Screen{
		width: width, height: height,
		fills: config.FillWindow && config.WindowScale == 0,
	}
	var (
		gameClock clock
		queue     inputQueue
		input     Input
		display   window
		updates   int // run so far, for the time uniform of shaders
		frames    int // drawn so far
		counter   frameCounter
		touches   []Touch // this frame's fingers, kept between frames to reuse

		// The mouse pointer as the frame before saw it, for whether it moved.
		pointerKnown               bool
		lastPointerX, lastPointerY float32
	)
	// A phone or a tablet is played with fingers from the first frame; anywhere
	// else the game waits for one, so a computer with a touch screen and a
	// mouse doesn't start with on-screen controls over the game.
	playingWithTouch.Store(device.TouchScreen())
	scene := game
	last := device.Time()
	for !device.WindowShouldClose() {
		display.apply(queue.held())
		if frames++; frames%targetFPS == 0 {
			lookAtMonitors() // once a second, for a monitor plugged in or out
		}
		focused := device.WindowFocused()
		windowUnfocused.Store(!focused)
		device.MeasureWindow() // on macOS, raylib can keep a wrong size from while the window opened
		windowWidth, windowHeight := device.WindowSize()
		screenWidth, screenHeight, fit := screenInWindow(
			config, float32(windowWidth), float32(windowHeight),
		)
		if config.WindowScale != 0 && (windowWidth <= 0 || windowHeight <= 0) {
			screenWidth, screenHeight = screen.width, screen.height
			fit = fitScreen(screenWidth, screenHeight,
				float32(windowWidth), float32(windowHeight), config.PixelArt,
			)
		}
		if screenWidth != screen.width || screenHeight != screen.height {
			render.resize(screenWidth, screenHeight)
			screen.width, screen.height = screenWidth, screenHeight
			if config.WindowScale != 0 && config.OnScreenResize != nil {
				config.OnScreenResize(int(screenWidth), int(screenHeight))
			}
		}

		now := device.Time()
		counter.count(now)
		queue.readKeyboard(deviceKeyDown, deviceKeyPressed)
		queue.readText(device.TypedText())
		pointerX, pointerY := device.MousePosition()
		mouseX, mouseY := toScreen(pointerX, pointerY, fit, screenWidth, screenHeight)
		spriteX, spriteY := mouseX, mouseY // the mouse sprite follows the mouse, not a finger
		// The oldest finger on a touch screen moves the mouse pointer and holds
		// its left button, so a tap works a game written for a mouse.
		touches = deviceTouches(touches, fit, screenWidth, screenHeight)
		if len(touches) > 0 {
			mouseX, mouseY = touches[0].Position.X, touches[0].Position.Y
		}
		mouseDown, mousePressed := mouseWithTouch(touches, deviceMouseDown, deviceMousePressed)
		wheel := device.MouseWheel()
		queue.readMouse(mouseX, mouseY, wheel, mouseDown, mousePressed)
		queue.readTouches(touches)
		queue.readGamepads(deviceGamepadFrame)
		// Whether the game shows on-screen controls follows what the player
		// last used, so one web build is played with fingers on a phone and
		// with the keyboard and the mouse on a computer, and so does whether
		// its prompts show gamepad buttons or keys. The mouse is read from
		// the machine, not from queue: a finger holds its left button too.
		mouseUsed := pointerKnown && (pointerX != lastPointerX || pointerY != lastPointerY)
		lastPointerX, lastPointerY, pointerKnown = pointerX, pointerY, true
		mouseUsed = mouseUsed || wheel != 0 ||
			deviceMouseDown(MouseLeft) || deviceMouseDown(MouseRight) || deviceMouseDown(MouseMiddle)
		followTouchPlaying(len(touches) > 0, queue.keyboardUsed, mouseUsed, queue.gamepadUsed)
		followGamepadPlaying(len(touches) > 0, queue.keyboardUsed, mouseUsed, queue.gamepadUsed)
		frameUpdates := gameClock.advanceUnlessPaused(now-last, !focused && config.PauseUnfocused)
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
		render.pointer = pointerAt(spriteX, spriteY, device.MouseInWindow())
		if err := render.present(nil, fit, float32(updates)*updateStep); err != nil {
			return err
		}
	}
	return nil
}

func windowScreenSize(config Config, windowWidth, windowHeight int) (int, int) {
	if config.WindowScale == 0 {
		return config.Width, config.Height
	}
	scale := config.WindowScale
	if windowWidth < config.WindowScaleMinWidth {
		scale = 1
	}
	return max(1, windowWidth/scale), max(1, windowHeight/scale)
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
	// Run scales the screen to fit, so the player may resize the window,
	// unless the game offers its own sizes.
	if !device.OpenWindow(config.Width, config.Height, config.Title, hidden, !config.FixedWindowSize) {
		return fmt.Errorf("golib.Run: could not open a %dx%d window: see the raylib warnings above", config.Width, config.Height)
	}
	if !hidden {
		device.SetWindowMinSize(max(config.Width/4, 1), max(config.Height/4, 1))
		if config.PixelArt && config.WindowScale == 0 {
			enlargeWindow(config.Width, config.Height)
		} else {
			shrinkWindow(config.Width, config.Height)
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
	if c.PixelArt {
		c.Antialias = false // pixel art keeps its steps
	}
	if c.WindowScale < 0 {
		return c, fmt.Errorf("golib.Run: invalid window scale %d: use a positive scale, or 0 for a fixed screen", c.WindowScale)
	}
	if c.WindowScaleMinWidth < 0 {
		return c, fmt.Errorf(
			"golib.Run: invalid window scale minimum width %d: "+
				"use a positive width, or 0 for no threshold",
			c.WindowScaleMinWidth,
		)
	}
	return c, nil
}
