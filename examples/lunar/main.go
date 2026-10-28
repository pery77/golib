// SELENE: Frontier is a lunar flight and logistics game built entirely on GoLib.
package main

import (
	_ "embed"
	"golib"
	"log"
)

const screenWidth = 1280
const screenHeight = 720

//go:embed shaders/cinema.fs
var cinemaSource string
var cinema = newCinemaShader(cinemaSource)

func main() {
	if err := loadRegions(); err != nil {
		log.Fatal(err)
	}
	app := newSession()
	app.applySettings()
	if err := golib.Run(newTitleScene(app), golib.Config{
		Title: "SELENE / Frontier", Width: screenWidth, Height: screenHeight,
		PauseUnfocused: true,
	}); err != nil {
		log.Fatal(err)
	}
}
