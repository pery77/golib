//go:build !steam || js

package golib

import (
	"strings"
	"testing"
)

// Without the build tag steam, every Steam function does nothing and says
// why.
func TestSteamWithoutTheTag(t *testing.T) {
	if SteamRunning() {
		t.Error("SteamRunning is true without the build tag steam")
	}
	if err := SteamError(); err == nil || !strings.Contains(err.Error(), "build tag steam") {
		t.Errorf("SteamError() = %v, want it to name the build tag steam", err)
	}
	SteamUnlockAchievement("ACH_WIN_ONE_GAME")
	SteamClearAchievement("ACH_WIN_ONE_GAME")
	SteamAchievementProgress("ACH_WIN_100_GAMES", 10, 100)
	if unlocked, ok := SteamAchievement("ACH_WIN_ONE_GAME"); unlocked || ok {
		t.Errorf("SteamAchievement() = %t, %t, want false, false", unlocked, ok)
	}
	if language := SteamLanguage(); language != "" {
		t.Errorf("SteamLanguage() = %q, want \"\"", language)
	}
	if SteamOnDeck() {
		t.Error("SteamOnDeck is true without Steam")
	}
	if startSteam() || SteamRunning() {
		t.Error("startSteam started something without the build tag steam")
	}
	stopSteam()
	if len(steamState.pending) != 0 {
		t.Errorf("achievements wait for Steam without the tag: %q", steamState.pending)
	}
}
