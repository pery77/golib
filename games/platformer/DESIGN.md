# Platformer

## Pitch
Run and jump across a single screen of platforms and collect every coin. It is GoLib's example game: it tests each framework feature as it lands and shows how to use it.

## Core loop
Walk, jump onto a higher platform, grab the coin there, and keep going without falling into the gap.

## Controls
| Input | Action |
| --- | --- |
| Left / Right arrows, A / D, d-pad or left stick | Walk; the stick walks slower when tilted less |
| Space, Up arrow, W or the A button | Jump (only while standing on something) |
| Esc or Start | Pause and resume while playing |
| Up / Down arrows, d-pad, mouse wheel or pointer | Choose a button on the title |
| Enter, A, Start or a click | Confirm the title's button |
| Esc | Quit, on the title |
| Q or B | Quit to the title, while paused |
| Enter or A / Esc or B | Play again / back to the title, after winning |
| Closing the window | Quit |

Gamepad controls read gamepad 0, the first one connected.

## Rules
- Collecting all 5 coins wins.
- Falling into the gap puts the player back at the start. Collected coins stay collected.
- There is no way to lose and no timer yet.

## Screens
Each screen is a scene in `scenes.go`:
- **Title:** the game's name, its controls, a Play / Quit menu that works with the keyboard, the mouse (pointer and wheel) and a gamepad, and the name of the connected gamepad.
- **Play:** the level, with the coin count.

The title and each play get their own random clouds in the sky.
- **Pause:** the level, frozen, under a "Paused" message.
- **Won:** the finished level under a "You win!" message.

There is no game over screen, because there is no way to lose.

## Sounds
GoLib makes every sound in code, so the game ships no sound files. `sounds.go` holds them, one line each, from GoLib's ready-made recipes: a jump, a coin, a fall back to the start and a fanfare for winning.

## Tuning
All in `world.go`: `moveSpeed`, `jumpSpeed`, `gravity` and `maxFallSpeed` at the top, `cloudCount` below them, and the level layout in `newWorld`. Colors are in `main.go`, sounds in `sounds.go`.

## Later
Follows the framework, one feature at a time:
- Sprites and animations drawn in Aseprite instead of rectangles.
- Music, when GoLib loads audio files.
- The level as a Tiled map.
- An icon drawn in Aseprite, instead of the placeholder `icon.png`, which was drawn in code.

## Changelog
- 2026-09-15: first version, drawn with shapes: walking, jumping, platforms, coins, falling off, winning. Replaces `games/hello`.
- 2026-09-15: Esc no longer quits during play, because GoLib now leaves every key to the game. After winning, Esc quits through `golib.Quit`.
- 2026-09-15: title, pause and won scenes, switched with `golib.SwitchScene`. Esc now pauses, and quits from the title.
- 2026-09-15: Play and Quit buttons on the title that work with the mouse, and random clouds with `golib.RandomFloat`.
- 2026-09-15: gamepad controls everywhere, and a title menu that also works with the arrows, the d-pad and the mouse wheel.
- 2026-09-16: sounds for jumping, collecting a coin, falling into the gap and winning, made in code by GoLib (`sounds.go`).
- 2026-09-16: `game.json` (title, version 0.1.0, author) and a placeholder `icon.png`, which `golib dist` puts in the Windows executable.
