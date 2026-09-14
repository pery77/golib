// Hello is the first GoLib game: a window with a greeting. It is the game that
// tests the framework, and grows as framework features land.
package main

import (
	"fmt"
	"log"

	"golib"
)

// hello holds the game state.
type hello struct {
	elapsed float32 // seconds since the game started
}

func (h *hello) Update(dt float32) {
	h.elapsed += dt
}

func (h *hello) Draw(screen *golib.Screen) {
	screen.Clear(golib.RayWhite)
	screen.DrawText("Hello from GoLib!", 40, 40, 40, golib.DarkGray)
	screen.DrawText(fmt.Sprintf("Running for %.1f seconds", h.elapsed), 40, 100, 20, golib.Gray)
	screen.DrawText("Close the window or press Esc to quit", 40, screen.Height()-40, 20, golib.Gray)
}

func main() {
	if err := golib.Run(&hello{}, golib.Config{Title: "Hello"}); err != nil {
		log.Fatal(err)
	}
}
