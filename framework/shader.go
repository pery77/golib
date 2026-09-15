package golib

import (
	"errors"
	"fmt"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// shaderVertexSource is the vertex shader every post-processing shader runs
// with: raylib's default one, which hands the texture coordinates and colors
// to the fragment shader.
const shaderVertexSource = `#version 330
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

// Shader is a post-processing effect, such as scanlines, a glow or a color
// grade: a GLSL fragment shader that runs over the whole picture after Draw.
// Create one with NewShader and turn it on with SetPostProcess.
type Shader struct {
	source    string
	shader    rl.Shader
	loaded    bool
	locations map[string]int32     // uniform locations, looked up once
	uniforms  map[string][]float32 // set with SetUniform, sent every frame
	err       error                // a SetUniform mistake, or a failed compile
}

// NewShader returns a post-processing shader made from the source code of a
// GLSL 330 fragment shader. The shader reads the picture from texture0 and
// writes finalColor. This one turns the game gray:
//
//	#version 330
//
//	in vec2 fragTexCoord;
//	in vec4 fragColor;
//	uniform sampler2D texture0;
//	out vec4 finalColor;
//
//	void main()
//	{
//	    vec4 color = texture(texture0, fragTexCoord);
//	    float gray = dot(color.rgb, vec3(0.299, 0.587, 0.114));
//	    finalColor = vec4(vec3(gray), color.a);
//	}
//
// fragTexCoord goes from 0, 0 at the bottom-left corner of the picture to 1, 1
// at the top-right corner, as in OpenGL: y grows upwards, unlike on Screen.
//
// Run also sets these uniforms, when the shader declares them:
//
//	uniform float time;      // seconds of game time: 60 updates are one second
//	uniform vec2 screenSize; // the screen games draw on, in pixels: Config.Width, Config.Height
//	uniform vec2 outputSize; // the picture this shader draws, in pixels
//
// For the last shader, outputSize is the part of the window the game fills,
// which is larger than screenSize in a large window or in fullscreen. Use it
// for effects that follow real pixels, like scanlines.
//
// Keep shader sources in the game's folder, such as shaders/crt.fs, and embed
// them with a //go:embed line above a string variable.
//
// NewShader doesn't compile the source. Run does, when the shader is first
// used, and returns an error if it doesn't compile; raylib's warnings above
// the error give the line and the reason.
func NewShader(source string) *Shader {
	return &Shader{source: source, locations: map[string]int32{}, uniforms: map[string][]float32{}}
}

// SetUniform sets a uniform the shader declares: pass one value for a float,
// two for a vec2, three for a vec3 or four for a vec4. The value stays until
// the next SetUniform with the same name. Names the shader doesn't declare
// are ignored, because GLSL compilers drop uniforms a shader doesn't use.
//
//	crt.SetUniform("curvature", 0.5)
//	glow.SetUniform("tint", 0.4, 0.9, 1.0)
func (s *Shader) SetUniform(name string, values ...float32) {
	if len(values) < 1 || len(values) > 4 {
		s.err = fmt.Errorf("golib: SetUniform(%q) got %d values: pass 1 for a float, 2 for a vec2, 3 for a vec3 or 4 for a vec4", name, len(values))
		return
	}
	s.uniforms[name] = append([]float32(nil), values...)
}

// load compiles the shader the first time it is used. The window must be open.
func (s *Shader) load() error {
	if s.err != nil {
		return s.err
	}
	if s.loaded {
		return nil
	}
	shader := rl.LoadShaderFromMemory(shaderVertexSource, s.source)
	// raylib falls back to its default shader when the source doesn't compile.
	if shader.ID == 0 || shader.ID == rl.GetShaderIdDefault() {
		s.err = errors.New("golib: a post-processing shader doesn't compile: the raylib warnings above say where and why")
		return s.err
	}
	s.shader = shader
	s.loaded = true
	return nil
}

// unload frees the compiled shader, so it compiles again if used again.
func (s *Shader) unload() {
	if s.loaded {
		rl.UnloadShader(s.shader)
		s.loaded = false
		s.locations = map[string]int32{}
	}
}

// apply sends the uniforms Run sets and those set with SetUniform. Call it
// between rl.BeginShaderMode and rl.EndShaderMode.
func (s *Shader) apply(time, screenWidth, screenHeight, outputWidth, outputHeight float32) {
	s.send("time", []float32{time})
	s.send("screenSize", []float32{screenWidth, screenHeight})
	s.send("outputSize", []float32{outputWidth, outputHeight})
	for name, values := range s.uniforms {
		s.send(name, values)
	}
}

func (s *Shader) send(name string, values []float32) {
	location, found := s.locations[name]
	if !found {
		location = rl.GetShaderLocation(s.shader, name)
		s.locations[name] = location
	}
	if location < 0 {
		return
	}
	// ShaderUniformFloat, Vec2, Vec3 and Vec4 follow each other.
	rl.SetShaderValue(s.shader, location, values, rl.ShaderUniformFloat+rl.ShaderUniformDataType(len(values)-1))
}

// postProcess holds the shaders SetPostProcess asks for.
var postProcess struct {
	sync.Mutex
	shaders []*Shader
	err     error
}

// SetPostProcess runs shaders, in order, over the whole picture after every
// Draw: each shader works on the result of the one before, and the last one
// draws to the window. Call it with no shaders to turn post-processing off.
// Call it before Run to start with the effects, or from Update to change
// them, for example from an options menu:
//
//	glow := golib.NewShader(glowSource)
//	crt := golib.NewShader(crtSource)
//	golib.SetPostProcess(glow, crt)
//
// Screenshots from golib shot show the picture after post-processing.
func SetPostProcess(shaders ...*Shader) {
	postProcess.Lock()
	defer postProcess.Unlock()
	postProcess.shaders = append([]*Shader(nil), shaders...)
	postProcess.err = nil
	for i, shader := range shaders {
		if shader == nil {
			postProcess.err = fmt.Errorf("golib.SetPostProcess: shader %d of %d is nil: create shaders with golib.NewShader", i+1, len(shaders))
			return
		}
	}
}

// currentPostProcess returns the shaders SetPostProcess asked for, or the
// mistake in that call.
func currentPostProcess() ([]*Shader, error) {
	postProcess.Lock()
	defer postProcess.Unlock()
	return postProcess.shaders, postProcess.err
}
