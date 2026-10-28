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
| F2 or Y | Screen effects on and off |
| F3 or X | Music on and off |
| F5 | Read the shaders from `assets/shaders/` again, to tune them while the game runs |
| Title: Enter, A or click | Start |
| Title: Esc | Quit |

Without the mouse or the arrows, the ship aims where it flies, so Space fires
straight ahead.

## Rules
- The arena is 3600 by 2400 pixels; the screen shows 1280 by 720 of it and
  follows the ship, looking 110 pixels ahead of where it aims and never past
  the arena's edge. A glowing border marks the edge; bullets that reach it
  vanish. The radar in the bottom-right corner shows the whole arena, and
  arrows at the screen's edge point at enemies off the screen.
- The ship has 5 hull points. An enemy bullet or a collision with an enemy
  costs one, then the ship can't be hurt for 1.2 seconds. At 0 the game is
  over. Every hit breaks the picture up for about half a second, and losing
  the ship breaks it up hardest of all.
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

## Screen effects
Three post-processing shaders run over the finished picture, in this order,
and F2 or the Y button turns all three off. They are files in
`assets/shaders/`, not Go code: a debug build reads them from disk every time
it starts, and F5 reads them again without leaving the game, so they can be
tuned by eye without building anything. Each file holds the numbers that
decide how it looks, marked "The look. Tune these."; the game only tells them
how it is going, through the uniforms `amount` (the glitch) and `strain` (the
lens). A `golib dist` build reads the copies embedded in the executable.
- Bloom (`assets/shaders/bloom.fs`): bright things bleed light over everything around
  them, over three widths at once, the widest one stretched sideways like a
  lens flare. It is what makes the bullets, the engines and the HUD glow.
- Lens (`assets/shaders/chroma.fs`): the colors drift apart towards the corners, as
  they do through glass. It gathers a whole spectrum along the drift instead
  of pulling the red, green and blue channels apart, so the fringes are soft
  and white stays white. The middle of the screen, where the ship is, stays
  sharp, and the drift grows while the ship is on its last hull point.
- Glitch (`assets/shaders/glitch.fs`): for about half a second after a hit, bands of
  the picture slide sideways, the colors come apart, blocks are cut out and
  filled with another part of the picture, a bright bar rolls down and the
  whole thing jumps. It follows `world.glitch`, which the hit sets and every
  update fades; at 0 the picture goes through untouched.

## Music
The game plays the first file in `assets/music/` that GoLib can play (`.ogg`,
`.mp3`, `.wav`, `.qoa`, `.xm` or `.mod`), under the sound effects, and F3 or
the X button turns it off. Nothing in the code names the file, so swapping the
tune needs no building either: drop another one in and start the game again.
It is "Building Energy", a tracker module by Drozerix. A file GoLib cannot
play, such as Impulse Tracker's `.it`, leaves the game silent and says so at
the bottom of the screen, and `TestTheMusicPlays` fails. Where the tune came
from, and the license still to be written down, are in
`assets/ATTRIBUTION.md`.

## The look of space
The arena is drawn over `assets/textures/background.png`, a nebula that covers
the screen wherever the camera looks and drifts at a twelfth of its speed.
The picture is drawn much darker than the file (`skyTint` in `main.go`): at
its own brightness its clouds are as bright as the ships and the bullets, and
the game is hard to read over them. Three layers of stars drawn in code move
faster over it, so the arena still feels deep, and the grid and the glowing
border say where its edges are.

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
- Camera lead, lag and shake: the top of `play.go`. The camera itself is
  `golib.Camera`: the game sets `Target` (the ship, `cameraLead` = 110 px
  ahead of its aim), `Bounds` (the arena) and `Lag` = 0.14 s, and calls
  `Shake`. `Lag` replaced the old catch-up rate of 7 per second: the camera
  closes about two thirds of the way to the ship in `Lag` seconds, and 1/7 s,
  rounded to 0.14, matches the old easing to within half a percent per update.
  `Shake` is asked, every update, for `shakeMax` = 14 px times trauma squared,
  lasting the time trauma has left, so the shake still fades with trauma
  squared and small hits barely move the view. HUD layout: the top of
  `hud.go`. The background picture's drift (`skyDepth`), the stars, the grid
  and the ships' shapes: the top of `draw.go`; how dark the picture is drawn
  (`skyTint`): `main.go`.
- Screen effects: the constants in each shader in `assets/shaders/`, under
  "The look. Tune these." (how bright a thing has to be to glow, how much glow
  is added and how far it reaches, how many pixels the lens pulls the colors
  apart in the corners and what the last hull point adds, how far the glitch's
  bands slide). Edit, then press F5. How hard and how long a hit breaks the
  picture up: `glitchHit`, `glitchLost` and `glitchDecay` at the top of
  `world.go`, with `dangerBeat`, the beat the last hull point pulses at, which
  the HUD's red edges and the lens share. Music volume: `musicVolume` in
  `music.go`.
- Colors: the top of `main.go`. Sounds: `sounds.go`. The gun
  (`laser.jfxr`), the ship being hit (`playerhit.jfxr`), a repair kit
  (`powerup.jfxr`), an enemy breaking apart (`Hit_hurt 5.jfxr`) and the ship's
  end (`playerexplosion.jfxr`) are files in `assets/sounds/`, which open in
  jfxr (<https://jfxr.frozenfractal.com>); the rest are recipes in code. Each
  file's volume is next to it in `sounds.go`, because jfxr saves them far
  louder than a sound made from a recipe: those numbers still have to be
  tuned by ear. `TestTheSoundFilesAreThere` fails when one is renamed away.

## Assumptions
Made without asking, to be corrected by the player:
- "Naves" means spaceships, so the arena is space.
- Twin-stick controls: fly with one hand, aim with the other (mouse, arrows or
  right stick).
- Endless waves with a score, rather than a fixed number of waves to win.
- A dash, repair kits and a radar were added because a large arena with enemy
  fire is hard to read and to survive without them.
- Music, the background picture and the two explosions are files in
  `assets/`: swapping any of them needs no building.

## Later
- Power-ups (spread shot, shield), bosses every five waves.
- Saving the best score, once GoLib can save data.
- Looping engine sound, once GoLib has looping sounds.
- Asteroids or walls in the arena to hide behind, from a Tiled map.

## Changelog
- 2026-09-17: created with `golib new`.
- 2026-09-17: first playable version: arena, camera, twin-stick shooting,
  dash, three enemy kinds, endless waves, repair kits, radar and arrows to
  enemies off the screen, title, pause and game over. Not yet tuned by
  playing: the numbers come from tests and screenshots only.
- 2026-09-17: the mouse pointer is hidden with `golib.SetMouseVisible` (GoLib M6), instead of raylib.
- 2026-09-17: moved to GoLib's camera, vectors and drawing helpers (GoLib M6),
  and deleted the game code they replace. `camera.go` is gone: the scenes keep
  a `golib.Camera`, the world is drawn through it in arena coordinates instead
  of converting every call to screen pixels, and the HUD is drawn with the
  camera off. Positions and velocities are `golib.Vector2`, and angles are
  degrees instead of radians, because that is what `Vector2` and `DrawOptions`
  use. `geometry.go` keeps only `clamp`, `abs` and `wrap`. Ship and enemy
  shapes are drawn with `Screen.DrawPolygon`, rings with
  `Screen.DrawCircleOutline`, panels with `Screen.DrawRectangleOutline` and
  centered text with `TextOptions{Align: AlignCenter}`. The game still keeps
  its own parallax backdrop, the arrows' edge maths, `withAlpha` (which scales
  a color's own opacity, where `golib.WithOpacity` sets one) and `darker`.
  The game plays the same; a shot of a given frame differs from an older one
  only because the camera now draws random numbers for the shake only while
  it shakes, which moves the rest of the game along the same random sequence.
- 2026-09-17: screen effects, in `effects.go` and `shaders/`: an intense bloom,
  a spectral chromatic aberration that strains on the last hull point, and a
  glitch that tears the picture apart for half a second whenever the ship is
  hit, hardest when it is destroyed. F2 or the Y button turns them off. The
  hit's break-up is `world.glitch`, next to the screen shake it goes with.
- 2026-09-17: the shaders moved from `shaders/` into `assets/shaders/` and are
  read with `golib.ReadAsset` instead of `//go:embed`, so tuning them takes no
  building: a debug build reads them from disk as it starts, and F5 reads them
  again while the game runs. The numbers that decide how each effect looks
  moved into its shader, so the game now only sends it `amount` and `strain`.
  Music: `music.go` plays the first file in `assets/music/` GoLib can play,
  F3 turns it off, and what the game couldn't load says so at the bottom of
  the screen (`drawNotes` in `hud.go`).
- 2026-09-17: the files the player brought: "Building Energy" by Drozerix
  (`assets/music/`) plays as the game's music; the ship's end is
  `assets/sounds/playerExplosion.jfxr`, turned down to 0.45 because its file
  is amplified to 100; and `assets/textures/background.png` is the sky behind
  the arena, drifting with the camera and drawn dark enough to read the game
  over it. The seven nebulae drawn in code are gone: the picture replaces
  them, and without their random numbers the game plays out differently from
  the same seed, so old screenshots don't match.
- 2026-09-18: the player's own sounds, from `assets/sounds/`: `laser.jfxr` is
  the gun, `playerhit.jfxr` the ship being hit, `powerup.jfxr` a repair kit and
  `Hit_hurt 5.jfxr` an enemy breaking apart, which took the place of the
  deleted `explosion.jfxr`. Each is turned down where it is made, since jfxr
  saves them louder than the sounds made from recipes. The picture behind the
  arena was made with ChatGPT; `assets/ATTRIBUTION.md` says so.
