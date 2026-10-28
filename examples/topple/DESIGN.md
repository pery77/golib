# Topple

## Pitch
You stand in a small yard full of wooden crates and barrels stacked on stone platforms, and knock them all down with balls you throw with the mouse. Three yards; the fewer balls, the better.

## Core loop
Aim at the base of a stack, throw, watch it topple; walk round for a better angle when a stone block or another stack is in the way; push the crates on the ground out of your path.

## Controls
| Input | Action |
| --- | --- |
| Mouse | Aim: the dashed arc shows where the ball goes |
| Left click, Space | Throw a ball at the crosshair |
| Arrows, WASD | Walk (forward is towards the stacks: left and right walk round them); walk into crates to push them |
| Gamepad | Left stick walks, right stick moves the crosshair, A or the right trigger throws, Start pauses |
| Esc, P | Pause: Esc carries on, R tries the level again, Q goes back to the title |
| 1, 2, 3 on the title | Start at a yard already reached |
| F11, Alt+Enter | Fullscreen |

## Rules
- Every crate and barrel that starts on a platform is a target. A target is down once it is no longer over a platform's top and has nearly stopped (`world.isDown`). The level is cleared when every target has been down for a second.
- Crates, barrels and balls are rigid bodies in a `golib.PhysicsWorld`; the platforms, the ground and the fence are still meshes; the stone blocks are rigid bodies of 6000 kg, which nothing in the yard moves. The player is a `golib.CharacterController`, which pushes what it walks into.
- A throw goes from the player's hand to the point under the crosshair, in an arc, in the time a throw at `throwSpeed` takes (at most `throwMaxTime`).
- Score: the balls thrown on each level, against its par. 3 stars at or under par, 2 up to two over, 1 otherwise. The game remembers the fewest balls for each level and for a whole game from the first yard (`record` in `scenes.go`, saved as `record`), and the furthest yard reached.

## Screens
Title (over the first yard, the camera going round), play, pause, cleared (stars and balls, over the yard), won (every yard with its balls and stars, and the total). `scenes.go` has them all.

## Levels
`assets/levels/level01.json` to `level03.json`, played in name order. Each gives a name, a par, where the player starts and what the camera keeps in view (`focus`), the platforms (middle, width, height, depth) with stacks on them (each layer a row of `C` for a crate and `B` for a barrel, from the bottom up, along `x` or `z`), the stone blocks and the loose props on the ground. The yard runs from -10 to 10 meters across X and Z. `world_test.go` checks that every level loads, that nothing starts inside anything else, that the stacks stand on their own, and that a simple player who throws at the lowest target it can see clears each one (9, 13 and 28 balls at the last count: the pars, 5, 8 and 12, assume a player who aims at the bottoms of the stacks).

## Tuning
At the top of `world.go`: walking speed, the sizes and masses of the crates (10 kg), barrels (9 kg), balls (5 kg) and stones, the throw's speed and cooldown, how long balls stay, and when a target counts as down. The camera is at the top of `controls.go`; the sounds are in `sounds.go`, the colors and materials in `models.go` and `draw.go`.

## Known limits
- At most `maxBalls` (4) balls stay in the yard, and a ball that has stopped rolling is taken away after a second, which keeps the yard readable.
- `Sound.PlayAt` has no volume or pitch of its own, so soft knocks are a quieter sound of their own (`woodSoftSound`).
- Nothing reports which bodies touched: hits are heard where a body's speed changes suddenly between updates (`world.listen`).

## Later
- A ball that bounces off the fence back into play, or explosive barrels.
- More yards, and moving platforms (a `Moved` rigid body).
- Sounds for the player's steps.

## Changelog
- 2026-10-24: created with `golib new`, then built: the yard, three levels from JSON files, rigid crates, barrels, balls and stone blocks, throwing with the mouse along a shown arc, walking and pushing with a `CharacterController`, hit sounds from where things hit, stars against par, best scores saved, title, pause, cleared and won scenes.
