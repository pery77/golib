package golib

// Rectangle is an axis-aligned rectangle in pixels. X and Y are its top-left
// corner; Y grows downwards, as on the screen.
type Rectangle struct {
	X, Y, Width, Height float32
}

// Overlaps reports whether r and other share any area. Rectangles that only
// touch along an edge don't overlap, so a character standing on a platform is
// not inside it.
func (r Rectangle) Overlaps(other Rectangle) bool {
	return r.X < other.X+other.Width && other.X < r.X+r.Width &&
		r.Y < other.Y+other.Height && other.Y < r.Y+r.Height
}

// Contains reports whether the point x, y is inside r, such as the mouse
// pointer over a button. The left and top edges are inside and the right and
// bottom edges are not, so two rectangles that touch never both contain a
// point.
func (r Rectangle) Contains(x, y float32) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
