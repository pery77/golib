package main

import "golib"

// The screen effects, run over the whole picture after every Draw, in this
// order: the bloom lights the picture up, the lens spreads the colors of
// everything it lit, and the glitch tears the finished picture apart for a
// moment when the ship is hit. F2 or the Y button turns all three off.
//
// The shaders are files in the game's assets folder, not code, so tuning them
// needs no building: a debug build reads them from disk every time it starts
// (golib.ReadAsset), and F5 reads them again without leaving the game. Each
// file holds the numbers that decide how it looks; the game only tells them
// how it is going, through the uniforms below. A golib dist build reads the
// copies embedded in the executable, which assets.go puts there.
const (
	bloomFile  = "shaders/bloom.fs"
	chromaFile = "shaders/chroma.fs"
	glitchFile = "shaders/glitch.fs"
)

// effects owns the post-processing shaders and the switch that turns them off.
// Every scene shares one, through the session.
type effects struct {
	on                    bool
	bloom, chroma, glitch *golib.Shader
	err                   string // why the last read failed, for the screen; empty when it worked
}

// newEffects reads the shaders and turns them on.
func newEffects() *effects {
	e := &effects{on: true}
	e.reload()
	return e
}

// reload reads the three shaders from the game's assets folder and puts them
// on the screen. A file that can't be read leaves the shaders that are already
// running alone and shows why; F5 tries again. The shaders themselves compile
// inside golib.Run, which stops the game and prints raylib's warnings if one
// of them has a mistake.
func (e *effects) reload() {
	shaders, err := readShaders(bloomFile, chromaFile, glitchFile)
	if err != nil {
		e.err = err.Error()
		return
	}
	e.err = ""
	e.bloom, e.chroma, e.glitch = shaders[0], shaders[1], shaders[2]
	e.setOn(e.on)
}

// readShaders makes a shader from each of those files in the game's assets
// folder, or says which one it couldn't read.
func readShaders(names ...string) ([]*golib.Shader, error) {
	shaders := make([]*golib.Shader, 0, len(names))
	for _, name := range names {
		source, err := golib.ReadAsset(name)
		if err != nil {
			return nil, err
		}
		shaders = append(shaders, golib.NewShader(string(source)))
	}
	return shaders, nil
}

// ready reports whether the shaders were read. They arrive together.
func (e *effects) ready() bool {
	return e.bloom != nil
}

// set tells the shaders how the game is going: glitch is how badly the picture
// breaks up after a hit, from 0 to 1, and strain how close the ship is to being
// destroyed, which pulls the lens's colors further apart. Call it from Update,
// because Draw never changes anything.
func (e *effects) set(glitch, strain float32) {
	if !e.ready() {
		return
	}
	e.glitch.SetUniform("amount", glitch)
	e.chroma.SetUniform("strain", strain)
}

// setOn turns the three effects on or off together.
func (e *effects) setOn(on bool) {
	e.on = on
	if on && e.ready() {
		golib.SetPostProcess(e.bloom, e.chroma, e.glitch)
		return
	}
	golib.SetPostProcess()
}
