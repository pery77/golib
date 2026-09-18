package device

import (
	"strings"
	"testing"
)

// A shader a game wrote for the desktop has to reach a browser as one OpenGL
// ES can compile: the version line changes and a precision line joins it.
// Everything else is the game's, untouched.
func TestESShader(t *testing.T) {
	const body = "in vec2 fragTexCoord;\nuniform sampler2D texture0;\nout vec4 finalColor;\n\nvoid main()\n{\n    finalColor = texture(texture0, fragTexCoord);\n}\n"
	for _, test := range []struct {
		name, source string
	}{
		{"the version GoLib documents", "#version 330\n" + body},
		{"the version with core", "#version 330 core\n" + body},
		{"blank lines before it", "\n\n#version 330\n" + body},
		{"no version at all", body},
	} {
		got := ESShader(test.source)
		if !strings.HasPrefix(got, "#version 300 es\nprecision highp float;\n") {
			t.Errorf("%s: shader starts with:\n%s\nwant the ES version and a precision line", test.name, first(got, 3))
		}
		if strings.Contains(got, "#version 330") {
			t.Errorf("%s: the desktop version line is still there:\n%s", test.name, first(got, 3))
		}
		if !strings.HasSuffix(got, body) {
			t.Errorf("%s: the game's own lines changed:\n%s", test.name, got)
		}
		if strings.Count(got, "precision highp float;") != 1 {
			t.Errorf("%s: %d precision lines, want 1", test.name, strings.Count(got, "precision highp float;"))
		}
	}
}

// A shader with nothing but a version line is not a reason to lose the lines
// a backend needs.
func TestESShaderOfNothingMuch(t *testing.T) {
	if got := ESShader("#version 330"); got != "#version 300 es\nprecision highp float;\n" {
		t.Errorf("ESShader of a lone version line = %q", got)
	}
	if got := ESShader(""); got != "#version 300 es\nprecision highp float;\n" {
		t.Errorf("ESShader of nothing = %q", got)
	}
}

// first returns the first n lines of a shader, for messages.
func first(source string, n int) string {
	lines := strings.Split(source, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
