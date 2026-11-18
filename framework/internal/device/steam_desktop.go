//go:build steam && !js

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/ebitengine/purego"
)

// Steam on the desktop, in a build with the tag steam: the Steamworks API,
// steam_api64.dll, libsteam_api.so or libsteam_api.dylib, found next to the
// game, or in the app's Frameworks folder on macOS, where golib puts it, or
// on the library search path, as golib test sets it. Its functions are
// called on the main thread, from the game loop, except the ones a game
// calls itself, which Steam allows from any thread.

// SteamUnavailable is nil in a build with Steam: SteamStart says whether
// it started.
func SteamUnavailable() error { return nil }

var steam struct {
	running bool

	userStats, apps, utils uintptr // the interfaces, once Steam runs

	restartAppIfNecessary func(appID uint32) bool
	initFlat              func(errMsg *byte) int32
	runCallbacks          func()
	shutdown              func()
	userStatsInterface    func() uintptr
	appsInterface         func() uintptr
	utilsInterface        func() uintptr
	setAchievement        func(self uintptr, name string) bool
	getAchievement        func(self uintptr, name string, achieved *bool) bool
	clearAchievement      func(self uintptr, name string) bool
	storeStats            func(self uintptr) bool
	indicateProgress      func(self uintptr, name string, current, max uint32) bool
	currentGameLanguage   func(self uintptr) string
	runningOnSteamDeck    func(self uintptr) bool
}

// steamLibraryName is the Steamworks API's file name on this system, or ""
// where Steam has none, as on Windows on ARM.
func steamLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "steam_api64.dll"
		}
		return ""
	case "darwin":
		return "libsteam_api.dylib"
	}
	return "libsteam_api.so"
}

// loadSteam loads the Steamworks API and finds the functions GoLib calls.
func loadSteam() error {
	name := steamLibraryName()
	if name == "" {
		return fmt.Errorf("Steam has no library for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	var places []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		places = append(places, filepath.Join(dir, name), filepath.Join(dir, "..", "Frameworks", name))
	}
	for _, dir := range filepath.SplitList(os.Getenv(libraryPathVariable())) {
		if dir != "" {
			places = append(places, filepath.Join(dir, name))
		}
	}
	var handle uintptr
	var tried error
	for _, p := range places {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if h, err := openLibraryBeside(p); err == nil {
			handle = h
			break
		} else if tried == nil {
			tried = err
		}
	}
	if handle == 0 {
		why := "it isn't next to the game"
		if tried != nil {
			why = tried.Error()
		}
		return fmt.Errorf("Steam needs the Steamworks API, %s: run golib steam-sdk with the Steamworks SDK, then build the game again with golib and --tags steam (%s)", name, why)
	}
	for _, f := range []struct {
		to   any
		name string
	}{
		{&steam.restartAppIfNecessary, "SteamAPI_RestartAppIfNecessary"},
		{&steam.initFlat, "SteamAPI_InitFlat"},
		{&steam.runCallbacks, "SteamAPI_RunCallbacks"},
		{&steam.shutdown, "SteamAPI_Shutdown"},
		{&steam.userStatsInterface, steamUserStatsInterface},
		{&steam.appsInterface, steamAppsInterface},
		{&steam.utilsInterface, steamUtilsInterface},
		{&steam.setAchievement, "SteamAPI_ISteamUserStats_SetAchievement"},
		{&steam.getAchievement, "SteamAPI_ISteamUserStats_GetAchievement"},
		{&steam.clearAchievement, "SteamAPI_ISteamUserStats_ClearAchievement"},
		{&steam.storeStats, "SteamAPI_ISteamUserStats_StoreStats"},
		{&steam.indicateProgress, "SteamAPI_ISteamUserStats_IndicateAchievementProgress"},
		{&steam.currentGameLanguage, "SteamAPI_ISteamApps_GetCurrentGameLanguage"},
		{&steam.runningOnSteamDeck, "SteamAPI_ISteamUtils_IsSteamRunningOnSteamDeck"},
	} {
		p, err := librarySymbol(handle, f.name)
		if err != nil || p == 0 {
			return fmt.Errorf("%s has no %s: it is from a Steamworks SDK other than the one this GoLib calls (see docs/tooling.md#steamworks)", name, f.name)
		}
		// purego reads a C bool from the low byte of its register, and
		// passes Go strings as C text that lives for the call.
		purego.RegisterFunc(f.to, p)
	}
	return nil
}

// SteamStart loads the Steamworks API and starts it for the game appID, and
// says why it couldn't. With restartThroughSteam, as in a dist build, a game
// that Steam didn't start asks Steam to start it, and relaunched says that
// Steam will: the game should then end at once. Otherwise, as in a debug
// build, the game tells Steam its app ID itself.
func SteamStart(appID uint32, restartThroughSteam bool) (relaunched bool, err error) {
	if err := loadSteam(); err != nil {
		return false, err
	}
	if restartThroughSteam {
		if steam.restartAppIfNecessary(appID) {
			return true, nil
		}
	} else if os.Getenv("SteamAppId") == "" {
		// What Steam sets for a game it starts, and what steam_appid.txt
		// says when it is in the working directory, which golib run's
		// isn't.
		os.Setenv("SteamAppId", strconv.FormatUint(uint64(appID), 10))
	}
	var message [1024]byte // SteamErrMsg
	result := steam.initFlat(&message[0])
	text := goString(message[:])
	if result != steamInitOK {
		why := "check that Steam is running and logged in"
		switch result {
		case steamInitNoSteamClient:
			why = "Steam isn't running, or isn't logged in"
		case steamInitVersionMismatch:
			why = "the Steam client is older than the Steamworks API: update Steam"
		}
		if text != "" {
			why += " (Steam says: " + text + ")"
		}
		return false, fmt.Errorf("Steam didn't start: %s", why)
	}
	steam.userStats = steam.userStatsInterface()
	steam.apps = steam.appsInterface()
	steam.utils = steam.utilsInterface()
	if steam.userStats == 0 || steam.apps == 0 || steam.utils == 0 {
		steam.shutdown()
		return false, errors.New("Steam started without the interfaces this GoLib calls: the Steam client may be too old (see docs/tooling.md#steamworks)")
	}
	steam.running = true
	return false, nil
}

// SteamRunCallbacks lets Steam do its work, once a frame.
func SteamRunCallbacks() {
	if steam.running {
		steam.runCallbacks()
	}
}

// SteamStop ends Steam, when the game ends.
func SteamStop() {
	if steam.running {
		steam.running = false
		steam.shutdown()
	}
}

// SteamSetAchievement marks the achievement id unlocked, in memory, and says
// whether Steam took it; SteamStoreStats sends it.
func SteamSetAchievement(id string) bool {
	return steam.running && steam.setAchievement(steam.userStats, id)
}

// SteamAchievement says whether the achievement id is unlocked; ok is false
// when Steam doesn't know it.
func SteamAchievement(id string) (unlocked, ok bool) {
	if !steam.running {
		return false, false
	}
	ok = steam.getAchievement(steam.userStats, id, &unlocked)
	return unlocked && ok, ok
}

// SteamClearAchievement locks the achievement id again, in memory.
func SteamClearAchievement(id string) bool {
	return steam.running && steam.clearAchievement(steam.userStats, id)
}

// SteamStoreStats sends the achievements set and cleared to Steam, and says
// whether it could.
func SteamStoreStats() bool {
	return steam.running && steam.storeStats(steam.userStats)
}

// SteamAchievementProgress shows Steam's notice of progress toward the
// achievement id.
func SteamAchievementProgress(id string, current, max uint32) {
	if steam.running {
		steam.indicateProgress(steam.userStats, id, current, max)
	}
}

// SteamLanguage is the language the player chose for the game in Steam, by
// Steam's name for it, such as "english" or "spanish".
func SteamLanguage() string {
	if !steam.running {
		return ""
	}
	return steam.currentGameLanguage(steam.apps)
}

// SteamOnDeck says whether the game runs on a Steam Deck.
func SteamOnDeck() bool {
	return steam.running && steam.runningOnSteamDeck(steam.utils)
}
