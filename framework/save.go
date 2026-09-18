package golib

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"golib/internal/device"
)

// saveFolderName is the folder, in the player's settings folder, that holds
// the saved data of every game made with GoLib, one folder per game, so that
// a game never writes into another program's folder.
const saveFolderName = "GoLib games"

// saveName names a dist build's folder in saveFolderName. golib dist sets it
// to the game's folder name, with -ldflags=-X golib.saveName=<game>.
var saveName string

// memorySaves holds the saved data of a program that saves nothing to disk:
// a test, or a game under golib shot.
var memorySaves struct {
	sync.Mutex
	data map[string][]byte
}

// init puts the data of golib shot --save in memory before main runs, so that
// a scene built in a package variable, or in main before Run, already finds
// what LoadData would have read from a player's save file.
func init() { seedShotSaves(os.Getenv, os.ReadFile) }

// seedShotSaves reads the file shotSaveEnv names, if any, into the data this
// program keeps in memory. A file that can't be used stops Run, as any other
// mistake made before Run does.
func seedShotSaves(getenv func(string) string, readFile func(string) ([]byte, error)) {
	path := getenv(shotSaveEnv)
	if path == "" {
		return
	}
	if err := readShotSaves(path, readFile); err != nil {
		reportError(err)
	}
}

// readShotSaves puts the contents of a golib shot --save file in memory: a
// JSON object with one saved value per name, as SaveData stores them.
func readShotSaves(path string, readFile func(string) ([]byte, error)) error {
	data, err := readFile(path)
	if err != nil {
		return fmt.Errorf("golib.Run: cannot read %s, the file golib shot --save was given: %w", path, err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf(`golib.Run: %s isn't saved data for golib shot --save: it holds one value per name, such as {"progress": {"Level": 5}}: %w`, path, err)
	}
	for _, name := range slices.Sorted(maps.Keys(saved)) {
		if !validSaveName(name) {
			return fmt.Errorf("golib.Run: %s: %q isn't a valid name for saved data: use lowercase letters, digits, - and _, such as \"progress\"", path, name)
		}
	}
	memorySaves.Lock()
	defer memorySaves.Unlock()
	if memorySaves.data == nil {
		memorySaves.data = map[string][]byte{}
	}
	for name, value := range saved {
		memorySaves.data[name] = value
	}
	return nil
}

// SaveData stores value under name, such as "progress" or "settings",
// replacing what was stored there before, so that the next time the game
// runs LoadData finds it:
//
//	type progress struct {
//		Level     int // exported: JSON leaves out fields in lower case
//		BestTimes []float32
//	}
//
//	if err := golib.SaveData("progress", s.progress); err != nil {
//		s.message = "Could not save your progress"
//	}
//
// The value is stored as JSON, so only exported fields are saved, and it
// should be small: settings, scores, how far the player got. Names are
// lowercase letters, digits, - and _.
//
// A golib dist build saves in the player's settings folder, in "GoLib
// games/<game>" (on Windows, %AppData%\GoLib games\<game>). A debug build
// saves next to its executable, in build/<game>/save/, which golib clean
// deletes. Under golib shot and go test nothing is written: data is kept in
// memory until the program ends, so screenshots and tests start with nothing
// saved, unless golib shot --save gives the game data to start from, such as
// a finished level (see docs/tooling.md#screenshots). SaveData returns an
// error when the data can't be written, such as on a full disk; the game
// should tell the player and carry on.
func SaveData(name string, value any) error {
	if err := checkSaveName("SaveData", name); err != nil {
		return err
	}
	if err := checkSavedValue(value); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("golib.SaveData: %q can't be saved as JSON: %w", name, err)
	}
	data = append(data, '\n')

	folder, inMemory, err := saveFolder()
	if err != nil {
		return fmt.Errorf("golib.SaveData: %w", err)
	}
	if inMemory {
		memorySaves.Lock()
		defer memorySaves.Unlock()
		if memorySaves.data == nil {
			memorySaves.data = map[string][]byte{}
		}
		memorySaves.data[name] = data
		return nil
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("golib.SaveData: %w", err)
	}
	// Write a new file, then put it in place of the old one, so that a game
	// that stops halfway leaves the old data whole.
	path := filepath.Join(folder, name+".json")
	temporary, err := os.CreateTemp(folder, name+"-*.tmp")
	if err != nil {
		return fmt.Errorf("golib.SaveData: %w", err)
	}
	_, err = temporary.Write(data)
	err = errors.Join(err, temporary.Close())
	if err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		os.Remove(temporary.Name())
		return fmt.Errorf("golib.SaveData: %w", err)
	}
	return nil
}

// LoadData reads what SaveData stored under name into value, which must be a
// pointer, and reports whether anything was stored. When nothing was, value
// stays as it is, so set its defaults first:
//
//	s.progress = progress{Level: 1}
//	if _, err := golib.LoadData("progress", &s.progress); err != nil {
//		s.message = "Your saved progress is damaged: starting again"
//	}
//
// It returns an error, and found is false, when the stored data can't be
// read or isn't what value holds, such as a file edited by hand.
func LoadData(name string, value any) (found bool, err error) {
	if err := checkSaveName("LoadData", name); err != nil {
		return false, err
	}
	if v := reflect.ValueOf(value); v.Kind() != reflect.Pointer || v.IsNil() {
		err := fmt.Errorf("golib.LoadData: got %T for %q: pass a pointer to the value to fill, such as &s.progress", value, name)
		reportError(err)
		return false, err
	}
	folder, inMemory, err := saveFolder()
	if err != nil {
		return false, fmt.Errorf("golib.LoadData: %w", err)
	}
	var data []byte
	path := name + ".json"
	if inMemory {
		memorySaves.Lock()
		stored, ok := memorySaves.data[name]
		memorySaves.Unlock()
		if !ok {
			return false, nil
		}
		data = stored
	} else {
		path = filepath.Join(folder, path)
		data, err = os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("golib.LoadData: %w", err)
		}
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, fmt.Errorf("golib.LoadData: %s can't be read: %w", path, err)
	}
	return true, nil
}

// DeleteData removes what SaveData stored under name, such as when the player
// starts over. Nothing stored is not an error. In tests, call it first when a
// test needs nothing saved, since the tests of a program share their data.
func DeleteData(name string) error {
	if err := checkSaveName("DeleteData", name); err != nil {
		return err
	}
	folder, inMemory, err := saveFolder()
	if err != nil {
		return fmt.Errorf("golib.DeleteData: %w", err)
	}
	if inMemory {
		memorySaves.Lock()
		defer memorySaves.Unlock()
		delete(memorySaves.data, name)
		return nil
	}
	err = os.Remove(filepath.Join(folder, name+".json"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("golib.DeleteData: %w", err)
	}
	return nil
}

// testSaveFolder, when set, is where this package's tests save to disk.
var testSaveFolder string

// saveFolder returns the folder saved data goes in, or inMemory when this
// program keeps its data in memory.
func saveFolder() (folder string, inMemory bool, err error) {
	if testSaveFolder != "" {
		return testSaveFolder, false, nil
	}
	// Tests, golib shot and backends with no files of their own, such as a
	// page in a browser, keep saved data in memory for the run.
	if testing.Testing() || os.Getenv(shotDirEnv) != "" || !device.SavesToDisk {
		return "", true, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if !distBuild {
		return filepath.Join(filepath.Dir(exe), "save"), false, nil
	}
	settings, err := os.UserConfigDir()
	if err != nil {
		return "", false, err
	}
	return distSaveFolder(settings, exe, saveName), false, nil
}

// distSaveFolder returns the folder a dist build saves in, given the player's
// settings folder, the executable's path and the game's name, which is the
// executable's when name is empty.
func distSaveFolder(settings, exe, name string) string {
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	}
	return filepath.Join(settings, saveFolderName, name)
}

// validSaveName reports whether name can be a file name everywhere.
func validSaveName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// checkSaveName reports a name that can't be a file name everywhere, as a
// mistake that stops Run.
func checkSaveName(function, name string) error {
	if validSaveName(name) {
		return nil
	}
	err := fmt.Errorf("golib.%s: the name %q isn't valid: use lowercase letters, digits, - and _, such as \"progress\"", function, name)
	reportError(err)
	return err
}

// checkSavedValue reports a struct with fields but no exported ones, which
// JSON would save as nothing at all, as a mistake that stops Run.
func checkSavedValue(value any) error {
	t := reflect.TypeOf(value)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || t.NumField() == 0 {
		return nil
	}
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			return nil
		}
	}
	err := fmt.Errorf("golib.SaveData: %s has no exported fields, so nothing of it would be saved: start the names of the fields to save with a capital letter", t)
	reportError(err)
	return err
}
