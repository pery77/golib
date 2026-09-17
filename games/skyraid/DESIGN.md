# Sky Raid

## Pitch
A top-down twin-stick space shooter in an arena several screens wide. Enemy
ships warp in, hunt you down and shoot at you, wave after wave, each one
bigger and faster than the last. Stay alive and rack up the score.

## Core loop
Fly around the arena, keep the enemy fire in view, aim and shoot at whatever
is closest, dash through a volley when it gets tight, and grab the repair kits
some wrecks leave behind. Clear the wave, catch your breath for three seconds,
and face the next one.

## Controls
| Input | Action |
| --- | --- |
| WASD, d-pad or left stick | Fly |
| Mouse | Aim (a crosshair shows where) |
| Left mouse button, Space, A or right trigger | Fire where you aim |
| Arrow keys | Aim and fire in that direction (keyboard only) |
| Right stick | Aim and fire in that direction |
| Shift, right mouse button, B or left bumper | Dash: a short burst of speed that bullets pass through |
| Esc, P or Start | Pause |
| F11 or Alt+Enter | Fullscreen on and off |
| Title: Enter, A or click | Start |
| Title: Esc | Quit |

Without the mouse or the arrows, the ship aims where it flies, so Space fires
straight ahead.

## Rules
- The arena is 3600 by 2400 pixels; the screen shows 1280 by 720 of it and
  follows the ship. A glowing border marks the edge; bullets that reach it
  vanish. The radar in the bottom-right corner shows the whole arena, and
  arrows at the screen's edge point at enemies off the screen.
- The ship has 5 hull points. An enemy bullet or a collision with an enemy
  costs one, then the ship can't be hurt for 1.2 seconds. At 0 the game is
  over.
- Dashing makes the ship untouchable while it lasts (0.18 s) and needs 1.1 s
  to recharge.
- Enemies warp in far from the ship, with a ring that shows where:
  - Scout: small and fast; flies straight at you and fires single shots.
    Ramming you destroys it, for half its 100 points, and still costs a hull
    point unless you are dashing or just hit.
  - Gunship: keeps its distance, circles you and fires fans of three.
    250 points. From wave 2.
  - Heavy: slow and tough; keeps far away and fires fans of seven.
    600 points. From wave 4.
- Enemies only fire when they are close enough to be on or near the screen.
- Points are multiplied by the wave number.
- A wave ends when all its enemies are destroyed. Clearing it repairs one hull
  point, and clearing it without being hit adds a flawless bonus of 500 times
  the wave number.
- Difficulty: each wave adds a scout, gunships and heavies join in, enemies
  arrive in bigger groups and faster, and they fly faster and fire more often
  (up to +50% speed and double fire rate).
- Wrecks sometimes leave a repair kit (scouts 5%, gunships 12%, heavies 40%),
  which lasts 10 seconds and repairs one hull point.
- There is no win: the goal is the best score. The best score and wave are
  kept while the game runs (GoLib can't save them to disk yet).

## Screens
- Title: the name, the controls, the best score so far; Enter, A or a click
  starts, Esc quits.
- Playing: the arena, with the score and hull (top left), the wave and
  enemies left (top middle), the dash charge (bottom left) and the radar
  (bottom right) over it. Between waves, a banner says which wave is next.
  The screen's edges pulse red at one hull point.
- Paused: the frozen game under a message; Esc, P or Start resumes, Q or Back
  goes to the title.
- Game over: the arena keeps moving under the final score and wave; Enter, A
  or a click plays again, Esc or B goes to the title.

## Tuning
- Ship, bullets, dash, hull, screen shake and explosions: the constants at the
  top of `world.go` (ship 340 px/s, a shot every 0.11 s at 950 px/s, dash
  1000 px/s for 0.18 s every 1.1 s, 5 hull points, 1.2 s safe after a hit).
- Enemy kinds (size, hull, speed, how far they keep away, fire rate, fans,
  bullet speed, points, repair kit chance): `enemyKinds` in `enemies.go`, with
  the fire range (720 px), the delay before a new enemy fires (1.2 s) and
  enemy bullet life (3.2 s) below it.
- Waves (enemy counts in `waveEnemies`, group sizes, spawn interval 1.8 s
  falling 0.1 s a wave to 0.7 s, speed +5% and fire rate +8% a wave, breaks of
  2 and 3 s, the flawless bonus, how far from the ship enemies warp in):
  `waves.go`.
- Camera lead, catch-up speed and shake: the top of `camera.go`. HUD layout:
  the top of `hud.go`. Stars, grid and shapes: the top of `draw.go`.
- Colors: the top of `main.go`. Sounds: `sounds.go` and
  `assets/sounds/explosion.jfxr`, which opens in jfxr
  (<https://jfxr.frozenfractal.com>).

## Assumptions
Made without asking, to be corrected by the player:
- "Naves" means spaceships, so the arena is space.
- Twin-stick controls: fly with one hand, aim with the other (mouse, arrows or
  right stick).
- Endless waves with a score, rather than a fixed number of waves to win.
- A dash, repair kits and a radar were added because a large arena with enemy
  fire is hard to read and to survive without them.
- No music: GoLib only plays music from a file the user provides.

## Later
- Power-ups (spread shot, shield), bosses every five waves.
- Saving the best score, once GoLib can save data.
- Looping engine sound, once GoLib has looping sounds.
- A glow shader over the bullets, as `games/asteroids` has.
- Asteroids or walls in the arena to hide behind, from a Tiled map.

## Changelog
- 2026-09-17: created with `golib new`.
- 2026-09-17: first playable version: arena, camera, twin-stick shooting,
  dash, three enemy kinds, endless waves, repair kits, radar and arrows to
  enemies off the screen, title, pause and game over. Not yet tuned by
  playing: the numbers come from tests and screenshots only.
- 2026-09-17: the mouse pointer is hidden with `golib.SetMouseVisible` (GoLib M6), instead of raylib.
