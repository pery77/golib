package golib

import (
	"reflect"
	"strings"
	"testing"
)

func TestSetUniform(t *testing.T) {
	shader := NewShader("")
	shader.SetUniform("tint", 1, 0.5, 0)
	if got := shader.uniforms["tint"]; !reflect.DeepEqual(got, []float32{1, 0.5, 0}) {
		t.Errorf("tint = %v, want [1 0.5 0]", got)
	}

	values := []float32{2}
	shader.SetUniform("strength", values...)
	values[0] = 3
	if got := shader.uniforms["strength"][0]; got != 2 {
		t.Errorf("strength = %v, want 2: SetUniform must copy the values", got)
	}
	if shader.err != nil {
		t.Fatalf("valid uniforms set an error: %v", shader.err)
	}

	shader.SetUniform("nothing")
	if shader.err == nil || !strings.Contains(shader.err.Error(), "pass 1 for a float") {
		t.Errorf("SetUniform with no values: error = %v, want one that says how many values to pass", shader.err)
	}
	if err := shader.load(); err != shader.err {
		t.Errorf("load() = %v, want the SetUniform mistake, before compiling anything", err)
	}
}

func TestSetPostProcess(t *testing.T) {
	t.Cleanup(func() { SetPostProcess() })

	first := NewShader("")
	SetPostProcess(first)
	if shaders, err := currentPostProcess(); err != nil || len(shaders) != 1 || shaders[0] != first {
		t.Errorf("after SetPostProcess(first): shaders = %v, error = %v", shaders, err)
	}

	SetPostProcess(first, nil)
	if _, err := currentPostProcess(); err == nil || !strings.Contains(err.Error(), "shader 2 of 2 is nil") {
		t.Errorf("a nil shader: error = %v, want one naming shader 2 of 2", err)
	}

	SetPostProcess()
	if shaders, err := currentPostProcess(); err != nil || len(shaders) != 0 {
		t.Errorf("after SetPostProcess(): shaders = %v, error = %v, want none and no error", shaders, err)
	}
}
