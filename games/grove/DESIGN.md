# Grove

## Pitch
A small 3D race against the clock: Kenney's little astronaut crosses a forest island of grass blocks to the flag at its far end, picking up coins and punching rocks open for more.

## Core loop
Walk or sprint up the island, jump onto the higher blocks, grab the coins on the way (each adds time), punch a rock three times to burst its coins out, and reach the flag before the clock runs out.

## Controls
| Input | Action |
| --- | --- |
| Arrows, WASD, d-pad, left stick | Walk, from where the camera looks; the stick walks as far as it is tilted |
| Shift, gamepad B or RT (held) | Sprint |
| Space, gamepad A | Jump (one block high) |
| J, K, left click, gamepad X | Punch; it turns you to a rock close by |
| Q and E, right stick | Turn the camera |
| Esc, P, Start | Pause (Q or B there: back to the title) |
| F11, Alt+Enter | Fullscreen |

## Rules
- The clock starts at 60 seconds. Each coin adds 1.5 seconds and 100 points.
- A rock breaks open after 3 punches, and 3 coins fly out of it.
- Falling into the sea costs 3 seconds; the astronaut comes back on the last ground it stood on.
- Reaching the flag wins: the score is the coins' points plus 20 points for every second left. The best score is saved.
- When the clock reaches 0 the astronaut falls over: time's up.

## Screens
Title (the island from above, the controls), playing (coins, clock, a marker over the flag, "Reach the flag!" at the start), paused, the end (the camera swings round to the astronaut: cheering at the flag with the score, or lying in the grass when time is up).

## Look
A sunny morning: a sky that fades to a pale horizon, a sun from the left that casts shadows, a rippling sea around the island, a little bloom and far fog. Everything on the island is a model from the FBX pack: grass blocks stacked one or two high, round trees and pines, flowers, rocks, coins and the flag.

## Animation
`poses.go` picks what the astronaut's whole body plays from its state: `idle`, `walk` or `sprint` at the speed it moves, `jump` and `fall` in the air, `emote-yes` at the flag, `die` when time is up. Punches (`punch`) and pick-ups (`pick-up`) play over it on the `torso` bone. The pack's animation files each carry Blender's whole 250-frame timeline, so golib sees every one as 10.375 seconds long; `models.go` keeps their real lengths and loops and ends them itself.

## Level
`assets/levels/island.txt`, a text file: one character a cell, the sea, grass 1 to 3 blocks high, and letters for the start, the flag, coins, rocks, trees and flowers. `island_test.go` checks that the flag and every coin and rock can be reached.

## Tuning
- Movement, jump, clock, coins, punches and rocks: the constants at the top of `world.go` (walk 2.1 m/s, sprint 5.2 m/s, jump 1.35 m, 60 s on the clock, 1.5 s a coin, 3 punches a rock).
- Animation paces: `poses.go`; the clips' real lengths: `models.go`.
- Sizes of trees, rocks, coins and the flag: `island.go`.
- Camera: `controls.go` (5.2 m back, 24 degrees down, closer and steeper behind a high block).
- Look: `draw.go`; colors: `main.go`.

## Later
- More islands, listed with `golib.ListAssets`.
- Music, from `golib.NewTune`.
- A ghost of the best run.

## Changelog
- 0.1.0: first version: the island, walking, sprinting, jumping, coins, rocks to punch open, the clock, the flag, the title, pause and end screens.
