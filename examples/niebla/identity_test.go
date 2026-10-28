package main

import (
	"errors"
	"strings"
	"testing"
)

func TestPlayerIDOfIsStableAndDistinct(t *testing.T) {
	first := playerIDOf("machine-one")
	if first != playerIDOf("machine-one") {
		t.Fatal("the same machine derived two identities")
	}
	if first == playerIDOf("machine-two") {
		t.Fatal("two machines derived the same identity")
	}
	if len(first) != 64 || strings.Trim(first, "0123456789abcdef") != "" {
		t.Fatalf("playerIDOf gave %q, which isn't 64 hex characters", first)
	}
}

func TestPlayerDisplayShortensAndCapitalizes(t *testing.T) {
	id := playerIDOf("machine-one")
	if got, want := playerDisplay(id), "#"+strings.ToUpper(id[:8]); got != want {
		t.Fatalf("playerDisplay gave %q, want %q", got, want)
	}
}

func TestMachineIDFromLinuxReadsSystemdThenDbus(t *testing.T) {
	readSystemd := func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("  abc123 \n"), nil
		}
		return nil, errors.New("no such file")
	}
	if got := machineIDFromLinux(readSystemd); got != "abc123" {
		t.Fatalf("machineIDFromLinux gave %q, want %q", got, "abc123")
	}
	readDbus := func(path string) ([]byte, error) {
		if path == "/var/lib/dbus/machine-id" {
			return []byte("dbus456\n"), nil
		}
		return nil, errors.New("no such file")
	}
	if got := machineIDFromLinux(readDbus); got != "dbus456" {
		t.Fatalf("machineIDFromLinux gave %q, want %q", got, "dbus456")
	}
	if got := machineIDFromLinux(func(string) ([]byte, error) {
		return nil, errors.New("no such file")
	}); got != "" {
		t.Fatalf("machineIDFromLinux gave %q with no files to read", got)
	}
}

func TestMachineIDFromWindowsParsesRegQuery(t *testing.T) {
	out := "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n" +
		"    MachineGuid    REG_SZ    5c3f-9a2b\r\n"
	if got := machineIDFromWindows(out); got != "5c3f-9a2b" {
		t.Fatalf("machineIDFromWindows gave %q, want %q", got, "5c3f-9a2b")
	}
	if got := machineIDFromWindows("no value here"); got != "" {
		t.Fatalf("machineIDFromWindows gave %q from nothing", got)
	}
}

func TestMachineIDFromDarwinParsesIoreg(t *testing.T) {
	out := "+-o IOPlatformExpertDevice\n" +
		"    \"IOPlatformUUID\" = \"AA1BB2-CC3DD4\"\n"
	if got := machineIDFromDarwin(out); got != "AA1BB2-CC3DD4" {
		t.Fatalf("machineIDFromDarwin gave %q, want %q", got, "AA1BB2-CC3DD4")
	}
	if got := machineIDFromDarwin("no uuid here"); got != "" {
		t.Fatalf("machineIDFromDarwin gave %q from nothing", got)
	}
}
