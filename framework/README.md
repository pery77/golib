# GoLib API guide

For AI agents writing game code with GoLib, and for anyone who wants the whole framework on one page. It covers every exported name in package `golib`, the Go code in this folder, grouped by what a game needs, with the rules the names don't tell you. The doc comments in the `.go` files have the full details, and [games/platformer](../games/platformer) and [games/asteroids](../games/asteroids) show the 2D in real games, and [games/marbles](../games/marbles), [games/hopper](../games/hopper) and [games/sunset](../games/sunset) the 3D: shapes, models and a Blender level, and light, an HDRI and materials of a game's own.

`golib test` checks this page against the code: it fails when an exported name is missing here, or when this page names one that doesn't exist (see `apiguide_test.go`). Change both together.

| To | Use | Section |
| --- | --- | --- |
| Start the game | `golib.Run(firstScene, golib.Config{...})` in `main` | [Game, Run and Config](#game-run-and-config) |
| Change the game 60 times per second | `Update(input *golib.Input, dt float32)` | [Game, Run and Config](#game-run-and-config) |
| Draw it | `Draw(screen *golib.Screen)` | [Drawing](#drawing) |
| Write text in a font | `golib.NewFont`, `screen.DrawText` with `golib.TextOptions` | [Fonts](#fonts) |
| Draw pictures and animations | `golib.NewSprite`, `golib.NewSpriteSheet`, `screen.DrawSprite`, `golib.Animation` | [Sprites and animations](#sprites-and-animations) |
| Draw a picture pixel by pixel, such as an emulator's screen | `golib.NewImage`, `image.SetPixel`, `screen.DrawImage` | [Pictures made pixel by pixel](#pictures-made-pixel-by-pixel) |
| Load levels made in Tiled | `golib.NewMap`, `screen.DrawMap`, `level.TilesIn`, `level.Objects` | [Maps](#maps) |
| Read keys, mouse and gamepads | `input.KeyDown`, `input.KeyPressed`, `input.MousePosition`, `input.GamepadDown` | [Input](#input) |
| Show the player the right key or gamepad button to press | `golib.PlayingWithGamepad`, `input.GamepadType` | [Gamepads](#gamepads) |
| Move, aim and chase | `golib.Vector2` and its `Add`, `Scale`, `Normalize`, `MoveTowards` | [Vectors, rectangles and collisions](#vectors-rectangles-and-collisions) |
| Check collisions | `golib.Rectangle` and its `Overlaps`; `Vector2.Distance` for circles | [Vectors, rectangles and collisions](#vectors-rectangles-and-collisions) |
| Scroll a world larger than the screen | `golib.NewCamera`, `screen.SetCamera` | [Camera](#camera) |
| Draw a 3D world | `golib.NewCamera3D`, `screen.Begin3D`, `screen.DrawMesh`, `golib.NewBoxMesh`, `golib.Vector3`, `golib.Transform` | [3D](#3d) |
| Move between title, play, pause and game over | `golib.SwitchScene` | [Scenes](#scenes) |
| Roll dice | `golib.RandomInt`, `golib.RandomFloat` | [Random numbers](#random-numbers) |
| Play sound effects | `golib.NewSound`, `golib.Laser` and the other recipes, `golib.NewSoundFile` for sound files and sounds designed in jfxr, `golib.NewSoundJfxr` for jfxr settings in code | [Sound effects](#sound-effects) |
| Play music | `golib.NewMusic` | [Music](#music) |
| Go fullscreen, add a CRT look | `golib.SetFullscreen`, `golib.NewShader`, `golib.SetPostProcess` | [Window, fullscreen and screen effects](#window-fullscreen-and-screen-effects) |
| Open the window as the player left it | `golib.WindowSize`, `golib.SetWindowSize`, `golib.SetMonitor` | [Window, fullscreen and screen effects](#window-fullscreen-and-screen-effects) |
| Remember high scores, settings and progress | `golib.SaveData`, `golib.LoadData` | [Saving data](#saving-data) |
| Start in the player's language | `golib.ChooseLanguage`, `golib.SystemLanguages` | [The player's language](#the-players-language) |
| Show which build the player has | `golib.GameVersion` | [The game's version](#the-games-version) |
| Read a data file | `golib.ReadAsset` | [Files: the assets folder](#files-the-assets-folder) |
| End the game | `golib.Quit` | [Quitting](#quitting) |

## A whole game

```go
package main

import (
	"log"

	"golib"
)

// game is the only scene: a square that moves with the arrow keys.
type game struct {
	x float32 // pixels from the left edge
}

func (g *game) Update(input *golib.Input, dt float32) {
	if input.KeyDown(golib.KeyRight) {
		g.x += 200 * dt // 200 pixels per second
	}
	if input.KeyDown(golib.KeyLeft) {
		g.x -= 200 * dt
	}
	if input.KeyPressed(golib.KeyEscape) {
		golib.Quit()
	}
}

func (g *game) Draw(screen *golib.Screen) {
	screen.Clear(golib.RayWhite)
	screen.DrawRectangle(golib.Rectangle{X: g.x, Y: 340, Width: 40, Height: 40}, golib.Maroon)
	screen.DrawText("Arrows to move, Esc to quit", 20, 20, 30, golib.DarkGray)
}

func main() {
	if err := golib.Run(&game{}, golib.Config{Title: "Square"}); err != nil {
		log.Fatal(err)
	}
}
```

`golib new <name>` writes a game like this into `games/<name>/`, already split into `main.go`, `play.go` and `world.go`, with the `go.mod` that points the `"golib"` import at `../../framework`.

## Rules the names don't tell you

1. **`dt` is always 1/60 of a second.** Time everything with it (`timer -= dt`), never with `time.Now`. Games then play the same at any frame rate, on any machine and in `golib shot`. A world that must follow the wall clock reads `Now` instead (see [Time](#time)).
2. **`Update` changes the game; `Draw` only draws.** `Run` draws once per frame, after zero, one or several updates, so a `Draw` that changed the game would make it play differently at each frame rate.
3. **A scene must be ready to draw as soon as it is made.** Set its state in its constructor, not in its first `Update`: `Run` can draw a scene before updating it, on the first frame and on the frame a `SwitchScene` lands.
4. **Pressed happens once, Down lasts.** `KeyPressed`, `MousePressed` and `GamepadPressed` are true in exactly one update per press: use them for jumping, firing and menus. `KeyDown`, `MouseDown` and `GamepadDown` stay true while held: use them for walking and thrust.
5. **Use `Input` only in `Update`, and `Screen` only in `Draw`.** Don't keep them in the game's state.
6. **Draw in screen pixels, or through a camera.** By default the screen is `Config.Width` by `Config.Height` pixels, with 0, 0 at the top-left corner and y growing downwards, whatever the window's size. With `Config.WindowScale` it follows the window instead: use `Config.OnScreenResize` to update the layout and camera. With `Config.FillWindow` it takes the window's shape, so there are no bars; `screen.Width()` and `screen.Height()` say its size. `WindowScale` takes precedence when both are set. `Run` scales the screen to the window and reports the mouse in the same pixels. After `screen.SetCamera(camera)`, positions are in the world instead, until `screen.SetCamera(nil)`.
7. **Make sprites, maps, fonts, meshes, sounds, music and shaders once, and keep them.** Create them as package variables or in `main`, never in `Update`, in `Draw` or in a scene that is made again for every new game: each one loads the first time it is used and stays loaded until `Run` returns. Making one reads nothing; asking it something does (`Markers`, `Bounds`, `Err`, `Animations`): ask in `main` or in a scene, never in a package variable's initializer, which runs before `assets.go` gives a dist or web build its assets, so the file isn't found there.
8. **Random numbers come from `RandomInt` and `RandomFloat`**, never from `math/rand`: `golib shot` starts them from the same seed, so its screenshots repeat.
9. **No key quits on its own, not even Esc.** Call `Quit` when the game should end; the game decides what Esc does.
10. **`SwitchScene` and `Quit` don't stop the current update.** The code after them still runs, so `return` right after unless that is what you want.
11. **Mistakes stop the game with a message.** `Run` returns an error for a nil scene, a shader that doesn't compile, a bad `SetUniform`, a sprite, map, font, sound or music file that is missing or can't be read, a frame or animation a sprite doesn't have, or a layer a map doesn't have. `main` prints it with `log.Fatal`; read it, it says what to fix.
12. **Sound is silent in tests and in `golib shot`.** `Play` is safe to call from the rules anyway, so they stay testable. You can't hear a game: tell the user what to listen for.

## Game, Run and Config

| Name | What it does |
| --- | --- |
| `Game` | The interface every scene implements, with `Game.Update` and `Game.Draw`. A game is one scene or several. |
| `Game.Update` | `Update(input *Input, dt float32)`: read the input and change the game. `Run` calls it 60 times per second of game time, always with `dt` = 1/60. |
| `Game.Draw` | `Draw(screen *Screen)`: draw the game as it is. `Run` calls it once per frame. |
| `Run` | `Run(game Game, config Config) error`: opens the window and runs `game` until the window closes or the game calls `Quit`. Call it once, from `main`. Under `golib shot` it takes the screenshots instead, with no code in the game. |
| `Config` | The window. Fields left at zero get their default. |

| Field | Default | Meaning |
| --- | --- | --- |
| `Config.Title` | `"GoLib"` | Window title |
| `Config.Width`, `Config.Height` | 1280, 720 | Size of the screen the game draws on, in pixels. It never changes, unless `WindowScale` or `FillWindow` is set; the window scales it. `golib shot` always takes pictures of this size. The window opens at this size, or, when that doesn't fit in nine tenths of the monitor, smaller with the same shape (1728 by 972 for 1920 by 1080 on a 1920 by 1080 monitor), so its title bar and the taskbar stay in sight; `PixelArt` opens it larger instead, unless `WindowScale` is set. |
| `Config.Fullscreen` | `false` | Start in fullscreen |
| `Config.HighDPI` | `false` | Request the desktop display's physical framebuffer. With `WindowScale: 1`, a Retina window measuring 2560 by 1440 logical points draws at 5120 by 2880 physical pixels; resize callbacks and input use the screen's physical pixels. Scale the game's layout and camera accordingly. Window sizes still use desktop coordinates. Browsers already use physical canvas pixels. Screenshots keep `Width` by `Height`, independently of display density. |
| `Config.PauseUnfocused` | `false` | Stop the game while its window doesn't have the player's attention, such as while they work in another program, and carry on where it was. `Draw` keeps running, so the window still shows the game, and sound and music play on. Ask `WindowFocused` to draw a sign or quieten the music. |
| `Config.PixelArt` | `false` | Scale the screen by whole numbers only, without smoothing, so pixels stay square and sharp. Use it with a small screen, such as 320 by 180: the window opens as many times larger as fits in most of the monitor, 1280 by 720 on a 1920 by 1080 monitor. With `WindowScale`, the window opens as it does without `PixelArt`. |
| `Config.WindowScale` | 0 | When positive, the screen follows the window's drawing area, at this fraction of it: 2 makes it half the window's width and height, so a 1920 by 1080 window, or fullscreen on a 1920 by 1080 monitor, gives a 960 by 540 screen. It changes whenever the window does: when the player resizes it, with `SetWindowSize` and with `SetFullscreen`. Takes precedence over `FillWindow`. `golib shot` still takes `Width` by `Height` pictures. |
| `Config.WindowScaleMinWidth` | 0 | With `WindowScale`, one screen pixel for each window pixel while the drawing area is narrower than this, and the `WindowScale` fraction from this width up. Zero disables the threshold. For example, `WindowScale: 2, WindowScaleMinWidth: 1025` gives a 1024 by 576 screen in a 1024 by 576 window and a 960 by 540 one in a 1920 by 1080 window. Ignored without `WindowScale`, and by `golib shot`. |
| `Config.OnScreenResize` | nil | With `WindowScale`, called with the screen's width and height before the first `Update` and `Draw`, and again whenever its size changes. Lay the game out again there, and make its camera again with the new size, keeping its target and zoom. Not called by `golib shot`. |
| `Config.FixedWindowSize` | `false` | The player can't resize the window by its edges or maximize it, for a game that offers window sizes in its settings with `SetWindowSize`, which still changes it, as `SetFullscreen` does. Ignored in a browser, where the canvas follows the page. |
| `Config.FillWindow` | `false` | The screen takes the window's shape, in a window and in fullscreen, so there are no black bars: `Width` by `Height` is the smallest it gets, and it grows wider or taller at the same scale, 1280 by 800 on a 1920 by 1200 monitor and 1720 by 720 on an ultrawide one. For tools, editors and games whose layout can stretch. Lay the game out from `screen.Width()` and `screen.Height()` in each `Draw`, and keep them for `Update`, which runs first; a camera passed to `screen.SetCamera` takes the screen's size. With `PixelArt` the scale stays whole, and less than one screen pixel is cut at the edges. Ignored when `WindowScale` is positive. `golib shot` takes `Width` by `Height` pictures. |
| `Config.Antialias` | `false` | Smooth edges on what the game draws: circles, lines, polygons and turned shapes get soft edges instead of steps. GoLib draws the screen at twice its width and height and shrinks it back, blending every four pixels into one, before the post-processing shaders and the window get it, so sizes, cameras, text and shaders work as they do without it, and `golib shot` pictures show it. Text in a font from a file is drawn twice as large too, so it stays sharp in a window larger than the screen. It costs four times the pixels to draw, which a 2D game hardly notices. For games drawn with shapes; ignored with `PixelArt`, whose steps are its look. |

## Time

There is no clock to read: an update is 1/60 of a second of game time. Count time in the game's state:

```go
s.spawnTimer -= dt
if s.spawnTimer <= 0 {
	s.spawnTimer += spawnInterval // seconds
	s.spawnEnemy()
}

// For animation: pulse goes from 0 to 1 and back, twice per second.
s.time += dt
pulse := 0.5 + 0.5*float32(math.Sin(float64(s.time)*4*math.Pi))
```

A `Timer` does that counting, for cooldowns, waits and anything that happens every so often.

| Name | What it does |
| --- | --- |
| `Timer` | Counts seconds down. Keep it in the game's state; the zero `Timer` is stopped. |
| `NewTimer` | `NewTimer(seconds float32) Timer`: runs for `seconds` and stops, such as how long a shield lasts. |
| `NewRepeatingTimer` | `NewRepeatingTimer(seconds float32) Timer`: finishes every `seconds` and starts over, such as the wait between enemies. |
| `Timer.Tick` | `Tick(dt float32) bool`: counts `dt` off and reports whether the timer reached zero in this update. A stopped timer counts nothing and reports false, so `Update` can tick every timer it has. |
| `Timer.Start` | `Start(seconds float32)`: runs it for `seconds` from now, whatever it was doing. Zero or less stops it. |
| `Timer.Stop` | `Stop()`: stops it without letting it finish. |
| `Timer.Running` | `Running() bool`: it has time left. A cooldown is ready again when it isn't running. |
| `Timer.Left` | `Left() float32`: the seconds left, 0 when it isn't running, to draw a countdown. |
| `Timer.Progress` | `Progress() float32`: how far it has come, from 0 to 1, for bars and fades; a stopped timer is 1. |

```go
type playScene struct {
	spawn    golib.Timer
	cooldown golib.Timer
}

func newPlayScene() *playScene {
	return &playScene{spawn: golib.NewRepeatingTimer(2)} // an enemy every 2 seconds
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	if s.spawn.Tick(dt) {
		s.spawnEnemy()
	}
	s.cooldown.Tick(dt)
	if input.KeyDown(golib.KeySpace) && !s.cooldown.Running() {
		s.shoot()
		s.cooldown.Start(0.25) // no shot for a quarter of a second
	}
}
```

A paused game stops counting because its scene stops getting updates.

A world that follows the wall clock instead, such as one that keeps going while the player is in another program, reads it with `Now`, never with `time.Now`. Under `golib shot`, which runs updates as fast as it can, `Now` moves 1/60 of a second per update, so the world moves in screenshots as it does when played, and they repeat.

| Name | What it does |
| --- | --- |
| `Now` | `Now() time.Time`: the time, as `time.Now` gives it; under `golib shot`, the time the program started plus 1/60 of a second for every update run so far. |

```go
func (w *world) Update(input *golib.Input, dt float32) {
	now := golib.Now()
	w.grow(now.Sub(w.last)) // the seconds that passed, even while the player was away
	w.last = now
}
```

## Input

`Input` is the keyboard, the mouse, the touch screen and up to four gamepads, as one update sees them. Read the keyboard and gamepad 0 together, so every game works with both:

```go
var moveX float32
if input.KeyDown(golib.KeyLeft) || input.KeyDown(golib.KeyA) || input.GamepadDown(0, golib.GamepadLeft) {
	moveX = -1
}
if input.KeyDown(golib.KeyRight) || input.KeyDown(golib.KeyD) || input.GamepadDown(0, golib.GamepadRight) {
	moveX = 1
}
if moveX == 0 {
	moveX, _ = input.GamepadLeftStick(0) // analog: half a tilt, half the speed
}
jump := input.KeyPressed(golib.KeySpace) || input.GamepadPressed(0, golib.GamepadA)
s.world.step(moveX, jump, dt) // the rules see intentions, not keys
```

### Keyboard

| Name | What it does |
| --- | --- |
| `Input.KeyDown` | `KeyDown(key Key) bool`: the key is held down. |
| `Input.KeyPressed` | `KeyPressed(key Key) bool`: the key went down since the previous update; true in one update per press. |
| `Input.TypedText` | `TypedText() string`: the characters typed since the previous update, as the player's keyboard makes them (its layout, Shift, AltGr and accents applied, so `#`, `ñ` and `é` come right on any keyboard), repeated while a key is held, as in any text box. For names, chat and code typed in a game. Enter, Backspace, the arrows and the other keys that type nothing aren't in it: read them with `KeyPressed`. |
| `Key` | A keyboard key: one of the constants below. Letters and digits are named after their place on a US keyboard. |

| Keys | Constants |
| --- | --- |
| Common | `KeySpace` `KeyEnter` `KeyEscape` `KeyTab` `KeyBackspace` |
| Arrows | `KeyLeft` `KeyRight` `KeyUp` `KeyDown` |
| Editing | `KeyDelete` `KeyHome` `KeyEnd` `KeyPageUp` `KeyPageDown` |
| Modifiers | `KeyLeftShift` `KeyRightShift` `KeyLeftControl` `KeyRightControl` `KeyLeftAlt` `KeyRightAlt` |
| Letters | `KeyA` `KeyB` `KeyC` `KeyD` `KeyE` `KeyF` `KeyG` `KeyH` `KeyI` `KeyJ` `KeyK` `KeyL` `KeyM` `KeyN` `KeyO` `KeyP` `KeyQ` `KeyR` `KeyS` `KeyT` `KeyU` `KeyV` `KeyW` `KeyX` `KeyY` `KeyZ` |
| Digits (spelled out: there is no `Key0`) | `KeyZero` `KeyOne` `KeyTwo` `KeyThree` `KeyFour` `KeyFive` `KeySix` `KeySeven` `KeyEight` `KeyNine` |
| Punctuation (where a US keyboard has it) | `KeyApostrophe` `KeyComma` `KeyMinus` `KeyPeriod` `KeySlash` `KeySemicolon` `KeyEqual` `KeyLeftBracket` `KeyBackslash` `KeyRightBracket` `KeyGrave` |
| Function keys | `KeyF1` `KeyF2` `KeyF3` `KeyF4` `KeyF5` `KeyF6` `KeyF7` `KeyF8` `KeyF9` `KeyF10` `KeyF11` `KeyF12` |

Only these keys are read. There is no numeric keypad, and a raylib key code converted to `Key` is never down. The punctuation keys are places on the keyboard, for controls laid out by hand, such as a piano on the rows Z to M and A to J: on another layout `KeyComma` is the key where a US keyboard has its comma, whatever it types there. Read what a key types with `TypedText`.

### Mouse

| Name | What it does |
| --- | --- |
| `Input.MousePosition` | `MousePosition() (x, y float32)`: the pointer, in screen pixels. Over the black bars or outside the window it can be outside the screen. |
| `Input.MouseMoved` | `MouseMoved() bool`: the pointer moved since the previous update, even in another scene; false in the first update. Let the pointer pick a menu item only when it moves or clicks, so a pointer resting over a button doesn't fight the keys or pick one as a menu opens. |
| `Input.MouseDown` | `MouseDown(button MouseButton) bool`: the button is held down. |
| `Input.MousePressed` | `MousePressed(button MouseButton) bool`: the button went down since the previous update; true in one update per click. |
| `Input.MouseWheel` | `MouseWheel() float32`: notches the wheel turned since the previous update. Positive is up, away from the player; 0 means it didn't move. |
| `SetMouseVisible` | `SetMouseVisible(visible bool)`: shows or hides the pointer over the window, from the next frame, such as to draw a crosshair in its place: the system's pointer or the game's sprite. `MousePosition` still works. |
| `SetMouseSprite` | `SetMouseSprite(sprite *Sprite, frame int, x, y float32)`: a frame of a sprite in place of the system's pointer, from the next frame, with its pixel x, y, from the frame's top-left corner, where the pointer points: 0, 0 for an arrow whose tip is that corner. `nil` brings the system's pointer back; call it again with another frame for another pointer, such as a hand over a button. Call it once, in `main` or in `Update`, not in `Draw`. |
| `IsMouseVisible` | `IsMouseVisible() bool`: whether the pointer shows, or will from the next frame. |
| `MouseButton` | `MouseLeft`, `MouseRight` or `MouseMiddle`. |

- **A pointer of the game's own is a sprite, not a drawing in `Draw`.** `SetMouseSprite` draws it over everything, post-processing included, where the system's pointer would be, at the window's resolution, so it moves as smoothly as the system's, and as many whole times larger as the screen is, so a pixel art pointer stays sharp. It hides by itself while the pointer is outside the window and while the player plays with a gamepad or with fingers, and comes back when the mouse moves. `golib shot` draws it once `--input` has moved the mouse.

```go
var pointer = golib.NewSprite("sprites/pointer.png")

func main() {
	golib.SetMouseSprite(pointer, 0, 0, 0) // its tip is the top-left pixel
	// ...
}
```

```go
x, y := input.MousePosition()
if s.playButton.Contains(x, y) && (input.MouseMoved() || input.MousePressed(golib.MouseLeft)) {
	s.selected = playButton // the pointer picks the button it moves over
}
if input.MousePressed(golib.MouseLeft) && s.playButton.Contains(x, y) {
	golib.SwitchScene(newPlayScene())
	return
}
```

### Touch screen

A game played in a browser on a phone or a tablet is played with fingers. The oldest finger on the screen moves the mouse pointer and holds `MouseLeft` down, so a menu or a button written for the mouse is worked by tapping it, with no touch code at all. Read the fingers themselves for controls a player holds, such as a steering pad or a thrust button, and for two of them at once.

**One build plays both ways.** The same web build is opened on a phone and on a computer, so don't ask what the machine can do: ask `PlayingWithTouch` how the player is playing, and draw on-screen controls only then. It starts true on a phone or a tablet and false everywhere else, a computer with a touch screen included, and from then on it follows the player: a finger turns it on, and the keyboard, the mouse or a gamepad turns it off.

| Name | What it does |
| --- | --- |
| `PlayingWithTouch` | `PlayingWithTouch() bool`: the player is playing with fingers, which is what decides whether to draw on-screen controls. True from the start on a phone or a tablet in a browser; false on a computer, even one with a touch screen, until a finger lands on the game; false again at the next key, mouse move or gamepad button. Between the two it holds its answer, so the controls don't blink between taps. Always false on the desktop, where GoLib reads no touch screen: a touch screen on Windows moves the mouse pointer instead, so the game is played with the mouse there. True under `golib shot` when the `--input` script has `Touch` in it. |
| `Input.Touches` | `Touches() []Touch`: the fingers on the screen now, oldest first. Empty where there is no touch screen, so it can be read in every update. The slice belongs to that update: read it, don't keep it. |
| `Input.TouchDownIn` | `TouchDownIn(area Rectangle) bool`: a finger is inside area, which is what an on-screen button held down asks. |
| `Input.TouchPressedIn` | `TouchPressedIn(area Rectangle) bool`: a finger landed inside area since the previous update; true in one update per landing, which is what a tapped button asks. |
| `Touch` | One finger on the screen. |
| `Touch.ID` | `ID int`: the same number while that finger stays down, so a game can follow one finger. A finger that lifts and lands again is a new one. |
| `Touch.Position` | `Position Vector2`: where the finger is, in screen pixels, as `Input.MousePosition` is. |
| `Touch.Pressed` | `Pressed bool`: the finger landed since the previous update. A finger that lands and lifts between two updates is reported to one update, so a quick tap is never lost. |

```go
// The controls, in screen pixels, laid out in the corners the thumbs reach.
var (
	turnLeft  = golib.Rectangle{X: 40, Y: 520, Width: 140, Height: 160}
	turnRight = golib.Rectangle{X: 200, Y: 520, Width: 140, Height: 160}
	fire      = golib.Rectangle{X: 1100, Y: 520, Width: 140, Height: 160}
)

func (s *playScene) Update(input *golib.Input, dt float32) {
	if input.KeyDown(golib.KeyLeft) || input.TouchDownIn(turnLeft) {
		s.ship.turn(-1, dt)
	}
	if input.KeyDown(golib.KeyRight) || input.TouchDownIn(turnRight) {
		s.ship.turn(1, dt)
	}
	if input.KeyPressed(golib.KeySpace) || input.TouchPressedIn(fire) {
		s.ship.fire()
	}
}

func (s *playScene) Draw(screen *golib.Screen) {
	s.drawWorld(screen)
	if golib.PlayingWithTouch() {
		for _, pad := range []golib.Rectangle{turnLeft, turnRight, fire} {
			screen.DrawRectangle(pad, golib.WithOpacity(golib.White, 0.15))
		}
	}
}
```

A finger anywhere else, for a game where the whole screen is the control, comes from `Input.Touches`:

```go
for _, touch := range input.Touches() {
	if touch.Pressed {
		s.ship.moveTo(touch.Position) // tap to send the ship there
	}
}
```

`golib shot --input "Touch@40-90:200,600"` puts a finger on the screen from update 40 to update 90, so a shot can check the controls; each `Touch` item is a finger of its own, so two that overlap are two fingers at once.

### Gamepads

Gamepads are numbered 0 to 3, in the order they were connected; a one-player game reads gamepad 0. Every call is safe on a gamepad that isn't connected: it reads as nothing pressed and sticks at rest.

| Name | What it does |
| --- | --- |
| `Input.GamepadConnected` | `GamepadConnected(pad int) bool` |
| `Input.GamepadName` | `GamepadName(pad int) string`: the name the system gives it, such as `"Xbox Controller"`, or `""` |
| `Input.GamepadType` | `GamepadType(pad int) GamepadType`: the kind of gamepad, guessed from its name, which says what is printed on its buttons, for showing the player which one to press. An unknown gamepad, or none, is `GamepadTypeXbox`. Names vary between systems and makers, so a game that shows button pictures lets the player choose them in its settings too |
| `GamepadType` | `GamepadTypeXbox`, `GamepadTypePlayStation` or `GamepadTypeNintendo`: the labels in the table below |
| `PlayingWithGamepad` | `PlayingWithGamepad() bool`: the player is playing with a gamepad, which is what decides whether a game's prompts show gamepad buttons or keys. False until a gamepad button is pressed or a stick tilted; false again at the next key, mouse move or finger. Between the two it holds its answer, so the prompts don't change while the player does nothing. True under `golib shot` when the `--input` script has gamepad items in it |
| `Input.GamepadDown` | `GamepadDown(pad int, button GamepadButton) bool`: the button is held down. |
| `Input.GamepadPressed` | `GamepadPressed(pad int, button GamepadButton) bool`: the button went down since the previous update; true in one update per press. |
| `Input.GamepadLeftStick` | `GamepadLeftStick(pad int) (x, y float32)`: each from -1 to 1, with y growing downwards like the screen. 0, 0 at rest: a small dead zone around the center is removed. |
| `Input.GamepadRightStick` | `GamepadRightStick(pad int) (x, y float32)`: the same for the right stick. |
| `GamepadButton` | A gamepad button: one of the constants below. |

Buttons are read by where they are, not by what is printed on them: `GamepadA` is always the bottom face button, whatever it says. So a Nintendo gamepad's B, at the bottom, is `GamepadA`, and its A, on the right, is `GamepadB`. To show a button as the player sees it, ask `Input.GamepadType` and use its column:

| Constant | Xbox | PlayStation | Nintendo | Usual use |
| --- | --- | --- | --- | --- |
| `GamepadA` | A, bottom | Cross | B | Jump, confirm |
| `GamepadB` | B, right | Circle | A | Back, cancel |
| `GamepadX` | X, left | Square | Y | Second action |
| `GamepadY` | Y, top | Triangle | X | Third action |
| `GamepadUp` `GamepadDown` `GamepadLeft` `GamepadRight` | D-pad | D-pad | D-pad | Move, menus |
| `GamepadLeftBumper` `GamepadRightBumper` | LB, RB | L1, R1 | L, R | |
| `GamepadLeftTrigger` `GamepadRightTrigger` | LT, RT, read as buttons | L2, R2 | ZL, ZR | |
| `GamepadStart` | Menu | Options | + | Pause |
| `GamepadBack` | View | Share, Select | − | |
| `GamepadLeftStickButton` `GamepadRightStickButton` | Pressing a stick in | L3, R3 | Pressing a stick in | |

```go
// The prompt for building: the key, or the bottom button as it is labeled.
func buildPrompt(input *golib.Input) string {
	if !golib.PlayingWithGamepad() {
		return "Space: build"
	}
	switch input.GamepadType(0) {
	case golib.GamepadTypePlayStation:
		return "Cross: build"
	case golib.GamepadTypeNintendo:
		return "B: build"
	}
	return "A: build"
}
```

## Drawing

`Screen` is what `Draw` draws on. Everything later covers what was drawn before, so draw from back to front, and start with `Clear`.

| Name | What it does |
| --- | --- |
| `Screen.Clear` | `Clear(color Color)`: fills the whole screen. |
| `Screen.DrawRectangle` | `DrawRectangle(rect Rectangle, color Color)`: a filled rectangle. |
| `Screen.DrawRectangleOutline` | `DrawRectangleOutline(rect Rectangle, thickness float32, color Color)`: the rectangle's edges, thickness pixels wide, inside it. |
| `Screen.DrawCircle` | `DrawCircle(x, y, radius float32, color Color)`: a filled circle centered at x, y. |
| `Screen.DrawCircleOutline` | `DrawCircleOutline(x, y, radius, thickness float32, color Color)`: a ring, the circle's edge, thickness pixels wide, inside it. |
| `Screen.DrawLine` | `DrawLine(x1, y1, x2, y2, thickness float32, color Color)`: a straight line, thickness pixels wide. |
| `Screen.DrawTriangle` | `DrawTriangle(x1, y1, x2, y2, x3, y3 float32, color Color)`: a filled triangle; the corners can come in any order. |
| `Screen.DrawPolygon` | `DrawPolygon(points []Vector2, color Color)`: a filled shape with these corners, in order, either way round. Every corner must be in sight from the shape's middle, as in ships, rocks and stars. |
| `Screen.DrawPolygonOutline` | `DrawPolygonOutline(points []Vector2, thickness float32, color Color)`: the shape's sides, back to the first corner, with round corners. |
| `Screen.DrawText` | `DrawText(text string, x, y, size float32, color Color, options ...TextOptions)`: text with its top at y, size pixels high, in the built-in font or the font the options give (see [Fonts](#fonts)). x is where each line starts, or its middle or end with `TextOptions.Align`. A `\n` starts a new line. |
| `Screen.TextWidth` | `TextWidth(text string, size float32, options ...TextOptions) float32`: how wide `DrawText` draws text with the same options, to fit it in a box or put something after it. |
| `Screen.SetBlendMode` | `SetBlendMode(mode BlendMode)`: how the drawing that follows mixes with what is under it. Every `Draw` starts with `BlendNormal`. |
| `BlendMode` | `BlendNormal` covers what is underneath, by the color's opacity; `BlendAdd` adds to it, so overlapping glows grow brighter and nothing darkens: explosions, flames, sparks, lasers. |
| `Screen.Width`, `Screen.Height` | `Width() float32`, `Height() float32`: the screen's current size, from `Config` or its `WindowScale` or `FillWindow`. |

- Positions and sizes are `float32`. Untyped constants convert by themselves; other numbers need `float32(n)`.
- For a shape that turns, keep its corners around 0, 0, and turn and move them before drawing: `corner.Rotate(angle).Add(position)` (see [Vectors, rectangles and collisions](#vectors-rectangles-and-collisions)). Pictures are sprites: see [Sprites and animations](#sprites-and-animations).
- To scroll a world larger than the screen, draw it through a `Camera`: see [Camera](#camera).
- The built-in font is raylib's pixel font, which is 10 pixels high: sizes that are multiples of 10 keep its pixels even. Other fonts come from files: see [Fonts](#fonts).
- Text is drawn at whole pixels: x and y are rounded, so letters stay sharp.
- Shapes have stepped edges: each pixel is in a shape or out of it. Set `Config.Antialias` for soft edges, as a game drawn with shapes, cards and icons wants.

```go
// A panel with a border, a centered heading, and a score on the right.
panel := golib.Rectangle{X: 340, Y: 200, Width: 600, Height: 320}
screen.DrawRectangle(panel, golib.WithOpacity(golib.Black, 0.8))
screen.DrawRectangleOutline(panel, 4, golib.Gold)
center := golib.TextOptions{Align: golib.AlignCenter}
screen.DrawText("Level complete", panel.Center().X, panel.Y+30, 40, golib.White, center)
screen.DrawText(fmt.Sprint(s.score), screen.Width()-20, 20, 30, golib.White, golib.TextOptions{Align: golib.AlignRight})

// The ship: a polygon turned to where it faces.
corners := make([]golib.Vector2, len(shipShape))
for i, corner := range shipShape { // shipShape points right, around 0, 0
	corners[i] = corner.Rotate(s.ship.angle).Add(s.ship.position)
}
screen.DrawPolygon(corners, shipColor)
screen.DrawPolygonOutline(corners, 2, golib.White)
```

### Colors

`Color` is a color with red, green, blue and opacity from 0 to 255: `golib.Color{R: 20, G: 24, B: 32, A: 255}`. `A` is 255 for solid colors; lower values let what is underneath show through, so `golib.Color{A: 150}` over the whole screen darkens it under a pause message. `Color` is raylib's `rl.Color`, so it can be passed to raylib directly.

The named colors are raylib's palette: `LightGray` `Gray` `DarkGray` `Yellow` `Gold` `Orange` `Pink` `Red` `Maroon` `Green` `Lime` `DarkGreen` `SkyBlue` `Blue` `DarkBlue` `Purple` `Violet` `DarkPurple` `Beige` `Brown` `DarkBrown` `White` `Black` `Magenta`, `RayWhite` (the off-white raylib uses for backgrounds) and `Blank` (fully transparent).

`WithOpacity`, `WithOpacity(color Color, opacity float32) Color`, returns the color with its opacity set, from 0, invisible, to 1, solid: `golib.WithOpacity(golib.White, flashLeft/flashTime)` fades a flash out. It sets the opacity instead of scaling the color's own, so fading a color that is already see-through means multiplying: `golib.WithOpacity(c, float32(c.A)/255*fade)`.

Keep a game's colors together as named variables, as `games/platformer/main.go` does, so its look changes in one place.

## Fonts

A font is a `.ttf` or `.otf` file in the assets folder. Pass it to `DrawText` and `TextWidth` in a `TextOptions`.

| Name | What it does |
| --- | --- |
| `Font` | A TrueType or OpenType font. |
| `NewFont` | `NewFont(name string) *Font`: the `.ttf` or `.otf` file `name` in the assets folder, with forward slashes, as in `ReadAsset`. |
| `TextOptions` | Changes how `DrawText` draws and `TextWidth` measures: `golib.TextOptions{Font: title}`. Pass at most one. |
| `TextOptions.Font` | The font. Default: the built-in pixel font. |
| `TextOptions.Align` | Which part of each line goes at x: `AlignLeft`, the default, `AlignCenter` or `AlignRight`. |
| `TextOptions.FullResolution` | Draws the text at the window's own resolution, its letters that many times larger, instead of in the screen's pixels: for a font from a file meant to be read smoothly, such as a book's serif, in a game whose small screen the window enlarges, as one with `Config.PixelArt`. x, y and size stay in screen pixels, and `TextWidth` with the same options measures in them. It goes over everything else the frame draws, after the post-processing shaders, which don't reach it, in the order it and frames drawn with `DrawOptions.FullResolution` are drawn. |
| `TextAlign` | The type of `AlignLeft`, `AlignCenter` and `AlignRight`. |

```go
var (
	titleFont = golib.NewFont("fonts/title.ttf") // games/<game>/assets/fonts/title.ttf
	title     = golib.TextOptions{Font: titleFont}
)

func (s *titleScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.Black)
	name := "Cave Runner"
	x := (screen.Width() - screen.TextWidth(name, 64, title)) / 2
	screen.DrawText(name, x, 120, 64, golib.Gold, title)
	screen.DrawText("Press Enter", 20, 300, 20, golib.White) // the built-in font
}
```

- **Draw text at a few sizes.** GoLib draws a font's letters at each size the first time the game uses it, so text stays sharp, and keeps the last 24 sizes of each font. Text whose size changes every frame, such as a title that grows, makes that work every frame: grow it in a few steps instead.
- Any letter the font has can be drawn, in any language: letters beyond English, such as ñ, é or ü, are added the first time they are drawn. A letter the font doesn't have shows as a question mark.
- The file is read the first time text is drawn with the font, so a missing or broken file stops `Run` under `golib shot` too. Measuring and drawing need the window, so they work only in `Draw`.
- Font collections (`.ttc`), web fonts (`.woff`) and bitmap fonts (`.fnt`) aren't read. Pixel fonts in `.ttf` look best at the sizes they were made for, or whole multiples of them.
- **A pixel-art game with smooth text.** In the screen's pixels, a font made for print, at 10 or 12 pixels, comes out blurred and cramped. Draw it with `TextOptions{Font: book, FullResolution: true}` and it is drawn at the window's resolution instead, as sharp as the window allows, over the pixel art, as many games do with their dialogue. The night, the CRT look and other post-processing don't touch it, so dialogue drawn this way stays readable over a dark scene.
- Only use fonts the user provides, whose license allows shipping them with a game, such as the SIL Open Font License, and write where each came from, and its license, in `assets/ATTRIBUTION.md`. The fonts that come with the computer usually can't be shipped.

## Sprites and animations

A sprite is a picture from the game's assets folder, made of frames of one size: a PNG image, a PNG sprite sheet on a grid, or an Aseprite file.

| Name | What it does |
| --- | --- |
| `Sprite` | A picture in frames, numbered from 0. |
| `NewSprite` | `NewSprite(name string) *Sprite`: a `.png` file as one frame, or an `.aseprite` or `.ase` file with Aseprite's frames and animations. `name` is relative to the assets folder, as in `ReadAsset`. |
| `NewSpriteSheet` | `NewSpriteSheet(name string, frameWidth, frameHeight int) *Sprite`: a `.png` file cut into frames of that size, numbered left to right, then top to bottom. The image must be a whole number of frames wide and high. |
| `Screen.DrawSprite` | `DrawSprite(sprite *Sprite, frame int, x, y float32, options ...DrawOptions)`: draws a frame with its top-left corner at x, y, at its own size. Pass one `DrawOptions` to change that. |
| `Sprite.Width`, `Sprite.Height` | `Width() float32`, `Height() float32`: one frame's size, in pixels. |
| `Sprite.Frames` | `Frames() int`: how many frames it has. |
| `Sprite.Animation` | `Animation(name string) Animation`: the animation of the Aseprite tag named `name`. Every sprite also has the animation named `""`: all its frames in order, as long as the Aseprite file says, or 0.1 seconds each. |

| `DrawOptions` field | Default | Meaning |
| --- | --- | --- |
| `DrawOptions.Scale` | 1 | Size, as a multiple of the frame's own. |
| `DrawOptions.FlipX` | `false` | Mirror left to right: a character facing the other way. |
| `DrawOptions.FlipY` | `false` | Mirror top to bottom. |
| `DrawOptions.Rotation` | 0 | Degrees, clockwise, around the origin. |
| `DrawOptions.OriginX`, `DrawOptions.OriginY` | 0, 0 | The point of the frame, in its own pixels from its top-left corner, that goes at x, y and that `Rotation` turns around. The middle of a 32 by 32 frame is 16, 16. |
| `DrawOptions.Tint` | `White` | Multiplies the frame's colors; its `A` makes the frame see-through: `golib.Color{R: 255, G: 255, B: 255, A: 128}` is half. |
| `DrawOptions.Smooth` | `false` | Filter a sprite with bilinear sampling and mipmaps, with positions between pixels. For painted backgrounds and rendered icons. Keeps the current draw order, camera, blending and post-processing; pixel-art draws of the same sprite keep their sharp sampling. Applies to `DrawSprite`; `DrawImage` keeps its pixel sampling. |
| `DrawOptions.FullResolution` | `false` | Draw the frame at the window's own resolution, smoothed, instead of in the screen's pixels: for a picture with more detail than the screen has room for, such as a studio's logo or art painted at high resolution, in a game whose small screen the window enlarges. See below. |

An `Animation` is a list of frames, each shown for a while. It keeps no time of its own: keep the seconds it has played in the game's state, add `dt` in `Update`, and ask which frame to draw.

| Name | What it does |
| --- | --- |
| `Animation` | `golib.Animation{Frames: []int{8, 9, 10, 11}, FrameTime: 0.1}` |
| `Animation.Frames` | Frame numbers, in the order they show. |
| `Animation.FrameTime` | Seconds each frame shows. Default: 0.1. |
| `Animation.Durations` | Seconds for each frame, one value per frame, when they differ; `FrameTime` is then ignored. Aseprite animations set them. |
| `Animation.Once` | Play once and stay on the last frame, instead of looping. |
| `Animation.Frame` | `Frame(time float32) int`: the frame to show after the animation has played for `time` seconds. |
| `Animation.Duration` | `Duration() float32`: seconds to play once. |
| `Animation.Finished` | `Finished(time float32) bool`: all frames have shown once, such as an attack that should end. |

```go
var (
	hero = golib.NewSpriteSheet("sprites/hero.png", 32, 32) // games/<game>/assets/sprites/hero.png
	idle = golib.Animation{Frames: []int{0, 1}, FrameTime: 0.4}
	run  = golib.Animation{Frames: []int{8, 9, 10, 11}, FrameTime: 0.1}
)

type player struct {
	x, y       float32
	moving     bool
	facingLeft bool
	animTime   float32 // seconds the current animation has played
}

func (p *player) setMoving(moving bool) {
	if moving != p.moving {
		p.moving, p.animTime = moving, 0 // a new animation starts from its first frame
	}
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	// ... move s.player, then:
	s.player.animTime += dt
}

func (s *playScene) Draw(screen *golib.Screen) {
	p := s.player
	animation := idle
	if p.moving {
		animation = run
	}
	screen.DrawSprite(hero, animation.Frame(p.animTime), p.x, p.y, golib.DrawOptions{FlipX: p.facingLeft})
}
```

With an Aseprite file, the animations come from its tags:

```go
var (
	slime = golib.NewSprite("sprites/slime.aseprite")
	hop   = slime.Animation("hop") // the tag named hop
)
```

- **Aseprite counts frames from 1 in its timeline; GoLib counts from 0.** Frame 1 in Aseprite is frame 0 here. Tags take care of that for you.
- Aseprite files are read as Aseprite shows them: visible layers only, with their opacity and blend mode, groups, linked cels and z-indexes. Hidden and reference layers aren't drawn. A tag plays forward, in reverse or in ping-pong, as many times as its Repeat field says (an empty field loops forever, and `Once` is then false); a repeated tag name uses the first tag. Slices and user data are ignored. A visible tilemap layer stops `Run` with an error.
- Keep the `.aseprite` file in the assets folder: the game reads it as it is, so there is no export step to forget.
- For a PNG sheet with no animation data, write the animations in code, as above. Only frames on a grid can be drawn; there is no way yet to draw a part of an image of another size.
- Pixels stay sharp: sprites are drawn without smoothing, at whole pixels, rounding x and y, as maps and text are. For pixel art, also set `Config.PixelArt` with a small screen, and round the camera's position too, so everything moves together.
- For painted art already drawn on a large screen, use `DrawOptions.Smooth`. This filters the sprite inside the scene; later panels, fades and post-processing still cover it normally.
- **A large picture drawn small loses its detail on a small screen; `FullResolution` keeps it.** Everything is drawn into the screen, `Config.Width` by `Config.Height` pixels, which the window then enlarges, so a 1500-pixel logo drawn at `Scale: 0.2` on a 640 by 360 screen keeps one pixel in five and is then enlarged blocky. With `DrawOptions.FullResolution` the frame is drawn after the screen is in the window, smoothed, at the window's pixels: x, y and `Scale` are still in screen pixels and need not be whole, and a camera still moves it. It goes over everything else the frame draws, in the order such frames are drawn, after the post-processing shaders, which don't reach it, and it always blends normally. `golib shot` pictures are the screen's size, so they show it at that size.
- `Width`, `Height`, `Frames` and `Animation` read the file, so they work in tests and before `Run`.
- Only use art the user provides, and write where it came from, and its license, in `assets/ATTRIBUTION.md`, as `games/platformer` does.

### Pictures made pixel by pixel

A sprite is a picture from a file. An `Image` is one the game makes itself, pixel by pixel, as it runs: the screen of an emulator or a fantasy console, a plasma effect, a minimap, a picture a player paints.

| Name | What it does |
| --- | --- |
| `NewImage` | `NewImage(width, height int) *Image`: a picture of that size, 1 to 4096 pixels each way, every pixel transparent (`Color{}`) until the game sets it. |
| `Image` | A picture made in code. |
| `Image.SetPixel` | `SetPixel(x, y int, color Color)`: sets a pixel, counted from 0, 0 at the top-left corner. Outside the picture it does nothing, so drawing can run off its edges. |
| `Image.Pixel` | `Pixel(x, y int) Color`: a pixel's color, or `Color{}` outside the picture. |
| `Image.Clear` | `Clear(color Color)`: sets every pixel. |
| `Image.Width`, `Image.Height` | `Width() int`, `Height() int`: its size, in pixels. |
| `Screen.DrawImage` | `DrawImage(img *Image, x, y float32, options ...DrawOptions)`: draws it with its top-left corner at x, y, one screen pixel for each of its pixels. `DrawOptions` scales, flips, rotates and tints it, as with `DrawSprite`. |

```go
var monitor = golib.NewImage(64, 64) // a package variable, made once

func (g *game) Update(input *golib.Input, dt float32) {
	monitor.Clear(golib.Black)
	monitor.SetPixel(g.x, g.y, golib.Green)
}

func (g *game) Draw(screen *golib.Screen) {
	screen.DrawImage(monitor, 32, 32, golib.DrawOptions{Scale: 4}) // 256 by 256 on the screen
}
```

- Changing pixels is plain Go, and cheap: the picture goes to the graphics card once, at the next `DrawImage` after it changed, however many pixels changed. Setting a pixel to the color it has isn't a change.
- It is drawn without smoothing, at whole pixels, so its pixels stay square and sharp at any scale, as a sprite's do.
- A size below 1 or above 4096 stops `Run` with an error the first time the picture is drawn.
- Make pictures once, as package variables or in the game's state, as sprites are: each stays on the graphics card until `Run` ends.

## Maps

A map is a level made in [Tiled](https://www.mapeditor.org): a `.tmx` file in the assets folder, with the tilesets (`.tsx`), templates (`.tx`) and PNG images it uses. The game reads the files as Tiled saves them, so there is no export step.

| Name | What it does |
| --- | --- |
| `Map` | A Tiled map: its layers, its tilesets and its objects. |
| `NewMap` | `NewMap(name string) *Map`: the `.tmx` file `name`, relative to the assets folder, as in `ReadAsset`. |
| `Screen.DrawMap` | `DrawMap(level *Map, x, y float32)`: draws every visible layer with the map's top-left corner at x, y, over the map's background color if Tiled gives it one. Through a camera, draw it at 0, 0, where it is in the world (see [Camera](#camera)). |
| `Screen.DrawMapLayer` | `DrawMapLayer(level *Map, layer string, x, y float32)`: draws one layer, even one hidden in Tiled. Draw a map layer by layer to put sprites between its layers. |
| `Map.Width`, `Map.Height` | `Width() float32`, `Height() float32`: the map's size, in pixels. |
| `Map.TileWidth`, `Map.TileHeight` | `TileWidth() float32`, `TileHeight() float32`: the size of the map's grid cells, in pixels. |
| `Map.Tile` | `Tile(layer string, column, row int) Tile`: the tile in a cell of a tile layer. Columns and rows count from 0 at the top-left corner; outside the layer, cells are empty. |
| `Map.TileAt` | `TileAt(layer string, x, y float32) Tile`: the tile of a tile layer under a point, in the map's pixels. |
| `Map.TilesIn` | `TilesIn(layer string, area Rectangle) []MapTile`: the tiles of a tile layer whose cells overlap `area`, row by row, leaving out empty cells. Use it for collisions. |
| `Map.Objects` | `Objects(layer string) []MapObject`: the objects of an object layer, in Tiled's order, or of every object layer for `""`. |
| `Map.Object` | `Object(name string) (MapObject, bool)`: the first object with that name, such as the player's start, and whether there is one. |
| `Map.Err` | `Err() error`: what stopped the map from loading, or nil. Ask it before anything else, since the other methods give zero values for a map that didn't load. Its message already names the file, so pass it on as it is. Drawing or asking such a map stops `Run` with the same mistake, so use `Err` only to handle it yourself: to skip a level, or to name the wrong file in a test. |
| `Map.Properties` | `Properties() Properties`: the map's custom properties. |
| `Map.LayerProperties` | `LayerProperties(layer string) Properties`: a layer's custom properties, with those of the groups around it. |

| Name | What it is |
| --- | --- |
| `Tile` | A tile: which one it is, and what its tileset says about it. |
| `Tile.Tileset` | The name of its tileset; `""` for an empty cell. |
| `Tile.ID` | Its ID in the tileset, as Tiled shows it. |
| `Tile.Class` | Its class in the tileset. |
| `Tile.Properties` | Its custom properties in the tileset. |
| `Tile.Empty` | `Empty() bool`: the cell has no tile. |
| `MapTile` | A `Tile` in a tile layer, with its cell: `MapTile.Column`, `MapTile.Row` and, embedded, the cell's `Rectangle` in the map's pixels, so `tile.X` and `tile.Overlaps` work. |
| `MapObject` | An object of an object layer. Its `Rectangle` is embedded: X, Y is its top-left corner and Width, Height its size, in the map's pixels, before rotation. |
| `MapObject.ID` | Tiled's ID for it, unique in the map. |
| `MapObject.Name` | Its name. |
| `MapObject.Class` | Its class (type in older files). A tile object without one has its tile's. |
| `MapObject.Layer` | The name of its object layer. |
| `MapObject.Shape` | `"rectangle"`, `"ellipse"`, `"point"`, `"polygon"`, `"polyline"`, `"text"` or `"tile"`. |
| `MapObject.Rotation` | Degrees, clockwise, around the point Tiled shows as its position. |
| `MapObject.Points` | A polygon's or a polyline's corners, from the object's X, Y. |
| `MapObject.Text` | A text object's text. |
| `MapObject.Tile` | A tile object's tile. |
| `MapObject.Visible` | Whether it is visible in Tiled. Hidden tile objects aren't drawn. |
| `MapObject.Properties` | Its custom properties. A tile object also has its tile's, and its own win. |
| `Vector2` | A point in pixels: see [Vectors, rectangles and collisions](#vectors-rectangles-and-collisions). |
| `Properties` | Custom properties by name, as the text Tiled saves (a `map[string]string`). Its methods read one as a value, and give the zero value when it is missing or isn't one. |
| `Properties.String` | `String(name string) string` |
| `Properties.Int` | `Int(name string) int` |
| `Properties.Float` | `Float(name string) float32` |
| `Properties.Bool` | `Bool(name string) bool` |
| `Properties.Color` | `Color(name string) Color` |

```go
var level = golib.NewMap("maps/level1.tmx") // games/<game>/assets/maps/level1.tmx

func newPlayScene() *playScene {
	s := &playScene{}
	if start, found := level.Object("start"); found { // a point object named start
		s.player.x, s.player.y = start.X, start.Y
	}
	for _, coin := range level.Objects("coins") { // an object layer
		s.coins = append(s.coins, coin.Rectangle)
	}
	return s
}

// solidTiles returns the solid tiles of the ground layer that box overlaps:
// in Tiled, the tileset gives those tiles a bool property named solid.
func solidTiles(box golib.Rectangle) []golib.MapTile {
	var solid []golib.MapTile
	for _, tile := range level.TilesIn("ground", box) {
		if tile.Properties.Bool("solid") {
			solid = append(solid, tile)
		}
	}
	return solid
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.Black)
	screen.SetCamera(s.camera) // see Camera, below
	screen.DrawMapLayer(level, "background", 0, 0)
	screen.DrawMapLayer(level, "ground", 0, 0)
	screen.DrawSprite(hero, 0, s.player.x, s.player.y)
	screen.DrawMapLayer(level, "leaves", 0, 0) // in front of the player
}
```

- **Mark tiles in Tiled, not by ID in code.** Give tiles a class or a property in the tileset, such as a bool named `solid`, and read it from `Tile.Class` or `Tile.Properties`: the rules then survive a redrawn tileset.
- **Objects are where the game's things go:** starts, enemies, doors, areas. `DrawMap` draws tile layers, image layers and tile objects, with Tiled's visibility, opacity, tint colors, offsets, parallax factors, flips and animated tiles, which follow the game's time. Other objects aren't drawn: the game decides what they mean.
- An object's X, Y is its top-left corner, even for tile objects, which Tiled places by another corner. A point has no size.
- Layers are named as Tiled names them. A layer in a group can also be named with its groups, `"level 1/ground"`; the plain name finds the first layer with that name.
- The members of a class property are named `"property.member"`, such as `object.Properties.Bool("door.locked")`, for the members Tiled saved.
- GoLib reads orthogonal maps saved as TMX, with the Tile Layer Format set to CSV or Base64 (uncompressed, zlib or gzip compressed), and PNG images. Zstandard compression, isometric and hexagonal maps, and Tiled's JSON files stop `Run` with an error that says what to change in Tiled. An infinite map is read as a fixed one that covers its tiles, moved so its top-left tile is at 0, 0.
- Keep maps, tilesets, templates and images inside the assets folder: Tiled saves paths relative to each file, and a path out of the folder stops `Run`.
- `DrawMap` draws only the tiles on the screen, or in the camera's view, so large maps are fine, and draws at whole pixels, rounding x and y, so tiles leave no gaps.
- `Width`, `Height`, `Tile`, `TileAt`, `TilesIn`, `Objects` and the properties read the file, so they work in tests and before `Run`.

## Vectors, rectangles and collisions

`Vector2` is a point or a direction, in pixels. Its methods return a new vector, so they chain, and leave the one they are called on as it is. Angles are in degrees, clockwise from pointing right, as for sprites: 90 points down the screen.

| Name | What it does |
| --- | --- |
| `Vector2` | `golib.Vector2{X: 3, Y: 4}`, with `Vector2.X` and `Vector2.Y`. Maps use it for points too. |
| `Vector2.Add`, `Vector2.Sub` | `Add(other Vector2) Vector2`, `Sub(other Vector2) Vector2`. `target.Sub(from)` is the way from `from` to `target`. |
| `Vector2.Scale` | `Scale(factor float32) Vector2`: both parts multiplied, such as a velocity times `dt`. |
| `Vector2.Length`, `Vector2.Distance` | `Length() float32`; `Distance(other Vector2) float32`, between two points. |
| `Vector2.Normalize` | `Normalize() Vector2`: length 1, the same way; the zero vector stays zero. |
| `Vector2.ClampLength` | `ClampLength(max float32) Vector2`: shortened to `max` if longer. Use it so that moving diagonally isn't faster than moving straight. |
| `Vector2.Dot` | `Dot(other Vector2) float32`: positive when the two point the same way, 0 at right angles. |
| `Vector2.Angle` | `Angle() float32`: where it points, from -180 to 180 degrees; 0 is right, 90 is down. |
| `Vector2.Rotate` | `Rotate(degrees float32) Vector2`: turned clockwise. |
| `Vector2FromAngle` | `Vector2FromAngle(degrees float32) Vector2`: length 1, pointing at `degrees`. |
| `Vector2.MoveTowards` | `MoveTowards(target Vector2, maxDistance float32) Vector2`: a point moved towards `target`, stopping there. |
| `Vector2.Lerp` | `Lerp(target Vector2, t float32) Vector2`: the point a share `t` of the way to `target`. |
| `Rectangle` | A rectangle aligned with the screen, in pixels: `golib.Rectangle{X: 10, Y: 20, Width: 40, Height: 60}`. |
| `Rectangle.X`, `Rectangle.Y` | Its top-left corner. |
| `Rectangle.Width`, `Rectangle.Height` | Its size. |
| `Rectangle.Center` | `Center() Vector2`: the point in its middle. |
| `Rectangle.Overlaps` | `Overlaps(other Rectangle) bool`: the two share some area. Rectangles that only touch along an edge don't overlap, so a player standing on a platform isn't inside it. |
| `Rectangle.Contains` | `Contains(x, y float32) bool`: the point is inside, such as the mouse over a button. The left and top edges are inside; the right and bottom ones are not. |

```go
// Moving with the keys or the stick, no faster diagonally.
move := golib.Vector2{X: moveX, Y: moveY}.ClampLength(1)
s.ship.position = s.ship.position.Add(move.Scale(shipSpeed * dt))

// Aiming at the mouse, through the camera, and firing that way.
aim := s.camera.ToWorld(input.MousePosition()).Sub(s.ship.position)
bullet := bullet{position: s.ship.position, velocity: aim.Normalize().Scale(bulletSpeed)}
s.ship.angle = aim.Angle() // for DrawOptions.Rotation

// Chasing the ship.
enemy.position = enemy.position.MoveTowards(s.ship.position, enemySpeed*dt)

// Circles touch when their centers are closer than their radii added up.
hit := enemy.position.Distance(bullet.position) < enemyRadius+bulletRadius
```

### Numbers

The small maths every game writes again: moving a number towards another one, keeping one inside its limits, and the curves that make a movement start or stop gently. Each takes and returns `float32`.

| Name | What it does |
| --- | --- |
| `Lerp` | `Lerp(a, b, t float32) float32`: the number a share `t` of the way from `a` to `b`. Outside 0 to 1 it carries on past the ends. `Vector2.Lerp` does the same for a point. |
| `Clamp` | `Clamp(value, low, high float32) float32`: `value` kept inside `low` to `high`. |
| `EaseIn` | `EaseIn(t float32) float32`: a share from 0 to 1 that starts slowly and speeds up, for something setting off. |
| `EaseOut` | `EaseOut(t float32) float32`: starts fast and slows down, for something coming to rest. The usual choice for menus and pop-ups. |
| `EaseInOut` | `EaseInOut(t float32) float32`: slow, fast, slow, for a camera or a door that shouldn't snap. |

```go
// A menu sliding in, and a health bar that never leaves the screen.
s.menuX = golib.Lerp(-menuWidth, 0, golib.EaseOut(s.opening.Progress()))
width := golib.Clamp(barWidth*s.health/maxHealth, 0, barWidth)
```

The easings take a share of the way from 0 to 1, which is usually `Timer.Progress`, and clamp anything outside it.

That is all the 2D geometry GoLib has: no physics in 2D (in 3D, see [Collisions and paths](#collisions-and-paths)). Pushing a player out of a wall is game code: `games/platformer/world.go` moves one axis at a time and pushes the player back out of any solid tile of the map it overlaps.

## Camera

A camera shows part of a world larger than the screen: a level that scrolls, an arena to fly around. Positions stay in the world; the camera decides what is on the screen.

| Name | What it does |
| --- | --- |
| `Camera` | Which part of the world the screen shows. Make one per scene with `NewCamera`, and keep it in the scene. |
| `NewCamera` | `NewCamera(width, height float32) *Camera`: a camera for the screen's size, the one in `Config`. It starts showing the world from 0, 0, as if there were none. |
| `Camera.Target` | The point in the world to keep in the middle of the screen, such as the player. |
| `Camera.Lag` | Seconds of smoothing: the view closes about two thirds of the way to `Target` in `Lag` seconds. 0, the default, keeps `Target` exactly in the middle. |
| `Camera.Zoom` | Screen pixels per world pixel: 2 shows everything twice as large. 0 counts as 1. |
| `Camera.Bounds` | The part of the world the view stays inside, such as the whole level, so the screen never shows beyond it. The zero `Rectangle` sets no limit. |
| `Camera.Update` | `Update(dt float32)`: moves the view towards `Target`, inside `Bounds`, and moves the shake on. Call it in every `Update`, after setting `Target`. |
| `Camera.Snap` | `Snap()`: puts the view on `Target` at once, without `Lag`: at the start of a level, or after a teleport. |
| `Camera.Shake` | `Shake(strength, seconds float32)`: shakes the view by up to `strength` world pixels, fading out over `seconds`, such as for an explosion. A stronger shake replaces a weaker one, so a game with a "how shaken is it" value can call it in every update. Call it from `Update`. |
| `Camera.Center` | `Center() Vector2`: the point of the world in the middle of the screen, shake included, so whatever a game places from it shakes too. |
| `Camera.View` | `View() Rectangle`: the part of the world on the screen, to skip drawing what isn't, or to place things just outside it. |
| `Camera.ToWorld` | `ToWorld(x, y float32) Vector2`: the world point at a screen point, such as `camera.ToWorld(input.MousePosition())`. |
| `Camera.ToScreen` | `ToScreen(point Vector2) Vector2`: where a world point is on the screen, possibly outside it, to draw a marker over it in screen pixels. |
| `Screen.SetCamera` | `SetCamera(camera *Camera)`: the drawing that follows is in the world, through `camera`. `SetCamera(nil)` goes back to screen pixels, for the score and menus. Every `Draw` starts without a camera. |

```go
type playScene struct {
	world  *world
	camera *golib.Camera
}

func newPlayScene() *playScene {
	s := &playScene{world: newWorld(), camera: golib.NewCamera(screenWidth, screenHeight)}
	s.camera.Bounds = golib.Rectangle{Width: level.Width(), Height: level.Height()}
	s.camera.Lag = 0.15
	s.camera.Target = s.world.player.Center()
	s.camera.Snap() // ready to draw before the first update
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	// ... move the player ...
	if s.world.player.hurt {
		s.camera.Shake(3, 0.3)
	}
	s.camera.Target = s.world.player.Center()
	s.camera.Update(dt)
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(skyColor)
	screen.SetCamera(s.camera)
	screen.DrawMap(level, 0, 0) // where the map is in the world
	screen.DrawSprite(hero, frame, s.world.player.X, s.world.player.Y)
	screen.SetCamera(nil)
	screen.DrawText(fmt.Sprint("Coins: ", s.world.coins), 4, 4, 10, golib.White)
}
```

- **The camera changes the view, not the rules.** Keep positions in the world, in the rules and in the tests; only `Draw` and the mouse go through the camera.
- The view sits on whole screen pixels, so pixel art, maps and text stay sharp while it moves. Zoom by whole numbers in pixel art games.
- With a camera set, `DrawMap` and `DrawMapLayer` draw only the tiles in the camera's view, and move layers with a parallax factor around its middle. Without one, pass the view's top-left corner, negated, as `x, y`.
- The shake uses `RandomFloat`, so call `Shake` and `Update` from `Update`, never from `Draw`: `golib shot` repeats it.
- `Update`, `Snap`, `View`, `ToWorld` and `ToScreen` work without a window, so tests can check what the camera shows.

## 3D

A 3D world, seen through a `Camera3D`, drawn between `screen.Begin3D` and `screen.End3D` in the same `Draw` as the 2D: the score and the menus go over it. In meters, with Y up; a model's front faces +Z. GoLib draws models from glTF and FBX files, as Blender, Mixamo and most tools export them, with their textures and materials, and shapes made in code, lit physically by a sun that casts shadows, the sky's soft light and lamps of the game's own, with fog and a sky behind, and plays their animations: characters bent by their bones, shape keys, blends between animations and layers of them. Characters walk through it with collisions and find their way along paths, and crates, barrels and balls fall, roll and stack as rigid bodies (see [Rigid bodies](#rigid-bodies)).

### Points, turns and places

| Name | What it does |
| --- | --- |
| `Vector3` | `golib.Vector3{X: 1, Y: 2, Z: 3}`, with `Vector3.X`, `Vector3.Y` and `Vector3.Z`: a point or a direction in meters. X and Z go across the ground, Y up. |
| `Vector3.Add`, `Vector3.Sub`, `Vector3.Scale` | As for `Vector2`: `target.Sub(from)` is the way from `from` to `target`. |
| `Vector3.Length`, `Vector3.Distance`, `Vector3.Dot` | As for `Vector2`. |
| `Vector3.Cross` | `Cross(other Vector3) Vector3`: at right angles to both, such as the up of a surface from two of its sides. |
| `Vector3.Normalize`, `Vector3.ClampLength`, `Vector3.MoveTowards`, `Vector3.Lerp` | As for `Vector2`. |
| `Vector3.Rotate` | `Rotate(rotation Rotation) Vector3`: a direction turned as a model turned by `rotation` turns it. |
| `Rotation` | How something is turned, in degrees: `golib.Rotation{Yaw: 90}`. The zero `Rotation` faces +Z. The turns happen Roll first, then Pitch, then Yaw: a cube stood on a corner is `golib.Rotation{Roll: 45, Pitch: 35.26}`. |
| `Rotation.Yaw` | Left and right around the world's up: 0 faces +Z, 90 faces +X, 180 faces -Z. Seen from behind, a larger Yaw turns left, because facing +Z the left side is +X: steer right with `yaw -= turnSpeed * dt`. |
| `Rotation.Pitch` | The front tilted up, positive, or down, negative: 90 faces straight up. |
| `Rotation.Roll` | Banking around the front: positive lowers the right side, as a plane rolls right. |
| `Rotation.Forward`, `Rotation.Right`, `Rotation.Up` | `Forward() Vector3` and the others: which way the front, the right side and the top point once turned, length 1. Move a ship with `position.Add(rotation.Forward().Scale(speed * dt))`. |
| `RotationTowards` | `RotationTowards(direction Vector3) Rotation`: the Yaw and Pitch that face `direction`, to aim a turret or turn a character to where it walks. |
| `Rotation.Turn` | `Turn(axis Vector3, degrees float32) Rotation`: turned further around any axis of the world, which three angles can't do on their own: a ball rolling along `velocity` turns around `golib.Vector3{Y: 1}.Cross(velocity)` by `speed * dt / radius * 180 / math.Pi` degrees. |
| `Transform` | Where a mesh is drawn: `Transform.Position`, `Transform.Rotation`, and `Transform.Scale`, which multiplies its size each way. A part of `Scale` left at 0 counts as 1, so `Scale: golib.Vector3{X: 40, Z: 40}` stretches a plane over the ground. |
| `Transform.Child` | `Child(local Transform) Transform`: where something placed at `local`, relative to this transform, is in the world: a turret on a tank, eyes on a face, a model's marker once the model is drawn moved. It moves, turns and scales with its parent. |

### Meshes and drawing

| Name | What it does |
| --- | --- |
| `Mesh` | A shape made of triangles. Make each once and keep it; draw it as many times as needed. |
| `NewBoxMesh` | `NewBoxMesh() *Mesh`: a box one meter each way, centered on 0, 0, 0. Scale it into walls, floors, crates and platforms. |
| `NewSphereMesh` | `NewSphereMesh() *Mesh`: a ball one meter across. |
| `NewCylinderMesh` | `NewCylinderMesh() *Mesh`: a closed cylinder one meter across and one high, standing on Y, centered on 0, 0, 0. |
| `NewPlaneMesh` | `NewPlaneMesh() *Mesh`: a flat square one meter a side, on the ground, facing up and seen from below too. Scale X and Z into a floor. |
| `NewConeMesh` | `NewConeMesh() *Mesh`: a cone one meter across at its base and one high, pointing up Y, centered on 0, 0, 0. |
| `NewCapsuleMesh` | `NewCapsuleMesh() *Mesh`: a capsule one meter high and half a meter across, the usual shape of a character, centered on 0, 0, 0. Scale it the same way each way. |
| `NewMesh` | `NewMesh(vertices []Vertex, indices []int) *Mesh`: a mesh made of triangles in code, three indices a triangle, counterclockwise from the side it faces: terrain from heights, a ramp, a gem. |
| `Vertex` | A corner of a `NewMesh` mesh: `Vertex.Position`; `Vertex.Normal`, which `NewMesh` works out when every vertex leaves it zero, smooth where triangles share a vertex; `Vertex.U` and `Vertex.V`, where it is on a texture; `Vertex.Color`, blended across the triangle and multiplied by the color it is drawn in, the zero `Color` counting as `White`. |
| `Screen.Begin3D` | `Begin3D(camera *Camera3D)`: starts drawing the 3D world through `camera`, over what `Draw` has drawn so far. The 3D is drawn at `End3D`, so 2D drawn before `End3D` ends up under it. |
| `Screen.End3D` | `End3D()`: draws the 3D and goes back to 2D; what follows goes over it. `Run` ends a pass left open at the end of `Draw`. |
| `Screen.DrawMesh` | `DrawMesh(mesh *Mesh, transform Transform, color Color)`: the mesh, lit. A color with `A` below 255 is see-through. |
| `Screen.DrawWireframe` | `DrawWireframe(mesh *Mesh, transform Transform, color Color)`: only its edges, one pixel wide, unlit: the box a game checks collisions with, while testing. |
| `Screen.DrawLine3D` | `DrawLine3D(from, to Vector3, color Color)`: a line one pixel wide, unlit, hidden by what is in front of it. |
| `Screen.DrawGrid` | `DrawGrid(slices int, spacing float32, color Color)`: a square grid on the ground around 0, 0, 0, `slices` squares a side, `spacing` meters apart. |
| `Screen.DrawAxes` | `DrawAxes(position Vector3, length float32)`: X in red, Y in green and Z in blue, from `position`, to see which way is which. |

### Materials

`DrawMesh` paints a mesh one color. `DrawMeshWith` gives it a `Material`: a texture, metal, a glow, or a shader of the game's own. Make materials once, like meshes.

| Name | What it does |
| --- | --- |
| `Material` | How a surface looks: `&golib.Material{Color: golib.Gold, Metallic: 1, Roughness: 0.3}`. Fields left at zero keep a painted look: white, not metallic, roughness 0.6. A changed field shows at the next draw. |
| `Screen.DrawMeshWith` | `DrawMeshWith(mesh *Mesh, transform Transform, material *Material)`: the mesh, its surface as the material says. |
| `Screen.DrawMeshWithTint` | `DrawMeshWithTint(mesh *Mesh, transform Transform, material *Material, tint Color)`: the same, its colors multiplied by `tint` and see-through by its opacity: one `Material` for every shade of a thing, a wake that fades. |
| `Material.Color` | Multiplies the texture, or is the color; White when zero. `A` below 255 is see-through. |
| `Material.Texture`, `Material.Tiling` | A `Texture` wrapped around the mesh by its texture coordinates; `Tiling` repeats it that many times each way, for a floor of tiles (1 when zero). |
| `Material.Metallic`, `Material.Roughness` | 0 for paint, wood, stone or plastic, 1 for bare metal, which tints what it reflects; and how blurred reflections are, 0.05 for a mirror, 1 for chalk (0.6 when zero). |
| `Material.Emission`, `Material.EmissionStrength` | The light the surface gives off, shown even in the dark: lava, screens, neon. `EmissionStrength` multiplies it (1 when zero); above white it glows with `Environment.Bloom`. |
| `Material.Unlit` | The color or texture as it is, with no light or shadow: a sky dome, a sign. As everywhere in the 3D, `Environment.Exposure` brightens or darkens it, and only colors above about 225 of 255 are softened a little by the tonemapper. |
| `Material.TwoSided` | Draws the back of each triangle too: leaves, flags, paper. |
| `Material.Toon` | Lights the surface in flat bands, as a cartoon is drawn: 2 for light and shade, 3 or 4 for softer steps, with a sharp highlight. 0 is lit smoothly. Shadows, lamps, the sky's light and fog still reach it. |
| `Material.Outline`, `Material.OutlineWidth` | A line of that color round the surface's edges as the camera sees them, `OutlineWidth` pixels wide whatever the distance (2 when zero); the zero `Color` is none. A mesh with smooth normals, such as a sphere or most models, gets a line all round; one whose faces don't share their corners, such as `NewBoxMesh`, gets one broken at its corners, and a sharp point, a gem's tip, gets a line thinning towards it. Only solid surfaces get one. Lines fade into fog and haze as what they outline does, and a bright glow nearby, bloom, spills over them. |
| `Material.Shader` | A GLSL function of the game's own, `void surface(inout Surface s)`, that changes the surface before GoLib lights it: `s.color` (vec4, linear), `s.emission` (vec3), `s.metallic`, `s.roughness`, `s.normal` (in the world), and to read, `s.position` (in the world), `s.texcoord` and `s.time` (seconds of game time). Lighting, shadows and fog stay GoLib's. One that doesn't compile stops `Run` with what the graphics card said. |
| `Texture` | A picture for 3D, from a PNG or JPEG file: smoothed, with mipmaps, repeating past its edges. |
| `NewTexture` | `NewTexture(name string) *Texture`: the texture in the assets folder named `name`, read the first time it is drawn. |
| `Texture.Pixelated` | Keeps a pixel art texture's pixels sharp up close. Set it before it is first drawn. |
| `Texture.Err` | `Err() error`: reads it, and says what stopped it from loading. |

```go
var (
	gold  = &golib.Material{Color: golib.Gold, Metallic: 1, Roughness: 0.3}
	lava  = &golib.Material{Color: golib.Black, Emission: golib.Orange, EmissionStrength: 4}
	tiles = &golib.Material{Texture: golib.NewTexture("textures/tiles.png"), Tiling: 20}
	toon  = &golib.Material{Color: golib.Orange, Toon: 2, Outline: golib.Black, OutlineWidth: 3}
	water = &golib.Material{Color: golib.Color{R: 40, G: 110, B: 160, A: 255}, Roughness: 0.05, Shader: `
void surface(inout Surface s) {
    s.normal = normalize(s.normal + 0.08 * vec3(sin(s.position.x * 3.0 + s.time * 2.0), 0.0, cos(s.position.z * 2.5 + s.time)));
}`}
)

screen.DrawMeshWith(s.plane, golib.Transform{Scale: golib.Vector3{X: 20, Z: 20}}, tiles)
screen.DrawMeshWith(s.sphere, golib.Transform{Position: coin}, gold)
```

### Models from Blender

A model is a glTF file, `.glb` or `.gltf`, or an FBX file, `.fbx`, in the game's assets folder; a level is one too, made in Blender, or a text file of the game's own read with `ReadAsset` that places a pack's models, as [games/grove](../games/grove) does: Blender's File > Export > glTF 2.0, Mixamo's downloads, and most other 3D tools and asset packs. Both are read the same way, and everything below works with both. GoLib reads its meshes, their materials (color, texture, emission, unlit, see-through or cut out, one or two sides), vertex colors and the empties placed in it, with their custom properties: a level made in Blender carries where things go. Make each model once and keep it.

| Name | What it does |
| --- | --- |
| `Model` | A model from a glTF or FBX file. Read the first time it is drawn or asked about; its textures go to the graphics card the first time it is drawn. |
| `NewModel` | `NewModel(name string) *Model`: the model in the assets folder named `name`, such as `"models/ship.glb"` or `"models/knight.fbx"`. A `.gltf` file's `.bin` and textures, and an FBX file's textures that aren't inside it, are read from its folder. |
| `Model.AddAnimations` | `AddAnimations(names ...string) *Model`: reads the animations of other files into the model, matching their bones by name: Mixamo's way, each animation an FBX file of its own without the character. A file with one animation names it after the file, so `"models/run.fbx"` plays as `"run"`. It returns the model: `golib.NewModel("models/hero.fbx").AddAnimations("models/idle.fbx", "models/run.fbx")`. |
| `Screen.DrawModel` | `DrawModel(model *Model, transform Transform, tint Color)`: the model, placed by `transform`. `golib.White` draws it as made; another tint multiplies its colors, and `A` below 255 makes it see-through. |
| `Model.Bounds` | `Bounds() (lowest, upmost Vector3)`: the box around it, in its own space: how big it is, and where its feet are. |
| `Model.Markers` | `Markers() []ModelMarker`: the empties placed in it, and any other node without a mesh, in the file's order. |
| `Model.Marker` | `Marker(name string) (ModelMarker, bool)`: the first marker called `name`, such as the player's start. |
| `Model.Toon`, `Model.Outline` | `Toon(bands int)` and `Outline(color Color, width float32)`: what `Material.Toon` and `Material.Outline` do, for every material of the model, wherever it is drawn: `hero.Toon(2); hero.Outline(golib.Black, 3)` for a cartoon character. `Toon(0)` and the zero `Color` take them away. |
| `Model.Hide`, `Model.Show` | `Hide(names ...string)`: hides the objects Blender named so, and the objects under them, wherever the model is drawn, until `Show(names ...string)`: a coin collected from a level, a door opened. A name the model doesn't have is reported. A `PhysicsWorld` the model was added to keeps them solid: `Remove` what goes. |
| `Model.Err` | `Err() error`: reads it, and says what stopped it from loading. Drawing or asking a model that didn't load stops `Run` with the same message, so a game needs it only to handle the mistake itself, or to check its models in a test. |
| `ModelMarker` | A marker: `ModelMarker.Name`, `ModelMarker.Position`, `ModelMarker.Rotation` and `ModelMarker.Scale` in the model's own space, and `ModelMarker.Properties`, its custom properties, read like a Tiled object's (`Properties.String`, `Properties.Float` and the others). |

```go
var level = golib.NewModel("models/level.glb")
var coin = golib.NewModel("models/coin.glb")

func newPlayScene() *playScene {
	s := &playScene{camera: golib.NewCamera3D(screenWidth, screenHeight)}
	if start, ok := level.Marker("Start"); ok {
		s.player.Position = start.Position
	}
	for _, m := range level.Markers() {
		if m.Properties.String("kind") == "coin" {
			s.coins = append(s.coins, m.Position)
		}
	}
	return s
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.SkyBlue)
	screen.Begin3D(s.camera)
	screen.DrawModel(level, golib.Transform{}, golib.White)
	for _, at := range s.coins {
		screen.DrawModel(coin, golib.Transform{Position: at, Rotation: golib.Rotation{Yaw: s.spin}}, golib.White)
	}
	screen.End3D()
}
```

- **FBX files:** binary FBX 7.1 and later, as every current tool saves them. A take ends where its last movement does: Blender exports each action over the scene's whole timeline, 250 frames unless it was set, holding the last pose after the moves, and GoLib leaves that out; it also starts at the scene's first frame, so an action keyed from frame 0 loses that frame unless the scene starts there. GoLib turns their units and axes into meters, Y up and +Z forward, and reads their meshes, materials (color, texture, normal map, emission, opacity), skins with four bones a vertex, shape keys with their names, custom properties as a marker's `Properties`, and each take as an animation, named as Blender names its action. Textures are read from inside the file, or from beside it. An FBX file saved as text, or older than 2011, is refused with what to do instead; a file's lights, cameras and in-between shapes are left out.
- **Mixamo:** download the character as FBX Binary, With Skin, and each animation as FBX Binary, Without Skin, at 30 frames a second, then read them together with `AddAnimations`. Tick In Place for animations the game moves itself, or leave it unticked and play them with `Animator.RootMotion`. Mixamo names its bones `"mixamorig:Hips"`, `"mixamorig:Spine"` and so on: `Model.Bones` lists them.
- **Exporting from Blender:** File > Export > glTF 2.0, format glTF Binary (`.glb`), with +Y Up ticked, as it is by default. Tick Include > Custom Properties for the markers' properties. Leave Compression off: GoLib refuses Draco and says so. Keep the source `.blend` in `sources/`, at the same path as the `.glb` in `assets/`.
- **Size and facing:** Blender's meter is GoLib's meter, and a model's front, Blender's -Y, faces +Z, as a zero `Rotation` faces. Check a new model's size with `Bounds`.
- **Materials are drawn as made:** color and texture, metallic and roughness, normal and occlusion maps, emission, unlit, see-through or cut out. Not read yet: a file's own lights and cameras; light a model with `Screen.AddLight`.
- **A rigged model is drawn at rest by `DrawModel`**, bent by its bones as the file's rest pose has them. To play its animations, draw it through an `Animator` (below).
- **Copies are cheap.** Every copy of the same model or mesh in a frame is drawn together, and what the camera doesn't see is left out: a forest of the same tree costs little more than one tree.

### Animation

A model's animations, as Blender exports its actions, play through an `Animator`: one for each thing that moves, since each character is in its own pose, while the `Model` is shared. Bones bend the meshes skinned to them (on the graphics card, up to four bones a vertex), shape keys blend a mesh towards other shapes, and any other node moves too: a door, a windmill's sails. Animations fade into each other, play once or loop, at any speed, and play on part of a skeleton over the rest.

| Name | What it does |
| --- | --- |
| `Animator` | Plays a model's animations for one character, enemy or door. Make one each, keep it in the thing it moves. |
| `NewAnimator` | `NewAnimator(model *Model) *Animator`: shows the model at rest until it plays something. |
| `Model.Animations` | `Animations() []string`: the names of its animations, in the file's order: Blender's actions, such as `"Run"` or `"Idle"`. |
| `Model.AnimationLength` | `AnimationLength(name string) float32`: how many seconds it lasts at its own speed, 0 for a name it doesn't have. |
| `Model.Bones` | `Bones() []string`: the names of its bones, what `AnimationOptions.Bone` and `Animator.Bone` take. Print them from a test with `t.Log`, and read them with `golib test <game> --run <TestName>`. |
| `Animator.Play` | `Play(name string)`: fades into the animation called `name` in 0.2 seconds and loops it. Playing what already plays changes nothing, so call it every update with what should play; the first animation an Animator plays starts at once. |
| `Animator.PlayWith` | `PlayWith(name string, options AnimationOptions)`: plays it as the options say. |
| `AnimationOptions` | How `PlayWith` plays an animation; the zero value is what `Play` does. |
| `AnimationOptions.Fade`, `AnimationOptions.NoFade` | Seconds to blend into it from what played: 0.2 when zero. `NoFade` switches at once. |
| `AnimationOptions.Speed` | How fast: 1 when zero, 2 twice as fast, -1 backwards. Playing it again with another speed changes it as it plays, such as a walk that follows how fast the character goes. |
| `AnimationOptions.Once` | Plays to its end and holds the last pose there, instead of looping: a jump, an attack, a death. |
| `AnimationOptions.Then` | With `Once`, the animation to fade into when it ends: `"Idle"` after `"Attack"`. |
| `AnimationOptions.Bone` | Plays only on that bone and the ones below it, over what plays on the rest: `"Spine"` swings a sword while the legs run. Each `Bone` is a layer of its own, and the one played last goes over the others where they share bones, so a nod on `"Spine"` covers an arm holding a stick until the arm's animation is played again; any node of the model can be named. A layer played `Once` without `Then` fades out at its end. Shape key animations on a layer go over the rest too, whatever the bone: a face's expression over the walk. |
| `AnimationOptions.Restart` | Starts it again from the beginning even if it is playing: a second punch. |
| `Animator.Stop` | `Stop(bone string)`: fades out the layer of `bone`, back to what plays under it; `Stop("")` fades the whole model back to its rest pose. |
| `Animator.Update` | `Update(dt float32)`: moves the animations on. Call it in every `Update`. |
| `Screen.DrawAnimated` | `DrawAnimated(animator *Animator, transform Transform, tint Color)`: draws the model posed as its animations have it now, as `DrawModel` draws it. |
| `Animator.Playing` | `Playing() string`: the animation the whole model plays or fades into, the last played without a `Bone`; `""` at rest. |
| `Animator.Time` | `Time() float32`: seconds into it, in the animation's own time, from 0 to its length, starting again as it loops: the moment a swing hits. At `Speed` 2 it reaches the length in half the real time. |
| `Animator.Finished` | `Finished() bool`: what `Playing` names was played `Once` and has reached its end. |
| `Animator.PlayingOn`, `Animator.TimeOn` | `PlayingOn(bone string) string` and `TimeOn(bone string) float32`: the same for the layer of `bone`. `PlayingOn` is `""` once the layer has faded out, as one played `Once` does at its end: the swing is over. |
| `Animator.Bone` | `Bone(name string) Transform`: where a bone is now, in the model's own space, without its scale: place what the character holds with `Transform.Child`. Its position is the bone's head, the joint it turns around, such as the shoulder for an arm, and its axes are the file's own: in Blender, a bone's Y runs along it, though a model from elsewhere may differ. A marker parented to a bone in Blender follows it, and is the best way to say where a held thing sits and how it is turned; without one, place the thing with a `Child` of the bone, and check it in a shot. |
| `Animator.SetShapeKey` | `SetShapeKey(name string, weight float32)`: holds the shape key called `name` at `weight`, 0 as modeled to 1 fully the shape, over what the animations give it: a blink, a smile, a bulge. |
| `Animator.RootMotion` | When true, the movement of the skeleton's top bone across the ground is taken out of the pose and given to `Moved`, for the game to move the character by, so its feet don't slide: for walks animated moving forward rather than in place. Up and down stays in the pose. |
| `Animator.Moved` | `Moved() Vector3`: how far the animations carried the top bone across the ground in the last `Update`, with `RootMotion` on, in the model's own space: turn it as the model is turned. |

```go
var hero = golib.NewModel("models/hero.glb")

type player struct {
	position golib.Vector3
	yaw      float32
	velocity golib.Vector3
	onGround bool
	animator *golib.Animator
}

func newPlayer() *player {
	return &player{animator: golib.NewAnimator(hero)}
}

func (p *player) update(input *golib.Input, dt float32) {
	// ... move the player, then say what plays, every update:
	switch {
	case !p.onGround:
		p.animator.Play("Fall")
	case p.velocity.Length() > 4:
		p.animator.Play("Run")
	case p.velocity.Length() > 0.1:
		p.animator.PlayWith("Walk", golib.AnimationOptions{Speed: p.velocity.Length() / 2})
	default:
		p.animator.Play("Idle")
	}
	if input.KeyPressed(golib.KeyX) {
		p.animator.PlayWith("Attack", golib.AnimationOptions{Bone: "Spine", Once: true, Restart: true})
	}
	p.animator.Update(dt)
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.SkyBlue)
	screen.Begin3D(s.camera)
	body := golib.Transform{Position: s.player.position, Rotation: golib.Rotation{Yaw: s.player.yaw}}
	screen.DrawAnimated(s.player.animator, body, golib.White)
	hand := body.Child(s.player.animator.Bone("SwordSocket")) // an empty parented to the hand in Blender
	screen.DrawModel(sword, hand, golib.White)
	screen.End3D()
}
```

- **Animations are named as the file names them.** `Model.Animations` lists them, and playing a name the model doesn't have stops `Run` with the names it has, as a missing bone or shape key does. Kenney's characters name theirs `"idle"`, `"walk"`, `"sprint"`, `"jump"` and more, in lowercase.
- **Exporting animations from Blender:** each action becomes an animation of its name, with Animation ticked in the glTF export, as it is by default. Keep every action the export should find in the NLA editor (Push Down) or with a fake user, and leave Include all bone influences unticked: GoLib reads four bones a vertex. Shape keys export with Shape Keys ticked, their names with them, and a shape key action becomes an animation too.
- **The state is the game's; the blending is the Animator's.** Work out what should play from the game's own state, as above, and call `Play` every update: the same name goes on, another fades in. Put the moment an attack lands in the rules, as a time, and check it against `Time` or a timer, rather than in the drawing.
- **Copies are still cheap.** Every animated copy of one model is drawn together, each in its own pose: 400 of Kenney's characters walking took 2 ms of each frame on the iMac of [docs/3d.md](../docs/3d.md).
- `Update`, `Bone`, `Playing`, `Time`, `Finished` and `Moved` work without a window, so a test can check that a jump plays `"Jump"`, or where a hand is.

### Collisions and paths

A `PhysicsWorld` is what is solid in a 3D world, as triangles: a level from Blender, the meshes a game places. Ask it where a ray hits, for the mouse or a shadow, move a `CharacterController` through it, as a player walks, and find paths across it for enemies with a `NavMesh`, and add rigid bodies to it, things that fall, roll, topple and stack on their own: crates, barrels, balls.

| Name | What it does |
| --- | --- |
| `PhysicsWorld` | What is solid: add things once, in the scene's constructor, and ask it in `Update`. |
| `NewPhysicsWorld` | `NewPhysicsWorld() *PhysicsWorld`: an empty one. |
| `PhysicsWorld.AddModel` | `AddModel(model *Model, transform Transform) int`: every mesh of a model, at rest, placed by `transform`, and its number. A mesh whose object has the custom property `collide` set to `false` in Blender is left out: water, grass, leaves. |
| `PhysicsWorld.AddMesh` | `AddMesh(mesh *Mesh, transform Transform) int`: a mesh, such as a box scaled into a wall, and its number. |
| `PhysicsWorld.Remove` | `Remove(id int)`: takes a thing out: a door opened, a wall broken. |
| `PhysicsWorld.SetTransform` | `SetTransform(id int, transform Transform)`: moves a thing, as a lift moves, and carries a `CharacterController` standing on it at its next `Move`: move a lift once each update, before the characters. |
| `PhysicsWorld.Raycast` | `Raycast(origin, direction Vector3, distance float32) (RayHit, bool)`: where a ray first hits, within `distance` meters, from either side of a surface. With `Camera3D.Ray`, what the mouse points at. |
| `PhysicsWorld.SphereCast` | `SphereCast(origin, direction Vector3, radius, distance float32) (RayHit, bool)`: the same for a ball `radius` meters across: where its center is when it first touches. |
| `RayHit` | `RayHit.Position`, where it hits; `RayHit.Normal`, the way the surface faces there; `RayHit.Distance`, how far along; `RayHit.Body`, the number of what it hit. |
| `CharacterController` | A character as a capsule standing on its feet, moved through a `PhysicsWorld`: it slides along walls, walks up slopes and steps up ledges, stands on the ground and stops under ceilings. Keep one in the player and in each enemy. |
| `CharacterController.Position` | Where its feet are: draw the character's model there. |
| `CharacterController.Height`, `CharacterController.Radius` | The capsule's height and how far it reaches from its middle: 1.8 and 0.3 meters when zero. Set them to the model's size. |
| `CharacterController.StepHeight`, `CharacterController.MaxSlope` | The tallest ledge it steps up as it walks, 0.3 meters when zero; the steepest ground it stands and walks up on, in degrees, 45 when zero: steeper is a wall it slides down. |
| `CharacterController.Move` | `Move(world *PhysicsWorld, motion Vector3)`: moves it by `motion`, its velocity times `dt`, as far as the world lets it. |
| `CharacterController.OnGround`, `CharacterController.GroundNormal`, `CharacterController.OnCeiling`, `CharacterController.OnWall` | What the last `Move` ended touching: standing on the ground, which way that ground faces, its head against a ceiling, a wall in the way. |
| `CharacterController.GroundBody` | The number of what it stands on, as the `Add` method returned it, or 0: is the player on the lift? |
| `CharacterController.Moved` | How far the last `Move` really took it, along walls and carried by a lift included: `Moved.Scale(1 / dt)` is its real velocity, the one to give `Camera3D.Follow`. |
| `NavMesh` | Where characters of one size can walk in a `PhysicsWorld`, to find their way round what is in it. |
| `NewNavMesh` | `NewNavMesh(world *PhysicsWorld, character CharacterController) *NavMesh`: where characters sized as `character` walk: its `Height`, `Radius`, `StepHeight` and `MaxSlope`. A grid of places `Radius` apart, on ground above ground too: make it once the world is built, which takes about half a second for 100 by 100 meters, and again when the world changes where they can go. |
| `NavMesh.Path` | `Path(from, to Vector3) ([]Vector3, bool)`: the points to walk through in turn, straight from one to the next, from `from` to `to`, or to the ground under `to` when it is in the air; false when there is no way. |

#### Rigid bodies

Rigid bodies are simulated by [Jolt Physics](https://github.com/jrouwe/JoltPhysics), the engine of Horizon Forbidden West and of Godot: in GoLib's native library on the desktop and as JoltPhysics.js in a browser, which `golib setup` downloads and the game's builds carry, only when it uses them (see [native/README.md](../native/README.md)). They meet what else is in the `PhysicsWorld`: they land on the level, rays hit them, characters stand on them and push them.

| Name | What it does |
| --- | --- |
| `PhysicsWorld.AddRigidBody` | `AddRigidBody(shape BodyShape, transform Transform, options RigidBodyOptions) *RigidBody`: a body of `shape`, placed by `transform`, whose `Scale` stretches the shape, as it stretches a mesh. Add the level first. |
| `PhysicsWorld.Update` | `Update(dt float32)`: moves the rigid bodies on by `dt`. Call it in every `Update` that has rigid bodies, after moving lifts with `SetTransform` and before moving characters. |
| `PhysicsWorld.Gravity` | What pulls the rigid bodies, in meters per second per second: Earth's, 9.81 down, when zero. Characters fall as the game makes them, as before. |
| `BodyShape` | A rigid body's shape. Boxes, balls and capsules are the cheapest, and roll and stack best. |
| `BoxShape`, `SphereShape`, `CapsuleShape`, `CylinderShape` | `BoxShape(size Vector3)`, a box `size` wide, high and deep, centered on the body's position: a crate; `SphereShape(radius float32)`: a ball; `CapsuleShape(height, radius float32)`: a capsule standing on Y, centered, round at both ends: a bowling pin; `CylinderShape(height, radius float32)`: a cylinder standing on Y, centered, flat at both ends, as `NewCylinderMesh` scaled by `Vector3{X: 2 * radius, Y: height, Z: 2 * radius}`: a barrel, a log, a column. |
| `ModelShape`, `MeshShape` | `ModelShape(model *Model)` and `MeshShape(mesh *Mesh)`: the shape around a model's meshes, at rest, or a mesh's, with any hollows filled in: a rock, a chair, a statue. Make the shape once and add as many bodies of it as the game needs. |
| `RigidBodyOptions` | What it is made of and how it moves; the zero value is light wood that falls, slides and bounces a little. |
| `RigidBodyOptions.Mass` | Kilograms: when zero, its size of light wood, 250 kilograms a cubic meter, so a meter-wide crate is 250 and a character pushes it. Heavier things push lighter ones aside. A game where thrown balls knock things over sets lighter masses, as toys are: a 5 kg ball barely moves a 250 kg crate. |
| `RigidBodyOptions.Friction`, `RigidBodyOptions.NoFriction` | How it grips, from 0, ice, to 1, rubber: 0.5 when zero; `NoFriction` makes it ice. |
| `RigidBodyOptions.Bounce` | How much of its speed it keeps as it hits something, from 0, a sack, to 1, a rubber ball. |
| `RigidBodyOptions.Moved` | A body the game moves itself with `MoveTo`, which pushes what is in its way and that nothing pushes back: a piston, a swinging door, a paddle. |
| `RigidBodyOptions.Fast` | Checks the whole way it moves each update, so a small, fast thing doesn't pass through a thin wall: a ball hit hard. It costs more: only the few that need it. |
| `RigidBodyOptions.NoGravity` | Floats where it is pushed: a balloon, a thing in space. |
| `RigidBody` | A body that moves on its own. Keep the pointer to draw it and push it. |
| `RigidBody.Transform`, `RigidBody.Position` | Where it is and how it is turned now, with the `Scale` it was added with: draw its mesh or model with `Transform()`. |
| `RigidBody.Velocity`, `RigidBody.Resting`, `RigidBody.Mass` | How fast it moves, in meters per second; whether it has come to rest, a crate that has landed; how heavy it is. |
| `RigidBody.ID` | Its number in the world: what `RayHit.Body` and `CharacterController.GroundBody` say when they meet it. |
| `RigidBody.SetVelocity` | `SetVelocity(velocity Vector3)`: how fast it moves, at once: a ball thrown. |
| `RigidBody.Push`, `RigidBody.PushAt` | `Push(impulse Vector3)`: a blow through its middle, in kilograms times meters per second, so the same kick sends a ball flying and nudges a car; `PushAt(impulse, point Vector3)`: a blow at a point on it, which spins it too: a crate hit at a corner. |
| `RigidBody.Teleport` | `Teleport(transform Transform)`: puts it somewhere else at once, still moving as it moved: a ball respawned. |
| `RigidBody.MoveTo` | `MoveTo(transform Transform)`: moves a body added with `Moved` there by the next `Update`, pushing what is in its way. Call it in every update the body moves. |
| `RigidBody.Remove` | `Remove()`: takes it out of the world: a crate broken, a ball lost. |

```go
var crateMesh = golib.NewBoxMesh() // one meter each way, as the shape

func newPlayScene() *playScene {
	s := &playScene{physics: golib.NewPhysicsWorld()}
	s.physics.AddModel(level, golib.Transform{})
	box := golib.BoxShape(golib.Vector3{X: 1, Y: 1, Z: 1})
	for i := range 5 { // a stack of crates
		at := golib.Vector3{Y: 0.5 + float32(i)}
		s.crates = append(s.crates, s.physics.AddRigidBody(box, golib.Transform{Position: at}, golib.RigidBodyOptions{}))
	}
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	s.physics.Update(dt) // before the characters
	s.player.body.Move(s.physics, s.player.velocity.Scale(dt)) // walks into the crates and pushes them
}

func (s *playScene) Draw(screen *golib.Screen) {
	// ...inside Begin3D and End3D:
	for _, c := range s.crates {
		screen.DrawMesh(crateMesh, c.Transform(), golib.Brown)
	}
}
```

- **A character pushes what it walks into** as a strong person would: a ball rolls away at its pace, a meter-wide crate slides briskly, two stacked slowly, three or a 2000-kilogram block not at all. It stands on rigid bodies, and is carried when they move under it. Rigid bodies meet a character as a post: a crate dropped on its head lands there or slides off, a ball bounces off its legs; they never push it. Jolt keeps a capsule for each character that moved since the last `Update`, a little thinner than its `CharacterController`.
- **The simulation follows the world's still and moved things**: a lift moved with `SetTransform` carries the crates on it, a wall taken out with `Remove` lets them fall. Add rigid bodies once the level is in.
- **Hundreds of bodies are fine**, and thousands when most are at rest: Jolt stops working out what has come to rest until something touches it. 118 crates and balls, most of them moving, took 0.15 to 0.35 milliseconds an update on an Intel Mac, and 0.7 in Chrome; a character's `Move` beside 118 falling balls took 0.4 milliseconds more, and 0.01 once they had come to rest, since bodies at rest keep their triangles where they are.
- **A world the game keeps no longer is freed**, Jolt's copy included, at the next `Update` of another: make a new `PhysicsWorld` for each level and drop the old one.
- **No collision events yet**: a sound when things hit is a sudden change in a body's `Velocity` from one update to the next, which the game checks.
- **When the native library is missing** (for every message and its fix, see [docs/tooling.md](../docs/tooling.md#when-rigid-bodies-dont-work)), `AddRigidBody` reports why, `golib build` fails and says to run `golib setup`, and the bodies stay where they were added.

```go
type player struct {
	body     golib.CharacterController
	velocity golib.Vector3
}

func newPlayScene() *playScene {
	s := &playScene{physics: golib.NewPhysicsWorld(), camera: golib.NewCamera3D(screenWidth, screenHeight)}
	s.physics.AddModel(level, golib.Transform{})
	start, _ := level.Marker("Start")
	s.player.body = golib.CharacterController{Position: start.Position, Height: 1.8, Radius: 0.3}
	s.nav = golib.NewNavMesh(s.physics, golib.CharacterController{Height: 1.8, Radius: 0.4})
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	p := &s.player
	var forward, right float32
	if input.KeyDown(golib.KeyUp) {
		forward = 1
	}
	if input.KeyDown(golib.KeyRight) {
		right = 1
	}
	move := s.camera.Direction(forward, right) // as the player sees it
	p.velocity.X, p.velocity.Z = move.X*runSpeed, move.Z*runSpeed
	p.velocity.Y -= gravity * dt
	if p.body.OnGround && input.KeyPressed(golib.KeySpace) {
		p.velocity.Y = jumpSpeed
	}
	p.body.Move(s.physics, p.velocity.Scale(dt))
	if p.body.OnGround && p.velocity.Y < 0 || p.body.OnCeiling && p.velocity.Y > 0 {
		p.velocity.Y = 0
	}
	s.camera.Follow(p.body.Position.Add(golib.Vector3{Y: 1.5}), p.body.Moved.Scale(1/dt), 6, -20, dt)
	s.camera.AvoidWalls(s.physics)
	s.camera.Update(dt)
}
```

- **Give `Follow` the movement the character really made**, `body.Moved.Scale(1 / dt)`, not the velocity the keys ask for: pushing into a wall, the character slides one way while the keys push another, and the camera would swing round the wrong way. With a `Follow` camera, a sideways key makes the character run in a circle, since the camera swings round behind it: steer by turning the camera, and in a `golib shot --input` script, by its turning keys and forward.

- **Gravity and jumps are the game's.** `Move` moves by what it is given: add gravity to the velocity every update, and set the vertical speed to 0 on the ground and under a ceiling, as above.
- **Enemies walk the same way:** a `CharacterController` each, moved towards the next point of their path. Ask for a new path every half second or so rather than every update: a path across a large level takes a few milliseconds. Characters don't collide with each other: keep them apart in the game, by their distance. A line of sight is a `Raycast` from eyes to eyes. Making a `NavMesh` again takes tens of milliseconds for a level of rooms: do it when a door opens, not every update.
- **Picking:** `origin, direction := s.camera.Ray(input.MousePosition())` and `s.physics.Raycast(origin, direction, 100)`. Things the game moves itself every frame, such as enemies, aren't in the world: check them against the ray in the game, by their distance from it.
- **Triangles are solid from both sides, and thin ones can be passed through** by something fast: a wall should be a box, not a plane, and a bullet a ray from where it was to where it is.

### Light, sky and fog

A pass is lit by a sun and by the soft light of the sky and the ground; `Screen.SetEnvironment` changes them, and adds fog and a sky behind everything. `Screen.AddLight` adds lamps. Materials are lit physically, in light brighter than white where it is, and brought back to the screen's colors by a tonemapper that leaves a lit color as it is and softens only what is brighter: `golib.Red` facing the sun is `golib.Red`.

| Name | What it does |
| --- | --- |
| `Environment` | How a pass is lit, and what shows behind it. Every field left at zero keeps GoLib's default. |
| `Screen.SetEnvironment` | `SetEnvironment(environment Environment)`: sets it for the pass begun last; every pass starts with the zero `Environment`. Call it between `Begin3D` and `End3D`. |
| `Environment.SunDirection` | The way the sunlight travels: straight down is `golib.Vector3{Y: -1}`, a low evening sun `golib.Vector3{X: 1, Y: -0.2}`. With an `HDRI`, from its picture's sun when zero. |
| `Environment.SunColor`, `Environment.SunStrength` | The sun's color, a soft white when zero, none with `golib.Black`; and how strong it is, 1 when zero, 2 or 3 for a glaring desert. With an `HDRI`, the zero `SunColor` is its picture's sun, as bright as it is there, and none for a picture without one; `SunStrength` still multiplies it. |
| `Environment.Sky`, `Environment.Ground` | The soft light from above and below that lights the side away from the sun; with `SkyBackground`, also the sky overhead and the ground below the horizon. |
| `Environment.SkyBackground`, `Environment.Horizon` | Draws the sky behind everything, fading from `Sky` overhead to `Horizon` at eye level, a paler `Sky` when zero, and to `Ground` below. Without it, what `Draw` drew before `Begin3D` shows behind the 3D. |
| `Environment.FogColor`, `Environment.FogDistance` | Fog that starts at a tenth of `FogDistance` meters and hides everything at `FogDistance`; 0 is no fog. The color defaults to the horizon's. |
| `Environment.Haze`, `Environment.HazeColor`, `Environment.HazeHeight` | Air that catches the light: lit where the sun and the lamps reach it and dark in their shadows, so sunlight through a window or between beams shows as shafts and a spotlight as a beam. `Haze` is how much light it takes in a meter: 0.01 for a hazy day, 0.05 for a dusty hall, 0.2 for smoke; 0 is none. Shafts show where the sun or a lamp is far brighter than the light all around: in a night level lit by its probes, a moon of `Strength` about 2, Blender's 7, shows them, and one of 0.8 is lost in the probes' haze. `HazeColor` tints the light it gives back, white when zero. `HazeHeight` keeps it below a height, thinning out over a meter or a quarter of the height above it, for mist on a lake or smoke in a cellar; 0 is everywhere. Inside a level with light probes it is lit by them, not the sky. Its shadows reach as far as `ShadowDistance`. It covers what is solid, the sky and water, not what is see-through or particles. It costs a walk of 40 steps for every pixel of a picture half the pass's size, and shows a fine grain up close. |
| `Environment.HDRI` | An `HDRI` that lights the world as the place in its panorama would, soft light from all around and reflections in shiny things, in place of `Sky` and `Ground`; with `SkyBackground`, it shows behind everything. |
| `Environment.HDRITurn` | Turns the HDRI's panorama round, in degrees, as `Yaw` turns a model: a larger one turns it left. Its sun, its light and its reflections turn with it. |
| `HDRI` | A panorama of a sky or a place, brighter than white where the sun is, from a Radiance `.hdr` file: the equirectangular kind Poly Haven (CC0) and most HDRI sites give. The middle of the picture faces +Z, as a zero `Rotation` does, and its right half is to the -X side. Its sun, thousands of times brighter than its sky, is taken out of its light as it loads and becomes the pass's sun, which casts the shadows: leave `SunDirection` and `SunColor` at zero to light the world as the photograph was, and set them for a sun of the game's own. Everything else in the picture shows as it is, ground and houses too: GoLib can't hide parts of it yet. |
| `NewHDRI` | `NewHDRI(name string) *HDRI`: the HDRI in the assets folder named `name`, such as `"skies/park.hdr"`, read the first time it is used. Panoramas wider than 2048 pixels are made smaller as they load. |
| `HDRI.Err` | `Err() error`: reads it, and says what stopped it from loading. Using one that didn't load stops `Run` with the same message. |
| `Environment.Exposure` | Brightens or darkens the whole picture: 1 when zero, 2 twice as bright. Under a bright HDRI, below 1. |
| `Environment.Bloom`, `Environment.NoBloom` | What is brighter than white glows, as in a camera's lens: emission, the sun off something shiny, a lamp close to a wall. `Bloom` is how strongly, 0.5 when zero; ordinary lit colors never glow. `NoBloom` turns it off. |
| `Environment.Blur` | Softens the whole 3D picture by about this many pixels, as a camera out of focus: for a menu, a map or a pause screen over a world that goes on behind it, which the 2D drawn after `End3D` stays sharp over. 0, the default, is none; 8 is soft, 32 strong. It comes in steps, each twice the one before, and costs about as little as bloom: the picture is halved a few times and grown back. Where the pass drew nothing, the 2D under it still shows, but that 2D isn't blurred: blur a background picture of the game's own to match. |
| `Environment.NoScreenReflections` | Smooth surfaces, roughness below 0.35, reflect what the pass shows where they can see it on the screen, blurred by their roughness and fading out towards 0.35, and the sky, the `HDRI` or a level's reflection probes elsewhere: a wet street shows its lamps, a polished floor its walls. They are drawn after everything rougher, from a copy of it, so they don't reflect each other, nor what is off the screen or hidden. `NoScreenReflections` turns them off, for a game that needs every frame it can get. |
| `Environment.ShadowDistance`, `Environment.NoShadows` | The sun casts shadows from everything solid onto everything, up to `ShadowDistance` meters from the camera, 50 when zero, sharpest near it. `NoShadows` turns them off. Lamps cast shadows when they ask, with `Light.Shadows`. |
| `Light` | A lamp: `Light.Position`, `Light.Color` (white when zero), `Light.Intensity` (1 when zero: lights what faces it a meter away as brightly as the sun, fading with the square of the distance), `Light.Range` (meters it reaches, 10 when zero). With `Light.Direction`, a spotlight lighting a cone `Light.Cone` degrees across, 45 when zero. `Light.Shadows` makes it cast shadows from everything solid: a spotlight costs one more drawing of the world a frame, a point light six; a pass gives them to the 4 nearest lamps that ask. The lamps of `Model.Lights` ask, as Blender's do. |
| `Screen.AddLight` | `AddLight(light Light)`: adds a lamp to the pass begun last, lighting everything it draws. A pass uses the 8 lamps nearest to the camera. |

```go
// A night, lit by the moon and two torches, with fog that hides the edge of the world.
screen.Begin3D(s.camera)
screen.SetEnvironment(golib.Environment{
	SunColor: golib.Color{R: 120, G: 140, B: 200, A: 255}, SunStrength: 0.3, SunDirection: golib.Vector3{X: -1, Y: -2, Z: 1},
	Sky: golib.Color{R: 20, G: 24, B: 50, A: 255}, Ground: golib.Black, SkyBackground: true,
	FogDistance: 40,
})
for _, torch := range s.torches {
	screen.AddLight(golib.Light{Position: torch, Color: golib.Orange, Intensity: 3, Range: 8})
}
screen.DrawModel(level, golib.Transform{}, golib.White)
screen.End3D()
```

### Particles

Many small things that fly, change and are gone: fire, smoke, sparks, dust, rain, magic. Each is a picture facing the camera, moved by GoLib; all of one `Particles` are one draw.

| Name | What it does |
| --- | --- |
| `Particles` | A set of particles that look and move alike. Make it once, as a package variable or in the scene. |
| `NewParticles` | `NewParticles(spec ParticleSpec) *Particles`: none born yet. |
| `ParticleSpec` | How they look and move; every field left at zero has a working default. |
| `ParticleSpec.Texture` | The picture of each, a white one tinted by `Color`: nil is a soft round dot. |
| `ParticleSpec.Color`, `ParticleSpec.EndColor` | Each particle's color as it is born and as it dies, blended along its life, opacity too: white when zero, and `Color` faded to nothing when `EndColor` is zero. |
| `ParticleSpec.Brightness` | Multiplies the colors, past white for a glow that blooms: 3 for dazzling sparks. 1 when zero. |
| `ParticleSpec.Size`, `ParticleSpec.EndSize` | Meters across as it is born, 0.2 when zero, and as it dies, `Size` when zero: smoke grows, embers shrink. |
| `ParticleSpec.Life` | Seconds each lives, 1 when zero: each between three quarters and five quarters of it. |
| `ParticleSpec.Speed`, `ParticleSpec.Spread` | How fast each leaves, in meters a second, along the direction `Emit` gives, turned by up to `Spread` degrees: 0 straight, 180 every way. |
| `ParticleSpec.Gravity`, `ParticleSpec.Drag` | What pulls them each second, `golib.Vector3{Y: -9.81}` for sparks that fall, `golib.Vector3{Y: 1}` for smoke that rises; and how much of their speed they lose in a second, from 0 to 1. |
| `ParticleSpec.Glow` | Adds their light to what is behind them, as fire, sparks and magic do. Without it they cover what is behind, as smoke and dust do. |
| `Particles.Emit` | `Emit(position, direction Vector3, count int)`: `count` new particles at `position`, leaving along `direction`, up when zero. Past 10,000 alive, the oldest give way. |
| `Particles.Update` | `Update(dt float32)`: moves them on and takes out those whose life is over. Call it in every `Update`. |
| `Particles.Count`, `Particles.Clear` | How many are alive; take them all out at once. |
| `Screen.DrawParticles` | `DrawParticles(p *Particles)`: draws them between `Begin3D` and `End3D`, after everything solid, lit by nothing. They are drawn after the haze and the reflections are made, so neither water nor a polished floor reflects them, and haze doesn't hide them. |

```go
var sparks = golib.NewParticles(golib.ParticleSpec{
	Color: golib.Yellow, EndColor: golib.Red, Brightness: 3, Size: 0.06, Life: 0.8,
	Speed: 4, Spread: 40, Gravity: golib.Vector3{Y: -9.81}, Glow: true,
})

// When a blade hits: sparks.Emit(hit.Position, hit.Normal, 30)
// In Update: sparks.Update(dt)
// In Draw, in the 3D: screen.DrawParticles(sparks)
```

- **A fire is two or three Particles:** flames that glow and rise, smoke that covers and grows, and a few sparks; emit a few flames every update, and a puff of smoke now and then.
- **They don't collide:** sparks fall through the floor. Give them short lives, or a `Life` that ends as they reach it.
- **Random numbers are golib's**, so `golib shot` draws them the same every time.

### Decals

A picture pressed onto whatever is solid inside a box: footprints, a scorch mark, blood, cracks, a bullet hole, a rune glowing on the floor. It follows the surface, a slope or the corner where a floor meets a step.

| Name | What it does |
| --- | --- |
| `Decal` | What is pressed: `Decal.Texture`, the picture, a soft round dot when nil; `Decal.Color`, multiplying it, white when zero, its opacity fading the decal. |
| `Decal.Glow`, `Decal.Brightness` | A decal darkens what it lies on by its picture's colors where it is opaque, as dirt and stains do; `Glow` adds its light instead, its colors times its opacity, `Brightness` times, 1 when zero, past white for a glow that blooms: a white picture with a transparent background and a `Color` glows in that color. |
| `Screen.DrawDecal` | `DrawDecal(decal Decal, transform Transform)`: presses it onto what is solid in the meter box `transform` places, its picture on the box's X and Z, pressed along its Y, its top towards the box's +Z: flat on the ground as it is, on a wall turned with `Pitch` 90. Draw it every frame, as everything else. |

```go
var footprint = golib.NewTexture("textures/footprint.png")

for _, step := range s.steps { // remembered where the player stepped
	screen.DrawDecal(golib.Decal{Texture: footprint, Color: golib.Color{R: 255, G: 255, B: 255, A: step.fade}},
		golib.Transform{Position: step.position, Rotation: golib.Rotation{Yaw: step.yaw}, Scale: golib.Vector3{X: 0.3, Y: 0.3, Z: 0.4}})
}
```

- **Keep the box thin across the way it is pressed:** everything solid in it gets the picture, a character walking through it too. A surface turned away from the way it is pressed fades out, so a decal on the floor doesn't smear down a wall.
- **They are drawn after the solid things and before what is see-through**, from a copy of what was drawn: see-through things and particles are never stained.

### Water

Lakes, seas, rivers and pools: a flat surface whose waves move in the light, that shows what is under it, bent and fading into its color as it deepens, with foam where it meets the shore, and reflects the sun, the level and what moves in it.

| Name | What it does |
| --- | --- |
| `Water` | How the water looks; every field left at zero has a working default. |
| `Water.Color`, `Water.Clarity` | Its color deep down, a blue-green when zero; and how many meters down the bottom still shows, 2 when zero: 0.5 for a muddy river, 6 for a tropical sea. |
| `Water.Waves`, `Water.Still` | How big its waves are, 1 when zero, a lake in a breeze, 3 a rough sea; `Still` makes it a mirror. |
| `Water.NoFoam` | Leaves out the foam where it is shallow: along the shore, around posts and rocks in it. |
| `Screen.DrawWater` | `DrawWater(water Water, transform Transform)`: a square a meter across on X and Z, placed and scaled by `transform`, such as `golib.Transform{Position: golib.Vector3{Y: -0.3}, Scale: golib.Vector3{X: 80, Z: 80}}`. |

```go
screen.DrawModel(island, golib.Transform{}, golib.White)
screen.DrawWater(golib.Water{Clarity: 3}, golib.Transform{Position: golib.Vector3{Y: -0.2}, Scale: golib.Vector3{X: 200, Z: 200}})
```

- **What it reflects:** what is on the screen, the shore, the boats, the player, wherever the reflection meets it there (screen-space reflections); past the screen's edge, a reflection probe's room or the sky. Something off the screen isn't reflected.
- **It is drawn after the solid things and before what is see-through**, from a copy of what was drawn: a see-through thing under it doesn't show through it. Its waves move its light, not its surface: a boat stays level on it.

### Light baked in Blender

A level made in Blender can carry its light baked by Cycles, Blender's ray tracer, into a lightmap: the sky's light as much of it as reaches each corner, and the light the sun, the lamps and the sky bounce off the walls, red beside a red wall, darker under a roof. GoLib can't work that out as it draws; baked, it costs one texture read. The sun and the lamps themselves stay GoLib's, with their shadows, so a character walking through the level is lit and shades it as it moves. Bake with GoLib's script, from the project root, with Blender in the background:

```bash
blender --background games/<game>/sources/level.blend --python tools/blender/bake_lightmaps.py -- --output games/<game>/assets/models/level.glb
```

It writes `level.glb`, and `level.lightmap.hdr`, `level.probes.json` and any reflections beside it. `golib.NewModel("models/level.glb")` reads both, and `DrawModel` draws the level lit by its lightmap; light it with the sun and lamps it was baked with:

| Name | What it does |
| --- | --- |
| `Model.Sun` | `Sun() (ModelSun, bool)`: the sun lamp the model was exported with, and false without one. |
| `ModelSun` | `ModelSun.Direction`, the way its light travels in the model's space; `ModelSun.Color` and `ModelSun.Strength`, as an `Environment` takes them. |
| `Model.Lights` | `Lights() []Light`: the model's point and spot lamps, as `Light`s in its own space, to add to every pass with `AddLight`. |

```go
var level = golib.NewModel("models/level.glb")

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Begin3D(s.camera)
	env := golib.Environment{SkyBackground: true}
	if sun, ok := level.Sun(); ok {
		env.SunDirection, env.SunColor, env.SunStrength = sun.Direction, sun.Color, sun.Strength
	}
	screen.SetEnvironment(env)
	for _, lamp := range level.Lights() {
		screen.AddLight(lamp)
	}
	screen.DrawModel(level, golib.Transform{}, golib.White) // drawn where it was baked
	screen.End3D()
}
```

- **Who exports the level:** the bake script does, with the lightmap's name in it: a level script for a baked level builds the scene and saves the `.blend`, and the bake then writes the `.glb`. A missing `.lightmap.hdr` stops `Run` with what is missing, as any asset does.
- **Markers:** the bake adds an empty `GoLib light probes`, and the reflection boxes are empties too, so `Model.Markers` lists them with the game's: pick the game's by their custom properties, such as `kind`.
- **What is baked:** every visible mesh, but those whose custom property `static` is `false`: the crates, doors and props the game moves.
- **What moves is lit by light probes:** the script also bakes a grid of points over the level, 2 meters apart, each holding the light that reaches it from every way, sky and bounces, as the lightmap does; points inside walls are left out. Everything drawn without a lightmap in a pass with the level, characters, crates, `DrawMesh` shapes, takes its soft light from the probes around it in place of the `Environment`'s `Sky` and `Ground`: darker under a roof, red beside a red wall. Nothing to call: drawing the level is enough, the first one drawn when there are several. Beyond the grid, the `Environment` lights as before, fading in over a probe's spacing. `--probe-spacing 1` for a finer grid, `--no-probes` for none.
- **Reflections of the level:** shiny things reflect the `Environment`'s sky, which looks wrong indoors. Add an empty in Blender, displayed as a cube and scaled over a room, with the custom property `reflection` set to true: the script renders a panorama of the level from its middle, `level.reflection1.hdr` and on, and inside its box a polished floor, a mirror or a chrome ball reflects the room, its walls where they are. Boxes may nest: the smallest a surface is in counts. A pass uses the 4 nearest the camera. What the game moves isn't in them. `--reflection-size 1024` for sharper ones (512 when not given).
- **What isn't:** the sun, point lamps and spot lamps light the level directly in GoLib, so their direct light isn't in the lightmap, only what they bounce; area lamps and glowing materials are baked whole. Light the level with the same `Sun` and `Lights`, or the bounced light won't match. A level drawn moved or turned keeps its lightmap, but its sun and lamps must be placed with it.
- **Lamps in many rooms:** `Model.Lights` gives each lamp a `Range` where it is a hundredth of the sun, about 11 m for a 50 W lamp, and asks for shadows, but a pass gives shadows to the 4 lamps nearest the camera: the others light through walls. In a level of many rooms, shorten their `Range` to their room, or turn the lamps past the nearest few off.
- **The script** unwraps a second UV map, `Lightmap`, for every still object, packed for all of them at one density, and keeps one made by hand. It needs the level's objects to have at most one UV map of their own, and modifiers that copy geometry, such as Array, applied first. `--size 2048` for a large level (1024 when not given), `--samples 512` for less grain, `--gpu` to bake on the graphics card, `--save` to keep the `Lightmap` UV maps in the `.blend`. A courtyard of 7 objects baked at 1024 by 1024 texels and 256 samples in 95 seconds on an Intel iMac's CPU, its 72 light probes in 5 more and its reflection in 13; the script prints each. The lightmap's time grows with the texels and the samples. Bake again whenever the level changes.
- **Units:** the script exports lamps in Blender's own units, Lighting Mode Raw; `Sun` and `Lights` turn them into GoLib's, a sun of strength 3 in Blender being `Strength` 0.95 here. A level exported by hand needs Include > Punctual Lights and Lighting Mode Raw for them.

### The 3D camera

| Name | What it does |
| --- | --- |
| `Camera3D` | The eye the world is seen through. Make one per scene with `NewCamera3D`, and keep it in the scene. |
| `NewCamera3D` | `NewCamera3D(width, height float32) *Camera3D`: a camera for the screen's size, the one in `Config`, 4 meters up and 8 back from 0, 0, 0, looking at it towards +Z, the way the zero `Rotation` faces. |
| `Camera3D.Position`, `Camera3D.Target` | Where the eye is, and the point it looks at. |
| `Camera3D.Orbit` | `Orbit(center Vector3, yaw, pitch, distance float32)`: `distance` meters from `center`, looking at it, facing `yaw` and `pitch` as a `Rotation` does. The yaw a player faces puts the camera behind them; a negative pitch looks down from above. The third-person camera. |
| `Camera3D.Look` | `Look(position Vector3, yaw, pitch float32)`: the eye at `position`, looking `yaw` and `pitch`. The first-person camera. |
| `Camera3D.Follow` | `Follow(target, velocity Vector3, distance, pitch, dt float32)`: behind `target`, `distance` meters away, looking down `pitch` degrees (negative), swinging round behind the way `velocity` moves it along the ground, but not while it moves back towards the camera. The third-person camera; call it in every `Update`. |
| `Camera3D.AvoidWalls` | `AvoidWalls(world *PhysicsWorld)`: keeps anything solid from coming between the camera and `Target`, so a wall or a block never hides what it looks at: it rises over what is in the way, up to looking down from above, and comes in front of it only when rising isn't enough, never nearer than 0.6 meters. Call it after `Follow` or `Orbit`, before `Update`. |
| `Camera3D.Heading` | The yaw `Follow` keeps the camera behind. `Follow` turns it; a game can turn it too, such as with the right stick. |
| `Camera3D.Direction` | `Direction(forward, right float32) Vector3`: the way to move on the ground for keys or a stick pushed `forward` and `right`, from -1 to 1, as the player sees it: forward is away from the camera. As long as the push, at most 1: a stick tilted half way gives half, for a walk rather than a run. A stick's Y grows downwards, so `Direction(-stickY, stickX)`. |
| `Camera3D.FieldOfView` | Degrees seen from the bottom of the screen to the top: 60, the default, is usual; less zooms in. With `Orthographic`, meters seen from bottom to top instead. |
| `Camera3D.Orthographic` | No perspective: far things are as large as near ones, for isometric and top-down games. |
| `Camera3D.Near`, `Camera3D.Far` | The closest and the farthest the camera draws. 0 counts as 0.1 and 1000 meters. |
| `Camera3D.Lag` | Seconds of smoothing towards `Position` and `Target`, as for the 2D camera. 0, the default, follows them exactly. |
| `Camera3D.Update` | `Update(dt float32)`: moves the view by `Lag` and moves the shake on. Call it in every `Update` when the camera has `Lag` or shakes. |
| `Camera3D.Snap` | `Snap()`: the view on `Position` and `Target` at once, without `Lag`. |
| `Camera3D.Shake` | `Shake(strength, seconds float32)`: shakes the view by up to `strength` meters, fading out. Call it from `Update`. |
| `Camera3D.Forward`, `Camera3D.Right` | `Forward() Vector3`: where it looks, tilted as the camera is; `Right() Vector3`: the screen's right, flat on the ground. Both length 1. To walk where the camera looks, flatten `Forward` first: set its `Y` to 0 and `Normalize` it, or keep the yaw given to `Orbit` and use `golib.Rotation{Yaw: yaw}.Forward()`. |
| `Camera3D.ToScreen` | `ToScreen(point Vector3) (Vector2, bool)`: where a point of the world is on the screen, to draw a name or a health bar over it; false when it is behind the camera. |
| `Camera3D.Ray` | `Ray(x, y float32) (origin, direction Vector3)`: the line from the camera through a point of the screen, such as `camera.Ray(input.MousePosition())`, to find what the player clicks on. |

```go
type playScene struct {
	camera         *golib.Camera3D
	ground, sphere *golib.Mesh
	ballAt         golib.Vector3
	yaw            float32 // where the player faces
}

func newPlayScene() *playScene {
	s := &playScene{
		camera: golib.NewCamera3D(screenWidth, screenHeight),
		ground: golib.NewPlaneMesh(),
		sphere: golib.NewSphereMesh(),
	}
	s.camera.Lag = 0.2
	s.camera.Orbit(s.ballAt, s.yaw, -20, 8)
	s.camera.Snap() // ready to draw before the first update
	return s
}

func (s *playScene) Update(input *golib.Input, dt float32) {
	if input.KeyDown(golib.KeyLeft) {
		s.yaw += 120 * dt // a larger yaw turns left
	}
	if input.KeyDown(golib.KeyRight) {
		s.yaw -= 120 * dt
	}
	if input.KeyDown(golib.KeyUp) {
		s.ballAt = s.ballAt.Add(golib.Rotation{Yaw: s.yaw}.Forward().Scale(5 * dt))
	}
	s.camera.Orbit(s.ballAt, s.yaw, -20, 8) // behind the ball, looking down at it
	s.camera.Update(dt)
}

func (s *playScene) Draw(screen *golib.Screen) {
	screen.Clear(golib.SkyBlue)
	screen.Begin3D(s.camera)
	screen.DrawMesh(s.ground, golib.Transform{Scale: golib.Vector3{X: 40, Z: 40}}, golib.DarkGreen)
	screen.DrawMesh(s.sphere, golib.Transform{Position: s.ballAt.Add(golib.Vector3{Y: 0.5})}, golib.Orange)
	screen.End3D()
	screen.DrawText("Arrows to roll", 10, 10, 20, golib.White)
}
```

- **Positions are the rules; the camera is only the view.** Keep the world in `Vector3`s in the rules and in the tests, as in 2D. Many 3D games play on the ground, so their rules can stay in X and Z.
- **Turns, in short:** a zero `Rotation` faces +Z; a larger `Yaw` turns left; seen through a camera facing +Z, the screen's right is -X. Check which way things move with a test that calls `Camera3D.ToScreen`, as [games/marbles](../games/marbles/world_test.go) does, rather than by reasoning about axes.
- **The basic meshes are one meter across, centered.** A box standing on the ground at `p` with height `h` is drawn at `p.Add(golib.Vector3{Y: h / 2})` with `Scale` Y `h`.
- **Two surfaces at the same height flicker through each other**, such as a plane laid on a box's top. Lift the one on top by a centimeter, or make floors from thin boxes.
- **Check what the camera sees with `golib shot`.** A camera looking the wrong way shows only the background color. While testing, `screen.DrawAxes(golib.Vector3{}, 1)` and `screen.DrawGrid(20, 1, golib.Gray)` show where the world is and which way is which.
- With the zero `Environment`, a sun high behind and to the right of a camera facing +Z, as `NewCamera3D`'s and a `Follow` camera's do, and a soft light from the sky: a color facing the sun shows as it is, and a side facing away from it at about half its brightness.
- The 3D smooths its edges as it goes over the 2D (FXAA). `Config.PixelArt` draws it at the screen's size, scaled up with its edges sharp, the look of the first 3D consoles; `Config.Antialias` draws it twice as large and shrinks it, smoother still. Post-processing shaders run over it as over the 2D.
- `Update`, `Snap`, `Orbit`, `Look`, `ToScreen` and `Ray` work without a window, so tests can check what the camera sees.

## Scenes

A scene is any value that implements `Game`. Make each screen a scene (title, play, pause, game over), pass the first one to `Run`, and switch between them from `Update`.

| Name | What it does |
| --- | --- |
| `SwitchScene` | `SwitchScene(next Game)`: from the next update, `Run` updates and draws `next` instead. The old scene keeps its state, so a pause scene can switch back to the game it paused. If one update calls it more than once, the last call wins; `nil` stops `Run` with an error. |

```go
func (s *playScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(&pauseScene{paused: s})
		return
	}
	// ... play
}

// pauseScene freezes a play scene and shows a message over it.
type pauseScene struct {
	paused *playScene
}

func (s *pauseScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyEscape) || input.GamepadPressed(0, golib.GamepadStart) {
		golib.SwitchScene(s.paused) // carries on where it stopped
	}
}

func (s *pauseScene) Draw(screen *golib.Screen) {
	s.paused.Draw(screen) // the frozen game under the message
	screen.DrawText("Paused", 40, 40, 40, golib.White)
}
```

`golib.SwitchScene(newPlayScene())` starts a fresh game. Settings every scene shares, such as whether the music is on, go in a struct that each scene holds a pointer to, like `options` in `games/asteroids/scenes.go`. The whole pattern, with a title menu, is in `games/platformer/scenes.go`.

## Random numbers

| Name | What it does |
| --- | --- |
| `RandomInt` | `RandomInt(low, high int) int`: a whole number from low to high, both included. |
| `RandomFloat` | `RandomFloat(low, high float32) float32`: a number from low up to, but not including, high. |
| `SetRandomSeed` | `SetRandomSeed(seed uint64)`: restarts both; the same seed gives the same numbers in the same order. Call it at the start of tests that use random numbers, or for a "random" level that is the same every time. |

The numbers differ on every run, except under `golib shot`, which starts them from the same seed. A one-in-four chance is `golib.RandomFloat(0, 1) < 0.25`.

## Sound effects

GoLib makes sound effects in code from a few numbers, so a game needs no sound files. It also makes them from `.jfxr` files, the small files of settings that the sound effect maker jfxr saves ([below](#sound-effects-from-jfxr)), and it plays sound files.

| Name | What it does |
| --- | --- |
| `NewSound` | `NewSound(spec SoundSpec) *Sound`: the sound the recipe describes. |
| `NewSoundFile` | `NewSoundFile(name string) *Sound`: the `.wav`, `.ogg`, `.mp3`, `.qoa` or `.jfxr` file `name` in the game's assets folder, with forward slashes, as in `ReadAsset`. |
| `NewSoundJfxr` | `NewSoundJfxr(settings string) *Sound`: the sound a `.jfxr` file with these contents gives, with no file: jfxr settings as JSON, written in the code or worked out as the game runs ([below](#sound-effects-from-jfxr)). |
| `Sound` | A sound effect, ready to play. |
| `Sound.Play` | `Play()`: plays the sound, over any copy of it that is still playing; up to four copies at once, and a fifth cuts off the oldest. |
| `Sound.PlayWith` | `PlayWith(volume, pitch float32)`: plays it like `Play`, but louder, quieter, higher or lower, so the same sound over and over doesn't tire the ear. `volume` is 0 to 1, under the sound's own `SetVolume`; `pitch` is 1 for the sound as it is, 2 an octave up, 0.5 an octave down, from 0.25 to 4. Both are kept inside their limits, and the next `Play` sounds as it always did. |
| `Sound.Loop` | `Loop()`: plays the sound over and over, with no gap, until `Stop`: an engine, an alarm, rain. Calling it while it loops changes nothing, so a game can call it in every update. |
| `Sound.PlayAt` | `PlayAt(position Vector3, camera *Camera3D)`: in a 3D game, plays it as if it came from `position`, heard from what `camera` looks at: as loud as `Play` within 2 meters, quieter farther away, silent past 50, and from the left or the right as the camera turns. |
| `Sound.LoopAt` | `LoopAt(position Vector3, camera *Camera3D)`: loops it as `Loop` does, from `position`, as `PlayAt` plays it: a waterfall, an engine. Call it in every update, so it follows the thing and the camera; `Loop` puts it back in the middle. |
| `Sound.Stop` | `Stop()`: silences the sound, its loop and every copy `Play` started. |
| `Sound.Looping` | `Looping() bool`: `Loop` was called, and `Stop` wasn't since. True in tests and shots too, where nothing is heard. |
| `Sound.SetVolume` | `SetVolume(volume float32)`: how loud this sound is, from 0 to 1, under `SetVolume`. Use it to even out sound files. |
| `Sound.Unload` | `Unload()`: stops the sound and frees its memory. Only for sounds made as the game runs, such as one for each set of settings with `NewSoundJfxr`, so they don't pile up; `Run` frees the rest when it ends. An unloaded sound can still play: it is made again then. |
| `Laser` | A falling zap, for shots. |
| `Explosion` | A low burst of noise, for things breaking apart. |
| `Pickup` | A bright blip that rises, for coins. |
| `Jump` | A soft rising note, for jumps. |
| `Hurt` | A harsh falling note, for taking damage. |
| `PowerUp` | A rising fanfare, for upgrades, extra lives and cleared levels. |
| `SetVolume` | `SetVolume(volume float32)`: how loud all sound and music is, from 0 to 1. It works before `Run` too. |

The six recipes return a `*Sound`, like `NewSound` and `NewSoundFile`. Keep every sound of a game in one file:

```go
package main

import "golib"

// The game's sounds. The rules in world.go play them.
var (
	jumpSound = golib.Jump()
	coinSound = golib.Pickup()

	// shotSound is a short zap whose pitch falls.
	shotSound = golib.NewSound(golib.SoundSpec{
		Wave: golib.WaveSquare, Frequency: 1200, Slide: -3000, Duration: 0.12,
		Attack: 0.001, Release: 0.09, Volume: 0.22, Duty: 0.2,
	})

	doorSound = golib.NewSoundFile("sounds/door.ogg") // games/<game>/assets/sounds/door.ogg
	hitSound  = golib.NewSoundFile("sounds/hit.jfxr") // designed in jfxr
)

func init() {
	doorSound.SetVolume(0.6) // the file is louder than the rest
}
```

and play them where things happen: `shotSound.Play()`. A sound that lasts as long as something happens loops:

```go
// In Update: the engine hums while the ship thrusts.
if s.ship.thrusting {
	engineSound.Loop()
} else {
	engineSound.Stop()
}
```

For a loop without a click where it starts again, make the sound's end meet its start: in code, give it no fades, with a negative `Attack` and `Release`, no `Slide`, and a `Duration` of whole waves, such as 0.25 seconds at 440 Hz; in a file, cut it at a quiet point. Stop looping sounds when the scene that plays them ends, such as in the code that switches to the pause scene.

`SoundSpec` is the recipe. Every field left at zero gets its default, so `SoundSpec{}` is a short beep.

| Field | Default | Meaning |
| --- | --- | --- |
| `SoundSpec.Wave` | `WaveSquare` | The sound's character (below). |
| `SoundSpec.Frequency` | 440 | Pitch the sound starts at, in Hz. |
| `SoundSpec.Slide` | 0 | Hz added every second: negative falls (a shot), positive rises (a coin). |
| `SoundSpec.Duration` | 0.25 | Seconds; at most 10. |
| `SoundSpec.Attack` | 0.005 | Seconds fading in, so the start doesn't click. Negative for none, for a loop. |
| `SoundSpec.Release` | 0.05 | Seconds fading out at the end. Negative for none, for a loop. |
| `SoundSpec.Volume` | 0.5 | From 0 to 1. |
| `SoundSpec.Duty` | 0.5 | Square waves only: the part of each wave that is high, from 0.05 to 0.95. 0.5 is even, 0.2 thin and nasal. |
| `SoundSpec.Vibrato` | 0 | Hz the pitch wobbles up and down. |
| `SoundSpec.VibratoRate` | 12 | Wobbles per second. |

| Waveform | Sounds |
| --- | --- |
| `WaveSquare` | Buzzy, like an 8-bit console |
| `WaveSaw` | Harsh and bright |
| `WaveTriangle` | Softer, a bit like a flute |
| `WaveSine` | Pure and smooth |
| `WaveNoise` | Hiss, for explosions and hits; the frequency sets how coarse |

`Waveform` is the type of these constants.

- A sound file is kept whole in memory: use `NewMusic` for long tracks. It is read the first time the sound plays, even under `golib shot` and in tests, which play nothing but stop at a file that is missing or can't be read.
- A game with sound files has an `assets/` folder, so it needs `assets.go` (see [Files](#files-the-assets-folder)). Only use sounds the user provides, and write where they came from, and their license, in `assets/ATTRIBUTION.md`. Sounds designed in jfxr for the game need no entry there.
- Make sounds once, as package variables: every sound stays in memory until `Run` ends. A game that makes new ones as it runs, such as from settings it works out with `NewSoundJfxr`, keeps the ones it plays again and calls `Sound.Unload` on the rest.

### Sound effects from jfxr

[jfxr](https://jfxr.frozenfractal.com) is a sound effect maker that runs in the browser, with nothing to install: presets such as "Pickup/coin" and "Explosion", ten waveforms, pitch jumps and filters, each on a slider. Its "Save current sound to file" button saves a `.jfxr` file, a small JSON file of those settings, and "Load sound from file" opens it again. Put the file in the game's `assets/` folder and play it with `NewSoundFile`: GoLib makes the sound from the settings the first time it plays, with the same samples as the WAV file jfxr exports for it.

```go
var coinSound = golib.NewSoundFile("sounds/coin.jfxr") // games/<game>/assets/sounds/coin.jfxr
```

A `.jfxr` file is plain JSON, so it can also be written by hand. Settings left out keep jfxr's defaults. The same settings can live in the code, with no file, through `NewSoundJfxr`, which gives the same sound; a game that works its sounds out as it runs, such as a synthesizer played by the player, builds the JSON and makes a sound from it. This one is a short rising jump:

```json
{
  "_version": 1,
  "waveform": "square",
  "sustain": 0.08,
  "decay": 0.12,
  "frequency": 300,
  "frequencySweep": 700,
  "squareDuty": 30,
  "amplification": 40
}
```

The same jump in the code, with no file:

```go
var jumpSound = golib.NewSoundJfxr(`{"waveform": "square", "sustain": 0.08, "decay": 0.12,
	"frequency": 300, "frequencySweep": 700, "squareDuty": 30, "amplification": 40}`)
```

The settings, with the labels jfxr shows for them. "Over the sound" means from its start to its end; pitch and duty sweeps start over at each repeat.

| Setting | jfxr's label | Default | Range | Meaning |
| --- | --- | --- | --- | --- |
| `waveform` | Waveform | `"sine"` | `"sine"`, `"triangle"`, `"sawtooth"`, `"square"`, `"tangent"`, `"whistle"`, `"breaker"`, `"whitenoise"`, `"pinknoise"`, `"brownnoise"` | The sound's character |
| `attack` | Attack | 0 | 0 to 5 | Seconds from silence to full volume |
| `sustain` | Sustain | 0 | 0 to 5 | Seconds at full volume |
| `sustainPunch` | Sustain punch | 0 | 0 to 100 | Percent louder at the start of the sustain, fading to full volume |
| `decay` | Decay | 0 | 0 to 5 | Seconds from full volume to silence |
| `tremoloDepth` | Tremolo depth | 0 | 0 to 100 | Percent the volume dips by, and back |
| `tremoloFrequency` | Tremolo frequency | 10 | 0 to 1000 | Volume dips per second |
| `frequency` | Frequency | 500 | 10 to 10000 | Hz the pitch starts at |
| `frequencySweep` | Frequency sweep | 0 | -10000 to 10000 | Hz added to the pitch over the sound, evenly |
| `frequencyDeltaSweep` | Freq. delta sweep | 0 | -10000 to 10000 | Hz added to the pitch over the sound, slowly at first |
| `repeatFrequency` | Repeat frequency | 0 | 0 to 100 | Times per second the pitch starts over, with its sweeps and jumps; 0 is once |
| `frequencyJump1Onset` | Freq. jump 1 onset | 33 | 0 to 100 | Percent of the way through each repeat where the pitch jumps |
| `frequencyJump1Amount` | Freq. jump 1 amount | 0 | -100 to 100 | Percent the pitch jumps by |
| `frequencyJump2Onset` | Freq. jump 2 onset | 66 | 0 to 100 | Where a second jump happens |
| `frequencyJump2Amount` | Freq. jump 2 amount | 0 | -100 to 100 | Percent the second jump changes the pitch by |
| `harmonics` | Harmonics | 0 | 0 to 5 | Copies of the sound at 2, 3, 4… times the pitch, mixed in |
| `harmonicsFalloff` | Harmonics falloff | 0.5 | 0 to 1 | Volume of each harmonic, as a fraction of the one before |
| `interpolateNoise` | Interpolate noise | `true` | `true`, `false` | Noise waveforms only: smoother noise |
| `vibratoDepth` | Vibrato depth | 0 | 0 to 1000 | Hz the pitch dips by, and back |
| `vibratoFrequency` | Vibrato frequency | 10 | 0 to 1000 | Pitch dips per second |
| `squareDuty` | Square duty | 50 | 0 to 100 | Square waves only: percent of each wave that is high; 50 is even, 20 thin and nasal |
| `squareDutySweep` | Square duty sweep | 0 | -100 to 100 | Added to the duty over the sound |
| `flangerOffset` | Flanger offset | 0 | 0 to 50 | Milliseconds a copy of the sound is delayed by before it is mixed in: a hollow, swooshing sound |
| `flangerOffsetSweep` | Flanger offset sweep | 0 | -50 to 50 | Milliseconds added to that delay over the sound |
| `bitCrush` | Bit crush | 16 | 1 to 16 | Bits per sample: fewer is crunchier |
| `bitCrushSweep` | Bit crush sweep | 0 | -16 to 16 | Bits added over the sound |
| `lowPassCutoff` | Low-pass cutoff | 22050 | 0 to 22050 | Hz above which frequencies are taken out: lower is duller |
| `lowPassCutoffSweep` | Low-pass sweep | 0 | -22050 to 22050 | Hz added to that cutoff over the sound |
| `highPassCutoff` | High-pass cutoff | 0 | 0 to 22050 | Hz below which frequencies are taken out: higher is thinner |
| `highPassCutoffSweep` | High-pass sweep | 0 | -22050 to 22050 | Hz added to that cutoff over the sound |
| `compression` | Compression | 1 | 0 to 5 | Power each sample is raised to: below 1 makes the quiet parts louder |
| `normalization` | Normalization | `true` | `true`, `false` | Scales the sound so its loudest point is full volume |
| `amplification` | Amplification | 100 | 0 to 500 | Percent of that volume; above 100 distorts |
| `sampleRate` | Sample rate | 44100 | 44100 | Always 44100 |

- The sound lasts `attack` + `sustain` + `decay` seconds, 15 at most. A sound whose three are all 0 stops `Run` with an error.
- jfxr makes every sound as loud as it gets, since `normalization` is on: a jfxr sound is louder than the recipes. Lower `amplification`, to about 30 or 40, or use `Sound.SetVolume`.
- Numbers out of range are kept in range, as jfxr keeps them. A setting jfxr doesn't have, a value of the wrong kind, a waveform jfxr doesn't have, or a `_version` above 1, in a file or in the settings given to `NewSoundJfxr`, stops `Run` with an error. The `_version`, `_name` and `_locked` that jfxr writes are allowed.
- GoLib's framework carries jfxr's synthesizer, rewritten in Go, under jfxr's BSD license, so `golib dist` adds jfxr's license to every game's `THIRD-PARTY-LICENSES.txt`. The sounds made with jfxr belong to whoever made them.

## Music

| Name | What it does |
| --- | --- |
| `NewMusic` | `NewMusic(name string) *Music`: the music file `name` in the game's assets folder, with forward slashes, as in `ReadAsset`. The file is read the first time the music plays. |
| `Music` | Music streamed a little at a time while the game runs. It loops until it is stopped. |
| `Music.Play` | `Play()`: starts the music, or carries on after `Pause`. Safe to call in every update. |
| `Music.Pause` | `Pause()`: holds it where it is. |
| `Music.Stop` | `Stop()`: ends it; the next `Play` starts from the beginning. |
| `Music.Playing` | `Playing() bool`: it is playing now, not paused, stopped or waiting for a sound device. |
| `Music.SetVolume` | `SetVolume(volume float32)`: how loud this music is, from 0 to 1, under `SetVolume`. Around 0.2 to 0.4 keeps it under the sound effects. |
| `Music.Preload` | `Preload()`: makes the tune, or reads the file, now rather than the first time it plays, which would hold the game up for a moment then. Call it on a title or loading screen, or in `main` before `Run`. A mistake it finds stops `Run`, as playing would. |
| `Music.Err` | `Err() error`: what stopped the music from playing, or nil. For a tune it makes the tune, so a test can check its notes; for a file it tells only what playing it has found so far. |

```go
var theme = golib.NewMusic("music/theme.xm") // games/<game>/assets/music/theme.xm

func (s *playScene) Update(input *golib.Input, dt float32) {
	if input.KeyPressed(golib.KeyM) {
		s.options.music = !s.options.music
	}
	if s.options.music {
		theme.Play()
	} else {
		theme.Pause()
	}
	// ... play
}
```

- Formats: `.ogg`, `.mp3`, `.wav`, `.qoa`, and the tracker modules `.xm` and `.mod`, which are a few dozen kilobytes. Not `.it`. The first `Play` stops `Run` with an error if the file is missing or in another format. `golib shot` and tests play nothing, so they don't notice: start the game with `golib run` to check.
- Not every machine plays every format: a browser reads `.ogg`, `.mp3` and `.wav` and nothing else. Where the file's format doesn't play, GoLib plays a file beside it with the same name in one that does, so `music/theme.ogg` next to `music/theme.xm` is what a web build plays, and the game's code says `music/theme.xm` everywhere. With no such file, the game runs without that music and GoLib says so on the console, instead of stopping a game that plays on the desktop. `golib web` and `golib dist --web` say which file the browser will play while they build.
- A module and the file beside it are **one tune, not two**, and the module is the one to name: it is a few dozen kilobytes against a few megabytes, and every machine but a browser plays it. A game that picks its music by listing the folder, rather than naming a file, sees both and has to say so: skip a playable file when a module of the same name sits beside it, as `games/skyraid/music.go` does.
- A game with music has an `assets/` folder, so it needs `assets.go` (see [Files](#files-the-assets-folder)).
- Only use music the user provides, and write where it came from, and its license, in `assets/ATTRIBUTION.md`, as `games/asteroids` does. `golib dist` copies that file into the `THIRD-PARTY-LICENSES.txt` it puts next to the game.
- There are no crossfades or playlists: `Stop` one `Music` and `Play` another.

Music can also be made from notes, with no file at all, for a game whose user has no music to give:

| Name | What it does |
| --- | --- |
| `NewTune` | `NewTune(spec TuneSpec) *Music`: music made from notes. It plays like any other `Music`, and loops. The tune is made the first time it plays. |
| `TuneSpec` | The recipe: the speed, and the voices that play together. |
| `TuneSpec.Tempo` | Beats per minute, from 20 to 400. A beat is what a note with no length lasts, so a tune written in eighth notes at 110 beats per minute has a `Tempo` of 220. Default: 120. |
| `TuneSpec.Voices` | The lines that play together, at most 8. |
| `TuneSpec.Reverb` | The room the tune plays in, from 0 to 1: 0.2 a small room, 0.5 a hall, 1 a cave. Its echoes soften notes made in code, which sound dry on their own, and the echoes at the end carry over into the start, so the loop has no seam. Default: 0, no room. |
| `Voice` | One line of the tune. |
| `Voice.Wave` | Its sound, as in `SoundSpec`. Default: `WaveSquare`. |
| `Voice.Volume` | How loud it is, from 0 to 1, before the music's own volume. Default: 0.5. |
| `Voice.Duty` | Shapes a square wave, from 0.05 to 0.95, as in `SoundSpec`. Default: 0.5. |
| `Voice.Vibrato`, `Voice.VibratoRate` | Wobble the pitch, as in `SoundSpec`. Default: no wobble. |
| `Voice.Gap` | The silence at the end of every note, in seconds. Default: 0.04. 0 runs notes together; more makes a short, clipped sound, such as a drum. |
| `Voice.Attack` | Seconds each note takes to swell to its full loudness, as a bowed string or a pad does: 0.1 to 0.5. Default: 0.005, at once. |
| `Voice.Decay` | Each note fades away as a plucked or struck string does: `Decay` seconds after it starts it is at about a third of its loudness. 0.1 to 0.3 for a marimba or a pluck, 1 to 2 for a piano or a bell. Default: 0, a note holds its loudness. |
| `Voice.Ring` | Seconds each note goes on sounding after it ends, fading out over the notes that follow, as a piano does with its pedal down, up to 10. The last notes ring on over the start of the loop. Default: 0, a note ends with its length. |
| `Voice.LowPass` | Softens the voice above this many Hz, from 20 to 20000: around 3000 takes the edge off a square or saw wave, 800 muffles it. Default: 0, no filter. |
| `Voice.Detune` | A second copy of the wave this many cents higher, up to 100, beating against the first for a fuller, warmer sound, like a chorus: 5 to 15 is subtle. Not for `WaveNoise`. Default: 0, one wave. |
| `Voice.Notes` | The notes, separated by spaces. |

A note is a letter from `a` to `g`, an optional `#` or `b`, and its octave: `c4` is middle C, `f#3` and `eb5`; capitals work too, and octaves go from 0 to 8. A dot is a silence, and a dash holds the note before it for another beat. `/` and a number give a length in beats: `c4/2` lasts two beats, `c4/0.5` half a beat. A tune lasts at most two minutes, and loops.

```go
// Sixteen beats a voice, so every voice loops in step.
var themeSpec = golib.TuneSpec{
	Tempo:  264, // the tune is written in eighth notes, at 132 a minute
	Reverb: 0.3, // a room, so the notes don't sound dry
	Voices: []golib.Voice{
		{Wave: golib.WaveTriangle, Decay: 0.6, Ring: 0.3, Notes: "c5 - g4 . a4 g4 e4 c4 d4/2 g4/2 c5/4"},                // the melody, plucked
		{Wave: golib.WaveTriangle, Volume: 0.35, LowPass: 600, Notes: "c3/2 g3/2 c3/2 g3/2 f3/2 c4/2 g3/2 g3/2"},          // the bass
		{Wave: golib.WaveNoise, Volume: 0.05, Gap: 0.1, Decay: 0.03, Notes: ". a7 . a7 . a7 . a7 . a7 . a7 . a7 . a7"}, // a tick on the off-beats
	},
}

var theme = golib.NewTune(themeSpec)
```

A voice with only a wave and its notes sounds like an 8-bit console: every note starts at once, holds its loudness and stops. For anything softer, give each voice an instrument's settings, and the tune a `Reverb` from 0.2 to 0.4. These are starting points, to tune by ear:

| Sounds like | Voice settings |
| --- | --- |
| A marimba or a kalimba, for arpeggios | `Wave: golib.WaveSine, Decay: 0.25, Ring: 0.3` |
| A plucked string or a harp, for a melody | `Wave: golib.WaveTriangle, Decay: 0.6, Ring: 0.4, Detune: 5` |
| An electric piano | `Wave: golib.WaveSine, Decay: 1.2, Ring: 0.5, Detune: 7` |
| A soft lead | `Wave: golib.WaveSquare, Duty: 0.3, LowPass: 2500, Vibrato: 3, VibratoRate: 5` |
| A warm pad, for long chords | `Wave: golib.WaveSaw, Attack: 0.4, LowPass: 1200, Detune: 10, Gap: 0` |
| A round bass | `Wave: golib.WaveTriangle, LowPass: 500, Decay: 1.5` |
| A drum's thud | `Wave: golib.WaveNoise, LowPass: 400, Decay: 0.08, Gap: 0.1` |
| A snare or a clap | `Wave: golib.WaveNoise, LowPass: 5000, Decay: 0.05, Gap: 0.1` |

A tune with none of these settings sounds as it did before they existed.

- A mistake in the notes, or a tune longer than two minutes, stops `Run` with a message, under `golib shot` too, where nothing can be heard. In a test, where nothing calls `Run`, `Music.Err` returns it: keep the `TuneSpec` in a variable of its own and test the tune made from it.
- Voices play together from the first beat; the longest one sets the tune's length, and the others end in silence. Make them add up to the same number of beats, or the loop falls out of step, and count those beats in a test.
- The tune is made the first time it plays, which takes a moment, longer with `Ring` and `Reverb`: about a third of a second for a minute and a half of tune on a desktop, and up to a second in a browser. Call `Music.Preload` in `main` before `Run`, or on a title screen, so its first `Play` doesn't hold the game up.
- Made music is a last resort: a tracker module or an OGG file from the user sounds better. Say so, and how to swap it in: only the `golib.NewTune` line changes.

## Window, fullscreen and screen effects

| Name | What it does |
| --- | --- |
| `SetFullscreen` | `SetFullscreen(on bool)`: fullscreen or a window, from the next frame. Fullscreen covers the monitor without changing its resolution, and the screen keeps its size, unless `Config.WindowScale` makes it the monitor's size divided by the scale or `Config.FillWindow` gives it the monitor's shape. On macOS it is the system's own fullscreen, the one the window's green button enters, which slides into a space of its own without the menu bar or the Dock; the switch waits until no key or mouse button is held, because macOS loses a release that comes while the window slides. Call it from `Update`; to start in fullscreen, set `Config.Fullscreen`, because `Run` replaces an earlier call with it. |
| `IsFullscreen` | `IsFullscreen() bool`: the game is in fullscreen, or will be from the next frame. It turns false by itself when the player leaves fullscreen on their own: with Esc in a browser, or the green button on macOS. |
| `WindowFocused` | `WindowFocused() bool`: the window has the player's attention. False while they work in another program, so a game can draw a sign over itself or quieten its music; with `Config.PauseUnfocused`, `Run` stops updating the game meanwhile and keeps drawing it. Always true under `golib shot` and in tests. |

```go
altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) && input.KeyPressed(golib.KeyEnter)
if input.KeyPressed(golib.KeyF11) || altEnter {
	golib.SetFullscreen(!golib.IsFullscreen())
}
```

For a game's graphics settings, the window's monitor, size and frame rate, and a link to open, such as a contact in an About screen. Like `SetFullscreen`, what a game asks for is applied at the start of the next frame. `SetMonitor` and `SetWindowSize` called before `Run` apply from the first frame, so a game that kept the player's window opens it as they left it (see `WindowSize`).

| Name | What it does |
| --- | --- |
| `Monitor` | A monitor connected to the machine. |
| `Monitor.Name` | Its name, as the system gives it; "" in a browser. |
| `Monitor.Width`, `Monitor.Height` | Its size, in pixels. |
| `Monitor.RefreshRate` | How many times a second it refreshes; 0 where the system doesn't say, such as in a browser. |
| `Monitors` | `Monitors() []Monitor`: the monitors connected, as `Run` last looked, once a second. One in a browser, the screen the page is on; none under `golib shot` and in tests. |
| `CurrentMonitor` | `CurrentMonitor() int`: the monitor the window is on, as an index into `Monitors`; 0 with one monitor, in a browser, and where there is none. |
| `DisplayScale` | `DisplayScale() float32`: how many of the screen's own pixels, across, one unit of `Monitors`' sizes and of `SetWindowSize` is: 2 on a Mac's Retina screen, where macOS measures in points, 1 on other Macs, on Windows and on Linux, and the browser's `devicePixelRatio` in a browser. Multiply by it to show a window's size in the screen's pixels. 1 under `golib shot` and in tests. |
| `SetMonitor` | `SetMonitor(i int)`: moves the window to monitor `i`: in a window, centered on it at its size; in fullscreen, covering it. On macOS, a game in fullscreen leaves it, moves, and enters it again. An index no monitor has is ignored, and so is the call in a browser. |
| `SetWindowSize` | `SetWindowSize(width, height int)`: makes the window's drawing area that many pixels, centered on its monitor and no larger than it; in fullscreen, the size it comes back to. The screen keeps its size, and `Run` scales it to fit, unless `Config.WindowScale` makes it this size divided by the scale (the way to offer resolutions) or `Config.FillWindow` gives it the window's shape. The system may make the window smaller, to fit it between its menu bar, taskbar or Dock and the title bar (a 1440-high window on a 2560 by 1440 Mac gets 1296), and with `PixelArt` on a screen of fixed size the screen then drops a whole size, with black borders: offer sizes up to about four fifths of the monitor, as `Run`'s first window is. Ignored in a browser, where the canvas follows the page. |
| `WindowSize` | `WindowSize() (width, height int)`: the window's drawing area, in the units `SetWindowSize` takes, as `Run` saw it at the start of the last frame. In fullscreen, the size the window had before, which it comes back to; a minimized window keeps the size it had. 0, 0 before `Run` opens the window and under `golib shot`; it keeps its last value after `Run` returns, for a game to save the window when it closes. The canvas's size in a browser. |
| `SetFrameRate` | `SetFrameRate(fps int)`: draws that many frames a second at most, from 15 to 360; a game starts at 60. Updates stay at 60 a second of game time: at 30 every frame runs two, above 60 some frames run none and draw the same state again. Each frame waits for the monitor to refresh (V-Sync), so the picture never tears, and a cap above the monitor's refresh rate draws no faster than it. |
| `FPS` | `FPS() int`: the frames `Run` drew in the last second, for a game to show. 0 under `golib shot`, in tests and in the game's first second. |
| `OpenURL` | `OpenURL(url string) error`: opens a web page, in the browser, or a mail to write, for a `mailto:` link. Only links that start with `https://`, `http://` or `mailto:`, with no spaces or quotes, open; any other is an error, so a game can't run anything else by mistake. Nothing opens under `golib shot` and in tests. |

```go
// In a settings scene's Update: the next monitor, a window of 1280 by 720,
// and 30 frames a second.
golib.SetMonitor((golib.CurrentMonitor() + 1) % max(1, len(golib.Monitors())))
golib.SetWindowSize(1280, 720)
golib.SetFrameRate(30)
```

```go
// Opening the window as the player left it, and keeping it when it closes.
type window struct {
	Width, Height, Monitor int
	Fullscreen             bool
}

func main() {
	config := golib.Config{Title: "My game", FillWindow: true}
	var w window
	if found, _ := golib.LoadData("window", &w); found {
		golib.SetMonitor(w.Monitor)
		golib.SetWindowSize(w.Width, w.Height)
		config.Fullscreen = w.Fullscreen
	}
	err := golib.Run(newGame(), config)
	if width, height := golib.WindowSize(); width > 0 {
		golib.SaveData("window", window{width, height, golib.CurrentMonitor(), golib.IsFullscreen()})
	}
	if err != nil {
		log.Fatal(err)
	}
}
```

Screen effects, such as scanlines, a glow or a color grade, are post-processing shaders: GLSL 330 fragment shaders that run over the whole picture after `Draw`. Screenshots from `golib shot` include them.

| Name | What it does |
| --- | --- |
| `Shader` | A post-processing effect. |
| `NewShader` | `NewShader(source string) *Shader`: the effect with this fragment shader source. It compiles the first time it is used; if it doesn't, `Run` stops with an error, and raylib's warnings above it give the line and the reason. |
| `Shader.SetUniform` | `SetUniform(name string, values ...float32)`: sets a uniform the shader declares: 1 value for a `float`, 2 for a `vec2`, 3 for a `vec3`, 4 for a `vec4`. The value stays until set again. Names the shader doesn't use are ignored. |
| `SetPostProcess` | `SetPostProcess(shaders ...*Shader)`: runs the shaders in order, each on the result of the one before. With no shaders, turns effects off. Call it before `Run` or from `Update`. |

A shader reads the picture from `texture0` at `fragTexCoord`, which goes from 0, 0 at the bottom-left corner to 1, 1 at the top-right one (y grows upwards, unlike on `Screen`), and writes `finalColor`. GoLib also sets these uniforms, when the shader declares them: `float time` (seconds of game time), `vec2 screenSize` (`Config.Width`, `Config.Height`) and `vec2 outputSize` (the pixels this shader draws; for the last shader, the part of the window the game fills, for effects that follow real pixels, like scanlines).

`shaders/gray.fs` in the game's folder:

```glsl
#version 330

in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
uniform float amount; // set by the game: 0 is full color, 1 is gray
out vec4 finalColor;

void main()
{
    vec4 color = texture(texture0, fragTexCoord);
    float gray = dot(color.rgb, vec3(0.299, 0.587, 0.114));
    finalColor = vec4(mix(color.rgb, vec3(gray), amount), color.a);
}
```

and the Go side, in `main.go`:

```go
import (
	_ "embed"
	"log"

	"golib"
)

//go:embed shaders/gray.fs
var graySource string

func main() {
	gray := golib.NewShader(graySource)
	gray.SetUniform("amount", 0.8)
	golib.SetPostProcess(gray)
	if err := golib.Run(newTitleScene(), golib.Config{Title: "Gray"}); err != nil {
		log.Fatal(err)
	}
}
```

Let the player turn effects off, with `golib.SetPostProcess()`. `games/asteroids` has a glow and a CRT shader, switched with F2.

## The player's language

A game translated to several languages can start in the one the player's system speaks, until the player chooses another in its settings. GoLib says which languages those are; the text itself, and how to translate it, is the game's.

| Name | What it does |
| --- | --- |
| `SystemLanguages` | `SystemLanguages() []string`: the languages the system shows its own text in, most preferred first, as tags: the language in lowercase, then its script and region when the system says them, such as `"es-ES"`, `"en"`, `"pt-BR"` or `"zh-Hans-CN"`. Windows's display languages, macOS's preferred languages, Linux's `LANGUAGE` and then `LC_ALL`, `LC_MESSAGES` or `LANG`, a browser's `navigator.languages`. Empty when the system says none, and under `golib shot`. |
| `ChooseLanguage` | `ChooseLanguage(have []string, fallback string) string`: which of the game's languages, by their tags, to start in: the first of the system's it has, whole or by its language alone, so `"es-MX"` chooses `"es"`; or `fallback` with none. |

```go
func main() {
	settings := loadSettings()
	if settings.Language == "" { // the player hasn't chosen one yet
		settings.Language = golib.ChooseLanguage([]string{"es", "en"}, "en")
	}
	// ...
}
```

- Both work before `Run`, so the first screen is already in the right language. Save the language only once the player chooses one, so a game keeps following the system until then.
- `golib shot` sees no system language, so its pictures are the same on every machine: shots show the game's fallback. Start a shot in another language with `--save` and the game's saved settings.
- The built-in font draws the Latin alphabet: a language in another script needs a font from a file (see [Fonts](#fonts)).

## The game's version

| Name | What it does |
| --- | --- |
| `GameVersion` | `GameVersion() string`: the `version` of the game's `game.json`, as written there, such as `"1.2.0"` or `"1.2.0-beta"`. Every build golib makes carries it: `golib build`, `run`, `shot`, `test`, `dist` and `web`, with or without `--dist`, `--web` and `--tags`. `""` when `game.json` has no `version`, and in a program golib didn't build, such as one from `golib go run` or `golib go test`. |

```go
// In the title scene's Draw, so a player's bug report says which build it was:
if version := golib.GameVersion(); version != "" {
	screen.DrawText("v"+version, 4, screen.Height()-14, 10, golib.Gray)
}
```

- It is the same in a demo and in the full game, since both come from one `game.json`: a game that tells them apart shows its own word next to it, from a constant in a file with `//go:build demo` (see [docs/tooling.md](../docs/tooling.md#build-tags)).
- Raise `version` in `game.json` each time a build is shared; `golib dist` puts it in the zip's name too.

## Saving data

Games keep high scores, settings and progress between runs with these functions. The data is JSON, in a file per name.

| Name | What it does |
| --- | --- |
| `SaveData` | `SaveData(name string, value any) error`: stores `value` under `name`, such as `"progress"` or `"settings"`, replacing what was there. Only exported fields are saved: a struct whose fields are all in lower case stops `Run` with an error. |
| `LoadData` | `LoadData(name string, value any) (found bool, err error)`: reads what was stored under `name` into `value`, a pointer. With nothing stored, `found` is false and `value` stays as it is. Stored data that can't be read, such as a file edited by hand, is an error. |
| `DeleteData` | `DeleteData(name string) error`: removes what was stored under `name`, such as when the player starts over. |

```go
// progress is what the game remembers between runs. JSON saves only
// exported fields, so their names start with a capital letter.
type progress struct {
	Level     int
	BestMoves map[int]int
}

func newTitleScene() *titleScene {
	s := &titleScene{progress: progress{Level: 1, BestMoves: map[int]int{}}}
	if _, err := golib.LoadData("progress", &s.progress); err != nil {
		s.message = "Your saved progress couldn't be read: starting again"
	}
	return s
}

// In Update, when a level is finished:
s.progress.Level = max(s.progress.Level, level+1)
if err := golib.SaveData("progress", s.progress); err != nil {
	s.message = "Your progress couldn't be saved"
}
```

- **Where it goes:** a `golib dist` build saves in the player's settings folder, in `GoLib games/<game>` (`%AppData%\GoLib games\<game>` on Windows), where `<game>` is the game's folder name, or `game.json`'s `fileName` when it gives one; a debug build saves in `build/<game>/save/`, next to its executable, which `golib clean` deletes.
- **Under `golib shot` and in tests, nothing is written:** data lives in memory while the program runs, so every shot and every test program starts with nothing saved. Tests of one game share their data, so a test that needs nothing saved calls `DeleteData` first, and a test can call `SaveData` to start from saved progress. `golib shot --save <file.json>` does the same for a screenshot, so a shot can open on level 8 (see [docs/tooling.md](../docs/tooling.md#screenshots)).
- Save when something worth keeping changes, such as a finished level or a changed setting, not in every update.
- Names are lowercase letters, digits, `-` and `_`. Keep the data small: settings, scores and how far the player got, not whole worlds.
- A failed save or load returns an error: tell the player, and carry on playing.

## Files: the assets folder

| Name | What it does |
| --- | --- |
| `ReadAsset` | `ReadAsset(name string) ([]byte, error)`: the contents of a file in `games/<game>/assets/`. `name` is relative to that folder, with forward slashes: `"levels/1.txt"`. |
| `ListAssets` | `ListAssets(folder string) ([]string, error)`: the names of the files in a folder of the assets folder, such as every level in `maps`, as `ReadAsset` takes them. Sorted by name, character by character, so number the files with the same number of digits: `level01.tmx`, not `level1.tmx`. Folders inside are left out; `""` lists the assets folder itself. |
| `EmbedAssets` | `EmbedAssets(files embed.FS)`: puts the assets folder inside a `golib dist` build. Only `assets.go` calls it. |

Debug builds (`golib run`, `shot`, `test`, F5) read files from disk, so an edited file shows up on the next run; `golib dist` builds read the copy inside the executable. A game with an `assets/` folder therefore needs this `assets.go` next to `main.go`, exactly as it is, or `golib dist` stops:

```go
//go:build golib_dist

package main

import (
	"embed"

	"golib"
)

//go:embed all:assets
var assets embed.FS

func init() { golib.EmbedAssets(assets) }
```

The whole folder ships, so a file that `golib run` finds is in the dist build too: read files when the game starts and treat an error as the end.

A build made with a tag that must leave files out, such as a demo built with `--tags demo` that ships without the full game's levels, keeps two files instead of this one: `assets.go` with `//go:build golib_dist && !demo` and `//go:embed all:assets`, and `assets_demo.go` with `//go:build golib_dist && demo`, whose `//go:embed` line lists only what the demo needs, such as `all:assets/sprites assets/locale/??.txt`. Each calls `EmbedAssets`. `game.json`'s `leaveOut` does the same for the files `golib dist` copies beside the executable. Both are in [docs/tooling.md](../docs/tooling.md#build-tags); play the demo with `golib run <game> --dist --tags demo` to see it finds every file it reads.

```go
func main() {
	level, err := golib.ReadAsset("levels/1.txt") // games/<game>/assets/levels/1.txt
	if err != nil {
		log.Fatal(err)
	}
	if err := golib.Run(newPlayScene(string(level)), golib.Config{Title: "Maze"}); err != nil {
		log.Fatal(err)
	}
}
```

Games read their own data files this way, such as levels in text or JSON; `NewSprite`, `NewSpriteSheet`, `NewMap`, `NewFont`, `NewSoundFile` and `NewMusic` read the assets folder the same way.

## Quitting

| Name | What it does |
| --- | --- |
| `Quit` | `Quit()`: `Run` returns right after the current update, without drawing again. Call it from `Update`, for example from a Quit menu entry. Closing the window always quits too. |

## Testing a game

Keep the rules in plain Go types, with no `Input` and no drawing, and test them by calling them directly, as `games/platformer/world_test.go` does:

```go
const dt = 1.0 / 60 // the step golib.Run passes to Update

func TestJumpOnlyFromTheGround(t *testing.T) {
	golib.SetRandomSeed(1) // needed only when the rules use random numbers
	w := newWorld()
	for range 60 {
		w.step(0, false, false, dt) // fall for a second
	}
	w.step(0, true, false, dt) // jump
	if w.player.velocityY >= 0 {
		t.Fatal("jumping from the ground didn't move the player up")
	}
}
```

- `Rectangle`, the random numbers, `ReadAsset`, `Animation`, a sprite's `Width`, `Height`, `Frames` and `Animation`, and everything a `Map` has but drawing work in tests; `golib test` runs them in the game's folder, so a test can check that a level has a start and the player can reach the exit. `golib test <game> --run <TestName>` runs only that test and shows what it logs with `t.Log`.
- `Sound.Play` and `Music.Play` play nothing in tests; `Sound.Play` still reads a sound file, so a test that plays it checks the file.
- In 3D, `Vector3`, `Rotation`, `Transform`, `Camera3D` (`ToScreen`, `Ray`, `Orbit`, `Look`, `Follow`, `AvoidWalls`, `Direction`, `Update`), a model's `Err`, `Bounds`, `Markers`, `Animations`, `AnimationLength` and `Bones`, an `Animator` (`Play`, `Update`, `Playing`, `Time`, `Bone`, `Moved`), and a `PhysicsWorld` with its `CharacterController`s, `NavMesh` and rigid bodies (with the native library `golib setup` downloads, which `golib test` finds) work in tests: check which way a key moves a character on the screen, that a level's markers are where they should be, as `games/hopper/level_test.go` does, that a jump plays the jump, that a wall stops the player, and that every key can be reached, as `games/crypt`'s tests do.
- `Run` and `Screen` need a window: check drawing with `golib shot` instead.

## Using raylib directly

Where GoLib has nothing yet, a game may call raylib itself, and tell the user, because that code should move to GoLib when the feature lands:

```go
import rl "github.com/gen2brain/raylib-go/raylib"
```

- raylib-go is already in the game's `go.mod`, as an indirect requirement; `golib go -C games/<name> mod tidy` marks it as direct.
- Call raylib only from `Update` and `Draw`, while `Run` has the window open. In `Draw`, raylib draws on the same screen as `Screen`, in screen pixels, and the post-processing applies to it too.
- Read input through `Input`, not raylib: raylib reports the mouse in window pixels, and its presses aren't delivered once per update.
- Never replace the game loop or open a window of your own.

## Playing in a browser

`golib web <game>` builds the game for the browser and serves it on the machine it runs on, so it can be played in a tab. The game's code is the same: package `golib` is the same on every platform, and the game's assets travel inside the build, so `ReadAsset` and `ListAssets` work as they do everywhere else.

A web build does less than a desktop build for now, and a game that wants to run in both should keep to the left column:

| Works in a browser | Doesn't, or not quite |
| --- | --- |
| Shapes, sprites, tilemaps, the camera, blend modes, fullscreen | Sound files in `.qoa`: browsers cannot decode them, and a game that asks for one stops with a message on the page saying so |
| Music in `.xm` and `.mod`, when a file of the same name in `.ogg`, `.mp3` or `.wav` sits beside it: GoLib plays that one | Music in `.xm` or `.mod` with no such file beside it: the game plays on without it, and says so in the browser's console |
| Sound effects and music, from code, `.jfxr`, `.wav`, `.ogg` and `.mp3` | |
| Post-processing shaders, compiled for OpenGL ES | A shader that mixes whole numbers into float arithmetic, such as `uv * 2`: ES refuses it, where the desktop allows it. Write `uv * 2.0` |
| Text in the built-in font, in the same places as on the desktop | Text from a `.ttf` or `.otf` file lands within a few pixels of where the desktop puts it, not on it: a browser's font metrics are not raylib's |
| Keyboard, mouse, gamepads and the touch screen of a phone or a tablet (see [Touch screen](#touch-screen)) | |
| `SaveData` and `LoadData`, kept by the browser for the address the game is served from | Data kept for a player who clears their browsing data, plays in a private window, or opens the game at another address |
| `golib shot <game> --web` takes the same screenshots in a browser, into `build/<game>/shots-web/` | `golib shot --web --save`: a page cannot read a file from the machine |

A game that calls raylib directly (see above) doesn't build for the browser at all.

`golib dist <game> --web` makes the zip to upload to itch.io, with `index.html` at the top of it. See [docs/roadmap.md](../docs/roadmap.md#web-build-started-2026-09-18) for what is left.

### On a phone

A web build is what a phone plays, so a game meant for one is a game meant for the browser:

- **Fingers, not keys.** A phone has no keyboard: everything the player does has to be reachable by tapping. A tap already works the mouse (see [Touch screen](#touch-screen)); a game whose only way into play is "press Enter" can't be started on a phone at all.
- **Fullscreen comes from a tap.** A browser only grants fullscreen for a moment after a key, a click or a touch, so `SetFullscreen(true)` takes effect at once when the game calls it on one of those, as a settings screen's fullscreen row does, and at the next one otherwise, as `Config.Fullscreen` does at the start. Leaving fullscreen is at once. The whole page goes fullscreen and the game fills the screen, however the phone is turned.
- **The screen is landscape or portrait, and the player chooses.** The screen the game draws on keeps its size and is scaled to fit, with black bars where the shapes differ, so a game designed for 1280 by 720 played in portrait gets thick bars; with `Config.FillWindow` it grows taller instead, and the game lays itself out for it. Put the controls inside the screen, not against the window's edges, and they stay where the thumbs are either way.
- **`golib web <game> --lan`** serves the game to the network, so a phone on the same Wi-Fi can open it and play while the game is still being written; the terminal prints the address to type.

## What GoLib doesn't have yet

| Missing | Status | Meanwhile |
| --- | --- | --- |
| Parts of an image that aren't on a grid, Aseprite slices and tilemap layers | Not on the roadmap yet | Save each part as its own PNG file, or put the parts on a grid |
| Isometric and hexagonal maps; drawing a map's shapes and text | Not on the roadmap yet | Orthogonal maps; draw what objects stand for with sprites and shapes |
| Positional sound in 2D | Not on the roadmap yet: 3D has `Sound.PlayAt` | `Sound.PlayWith` at a lower volume for what is far away |
| Physics in 2D: things that fall, roll and bounce on their own | Not on the roadmap yet: 3D has rigid bodies, with Jolt | Simple movement and `Rectangle` overlap checks in the game |
| Joints between rigid bodies (hinges, chains, ragdolls), cars, rigid bodies that push characters, and collision events | Not on the roadmap yet | A swinging door is a `Moved` body turned by the game; a falling crate the game checks against the player; a hit is a sudden change in `Velocity` |
| Trigger pressure, vibration | Not on the roadmap yet | Triggers read as buttons |
| Inverse kinematics, and reading one part of a model, such as where its mesh is | Not on the roadmap yet | `Model.Hide` and `Show` hide parts; markers say where things are |
| Hiding or dimming part of an HDRI | Not on the roadmap yet | Paint the HDRI over in a copy made offline, as [games/sunset](../games/sunset)'s `sources/skies/lakesky.go` does |
| Smooth surfaces reflecting each other and what is off the screen; particles in reflections | Not on the roadmap yet | Reflection probes baked in Blender show the level off the screen; a small solid glowing shape under a flame is reflected where its particles aren't |
| Keeping one thing out of the haze, a thing that casts no shadow, a color for each burst of one `Particles`, and taking one object of a model out of a `PhysicsWorld` | Not on the roadmap yet | A second `Begin3D` pass without haze over the first; a `Particles` for each color; what the game removes added on its own with `AddMesh` or `AddModel` |
| `.qoa`, `.xm` and `.mod` in a browser | Not planned: browsers cannot decode them | Save the sound as `.wav`, `.ogg` or `.mp3`. For music, put that file next to the `.xm` under the same name: a web build plays it, and the desktop keeps playing the module |

When a game needs one of these, say so to the user and point to [docs/roadmap.md](../docs/roadmap.md), as [AGENTS.md](../AGENTS.md) asks, instead of building an engine to fill the gap.
