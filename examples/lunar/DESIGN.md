# SELENE: Frontier

## Pitch
A cinematic 2D lunar lander and a small space logistics company. Fly with inertia,
touch down gently, and turn contract income into better ships and a lunar economy.
Original geometric artwork, made with GoLib. No external dependencies.

## Core loop
Choose a region and cargo contract, fly to its illuminated pad, stabilize and land.
Precision, fuel efficiency and timely express deliveries add bonuses. Spend credits
on four upgrade tracks, three ships and five colonies. Colonies pay dividends on
every successful delivery, compounding the player's earning power.
Establish all five colonies to complete the campaign; continue playing afterward.
Crashes lose the contract, never the ship or existing savings. Retrying is free.

## Controls
| Input | Action |
| --- | --- |
| W / Up / Space / gamepad A | Main engine |
| A / D, Left / Right, left stick | Rotate |
| S / Down / Shift / gamepad X | Stabilize and brake horizontal drift; uses fuel |
| Mouse wheel / Q / E / gamepad bumpers | Zoom |
| C / gamepad Y | Automatic camera |
| Esc / P / gamepad Start | Pause or resume |
| Mouse / arrows + Enter / d-pad + A | Menus |
| 1 / 2 / 3 / Tab | Mission control tabs |
| F2 | Toggle post-processing |
| F5 | Reload cinema.fs during desktop development |
| M | Mute all audio |
| F11 / Alt+Enter | Fullscreen |
| Touch buttons | Flight controls on touch screens |

## Rules
Five regions unlock at 0, 2, 5, 9 and 14 deliveries. Supply, research and express
contracts have different destinations and bonuses. Research needs softer landings.
Express pays extra within its advertised time; late deliveries still pay.
Both feet must be on the destination pad, with safe descent, drift and tilt.
Collision geometry and spawn points come from editable Tiled object maps.
Progress and settings save after transactions and flights. Saving errors appear
in the interface. There is no real-time offline income.

## Screens
Title and flight manual; mission control (contracts, hangar, colonies); flight;
pause; debrief; campaign completion celebration; audio/effects/fullscreen
settings and campaign reset with confirmation.

## Tuning
Flight units and tolerances: world.go. Prices, ships, unlocks: campaign.go.
Terrain, pads, insertion points: assets/maps/*.tmx, editable in Tiled.
Palette and geometry: art.go, ui.go. Effects: shaders/cinema.fs.
The shader adds a normalized square blur of the source image to the sharp scene,
with radial chromatic aberration, vignette and grain. Its default kernel is 7 × 7
(49 samples); there is no highlight extraction or downsample pyramid.
`KERNEL_SIZE`, `samplePosMult`, `bloomStrength` and `CHROMATIC_ABERRATION_PIXELS`
tune it in the shader. Spacing and aberration use game pixels; the center has no
channel separation. F5 rereads the file in a desktop development build and uses
GoLib's `Shader.Reload` to compile before replacing the current program. Errors
keep the previous shader, and reloads preserve flight state and uniform values.
Original planet sprites and icon: sources/art/main.go.
Synthesized engine, feedback and original ambient score: sounds.go.

## Later
Additional regions, player-supplied music, more cargo mechanics.

## Changelog
- 2026-09-23: Replaced Kawase glow with a normalized square blur and added F5
  shader reload during desktop development, with recovery from compile errors.
- 2026-09-23: Replaced the radial bloom with a soft-knee Kawase glow at two
  scales and added radial chromatic aberration. F2 still toggles all effects.
- 2026-09-23: Flight, contracts, upgrades, ships, colonies, persistent progression,
  particles, cinematic camera, post-processing, keyboard/gamepad/touch controls.
