# Playbook: making a game with GoLib

For AI agents. Follow it when a user asks you to create a game, or to change the game in this project.

> **Status:** the framework is still small (M5, Shipping, is done on Windows, and M6, 2D essentials, has started): a window, a fixed-step game loop, keyboard, mouse and gamepad input, random numbers, rectangles, circles, lines, triangles and text, sprites and animations from PNG and Aseprite files, Tiled maps, fonts, vector math, a camera for worlds larger than the screen, fullscreen, post-processing shaders, sound effects made in code, from jfxr's `.jfxr` files or from sound files, music from the game's `assets/` folder, reading files from `assets/`, scenes with `golib.SwitchScene`, quitting with `golib.Quit`, screenshots through `golib shot` and zips to share through `golib dist`. Check the "Project status" table in [AGENTS.md](../../AGENTS.md), and the end of [framework/README.md](../../framework/README.md) for what else is missing. If the game needs something that doesn't exist yet, tell the user. Don't build a private engine to fill the gap.

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
| `main.go` | `main`, which calls `golib.Run` with the first scene; the screen's size and the game's colors |
| `play.go` | The play scene: `Update` turns input into actions, `Draw` draws the world |
| `world.go` | The rules, the tuning constants and the world's size, with no input or drawing |
| `world_test.go` | Tests for the rules |
| `DESIGN.md` | The design brief, with placeholder text to replace |
| `game.json` | The title, version and author that `golib dist` writes into the executable |
| `go.mod`, `go.sum` | The Go module; `replace golib => ../../framework` points it at the framework |

When the user wants the game kept out of the GoLib repository — a commercial project, a client's game, content they can't redistribute — start the name with `_`: `golib new _moonshot`. `.gitignore` has `/games/_*/`, so that folder never enters this repository and can hold a git repository of its own, and everything else works exactly the same. Choose it only when the user asks for it; a game is public otherwise. The rules are in [architecture.md](../architecture.md#private-games), and they don't change what you may do: a private game still uses the framework's exported API, and anything it needs from GoLib is a separate framework change, which is public.

[games/platformer](../../games/platformer) is the reference game. Read it before writing one, and follow its shape:

| File | Shows |
| --- | --- |
| [main.go](../../games/platformer/main.go) | How a game starts: `main` calls `golib.Run` with the first scene, on a 320 by 180 pixel art screen |
| [scenes.go](../../games/platformer/scenes.go) | Scenes (title, play, pause, won) and `golib.SwitchScene` between them. The play scene's `Update` turns input into actions and its `Draw` draws the state; pause and won draw the play scene under a message; the title menu works with the keyboard, the mouse and a gamepad, over the level drawn with `screen.DrawMap` |
| [world.go](../../games/platformer/world.go) | The rules as plain Go types, with no input or drawing, and the tuning constants at the top. The level comes from a Tiled map: the start and the snakes are objects, and tiles are solid or water by their properties |
| [draw.go](../../games/platformer/draw.go) | Drawing the world through a `golib.Camera` that follows the player (made and moved in [scenes.go](../../games/platformer/scenes.go)), the map layer by layer with sprites between the layers, animations, and clouds placed with `golib.RandomFloat` |
| [art.go](../../games/platformer/art.go) | The sprite sheets, their frames and animations, and the map, made once as package variables |
| [assets/maps/](../../games/platformer/assets/maps) | The level, `forest.tmx`, and its tileset, `forest.tsx`, as Tiled saves them |
| [sounds.go](../../games/platformer/sounds.go), [assets/sounds/](../../games/platformer/assets/sounds) | The sounds, made once as package variables: recipes, a `golib.SoundSpec`, and `chest.jfxr`, a sound as jfxr saves it |
| [world_test.go](../../games/platformer/world_test.go) | Testing the rules by calling them directly, without a window or a keyboard, on the real level |
| [DESIGN.md](../../games/platformer/DESIGN.md) | The design brief |

[games/asteroids](../../games/asteroids) is a second example, for screen effects: GLSL shaders in `shaders/`, embedded with `//go:embed` and turned on and off with `golib.SetPostProcess` (F2), fullscreen on F11 or Alt+Enter, and sound effects made in code with the music, kept together in [sounds.go](../../games/asteroids/sounds.go); its `assets/` folder holds the music and the `ATTRIBUTION.md` that says where it comes from.

Before writing code, read [framework/README.md](../../framework/README.md), the framework's API guide: every name grouped by task, the rules the names don't tell you (such as drawing before the first update, or making sounds only once), and what GoLib doesn't have yet. The doc comments in `framework/*.go` have the details. Where the framework has nothing yet, a game may call raylib directly (`github.com/gen2brain/raylib-go/raylib`). Tell the user when you do, because that code should move to framework APIs as they land.

## 1. Understand the request

- Pull out the genre and core mechanic, the controls, the goal (score, win, lose), the mood and visual style, and any hints about scope.
- Ask questions only when the answer changes the game fundamentally, such as "turn-based or real-time?". Ask at most three, all in one message. Otherwise decide, and list your assumptions so the user can correct them.
- When the user doesn't say, use these defaults:

| Topic | Default |
| --- | --- |
| Window | 1280x720, resizable; F11 or Alt+Enter for fullscreen |
| Timing | 60 FPS target; movement scaled by frame time |
| Input | Keyboard (arrow keys and WASD) and gamepad 0 (d-pad or left stick, A to act, Start to pause) together; mouse when the genre needs it |
| Art | Simple shapes and a small, coherent color palette drawn in code, or sprites when the user provides art (PNG or Aseprite files). No downloaded files; files you make for the game, such as Tiled maps, `.jfxr` sounds and a tileset (see [Content](#content)), are fine |
| Audio | Sound effects for every action that needs feedback, made in code with `golib.NewSound`, from `.jfxr` files you write, or from files the user provides, both with `golib.NewSoundFile`; a sound that lasts, such as an engine, loops with `Sound.Loop`; music from a file the user provides, with `golib.NewMusic`, or from notes with `golib.NewTune` when they have no file. The game must stay fully playable muted |
| Saving | A game with a score keeps the best one with `golib.SaveData`; settings and progress too, when the game has them |
| Text | English, readable at a glance; controls shown on the title screen. The built-in font, or a font file the user provides, with `golib.NewFont` |
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

- Run `golib test <game>`, which runs `go vet` and `go test` for your game, and fix everything it reports. Before you finish, run `golib test` without a name too, which checks the framework and every game.
- Test pure logic with Go tests: collisions, scoring, level generation, state transitions. Keep that logic free of drawing calls so it stays testable.
- In a puzzle game, test that every level can be finished, with a small solver in the tests (a breadth-first search over the moves is enough for small levels), and that it can't be finished in fewer moves than any par you show. A level that can't be finished is the worst bug a puzzle game can have, and shots won't find it.
- Run the game with `golib run <name>`, and look at it with `golib shot <name> [frame...]`. It saves PNG screenshots of the given frames without opening a visible window; frame N shows the game after N updates, and 60 updates are one second. Open the files and check what they show. Add `--input` to play keys and the mouse on chosen updates: `golib shot <name> 120 --input "Enter@1 Right@10-100 Space@40"` presses Enter, walks right and jumps, `"Mouse@5:640,500 MouseLeft@6"` clicks at 640, 500, and `"GamepadLeftStick@10:1,0 GamepadA@20"` walks and jumps with a gamepad. Take shots of every scene this way, not just the first one. To reach a later state that input can't reach quickly, such as level 8 or a full trophy list, write the game's saved data into a JSON file and pass it with `--save`: `golib shot <name> --save build/<name>/level8.json` (see [docs/tooling.md](../tooling.md#screenshots)). For pixel art too small to judge, add `--scale 3`.
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
| Music | A file the user provides: a tracker module (XM, MOD) or OGG, MP3, WAV or QOA |

- Files the user provides go in the game's `assets/` folder, with paths relative to that folder: load pictures with `golib.NewSprite("sprites/player.aseprite")` or `golib.NewSpriteSheet("sprites/player.png", 32, 32)`, maps with `golib.NewMap("maps/level1.tmx")`, music with `golib.NewMusic`, and other files with `golib.ReadAsset`. Keep Aseprite files as `.aseprite`: the game reads them as they are, with their tags as animations.
- Source files the game doesn't load, such as a `.blend` file next to the `.glb` exported from it, go in the game's `sources/` folder, with the same paths as in `assets/`. Everything in `assets/` ships in the dist build; `sources/` doesn't.
- A game with an `assets/` folder also needs `assets.go` next to `main.go`, so `golib dist` embeds the folder. Copy it exactly from the `golib.EmbedAssets` documentation in `framework/assets.go`. `golib dist` stops and says so when it is missing.
- Load a list of levels, or of any other files, with `golib.ListAssets`, instead of naming each file in code, and check a map with `Map.Err` when the game should carry on without it.
- When a game has levels, make them Tiled maps instead of arrays in code, so the user can open and change them in Tiled. Tiled needs a tileset picture to show them: without art from the user, draw a small one yourself, as a PNG made by a short Go program in the game's `sources/` folder, run with `golib go run`, so the user can see how it was made, change it or replace the picture with one drawn in Aseprite. A map is XML you can write directly: see the example below and the Maps section of [framework/README.md](../../framework/README.md). Put the tileset in its own `.tsx` file, so every level shares it, and mark tiles there with properties such as `solid`, not by ID in code.
- Never build a level editor, sprite editor or other content tool, inside the game or next to it.

A level written by hand, `assets/maps/level1.tmx`, with the tileset `assets/maps/tiles.tsx` for the 16 by 16 pixel tiles of `assets/sprites/tiles.png`. Tile layer data is one tile per cell, row by row: 0 for none, else the tile's ID in the tileset plus the tileset's `firstgid`.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" orientation="orthogonal" renderorder="right-down" width="8" height="4" tilewidth="16" tileheight="16" infinite="0" nextlayerid="3" nextobjectid="3">
 <tileset firstgid="1" source="tiles.tsx"/>
 <layer id="1" name="ground" width="8" height="4">
  <data encoding="csv">
0,0,0,0,0,0,0,0,
0,0,0,0,0,3,3,0,
0,0,0,0,0,0,0,0,
1,1,1,1,1,1,1,1
</data>
 </layer>
 <objectgroup id="2" name="things">
  <object id="1" name="start" x="16" y="32">
   <point/>
  </object>
  <object id="2" name="exit" x="96" y="16" width="16" height="32"/>
 </objectgroup>
</map>
```

```xml
<?xml version="1.0" encoding="UTF-8"?>
<tileset version="1.10" name="tiles" tilewidth="16" tileheight="16" tilecount="8" columns="4">
 <image source="../sprites/tiles.png" width="64" height="32"/>
 <tile id="0">
  <properties>
   <property name="solid" type="bool" value="true"/>
  </properties>
 </tile>
 <tile id="2">
  <properties>
   <property name="solid" type="bool" value="true"/>
  </properties>
 </tile>
</tileset>
```

Keep `id`, `nextlayerid` and `nextobjectid` unique and increasing, as Tiled does, so the file opens cleanly in Tiled. Check a new map with a test that reads it (see "Testing a game" in the API guide) and with `golib shot`.

## Code organization

- Group tuning constants together, with units in the name or a comment: `playerSpeed = 240 // pixels per second`. "Make the player faster" should be a one-line change.
- Keep game state in structs you pass around, not in package-level variables.
- Keep what the game remembers between runs, such as the best score, the settings and the levels finished, in one struct with exported fields. Load it with `golib.LoadData` when the game starts, and save it with `golib.SaveData` when it changes, not in every update. Under `golib shot` and in tests nothing is written, so shots and tests start with nothing saved, unless `golib shot --save <file.json>` gives the game data to start from.
- Count waits with `golib.Timer`: `NewTimer` for a one-off, `NewRepeatingTimer` for something that happens every so often, `Tick(dt)` in `Update`, `Running` for a cooldown and `Progress` for a bar. Move numbers with `golib.Lerp`, keep them in range with `golib.Clamp`, and soften a movement with `golib.EaseIn`, `golib.EaseOut` or `golib.EaseInOut`. Base every timer, movement and animation on `dt` (always 1/60 s), never on `time.Now`. The game then plays the same on every machine and in screenshots.
- Draw for the screen size in `golib.Config`, never for the window: GoLib scales the screen to any window size and to fullscreen, and reports mouse positions in screen pixels.
- Screen effects (glow, CRT, color grading) are GLSL 330 fragment shaders in `shaders/*.fs`, embedded with `//go:embed` and run with `golib.SetPostProcess`; `golib.NewShader` documents the uniforms GoLib sets. Let the player turn them off, and check them with `golib shot`: screenshots include post-processing.
- Sound effects come from `golib.NewSound`, which makes them from a `golib.SoundSpec` in code, so there are no sound files to ship: start from the ready-made recipes (`golib.Laser`, `golib.Explosion`, `golib.Pickup`, `golib.Jump`, `golib.Hurt`, `golib.PowerUp`), and keep every sound in one file, as `games/asteroids/sounds.go` does. For a sound the user will want to tune by ear, write a `.jfxr` file instead, in `assets/sounds/`, with the settings in [framework/README.md](../../framework/README.md#sound-effects-from-jfxr), and play it with `golib.NewSoundFile`: the user opens it in jfxr (<https://jfxr.frozenfractal.com>), changes it, saves it and copies the saved file back over it. Set its `amplification` to about 30 to 40, or jfxr makes it louder than everything else, and add `assets.go`, as for any `assets/` folder. Sound files the user provides (WAV, OGG, MP3 or QOA) play with `golib.NewSoundFile` too: write where they came from, and their license, in `assets/ATTRIBUTION.md`, and even out their loudness with `sound.SetVolume`. Give the player feedback for shooting, hitting, dying and scoring. For a sound that plays over and over, such as a shot or a hit, use `sound.PlayWith(1, golib.RandomFloat(0.92, 1.1))` so each one has a pitch of its own. Screenshots are silent, so you can't check sound yourself: tell the user what to listen for.
- Music comes from a file in the game's `assets/` folder, played with `golib.NewMusic` (OGG, MP3, WAV, QOA, XM or MOD; not IT). When the user wants music but gives no file, make a tune from notes with `golib.NewTune` (see the Music section of [framework/README.md](../../framework/README.md#music)), and tell them that a tracker module or an OGG file of their own would sound better, and that only the `NewTune` line has to change. Never build a music player out of sound effects. Tracker modules are a few dozen kilobytes, so they suit a dist build. Only use music the user provides, never a file downloaded on your own: write where it came from and under which license in `assets/ATTRIBUTION.md`, and tell the user when the license is unclear. Let the player turn it off, and keep it under the sound effects with `music.SetVolume`. A game with an `assets/` folder needs `assets.go` too, or `golib dist` stops.
- Get random numbers from `golib.RandomInt` and `golib.RandomFloat`, never from `math/rand`: they start from the same seed under `golib shot`, so shots repeat. Tests that use them call `golib.SetRandomSeed` first.
- Separate updating (input, logic) from drawing. Drawing never changes game state.
- End the game with `golib.Quit()` from `Update`, for example from a Quit menu entry. No key quits on its own, not even Esc; closing the window always does.
- In `Update`, turn keys into intentions (move left, jump) and pass those to the rules. The rules then never touch `golib.Input`, and tests can call them directly.
- Split files by responsibility (player, enemies, level, UI) once a file grows past roughly 400 lines.
- Keep everything for the game inside `games/<name>/`. Don't modify `framework/` to build a game: if the framework lacks something, work around it in game code and tell the user; framework changes follow [contributing.md](../contributing.md).

## Sharing the game

`golib web <name>` builds the game for the browser and serves it on this machine, to be played in a tab; `golib shot <name> --web [frame...]` takes the same screenshots there as on the desktop, into `build/<name>/shots-web/`, so a web build can be checked the same way; and `golib dist <name> --web` makes the zip to upload to itch.io. The game's code doesn't change: shapes, sprites, maps, input, sound, shaders, font files and saved data all work there. What doesn't: `.qoa` sounds and `.xm` or `.mod` music, which browsers cannot decode, and a game that calls raylib directly, which doesn't build for the browser at all (see "Playing in a browser" in the API guide). Music is the one of those with a way out: keep the tracker module and put a render of it in `.ogg` or `.mp3` beside it, under the same name, and GoLib plays the render in a browser and the module everywhere else, with the game's code naming the module. Ask the user for that render rather than converting their music yourself, and name both files in `assets/ATTRIBUTION.md`. A game that picks its music by listing the folder must count the two as one tune and name the module (see `games/skyraid/music.go`).

### On a phone

A game published on itch.io is opened on phones, so ask the user whether theirs is meant to be played on one, and build for it from the start if it is: controls bolted on afterwards never fit. What it takes:

- **Everything reachable by tapping.** A phone has no keyboard, so "press Enter to start" is a dead end there. A tap already moves the mouse pointer and holds `MouseLeft`, so menus written for a mouse work; anything held, such as steering or thrust, needs on-screen pads read with `Input.TouchDownIn` and `Input.TouchPressedIn` (see "Touch screen" in the API guide).
- **Pads drawn only while the player is using them**, behind `golib.PlayingWithTouch`, so the one web build shows them on a phone and not on a computer, where they would be in the way. It follows the player: a finger turns it on, a key or the mouse turns it off. `games/asteroids` is the reference: its pads and buttons are in `touch.go`, the scenes read them alongside the keys, and F4 chooses instead of the automatic answer.
- **Room for the thumbs.** Put the pads inside the screen, not against its edges, in the bottom corners, and keep the middle clear of anything the player has to see while a thumb is on it. A pad smaller than about 120 by 120 screen pixels is hard to hit.
- **A way into fullscreen.** A browser only grants fullscreen while it handles a tap, so a game for a phone offers a button for it; `golib.IsFullscreen` follows the player when they leave it with a gesture.
- **Check it with shots and then on a real phone.** `golib shot <name> --input "Touch@40-90:200,600"` puts fingers on the screen and draws the pads, and `golib web <name> --lan` serves the game to the network so the user can open it on their own phone.

`golib dist <name>` builds the game for players into `build/<name>/dist/`: a folder, `<name>/`, with the executable (its assets inside), the raylib libraries it loads and `THIRD-PARTY-LICENSES.txt`, and a zip of that folder, such as `<name>-1.0.0-windows-amd64.zip`, ready to share, for example on itch.io. Players unzip it and start the executable, which needs the files next to it. Debug builds from `golib run` and `golib build` aren't meant for players, so don't hand those out. Play the dist build once before sharing it, with `golib run <name> --dist`: it carries its assets inside the executable, where a debug build reads them from the game's folder, so a file the game forgot to embed only goes missing there. When the user wants to share the game, run `golib dist` and tell them where the zip is.

`THIRD-PARTY-LICENSES.txt` holds the licenses that ask to go with the game, and copies the game's `assets/ATTRIBUTION.md`, so write down the source and license of every file in `assets/` that wasn't made for the game there. If the user is going to publish the game, point them to the License section of [README.md](../../README.md#license): the file is a best effort, not legal advice.

On Windows, the executable also carries what players see in Explorer, the title bar and the taskbar:

- `game.json`, which `golib new` writes, holds the title, version and author. Keep `title` the same as `Config.Title`, fill in `author` when the user says who they are, and raise `version` (major.minor.patch) each time the user shares a new build. [docs/tooling.md](../tooling.md#icon-and-version-information-windows) lists every field.
- `icon.png`, next to it, is the icon: a square PNG, ideally 256 by 256 pixels, transparent around the shape. Ask the user for one, for example drawn in Aseprite, when they want to share the game; never download one. Without it, Windows shows its default icon.

`golib dist` prints what it used. When either file has a mistake, `golib build`, `run`, `shot` and `dist` stop with a `[fail]` line that says what to fix.

## Quality checklist

Before calling a game done:

- [ ] Starts with `golib run <name>`, with no errors in the output.
- [ ] Title screen shows the game's name and its controls.
- [ ] The core loop plays for several minutes without crashes or soft-locks.
- [ ] Scoring, damage and failure give clear feedback.
- [ ] Pause and resume work; restart after game over works without relaunching.
- [ ] Closing the window exits cleanly.
- [ ] Fullscreen switches on and off, and the game looks right in a resized window.
- [ ] If the game is meant for phones: every scene can be worked with fingers alone, the pads are only drawn where there is a touch screen, and shots with `Touch@` items show them.
- [ ] `golib dist <name>` succeeds, and `game.json` has the game's title and current version.
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
