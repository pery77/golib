# Playbook: making a game with GoLib

For AI agents. Follow it when a user asks you to create a game, or to change the game in this project.

> **Status:** the framework is a seed (M1): a window, a game loop, clearing the screen and drawing text. Input, sprites, audio and scenes arrive in M2; check the "Project status" table in [AGENTS.md](../../AGENTS.md). If the game needs something that doesn't exist yet, tell the user. Don't build a private engine to fill the gap.

## Goal

A game the user can play and enjoy after one prompt, which you (or a later session that remembers nothing of this one) can keep improving in short iterations.

## Where a game lives

Each game is its own folder, and its own Go module, in `games/`. Until `golib new` exists (M2), create one by hand:

1. Pick a short lowercase folder name, such as `asteroids`, and create `games/asteroids/`.
2. Write `games/asteroids/go.mod`:

   ```text
   module asteroids

   go 1.27.1

   require golib v0.0.0

   replace golib => ../../framework
   ```

3. Write `main.go` in `package main`, calling `golib.Run`. [games/hello/main.go](../../games/hello/main.go) is a complete example.
4. Run `golib go -C games/asteroids mod tidy`, then `golib run asteroids`.

The framework's API is documented in the doc comments of `framework/*.go`: read them before writing game code. Where the framework has nothing yet, a game may call raylib directly (`github.com/gen2brain/raylib-go/raylib`). Tell the user when you do, because that code should move to framework APIs as they land.

## 1. Understand the request

- Pull out the genre and core mechanic, the controls, the goal (score, win, lose), the mood and visual style, and any hints about scope.
- Ask questions only when the answer changes the game fundamentally, such as "turn-based or real-time?". Ask at most three, all in one message. Otherwise decide, and list your assumptions so the user can correct them.
- When the user doesn't say, use these defaults:

| Topic | Default |
| --- | --- |
| Window | 1280x720 |
| Timing | 60 FPS target; movement scaled by frame time |
| Input | Keyboard (arrow keys and WASD); mouse when the genre needs it |
| Art | Simple shapes and a small, coherent color palette drawn in code. No external files unless the user provides them |
| Audio | Optional. The game must be fully playable muted |
| Text | English, readable at a glance; controls shown on the title screen |
| Scope | One polished core loop rather than many half-finished features |

## 2. Write the design brief

Before writing code, create a short `DESIGN.md` in the game's folder: `games/<name>/DESIGN.md`. It is the game's memory across sessions, so keep it current.

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

1. **Skeleton:** window, game loop, clear screen, clean exit.
2. **Player:** the player moves with the controls.
3. **Core mechanic:** the thing that makes this game this game.
4. **Rules:** score, failure, victory, difficulty ramp.
5. **Flow:** title screen, pause, game over, restart without relaunching.
6. **Feel:** feedback for every player action, such as flashes, particles, screen shake, easing and sound.
7. **Balance and polish:** tune the numbers, fix rough edges, go through the quality checklist.

## 4. Verify every slice

- Run `golib test`, which runs `go vet` and `go test` for the framework and every game, and fix everything it reports.
- Test pure logic with Go tests: collisions, scoring, level generation, state transitions. Keep that logic free of drawing calls so it stays testable.
- Run the game with `golib run <name>`. Once screenshots can be captured without a human (planned: `golib shot`), capture them and look at them.
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

- Files the user provides go in the game's `assets/` folder.
- When a game has levels and the framework loads Tiled maps, write the levels as Tiled maps instead of arrays in code, so the user can open and change them in Tiled.
- Never build a level editor, sprite editor or other content tool, inside the game or next to it.

## Code organization

- Group tuning constants together, with units in the name or a comment: `playerSpeed = 240 // pixels per second`. "Make the player faster" should be a one-line change.
- Keep game state in structs you pass around, not in package-level variables.
- Separate updating (input, logic) from drawing. Drawing never changes game state.
- Split files by responsibility (player, enemies, level, UI) once a file grows past roughly 400 lines.
- Keep everything for the game inside `games/<name>/`. Don't modify `framework/` to build a game: if the framework lacks something, work around it in game code and tell the user; framework changes follow [contributing.md](../contributing.md).

## Quality checklist

Before calling a game done:

- [ ] Starts with `golib run <name>`, with no errors in the output.
- [ ] Title screen shows the game's name and its controls.
- [ ] The core loop plays for several minutes without crashes or soft-locks.
- [ ] Scoring, damage and failure give clear feedback.
- [ ] Pause and resume work; restart after game over works without relaunching.
- [ ] Closing the window exits cleanly.
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
