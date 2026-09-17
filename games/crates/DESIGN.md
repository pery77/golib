# Crates

## Pitch
A Sokoban puzzle game: push every crate onto a goal in a small warehouse room. Ten levels go from one crate to five, a quiet chiptune plays in the background, and the game remembers how far you got.

## Core loop
Look at the room, plan a few pushes, walk and push, undo when a crate gets stuck, and finish the room, ideally in par moves.

## Controls
| Input | Action |
| --- | --- |
| Arrows, WASD, d-pad or left stick | Walk; walking into a crate pushes it. Hold to keep walking |
| Z, U, Backspace or the B button | Undo the last move; hold to keep undoing |
| R or the Y button | Restart the level |
| Esc or Start | Pause menu, while playing |
| M or the Back button | Music on and off, in every scene |
| F11 or Alt+Enter | Fullscreen on and off, in every scene |
| Arrows, WASD, d-pad, stick, wheel or pointer | Choose in menus and the level list |
| Enter, Space, A, Start or a click | Confirm in menus and the level list |
| Esc or B | Back: resume from the pause menu, the level list from the complete screen, the title from the level list |
| Esc, on the title | Quit |
| Closing the window | Quit |

Gamepad controls read gamepad 0. The title and the play screen show the gamepad's controls while one is connected.

## Rules
- A crate moves only when pushed, one cell, and only onto free floor or a goal: never into a wall or another crate. Crates can't be pulled.
- The level is complete when every crate is on a goal. A level has as many goals as crates.
- Moves count every step, pushes included; undo takes a move back. Each level has a par, the fewest moves that finish it; finishing in par moves earns a star. Over par, the move counter turns amber.
- Levels unlock in order: finishing a level unlocks the next. Finished levels can be replayed from the level list, which shows each one's best moves.
- Difficulty grows with the number of crates (1, 1, 2, 2, 2, 3, 3, 4, 4, 5) and with par (3 to 68 moves); `levels_test.go` checks both always grow, and that every level can be finished in exactly its par.

## Progress
The game saves the fewest moves for each finished level, and whether the music is on, with `golib.SaveData` under the name `progress`, after every finished level and every music switch (`session.go`). The title's first button continues from the first unfinished level, and the title shows how many levels are finished. If the progress can't be written, the title says so, the error goes to the console, and the game goes on.

- `golib run` and F5 save in `build/crates/save/progress.json`, so making the game writes nothing outside the project. `golib clean` deletes it, and the progress with it.
- `golib dist` builds save in the player's settings folder: `%AppData%\GoLib games\crates\` on Windows.
- Under `golib shot` and in tests nothing is written, so every shot and test starts with no progress.

## Screens
- Title: the name, a menu (Continue or Play / Level list / Music / Quit), the controls, and how many levels are finished.
- Level list: a grid of the ten levels; locked ones are dimmed, finished ones show their best moves, and a star at par. Below it, the selected level's name, par and best.
- Play: the room in the middle; above it, the level's number and name, the moves and the par; below it, the crates on goals and the controls.
- Pause (Esc or Start): Resume / Restart level / Level list / Music / Title screen, over the frozen room.
- Level complete: moves, pushes and par, a star at par, then Next level / Replay / Level list.
- End, after the last level: how many levels were finished at par, and back to the title.

## Content
- Levels are Tiled maps, `assets/maps/level01.tmx` to `level10.tmx`, with 16 by 16 pixel tiles, at most 20 by 9 cells, sharing the tileset `assets/maps/tiles.tsx`.
  - The layer `floor` holds tiles of class `floor`, `goal` and `wall`. Cells with no tile are outside the room.
  - The layer `things` holds tiles of class `crate`, and one of class `start`, where the player starts. The game draws the crates and the player itself, so this layer only matters in Tiled.
  - The map's custom properties are `title` (string) and `par` (int).
  - The levels are every `.tmx` file in `assets/maps/`, played in the order of their names, which is why they are numbered: `levelFiles` in `levels.go` finds them with `golib.ListAssets`, so saving a new map there adds a level and nothing else changes. `levels_test.go` solves every level it finds, and says the right par when a map's is wrong.
- `assets/sprites/tiles.png` is the only picture, drawn for this game: 8 by 2 tiles of 16 by 16 pixels. Top row: floor, wall, goal, crate, crate on a goal, the start (the player facing down), cracked floor, cracked wall. Bottom row: the player facing down, up, left and right, standing and mid-step. Edit it in Aseprite, keeping the grid; `art.go` names the frames.
- Sound effects are recipes in `sounds.go`, except the level-complete fanfare, `assets/sounds/complete.jfxr`, which opens in jfxr (<https://jfxr.frozenfractal.com>).
- The music is a tune written in `music.go` as notes for `golib.NewTune`: a lead, a bass and a soft tick, 16 bars of 8 beats that loop, about 34 seconds, so the game needs no music file. It is an ordinary `golib.Music`, which `session.go` plays and pauses with the music switch. To use a music file instead, see the comment at the top of `music.go`.
- Every file in `assets/` was made for this game, so `assets/ATTRIBUTION.md` isn't needed. Add it for any file that comes from elsewhere.

## Tuning
- `play.go`, top: how fast the player and a pushed crate slide (`slideTime`), the delay and pace of walking and undoing while a key is held (`repeatDelay`, `repeatTime`), the glow of a crate that lands (`flashTime`), and the pause before the complete screen (`solvedDelay`).
- `controls.go`, top: how far the stick tilts before it counts (`stickTilt`).
- `music.go`, top: tempo (`tuneTempo`, in eighth notes) and the volume of each voice.
- `sounds.go`: every sound effect's recipe.
- `main.go`: the screen size and the colors. `draw.go` and `scenes.go`: where things go on the screen.

## Later
- More levels, and more rooms per crate count; a larger screen for rooms over 20 by 9 cells.
- A push counter in the par, or a second par for pushes.
- Mouse control in play: click a cell to walk there.
- A music file of the player's choice, and an icon (`icon.png`) for `golib dist`.

## Changelog
- 2026-09-17: created with `golib new`, then built as a Sokoban game: 10 Tiled levels checked by a solver, undo and restart, par and stars, a level list, saved progress, sound effects and a background tune made in code, pixel art tiles drawn for the game.
- 2026-09-17: progress is saved with `golib.SaveData` (GoLib M6), instead of the game's own files and build tags; the save folder is now `GoLib games\crates` in the player's settings folder.
- 2026-09-17: the game moved onto three new pieces of GoLib: the tune is `golib.NewTune` notes instead of the game's own note-by-note jukebox, the level list comes from `golib.ListAssets("maps")` instead of a hard-coded list, and a level that can't be read is reported by `(*golib.Map).Err`. The game lost about 150 lines, 110 of them the jukebox in `music.go`.
