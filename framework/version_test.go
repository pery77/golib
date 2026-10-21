package golib

import "testing"

func TestGameVersion(t *testing.T) {
	defer func(saved string) { gameVersion = saved }(gameVersion)

	gameVersion = ""
	if got := GameVersion(); got != "" {
		t.Errorf("GameVersion() = %q without a version, want \"\"", got)
	}
	// What golib build passes with -ldflags=-X golib.gameVersion=1.2.0-beta.
	gameVersion = "1.2.0-beta"
	if got := GameVersion(); got != "1.2.0-beta" {
		t.Errorf("GameVersion() = %q, want \"1.2.0-beta\"", got)
	}
}
