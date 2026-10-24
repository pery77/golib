package fbx

import "math"

// mat is a 4 by 4 matrix in column-major order, in float64: FBX keeps its
// transforms in doubles, and a rig's chain of them loses less this way.
type mat [16]float64

func identity() mat {
	return mat{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// matOf returns the 16 numbers of a matrix in the file, or the identity.
func matOf(v []float64) mat {
	if len(v) < 16 {
		return identity()
	}
	var m mat
	copy(m[:], v)
	return m
}

// mul returns a*b: b applied first, then a.
func (a mat) mul(b mat) mat {
	var r mat
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			var sum float64
			for k := 0; k < 4; k++ {
				sum += a[k*4+row] * b[col*4+k]
			}
			r[col*4+row] = sum
		}
	}
	return r
}

func translation(t [3]float64) mat {
	m := identity()
	m[12], m[13], m[14] = t[0], t[1], t[2]
	return m
}

func scaling(s [3]float64) mat {
	m := identity()
	m[0], m[5], m[10] = s[0], s[1], s[2]
	return m
}

// euler returns the turn of three angles in degrees, around X, Y and Z, in
// the order FBX names: 0 XYZ (X first), 1 XZY, 2 YZX, 3 YXZ, 4 ZXY, 5 ZYX.
func euler(r [3]float64, order int) mat {
	if r == [3]float64{} {
		return identity()
	}
	toRadians := math.Pi / 180
	sx, cx := math.Sincos(r[0] * toRadians)
	sy, cy := math.Sincos(r[1] * toRadians)
	sz, cz := math.Sincos(r[2] * toRadians)
	x := mat{1, 0, 0, 0, 0, cx, sx, 0, 0, -sx, cx, 0, 0, 0, 0, 1}
	y := mat{cy, 0, -sy, 0, 0, 1, 0, 0, sy, 0, cy, 0, 0, 0, 0, 1}
	z := mat{cz, sz, 0, 0, -sz, cz, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	switch order {
	case 1:
		return y.mul(z).mul(x)
	case 2:
		return x.mul(z).mul(y)
	case 3:
		return z.mul(x).mul(y)
	case 4:
		return y.mul(x).mul(z)
	case 5:
		return x.mul(y).mul(z)
	}
	return z.mul(y).mul(x)
}

// point returns p moved by the whole matrix.
func (m mat) point(p [3]float64) [3]float64 {
	return [3]float64{
		m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12],
		m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13],
		m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14],
	}
}

// direction returns d turned and scaled by the matrix, without moving it.
func (m mat) direction(d [3]float64) [3]float64 {
	return [3]float64{
		m[0]*d[0] + m[4]*d[1] + m[8]*d[2],
		m[1]*d[0] + m[5]*d[1] + m[9]*d[2],
		m[2]*d[0] + m[6]*d[1] + m[10]*d[2],
	}
}

// normalMatrix returns what turns normals as m turns surfaces: the inverse
// of its upper 3 by 3, transposed, without moving.
func (m mat) normalMatrix() mat {
	inv := m.inverse()
	return mat{
		inv[0], inv[4], inv[8], 0,
		inv[1], inv[5], inv[9], 0,
		inv[2], inv[6], inv[10], 0,
		0, 0, 0, 1,
	}
}

// inverse returns m's inverse, or the identity when it has none.
func (m mat) inverse() mat {
	var inv mat
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
		return identity()
	}
	for i := range inv {
		inv[i] /= det
	}
	return inv
}

func (m mat) float32s() [16]float32 {
	var out [16]float32
	for i, v := range m {
		out[i] = float32(v)
	}
	return out
}

// decompose splits m into a translation, a turn as a quaternion x, y, z,
// w, and a scale, mirrored along X when m mirrors.
func (m mat) decompose() (t [3]float32, q [4]float32, s [3]float32) {
	t = [3]float32{float32(m[12]), float32(m[13]), float32(m[14])}
	length := func(c int) float64 { return math.Sqrt(m[c*4]*m[c*4] + m[c*4+1]*m[c*4+1] + m[c*4+2]*m[c*4+2]) }
	sx, sy, sz := length(0), length(1), length(2)
	det := m[0]*(m[5]*m[10]-m[9]*m[6]) - m[4]*(m[1]*m[10]-m[9]*m[2]) + m[8]*(m[1]*m[6]-m[5]*m[2])
	if det < 0 {
		sx = -sx
	}
	s = [3]float32{float32(sx), float32(sy), float32(sz)}
	div := func(v, by float64) float64 {
		if by == 0 {
			return 0
		}
		return v / by
	}
	// The turn alone, its columns made length 1.
	r := [9]float64{
		div(m[0], sx), div(m[1], sx), div(m[2], sx),
		div(m[4], sy), div(m[5], sy), div(m[6], sy),
		div(m[8], sz), div(m[9], sz), div(m[10], sz),
	}
	// r[col*3+row]: m00 = r[0], m11 = r[4], m22 = r[8], m21 = r[5], m12 = r[7]...
	m00, m11, m22 := r[0], r[4], r[8]
	m21, m12 := r[5], r[7]
	m02, m20 := r[6], r[2]
	m10, m01 := r[1], r[3]
	var x, y, z, w float64
	switch trace := m00 + m11 + m22; {
	case trace > 0:
		k := 0.5 / math.Sqrt(trace+1)
		w, x, y, z = 0.25/k, (m21-m12)*k, (m02-m20)*k, (m10-m01)*k
	case m00 > m11 && m00 > m22:
		k := 2 * math.Sqrt(1+m00-m11-m22)
		w, x, y, z = (m21-m12)/k, 0.25*k, (m01+m10)/k, (m02+m20)/k
	case m11 > m22:
		k := 2 * math.Sqrt(1+m11-m00-m22)
		w, x, y, z = (m02-m20)/k, (m01+m10)/k, 0.25*k, (m12+m21)/k
	default:
		k := 2 * math.Sqrt(1+m22-m00-m11)
		w, x, y, z = (m10-m01)/k, (m02+m20)/k, (m12+m21)/k, 0.25*k
	}
	n := math.Sqrt(x*x + y*y + z*z + w*w)
	if n == 0 {
		return t, [4]float32{0, 0, 0, 1}, s
	}
	return t, [4]float32{float32(x / n), float32(y / n), float32(z / n), float32(w / n)}, s
}
