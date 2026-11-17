package golib

import (
	"slices"
	"testing"
)

// Achievements wait until Steam takes them, and are stored once, again a
// second later when Steam couldn't store them.
func TestSteamAchievementsWaitForSteam(t *testing.T) {
	var set, stored []string
	accept, storeWorks := map[string]bool{"ACH_A": true}, false
	saved := steamCalls
	defer func() {
		steamCalls = saved
		steamState.running, steamState.pending, steamState.done = false, nil, nil
		steamState.store, steamState.fresh = false, false
	}()
	steamCalls.set = func(id string) bool {
		set = append(set, id)
		return accept[id]
	}
	steamCalls.store = func() bool {
		stored = append(stored, "store")
		return storeWorks
	}
	steamCalls.clear = func(id string) bool { return true }
	steamState.running, steamState.frames = true, 0
	steamState.pending, steamState.fresh = []string{"ACH_A", "ACH_B"}, true

	flushSteam(false)
	if !slices.Equal(set, []string{"ACH_A", "ACH_B"}) || len(stored) != 1 {
		t.Fatalf("first frame: set %q, stored %d times", set, len(stored))
	}
	if !slices.Equal(steamState.pending, []string{"ACH_B"}) || !steamState.done["ACH_A"] {
		t.Fatalf("after the first frame: pending %q, done %v", steamState.pending, steamState.done)
	}
	// The next frames wait for the second's end.
	set, stored = nil, nil
	for range targetFPS - 2 {
		flushSteam(false)
	}
	if len(set) != 0 || len(stored) != 0 {
		t.Fatalf("frames within the second: set %q, stored %d times", set, len(stored))
	}
	accept["ACH_B"], storeWorks = true, true
	flushSteam(false)
	if !slices.Equal(set, []string{"ACH_B"}) || len(stored) != 1 || len(steamState.pending) != 0 || steamState.store {
		t.Fatalf("a second later: set %q, stored %d times, pending %q", set, len(stored), steamState.pending)
	}
	// Nothing new: nothing sent, and no store.
	set, stored = nil, nil
	for range 3 * targetFPS {
		flushSteam(false)
	}
	if len(set) != 0 || len(stored) != 0 {
		t.Errorf("with nothing waiting: set %q, stored %d times", set, len(stored))
	}
	// Cleared, an achievement can be unlocked again.
	SteamClearAchievement("ACH_A")
	if steamState.done["ACH_A"] || len(stored) != 1 {
		t.Errorf("clearing: done %v, stored %d times", steamState.done, len(stored))
	}
}
