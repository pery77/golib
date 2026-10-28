# Crypt

## Pitch
A small 3D sneaking game: Kenney's little astronaut explores a torch-lit crypt of halls and corridors, breaks crates and climbs to a gallery for three keys, rides a stone lift, and slips past skeleton guards to the way out.

## Core loop
Look over a room, wait for the guard to turn its back, run to the next crate, key or door. Break crates (one in each of two rooms hides a key), climb the great hall's stairs to its gallery for another, and use each key on a locked door, which crumbles away. A guard that sees you shrieks and chases you round the walls and pillars; lose it by getting out of its sight, or it grabs you and you start again at the last checkpoint with a heart less.

## Controls
| Input | Action |
| --- | --- |
| Arrows, WASD, d-pad, left stick | Run, from where the camera looks (the stick runs as far as it is tilted) |
| Left click on a crate | Break it, if the astronaut is close enough (a crate in reach lights up under the pointer) |
| F, J, Space, gamepad X or A | Break the crate in reach, nearest first |
| Q and E, right stick | Turn the camera |
| Esc, P, Start | Pause (Q or B there: back to the title) |
| F11, Alt+Enter | Fullscreen |
| Title: Enter or A | New game; C or Y carries on from the last checkpoint |

## Rules
- Three keys: one in a crate in the entry hall, one on the great hall's gallery, one in a crate by the lift. Each opens one locked door; the way out needs all three found.
- Three hearts. A guard that reaches you grabs you: a heart less, and you start again at the last checkpoint reached, with the guards back on their rounds. The last heart ends the game; you can try again from the checkpoint.
- Guards see 9 m ahead, 60 degrees either side of where they face, and sense you 1.5 m away whichever way they face; walls, pillars, crates and doors hide you. They chase along the crypt's paths, a little slower than you run, and give up 3.5 s after losing sight of you.
- The lift waits 2.5 s at each floor and travels 3.76 m in under 3 s; it waits on its way down while anybody stands under it.
- Checkpoints: the entry hall, the corridor to the great hall, the corridor to the lift hall, the corridor to the exit chamber. Reaching one saves the game.
- The best escape time is saved.

## Screens
Title (the entry hall, the story and controls, carry on), playing (hearts, keys, room, clock, "!" over a guard that has seen you, messages, a prompt by a crate), paused, game over, and the win screen (the astronaut cheering in front of the way out, the time).

## Level
`assets/models/level.glb`, made in Blender by `sources/models/level.py`, which also paints the textures. Its numbers (rooms, doors, crates, guards' rounds, torches, the lift) are at the top of the script: change them and run it again (the command is at the top of the script). Empties with a custom property `kind` tell the game where things go; `level.go` reads them. Doors, crates, the lift, keys, flames and the way out are drawn by the game, so it can move and remove them. Anything that must stop a character is at least 0.6 m tall: golib's CharacterController steps up ledges as high as its StepHeight plus about its radius.

## Tuning
- Running, punching, keys, doors, the lift, hearts: the constants at the top of `world.go` (run 3.6 m/s, punch reach 0.9 m, lift 1.3 m/s).
- Guards: the constants at the top of `guards.go` (patrol 1.3 m/s, chase 3.0 m/s, sight 9 m and 60 degrees).
- Camera: `controls.go` (3.8 m behind, 26 degrees down, 72 degree view, steeper when walls crowd it).
- Light: `draw.go` (moonlight, fog, torches' strength and reach); colors: `main.go`.

## Checking it
`script_test.go` plays the game from a `golib shot --input` script exactly as a shot plays it, and `TestThePlaythroughWins` plays `playthrough` from the title to the way out: a wall slide, a crate broken with a click, three doors, a guard chasing round a pillar and grabbing the astronaut, the stairs, the ramp, the lift and the win. The shots: `golib shot crypt <frames> --input "<playthrough>"` with frames 200 (sliding along the wall), 381 and 392 (the click and the crate breaking), 600 (a door crumbling), 1150 (the guard rounding a pillar), 1325 (grabbed), 1960 and 2080 (the stairs), 2297 (the gallery's key), 2560 (the ramp), 3300, 3480 and 3600 (the lift), 3750 (the third door), 3968 and 4100 (the win); add `--web` for the same in a browser.

## Later
- Guards that hear footsteps when the astronaut runs close; a walk that is quieter.
- More floors, a second crypt listed with `golib.ListAssets`.
- Music from `golib.NewTune`: a slow, low drone.

## Changelog
- 0.1.0: first version: the crypt in Blender, three keys and doors, crates, the lift, three guards, checkpoints and hearts, title, pause, game over and win screens.
