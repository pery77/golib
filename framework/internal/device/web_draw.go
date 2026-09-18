//go:build js

package device

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"math"
	"syscall/js"
)

// The drawing commands. Each one is its number followed by a fixed count of
// float32s, and web.js reads them back with the same table: change one side
// and change the other. Colors travel as four numbers from 0 to 255.
const (
	opClear = iota + 1
	opRectangle
	opRectangleOutline
	opCircle
	opRing
	opLine
	opTriangle
	opTexture
	opBeginTarget
	opEndTarget
	opBeginCamera
	opEndCamera
	opBlend
	opBeginShader
	opEndShader
	opShaderValues
	opBeginFrame
	opEndFrame
)

// The blend modes, for opBlend.
const (
	blendNormal = 0
	blendAdd    = 1
	blendCopy   = 2
)

// commands holds the frame being built, as the bytes that go to the page, and
// the buffer they are copied into.
var commands struct {
	bytes []byte
	array js.Value
	woken js.Func       // what web.js calls when the next frame is due
	tick  chan struct{} // it wakes the game through this
}

// push starts a command, and putf adds one of its numbers.
func push(op int, args ...float32) {
	putf(float32(op))
	for _, arg := range args {
		putf(arg)
	}
}

func putf(v float32) {
	commands.bytes = binary.LittleEndian.AppendUint32(commands.bytes, math.Float32bits(v))
}

// putColor adds a color as its four channels.
func putColor(c Color) {
	putf(float32(c.R))
	putf(float32(c.G))
	putf(float32(c.B))
	putf(float32(c.A))
}

// flush hands the commands built so far to the page, which draws them in
// order. Anything that changes what the page holds - a new texture, a freed
// one - flushes first, so the drawing that came before it still sees the old
// state.
func flush() {
	if len(commands.bytes) == 0 {
		return
	}
	if commands.array.IsUndefined() || commands.array.Length() < len(commands.bytes) {
		commands.array = js_().Call("drawBuffer", len(commands.bytes)*2)
	}
	js.CopyBytesToJS(commands.array, commands.bytes)
	js_().Call("draw", len(commands.bytes)/4)
	commands.bytes = commands.bytes[:0]
}

// BeginFrame starts drawing on the canvas, and EndFrame shows what was drawn
// and waits for the browser to ask for the next frame. Waiting here is what
// paces a web game: the browser decides when to draw, as the monitor does on
// the desktop.
func BeginFrame() {
	push(opBeginFrame)
}

// EndFrame ends the frame BeginFrame started and waits for the next one.
func EndFrame() {
	push(opEndFrame)
	flush()
	waitForFrame()
}

// waitForFrame blocks the game until the browser is ready to draw again.
// While it waits, the page answers clicks, keys and everything else: Go hands
// the thread back whenever every goroutine is waiting.
func waitForFrame() {
	if commands.tick == nil {
		commands.tick = make(chan struct{}, 1)
		commands.woken = js.FuncOf(func(js.Value, []js.Value) any {
			select {
			case commands.tick <- struct{}{}:
			default: // a frame is already waiting; one is enough
			}
			return nil
		})
		js_().Call("onFrame", commands.woken)
	}
	js_().Call("askForFrame")
	<-commands.tick
}

// BeginTarget sends the drawing that follows into target instead of the
// canvas, until EndTarget.
func BeginTarget(target Target) {
	push(opBeginTarget, float32(target.ID))
}

// EndTarget ends the drawing BeginTarget sent into a target.
func EndTarget() {
	push(opEndTarget)
}

// Clear fills everything being drawn on with color.
func Clear(c Color) {
	push(opClear)
	putColor(c)
}

// FlushDrawing sends the drawing waiting in memory to the graphics card.
func FlushDrawing() {
	flush()
}

// DrawRectangle fills rect with color.
func DrawRectangle(rect Rectangle, c Color) {
	push(opRectangle, rect.X, rect.Y, rect.Width, rect.Height)
	putColor(c)
}

// DrawRectangleOutline draws the edges of rect, thickness pixels wide, inside
// it.
func DrawRectangleOutline(rect Rectangle, thickness float32, c Color) {
	push(opRectangleOutline, rect.X, rect.Y, rect.Width, rect.Height, thickness)
	putColor(c)
}

// DrawCircle fills the circle centered at x, y with color.
func DrawCircle(x, y, radius float32, c Color) {
	push(opCircle, x, y, radius)
	putColor(c)
}

// DrawRing fills the space between two circles centered at x, y: a ring.
func DrawRing(x, y, inner, outer float32, c Color) {
	push(opRing, x, y, inner, outer)
	putColor(c)
}

// DrawLine draws a straight line from x1, y1 to x2, y2, thickness pixels wide.
func DrawLine(x1, y1, x2, y2, thickness float32, c Color) {
	push(opLine, x1, y1, x2, y2, thickness)
	putColor(c)
}

// DrawTriangle fills the triangle with corners x1, y1, x2, y2 and x3, y3,
// whatever order the corners come in.
func DrawTriangle(x1, y1, x2, y2, x3, y3 float32, c Color) {
	push(opTriangle, x1, y1, x2, y2, x3, y3)
	putColor(c)
}

// DrawTexture draws the source part of texture into dest, turned rotation
// degrees around origin, which is measured from dest's corner, and colored by
// tint. A negative source width or height draws the picture mirrored.
func DrawTexture(texture Texture, source, dest Rectangle, origin Vector2, rotation float32, tint Color) {
	push(opTexture, float32(texture.ID), float32(texture.Width), float32(texture.Height),
		source.X, source.Y, source.Width, source.Height,
		dest.X, dest.Y, dest.Width, dest.Height,
		origin.X, origin.Y, rotation)
	putColor(tint)
}

// NewTexture puts a picture on the graphics card: width by height pixels of
// RGBA, one byte a channel, the top row first.
func NewTexture(pixels []byte, width, height int) Texture {
	flush()
	buffer := js.Global().Get("Uint8Array").New(len(pixels))
	js.CopyBytesToJS(buffer, pixels)
	id := js_().Call("newTexture", width, height, buffer).Int()
	return Texture{ID: uint32(id), Width: int32(width), Height: int32(height)}
}

// UnloadTexture frees a texture.
func UnloadTexture(texture Texture) {
	flush()
	js_().Call("unloadTexture", int(texture.ID))
}

// NewTarget makes a texture the game can draw into, smoothed when it is
// scaled unless smooth is false, as pixel art wants.
func NewTarget(width, height int, smooth bool) Target {
	flush()
	made := js_().Call("newTarget", width, height, smooth)
	return Target{
		ID: uint32(made.Index(0).Int()),
		Texture: Texture{
			ID:     uint32(made.Index(1).Int()),
			Width:  int32(width),
			Height: int32(height),
		},
	}
}

// UnloadTarget frees a target.
func UnloadTarget(target Target) {
	flush()
	js_().Call("unloadTarget", int(target.ID))
}

// TargetTexture returns what has been drawn into target, ready to draw with.
// Like OpenGL's, it holds its rows the other way up, which is why package
// golib draws it with a negative source height.
func TargetTexture(target Target) Texture {
	return target.Texture
}

// ReadTarget returns what was drawn into target as a picture, its top row
// first.
func ReadTarget(target Target) *image.NRGBA {
	flush()
	width, height := int(target.Texture.Width), int(target.Texture.Height)
	pixels := make([]byte, width*height*4)
	buffer := js.Global().Get("Uint8Array").New(len(pixels))
	js_().Call("readTarget", int(target.ID), width, height, buffer)
	js.CopyBytesToGo(pixels, buffer)
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		// The graphics card hands back the bottom row first.
		row := (height - 1 - y) * width * 4
		for x := range width {
			at := row + x*4
			picture.SetNRGBA(x, y, color.NRGBA{R: pixels[at], G: pixels[at+1], B: pixels[at+2], A: pixels[at+3]})
		}
	}
	return picture
}

// BeginCamera moves, scales and centers the drawing that follows: the world
// point at targetX, targetY lands at offsetX, offsetY on the screen, zoom
// times as large. EndCamera goes back to screen pixels.
func BeginCamera(offsetX, offsetY, targetX, targetY, zoom float32) {
	push(opBeginCamera, offsetX, offsetY, targetX, targetY, zoom)
}

// EndCamera ends the view BeginCamera started.
func EndCamera() {
	push(opEndCamera)
}

// BeginBlendAdd adds what is drawn next to what is underneath, so overlapping
// shapes grow brighter, until EndBlend.
func BeginBlendAdd() {
	push(opBlend, blendAdd)
}

// BeginBlendCopy copies the pixels drawn next as they are, opacity included,
// instead of blending them over what is underneath, until EndBlend.
func BeginBlendCopy() {
	push(opBlend, blendCopy)
}

// EndBlend goes back to blending normally.
func EndBlend() {
	push(opBlend, blendNormal)
}

// NewShader reports that post-processing shaders are not in the web build
// yet: they are stage 3 (see docs/roadmap.md). Package golib passes the reason
// on, so a game that uses SetPostProcess says so instead of quietly drawing
// the wrong picture.
func NewShader(fragment string) (Shader, error) {
	return Shader{}, errors.New("a web build has no post-processing shaders yet: turn them off with golib.SetPostProcess(), or play the game on the desktop")
}

// UnloadShader has nothing to free yet.
func UnloadShader(shader Shader) {}

// BeginShader has no shader to run yet.
func BeginShader(shader Shader) {}

// EndShader has no shader to end yet.
func EndShader() {}

// ShaderLocation reports that no shader declares the uniform yet.
func ShaderLocation(shader Shader, name string) int32 {
	return -1
}

// SetShaderValues has no shader to set a uniform on yet.
func SetShaderValues(shader Shader, location int32, values []float32) {}
