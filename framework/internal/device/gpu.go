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
	// LayoutLit is a position (location 0), a normal (location 1), texture
	// coordinates (location 2) and an RGBA color from 0 to 1, linear
	// (location 3): 12 floats a vertex. Meshes in it are drawn instanced:
	// each instance's model matrix is at locations 4 to 7 and its tint at 8.
	LayoutLit = iota

	// LayoutColored is a position (location 0) and an RGBA color from 0 to
	// 1 (location 3): 7 floats a vertex.
	LayoutColored

	// LayoutSkinned is LayoutLit followed by four joints (location 9), as
	// whole numbers, and their four weights (location 10): 20 floats a
	// vertex, drawn instanced as LayoutLit is.
	LayoutSkinned
)

// LayoutFloats is how many floats a vertex of each layout takes.
var LayoutFloats = [...]int{LayoutLit: 12, LayoutColored: 7, LayoutSkinned: 20}

// InstanceFloats is how many floats an instance takes in DrawMeshInstanced:
// its model matrix, column-major, then its tint, RGBA.
const InstanceFloats = 16 + 4

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

// Texture3D is a picture on the graphics card for 3D, with mipmaps. Portable
// fields: ID, 0 for none, Width and Height.
type Texture3D struct {
	ID            uint32
	Width, Height int
}

// ShadowMap is a depth picture of the world seen from a light, which lit
// surfaces compare their own depth with to know whether they are in shadow.
// Sampled with comparison, smoothed between texels. Portable fields: ID, 0
// for none, Width and Height.
type ShadowMap struct {
	ID            uint32
	texture       uint32 // the raylib backend's depth texture
	Width, Height int
}

// WorkTarget is a picture a pass draws into and reads back from on the way
// to the screen, such as the steps of a glow: colors in half floats where
// the browser allows, smoothed when sampled, clamped at its edges. Portable
// fields: ID, 0 for none, Width and Height.
type WorkTarget struct {
	ID            uint32
	texture       uint32 // the raylib backend's color texture
	Width, Height int
}

// DataTexture is numbers for shaders rather than a picture: RGBA texels of
// 32-bit floats, DataTextureWidth to a row, never smoothed, which a shader
// reads with texelFetch, such as the bones of every animated model in a pass.
// Portable fields: ID, 0 for none.
type DataTexture struct {
	ID      uint32
	texture uint32 // the raylib backend's texture
	rows    int    // the raylib backend's rows, as made
}

// DataTextureWidth is how many texels a row of a DataTexture holds: texel k
// is at k % DataTextureWidth across and k / DataTextureWidth down.
const DataTextureWidth = 1024

// dataRows pads values, four for each texel, to whole rows of a
// DataTexture, and says how many rows they fill, at least one.
func dataRows(values []float32) ([]float32, int) {
	row := DataTextureWidth * 4
	rows := max(1, (len(values)+row-1)/row)
	if len(values) < rows*row {
		// A copy: the caller's values may share their array with more.
		values = append(values[:len(values):len(values)], make([]float32, rows*row-len(values))...)
	}
	return values, rows
}

// TextureOptions say how a Texture3D is stored and sampled. The zero value
// is a color picture, smoothed, repeated both ways.
type TextureOptions struct {
	Linear         bool // the numbers are data, such as normals, not sRGB colors
	Nearest        bool // magnified without smoothing, as pixel art wants
	NearestMipmaps bool // minified without blending between sizes
	ClampS, ClampT bool // the edge pixel goes on past the edge, across and down
	MirrorS        bool // repeated mirrored across
	MirrorT        bool
}
