package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{nil, "golib: no command given\n"},
		{[]string{"launch"}, "golib: unknown command \"launch\"\n"},
		{[]string{"Dist"}, "golib: unknown command \"Dist\"\n"},
	} {
		var stdout, stderr bytes.Buffer
		if code := execute(tt.args, &stdout, &stderr); code != 2 {
			t.Errorf("execute(%q) = %d, want 2", tt.args, code)
		}
		if want := tt.want + "Run \"golib help\" for usage.\n"; stderr.String() != want || stdout.Len() > 0 {
			t.Errorf("execute(%q) printed %q and %q, want %q on stderr only", tt.args, stdout.String(), stderr.String(), want)
		}
	}
}

func TestRunOutsideAProject(t *testing.T) {
	// The test binary isn't in build/golib/ of a project.
	var stdout, stderr bytes.Buffer
	if code := execute([]string{"dist"}, &stdout, &stderr); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "golib: ") || !strings.Contains(stderr.String(), "start it with golib") || stdout.Len() > 0 {
		t.Errorf("printed %q and %q, want an explanation on stderr only", stdout.String(), stderr.String())
	}
}

func TestChecksAndSummary(t *testing.T) {
	var stdout bytes.Buffer
	c := &cli{stdout: &stdout}
	c.check("ok", "one")
	c.check("info", "two")
	c.check("warn", "three")
	if code := c.summary("build"); code != 0 {
		t.Errorf("summary after a warning = %d, want 0", code)
	}
	c.check("fail", "four")
	if code := c.summary("build"); code != 1 {
		t.Errorf("summary after a failure = %d, want 1", code)
	}
	want := `[ok]   one
[info] two
[warn] three

build: 0 failed, 1 warning(s)
[fail] four

build: 1 failed, 1 warning(s)
`
	if stdout.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", stdout.String(), want)
	}
}
