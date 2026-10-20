package golib

import "math"

// matrix is a 4 by 4 matrix in column-major order, as OpenGL and glTF keep
// them: m[12], m[13] and m[14] are the translation. Games don't see it;
// [Transform] and [Camera3D] make them.
type matrix [16]float32

func identityMatrix() matrix {
	return matrix{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// mul returns a*b: b applied first, then a.
func (a matrix) mul(b matrix) matrix {
	var r matrix
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			var sum float32
			for k := 0; k < 4; k++ {
				sum += a[k*4+row] * b[col*4+k]
			}
			r[col*4+row] = sum
		}
	}
	return r
}

// transformPoint returns p moved by the whole matrix, translation included,
// divided by w for a projection.
func (m matrix) transformPoint(p Vector3) Vector3 {
	x := m[0]*p.X + m[4]*p.Y + m[8]*p.Z + m[12]
	y := m[1]*p.X + m[5]*p.Y + m[9]*p.Z + m[13]
	z := m[2]*p.X + m[6]*p.Y + m[10]*p.Z + m[14]
	w := m[3]*p.X + m[7]*p.Y + m[11]*p.Z + m[15]
	if w != 0 && w != 1 {
		return Vector3{X: x / w, Y: y / w, Z: z / w}
	}
	return Vector3{X: x, Y: y, Z: z}
}

// transformDirection returns d turned and scaled by the matrix, without its
// translation.
func (m matrix) transformDirection(d Vector3) Vector3 {
	return Vector3{
		X: m[0]*d.X + m[4]*d.Y + m[8]*d.Z,
		Y: m[1]*d.X + m[5]*d.Y + m[9]*d.Z,
		Z: m[2]*d.X + m[6]*d.Y + m[10]*d.Z,
	}
}

// matrix returns the rotation as a matrix: Yaw around Y, then Pitch, then
// Roll, applied to a point in the opposite order. Pitch up turns the front,
// +Z, towards +Y, which is a negative turn around X in a right-handed world.
func (r Rotation) matrix() matrix {
	if r == (Rotation{}) {
		return identityMatrix()
	}
	toRadians := math.Pi / 180
	sy, cy := math.Sincos(float64(r.Yaw) * toRadians)
	sp, cp := math.Sincos(float64(-r.Pitch) * toRadians)
	sr, cr := math.Sincos(float64(r.Roll) * toRadians)
	yaw := matrix{float32(cy), 0, float32(-sy), 0, 0, 1, 0, 0, float32(sy), 0, float32(cy), 0, 0, 0, 0, 1}
	pitch := matrix{1, 0, 0, 0, 0, float32(cp), float32(sp), 0, 0, float32(-sp), float32(cp), 0, 0, 0, 0, 1}
	roll := matrix{float32(cr), float32(sr), 0, 0, float32(-sr), float32(cr), 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	return yaw.mul(pitch).mul(roll)
}

// perspectiveMatrix is OpenGL's projection for a camera that sees fovY
// degrees from the bottom of the screen to the top, with depth from -1 to 1.
func perspectiveMatrix(fovY, aspect, near, far float32) matrix {
	f := float32(1 / math.Tan(float64(fovY)*math.Pi/360))
	return matrix{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) / (near - far), -1,
		0, 0, 2 * far * near / (near - far), 0,
	}
}

// orthographicMatrix sees a box height units tall, as wide as aspect says,
// with no perspective.
func orthographicMatrix(height, aspect, near, far float32) matrix {
	top := height / 2
	right := top * aspect
	return matrix{
		1 / right, 0, 0, 0,
		0, 1 / top, 0, 0,
		0, 0, -2 / (far - near), 0,
		0, 0, -(far + near) / (far - near), 1,
	}
}

// lookAtMatrix is the view of an eye at eye looking at target, with Y up, or
// with Z as up when it looks straight up or down.
func lookAtMatrix(eye, target Vector3) matrix {
	f := target.Sub(eye).Normalize()
	if f == (Vector3{}) {
		f = Vector3{Z: -1}
	}
	up := Vector3{Y: 1}
	if math.Abs(float64(f.Y)) > 0.9999 {
		up = Vector3{Z: -f.Y}
	}
	s := f.Cross(up).Normalize()
	u := s.Cross(f)
	return matrix{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-s.Dot(eye), -u.Dot(eye), f.Dot(eye), 1,
	}
}

// normalMatrix returns the upper 3 by 3 of m's inverse, transposed, in a
// matrix: what turns normals the way m turns surfaces, even when m stretches
// them more one way than another.
func (m matrix) normalMatrix() matrix {
	a, b, c := m[0], m[4], m[8]
	d, e, f := m[1], m[5], m[9]
	g, h, i := m[2], m[6], m[10]
	det := a*(e*i-f*h) - b*(d*i-f*g) + c*(d*h-e*g)
	if det == 0 {
		return identityMatrix()
	}
	inv := 1 / det
	return matrix{
		(e*i - f*h) * inv, -(b*i - c*h) * inv, (b*f - c*e) * inv, 0,
		-(d*i - f*g) * inv, (a*i - c*g) * inv, -(a*f - c*d) * inv, 0,
		(d*h - e*g) * inv, -(a*h - b*g) * inv, (a*e - b*d) * inv, 0,
		0, 0, 0, 1,
	}
}

// inverse returns m's inverse, or the identity when m has none.
func (m matrix) inverse() matrix {
	var inv matrix
	inv[0] = m[5]*m[10]*m[15] - m[5]*m[11]*m[14] - m[9]*m[6]*m[15] + m[9]*m[7]*m[14] + m[13]*m[6]*m[11] - m[13]*m[7]*m[10]
	inv[4] = -m[4]*m[10]*m[15] + m[4]*m[11]*m[14] + m[8]*m[6]*m[15] - m[8]*m[7]*m[14] - m[12]*m[6]*m[11] + m[12]*m[7]*m[10]
	inv[8] = m[4]*m[9]*m[15] - m[4]*m[11]*m[13] - m[8]*m[5]*m[15] + m[8]*m[7]*m[13] + m[12]*m[5]*m[11] - m[12]*m[7]*m[9]
	inv[12] = -m[4]*m[9]*m[14] + m[4]*m[10]*m[13] + m[8]*m[5]*m[14] - m[8]*m[6]*m[13] - m[12]*m[5]*m[10] + m[12]*m[6]*m[9]
	inv[1] = -m[1]*m[10]*m[15] + m[1]*m[11]*m[14] + m[9]*m[2]*m[15] - m[9]*m[3]*m[14] - m[13]*m[2]*m[11] + m[13]*m[3]*m[10]
	inv[5] = m[0]*m[10]*m[15] - m[0]*m[11]*m[14] - m[8]*m[2]*m[15] + m[8]*m[3]*m[14] + m[12]*m[2]*m[11] - m[12]*m[3]*m[10]
	inv[9] = -m[0]*m[9]*m[15] + m[0]*m[11]*m[13] + m[8]*m[1]*m[15] - m[8]*m[3]*m[13] - m[12]*m[1]*m[11] + m[12]*m[3]*m[9]
	inv[13] = m[0]*m[9]*m[14] - m[0]*m[10]*m[13] - m[8]*m[1]*m[14] + m[8]*m[2]*m[13] + m[12]*m[1]*m[10] - m[12]*m[2]*m[9]
	inv[2] = m[1]*m[6]*m[15] - m[1]*m[7]*m[14] - m[5]*m[2]*m[15] + m[5]*m[3]*m[14] + m[13]*m[2]*m[7] - m[13]*m[3]*m[6]
	inv[6] = -m[0]*m[6]*m[15] + m[0]*m[7]*m[14] + m[4]*m[2]*m[15] - m[4]*m[3]*m[14] - m[12]*m[2]*m[7] + m[12]*m[3]*m[6]
	inv[10] = m[0]*m[5]*m[15] - m[0]*m[7]*m[13] - m[4]*m[1]*m[15] + m[4]*m[3]*m[13] + m[12]*m[1]*m[7] - m[12]*m[3]*m[5]
	inv[14] = -m[0]*m[5]*m[14] + m[0]*m[6]*m[13] + m[4]*m[1]*m[14] - m[4]*m[2]*m[13] - m[12]*m[1]*m[6] + m[12]*m[2]*m[5]
	inv[3] = -m[1]*m[6]*m[11] + m[1]*m[7]*m[10] + m[5]*m[2]*m[11] - m[5]*m[3]*m[10] - m[9]*m[2]*m[7] + m[9]*m[3]*m[6]
	inv[7] = m[0]*m[6]*m[11] - m[0]*m[7]*m[10] - m[4]*m[2]*m[11] + m[4]*m[3]*m[10] + m[8]*m[2]*m[7] - m[8]*m[3]*m[6]
	inv[11] = -m[0]*m[5]*m[11] + m[0]*m[7]*m[9] + m[4]*m[1]*m[11] - m[4]*m[3]*m[9] - m[8]*m[1]*m[7] + m[8]*m[3]*m[5]
	inv[15] = m[0]*m[5]*m[10] - m[0]*m[6]*m[9] - m[4]*m[1]*m[10] + m[4]*m[2]*m[9] + m[8]*m[1]*m[6] - m[8]*m[2]*m[5]
	det := m[0]*inv[0] + m[1]*inv[4] + m[2]*inv[8] + m[3]*inv[12]
	if det == 0 {
		return identityMatrix()
	}
	for i := range inv {
		inv[i] /= det
	}
	return inv
}
