package device

// The 3D part of the contract: a layer at the level of a graphics card,
// shaped like WebGL 2, which OpenGL 3.3 covers. Package golib builds its 3D
// on it, in plain Go, once for both backends (see docs/3d.md).
//
// Objects - meshes and programs - are made and freed by direct calls. The
// rest is commands, given between Begin3D and End3D: the raylib backend runs
// them at once, and the web backend writes them into the buffer its 2D
// drawing goes through, so 2D and 3D stay in the order they were drawn.
//
// Nothing here asks the graphics card a question while drawing: in a
// browser, each question waits for the graphics card to catch up.

// The vertex layouts a mesh can have: how many floats each vertex takes, and
// what they mean. The attribute locations are fixed, so every program reads
// a layout the same way.
const (
	// LayoutLit is a position (location 0), a normal (location 1) and
	// texture coordinates (location 2): 8 floats a vertex.
	LayoutLit = iota

	// LayoutColored is a position (location 0) and an RGBA color from 0 to
	// 1 (location 3): 7 floats a vertex.
	LayoutColored
)

// LayoutFloats is how many floats a vertex of each layout takes.
var LayoutFloats = [...]int{LayoutLit: 8, LayoutColored: 7}

// Mesh is a mesh on the graphics card: vertices in one of the layouts, and
// indices into them, three for each triangle or, for a mesh of lines, two for
// each line. Portable field: ID, 0 for none.
type Mesh struct {
	ID       uint32
	vbo, ibo uint32 // the raylib backend's buffers
	count    int32  // indices
	lines    bool
}

// Program is a vertex shader and a fragment shader linked together, for 3D.
// Portable field: ID, 0 for none.
type Program struct {
	ID uint32
}
