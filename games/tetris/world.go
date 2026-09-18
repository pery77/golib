package main

import "golib"

// The board is boardColumns cells wide and boardRows tall, which is what the
// player sees: the standard 10 by 20 Tetris playfield. Pieces spawn at the top
// and stack downwards; a new piece that can't fit ends the game.
const (
	boardColumns = 10
	boardRows    = 20
)

// Gravity: how long the piece takes to fall one cell at level 1, how much
// faster every level makes it, the fastest it ever gets, and how much faster a
// soft drop (holding Down) is.
const (
	baseDropInterval = 0.80 // seconds per cell at level 1
	dropAcceleration = 0.74 // times the interval, per level
	minDropInterval  = 0.05 // seconds per cell, the floor
	softDropFactor   = 16   // soft drop is this many times faster
)

// Lock: how long a landed piece waits before it locks, and how many moves or
// rotations may put that wait off. The delay lets the player slide or twist a
// piece into place after it has touched down.
const (
	lockDelay     = 0.5 // seconds
	maxLockResets = 15  // moves or rotations that may restart the lock delay
)

// Clearing: how long a full row flashes before it disappears.
const clearDuration = 0.33 // seconds

// Scoring: points for each number of lines cleared at once, times the level
// (lineScores); points for each cell a soft or hard drop moved the piece; and
// the bonus for clearing lines on several consecutive pieces (a "combo").
const (
	softDropPoints = 1
	hardDropPoints = 2
	comboBonus     = 50 // times the level, per extra combo step
	linesPerLevel  = 10
)

// lineScores is the base score for clearing n lines at once, times the level.
// Four at once (a "Tetris") is worth far more than four single clears.
var lineScores = [5]int{0, 100, 300, 500, 800}

// pieceKind is one of the seven tetrominoes.
type pieceKind int

const (
	pieceI pieceKind = iota
	pieceO
	pieceT
	pieceS
	pieceZ
	pieceJ
	pieceL
	pieceKinds // the number of kinds; keep it last
)

// cell is a grid position, in cells from the board's top-left corner.
type cell struct{ x, y int }

// shapes holds the four cells of every piece for each of its four rotations,
// clockwise, as offsets from the piece's origin. The origin is the top-left
// corner of the box the piece turns in: 4 by 4 for I, 2 by 2 for O, 3 by 3 for
// the rest. These are the standard Super Rotation System shapes, with y growing
// downwards like the screen.
var shapes = [pieceKinds][4][4]cell{
	pieceI: {
		{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
		{{2, 0}, {2, 1}, {2, 2}, {2, 3}},
		{{0, 2}, {1, 2}, {2, 2}, {3, 2}},
		{{1, 0}, {1, 1}, {1, 2}, {1, 3}},
	},
	pieceO: {
		{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
	},
	pieceT: {
		{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
		{{1, 0}, {1, 1}, {2, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {1, 2}},
		{{1, 0}, {0, 1}, {1, 1}, {1, 2}},
	},
	pieceS: {
		{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
		{{1, 0}, {1, 1}, {2, 1}, {2, 2}},
		{{1, 1}, {2, 1}, {0, 2}, {1, 2}},
		{{0, 0}, {0, 1}, {1, 1}, {1, 2}},
	},
	pieceZ: {
		{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
		{{2, 0}, {1, 1}, {2, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {1, 2}, {2, 2}},
		{{1, 0}, {0, 1}, {1, 1}, {0, 2}},
	},
	pieceJ: {
		{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
		{{1, 0}, {2, 0}, {1, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {2, 2}},
		{{1, 0}, {1, 1}, {0, 2}, {1, 2}},
	},
	pieceL: {
		{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
		{{1, 0}, {1, 1}, {1, 2}, {2, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {0, 2}},
		{{0, 0}, {1, 0}, {1, 1}, {1, 2}},
	},
}

// Wall kicks: when a rotation would put a piece through a wall, the floor or a
// stack, these offsets are tried in order until one of them fits. They are the
// standard Super Rotation System tables, with y converted to grow downwards: a
// positive y moves the piece down the screen.
var (
	kicksCW = [4][5]cell{
		{{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},  // 0 -> R
		{{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},    // R -> 2
		{{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},     // 2 -> L
		{{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}}, // L -> 0
	}
	kicksCCW = [4][5]cell{
		{{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},     // 0 -> L
		{{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},    // R -> 0
		{{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},  // 2 -> R
		{{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}}, // L -> 2
	}
	kicksICW = [4][5]cell{
		{{0, 0}, {-2, 0}, {1, 0}, {-2, 1}, {1, -2}}, // 0 -> R
		{{0, 0}, {-1, 0}, {2, 0}, {-1, -2}, {2, 1}}, // R -> 2
		{{0, 0}, {2, 0}, {-1, 0}, {2, -1}, {-1, 2}}, // 2 -> L
		{{0, 0}, {1, 0}, {-2, 0}, {1, 2}, {-2, -1}}, // L -> 0
	}
	kicksICC = [4][5]cell{
		{{0, 0}, {-1, 0}, {2, 0}, {-1, -2}, {2, 1}}, // 0 -> L
		{{0, 0}, {2, 0}, {-1, 0}, {2, -1}, {-1, 2}}, // R -> 0
		{{0, 0}, {1, 0}, {-2, 0}, {1, 2}, {-2, -1}}, // 2 -> R
		{{0, 0}, {-2, 0}, {1, 0}, {-2, 1}, {1, -2}}, // L -> 2
	}
)

// phase is what the game is doing right now.
type phase int

const (
	phasePlaying  phase = iota // the player moves and drops the piece
	phaseClearing              // full rows flash before they disappear
	phaseGameOver              // no room for a new piece
)

// eventKind is something worth telling the scene about, so it can play a sound
// or start an effect. The rules never touch sound or drawing themselves.
type eventKind int

const (
	eventLock eventKind = iota
	eventHardDrop
	eventClear
	eventLevelUp
	eventHold
	eventGameOver
)

// event is one thing that happened this update.
type event struct {
	kind  eventKind
	value int   // lines cleared, for eventClear; the level, for eventLevelUp
	rows  []int // the rows that cleared, for eventClear, for the effect
}

// activePiece is the piece the player is moving: which tetromino, how it is
// turned, and where its origin sits on the board.
type activePiece struct {
	kind     pieceKind
	rotation int // 0 to 3, clockwise
	x, y     int // cells from the board's top-left corner
}

// cells returns the four board cells the piece covers.
func (p activePiece) cells() [4]cell {
	shape := shapes[p.kind][p.rotation]
	var out [4]cell
	for i, c := range shape {
		out[i] = cell{x: p.x + c.x, y: p.y + c.y}
	}
	return out
}

// moved returns the piece moved by dx, dy cells.
func (p activePiece) moved(dx, dy int) activePiece {
	p.x += dx
	p.y += dy
	return p
}

// actions is what the player asked for in one update, turned from the keyboard
// and gamepad by play.go. The world never sees the input itself.
type actions struct {
	moveX     int // -1 left, +1 right, 0 still: at most one cell this update
	rotateCW  bool
	rotateCCW bool
	softDrop  bool
	hardDrop  bool
	hold      bool
}

// world is the whole game state and its rules. It knows nothing about the input
// or the screen, so world_test.go can play it directly.
type world struct {
	board [boardRows][boardColumns]int // 0 empty, else int(kind)+1
	piece activePiece
	next  pieceKind
	hold  pieceKind
	// hasHold is whether anything is in hold; holdLocked is whether hold has
	// already been used for the current piece (once per piece).
	hasHold    bool
	holdLocked bool
	bag        []pieceKind // the pieces left in the current seven-piece bag

	score int
	level int
	lines int
	combo int // lines cleared on consecutive pieces, for the combo bonus

	phase      phase
	gravity    float32 // seconds of falling saved up
	lockTimer  float32 // seconds the landed piece has been resting
	lockResets int     // times a move has restarted the lock delay
	clearTimer float32 // seconds the clearing rows have been flashing

	events []event
}

// newWorld returns a world at the start of a game, with the first piece already
// on the board.
func newWorld() *world {
	w := &world{level: 1}
	w.next = w.draw()
	w.spawnNext()
	return w
}

// draw takes the next piece from a shuffled bag of all seven tetrominoes. The
// bag is refilled and reshuffled whenever it empties, so a piece never waits
// too long, and the same seed gives the same sequence in shots.
func (w *world) draw() pieceKind {
	if len(w.bag) == 0 {
		bag := []pieceKind{pieceI, pieceO, pieceT, pieceS, pieceZ, pieceJ, pieceL}
		for i := len(bag) - 1; i > 0; i-- {
			j := golib.RandomInt(0, i)
			bag[i], bag[j] = bag[j], bag[i]
		}
		w.bag = bag
	}
	kind := w.bag[0]
	w.bag = w.bag[1:]
	return kind
}

// spawnNext puts the upcoming piece on the board and draws the one after it.
func (w *world) spawnNext() {
	kind := w.next
	w.next = w.draw()
	w.spawn(kind)
}

// spawn places a piece at the top of the board. If there is no room for it, the
// game is over.
func (w *world) spawn(kind pieceKind) {
	p := activePiece{kind: kind, x: spawnX(kind), y: spawnY(kind)}
	if w.collides(p) {
		w.phase = phaseGameOver
		w.emit(eventGameOver, 0)
		return
	}
	w.piece = p
	w.gravity, w.lockTimer, w.lockResets = 0, 0, 0
	w.holdLocked = false
}

// boxWidth is how wide the box a piece turns in is, in cells.
func boxWidth(k pieceKind) int {
	switch k {
	case pieceI:
		return 4
	case pieceO:
		return 2
	default:
		return 3
	}
}

// spawnX centers a piece's box over the board.
func spawnX(k pieceKind) int {
	return (boardColumns - boxWidth(k)) / 2
}

// spawnY puts a piece's topmost cell on the board's first row, so it appears
// whole at the top as soon as it spawns.
func spawnY(k pieceKind) int {
	top := shapes[k][0][0].y
	for _, c := range shapes[k][0] {
		top = min(top, c.y)
	}
	return -top
}

// collides reports whether the piece would overlap a wall, the floor or a
// stacked block where it is.
func (w *world) collides(p activePiece) bool {
	for _, c := range p.cells() {
		if c.x < 0 || c.x >= boardColumns || c.y >= boardRows {
			return true
		}
		if c.y >= 0 && w.board[c.y][c.x] != 0 {
			return true
		}
	}
	return false
}

// canMove reports whether the piece could move by dx, dy cells.
func (w *world) canMove(dx, dy int) bool {
	return !w.collides(w.piece.moved(dx, dy))
}

// tryMove moves the piece by dx, dy cells if it fits, and reports whether it
// moved. A sideways move restarts the lock delay.
func (w *world) tryMove(dx, dy int) bool {
	p := w.piece.moved(dx, dy)
	if w.collides(p) {
		return false
	}
	w.piece = p
	if dx != 0 {
		w.delayLock()
	}
	return true
}

// delayLock restarts the lock delay after a move or a rotation, while the piece
// still has resets left. Once they run out, a resting piece locks on time.
func (w *world) delayLock() {
	if w.canMove(0, 1) || w.lockResets >= maxLockResets {
		return
	}
	w.lockTimer = 0
	w.lockResets++
}

// rotate turns the piece a quarter turn: dir is +1 clockwise, -1 anticlockwise.
// It tries the wall kicks of the piece's kind and reports whether it turned.
func (w *world) rotate(dir int) bool {
	if w.piece.kind == pieceO {
		return false
	}
	from := w.piece.rotation
	to := (from + dir + 4) % 4
	for _, k := range w.kicks(from, dir) {
		p := w.piece
		p.rotation = to
		p.x += k.x
		p.y += k.y
		if !w.collides(p) {
			w.piece = p
			w.delayLock()
			return true
		}
	}
	return false
}

// kicks returns the wall-kick offsets for a rotation of the current piece.
func (w *world) kicks(from, dir int) [5]cell {
	clockwise := dir > 0
	if w.piece.kind == pieceI {
		if clockwise {
			return kicksICW[from]
		}
		return kicksICC[from]
	}
	if clockwise {
		return kicksCW[from]
	}
	return kicksCCW[from]
}

// hardDrop drops the piece to the bottom at once, scores the cells it fell, and
// locks it. It reports the drop as its own event, so the scene can shake the
// screen harder than for a piece that simply came to rest.
func (w *world) hardDrop() {
	dropped := 0
	for w.canMove(0, 1) {
		w.piece.y++
		dropped++
	}
	w.score += dropped * hardDropPoints
	w.emit(eventHardDrop, dropped)
	w.lock()
}

// lock fixes the piece to the board, clears any full rows, and brings the next
// piece. When no full row was made, the next piece comes right away; otherwise
// the rows flash first and finishClear brings it later.
func (w *world) lock() {
	for _, c := range w.piece.cells() {
		if c.y >= 0 && c.y < boardRows && c.x >= 0 && c.x < boardColumns {
			w.board[c.y][c.x] = int(w.piece.kind) + 1
		}
	}
	w.emit(eventLock, 0)

	full := w.fullRows()
	if len(full) == 0 {
		w.combo = 0
		w.spawnNext()
		return
	}

	w.combo++
	cleared := len(full)
	w.score += lineScores[cleared] * w.level
	if w.combo > 1 {
		w.score += comboBonus * (w.combo - 1) * w.level
	}
	w.phase = phaseClearing
	w.clearTimer = 0
	w.emitClear(cleared, full)
}

// finishClear removes the full rows, drops everything above them, counts the
// lines, raises the level when due, and brings the next piece.
func (w *world) finishClear() {
	cleared := 0
	write := boardRows - 1
	for y := boardRows - 1; y >= 0; y-- {
		if w.isFull(y) {
			cleared++
			continue
		}
		w.board[write] = w.board[y]
		write--
	}
	for y := write; y >= 0; y-- {
		w.board[y] = [boardColumns]int{}
	}

	w.lines += cleared
	if newLevel := w.lines/linesPerLevel + 1; newLevel > w.level {
		w.level = newLevel
		w.emit(eventLevelUp, newLevel)
	}
	w.phase = phasePlaying
	w.clearTimer = 0
	w.spawnNext()
}

// isFull reports whether a row is completely filled.
func (w *world) isFull(y int) bool {
	for x := 0; x < boardColumns; x++ {
		if w.board[y][x] == 0 {
			return false
		}
	}
	return true
}

// fullRows returns the rows that are completely filled, from the top down.
func (w *world) fullRows() []int {
	rows := make([]int, 0, 4)
	for y := 0; y < boardRows; y++ {
		if w.isFull(y) {
			rows = append(rows, y)
		}
	}
	return rows
}

// ghostY is the row the piece would rest on if it fell straight down now, for
// the shadow the game draws under it.
func (w *world) ghostY() int {
	y := w.piece.y
	for !w.collides(activePiece{kind: w.piece.kind, rotation: w.piece.rotation, x: w.piece.x, y: y + 1}) {
		y++
	}
	return y
}

// dropInterval is the current seconds between automatic one-cell drops. It
// falls with the level, down to a floor.
func (w *world) dropInterval() float32 {
	interval := float32(baseDropInterval)
	for i := 1; i < w.level; i++ {
		interval *= dropAcceleration
	}
	return max(interval, minDropInterval)
}

// holdPiece puts the current piece aside and brings back the one in hold, or,
// the first time, takes the piece and brings the next one. It works once per
// piece.
func (w *world) holdPiece() {
	if w.holdLocked || w.phase != phasePlaying {
		return
	}
	if !w.hasHold {
		w.hasHold = true
		w.hold = w.piece.kind
		w.spawnNext()
		if w.phase == phaseGameOver {
			return
		}
	} else {
		w.hold, w.piece.kind = w.piece.kind, w.hold
		w.piece.rotation = 0
		w.piece.x = spawnX(w.piece.kind)
		w.piece.y = spawnY(w.piece.kind)
		if w.collides(w.piece) {
			w.phase = phaseGameOver
			w.emit(eventGameOver, 0)
			return
		}
		w.gravity, w.lockTimer, w.lockResets = 0, 0, 0
	}
	w.holdLocked = true
	w.emit(eventHold, 0)
}

// step advances the game by dt seconds. a is what the player asked for this
// update. golib.Run calls it from Update 60 times per second; world_test.go
// calls it directly.
func (w *world) step(a actions, dt float32) {
	switch w.phase {
	case phaseGameOver:
		return
	case phaseClearing:
		w.clearTimer += dt
		if w.clearTimer >= clearDuration {
			w.finishClear()
		}
		return
	}

	if a.hold {
		w.holdPiece()
	}
	if a.rotateCW {
		w.rotate(1)
	}
	if a.rotateCCW {
		w.rotate(-1)
	}
	if a.moveX != 0 {
		w.tryMove(a.moveX, 0)
	}

	if a.hardDrop {
		w.hardDrop()
		return
	}

	// Gravity, faster while soft dropping. dt is only 1/60, so the loop runs
	// once most of the time and a few times when the piece is falling fast.
	interval := w.dropInterval()
	if a.softDrop {
		interval = max(interval/softDropFactor, 0.02)
	}
	w.gravity += dt
	for w.gravity >= interval {
		w.gravity -= interval
		if !w.tryMove(0, 1) {
			break
		}
		if a.softDrop {
			w.score += softDropPoints
		}
	}

	// Lock delay: a piece resting on something waits a moment, so the player
	// can still slide or twist it into place.
	if w.canMove(0, 1) {
		w.lockTimer = 0
		w.lockResets = 0
		return
	}
	w.lockTimer += dt
	if w.lockTimer >= lockDelay {
		w.lock()
	}
}

// emit records an event for the scene to react to.
func (w *world) emit(kind eventKind, value int) {
	w.events = append(w.events, event{kind: kind, value: value})
}

// emitClear records a line clear, with the rows that cleared so the scene can
// put its effect in the right place.
func (w *world) emitClear(cleared int, rows []int) {
	w.events = append(w.events, event{kind: eventClear, value: cleared, rows: rows})
}

// TakeEvents returns the events that have happened since the last call and
// forgets them. The scene plays a sound and starts an effect for each.
func (w *world) TakeEvents() []event {
	events := w.events
	w.events = nil
	return events
}
