package main

import "golib"

// tiles is the game's one picture, assets/sprites/tiles.png: 16 by 16 pixel
// tiles, 8 to a row, drawn for this game. The level maps use it through their
// tileset, assets/maps/tiles.tsx, and the game draws the crates and the
// player from it. A frame here is the tile's ID in Tiled: both count left to
// right, then top to bottom.
var tiles = golib.NewSpriteSheet("sprites/tiles.png", tileSize, tileSize)

// Frames of tiles.
const (
	floorFrame        = 0
	wallFrame         = 1
	goalFrame         = 2
	crateFrame        = 3
	crateOnGoalFrame  = 4 // a crate on a goal glows green
	crackedFloorFrame = 6
	crackedWallFrame  = 7
)

// playerFrames are the player's frames facing each way: standing, then
// mid-step.
var playerFrames = map[direction][2]int{
	down:  {8, 9},
	up:    {10, 11},
	left:  {12, 13},
	right: {14, 15},
}
