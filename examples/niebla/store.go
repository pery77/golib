package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"

	"golib"
)

// The local database keeps player identities and one JSON colony per slot.

// db is the game's local database, nil when saving is off. resolvePlayer
// opens it before the first scene; the scenes reach it through saveBase
// and resumeState.
var db *store

// saveWarning says why saving is off, when it is, for the menu to show.
var saveWarning string

const regionSlot = "1"

type saveInfo struct {
	Slot      string
	Ticks     int64
	UpdatedAt time.Time
}

func latestSave(saves []saveInfo) saveInfo {
	var latest saveInfo
	for _, saved := range saves {
		if !saved.UpdatedAt.Before(latest.UpdatedAt) {
			latest = saved
		}
	}
	return latest
}

// store is the local database.
type store struct {
	sql *sql.DB
}

// storePath returns where the database file goes: the player's settings
// folder, in the same place a GoLib game keeps its saves, so a debug
// build and a dist one share the base. Under golib shot it returns
// ":memory:", and nothing is ever written.
func storePath(getenv func(string) string, userDir func() (string, error)) (string, error) {
	if getenv("GOLIB_SHOT_DIR") != "" {
		return ":memory:", nil
	}
	settings, err := userDir()
	if err != nil {
		return "", fmt.Errorf("can't find the settings folder: %w", err)
	}
	if settings == "" {
		return "", errors.New("can't find the settings folder")
	}
	return filepath.Join(settings, "GoLib games", "niebla", "niebla.db"), nil
}

// openStore opens the database — making file and tables when they are
// missing. One connection at a time: the game writes alone, and a
// ":memory:" database lives only while its one connection does.
func openStore(path string) (*store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("making the save folder: %w", err)
		}
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(1)
	st := &store{sql: sqlDB}
	if err := st.configure(); err != nil {
		st.close()
		return nil, err
	}
	if err := st.migrate(); err != nil {
		st.close()
		return nil, err
	}
	return st, nil
}

// configure sets the pragmas this game needs. The database has one
// connection, so they hold for every statement after it.
func (st *store) configure() error {
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = 1",
	} {
		rows, err := st.sql.Query(pragma)
		if err != nil {
			return fmt.Errorf("setting up the database: %w", err)
		}
		rows.Close()
	}
	return nil
}

// migrate creates the tables and renames the legacy save without replacing
// an existing numbered slot, including when an older game build was run.
func (st *store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS players (
	id         TEXT PRIMARY KEY, -- the player identity of identity.go; a server keys its players the same way
	source     TEXT NOT NULL,    -- 'machine' or 'random'
	created_at TEXT NOT NULL,    -- RFC 3339, UTC
	token      TEXT NOT NULL DEFAULT '' -- empty until a server hands one out
);
CREATE TABLE IF NOT EXISTS saves (
	player     TEXT NOT NULL REFERENCES players(id),
	slot       TEXT NOT NULL,
	state      TEXT NOT NULL, -- the whole State, as JSON
	ticks      INTEGER NOT NULL,
	updated_at TEXT NOT NULL, -- RFC 3339, UTC
	PRIMARY KEY (player, slot)
);
CREATE TABLE IF NOT EXISTS machine (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`
	if _, err := st.sql.Exec(schema); err != nil {
		return fmt.Errorf("making the tables: %w", err)
	}
	_, err := st.sql.Exec(`UPDATE saves SET slot = '1'
		WHERE slot = 'region' AND NOT EXISTS (
			SELECT 1 FROM saves AS other
			WHERE other.player = saves.player AND other.slot = '1'
		)`)
	if err != nil {
		return fmt.Errorf("migrating the first save slot: %w", err)
	}
	return nil
}

// player returns this machine's identity, registering it the first
// time: the machine ID hashed, or a random one kept in the machine
// table for machines that won't say who they are.
func (st *store) player(machine string) (string, error) {
	id := playerIDOf(machine)
	source := "machine"
	if machine == "" {
		stored, err := st.machineValue("fallback player")
		if err != nil {
			return "", err
		}
		if stored != "" {
			return stored, nil
		}
		if id, err = randomID(); err != nil {
			return "", err
		}
		if err := st.setMachineValue("fallback player", id); err != nil {
			return "", err
		}
		source = "random"
	}
	_, err := st.sql.Exec(
		`INSERT INTO players (id, source, created_at) VALUES (?, ?, ?)
		 ON CONFLICT (id) DO NOTHING`,
		id, source, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("registering the player: %w", err)
	}
	return id, nil
}

func (st *store) saveState(id, slot string, s *State) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("saving: state can't be written as JSON: %w", err)
	}
	_, err = st.sql.Exec(
		`INSERT INTO saves (player, slot, state, ticks, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (player, slot) DO UPDATE SET
		   state = excluded.state, ticks = excluded.ticks, updated_at = excluded.updated_at`,
		id, slot, string(data), s.Ticks,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("saving: %w", err)
	}
	return nil
}

func (st *store) loadState(id, slot string) (*State, bool, error) {
	var data string
	err := st.sql.QueryRow(
		`SELECT state FROM saves WHERE player = ? AND slot = ?`, id, slot,
	).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("loading: %w", err)
	}
	var s State
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, false, fmt.Errorf("loading: the save can't be read: %w", err)
	}
	return &s, true, nil
}

func (st *store) listSaves(id string) ([]saveInfo, error) {
	rows, err := st.sql.Query(
		`SELECT slot, ticks, updated_at FROM saves WHERE player = ?
		 ORDER BY CAST(slot AS INTEGER), slot`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("listing saves: %w", err)
	}
	defer rows.Close()
	var saves []saveInfo
	for rows.Next() {
		var saved saveInfo
		var stamp string
		if err := rows.Scan(&saved.Slot, &saved.Ticks, &stamp); err != nil {
			return nil, fmt.Errorf("reading save details: %w", err)
		}
		saved.UpdatedAt, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, fmt.Errorf("reading save date: %w", err)
		}
		saves = append(saves, saved)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing saves: %w", err)
	}
	return saves, nil
}

func (st *store) nextSlot(id string) (string, error) {
	var last int64
	err := st.sql.QueryRow(
		`SELECT COALESCE(MAX(CAST(slot AS INTEGER)), 0)
		 FROM saves WHERE player = ?`, id,
	).Scan(&last)
	if err != nil {
		return "", fmt.Errorf("choosing a new save slot: %w", err)
	}
	return strconv.FormatInt(last+1, 10), nil
}

func (st *store) close() error {
	return st.sql.Close()
}

// machineValue reads one of the machine table's values, "" when the key
// isn't there.
func (st *store) machineValue(key string) (string, error) {
	var value string
	err := st.sql.QueryRow(`SELECT value FROM machine WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %q from the database: %w", key, err)
	}
	return value, nil
}

func (st *store) setMachineValue(key, value string) error {
	_, err := st.sql.Exec(
		`INSERT INTO machine (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("keeping %q in the database: %w", key, err)
	}
	return nil
}

// randomID makes an identity from nowhere, for machines that have no ID
// of their own. As long as a hashed machine ID, so everything that
// keys or shows identities treats both the same.
func randomID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("making a player identity: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// saveBase writes the colony to the local database, when saving is on.
func saveBase(slot string, s *State) error {
	if db == nil {
		return errors.New("saving is off")
	}
	return db.saveState(player, slot, s)
}

// resumeState returns the state the player comes back to: the one a
// golib shot --save seeded, when there is one — so shots can start deep
// in a game —, else the last one saved for this player. The second
// result says whether any was found.
func resumeState() (*State, bool, error) {
	var seeded State
	if found, err := golib.LoadData("state", &seeded); err != nil || found {
		return &seeded, found, err
	}
	if db == nil {
		return nil, false, nil
	}
	saves, err := db.listSaves(player)
	if err != nil || len(saves) == 0 {
		return nil, false, err
	}
	return db.loadState(player, latestSave(saves).Slot)
}
