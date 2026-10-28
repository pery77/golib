package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// The player's identity. The machine says who is playing: its system ID,
// hashed with the game's salt, gives a number every run of the game
// agrees on, no other machine shares, and that can show on screen and
// travel to a server without ever naming the raw ID. A machine without
// an ID of its own gets a random one, kept in the local database (see
// store.go), so it is still stable from then on.

// player is the identity of the machine that is playing: 64 hex
// characters, stable across runs. main fills it before the first scene;
// the menu shows its first eight, the database keys the saves with all
// of it, and a later server hands its tokens out by it.
var player string

// playerIDSalt makes this game's identities its own: the same machine ID
// under another salt, or another game, hashes to something else. Change
// it and every player becomes new.
const playerIDSalt = "niebla player id v1\x00"

// playerIDOf derives the player identity from a machine ID. An empty
// machine ID gives an identity too, but a resolvePlayer that finds
// nothing better replaces it with a random one kept in the database.
func playerIDOf(machine string) string {
	sum := sha256.Sum256([]byte(playerIDSalt + machine))
	return hex.EncodeToString(sum[:])
}

// playerDisplay shortens an identity to the number the menu shows: eight
// hex characters, enough to tell machines apart by eye. The whole
// identity stays the one that counts.
func playerDisplay(id string) string {
	return "#" + strings.ToUpper(id[:8])
}

// resolvePlayer works out who is playing: it opens the local database,
// asks it for this machine's identity — which registers the player the
// first time, the moment a server would hand a token out — and fills
// player and db. When either side fails, saving is off, the menu says
// so through saveWarning, and the identity still comes from the machine.
func resolvePlayer() {
	machine := machineID(os.ReadFile, runCommand)
	player = playerIDOf(machine)
	path, err := storePath(os.Getenv, os.UserConfigDir)
	if err == nil {
		db, err = openStore(path)
	}
	if err != nil {
		saveWarning = "saving is off: " + err.Error()
		return
	}
	player, err = db.player(machine)
	if err != nil {
		db.close()
		db = nil
		player = playerIDOf(machine)
		saveWarning = "saving is off: " + err.Error()
	}
}

// machineID returns the identifier this machine's system keeps, or ""
// when it can't find one. read and run stand for os.ReadFile and
// runCommand, so the tests can feed their outputs.
func machineID(read func(string) ([]byte, error), run func(string, ...string) (string, error)) string {
	switch runtime.GOOS {
	case "windows":
		out, err := run("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid")
		if err != nil {
			return ""
		}
		return machineIDFromWindows(out)
	case "darwin":
		out, err := run("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		if err != nil {
			return ""
		}
		return machineIDFromDarwin(out)
	default:
		return machineIDFromLinux(read)
	}
}

// machineIDFromLinux reads systemd's machine ID, falling back to dbus's,
// trimming what the files carry around it.
func machineIDFromLinux(read func(string) ([]byte, error)) string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := read(path)
		if err != nil {
			continue
		}
		if id := strings.ToLower(strings.TrimSpace(string(data))); id != "" {
			return id
		}
	}
	return ""
}

// machineIDFromWindows reads MachineGuid out of what reg query printed:
// its line holds the name, the type and the GUID, and the GUID is the
// last word of it.
func machineIDFromWindows(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "MachineGuid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			return fields[len(fields)-1]
		}
	}
	return ""
}

// machineIDFromDarwin reads the platform UUID out of what ioreg printed.
func machineIDFromDarwin(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if _, value, ok := strings.Cut(line, `"IOPlatformUUID" = `); ok {
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return ""
}

// runCommand runs one of the system's own commands and returns its
// standard output, the way machineID reads a Windows registry value or a
// Mac's platform UUID.
func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}
