# Playbook: making a game with GoLib

For AI agents. Follow it when a user asks you to create a game, or to change the game in this project.

> **Status:** the framework is still small (M2 in progress): a window, a fixed-step game loop, keyboard, mouse and gamepad input, random numbers, rectangles, circles and text, reading files from `assets/`, quitting with `golib.Quit`, screenshots through `golib shot` and single-file builds through `golib dist`. scenes with `golib.SwitchScene`. Sprites, maps and audio are postponed; check the "Project status" table in [AGENTS.md](../../AGENTS.md). If the game needs something that doesn't exist yet, tell the user. Don't build a private engine to fill the gap.

## Goal

A game the user can play and enjoy after one prompt, which you (or a later session that remembers nothing of this one) can keep improving in short iterations.

## Where a game lives

Each game is its own folder, and its own Go module, in `games/`. Create one with `golib new`, and a short lowercase name:

```text
golib new asteroids
golib run asteroids
```

`golib new` copies `tools/template/game/` into `games/asteroids/` and runs `go mod tidy`, so the game runs straight away: a square that moves with the arrows, WASD, the d-pad or the left stick. Build the real game on top of it:

| File | Holds |
| --- | --- |
| `main.go` | `main`, which calls `golib.Run` with the first scene |
| `play.go` | The play scene: `Update` turns input into actions, `Draw` draws the world |
| `world.go` | The rules and the tuning constants, with no input or drawing |
| `world_test.go` | Tests for the rules |
| `DESIGN.md` | The design brief, with placeholder text to replace |
| `go.mod`, `go.sum` | The Go module; `replace golib => ../../framework` points it at the framework |

[games/platformer](../../games/platformer) is the reference game. Read it before writing one, and follow its shape:

| File | Shows |
| --- | --- |
| [main.go](../../games/platformer/main.go) | How a game starts: `main` calls `golib.Run` with the first scene |
| [scenes.go](../../games/platformer/scenes.go) | Scenes (title, play, pause, won) and `golib.SwitchScene` between them. The play scene's `Update` turns input into actions and its `Draw` draws the state; pause and won draw the play scene under a message; the title menu works with the keyboard, the mouse and a gamepad; clouds are placed with `golib.RandomFloat` |
| [world.go](../../games/platformer/world.go) | The rules as plain Go types, with no input or drawing, and the tuning constants at the top |
| [world_test.go](../../games/platformer/world_test.go) | Testing the rules by calling them directly, without a window or a keyboard |
| [DESIGN.md](../../games/platformer/DESIGN.md) | The design brief |

The framework's API is documented in the doc comments of `framework/*.go`: read them too. Where the framework has nothing yet, a game may call raylib directly (`github.com/gen2brain/raylib-go/raylib`). Tell the user when you do, because that code should move to framework APIs as they land.

## 1. Understand the request

- Pull out the genre and core mechanic, the controls, the goal (score, win, lose), the mood and visual style, and any hints about scope.
- Ask questions only when the answer changes the game fundamentally, such as "turn-based or real-time?". Ask at most three, all in one message. Otherwise decide, and list your assumptions so the user can correct them.
- When the user doesn't say, use these defaults:

| Topic | Default |
| --- | --- |
| Window | 1280x720 |
| Timing | 60 FPS target; movement scaled by frame time |
| Input | Keyboard (arrow keys and WASD) and gamepad 0 (d-pad or left stick, A to act, Start to pause) together; mouse when the genre needs it |
| Art | Simple shapes and a small, coherent color palette drawn in code. No external files unless the user provides them |
| Audio | Optional. The game must be fully playable muted |
| Text | English, readable at a glance; controls shown on the title screen |
| Scope | One polished core loop rather than many half-finished features |

## 2. Write the design brief

Before writing code, fill in `games/<name>/DESIGN.md`, which `golib new` creates with placeholder text. Keep it short. It is the game's memory across sessions, so keep it current.

````markdown
# <Game title>

## Pitch
One or two sentences.

## Core loop
What the player does every few seconds.

## Controls
| Input | Action |

## Rules
Scoring, how you lose, how you win, how difficulty grows.

## Screens
Title, playing, paused, game over.

## Tuning
The numbers that define the feel (speeds, timers, spawn rates) and where they live in the code.

## Later
Ideas that are out of scope for now.

## Changelog
One line per iteration: what changed and why.
````

Summarize the brief to the user in a few lines and carry on, unless they object.

## 3. Build in playable slices

Each slice ends with a game that builds and runs. Never write the whole game before running it once.

1. **Skeleton:** window, game loop, clear screen, clean exit. `golib new` starts you here.
2. **Player:** the player moves with the controls.
3. **Core mechanic:** the thing that makes this game this game.
4. **Rules:** score, failure, victory, difficulty ramp.
5. **Flow:** title screen, pause, game over, restart without relaunching. Make each screen a scene and move between them with `golib.SwitchScene`, as `games/platformer/scenes.go` does.
6. **Feel:** feedback for every player action, such as flashes, particles, screen shake, easing and sound.
7. **Balance and polish:** tune the numbers, fix rough edges, go through the quality checklist.

## 4. Verify every slice

- Run `golib test`, which runs `go vet` and `go test` for the framework and every game, and fix everything it reports.
- Test pure logic with Go tests: collisions, scoring, level generation, state transitions. Keep that logic free of drawing calls so it stays testable.
- Run the game with `golib run <name>`, and look at it with `golib shot <name> [frame...]`. It saves PNG screenshots of the given frames without opening a visible window; frame N shows the game after N updates, and 60 updates are one second. Open the files and check what they show. Add `--input` to play keys and the mouse on chosen updates: `golib shot <name> 120 --input "Enter@1 Right@10-100 Space@40"` presses Enter, walks right and jumps, `"Mouse@5:640,500 MouseLeft@6"` clicks at 640, 500, and `"GamepadLeftStick@10:1,0 GamepadA@20"` walks and jumps with a gamepad. Take shots of every scene this way, not just the first one.
- For anything you can't observe yourself, like feel, difficulty or audio, say so and tell the user exactly what to try.
- Never claim a game works or is fun because it compiles.

## 5. Iterate with the user

- End each iteration with: what you built, how to run it, what to try (specific keys and situations), known issues, and two or three ideas for the next step.
- Turn vague feedback into a concrete, named change. "Too hard" becomes "enemy speed 220 to 160 px/s, spawn interval 1.0 to 1.4 s".
- Keep iterations small, and update the Tuning and Changelog sections of the brief every time.

## Content

GoLib has no editors. Content comes from established tools, and the game loads their files from its `assets/` folder. Formats and support status are in [architecture.md](../architecture.md).

| Content | Tool |
| --- | --- |
| 2D maps and levels | Tiled |
| Sprites and animations | Aseprite |
| 3D models | Blender |

- Files the user provides go in the game's `assets/` folder. Read them with `golib.ReadAsset("sprites/player.png")`, with paths relative to that folder.
- A game with an `assets/` folder also needs `assets.go` next to `main.go`, so `golib dist` embeds the folder. Copy it exactly from the `golib.EmbedAssets` documentation in `framework/assets.go`. `golib dist` stops and says so when it is missing.
- When a game has levels and the framework loads Tiled maps, write the levels as Tiled maps instead of arrays in code, so the user can open and change them in Tiled.
- Never build a level editor, sprite editor or other content tool, inside the game or next to it.

## Code organization

- Group tuning constants together, with units in the name or a comment: `playerSpeed = 240 // pixels per second`. "Make the player faster" should be a one-line change.
- Keep game state in structs you pass around, not in package-level variables.
- Base every timer, movement and animation on `dt` (always 1/60 s), never on `time.Now`. The game then plays the same on every machine and in screenshots.
- Get random numbers from `golib.RandomInt` and `golib.RandomFloat`, never from `math/rand`: they start from the same seed under `golib shot`, so shots repeat. Tests that use them call `golib.SetRandomSeed` first.
- Separate updating (input, logic) from drawing. Drawing never changes game state.
- End the game with `golib.Quit()` from `Update`, for example from a Quit menu entry. No key quits on its own, not even Esc; closing the window always does.
- In `Update`, turn keys into intentions (move left, jump) and pass those to the rules. The rules then never touch `golib.Input`, and tests can call them directly.
- Split files by responsibility (player, enemies, level, UI) once a file grows past roughly 400 lines.
- Keep everything for the game inside `games/<name>/`. Don't modify `framework/` to build a game: if the framework lacks something, work around it in game code and tell the user; framework changes follow [contributing.md](../contributing.md).

## Sharing the game

`golib dist <name>` builds `build/<name>/dist/<name>.exe` (no `.exe` on Linux and macOS): a single file with raylib and the game's assets inside, ready to share, for example on itch.io. Debug builds from `golib run` and `golib build` need the library files next to them, so don't hand those out. When the user wants to share the game, run `golib dist` and tell them where the file is.

## Quality checklist

Before calling a game done:

- [ ] Starts with `golib run <name>`, with no errors in the output.
- [ ] Title screen shows the game's name and its controls.
- [ ] The core loop plays for several minutes without crashes or soft-locks.
- [ ] Scoring, damage and failure give clear feedback.
- [ ] Pause and resume work; restart after game over works without relaunching.
- [ ] Closing the window exits cleanly.
- [ ] `golib dist <name>` succeeds.
- [ ] Game speed is the same at 30, 60 and 144 FPS.
- [ ] Visual style is consistent and text is readable.
- [ ] The game is fully playable with audio muted.
- [ ] No debug output or leftover test code.
- [ ] `DESIGN.md` matches the game.

## Anti-patterns

- Writing the entire game in one go, then running it for the first time.
- Building an engine layer, ECS or plugin system on top of GoLib "for later".
- Building a level editor, sprite editor or content GUI instead of using Tiled, Aseprite or Blender files.
- Adding third-party dependencies without the user's explicit approval.
- Downloading images or sounds from the internet: their licenses are unknown. Use shapes, art generated in code, or files the user provides.
- Magic numbers scattered through the code.
- Opening with a long questionnaire instead of a first playable version.
