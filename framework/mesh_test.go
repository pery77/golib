package golib

import (
	"math"
	"testing"

	"golib/internal/device"
)

func TestMeshesAreClosedAndFaceOut(t *testing.T) {
	for name, mesh := range map[string]*Mesh{
		"box":      NewBoxMesh(),
		"sphere":   NewSphereMesh(),
		"cylinder": NewCylinderMesh(),
		"plane":    NewPlaneMesh(),
		"cone":     NewConeMesh(),
		"capsule":  NewCapsuleMesh(),
	} {
		stride := device.LayoutFloats[device.LayoutLit]
		count := mesh.vertexCount()
		if len(mesh.vertices) != int(count)*stride || len(mesh.indices)%3 != 0 || len(mesh.indices) == 0 {
			t.Errorf("%s: %d floats and %d indices don't make whole vertices and triangles", name, len(mesh.vertices), len(mesh.indices))
			continue
		}
		position := func(i uint32) Vector3 {
			p := mesh.vertices[int(i)*stride:]
			return Vector3{X: p[0], Y: p[1], Z: p[2]}
		}
		normal := func(i uint32) Vector3 {
			p := mesh.vertices[int(i)*stride+3:]
			return Vector3{X: p[0], Y: p[1], Z: p[2]}
		}
		for i := uint32(0); i < count; i++ {
			p := position(i)
			if math.Abs(float64(p.X)) > 0.5001 || math.Abs(float64(p.Y)) > 0.5001 || math.Abs(float64(p.Z)) > 0.5001 {
				t.Errorf("%s: vertex %d at %v is outside the meter around 0, 0, 0", name, i, p)
				break
			}
			if l := normal(i).Length(); math.Abs(float64(l-1)) > 1e-3 {
				t.Errorf("%s: vertex %d has a normal of length %v, want 1", name, i, l)
				break
			}
		}
		for k := 0; k < len(mesh.indices); k += 3 {
			a, b, c := mesh.indices[k], mesh.indices[k+1], mesh.indices[k+2]
			if a >= count || b >= count || c >= count {
				t.Errorf("%s: triangle %d names a vertex past the %d there are", name, k/3, count)
				break
			}
			face := position(b).Sub(position(a)).Cross(position(c).Sub(position(a)))
			if face.Length() < 1e-9 {
				continue // a sliver at a sphere's pole
			}
			if face.Dot(normal(a).Add(normal(b)).Add(normal(c))) <= 0 {
				t.Errorf("%s: triangle %d turns clockwise seen from outside, want counterclockwise", name, k/3)
				break
			}
		}
	}

	if edges := len(NewBoxMesh().outlineEdges()) / 2; edges != 12 {
		t.Errorf("a box's wireframe has %d edges, want its 12", edges)
	}
}
