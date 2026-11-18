//go:build !steam || js

package device

import (
	"errors"
	"runtime"
)

// SteamUnavailable says why this build has no Steam: it wasn't built with
// the build tag steam, or it runs in a browser.
func SteamUnavailable() error {
	if runtime.GOOS == "js" {
		return errors.New("Steam isn't in a browser: a web build plays without it")
	}
	return errors.New("the game wasn't built with the build tag steam: build it with --tags steam, with \"steamAppID\" in game.json (see docs/tooling.md#steamworks)")
}

// SteamStart does nothing without Steam, and says why.
func SteamStart(appID uint32, restartThroughSteam bool) (relaunched bool, err error) {
	return false, SteamUnavailable()
}

func SteamRunCallbacks()                                      {}
func SteamStop()                                              {}
func SteamSetAchievement(id string) bool                      { return false }
func SteamAchievement(id string) (unlocked, ok bool)          { return false, false }
func SteamClearAchievement(id string) bool                    { return false }
func SteamStoreStats() bool                                   { return false }
func SteamAchievementProgress(id string, current, max uint32) {}
func SteamLanguage() string                                   { return "" }
func SteamOnDeck() bool                                       { return false }
