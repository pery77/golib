package golib

import "golib/internal/device"

// Outlines, as cartoons are drawn: a material's Outline draws its mesh a
// second time, its back faces only, each vertex pushed out along its normal
// by the line's width in pixels, in one color: what shows round the mesh is
// the line. Only solid things get one.

// Model.Outline and Model.Toon give every material of a model a line and a
// cartoon's light.

// Outline draws a line of color, width pixels wide (2 when zero), round
// every part of the model, wherever it is drawn; the zero Color takes it
// away.
func (m *Model) Outline(color Color, width float32) {
	m.everyMaterial(func(made *material3D) { made.setOutline(color, width) })
}

// Toon lights every part of the model in bands, as Material.Toon does: 2 for
// light and shade; 0 lights it smoothly again.
func (m *Model) Toon(bands int) {
	m.everyMaterial(func(made *material3D) { made.toon = max(bands, 0) })
}

func (m *Model) everyMaterial(change func(*material3D)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return
	}
	done := map[*material3D]bool{}
	for k, part := range m.parts {
		if done[part.material] {
			continue
		}
		if part.material == plainMaterial {
			// GoLib's own white, which other things are drawn with too:
			// the model gets one of its own.
			own := *plainMaterial
			for j := range m.parts {
				if m.parts[j].material == plainMaterial {
					m.parts[j].material = &own
				}
			}
			part = m.parts[k]
		}
		done[part.material] = true
		change(part.material)
	}
}

const (
	outlineVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 1) in vec3 vertexNormal;
layout(location = 4) in mat4 instanceModel;
uniform mat4 viewProjection;
uniform vec2 viewSize;
uniform float width;

// ANIMATION

void main()
{
    vec3 position = vertexPosition;
    vec3 n = vertexNormal;
#ifdef ANIMATED
    animate(position, n);
#endif
    vec4 world = instanceModel * vec4(position, 1.0);
    vec4 clip = viewProjection * world;
    // Pushed out along the normal as the screen shows it, by the width in
    // pixels whatever the distance.
    vec4 turned = viewProjection * vec4(normalize(mat3(instanceModel) * n), 0.0);
    vec2 out2 = turned.xy;
    if (dot(out2, out2) > 1e-12) clip.xy += normalize(out2) * width * 2.0 / viewSize * clip.w;
    gl_Position = clip;
}
`
	outlineFragmentShader = `#version 330
uniform vec4 color;
out vec4 fragColor;

void main()
{
    fragColor = vec4(color.rgb, 1.0);
}
`
)

// outlineProgram draws outlines.
type outlineProgram struct {
	program                  device.Program
	viewProjection, viewSize int32
	width, color             int32
	bending                  bendingUniforms
}

// outlinePrograms returns the programs outlines are drawn with, the plain
// one and the one that bends, made the first time a pass has an outline;
// nil for one that doesn't compile.
func (r *state3D) outlinePrograms() (plain, bent *outlineProgram) {
	build := func(name, vertex string) *outlineProgram {
		program, err := compile3D(name, vertex, outlineFragmentShader)
		if err != nil {
			reportError(err)
			return &outlineProgram{}
		}
		at := func(n string) int32 { return device.UniformLocation(program, n) }
		return &outlineProgram{
			program: program, viewProjection: at("viewProjection"), viewSize: at("viewSize"),
			width: at("width"), color: at("color"), bending: locateBending(program),
		}
	}
	if r.outline == nil {
		r.outline = build("outline", outlineVertexShader)
		r.outlineBent = build("animated outline", animated3D(outlineVertexShader))
	}
	if r.outline.program.ID != 0 {
		plain = r.outline
	}
	if r.outlineBent.program.ID != 0 {
		bent = r.outlineBent
	}
	return plain, bent
}

// drawOutlines draws the lines round the solid groups whose material has
// one.
func (r *state3D) drawOutlines(groups []shadowGroup, viewProjection *[16]float32, width, height int) {
	plain, bent := r.outlinePrograms()
	device.SetCullFront(true)
	for _, g := range groups {
		m := g.material
		if m.outline[3] <= 0 {
			continue
		}
		p := plain
		animated := g.animation.stride > 0
		if animated {
			p = bent
		}
		if p == nil {
			continue
		}
		device.UseProgram(p.program)
		device.SetUniformMatrix(p.viewProjection, viewProjection)
		device.SetUniform(p.viewSize, 2, [4]float32{float32(width), float32(height)})
		device.SetUniform(p.width, 1, [4]float32{m.outlineWidth})
		device.SetUniform(p.color, 4, m.outline)
		if animated {
			p.bending.set(g.animation)
		}
		device.DrawMeshInstanced(g.gpu, g.instances)
	}
	device.SetCullFront(false)
}
