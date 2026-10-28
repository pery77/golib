//go:build js || golib_dist

package main

import "fmt"

const liveShaderEditing = false

func readLiveShader() (string, error) {
	return "", fmt.Errorf("this build uses the embedded shader; rebuild to change it")
}
