# Sunset Lagoon

## Pitch

A little metal hovercraft skims a calm lagoon at sunset, collecting glowing
crystals that float over the water before a 90-second timer runs out.

## Core loop

Spot the nearest crystals, swing the hovercraft round and drift through them,
weaving between the islets and the old mooring poles. Every few crystals a
golden one appears for a short while: worth three, if you reach it in time.

## Controls

| Input | Action |
| --- | --- |
| Up / W, gamepad A, right trigger or left stick up | Thrust forward |
| Down / S, gamepad B, left trigger or left stick down | Brake, then reverse |
| Left / Right, A / D, d-pad or left stick | Turn |
| Esc, P or Start | Pause (Q or B in the pause menu: back to the title) |
| F11 or Alt+Enter | Fullscreen |
| Enter, Space or A | Start, play again |

## Rules

- 90 seconds a run. When the time is up, the run ends and the score is shown.
- Seven crystals float over the water at a time. Touching one collects it,
  one point, and a new one appears somewhere else on the lagoon, never right
  next to the hovercraft.
- Every fifth crystal that appears is golden: three points, but it fades away
  after 12 seconds if it isn't collected.
- The lagoon is ringed by buoys 59 meters from the middle: past 55 meters the
  hovercraft is pushed back, and it can't go past 56.5. Islets and mooring
  poles are solid; the hovercraft bounces off them and loses speed.
- The best score is saved.

## Screens

Title (the hovercraft idling in the sunset, the controls), playing, paused,
time up (score, best, play again).

## Look

- Sky and light: `assets/skies/lake_sunset.hdr`, made from Poly Haven's
  `venice_sunset_1k.hdr` (CC0, kept in `sources/skies/`) by
  `sources/skies/lakesky.go`: the waterfront's houses, trees and paving are
  painted out with the sky beside them, the ground under the horizon is the
  sky mirrored and darkened, like calm water, and the sun's disc is dimmed so
  it doesn't light the world twice. Run it again with
  `./golib go run examples/sunset/sources/skies/lakesky.go`.
- The sun: GoLib's sun points where the HDRI's is (towards -X and +Z, 3
  degrees up), raised to 9 degrees so the shadows stay long but readable,
  orange, and strong (`sunStrength` 7), since the HDRI's sky lights
  everything softly and a weaker sun hardly shows.
- Water: a huge plane with a shader that ripples its normals, smooth, so it
  reflects the sky, with the sun's glitter path; a little teal, so the long
  shadows show on it.
- Crystals: hexagonal bipyramids made with `golib.NewMesh`, emissive and
  pulsing, in a faint glowing halo, each with a small lamp that lights the
  water under it. Bloom is kept low (0.15): the low sun's glare on the water
  would otherwise fog the whole screen.
- Hovercraft: a polished steel hull and side pods with orange stripes and
  rudders, a rubber skirt, a glass canopy over the pilot's helmet and a
  spinning fan, all from boxes, cylinders, spheres and cones; a foam wake
  and spray behind it.

## Tuning

All in `world.go`, at the top: `roundTime` 90 s, `thrust` 16 m/s²,
`maxSpeed` 17 m/s, `turnSpeed` 120 deg/s, `forwardDrag` 0.7/s, `sideGrip`
2.2/s (lower drifts more), `crystalCount` 7, `goldenEvery` 5,
`goldenLife` 12 s, `lagoonRadius` 55 m. The camera is in `scenes.go`
(`cameraDistance` 8.5 m, `cameraPitch` -17, `cameraLag` 0.15 s), the light in
`draw.go` (`sunStrength`, `exposure`, `bloom`) and `main.go`
(`sunDirection`, the colors), the wake and spray in `effects.go`.

## Later

- Engine pitch that follows the throttle (GoLib loops a sound only at one
  pitch).
- A wake that ripples the water itself.
- Online or local high-score table.

## Changelog

- 0.1.0: first version: the hovercraft, crystals, golden crystals, islets
  and poles, the 90-second round, title, pause and time-up screens, the
  best score, the cleaned-up sunset sky, the wake, markers at the screen's
  edge for crystals out of sight.
