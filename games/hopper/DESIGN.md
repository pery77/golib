# Hopper

## Pitch
A small 3D platformer: a round little astronaut from Kenney's Platformer Kit hops across grass blocks floating in the sky, picks up the gold coins on the way, and runs for the flag at the far end, as fast as they can, with three lives.

## Core loop
Every couple of seconds: line up the next block, jump, steer in the air, land. Coins hang over the blocks and over the gaps, so the best line picks them up; a fall costs a life and puts the character back on the last checkpoint.

## Controls
| Input | Action |
| --- | --- |
| Arrows or WASD, d-pad or left stick | Run: up is away from the camera, left and right are the screen's |
| Space, or A on a gamepad | Jump (hold it for a higher jump) |
| Q and E, or the right stick | Turn the camera |
| Esc, P or Start | Pause |
| Enter, Space, A or a click | Start, play again |
| F11 or Alt+Enter | Fullscreen |

The camera stays behind the character: while it runs away from the camera, the camera swings round behind the way it runs.

## Rules
- The level is `assets/models/level.glb`, made in Blender (see "The level in Blender" below). Every block, tree and rock marked `kind = block` is solid, as the box around its kit model: the character stands on its top and is stopped by its sides. Flowers, grass and the far islands (`kind = scenery`) are only to look at.
- Coins (`kind = coin`) are picked up by touching them. The count shows in the top-left corner, as `12 / 20`, with the time since the start.
- Falling below the level costs a life: the character starts again on the last checkpoint it touched (`kind = checkpoint`, a small flag), or on `Start`. Coins already picked up stay picked up, and the clock keeps running.
- Three lives. When the last one goes, the game is over.
- Touching the big flag at the end (`kind = goal`) wins: the time and the coins show, and the best time (and the most coins) are saved.

## Screens
Title (the level, seen from above, turning slowly; the controls), playing (coins, time and lives in the top-left corner), paused, won (time, coins, best), game over (play again or go back to the title).

## The level in Blender
- `sources/models/level.py` builds the level from scratch, saves `sources/models/level.blend` and exports `assets/models/level.glb`. Run it once again only to start over: it overwrites the `.blend`.
- To change the level, open `sources/models/level.blend` in Blender. The Level scene holds the level; the Kit scene holds one collection per kit model, which the level's blocks instance.
  - Move, turn (in 90 degree steps for solid blocks) or Shift+D blocks, coins and checkpoints as you like. A new block: Add > Collection Instance > one of the `block-*` collections (the Kit scene has a few the level doesn't use yet, such as `block-grass-hexagon`). Don't use the collections' instance offset: Blender's export moves the instancing empty by it, and the game reads the empty's place.
  - Coins, checkpoints and the start are plain empties: Shift+D a coin to make another.
  - Then export: in the Scripting tab, open the text `export_level.py` and press Run Script; or from a terminal, `Blender --background sources/models/level.blend --python sources/models/export_level.py`. The script marks every instance of a `block-*`, `tree*` or `rocks` collection `kind = block` (unless it already has a `kind`, such as `scenery`, which is not solid) and names its kit model in `model`, then exports with the options below.
  - By hand instead, File > Export > glTF 2.0: glTF Binary, Include > Custom Properties and Active Scene ticked, +Y Up ticked, Compression off, to `games/hopper/assets/models/level.glb`. Every solid block then needs the custom properties `kind = block` and `model = <its collection's name>`.
- Another kit model: copy its `.glb` from the kit's `Models/GLB format/` into `assets/models/kit/`, add its name to `KIT_MODELS` in `level.py`, and either run `level.py` again (which starts the level over) or import it into the Kit scene by hand, into a collection of the same name.
- `level_test.go` checks the exported level: it has a start, a goal and coins on blocks, the flag can be reached by jumps the character can make, with room to spare, and every coin hangs within reach of a jump.

## Tuning
All in `world.go`, at the top: run speed 5.5 m/s, ground acceleration 45 m/s², air acceleration 22 m/s²; jump speed 10 m/s under gravity of 30 m/s², so a full jump rises about 1.65 m and crosses about 3.5 m; releasing the jump early cuts the rise; 0.1 s of coyote time and 0.12 s of jump buffer. The camera's distance, pitch and how fast it swings round are at the top of `controls.go`.

## Later
- The character's walk and jump animations, once GoLib plays glTF animations (M7, stage 4): for now it is drawn in its rest pose, squashed and stretched by code.
- Real shadows (M7, stage 3) instead of the dark disc under the character.
- Moving blocks, springs and spikes from the kit; more levels, listed with `golib.ListAssets`.

## Changelog
- 0.1.0: first version: the Blender level, running and jumping, coins, checkpoints, lives, the timer, title, pause, won and game over.
