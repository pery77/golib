//go:build steam && !js

package device

import (
	"os"
	"strings"
	"testing"
)

// The Steamworks API loads and starts, or says that Steam isn't running:
// both show that GoLib calls it right. The test runs with golib test --tags
// steam, which puts the library that golib steam-sdk copied on the library
// search path, and is skipped without it. It plays Spacewar, Valve's app
// 480 for trying the API, and changes nothing on the Steam account.
func TestSteamStarts(t *testing.T) {
	if err := loadSteam(); err != nil {
		t.Skipf("no Steamworks API to try: run golib steam-sdk first (%v)", err)
	}
	t.Setenv("SteamAppId", "")
	os.Unsetenv("SteamAppId")
	relaunched, err := SteamStart(480, false)
	if relaunched {
		t.Fatal("SteamStart relaunched a debug build")
	}
	if os.Getenv("SteamAppId") != "480" {
		t.Errorf("SteamAppId is %q, want 480", os.Getenv("SteamAppId"))
	}
	if err != nil {
		if !strings.HasPrefix(err.Error(), "Steam didn't start: ") {
			t.Fatalf("SteamStart: %v, want Steam's own answer", err)
		}
		t.Logf("the API loaded, and Steam answered: %v", err)
		if SteamSetAchievement("ACH_WIN_ONE_GAME") || SteamLanguage() != "" || SteamOnDeck() {
			t.Error("Steam's functions answered without Steam")
		}
		return
	}
	defer SteamStop()
	SteamRunCallbacks()
	language := SteamLanguage()
	t.Logf("Steam runs: language %q, on a Steam Deck: %t", language, SteamOnDeck())
	if language == "" {
		t.Error("Steam runs and gives no language")
	}
	if _, ok := SteamAchievement("ACH_WIN_ONE_GAME"); !ok {
		t.Log("Spacewar's ACH_WIN_ONE_GAME is unknown: Steam may not have sent the stats yet")
	}
	if _, ok := SteamAchievement("GOLIB_NO_SUCH_ACHIEVEMENT"); ok {
		t.Error("an unknown achievement is ok")
	}
}
