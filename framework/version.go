package golib

// gameVersion is the "version" of the game's game.json. The golib commands
// set it in every build they make, with -ldflags=-X golib.gameVersion=<version>.
var gameVersion string

// GameVersion returns the game's version, as its game.json's "version" says,
// such as "1.2.0" or "1.2.0-beta", so the game can show it, for example in a
// corner of its title screen, and a player's bug report says which build it
// was:
//
//	if version := golib.GameVersion(); version != "" {
//		screen.DrawText("v"+version, 4, 4, 10, golib.Gray)
//	}
//
// It is "" when game.json gives no version, and in a program that golib
// didn't build, such as one started by go run or go test. golib build, run,
// shot, test, dist and web, and their --dist, --web and --tags forms, all
// give it.
func GameVersion() string { return gameVersion }
