# Asteroids

## Pitch
Fly a small ship through a field of drifting rocks and shoot them to pieces before they hit you. A vector-arcade classic, with a glow and an old CRT look. It is also GoLib's test game for post-processing shaders and fullscreen.

## Core loop
Turn towards a rock, shoot it, dodge the two smaller pieces, and keep moving: the edges wrap around, so danger comes from every side.

## Controls
| Input | Action |
| --- | --- |
| Left / Right arrows, A / D, d-pad or left stick | Turn; the stick turns slower when tilted less |
| Up arrow, W, B, d-pad up or right trigger | Thrust |
| Space or A (hold to keep firing) | Fire |
| Esc or Start | Pause and resume while playing |
| Enter, A or Start | Start, on the title; play again, after game over |
| Q or B | Quit to the title, while paused; B also after game over |
| Esc | Quit, on the title; back to the title, after game over |
| F11 or Alt+Enter | Fullscreen on and off, in every scene |
| F2 or Y | Screen effects (glow and CRT) on and off, in every scene |

Gamepad controls read gamepad 0, the first one connected.

## Rules
- Big rocks split into two medium rocks, medium into two small ones, and small rocks break apart. Hits score 20, 50 and 100 points.
- The ship, rocks and bullets wrap around the screen edges.
- Touching a rock costs a ship (the rock breaks too). A new ship appears in the middle after 1.5 seconds and can't crash for 2.5 seconds, while it blinks.
- Three ships per game. Losing the last one ends the game.
- Clearing every rock starts the next wave, with one more big rock.
- At most 8 bullets fly at once, and a bullet lasts 0.85 seconds.

## Screens
Each screen is a scene in `scenes.go`:
- **Title:** the name and controls over drifting rocks.
- **Play:** the field, with the score, the wave and the ships left.
- **Pause:** the field, frozen, under a message.
- **Game over:** the final score while the rocks keep drifting.

## Tuning
All the numbers are at the top of `world.go`: ship handling (`turnSpeed`, `thrustPower`, `maxShipSpeed`, `shipDrag`), shooting (`bulletSpeed`, `bulletLifetime`, `fireCooldown`, `maxBullets`), lives and respawning, and rocks (`firstWaveRocks`, speeds, sizes and points). The look is in `main.go`: colors, `lineWidth`, and the effect settings `glowStrength` and `crtCurvature`, which reach `shaders/glow.fs` and `shaders/crt.fs` as uniforms.

## Later
- Sounds: shots, explosions, thrust, and a heartbeat that speeds up (when GoLib has audio).
- A flying saucer that shoots back.
- Hyperspace: jump to a random spot, at a risk.
- A high score that lasts between runs.

## Changelog
- 2026-09-15: created with `golib new`: a square that moves around the screen.
- 2026-09-15: the game: ship, rocks that split, bullets, sparks, lives, waves, score; title, pause and game over scenes; keyboard and gamepad; glow and CRT screen effects (F2 or Y); fullscreen (F11 or Alt+Enter).
