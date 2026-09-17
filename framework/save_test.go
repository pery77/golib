package golib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type savedProgress struct {
	Level int
	Best  map[string]float32
	note  string // not saved
}

func TestSaveDataInMemory(t *testing.T) {
	// Tests keep saved data in memory.
	if err := DeleteData("progress"); err != nil {
		t.Fatal(err)
	}
	var progress savedProgress
	if found, err := LoadData("progress", &progress); found || err != nil {
		t.Fatalf("LoadData before any save: %v, %v", found, err)
	}
	if err := SaveData("progress", savedProgress{Level: 3, Best: map[string]float32{"1": 12.5}, note: "x"}); err != nil {
		t.Fatal(err)
	}
	progress = savedProgress{Level: 1}
	found, err := LoadData("progress", &progress)
	if !found || err != nil || progress.Level != 3 || progress.Best["1"] != 12.5 || progress.note != "" {
		t.Errorf("LoadData = %v, %v, %+v", found, err, progress)
	}
	if err := DeleteData("progress"); err != nil {
		t.Fatal(err)
	}
	if found, _ := LoadData("progress", &progress); found {
		t.Error("LoadData found data after DeleteData")
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveDataOnDisk(t *testing.T) {
	testSaveFolder = filepath.Join(t.TempDir(), "save")
	t.Cleanup(func() { testSaveFolder = "" })

	settings := map[string]any{"volume": 0.5, "fullscreen": true}
	if err := SaveData("settings", settings); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(testSaveFolder, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"fullscreen\": true,\n  \"volume\": 0.5\n}\n"; string(data) != want {
		t.Errorf("settings.json holds %q, want %q", data, want)
	}
	// Saving again replaces the file, and leaves no other file behind.
	if err := SaveData("settings", map[string]any{"volume": 1}); err != nil {
		t.Fatal(err)
	}
	var loaded struct{ Volume float32 }
	if found, err := LoadData("settings", &loaded); !found || err != nil || loaded.Volume != 1 {
		t.Errorf("LoadData = %v, %v, %+v", found, err, loaded)
	}
	if entries, _ := os.ReadDir(testSaveFolder); len(entries) != 1 {
		t.Errorf("the save folder holds %d files, want 1", len(entries))
	}

	// A damaged file is an error, not data.
	if err := os.WriteFile(filepath.Join(testSaveFolder, "settings.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded.Volume = 0.25
	found, err := LoadData("settings", &loaded)
	if found || err == nil || !strings.Contains(err.Error(), "settings.json can't be read") {
		t.Errorf("LoadData of a damaged file = %v, %v", found, err)
	}

	if err := DeleteData("settings"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteData("settings"); err != nil {
		t.Errorf("deleting what isn't there: %v", err)
	}
	if found, err := LoadData("settings", &loaded); found || err != nil {
		t.Errorf("LoadData after DeleteData = %v, %v", found, err)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveDataMistakes(t *testing.T) {
	takeError()
	for _, name := range []string{"", "Progress", "level/1", "../x", strings.Repeat("a", 65)} {
		if err := SaveData(name, 1); err == nil {
			t.Errorf("SaveData(%q): no error", name)
		}
		wantError(t, "isn't valid: use lowercase letters, digits, - and _")
	}
	type lowercase struct{ level, score int }
	if err := SaveData("progress", lowercase{level: 2}); err == nil {
		t.Error("SaveData of a struct with no exported fields: no error")
	}
	wantError(t, "golib.SaveData: golib.lowercase has no exported fields")

	var level int
	if _, err := LoadData("progress", level); err == nil {
		t.Error("LoadData into a value, not a pointer: no error")
	}
	wantError(t, "golib.LoadData: got int for \"progress\": pass a pointer")

	if err := SaveData("channel", make(chan int)); err == nil || !strings.Contains(err.Error(), "can't be saved as JSON") {
		t.Errorf("SaveData of a channel: %v", err)
	}
}

func TestDistSaveFolder(t *testing.T) {
	settings := filepath.Join("home", "config")
	tests := []struct{ exe, name, want string }{
		{filepath.Join("games", "crates.exe"), "crates", filepath.Join(settings, "GoLib games", "crates")},
		{filepath.Join("anywhere", "renamed.exe"), "crates", filepath.Join(settings, "GoLib games", "crates")},
		{filepath.Join("games", "rocks"), "", filepath.Join(settings, "GoLib games", "rocks")},
		{filepath.Join("games", "rocks.exe"), "", filepath.Join(settings, "GoLib games", "rocks")},
	}
	for _, test := range tests {
		if got := distSaveFolder(settings, test.exe, test.name); got != test.want {
			t.Errorf("distSaveFolder(%q, %q) = %q, want %q", test.exe, test.name, got, test.want)
		}
	}
}
