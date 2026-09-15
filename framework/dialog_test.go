package golib

import (
	"fmt"
	"strings"
	"testing"
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
