//go:build !js && !golib_dist

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const liveShaderEditing = true

func readLiveShader() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	exe, _ := os.Executable()
	return readShaderFrom(workDir, exe)
}

// The CLI starts in examples/lunar. The second location also supports opening a
// debug executable from build/lunar in Explorer. Dist builds never use this.
func readShaderFrom(workDir, exe string) (string, error) {
	paths := []string{filepath.Join(workDir, "shaders", "cinema.fs")}
	buildDir := filepath.Dir(exe)
	if filepath.Base(filepath.Dir(buildDir)) == "build" {
		paths = append(paths, filepath.Join(buildDir, "..", "..", "games", filepath.Base(buildDir), "shaders", "cinema.fs"))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read cinema.fs: %w", err)
		}
	}
	return "", fmt.Errorf("shaders/cinema.fs was not found: run golib run lunar from the project root")
}
