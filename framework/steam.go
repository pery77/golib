package golib

import (
	"errors"
	"slices"
	"strconv"
	"sync"

	"golib/internal/device"
)

// Steam is Valve's store, and its Steamworks API what a game sold there
// calls for achievements and the player's language. It is in a game built
// with the build tag steam, golib build --tags steam, whose game.json gives
// its "steamAppID", once golib steam-sdk has copied the API from Valve's SDK
// (see docs/tooling.md#steamworks). Run starts it before the window opens
// and lets it work each frame; without the tag, and in a browser, every
// function here does nothing, and the game neither carries nor needs
// Steam's library.

// steamAppID is the "steamAppID" of the game's game.json. The golib
// commands set it in every build they make, with -ldflags=-X
// golib.steamAppID=<id>.
var steamAppID string

// steamState is Steam as the game sees it.
var steamState struct {
	sync.Mutex
	running bool
	err     error // why it isn't running, once Run has tried

	pending []string        // achievements to unlock, which Steam hasn't taken yet
	done    map[string]bool // achievements Steam took since the game started
	store   bool            // something changed that Steam hasn't stored
	fresh   bool            // something to try in the next frame, not a second later
	frames  int             // frames since Steam started
}

// steamCalls are the calls to Steam that the achievements make, which
// tests replace.
var steamCalls = struct {
	set, clear func(id string) bool
	store      func() bool
}{device.SteamSetAchievement, device.SteamClearAchievement, device.SteamStoreStats}

// SteamRunning reports whether the game is connected to Steam: it was built
// with the build tag steam, Run has started, and Steam started with it. It
// is false in a game Steam isn't running for, which plays on without it.
func SteamRunning() bool {
	steamState.Lock()
	defer steamState.Unlock()
	return steamState.running
}

// SteamError says why SteamRunning is false: the game wasn't built with the
// build tag steam, Steam's library is missing, Steam isn't running or isn't
// logged in, or Run hasn't started yet. It is nil while Steam runs. Run
// prints it once, too, when Steam doesn't start.
func SteamError() error {
	steamState.Lock()
	defer steamState.Unlock()
	switch {
	case steamState.running:
		return nil
	case steamState.err != nil:
		return steamState.err
	}
	if err := device.SteamUnavailable(); err != nil {
		return err
	}
	return errors.New("Steam starts when golib.Run does: ask once the game runs")
}

// SteamUnlockAchievement unlocks the achievement id, its API name in the
// game's Steamworks settings, such as "ACH_WIN_ONE_GAME", and Steam shows
// it to the player. Call it as often as you like: an achievement unlocked
// already stays unlocked, and Steam is told once. Steam takes it in the
// next frame, or as soon as it can if it can't yet, and stores it then; an
// achievement unlocked before Run starts is sent once Steam does. Without
// Steam it does nothing.
func SteamUnlockAchievement(id string) {
	steamState.Lock()
	defer steamState.Unlock()
	if device.SteamUnavailable() != nil || steamState.done[id] || slices.Contains(steamState.pending, id) {
		return
	}
	steamState.pending = append(steamState.pending, id)
	steamState.fresh = true
}

// SteamAchievement reports whether the achievement id is unlocked for the
// player. ok is false without Steam, and when Steam doesn't know the
// achievement: check its API name in the game's Steamworks settings.
func SteamAchievement(id string) (unlocked, ok bool) {
	if !SteamRunning() {
		return false, false
	}
	return device.SteamAchievement(id)
}

// SteamClearAchievement locks the achievement id again, for trying an
// achievement while making the game: players never lose one. Without Steam
// it does nothing.
func SteamClearAchievement(id string) {
	steamState.Lock()
	defer steamState.Unlock()
	steamState.pending = slices.DeleteFunc(steamState.pending, func(p string) bool { return p == id })
	delete(steamState.done, id)
	if !steamState.running || !steamCalls.clear(id) {
		return
	}
	if !steamCalls.store() {
		steamState.store = true // try again in a second
	}
}

// SteamAchievementProgress shows Steam's notice of the player's progress
// toward the achievement id, such as 50 of 100 coins: current of max. It
// unlocks nothing: call SteamUnlockAchievement when current reaches max.
// Without Steam it does nothing.
func SteamAchievementProgress(id string, current, max int) {
	if current < 0 || max <= 0 || !SteamRunning() {
		return
	}
	device.SteamAchievementProgress(id, uint32(min(current, max)), uint32(max))
}

// SteamLanguage returns the language the player chose for the game in
// Steam, by Steam's name for it: "english", "spanish", "latam" for Latin
// American Spanish, "schinese" for Simplified Chinese, and so on. It is ""
// without Steam: then follow SystemLanguages.
func SteamLanguage() string {
	if !SteamRunning() {
		return ""
	}
	return device.SteamLanguage()
}

// SteamOnDeck reports whether the game runs on a Steam Deck, for a game that
// sizes its text or its controls for one. It is false without Steam.
func SteamOnDeck() bool {
	if !SteamRunning() {
		return false
	}
	return device.SteamOnDeck()
}

// startSteam starts Steam, in a build with the tag steam, before Run opens
// the window, as Steam's overlay needs. In a dist build that Steam didn't
// start, Steam starts the game again itself, and startSteam returns true:
// the game must end. A Steam that doesn't start leaves the game playing
// without it, and says why once.
func startSteam() (relaunched bool) {
	if device.SteamUnavailable() != nil {
		return false
	}
	id, err := strconv.ParseUint(steamAppID, 10, 32)
	if err != nil || id == 0 {
		err = errors.New(`Steam needs the game's app ID: add "steamAppID" to game.json and build the game with golib`)
	} else {
		relaunched, err = device.SteamStart(uint32(id), distBuild)
	}
	if relaunched {
		warnf("golib: Steam didn't start the game, so the game asks Steam to start it, and ends (see docs/tooling.md#steamworks)")
		return true
	}
	steamState.Lock()
	defer steamState.Unlock()
	if err != nil {
		steamState.err = err
		warnf("golib: playing without Steam: %v", err)
		return false
	}
	steamState.running, steamState.err = true, nil
	steamState.frames = 0
	steamState.fresh = len(steamState.pending) > 0
	return false
}

// steamOff leaves Steam off where Run runs a game for golib shot, so that
// shots repeat and unlock nothing on the developer's account.
func steamOff(why string) {
	if device.SteamUnavailable() != nil {
		return
	}
	steamState.Lock()
	defer steamState.Unlock()
	steamState.err = errors.New("Steam stays off " + why)
}

// steamFrame lets Steam do its work, once a frame, and sends it the
// achievements the game unlocked.
func steamFrame() {
	if !SteamRunning() {
		return
	}
	device.SteamRunCallbacks()
	flushSteam(false)
}

// flushSteam gives Steam the achievements waiting for it and stores them:
// in the frame after they were unlocked, then once a second for those Steam
// didn't take, and at once when now is true.
func flushSteam(now bool) {
	steamState.Lock()
	defer steamState.Unlock()
	if !steamState.running {
		return
	}
	steamState.frames++
	if !now && !steamState.fresh && steamState.frames%targetFPS != 0 {
		return
	}
	steamState.fresh = false
	kept := steamState.pending[:0]
	for _, id := range steamState.pending {
		if !steamCalls.set(id) {
			kept = append(kept, id)
			continue
		}
		if steamState.done == nil {
			steamState.done = map[string]bool{}
		}
		steamState.done[id] = true
		steamState.store = true
	}
	steamState.pending = kept
	if steamState.store && steamCalls.store() {
		steamState.store = false
	}
}

// stopSteam sends Steam what is waiting and ends it, when Run returns.
func stopSteam() {
	if !SteamRunning() {
		return
	}
	flushSteam(true)
	device.SteamStop()
	steamState.Lock()
	defer steamState.Unlock()
	steamState.running = false
	steamState.err = errors.New("Steam stopped when golib.Run returned")
}
