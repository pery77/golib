# Marbles

## Pitch
A marble rolling on a square platform floating in the sky. Roll into the spinning gems to score, and keep away from the red cubes that slide after you, and from the edge.

## Core loop
Every few seconds: spot the nearest gem, roll to it around the cubes, brake before the edge, and pick the next one. Every 5 gems the cubes get faster.

## Controls
| Input | Action |
| --- | --- |
| Arrows or WASD, d-pad or left stick | Roll: up is away from the camera, left and right are the screen's |
| Right stick | Turn the camera |
| Esc, P or Start | Pause |
| Enter, Space, A or a click | Start, play again |
| F11 or Alt+Enter | Fullscreen |

The camera follows behind the marble: while it rolls away from the camera, the camera swings round behind the way it rolls.

## Rules
- The platform is 28 by 28 meters. The marble rolls with momentum: it speeds up, coasts and slows down by itself.
- Three gems float above the platform at any time. Rolling into one scores a point, and a new one appears somewhere else, away from the marble.
- Three red cubes chase the marble, steering with limited grip, so a quick turn gets away from them. They stay on the platform.
- Touching a cube, or rolling off the edge, costs a life. The marble then starts again in the middle, the cubes in their corners, with two seconds in which the cubes can't hurt it.
- Three lives. When the last one goes, the game is over; the best score is saved.
- Every 5 gems is a level: the cubes' top speed rises by 0.6 m/s, up to 7.5 m/s (the marble's is 9).

## Screens
Title (the platform turning slowly, the controls), playing (score, lives and level in the top-left corner), paused, game over (score and best, play again or go back to the title).

## Tuning
All in `world.go`, at the top: platform 28 m; marble acceleration 11 m/s², drag 1.0/s, top speed 9 m/s; cubes 2.4 m/s plus 0.6 per level, grip 5 m/s²; 3 gems, 5 gems a level; 2 seconds of safety after a lost life. The camera's distance, height and how fast it swings round are at the top of `controls.go`.

## Later
- Arrows at the screen's edge pointing to gems out of sight.
- Gems worth more the faster the cubes are; a gem that slows the cubes for a while.
- Platforms with holes, ramps and moving parts, as Blender levels once GoLib loads models (M7).
- A shadow from GoLib's own lights (M7) instead of the dark disc under each thing.

## Changelog
- 0.1.0: first version: rolling, gems, chasing cubes, lives, levels, title, pause and game over. The platform grew from 20 to 28 meters and the marble slowed from 10 to 9 m/s after the first shots: at first it crossed the platform in under two seconds.
