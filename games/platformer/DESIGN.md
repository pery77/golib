# Platformer

## Pitch
Run and jump through a forest at night, slash the snakes and open every chest. It is GoLib's example game: it tests each framework feature as it lands and shows how to use it.

## Core loop
Walk, jump onto a higher ledge, open the chest there, slash the snake in the way, and don't fall into the water.

## Controls
| Input | Action |
| --- | --- |
| Left / Right arrows, A / D, d-pad or left stick | Walk; the stick walks slower when tilted less |
| Space, Up arrow, W or the A button | Jump (only while standing on something) |
| X, J or the X button | Slash in front of the hero |
| Esc or Start | Pause and resume while playing |
| Up / Down arrows, d-pad, mouse wheel or pointer | Choose a button on the title |
| Enter, A, Start or a click | Confirm the title's button |
| Esc | Quit, on the title |
| Q or B | Quit to the title, while paused |
| Enter or A / Esc or B | Play again / back to the title, after winning |
| Closing the window | Quit |

Gamepad controls read gamepad 0, the first one connected.

## Rules
- Opening all 5 chests wins. Touching a chest opens it.
- Snakes crawl back and forth. Touching one, falling into the water or falling off the level puts the player back at the start. Opened chests stay open, and defeated snakes stay defeated.
- A slash defeats the snakes in front of the hero while it lasts, 0.3 seconds.
- There is no way to lose and no timer yet.

## Level
One Tiled map, `assets/maps/forest.tmx`, 64 by 12 tiles of 16 pixels, with its tileset in `assets/maps/forest.tsx`. Open them in Tiled to change the level; the game reads them as Tiled saves them.

| Layer | Holds |
| --- | --- |
| `far` | Trees in the distance: a parallax factor of 0.5 and a blue tint |
| `back` | A tree, a bush, signs and a fence, behind the hero |
| `ground` | The ground, the ledges, a crate and the water. The tileset marks tiles with a bool property: `solid` for the ground and the crate, `water` for the water |
| `chests` | The chests, as tile objects of class `chest` |
| `things` | The point named `start`, where the hero's feet start, and points of class `snake`, where each snake starts crawling right, with a float property `distance`, in pixels |
| `front` | Grass, flowers and stones, in front of the hero |

The ledges are three tiles, 48 pixels, above what the hero jumps from; a jump rises about 52. The map's parallax origin, 160, 90, is the middle of the screen when the camera is at the level's top-left corner, so the far trees start where Tiled shows them.

## Screens
Each screen is a scene in `scenes.go`:
- **Title:** the level, darkened, under the game's name, its controls, a Play / Quit menu that works with the keyboard, the mouse (pointer and wheel) and a gamepad, and the name of the connected gamepad.
- **Play:** the level, with a camera that follows the hero, clouds that drift slower than the camera, and the count of opened chests. Each play gets its own random clouds.
- **Pause:** the level, frozen, under a "Paused" message.
- **Won:** the finished level under a "You win!" message.

There is no game over screen, because there is no way to lose.

## Art
The screen is 320 by 180 pixels, scaled up by whole numbers (`Config.PixelArt`). The pictures are the sprite sheets of "A platformer in the forest" (CC0, by Buch; see `assets/ATTRIBUTION.md`): `sheet.png`, the tileset, which also gives the chests; `characters.png`, 32 by 32 frames, for the hooded hero and the snakes; and `swoosh.png`, the slash. `art.go` names their frames and animations. `icon.png` is the hero's first frame. Text uses GoLib's built-in font: the game has no font file.

## Sounds
GoLib makes every sound in code, so the game ships no sound files. `sounds.go` holds them: GoLib's recipes for a jump, a chest, a fall back to the start, a defeated snake and winning, and a slash made from a `golib.SoundSpec`.

## Tuning
In `world.go`: `moveSpeed`, `jumpSpeed`, `gravity` and `maxFallSpeed` at the top, the hitbox sizes, the slash and the snakes' speed below them. The level is the map. The clouds are in `draw.go`, the colors and the screen size in `main.go`, and the frames and animations in `art.go`.

## Later
Follows the framework, one feature at a time:
- Music, and sound effects from files, when the user provides them.
- A font, when the user provides one.
- Climbing the ladder, with the sheet's climb animation.
- More levels, as more Tiled maps.

## Changelog
- 2026-09-15: first version, drawn with shapes: walking, jumping, platforms, coins, falling off, winning. Replaces `games/hello`.
- 2026-09-15: Esc no longer quits during play, because GoLib now leaves every key to the game. After winning, Esc quits through `golib.Quit`.
- 2026-09-15: title, pause and won scenes, switched with `golib.SwitchScene`. Esc now pauses, and quits from the title.
- 2026-09-15: Play and Quit buttons on the title that work with the mouse, and random clouds with `golib.RandomFloat`.
- 2026-09-15: gamepad controls everywhere, and a title menu that also works with the arrows, the d-pad and the mouse wheel.
- 2026-09-16: sounds for jumping, collecting a coin, falling into the gap and winning, made in code by GoLib (`sounds.go`).
- 2026-09-16: `game.json` (title, version 0.1.0, author) and a placeholder `icon.png`, which `golib dist` puts in the Windows executable.
- 2026-09-16: `assets/` with the sprite sheets of "A platformer in the forest" (CC0, by Buch; see `assets/ATTRIBUTION.md`), not drawn yet because GoLib can't load images until M4, and `assets.go`, so `golib dist` embeds the folder.
- 2026-09-17: version 0.2.0, drawn with the forest art on a 320 by 180 pixel art screen. The level is a Tiled map that scrolls, with a far layer in parallax. Coins became chests, and the gap became water; snakes crawl on the ground, and the hero slashes them. The hero walks, jumps and slashes with the sheet's animations. `icon.png` is now the hero.
