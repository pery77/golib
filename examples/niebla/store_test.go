package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golib"
)

// openStoreAt opens a store over one file, closed when the test ends.
func openStoreAt(t *testing.T, path string) *store {
	t.Helper()
	st, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { st.close() })
	return st
}

// openTestStore opens a store over a fresh file of the test's own.
func openTestStore(t *testing.T) (*store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "niebla.db")
	return openStoreAt(t, path), path
}

func TestAPlayerKeepsItsIdentity(t *testing.T) {
	st, path := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	if want := playerIDOf("machine-one"); id != want {
		t.Fatalf("player gave %q, want the machine's hash %q", id, want)
	}
	st.close()
	reopened := openStoreAt(t, path)
	again, err := reopened.player("machine-one")
	if err != nil {
		t.Fatalf("player again: %v", err)
	}
	if again != id {
		t.Fatalf("the identity changed across runs: %q then %q", id, again)
	}
}

func TestAFallbackPlayerPersists(t *testing.T) {
	st, path := openTestStore(t)
	first, err := st.player("")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	if first == playerIDOf("") {
		t.Fatal("the fallback identity is the empty machine's hash, not a random one")
	}
	if len(first) != 64 {
		t.Fatalf("the fallback identity is %q, not 64 hex characters", first)
	}
	st.close()
	reopened := openStoreAt(t, path)
	again, err := reopened.player("")
	if err != nil {
		t.Fatalf("player again: %v", err)
	}
	if again != first {
		t.Fatalf("the fallback identity changed across runs: %q then %q", first, again)
	}
}

func TestARegisteredPlayerHasATokenWaiting(t *testing.T) {
	st, _ := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	var token string
	if err := st.sql.QueryRow(
		`SELECT token FROM players WHERE id = ?`, id,
	).Scan(&token); err != nil {
		t.Fatalf("the player row is missing: %v", err)
	}
	if token != "" {
		t.Fatalf("a new player carries a token %q already", token)
	}
}

func TestSaveThenLoadRoundTripsTheBase(t *testing.T) {
	st, _ := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	want := newGame()
	for i := 0; i < 200; i++ {
		Apply(want, Tick{})
	}
	if err := st.saveState(id, regionSlot, want); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	got, found, err := st.loadState(id, regionSlot)
	if err != nil || !found {
		t.Fatalf("loadState: found %v, err %v", found, err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("the base that came back isn't the base that was saved")
	}
}

func TestSavingAgainReplacesTheBase(t *testing.T) {
	st, _ := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	first := newGame()
	if err := st.saveState(id, regionSlot, first); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	second := newGame()
	for i := 0; i < 500; i++ {
		Apply(second, Tick{})
	}
	if err := st.saveState(id, regionSlot, second); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	got, found, err := st.loadState(id, regionSlot)
	if err != nil || !found {
		t.Fatalf("loadState: found %v, err %v", found, err)
	}
	if got.Ticks != second.Ticks {
		t.Fatalf("the save came back at tick %d, want the latest %d", got.Ticks, second.Ticks)
	}
	if !reflect.DeepEqual(second, got) {
		t.Fatal("the save that came back isn't the latest one")
	}
}

func TestAPlayerLoadsOnlyItsOwnBase(t *testing.T) {
	st, _ := openTestStore(t)
	one, err := st.player("machine-one")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	other, err := st.player("machine-two")
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	if err := st.saveState(one, regionSlot, newGame()); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	if _, found, err := st.loadState(other, regionSlot); err != nil || found {
		t.Fatalf("another player's load: found %v, err %v, want none", found, err)
	}
}

func TestStorePath(t *testing.T) {
	path, err := storePath(
		func(string) string { return "build/niebla/shots" },
		func() (string, error) { return "/home/x/.config", nil },
	)
	if err != nil || path != ":memory:" {
		t.Fatalf("under golib shot the store is %q, err %v, want :memory:", path, err)
	}
	path, err = storePath(
		func(string) string { return "" },
		func() (string, error) { return "/home/x/.config", nil },
	)
	if err != nil {
		t.Fatalf("storePath: %v", err)
	}
	if want := filepath.Join("/home/x/.config", "GoLib games", "niebla", "niebla.db"); path != want {
		t.Fatalf("storePath gave %q, want %q", path, want)
	}
	if _, err := storePath(
		func(string) string { return "" },
		func() (string, error) { return "", errors.New("gone") },
	); err == nil {
		t.Fatal("a missing settings folder must be an error")
	}
}

func TestResumeStatePrefersTheShotSeed(t *testing.T) {
	// Under go test the saved-data channel is memory, so the test can
	// seed it the way golib shot --save does and resumeState must take
	// it over the database.
	seeded := newGame()
	for i := 0; i < 123; i++ {
		Apply(seeded, Tick{})
	}
	if err := golib.SaveData("state", seeded); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	defer golib.DeleteData("state")
	got, found, err := resumeState()
	if err != nil || !found {
		t.Fatalf("resumeState: found %v, err %v", found, err)
	}
	if got.Ticks != seeded.Ticks {
		t.Fatalf("resumeState gave tick %d, want the seeded %d", got.Ticks, seeded.Ticks)
	}
}

func TestSaveSlotsKeepIndependentColonies(t *testing.T) {
	st, _ := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldPlayer, oldResize := db, player, activeResize
	db, player = st, id
	t.Cleanup(func() {
		db, player, activeResize = oldDB, oldPlayer, oldResize
	})
	first := newPlayScene(newGameOn(17))
	first.state.Ticks = 1234
	first.saveNow()
	if first.saveFailed {
		t.Fatal("the first colony could not be saved")
	}
	slot, err := st.nextSlot(id)
	if err != nil || slot != "2" {
		t.Fatalf("next slot = %q, error = %v", slot, err)
	}
	second := newPlayScene(nil)
	second.slot = slot
	second.state.Ticks = 60
	second.saveNow()
	if second.saveFailed {
		t.Fatal("the second colony could not be saved")
	}
	for _, scene := range []*playScene{first, second} {
		loaded, found, err := st.loadState(id, scene.slot)
		if err != nil || !found {
			t.Fatalf("load %s: found = %v, error = %v", scene.slot, found, err)
		}
		if !reflect.DeepEqual(loaded, scene.state) {
			t.Fatalf("Save %s was overwritten by another colony", scene.slot)
		}
	}
	for slot, stamp := range map[string]string{
		"1": "2026-10-01T12:00:00.987654321Z",
		"2": "2026-10-01T12:00:00.123456789Z",
	} {
		if _, err := st.sql.Exec(
			`UPDATE saves SET updated_at = ? WHERE player = ? AND slot = ?`,
			stamp, id, slot,
		); err != nil {
			t.Fatal(err)
		}
	}
	saves, err := st.listSaves(id)
	if err != nil || len(saves) != 2 {
		t.Fatalf("list: %v, error = %v", saves, err)
	}
	if saves[0].Slot != "1" || saves[1].Slot != "2" ||
		saves[0].Ticks != 1234 || saves[1].Ticks != 60 {
		t.Fatalf("wrong list order or play times: %+v", saves)
	}
	if latestSave(saves).Slot != "1" {
		t.Fatal("Continue chose the newest slot instead of the last played one")
	}
	resumed, found, err := resumeState()
	if err != nil || !found || resumed.Seed != 17 {
		t.Fatalf("resume: state = %+v, found = %v, error = %v",
			resumed, found, err)
	}
	other, err := st.player("machine-two")
	if err != nil {
		t.Fatal(err)
	}
	if saves, err := st.listSaves(other); err != nil || len(saves) != 0 {
		t.Fatalf("another player's list: %+v, error = %v", saves, err)
	}
	if slot, err := st.nextSlot(other); err != nil || slot != "1" {
		t.Fatalf("another player's first slot = %q, error = %v", slot, err)
	}
}

func TestLegacySaveBecomesTheFirstSlotAcrossRuns(t *testing.T) {
	st, path := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatal(err)
	}
	want := newGameOn(23)
	want.Ticks = 5678
	if err := st.saveState(id, "region", want); err != nil {
		t.Fatal(err)
	}
	st.close()
	for range 2 {
		reopened := openStoreAt(t, path)
		got, found, err := reopened.loadState(id, "1")
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("migrated colony: found = %v, error = %v", found, err)
		}
		saves, err := reopened.listSaves(id)
		if err != nil || len(saves) != 1 || saves[0].Slot != "1" {
			t.Fatalf("migration duplicated a save: %+v, error = %v", saves, err)
		}
		if slot, err := reopened.nextSlot(id); err != nil || slot != "2" {
			t.Fatalf("next slot after migration = %q, error = %v", slot, err)
		}
		reopened.close()
	}
}

func TestUnreadableSaveDoesNotStartANewColony(t *testing.T) {
	st, _ := openTestStore(t)
	id, err := st.player("machine-one")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.sql.Exec(
		`INSERT INTO saves (player, slot, state, ticks, updated_at)
		 VALUES (?, '1', 'not JSON', 0, ?)`,
		id, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}
	oldDB, oldPlayer, oldResize := db, player, activeResize
	db, player = st, id
	t.Cleanup(func() {
		db, player, activeResize = oldDB, oldPlayer, oldResize
	})
	if err := loadGame("1"); err == nil {
		t.Fatal("loading an unreadable save succeeded")
	}
	var data string
	if err := st.sql.QueryRow(
		`SELECT state FROM saves WHERE player = ? AND slot = '1'`, id,
	).Scan(&data); err != nil || data != "not JSON" {
		t.Fatalf("failed load replaced the save: %q, error = %v", data, err)
	}
}
