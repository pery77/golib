//go:build !js

package device

import (
	"errors"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// vertexShaderSource is the vertex shader every post-processing shader runs
// with: raylib's default one, which hands the texture coordinates and colors
// to the fragment shader. A backend on another graphics API writes its own.
const vertexShaderSource = `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec4 vertexColor;
out vec2 fragTexCoord;
out vec4 fragColor;
uniform mat4 mvp;

void main()
{
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}
`

// BeginFrame starts drawing on the window, and EndFrame shows what was drawn.
func BeginFrame() {
	rl.BeginDrawing()
}

// EndFrame ends the frame BeginFrame started.
func EndFrame() {
	rl.EndDrawing()
}

// BeginTarget sends the drawing that follows into target instead of the
// window, until EndTarget.
func BeginTarget(target Target) {
	rl.BeginTextureMode(target)
}

// EndTarget ends the drawing BeginTarget sent into a target.
func EndTarget() {
	rl.EndTextureMode()
}

// Clear fills everything being drawn on with color.
func Clear(color Color) {
	rl.ClearBackground(color)
}

// FlushDrawing sends the drawing waiting in memory to the graphics card. A
// texture or font about to be freed has to go through this first.
func FlushDrawing() {
	rl.DrawRenderBatchActive()
}

// DrawRectangle fills rect with color.
func DrawRectangle(rect Rectangle, color Color) {
	rl.DrawRectangleRec(rect, color)
}

// DrawRectangleOutline draws the edges of rect, thickness pixels wide, inside
// it.
func DrawRectangleOutline(rect Rectangle, thickness float32, color Color) {
	rl.DrawRectangleLinesEx(rect, thickness, color)
}

// DrawCircle fills the circle centered at x, y with color.
func DrawCircle(x, y, radius float32, color Color) {
	rl.DrawCircleV(Vector2{X: x, Y: y}, radius, color)
}

// DrawRing fills the space between two circles centered at x, y: a ring.
func DrawRing(x, y, inner, outer float32, color Color) {
	// 0 segments lets raylib pick enough for the circle to look round.
	rl.DrawRing(Vector2{X: x, Y: y}, inner, outer, 0, 360, 0, color)
}

// DrawLine draws a straight line from x1, y1 to x2, y2, thickness pixels wide.
func DrawLine(x1, y1, x2, y2, thickness float32, color Color) {
	rl.DrawLineEx(Vector2{X: x1, Y: y1}, Vector2{X: x2, Y: y2}, thickness, color)
}

// DrawTriangle fills the triangle with corners x1, y1, x2, y2 and x3, y3,
// whatever order the corners come in.
func DrawTriangle(x1, y1, x2, y2, x3, y3 float32, color Color) {
	// raylib only fills triangles whose corners go counterclockwise on screen.
	if (x2-x1)*(y3-y1)-(y2-y1)*(x3-x1) > 0 {
		x2, y2, x3, y3 = x3, y3, x2, y2
	}
	rl.DrawTriangle(Vector2{X: x1, Y: y1}, Vector2{X: x2, Y: y2}, Vector2{X: x3, Y: y3}, color)
}

// NewTexture puts a picture on the graphics card: width by height pixels of
// RGBA, one byte a channel, the top row first.
func NewTexture(pixels []byte, width, height int) Texture {
	picture := rl.NewImage(pixels, int32(width), int32(height), 1, rl.UncompressedR8g8b8a8)
	return rl.LoadTextureFromImage(picture)
}

// UnloadTexture frees a texture.
func UnloadTexture(texture Texture) {
	rl.UnloadTexture(texture)
}

// DrawTexture draws the source part of texture into dest, turned rotation
// degrees around origin, which is measured from dest's corner, and colored by
// tint. A negative source height draws the picture upside down.
func DrawTexture(texture Texture, source, dest Rectangle, origin Vector2, rotation float32, tint Color) {
	rl.DrawTexturePro(texture, source, dest, origin, rotation, tint)
}

// NewTarget makes a texture the game can draw into, smoothed when it is
// scaled unless smooth is false, as pixel art wants.
func NewTarget(width, height int, smooth bool) Target {
	target := rl.LoadRenderTexture(int32(width), int32(height))
	filter := rl.FilterBilinear
	if !smooth {
		filter = rl.FilterPoint
	}
	rl.SetTextureFilter(target.Texture, filter)
	return target
}

// UnloadTarget frees a target.
func UnloadTarget(target Target) {
	rl.UnloadRenderTexture(target)
}

// BeginCamera moves, scales and centers the drawing that follows: the world
// point at targetX, targetY lands at offsetX, offsetY on the screen, zoom
// times as large. EndCamera goes back to screen pixels.
func BeginCamera(offsetX, offsetY, targetX, targetY, zoom float32) {
	rl.BeginMode2D(rl.Camera2D{
		Offset: Vector2{X: offsetX, Y: offsetY},
		Target: Vector2{X: targetX, Y: targetY},
		Zoom:   zoom,
	})
}

// EndCamera ends the view BeginCamera started.
func EndCamera() {
	rl.EndMode2D()
}

// OpenGL blend values, for rl.SetBlendFactors.
const (
	glZero    = 0
	glOne     = 1
	glFuncAdd = 0x8006
)

// BeginBlendAdd adds what is drawn next to what is underneath, so overlapping
// shapes grow brighter, until EndBlend.
func BeginBlendAdd() {
	rl.BeginBlendMode(rl.BlendAdditive)
}

// BeginBlendCopy copies the pixels drawn next as they are, opacity included,
// instead of blending them over what is underneath, until EndBlend.
func BeginBlendCopy() {
	rl.SetBlendFactors(glOne, glZero, glFuncAdd)
	rl.BeginBlendMode(rl.BlendCustom)
}

// EndBlend goes back to blending normally.
func EndBlend() {
	rl.EndBlendMode()
}

// NewShader compiles a post-processing shader from the source of a fragment
// shader, or says why it couldn't.
func NewShader(fragment string) (Shader, error) {
	shader := rl.LoadShaderFromMemory(vertexShaderSource, fragment)
	// raylib falls back to its default shader when the source doesn't compile.
	if shader.ID == 0 || shader.ID == rl.GetShaderIdDefault() {
		return Shader{}, errors.New("it doesn't compile: the raylib warnings above say where and why")
	}
	return shader, nil
}

// UnloadShader frees a compiled shader.
func UnloadShader(shader Shader) {
	rl.UnloadShader(shader)
}

// BeginShader runs shader over the drawing that follows, until EndShader.
func BeginShader(shader Shader) {
	rl.BeginShaderMode(shader)
}

// EndShader ends the shader BeginShader started.
func EndShader() {
	rl.EndShaderMode()
}

// ShaderLocation returns where the uniform called name sits in shader, or a
// negative number when the shader doesn't have it.
func ShaderLocation(shader Shader, name string) int32 {
	return rl.GetShaderLocation(shader, name)
}

// SetShaderValues sets a uniform of shader to one to four floats: a float, a
// vec2, a vec3 or a vec4.
func SetShaderValues(shader Shader, location int32, values []float32) {
	// ShaderUniformFloat, Vec2, Vec3 and Vec4 follow each other.
	rl.SetShaderValue(shader, location, values, rl.ShaderUniformFloat+rl.ShaderUniformDataType(len(values)-1))
}

// DefaultFont returns the backend's built-in font, which needs no file.
func DefaultFont() Font {
	return rl.GetFontDefault()
}

// NewFont draws the letters in runes from a font file held in memory, at size
// pixels high, and returns them ready to draw with, or says why it couldn't.
// format is the file's extension, such as ".ttf".
func NewFont(format string, data []byte, size int, runes []rune) (Font, error) {
	// raylib warns about every letter taller than the size, which many fonts
	// have, such as capitals with accents.
	rl.SetTraceLogLevel(rl.LogError)
	font := rl.LoadFontFromMemory(format, data, int32(size), runes)
	rl.SetTraceLogLevel(rl.LogWarning)
	// raylib gives its own font back when it can't read the file.
	if font.Texture.ID == 0 || font.Texture.ID == rl.GetFontDefault().Texture.ID {
		return Font{}, errors.New("raylib could not read it: check that the file is a TrueType or OpenType font")
	}
	return font, nil
}

// UnloadFont frees a font drawn at one size.
func UnloadFont(font Font) {
	rl.UnloadFont(font)
}

// FontID identifies the texture a font's letters are drawn on. It is 0 when
// the font didn't load.
func FontID(font Font) uint32 {
	return font.Texture.ID
}

// DrawText draws text with its top-left corner at x, y, size pixels high,
// with spacing pixels between its letters.
func DrawText(font Font, text string, x, y, size, spacing float32, color Color) {
	rl.DrawTextEx(font, text, Vector2{X: x, Y: y}, size, spacing, color)
}

// TextWidth returns how wide DrawText would draw text, in pixels.
func TextWidth(font Font, text string, size, spacing float32) float32 {
	return rl.MeasureTextEx(font, text, size, spacing).X
}

// TargetTexture returns what has been drawn into target, ready to draw with.
func TargetTexture(target Target) Texture {
	return target.Texture
}
