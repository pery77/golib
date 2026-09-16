package main

import "golib"

// The game's pictures and its level, from the assets folder. GoLib reads each
// file the first time it is used. assets/ATTRIBUTION.md says where the
// pictures come from; assets/maps/forest.tmx is the level, which opens in
// Tiled.
var (
	level      = golib.NewMap("maps/forest.tmx")
	tiles      = golib.NewSpriteSheet("sheet.png", 16, 16)      // the level's tileset, for drawing the chests
	characters = golib.NewSpriteSheet("characters.png", 32, 32) // 23 frames a row: an animal, a knight, the hero and a snake
	swoosh     = golib.NewSpriteSheet("swoosh.png", 32, 32)     // the slash, in 4 frames
)

// Frames of tiles. A tile's frame is its ID in Tiled, because both count the
// same way: left to right, then top to bottom.
const (
	closedChestFrame = 83
	openChestFrame   = 84
)

// Frames of characters. Each row has the same poses, as the sheet's author
// laid them out: walk (4 frames), ready to jump, rising, falling, landing,
// hit (2), slash (3), punch, run (4), climb (4) and seen from behind.
const (
	frameSize = 32 // pixels, in both sheets of characters

	heroRow   = 2 * 23 // the hooded hero is the third row
	heroStand = heroRow
	heroRise  = heroRow + 5
	heroFall  = heroRow + 6
	snakeRow  = 3 * 23

	// Where each hitbox sits in its 32 by 32 frame: the feet are on the
	// frame's bottom row, and the body is in the middle.
	heroOffsetX  = 9 // pixels from the frame's left side to the hitbox's
	snakeOffsetX = 8
)

var (
	heroWalk    = golib.Animation{Frames: []int{heroRow, heroRow + 1, heroRow + 2, heroRow + 3}, FrameTime: 0.12}
	heroSlash   = golib.Animation{Frames: []int{heroRow + 10, heroRow + 11, heroRow + 12}, FrameTime: slashTime / 3, Once: true}
	snakeCrawl  = golib.Animation{Frames: []int{snakeRow, snakeRow + 1, snakeRow + 2, snakeRow + 3}, FrameTime: 0.15}
	slashEffect = golib.Animation{Frames: []int{0, 1, 2, 3}, FrameTime: slashTime / 4, Once: true}
)
