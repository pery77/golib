package golib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrashMessage(t *testing.T) {
	var stack strings.Builder
	for i := range 50 {
		fmt.Fprintf(&stack, "frame %d\n", i)
	}
	got := crashMessage("boom", []byte(stack.String()))

	if !strings.HasPrefix(got, "The game crashed: boom\n\n") {
		t.Errorf("crashMessage() starts with %q, want the panic value first", strings.SplitN(got, "\n", 2)[0])
	}
	if !strings.Contains(got, "frame 29\n...") || strings.Contains(got, "frame 30") {
		t.Errorf("crashMessage() = %q, want the first %d stack lines followed by ...", got, crashStackLines)
	}
}

// TestKeepCrash checks that a panic is kept in crash.txt where saved data
// goes, whole, and goes on as a panic; and that nothing is written where
// saved data has no folder, as under go test.
func TestKeepCrash(t *testing.T) {
	crash := func() (r any) {
		defer func() { r = recover() }()
		defer keepCrash()
		panic("boom")
	}
	t.Cleanup(func() { crashKept = "" })

	if r := crash(); r != "boom" || crashKept != "" {
		t.Errorf("with no save folder: recovered %v, kept in %q; want boom, and nothing kept", r, crashKept)
	}

	testSaveFolder = filepath.Join(t.TempDir(), "save")
	t.Cleanup(func() { testSaveFolder = "" })
	if r := crash(); r != "boom" {
		t.Errorf("keepCrash() swallowed the panic: recovered %v", r)
	}
	want := filepath.Join(testSaveFolder, crashFile)
	if crashKept != want {
		t.Fatalf("the crash is kept in %q, want %q", crashKept, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"The game crashed: boom", "When: ", "Version: ", "System: ", "TestKeepCrash"} {
		if !strings.Contains(string(data), part) {
			t.Errorf("%s lacks %q:\n%s", crashFile, part, data)
		}
	}
}

func TestCrashReport(t *testing.T) {
	when := time.Date(2026, 11, 14, 9, 5, 0, 0, time.UTC)
	got := crashReport("boom", []byte("frame 1\nframe 2\n"), when)
	if !strings.HasPrefix(got, "The game crashed: boom\n\nWhen: 2026-11-14 09:05:00\nVersion: ") || !strings.HasSuffix(got, "\n\nframe 1\nframe 2\n") {
		t.Errorf("crashReport() = %q", got)
	}
}
