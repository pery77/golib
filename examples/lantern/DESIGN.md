# Lantern

## Pitch
A night in a flooded temple. A hooded explorer with a lantern walks three torch-lit stone halls, wades round a moonlit pool under a broken roof, and takes six glowing gems; each leaves a rune on the floor, and the six open the gate out.

## Core loop
Walk through hazy, flickering halls, spot the next glowing gem, reach it (around pillars, through the water), watch its rune light up on the floor.

## Controls
| Input | Action |
| --- | --- |
| Up/Down, W/S, left stick | Walk forward and back, away from and towards the camera |
| Left/Right, A/D, right stick | Turn the view; the explorer walks where it looks |
| Q/E, d-pad left/right | Step sideways |
| Esc, Start | Pause (Esc or Enter goes on, T to the title) |
| Enter, Space, A | Start, on the title |
| F11, Alt+Enter | Fullscreen |

## Rules
- Six gems float in the halls: two in the hall of pillars, two in the flooded hall (one over the pool), two in the hall of the gate. Walking into one takes it; its rune glows on the floor under where it was, and a socket on the gate lights in its color.
- The sixth gem sinks the gate into the floor; walking through it to the exit wins. The time it took shows.
- In the pool the explorer wades, slower. Nothing can hurt them.

## Screens
Title (the flooded hall turning slowly), playing, paused, escaped.

## Look
- The level is built by `sources/models/level.py` in Blender and its light baked by `tools/blender/bake_lightmaps.py`: moonlight and the night sky through the roof's gaps, the torches' bounce, light probes for the explorer and gems, and a reflection probe per hall for the polished floors.
- Eight torches, Blender point lamps read with `Model.Lights`, flicker (`flicker` in `draw.go`) and cast shadows; fire, smoke and sparks are particles.
- Haze shows the moon's shafts through the roof and the torches' glow; the pool is `DrawWater`; the marble floors reflect the torches (screen-space reflections, roughness 0.12 in Blender).
- Each torch has a small solid flame core under its particles, so the floor and water reflect it: particles aren't in screen-space reflections.
- Gems: toon materials, three bands, a black outline 6 pixels wide, inked over the haze, a little emission and a small colored lamp each.
- Runes: glowing decals, `assets/textures/rune.png` painted by `sources/textures/rune.go`.

## Tuning
- `world.go`: walk 3.6 m/s, wade 1.9 m/s, gem reach 1.1 m, gate 2.5 s.
- `controls.go`: camera 4.4 m behind, -16 degrees, turn 110 degrees/s.
- `draw.go`: haze 0.04, exposure 1.25, bloom 0.7, torch flicker 28 %, torch reach 10 m, gem lamps 0.5 over 3 m.
- `sources/models/level.py`: the halls, the pool, the roof's gaps, the torches (50 W), the moon (strength 7.5), the gems and the start.

## Later
- A modelled, animated explorer from Blender instead of shapes.
- Torch crackle heard from each torch with `Sound.LoopAt`, music from notes.
- Something that wanders the halls.

## Changelog
- 0.1.0: first version, from one prompt.
