//go:build raylib_no_embed && ffi_no_embed

package golib

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMissingLibraries starts this test program again where it can't find
// libffi or raylib: golib/internal/startup must stop it with a message before
// ffi and raylib-go panic.
func TestMissingLibraries(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^$")
	cmd.Dir = t.TempDir()
	for _, variable := range os.Environ() {
		if name, _, _ := strings.Cut(variable, "="); !strings.EqualFold(name, "PATH") {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	system := os.Getenv("SystemRoot")
	cmd.Env = append(cmd.Env, "PATH="+filepath.Join(system, "System32")+string(os.PathListSeparator)+system)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Skipf("the libraries loaded from Windows' own folders; output:\n%s", output)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("the program ended with %v, want exit code 1; output:\n%s", err, output)
	}
	for _, want := range []string{
		"cannot load library libffi-8.dll: ",
		"\ncannot load library raylib.dll: ",
		"\nThe game needs libffi-8.dll and raylib.dll next to its executable or in a folder in PATH: golib build, run, shot and test put them there, and golib dist puts them in the game's folder.\n",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("output doesn't contain %q:\n%s", want, output)
		}
	}
	if strings.Contains(string(output), "panic") {
		t.Errorf("ffi or raylib-go panicked first:\n%s", output)
	}
}
