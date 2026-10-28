package main

import (
	"golib"
)

func newCinemaShader(source string) *golib.Shader {
	shader := golib.NewShader(source)
	shader.SetUniform("frameSizeRCP", 1.0/screenWidth, 1.0/screenHeight)
	return shader
}

func (s *session) reloadCinema() {

}
