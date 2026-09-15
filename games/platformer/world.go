package main

import "golib"

// Tuning: the numbers that define how the game feels.
const (
	moveSpeed    = 320  // pixels per second
	jumpSpeed    = 760  // pixels per second, upwards; jumps about 160 pixels high
	gravity      = 1800 // pixels per second, gained every second
	maxFallSpeed = 1200 // pixels per second

	playerWidth  = 36 // pixels
	playerHeight = 48 // pixels
	coinRadius   = 12 // pixels
)

// Level size and start position, in pixels.
const (
	levelWidth  = 1280
	levelHeight = 720
	spawnX      = 80
	spawnY      = 600
	fallLimit   = levelHeight + 100 // below this, the player starts over at the spawn point
	cloudCount  = 6                 // background clouds, placed at random
)

// player is the character the user controls.
type player struct {
	x, y      float32 // top-left corner, in pixels
	velocityX float32 // pixels per second; positive is right
	velocityY float32 // pixels per second; positive is down
	onGround  bool
}

func (p player) bounds() golib.Rectangle {
	return golib.Rectangle{X: p.x, Y: p.y, Width: playerWidth, Height: playerHeight}
}

// coin is something to collect.
type coin struct {
	x, y      float32 // center, in pixels
	collected bool
}

func (c coin) bounds() golib.Rectangle {
	return golib.Rectangle{X: c.x - coinRadius, Y: c.y - coinRadius, Width: 2 * coinRadius, Height: 2 * coinRadius}
}

// world is the whole game state and its rules. It knows nothing about the
// keyboard or the screen, so world_test.go can play it directly.
type world struct {
	player    player
	platforms []golib.Rectangle
	coins     []coin
	won       bool
}

// newWorld returns the level at its start. It becomes a Tiled map once GoLib
// loads them.
func newWorld() world {
	return world{
		player: player{x: spawnX, y: spawnY},
		platforms: []golib.Rectangle{
			{X: 0, Y: 660, Width: 500, Height: 60},   // ground, left of the gap
			{X: 620, Y: 660, Width: 660, Height: 60}, // ground, right of the gap
			{X: 180, Y: 530, Width: 180, Height: 24},
			{X: 440, Y: 410, Width: 160, Height: 24},
			{X: 700, Y: 300, Width: 180, Height: 24},
			{X: 980, Y: 430, Width: 200, Height: 24},
		},
		coins: []coin{
			{x: 270, y: 490},
			{x: 520, y: 370},
			{x: 790, y: 260},
			{x: 1080, y: 390},
			{x: 560, y: 600}, // over the gap
		},
	}
}

// coinsCollected returns how many coins the player has picked up.
func (w *world) coinsCollected() int {
	count := 0
	for _, c := range w.coins {
		if c.collected {
			count++
		}
	}
	return count
}

// step advances the world by dt seconds. move is -1 (left), 0 or 1 (right).
// jump asks for a jump, which only happens while standing on something.
func (w *world) step(move float32, jump bool, dt float32) {
	if w.won {
		return
	}
	p := &w.player

	p.velocityX = move * moveSpeed
	if jump && p.onGround {
		p.velocityY = -jumpSpeed
	}
	p.velocityY = min(p.velocityY+gravity*dt, maxFallSpeed)

	// Move one axis at a time, and push the player back out of any platform it
	// runs into. Doing X first, then Y, keeps walls and floors separate.
	p.x = max(0, min(p.x+p.velocityX*dt, levelWidth-playerWidth))
	for _, platform := range w.platforms {
		if p.bounds().Overlaps(platform) {
			if p.velocityX > 0 {
				p.x = platform.X - playerWidth
			} else if p.velocityX < 0 {
				p.x = platform.X + platform.Width
			}
		}
	}

	p.y += p.velocityY * dt
	p.onGround = false
	for _, platform := range w.platforms {
		if p.bounds().Overlaps(platform) {
			if p.velocityY > 0 {
				p.y = platform.Y - playerHeight // landed on top
				p.onGround = true
			} else if p.velocityY < 0 {
				p.y = platform.Y + platform.Height // bumped a head
			}
			p.velocityY = 0
		}
	}

	if p.y > fallLimit {
		*p = player{x: spawnX, y: spawnY}
	}

	for i := range w.coins {
		if !w.coins[i].collected && p.bounds().Overlaps(w.coins[i].bounds()) {
			w.coins[i].collected = true
		}
	}
	w.won = w.coinsCollected() == len(w.coins)
}
