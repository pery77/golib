package golib

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func Test3DInAWindow(t *testing.T) {
	screen, capture := openTestWindow(t, 64, 64)
	takeError()
	camera := NewCamera3D(64, 64)
	camera.Position, camera.Target = Vector3{Z: -4}, Vector3{} // facing +Z, the sun behind
	box := NewBoxMesh()
	at := func(picture *image.NRGBA, x, y int) color.NRGBA { return picture.NRGBAAt(x, y) }

	red := capture(func() {
		screen.Begin3D(camera)
		screen.DrawMesh(box, Transform{}, Red)
		screen.End3D()
	})
	if c := at(red, 32, 32); c.A != 255 || c.R < 150 || c.G > 60 || c.B > 60 {
		t.Errorf("a red box facing the camera is %v in the middle, want lit red", c)
	}
	if c := at(red, 2, 2); c.A != 0 {
		t.Errorf("the corner, beside the box, is %v, want left as it was", c)
	}

	// The nearer box hides the farther one, whatever order they are drawn in.
	nearRed := capture(func() {
		screen.Begin3D(camera)
		screen.DrawMesh(box, Transform{Position: Vector3{Z: -1}}, Red)
		screen.DrawMesh(box, Transform{Scale: Vector3{X: 3, Y: 3, Z: 1}}, Blue)
		screen.End3D()
	})
	if c := at(nearRed, 32, 32); c.R < c.B {
		t.Errorf("a red box in front of a blue one is %v in the middle, want red", c)
	}

	// A see-through box in front lets the one behind show through.
	glass := capture(func() {
		screen.Begin3D(camera)
		screen.DrawMesh(box, Transform{Position: Vector3{Z: -1}}, Color{R: 0, G: 121, B: 241, A: 128})
		screen.DrawMesh(box, Transform{Scale: Vector3{X: 3, Y: 3, Z: 1}}, Red)
		screen.End3D()
	})
	if c := at(glass, 32, 32); c.R < 40 || c.B < 40 {
		t.Errorf("a see-through blue box over a red one is %v in the middle, want both mixed", c)
	}

	// 2D after End3D goes over the 3D; lines show through nothing.
	over := capture(func() {
		screen.Begin3D(camera)
		screen.DrawMesh(box, Transform{}, Red)
		screen.DrawLine3D(Vector3{X: -3, Y: 1.5}, Vector3{X: 3, Y: 1.5}, Green)
		screen.End3D()
		screen.DrawRectangle(Rectangle{X: 30, Y: 30, Width: 4, Height: 4}, White)
	})
	if c := at(over, 32, 32); c != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("a white square drawn after End3D is %v, want white over the box", c)
	}
	line := false
	for y := 0; y < 32; y++ {
		if c := at(over, 5, y); c.G > 150 && c.R < 100 {
			line = true
		}
	}
	if !line {
		t.Error("the green line above the box isn't drawn")
	}

	// The wireframe draws edges, not faces.
	wire := capture(func() {
		screen.Begin3D(camera)
		screen.DrawWireframe(box, Transform{}, Yellow)
		screen.End3D()
	})
	if c := at(wire, 32, 32); c.A != 0 {
		t.Errorf("the middle of a wireframe box is %v, want nothing", c)
	}
	if err := takeError(); err != nil {
		t.Fatal(err)
	}

	// 3D outside a pass is a mistake the game is told about.
	capture(func() { screen.DrawMesh(box, Transform{}, Red) })
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "Begin3D") {
		t.Errorf("DrawMesh outside a 3D pass reported %v, want a mistake that names Begin3D", err)
	}
	capture(func() { screen.Begin3D(NewCamera3D(32, 32)) })
	if err := takeError(); err == nil || !strings.Contains(err.Error(), "NewCamera3D") {
		t.Errorf("a camera for another screen size reported %v, want a mistake that names NewCamera3D", err)
	}
	screen.pass.open = false
}
