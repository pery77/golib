package main

import "golib"

// Tuning: the numbers that define how the game feels. The screen is 320 by
// 180 pixels, and a tile of the level is 16 by 16.
const (
	moveSpeed    = 85  // pixels per second
	jumpSpeed    = 230 // pixels per second, upwards; jumps about 52 pixels, just over three tiles
	gravity      = 500 // pixels per second, gained every second
	maxFallSpeed = 300 // pixels per second

	playerWidth  = 12 // pixels: the hero's body, which is smaller than the hero's 32 by 32 frame
	playerHeight = 22

	slashTime   = 0.3 // seconds a slash lasts
	slashReach  = 18  // pixels in front of the player that a slash hits
	slashHeight = 16  // pixels, up from the player's feet

	snakeSpeed  = 24 // pixels per second
	snakeWidth  = 14 // pixels
	snakeHeight = 10

	fallMargin = 64 // pixels below the level where a falling player starts over
)

// The level's layers, as assets/maps/forest.tmx names them. The tileset,
// assets/maps/forest.tsx, gives the tiles of the ground layer a bool property:
// solid for the ground and the crate, water for the water.
const (
	groundLayer = "ground"
	chestLayer  = "chests" // tile objects of class chest
	thingLayer  = "things" // the point named start, and points of class snake with a float property named distance
)

// player is the character the user controls.
type player struct {
	x, y       float32 // top-left corner of the hitbox, in pixels
	velocityX  float32 // pixels per second; positive is right
	velocityY  float32 // pixels per second; positive is down
	onGround   bool
	facingLeft bool
	walkTime   float32 // seconds walked on the ground without stopping, for the walk animation
	slashLeft  float32 // seconds left of the current slash; 0 when not slashing
}

func (p player) bounds() golib.Rectangle {
	return golib.Rectangle{X: p.x, Y: p.y, Width: playerWidth, Height: playerHeight}
}

// slashArea is what a slash hits: the space in front of the player.
func (p player) slashArea() golib.Rectangle {
	x := p.x + playerWidth
	if p.facingLeft {
		x = p.x - slashReach
	}
	return golib.Rectangle{X: x, Y: p.y + playerHeight - slashHeight, Width: slashReach, Height: slashHeight}
}

// chest is something to open. Opening every chest wins.
type chest struct {
	bounds golib.Rectangle
	open   bool
}

// snake crawls back and forth. Touching it sends the player back to the
// start; a slash defeats it.
type snake struct {
	x, y        float32 // top-left corner of the hitbox, in pixels
	left, right float32 // where the hitbox's left side turns back
	facingLeft  bool
	crawlTime   float32 // seconds crawled, for the animation
	defeated    bool
}

func (s snake) bounds() golib.Rectangle {
	return golib.Rectangle{X: s.x, Y: s.y, Width: snakeWidth, Height: snakeHeight}
}

// world is the whole game state and its rules. It knows nothing about the
// keyboard or the screen, so world_test.go can play it directly. It reads the
// level from the map, which works in tests too, and plays the sounds of
// sounds.go, which stay silent when there is no sound device, as in tests and
// golib shot.
type world struct {
	player         player
	startX, startY float32 // where the player's feet start, from the level
	width, height  float32 // the level's size, in pixels
	chests         []chest
	snakes         []snake
	won            bool
}

// newWorld returns the level at its start, as the map describes it.
func newWorld() world {
	w := world{width: level.Width(), height: level.Height()}
	if start, found := level.Object("start"); found {
		w.startX, w.startY = start.X, start.Y
	}
	w.player = w.startingPlayer()
	for _, object := range level.Objects(chestLayer) {
		if object.Class == "chest" {
			w.chests = append(w.chests, chest{bounds: object.Rectangle})
		}
	}
	for _, object := range level.Objects(thingLayer) {
		if object.Class == "snake" {
			// A point object is where the snake's patrol starts, on the ground.
			w.snakes = append(w.snakes, snake{
				x:     object.X,
				y:     object.Y - snakeHeight,
				left:  object.X,
				right: object.X + object.Properties.Float("distance"),
			})
		}
	}
	return w
}

// startingPlayer returns the player standing at the start.
func (w *world) startingPlayer() player {
	return player{x: w.startX - playerWidth/2, y: w.startY - playerHeight}
}

// chestsOpened returns how many chests the player has opened.
func (w *world) chestsOpened() int {
	count := 0
	for _, c := range w.chests {
		if c.open {
			count++
		}
	}
	return count
}

// step advances the world by dt seconds. move is -1 (left), 0 or 1 (right).
// jump asks for a jump, which only happens while standing on something, and
// slash asks for a slash, which only starts when the last one has ended.
func (w *world) step(move float32, jump, slash bool, dt float32) {
	if w.won {
		return
	}
	w.moveSnakes(dt)
	p := &w.player

	p.velocityX = move * moveSpeed
	if move < 0 {
		p.facingLeft = true
	} else if move > 0 {
		p.facingLeft = false
	}
	if jump && p.onGround {
		p.velocityY = -jumpSpeed
		jumpSound.Play()
	}
	p.slashLeft = max(0, p.slashLeft-dt)
	if slash && p.slashLeft == 0 {
		p.slashLeft = slashTime
		slashSound.Play()
	}
	p.velocityY = min(p.velocityY+gravity*dt, maxFallSpeed)

	// Move one axis at a time, and push the player back out of any solid tile
	// it runs into. Doing X first, then Y, keeps walls and floors separate.
	p.x = max(0, min(p.x+p.velocityX*dt, w.width-playerWidth))
	for _, tile := range solidTiles(p.bounds()) {
		if p.velocityX > 0 {
			p.x = tile.X - playerWidth
		} else if p.velocityX < 0 {
			p.x = tile.X + tile.Width
		}
	}

	p.y += p.velocityY * dt
	p.onGround = false
	for _, tile := range solidTiles(p.bounds()) {
		if p.velocityY > 0 {
			p.y = tile.Y - playerHeight // landed on top
			p.onGround = true
		} else if p.velocityY < 0 {
			p.y = tile.Y + tile.Height // bumped a head
		}
		p.velocityY = 0
	}
	if p.onGround && p.velocityX != 0 {
		p.walkTime += dt
	} else {
		p.walkTime = 0
	}

	if p.slashLeft > 0 {
		area := p.slashArea()
		for i := range w.snakes {
			if s := &w.snakes[i]; !s.defeated && area.Overlaps(s.bounds()) {
				s.defeated = true
				hitSound.Play()
			}
		}
	}

	if p.y > w.height+fallMargin || touchesWater(p.bounds()) || w.bitten() {
		*p = w.startingPlayer()
		fallSound.Play()
		return
	}

	for i := range w.chests {
		if c := &w.chests[i]; !c.open && p.bounds().Overlaps(c.bounds) {
			c.open = true
			chestSound.Play()
		}
	}
	// Winning happens once: the next step returns above, with w.won already set.
	w.won = w.chestsOpened() == len(w.chests)
	if w.won {
		winSound.Play()
	}
}

// moveSnakes crawls every snake along its patrol, turning at each end.
func (w *world) moveSnakes(dt float32) {
	for i := range w.snakes {
		s := &w.snakes[i]
		if s.defeated {
			continue
		}
		s.crawlTime += dt
		if s.facingLeft {
			s.x -= snakeSpeed * dt
			if s.x <= s.left {
				s.x, s.facingLeft = s.left, false
			}
		} else {
			s.x += snakeSpeed * dt
			if s.x >= s.right {
				s.x, s.facingLeft = s.right, true
			}
		}
	}
}

// bitten reports whether a snake touches the player.
func (w *world) bitten() bool {
	for _, s := range w.snakes {
		if !s.defeated && w.player.bounds().Overlaps(s.bounds()) {
			return true
		}
	}
	return false
}

// solidTiles returns the cells of the solid tiles that area overlaps.
func solidTiles(area golib.Rectangle) []golib.Rectangle {
	var solid []golib.Rectangle
	for _, tile := range level.TilesIn(groundLayer, area) {
		if tile.Properties.Bool("solid") {
			solid = append(solid, tile.Rectangle)
		}
	}
	return solid
}

// touchesWater reports whether area overlaps a water tile.
func touchesWater(area golib.Rectangle) bool {
	for _, tile := range level.TilesIn(groundLayer, area) {
		if tile.Properties.Bool("water") {
			return true
		}
	}
	return false
}
