# Tetris

A complete Tetris made with GoLib: the classic falling-block game with modern
touches. This file says what the game does, how it is laid out, and what is
still missing.

Run it with `./golib run tetris` (Windows: `.\golib run tetris`).

## What the game does

- The standard 10 by 20 playfield, with the seven tetrominoes in their seven
  colors.
- **Seven-piece bag:** the pieces are shuffled in groups of seven, so every
  piece comes once before any repeats, and none waits too long.
- **Super Rotation System:** pieces turn with the standard wall kicks, so a
  piece next to a wall or resting on the stack slides into place instead of
  refusing to turn.
- **Ghost piece:** a faint outline shows where the current piece will land.
- **Hold:** set the current piece aside and bring it back later, once per
  piece.
- **Lock delay:** a piece that has touched down waits half a second before it
  locks, and up to 15 moves or turns restart that wait, so the player can slide
  or twist it into place after it lands.
- **Scoring:** 100, 300, 500 and 800 points for clearing one to four lines at
  once, times the level; a combo bonus for clearing on consecutive pieces; and
  points for soft and hard drops.
- **Levels:** the level rises every ten lines and the pieces fall faster, down
  to a floor.
- **Scenes:** a title screen with a menu, the game, a pause screen and a game
  over screen.
- **Saved records:** the best score is kept between runs with
  `golib.SaveData` and shown on the title and game over screens.

## Music, graphics and effects

Everything the game shows and hears is made from the framework; the game ships
no asset files.

- **Music** (`sounds.go`): a looping theme built note by note with
  `golib.NewTune`, in the classic Tetris spirit: a fast minor-key melody over a
  simple bass, with a tick on the beat. The design comment says how to swap it
  for a real OGG or tracker file in `assets/` instead.
- **Sound effects** (`sounds.go`): short blips for moving and turning, a thud
  for landing, a two-tone blip for hold, a rising sweep for a clear, a bigger
  fanfare for a Tetris, `golib.PowerUp` for a level-up and `golib.Hurt` for
  game over. All made with `golib.NewSound` and `golib.SoundSpec`, some with a
  random pitch each play so repeats don't sound mechanical.
- **Graphics** (`draw.go`, `main.go`): each block is a shaded tile (a lighter
  top and left edge, a darker bottom and right), the board has a faint grid,
  and the hold, next and score sit in panels around it. Pieces keep their
  classic colors. The title screen has a slow falling-block backdrop.
- **Effects** (`play.go`, `draw.go`):
  - A screen shake when a piece lands, stronger for a hard drop and strongest
    for a clear or game over.
  - A white flash over the board on a clear, brighter for a Tetris.
  - Sparks that fly off every cleared row, drawn with additive blending so
    they glow, and pulled down by gravity.
  - A cleared row flashes between white and its own color before it disappears
    (`clearDuration`).
  - A "toast" that names the clear ("DOUBLE", "TETRIS!", "LEVEL 2") and fades.
  - A head-up note when the window loses focus, because the game pauses then.

## Controls

| Action | Keyboard | Gamepad |
| --- | --- | --- |
| Move left/right | Left/Right or A/D | D-pad or left stick |
| Turn clockwise | Up or X | Up or A |
| Turn anticlockwise | Z | B |
| Soft drop | Down or S | Down |
| Hard drop | Space | X |
| Hold | C or Shift | Left bumper |
| Pause | Esc | Start |
| Music on/off | M | Y |
| Fullscreen | F11 or Alt+Enter | — |
| Menu select / confirm | Arrows, Enter, mouse | D-pad, A, Start |
| Quit (menu) | Esc or the Quit button | — |

## How the code is laid out

Read the files in this order; `main.go`'s package comment says the same.

- `main.go` starts the game: the screen size, the colors and the settings.
- `scenes.go` holds the title, pause and game over scenes, and the settings and
  saved records they share.
- `play.go` is the scene where the game is played: it turns the input into
  actions, plays sounds and effects, and draws by calling `draw.go`.
- `world.go` is the whole game as plain Go, with no input and no drawing.
- `world_test.go` tests the rules by playing `world` directly: the bag, moving,
  rotating, locking, clearing, scoring, combos, levels, hold, the ghost and
  game over.
- `draw.go` draws the board, blocks, panels and effects.
- `sounds.go` makes the music and the sound effects.

The split keeps the rules testable and the drawing simple: `Update` (in
`play.go`) reads the input into an `actions` value, `world.step` applies it and
records `event`s, and the scene reacts to those events with sound and effects.
The world never touches the screen or the speakers.

## What is missing

These are deliberate limits, either because the framework doesn't have the
feature yet or because the game didn't need it. None of them stops the game
from being played and finished.

- **No music file.** The theme is generated with `golib.NewTune`. A real track
  would sound richer; the game has no `assets/` folder and no `.ogg` or tracker
  module, so swapping it is a one-line change noted in `sounds.go` (and it
  would need an `assets.go`, see `golib.EmbedAssets`).
- **No sprite files.** Every block is drawn with rectangles and the built-in
  font. The framework can load PNG sprites and fonts (`golib.NewSprite`,
  `golib.NewFont`), but the game keeps to shapes so it needs no assets at all.
- **No high-score table.** Only the single best score and the last score are
  saved, not a list of names and scores. `golib.SaveData` can store a slice of
  records; the game would need a name-entry screen to go with it.
- **No lock-delay step reset on rotation into the stack beyond the move
  count.** The lock delay restarts on moves and turns while resets remain
  (`maxLockResets`), which matches the common modern rule; it does not
  implement the rarer "move reset" infinite-stall prevention exactly.
- **No 3D, no shaders.** The game is 2D, as Tetris is. GoLib has shaders
  (`golib.SetPostProcess`) and no 3D yet; neither is used.
- **No replays or saved games.** A game in progress is not written to disk.
  `golib.SaveData` could store the world, and `golib shot --save` could open a
  screenshot on it, but the game doesn't.
- **Sound and music are switched on or off, not volume sliders.** There is one
  music toggle (`M`); effect volumes are fixed at the values in `sounds.go`.
