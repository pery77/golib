# GoLib API guide

For AI agents writing game code with GoLib, and for anyone who wants the whole framework on one page. It covers every exported name in package `golib`, the Go code in this folder, grouped by what a game needs, with the rules the names don't tell you. The doc comments in the `.go` files have the full details, and [games/platformer](../games/platformer) and [games/asteroids](../games/asteroids) show all of it in real games.

`golib test` checks this page against the code: it fails when an exported name is missing here, or when this page names one that doesn't exist (see `apiguide_test.go`). Change both together.

| To | Use | Section |
| --- | --- | --- |
| Start the game | `golib.Run(firstScene, golib.Config{...})` in `main` | [Game, Run and Config](#game-run-and-config) |
| Change the game 60 times per second | `Update(input *golib.Input, dt float32)` | [Game, Run and Config](#game-run-and-config) |
| Draw it | `Draw(screen *golib.Screen)` | [Drawing](#drawing) |
| Read keys, mouse and gamepads | `input.KeyDown`, `input.KeyPressed`, `input.MousePosition`, `input.GamepadDown` | [Input](#input) |
| Check collisions | `golib.Rectangle` and its `Overlaps` | [Rectangles and collisions](#rectangles-and-collisions) |
| Move between title, play, pause and game over | `golib.SwitchScene` | [Scenes](#scenes) |
| Roll dice | `golib.RandomInt`, `golib.RandomFloat` | [Random numbers](#random-numbers) |
| Play sound effects | `golib.NewSound`, `golib.Laser` and the other recipes | [Sound effects](#sound-effects) |
| Play music | `golib.NewMusic` | [Music](#music) |
| Go fullscreen, add a CRT look | `golib.SetFullscreen`, `golib.NewShader`, `golib.SetPostProcess` | [Window, fullscreen and screen effects](#window-fullscreen-and-screen-effects) |
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

1. **`dt` is always 1/60 of a second.** Time everything with it (`timer -= dt`), never with `time.Now`. Games then play the same at any frame rate, on any machine and in `golib shot`.
2. **`Update` changes the game; `Draw` only draws.** `Run` draws once per frame, after zero, one or several updates, so a `Draw` that changed the game would make it play differently at each frame rate.
3. **A scene must be ready to draw as soon as it is made.** Set its state in its constructor, not in its first `Update`: `Run` can draw a scene before updating it, on the first frame and on the frame a `SwitchScene` lands.
4. **Pressed happens once, Down lasts.** `KeyPressed`, `MousePressed` and `GamepadPressed` are true in exactly one update per press: use them for jumping, firing and menus. `KeyDown`, `MouseDown` and `GamepadDown` stay true while held: use them for walking and thrust.
5. **Use `Input` only in `Update`, and `Screen` only in `Draw`.** Don't keep them in the game's state.
6. **Draw in screen pixels.** The screen is `Config.Width` by `Config.Height` pixels, with 0, 0 at the top-left corner and y growing downwards, whatever the window's size. `Run` scales it to the window, with black bars where the shapes differ, and reports the mouse in the same pixels.
7. **Make sounds, music and shaders once, and keep them.** Create them as package variables or in `main`, never in `Update`, in `Draw` or in a scene that is made again for every new game: each one loads the first time it is used and stays loaded until `Run` returns.
8. **Random numbers come from `RandomInt` and `RandomFloat`**, never from `math/rand`: `golib shot` starts them from the same seed, so its screenshots repeat.
9. **No key quits on its own, not even Esc.** Call `Quit` when the game should end; the game decides what Esc does.
10. **`SwitchScene` and `Quit` don't stop the current update.** The code after them still runs, so `return` right after unless that is what you want.
11. **Mistakes stop the game with a message.** `Run` returns an error for a nil scene, a shader that doesn't compile, a bad `SetUniform` or music that can't be read. `main` prints it with `log.Fatal`; read it, it says what to fix.
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
| `Config.Width`, `Config.Height` | 1280, 720 | Size of the screen the game draws on, in pixels. It never changes; the window scales it. |
| `Config.Fullscreen` | `false` | Start in fullscreen |
| `Config.PixelArt` | `false` | Scale the screen by whole numbers only, without smoothing, so pixels stay square and sharp. Use it with a small screen, such as 320 by 180. |

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

A paused game stops counting because its scene stops getting updates.

## Input

`Input` is the keyboard, the mouse and up to four gamepads, as one update sees them. Read the keyboard and gamepad 0 together, so every game works with both:

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
| `Key` | A keyboard key: one of the constants below. Letters and digits are named after their place on a US keyboard. |

| Keys | Constants |
| --- | --- |
| Common | `KeySpace` `KeyEnter` `KeyEscape` `KeyTab` `KeyBackspace` |
| Arrows | `KeyLeft` `KeyRight` `KeyUp` `KeyDown` |
| Modifiers | `KeyLeftShift` `KeyRightShift` `KeyLeftControl` `KeyRightControl` `KeyLeftAlt` `KeyRightAlt` |
| Letters | `KeyA` `KeyB` `KeyC` `KeyD` `KeyE` `KeyF` `KeyG` `KeyH` `KeyI` `KeyJ` `KeyK` `KeyL` `KeyM` `KeyN` `KeyO` `KeyP` `KeyQ` `KeyR` `KeyS` `KeyT` `KeyU` `KeyV` `KeyW` `KeyX` `KeyY` `KeyZ` |
| Digits (spelled out: there is no `Key0`) | `KeyZero` `KeyOne` `KeyTwo` `KeyThree` `KeyFour` `KeyFive` `KeySix` `KeySeven` `KeyEight` `KeyNine` |
| Function keys | `KeyF1` `KeyF2` `KeyF3` `KeyF4` `KeyF5` `KeyF6` `KeyF7` `KeyF8` `KeyF9` `KeyF10` `KeyF11` `KeyF12` |

Only these keys are read. There is no numeric keypad, punctuation, Delete or Home yet, and a raylib key code converted to `Key` is never down.

### Mouse

| Name | What it does |
| --- | --- |
| `Input.MousePosition` | `MousePosition() (x, y float32)`: the pointer, in screen pixels. Over the black bars or outside the window it can be outside the screen. |
| `Input.MouseDown` | `MouseDown(button MouseButton) bool`: the button is held down. |
| `Input.MousePressed` | `MousePressed(button MouseButton) bool`: the button went down since the previous update; true in one update per click. |
| `Input.MouseWheel` | `MouseWheel() float32`: notches the wheel turned since the previous update. Positive is up, away from the player; 0 means it didn't move. |
| `MouseButton` | `MouseLeft`, `MouseRight` or `MouseMiddle`. |

```go
x, y := input.MousePosition()
if input.MousePressed(golib.MouseLeft) && s.playButton.Contains(x, y) {
	golib.SwitchScene(newPlayScene())
	return
}
```

### Gamepads

Gamepads are numbered 0 to 3, in the order they were connected; a one-player game reads gamepad 0. Every call is safe on a gamepad that isn't connected: it reads as nothing pressed and sticks at rest.

| Name | What it does |
| --- | --- |
| `Input.GamepadConnected` | `GamepadConnected(pad int) bool` |
| `Input.GamepadName` | `GamepadName(pad int) string`: the name the system gives it, such as `"Xbox Controller"`, or `""` |
| `Input.GamepadDown` | `GamepadDown(pad int, button GamepadButton) bool`: the button is held down. |
| `Input.GamepadPressed` | `GamepadPressed(pad int, button GamepadButton) bool`: the button went down since the previous update; true in one update per press. |
| `Input.GamepadLeftStick` | `GamepadLeftStick(pad int) (x, y float32)`: each from -1 to 1, with y growing downwards like the screen. 0, 0 at rest: a small dead zone around the center is removed. |
| `Input.GamepadRightStick` | `GamepadRightStick(pad int) (x, y float32)`: the same for the right stick. |
| `GamepadButton` | A gamepad button: one of the constants below. |

| Constant | Xbox | PlayStation | Usual use |
| --- | --- | --- | --- |
| `GamepadA` | A, bottom | Cross | Jump, confirm |
| `GamepadB` | B, right | Circle | Back, cancel |
| `GamepadX` | X, left | Square | Second action |
| `GamepadY` | Y, top | Triangle | Third action |
| `GamepadUp` `GamepadDown` `GamepadLeft` `GamepadRight` | D-pad | D-pad | Move, menus |
| `GamepadLeftBumper` `GamepadRightBumper` | LB, RB | L1, R1 | |
| `GamepadLeftTrigger` `GamepadRightTrigger` | LT, RT, read as buttons | L2, R2 | |
| `GamepadStart` | Menu | Options | Pause |
| `GamepadBack` | View | Share, Select | |
| `GamepadLeftStickButton` `GamepadRightStickButton` | Pressing a stick in | L3, R3 | |

## Drawing

`Screen` is what `Draw` draws on. Everything later covers what was drawn before, so draw from back to front, and start with `Clear`.

| Name | What it does |
| --- | --- |
| `Screen.Clear` | `Clear(color Color)`: fills the whole screen. |
| `Screen.DrawRectangle` | `DrawRectangle(rect Rectangle, color Color)`: a filled rectangle. |
| `Screen.DrawCircle` | `DrawCircle(x, y, radius float32, color Color)`: a filled circle centered at x, y. |
| `Screen.DrawLine` | `DrawLine(x1, y1, x2, y2, thickness float32, color Color)`: a straight line, thickness pixels wide. |
| `Screen.DrawTriangle` | `DrawTriangle(x1, y1, x2, y2, x3, y3 float32, color Color)`: a filled triangle; the corners can come in any order. |
| `Screen.DrawText` | `DrawText(text string, x, y, size float32, color Color)`: text with its top-left corner at x, y, size pixels high. |
| `Screen.TextWidth` | `TextWidth(text string, size float32) float32`: how wide `DrawText` draws text, to center or right-align it. |
| `Screen.Width`, `Screen.Height` | `Width() float32`, `Height() float32`: the screen's size, from `Config`. |

- Positions and sizes are `float32`. Untyped constants convert by themselves; other numbers need `float32(n)`.
- Shapes are filled. For an outline, draw lines. For a rotated shape, work out its corners with `math.Sin` and `math.Cos` and draw triangles or lines, as `games/asteroids/scenes.go` does for the ship and the rocks.
- There is no camera. To scroll, subtract the camera's position from everything you draw.
- There is one font, raylib's built-in pixel font, which is 10 pixels high: sizes that are multiples of 10 keep its pixels even.

```go
// drawCentered draws text centered across the screen.
func drawCentered(screen *golib.Screen, text string, y, size float32, color golib.Color) {
	x := (screen.Width() - screen.TextWidth(text, size)) / 2
	screen.DrawText(text, x, y, size, color)
}
```

### Colors

`Color` is a color with red, green, blue and opacity from 0 to 255: `golib.Color{R: 20, G: 24, B: 32, A: 255}`. `A` is 255 for solid colors; lower values let what is underneath show through, so `golib.Color{A: 150}` over the whole screen darkens it under a pause message. `Color` is raylib's `rl.Color`, so it can be passed to raylib directly.

The named colors are raylib's palette: `LightGray` `Gray` `DarkGray` `Yellow` `Gold` `Orange` `Pink` `Red` `Maroon` `Green` `Lime` `DarkGreen` `SkyBlue` `Blue` `DarkBlue` `Purple` `Violet` `DarkPurple` `Beige` `Brown` `DarkBrown` `White` `Black` `Magenta`, `RayWhite` (the off-white raylib uses for backgrounds) and `Blank` (fully transparent).

Keep a game's colors together as named variables, as `games/platformer/main.go` does, so its look changes in one place.

## Rectangles and collisions

| Name | What it does |
| --- | --- |
| `Rectangle` | A rectangle aligned with the screen, in pixels: `golib.Rectangle{X: 10, Y: 20, Width: 40, Height: 60}`. |
| `Rectangle.X`, `Rectangle.Y` | Its top-left corner. |
| `Rectangle.Width`, `Rectangle.Height` | Its size. |
| `Rectangle.Overlaps` | `Overlaps(other Rectangle) bool`: the two share some area. Rectangles that only touch along an edge don't overlap, so a player standing on a platform isn't inside it. |
| `Rectangle.Contains` | `Contains(x, y float32) bool`: the point is inside, such as the mouse over a button. The left and top edges are inside; the right and bottom ones are not. |

That is all the geometry GoLib has: no vectors, circles or physics. Check circles with their distance:

```go
dx, dy := a.x-b.x, a.y-b.y
reach := a.radius + b.radius
hit := dx*dx+dy*dy < reach*reach
```

Pushing a player out of a wall is game code: `games/platformer/world.go` moves one axis at a time and pushes the player back out of any platform it overlaps.

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

GoLib makes sound effects in code from a few numbers, so a game ships no sound files.

| Name | What it does |
| --- | --- |
| `NewSound` | `NewSound(spec SoundSpec) *Sound`: the sound the recipe describes. |
| `Sound` | A sound effect, ready to play. |
| `Sound.Play` | `Play()`: plays the sound, over any copy of it that is still playing; up to four copies at once, and a fifth cuts off the oldest. |
| `Laser` | A falling zap, for shots. |
| `Explosion` | A low burst of noise, for things breaking apart. |
| `Pickup` | A bright blip that rises, for coins. |
| `Jump` | A soft rising note, for jumps. |
| `Hurt` | A harsh falling note, for taking damage. |
| `PowerUp` | A rising fanfare, for upgrades, extra lives and cleared levels. |
| `SetVolume` | `SetVolume(volume float32)`: how loud all sound and music is, from 0 to 1. It works before `Run` too. |

The six recipes return a `*Sound`, like `NewSound`. Keep every sound of a game in one file:

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
)
```

and play them where things happen: `shotSound.Play()`.

`SoundSpec` is the recipe. Every field left at zero gets its default, so `SoundSpec{}` is a short beep.

| Field | Default | Meaning |
| --- | --- | --- |
| `SoundSpec.Wave` | `WaveSquare` | The sound's character (below). |
| `SoundSpec.Frequency` | 440 | Pitch the sound starts at, in Hz. |
| `SoundSpec.Slide` | 0 | Hz added every second: negative falls (a shot), positive rises (a coin). |
| `SoundSpec.Duration` | 0.25 | Seconds; at most 10. |
| `SoundSpec.Attack` | 0.005 | Seconds fading in, so the start doesn't click. |
| `SoundSpec.Release` | 0.05 | Seconds fading out at the end. |
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

`Waveform` is the type of these constants. A sound can't loop, be stopped or be loaded from a file yet.

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
- A game with music has an `assets/` folder, so it needs `assets.go` (see [Files](#files-the-assets-folder)).
- Only use music the user provides, and write where it came from, and its license, in `assets/ATTRIBUTION.md`, as `games/asteroids` does.
- There are no crossfades or playlists: `Stop` one `Music` and `Play` another.

## Window, fullscreen and screen effects

| Name | What it does |
| --- | --- |
| `SetFullscreen` | `SetFullscreen(on bool)`: fullscreen or a window, from the next frame. Fullscreen covers the monitor without changing its resolution, and the screen keeps its size. Call it from `Update`; to start in fullscreen, set `Config.Fullscreen`, because `Run` replaces an earlier call with it. |
| `IsFullscreen` | `IsFullscreen() bool`: the game is in fullscreen, or will be from the next frame. |

```go
altEnter := (input.KeyDown(golib.KeyLeftAlt) || input.KeyDown(golib.KeyRightAlt)) && input.KeyPressed(golib.KeyEnter)
if input.KeyPressed(golib.KeyF11) || altEnter {
	golib.SetFullscreen(!golib.IsFullscreen())
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

## Files: the assets folder

| Name | What it does |
| --- | --- |
| `ReadAsset` | `ReadAsset(name string) ([]byte, error)`: the contents of a file in `games/<game>/assets/`. `name` is relative to that folder, with forward slashes: `"levels/1.txt"`. |
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

Today a game can read its own data files this way, such as levels in text or JSON, and music through `NewMusic`. Images, maps and fonts can't be loaded yet.

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
		w.step(0, false, dt) // fall for a second
	}
	w.step(0, true, dt)
	if w.player.velocityY >= 0 {
		t.Fatal("jumping from the ground didn't move the player up")
	}
}
```

- `Rectangle`, the random numbers and `ReadAsset` work in tests; `golib test` runs them in the game's folder.
- `Sound.Play` and `Music.Play` do nothing in tests.
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

## What GoLib doesn't have yet

| Missing | Status | Meanwhile |
| --- | --- | --- |
| Images, sprites and animations (Aseprite) | M4, postponed | Shapes drawn in code |
| Maps (Tiled) | M4, postponed | Levels in code, or in a text file read with `ReadAsset` |
| Fonts | M4, postponed | The built-in font of `DrawText` |
| Sound effects from files | M4, postponed | `NewSound` |
| Looping sounds, stopping a sound | Not on the roadmap yet | Short sounds, played again |
| Camera, vectors, rotation, physics | Not on the roadmap yet | `float32` math in the game: subtract a camera position, rotate with `math.Sin` and `math.Cos` |
| Trigger pressure, vibration | Not on the roadmap yet | Triggers read as buttons |
| Saving high scores or settings | Not on the roadmap yet | Keep them while the game runs |
| 3D | M6, after M4 | None |

When a game needs one of these, say so to the user and point to [docs/roadmap.md](../docs/roadmap.md), as [AGENTS.md](../AGENTS.md) asks, instead of building an engine to fill the gap.
