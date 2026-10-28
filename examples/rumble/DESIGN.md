# Rumble

## Pitch
A small 3D brawler: Kenney's little astronaut, stick in hand, holds a walled courtyard of grass against waves of red astronauts.

## Core loop
Red astronauts come in through the four gates, wander, notice you and chase you. You run round them, swing your stick (even while sprinting), and whack each one three times to knock it out. Clear the wave, nod, catch your breath, and a bigger one comes in.

## Controls
| Input | Action |
| --- | --- |
| Arrows, WASD, d-pad, left stick | Run, from where the camera looks. The keys sprint; Shift with them walks. The stick walks when tilted gently and sprints when pushed all the way |
| J, X, left click, gamepad X or RB | Swing the stick; it turns you to the nearest enemy in reach |
| Space, gamepad A | Jump |
| Q and E, right stick | Turn the camera |
| Esc, P, Start | Pause (Q or B there: back to the title) |
| F11, Alt+Enter | Fullscreen |

## Rules
- You have 5 hearts. A punch takes one, and for a second after it nothing else can.
- An enemy goes down after 3 whacks: it plays its death animation, lies there and fades away.
- Score: 10 a whack, 100 a knockout, and 50 times the wave's number for clearing a wave.
- Wave n has 1 + n enemies (up to 12), and they chase a little faster each wave, never as fast as your sprint.
- When the last enemy of a wave is down, you nod yes; 3.4 seconds later the next wave comes in.
- The best score and the furthest wave are saved.

## Screens
Title (the astronaut waiting by the well, the controls), playing (hearts, score, wave, enemies left, banners for each wave), paused, game over (score, wave, best, over the courtyard where the enemies wander on).

## Look
A sunny afternoon: a sky that fades to a pale horizon, a sun from the side that casts shadows, a little fog far away and a touch of bloom on the torches. A stone courtyard with battlements, four gates with banners and torches, round towers with red roofs, a well in the middle and flagstone paths, a mown lawn with stripes inside and a wilder meadow with trees outside. The enemies are the player's model tinted red; they flash gold when whacked.

## Animation
Each character has a `golib.Animator`. `poses.go` picks what the whole body plays every update from the character's own state: `idle`, `walk` or `sprint` at the speed it moves, `jump` and `fall` in the air, `die` when knocked out. Over it, on the `torso` bone, play the player's swing (`attack-melee-right`), its nod (`emote-yes`) and the enemies' punch (`attack-melee-left`), so the legs keep running. The right arm plays `holding-right` on its own layer, so the stick is held up; a torso layer covers it while it plays. The stick is a cylinder placed at `Animator.Bone("arm-right")`.

## Tuning
- Movement, jump, swing, hearts, waves and score: the constants at the top of `world.go` (sprint 5.2 m/s, walk up to 1.9 m/s, swing 0.36 s landing at 0.15 s, reach 1.35 m).
- Enemies: the constants at the top of `enemies.go` (chase 2.7 m/s plus 0.2 a wave up to 4.3, punch every 1.1 s, notice at 5.5 m).
- Animation speeds and the walk/sprint threshold: `poses.go`.
- Camera: `controls.go` (4.6 m back, 22 degrees down, closer and steeper by a wall).
- Courtyard and look: `draw.go`; colors: `main.go`.

## Later
- Enemies of other kinds: a big slow one, one that kicks (`attack-kick-right`).
- A dodge roll, a combo of swings, a charged swing.
- Pick-ups: a heart dropped now and then.
- Music, from `golib.NewTune`.

## Changelog
- 0.1.0: first version: the courtyard, the player's moves and stick, enemies that wander, chase and punch, waves, the title, pause and game over screens.
