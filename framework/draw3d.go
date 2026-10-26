package golib

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"golib/internal/device"
)

// The 3D a Draw asks for is gathered between Screen.Begin3D and Screen.End3D
// and drawn at End3D, all at once. What the camera can't see is left out.
// What is solid is drawn first, every copy of the same mesh with the same
// material in one instanced draw; then the outlines and lines; then what is
// see-through, farthest first, so that it blends over whatever is behind it.

// material3D is how a surface looks: its colors, its textures and how it
// lets light and what is behind it through. Colors are linear.
type material3D struct {
	baseColor         [4]float32
	baseTexture       *texture3D // nil for none
	emissive          [3]float32
	emissiveTexture   *texture3D
	metallic          float32    // 0 for paint, wood or stone; 1 for bare metal
	roughness         float32    // 0 for a mirror, 1 for chalk
	metalRoughTexture *texture3D // roughness in green, metallic in blue, multiplying the two
	normalTexture     *texture3D // the surface's small bumps
	normalScale       float32
	occlusionTexture  *texture3D // where the soft light doesn't reach, in red
	occlusionStrength float32
	alpha             alphaMode
	alphaCutoff       float32
	twoSided          bool       // its back faces are drawn too
	unlit             bool       // its color shows as it is, without the sun
	tiling            [2]float32 // how many times textures repeat across and down; zero is once
	surface           string     // a game's own surface function, in GLSL, or ""
}

type alphaMode int

const (
	alphaOpaque alphaMode = iota // opacity ignored
	alphaMask                    // cut out where opacity is below alphaCutoff
	alphaBlend                   // see-through by its opacity
)

// plainRoughness is how rough the meshes GoLib makes are: painted wood or
// plastic, with a soft highlight.
const plainRoughness = 0.6

// The materials of the meshes GoLib makes: white, so DrawMesh's color is the
// color.
var (
	plainMaterial         = &material3D{baseColor: [4]float32{1, 1, 1, 1}, roughness: plainRoughness}
	plainTwoSidedMaterial = &material3D{baseColor: [4]float32{1, 1, 1, 1}, roughness: plainRoughness, twoSided: true}
)

// texture3D is a picture a material shows, put on the graphics card the
// first time something with it is drawn.
type texture3D struct {
	data    []byte // the PNG or JPEG file, until it is on the graphics card
	options device.TextureOptions
	decoded device.DecodedImage
	err     error
	gpu     device.Texture3D

	// A lightmap is a picture brighter than white, decoded already: its
	// texels in half floats, RGBA, the top row first.
	hdr                 []uint16
	hdrWidth, hdrHeight int
}

// item3D is one mesh drawn in a 3D pass.
type item3D struct {
	gpu      device.Mesh
	material *material3D
	model    matrix
	tint     Color
	center   Vector3 // the middle of its bounds, in the world
	radius   float32 // how far its bounds reach from center, in the world
	wire     *Mesh   // for DrawWireframe: the mesh whose edges are drawn
	anim     itemAnimation
	lightmap *texture3D // the light baked for a model's mesh in Blender, or nil
}

// itemAnimation is how an item's vertices are bent: by the joints of a skin
// and by shape keys, from its block of the pass's animation numbers. A zero
// one isn't bent.
type itemAnimation struct {
	at, floats   int // its block in pass3D.animation: 16 numbers a joint, then the shape keys' weights, in fours
	joints       int
	morphs       int
	vertices     int // its mesh's, to find each vertex's shape keys
	morphTexture device.DataTexture
}

// pass3D is what a Screen gathers between Begin3D and End3D.
type pass3D struct {
	open        bool
	camera      *Camera3D
	items       []item3D
	lines       []float32 // device.LayoutColored
	lineIndices []uint32
	animation   []float32 // the blocks of the animated items, see itemAnimation
	environment Environment
	lights      []Light
	probes      *lightProbes // the light probes of a level drawn in the pass, or nil
	probesPlace matrix       // where that level is drawn
	width       int          // the size of the target the pass draws over, in its texels
	height      int
	time        float32 // seconds of game time, for a game's own surfaces
}

// Begin3D starts drawing the 3D world seen through camera, over what Draw
// has drawn so far: call it after Screen.Clear, draw meshes with
// [Screen.DrawMesh] and the other 3D methods, and call [Screen.End3D] before
// drawing the score and the menus over the 3D:
//
//	screen.Clear(golib.SkyBlue)
//	screen.Begin3D(s.camera)
//	screen.DrawMesh(s.ground, golib.Transform{Scale: golib.Vector3{X: 40, Z: 40}}, golib.DarkGreen)
//	screen.DrawMesh(s.ball, golib.Transform{Position: s.ball.Position}, golib.Orange)
//	screen.End3D()
//	screen.DrawText(score, 10, 10, 20, golib.White)
//
// The 3D is drawn at End3D, so 2D drawn between Begin3D and End3D ends up
// under it. A Draw can have several 3D passes, each with its own camera; each
// starts with nothing in front of it. A sun from above and a soft light from
// the sky light every mesh.
func (s *Screen) Begin3D(camera *Camera3D) {
	if s.pass.open {
		s.End3D()
	}
	if camera == nil {
		reportError(errors.New("golib: Screen.Begin3D got a nil camera: make one with golib.NewCamera3D"))
		return
	}
	if s.fills {
		camera.width, camera.height = s.width, s.height // the screen changes with the window
	} else if camera.width != s.width || camera.height != s.height {
		reportError(fmt.Errorf("golib: Screen.Begin3D got a camera for a %g by %g screen, but the screen is %g by %g: pass the screen's size to NewCamera3D", camera.width, camera.height, s.width, s.height))
	}
	s.pass = pass3D{
		open:        true,
		camera:      camera,
		items:       s.pass.items[:0],
		lines:       s.pass.lines[:0],
		lineIndices: s.pass.lineIndices[:0],
		animation:   s.pass.animation[:0],
		lights:      s.pass.lights[:0],
		time:        s.time,
		width:       max(1, int(s.width*textScale)),
		height:      max(1, int(s.height*textScale)),
	}
}

// smoothEdges3D says whether the 3D smooths its edges as it goes over the
// 2D: not in a pixel art game, whose edges should stay sharp, nor with
// Config.Antialias, which smooths them already. Run sets it.
var smoothEdges3D = true

// in3D says whether a 3D pass is open, and reports the mistake when it isn't.
func (s *Screen) in3D(call string) bool {
	if !s.pass.open {
		reportError(fmt.Errorf("golib: Screen.%s was called outside a 3D pass: call screen.Begin3D(camera) first, and screen.End3D() after the 3D", call))
		return false
	}
	return true
}

// DrawMesh draws mesh placed by transform, in color, lit by the sun and the
// sky. A color that is partly see-through, with A below 255, lets what is
// behind the mesh show through.
func (s *Screen) DrawMesh(mesh *Mesh, transform Transform, color Color) {
	if !s.in3D("DrawMesh") {
		return
	}
	if mesh == nil {
		reportError(errors.New("golib: Screen.DrawMesh got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return
	}
	material := plainMaterial
	if mesh.twoSided {
		material = plainTwoSidedMaterial
	}
	s.addItem(mesh, material, transform.matrix(), color)
}

// addItem adds mesh, drawn with material, placed by model.
func (s *Screen) addItem(mesh *Mesh, material *material3D, model matrix, tint Color) {
	gpu := mesh.upload()
	if gpu.ID == 0 {
		return // no triangles
	}
	mesh.bounds()
	// The material's textures go to the graphics card now, before the pass
	// draws anything.
	for _, t := range []*texture3D{material.baseTexture, material.emissiveTexture, material.metalRoughTexture, material.normalTexture, material.occlusionTexture} {
		t.upload()
	}
	s.pass.items = append(s.pass.items, item3D{
		gpu: gpu, material: material, model: model, tint: tint,
		center: model.transformPoint(mesh.center), radius: mesh.radius * model.largestScale(),
	})
}

// addAnimated adds mesh, bent as anim says, drawn with material, placed by
// model, inside the sphere at center, radius across, in the world.
func (s *Screen) addAnimated(mesh *Mesh, material *material3D, model matrix, tint Color, anim itemAnimation, center Vector3, radius float32) {
	gpu := mesh.upload()
	if gpu.ID == 0 {
		return
	}
	for _, t := range []*texture3D{material.baseTexture, material.emissiveTexture, material.metalRoughTexture, material.normalTexture, material.occlusionTexture} {
		t.upload()
	}
	if anim.morphs > 0 {
		anim.morphTexture = mesh.uploadMorphs()
		anim.vertices = int(mesh.vertexCount())
	}
	s.pass.items = append(s.pass.items, item3D{
		gpu: gpu, material: material, model: model, tint: tint,
		center: center, radius: radius, anim: anim,
	})
}

// DrawWireframe draws the edges of mesh, placed by transform, as lines one
// pixel wide, unlit: to see where something is, such as the box a game
// checks collisions with. Edges inside a flat side, such as the diagonals of
// a box's faces, are left out.
func (s *Screen) DrawWireframe(mesh *Mesh, transform Transform, color Color) {
	if !s.in3D("DrawWireframe") {
		return
	}
	if mesh == nil {
		reportError(errors.New("golib: Screen.DrawWireframe got a nil mesh: make one with golib.NewBoxMesh or another New...Mesh"))
		return
	}
	mesh.bounds()
	model := transform.matrix()
	s.pass.items = append(s.pass.items, item3D{
		wire: mesh, model: model, tint: color,
		center: model.transformPoint(mesh.center), radius: mesh.radius * model.largestScale(),
	})
}

// DrawLine3D draws a line one pixel wide from one point of the world to
// another, unlit, hidden by what is in front of it: a laser, a path, or a
// direction seen while testing.
func (s *Screen) DrawLine3D(from, to Vector3, color Color) {
	if !s.in3D("DrawLine3D") {
		return
	}
	s.line(from, to, color)
}

func (s *Screen) line(from, to Vector3, color Color) {
	r, g, b, a := float32(color.R)/255, float32(color.G)/255, float32(color.B)/255, float32(color.A)/255
	first := uint32(len(s.pass.lines) / device.LayoutFloats[device.LayoutColored])
	s.pass.lines = append(s.pass.lines, from.X, from.Y, from.Z, r, g, b, a, to.X, to.Y, to.Z, r, g, b, a)
	s.pass.lineIndices = append(s.pass.lineIndices, first, first+1)
}

// DrawGrid draws a square grid of lines on the ground, centered on 0, 0, 0,
// with slices squares along each side, spacing meters apart: a floor to see
// where things are while a game has none.
//
//	screen.DrawGrid(20, 1, golib.Gray) // 20 by 20 meters, a line every meter
func (s *Screen) DrawGrid(slices int, spacing float32, color Color) {
	if !s.in3D("DrawGrid") {
		return
	}
	slices = max(1, slices)
	half := float32(slices) * spacing / 2
	for i := 0; i <= slices; i++ {
		at := -half + float32(i)*spacing
		s.line(Vector3{X: at, Z: -half}, Vector3{X: at, Z: half}, color)
		s.line(Vector3{X: -half, Z: at}, Vector3{X: half, Z: at}, color)
	}
}

// DrawAxes draws the world's three axes from position, length meters long:
// X in red, Y in green and Z in blue. Put them at 0, 0, 0, or at a model, to
// see which way is which.
func (s *Screen) DrawAxes(position Vector3, length float32) {
	if !s.in3D("DrawAxes") {
		return
	}
	s.line(position, position.Add(Vector3{X: length}), Red)
	s.line(position, position.Add(Vector3{Y: length}), Green)
	s.line(position, position.Add(Vector3{Z: length}), Blue)
}

// End3D draws the 3D gathered since Begin3D, and goes back to 2D: what is
// drawn after it goes over the 3D. Run ends a 3D pass the game left open at
// the end of Draw.
func (s *Screen) End3D() {
	if !s.pass.open {
		return
	}
	s.pass.open = false
	if err := renderer3D.load(); err != nil {
		reportError(err)
		return
	}
	// The 3D blends see-through things normally; the 2D's own mode comes
	// back after it.
	if s.blend != BlendNormal {
		device.EndBlend()
	}
	renderer3D.draw(&s.pass)
	if s.blend == BlendAdd {
		device.BeginBlendAdd()
	}
}

// The shaders of the 3D, in GLSL 3.30; the web backend turns them into GLSL
// ES. Each instance brings its matrix and its tint; the tint comes in sRGB,
// as games give colors. Everything is lit in linear light, physically, and
// written into the pass's own picture, which can be brighter than white; the
// compose shader brings it back to the screen's colors.
const (
	litVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 1) in vec3 vertexNormal;
layout(location = 2) in vec2 vertexTexcoord;
layout(location = 3) in vec4 vertexColor;
layout(location = 4) in mat4 instanceModel;
layout(location = 8) in vec4 instanceTint;
uniform mat4 viewProjection;
out vec3 worldPosition;
out vec3 normal;
out vec2 texcoord;
out vec4 color;
#ifdef LIGHTMAPPED
layout(location = 11) in vec2 vertexLightcoord;
out vec2 lightcoord;
#endif

// ANIMATION

void main()
{
    vec3 position = vertexPosition;
    vec3 shapeNormal = vertexNormal;
#ifdef ANIMATED
    animate(position, shapeNormal);
#endif
    mat3 turn = mat3(instanceModel);
    normal = transpose(inverse(turn)) * shapeNormal;
    texcoord = vertexTexcoord;
#ifdef LIGHTMAPPED
    lightcoord = vertexLightcoord;
#endif
    color = vertexColor * vec4(pow(instanceTint.rgb, vec3(2.2)), instanceTint.a);
    vec4 world = instanceModel * vec4(position, 1.0);
    worldPosition = world.xyz;
    gl_Position = viewProjection * world;
}
`
	litFragmentShader = `#version 330
in vec3 worldPosition;
in vec3 normal;
in vec2 texcoord;
in vec4 color;

uniform vec3 eye;
uniform vec4 baseColor;
uniform sampler2D baseTexture;
uniform int hasBaseTexture;
uniform vec3 emissive;
uniform sampler2D emissiveTexture;
uniform int hasEmissiveTexture;
uniform float metallic;
uniform float roughness;
uniform sampler2D metalRoughTexture;
uniform int hasMetalRoughTexture;
uniform sampler2D normalTexture;
uniform int hasNormalTexture;
uniform float normalScale;
uniform sampler2D occlusionTexture;
uniform int hasOcclusionTexture;
uniform float occlusionStrength;
uniform int alphaMode;
uniform float alphaCutoff;
uniform int unlit;
uniform vec2 tiling;
uniform float time;

uniform vec3 sunDirection;
uniform vec3 sunColor;
uniform vec3 skyColor;
uniform vec3 groundColor;
uniform vec4 fog;
uniform sampler2D environmentMap;
uniform int hasEnvironment;
uniform float environmentLevels;
uniform vec2 environmentTurn;
uniform vec4 irradiance[9];
uniform sampler2DShadow shadowMap;
uniform int hasShadows;
uniform mat4 shadowMatrix0;
uniform mat4 shadowMatrix1;
uniform mat4 shadowMatrix2;
uniform vec4 shadowSplits;
uniform vec4 shadowTexels;
uniform vec3 viewForward;
uniform int lightCount;
uniform vec4 lightPositions[8];
uniform vec4 lightColors[8];
uniform vec4 lightDirections[8];
uniform int hasProbes;
uniform highp sampler2D probes;
uniform mat4 probeGrid;
uniform vec3 probeSize;
uniform int reflectionCount;
uniform mat4 levelFromWorld;
uniform vec4 reflectionCenters[4];
uniform vec4 reflectionLows[4];
uniform vec4 reflectionHighs[4];
uniform sampler2D reflection0;
uniform sampler2D reflection1;
uniform sampler2D reflection2;
uniform sampler2D reflection3;
#ifdef LIGHTMAPPED
in vec2 lightcoord;
uniform sampler2D lightmap;
#endif

out vec4 fragColor;

const float PI = 3.14159265;

// The surface's frame from how its position and texture coordinates change
// across the screen, so normal maps need no tangents in the mesh. V grows
// down the picture, where a normal map's green points up.
mat3 surfaceFrame(vec3 n, vec3 p, vec2 uv)
{
    vec3 dp1 = dFdx(p);
    vec3 dp2 = dFdy(p);
    vec2 duv1 = dFdx(uv);
    vec2 duv2 = dFdy(uv);
    vec3 dp2perp = cross(dp2, n);
    vec3 dp1perp = cross(n, dp1);
    vec3 t = dp2perp * duv1.x + dp1perp * duv2.x;
    vec3 b = dp2perp * duv1.y + dp1perp * duv2.y;
    float scale = inversesqrt(max(max(dot(t, t), dot(b, b)), 1e-20));
    return mat3(t * scale, -b * scale, n);
}

// Light that reaches the eye from one light: Lambert's diffuse and GGX's
// highlight.
vec3 shade(vec3 n, vec3 v, vec3 l, vec3 radiance, vec3 albedo, float metal, float rough, vec3 f0)
{
    float nl = max(dot(n, l), 0.0);
    if (nl <= 0.0) return vec3(0.0);
    vec3 h = normalize(v + l);
    float nv = max(dot(n, v), 1e-4);
    float nh = max(dot(n, h), 0.0);
    float a = rough * rough;
    float a2 = a * a;
    float d = a2 / (PI * pow(nh * nh * (a2 - 1.0) + 1.0, 2.0));
    float k = (rough + 1.0) * (rough + 1.0) / 8.0;
    float g = nv / (nv * (1.0 - k) + k) * nl / (nl * (1.0 - k) + k);
    vec3 f = f0 + (1.0 - f0) * pow(1.0 - max(dot(h, v), 0.0), 5.0);
    vec3 specular = d * g * f / (4.0 * nv * nl + 1e-4);
    vec3 diffuse = (1.0 - f) * (1.0 - metal) * albedo;
    return (diffuse + specular * PI) * radiance * nl;
}

// Where a direction of the world is on the HDRI's equirectangular
// panorama, turned back by HDRITurn, its cosine and sine.
vec2 panorama(vec3 d)
{
    d = vec3(d.x * environmentTurn.x - d.z * environmentTurn.y, d.y, d.x * environmentTurn.y + d.z * environmentTurn.x);
    return vec2(0.5 - atan(d.x, d.z) / 6.2831853, acos(clamp(d.y, -1.0, 1.0)) / 3.14159265);
}

// The soft light a surface facing n gets from the HDRI, from its spherical
// harmonics.
vec3 surrounding(vec3 n)
{
    n = vec3(n.x * environmentTurn.x - n.z * environmentTurn.y, n.y, n.x * environmentTurn.y + n.z * environmentTurn.x);
    return max(irradiance[0].rgb * 0.282095
        + irradiance[1].rgb * 0.488603 * n.y + irradiance[2].rgb * 0.488603 * n.z + irradiance[3].rgb * 0.488603 * n.x
        + irradiance[4].rgb * 1.092548 * n.x * n.y + irradiance[5].rgb * 1.092548 * n.y * n.z
        + irradiance[6].rgb * 0.315392 * (3.0 * n.z * n.z - 1.0) + irradiance[7].rgb * 1.092548 * n.x * n.z
        + irradiance[8].rgb * 0.546274 * (n.x * n.x - n.y * n.y), vec3(0.0));
}

// How much of the sun reaches the surface: 1 in full sun, 0 in shadow,
// smoothed over 3 by 3 texels, and fading to sun where the shadows end.
float sunShadow(vec3 n)
{
    if (hasShadows == 0) return 1.0;
    float depth = dot(worldPosition - eye, viewForward);
    if (depth > shadowSplits.z) return 1.0;
    int slice = depth < shadowSplits.x ? 0 : (depth < shadowSplits.y ? 1 : 2);
    float texel = slice == 0 ? shadowTexels.x : (slice == 1 ? shadowTexels.y : shadowTexels.z);
    // Moved off the surface along its normal, more where the sun grazes it,
    // so a surface doesn't shade itself.
    float facing = clamp(dot(n, -sunDirection), 0.0, 1.0);
    vec3 p = worldPosition + n * texel * (1.0 + 2.0 * (1.0 - facing));
    mat4 m = slice == 0 ? shadowMatrix0 : (slice == 1 ? shadowMatrix1 : shadowMatrix2);
    vec4 s = m * vec4(p, 1.0);
    vec3 uvz = s.xyz / s.w * 0.5 + 0.5;
    if (uvz.x < 0.0 || uvz.x > 1.0 || uvz.y < 0.0 || uvz.y > 1.0 || uvz.z > 1.0) return 1.0;
    vec2 step = vec2(1.0 / (3.0 * shadowTexels.w), 1.0 / shadowTexels.w);
    float left = (float(slice) + 0.5 / shadowTexels.w) / 3.0;
    float right = (float(slice) + 1.0 - 0.5 / shadowTexels.w) / 3.0;
    float lit = 0.0;
    for (int x = -1; x <= 1; x++) {
        for (int y = -1; y <= 1; y++) {
            vec2 uv = vec2((uvz.x + float(slice)) / 3.0, uvz.y) + vec2(float(x), float(y)) * step;
            uv.x = clamp(uv.x, left, right);
            lit += texture(shadowMap, vec3(uv, uvz.z - 0.0003));
        }
    }
    lit /= 9.0;
    return mix(lit, 1.0, smoothstep(shadowSplits.z * 0.85, shadowSplits.z, depth));
}

// The soft light the level's probes give a surface at p facing n, blended
// from the eight around it, leaving out those inside walls, and how much it
// counts: 1 inside the grid, fading to 0 a probe's spacing outside it.
vec4 probeTexel(int k)
{
    return texelFetch(probes, ivec2(k % DATA_WIDTH, k / DATA_WIDTH), 0);
}

vec4 probeLight(vec3 p, vec3 n)
{
    vec3 g = (probeGrid * vec4(p, 1.0)).xyz;
    vec3 inside = clamp(g, vec3(0.0), probeSize - 1.0);
    float reach = clamp(1.0 - length(g - inside), 0.0, 1.0);
    if (reach <= 0.0) return vec4(0.0);
    vec3 m = normalize(mat3(probeGrid) * n);
    vec3 base = min(floor(inside), max(probeSize - 2.0, vec3(0.0)));
    vec3 f = clamp(inside - base, 0.0, 1.0);
    vec3 light = vec3(0.0);
    float weight = 0.0;
    for (int c = 0; c < 8; c++) {
        vec3 corner = base + vec3(float(c & 1), float((c >> 1) & 1), float((c >> 2) & 1));
        if (any(greaterThan(corner, probeSize - 1.0))) continue;
        vec3 w3 = mix(vec3(1.0) - f, f, vec3(float(c & 1), float((c >> 1) & 1), float((c >> 2) & 1)));
        float w = w3.x * w3.y * w3.z;
        int k = int(corner.x + probeSize.x * (corner.y + probeSize.y * corner.z)) * PROBE_TEXELS;
        vec4 t0 = probeTexel(k);
        if (t0.w < 0.5 || w <= 0.0) continue;
        vec3 sh = t0.rgb * 0.282095
            + probeTexel(k + 1).rgb * 0.488603 * m.y
            + probeTexel(k + 2).rgb * 0.488603 * m.z
            + probeTexel(k + 3).rgb * 0.488603 * m.x;
        light += max(sh, vec3(0.0)) * w;
        weight += w;
    }
    if (weight <= 0.0) return vec4(0.0);
    return vec4(light / weight, reach);
}

// What a level's reflection probe shows along r from p, roughened by rough,
// and 1, or 0 outside every probe's box: the panorama seen from the probe,
// looked up where r leaves the box, so the walls it reflects are where they
// are. The smallest box p is in counts.
vec4 reflectionAt(vec3 p, vec3 r, float rough)
{
    if (reflectionCount == 0) return vec4(0.0);
    vec3 lp = (levelFromWorld * vec4(p, 1.0)).xyz;
    vec3 lr = normalize(mat3(levelFromWorld) * r);
    for (int i = 0; i < 4; i++) {
        if (i >= reflectionCount) break;
        vec3 lo = reflectionLows[i].xyz, hi = reflectionHighs[i].xyz;
        if (any(lessThan(lp, lo)) || any(greaterThan(lp, hi))) continue;
        vec3 leave = max((hi - lp) / lr, (lo - lp) / lr);
        float t = min(min(leave.x, leave.y), leave.z);
        vec3 d = normalize(lp + lr * t - reflectionCenters[i].xyz);
        vec2 uv = vec2(0.5 - atan(d.x, d.z) / 6.2831853, acos(clamp(d.y, -1.0, 1.0)) / 3.14159265);
        float lod = rough * reflectionCenters[i].w;
        vec3 c;
        if (i == 0) c = textureLod(reflection0, uv, lod).rgb;
        else if (i == 1) c = textureLod(reflection1, uv, lod).rgb;
        else if (i == 2) c = textureLod(reflection2, uv, lod).rgb;
        else c = textureLod(reflection3, uv, lod).rgb;
        return vec4(c, 1.0);
    }
    return vec4(0.0);
}

// What a material's surface is at one point, which a game's own shader,
// Material.Shader, changes before GoLib lights it.
struct Surface {
    vec4 color;      // linear, with its opacity
    vec3 emission;   // linear light it gives off
    float metallic;
    float roughness;
    vec3 normal;     // length 1, in the world
    vec3 position;   // in the world
    vec2 texcoord;   // tiled
    float time;      // seconds of game time
};

// SURFACE

void main()
{
    vec2 uv = texcoord * tiling;
    vec4 albedo = baseColor * color;
    if (hasBaseTexture == 1) albedo *= texture(baseTexture, uv);
    vec3 glow = emissive;
    if (hasEmissiveTexture == 1) glow *= texture(emissiveTexture, uv).rgb;
    float metal = metallic;
    float rough = roughness;
    if (hasMetalRoughTexture == 1) {
        vec4 mr = texture(metalRoughTexture, uv);
        rough *= mr.g;
        metal *= mr.b;
    }
    vec3 n = normalize(normal);
    if (!gl_FrontFacing) n = -n;
    if (hasNormalTexture == 1) {
        vec3 bump = texture(normalTexture, uv).xyz * 2.0 - 1.0;
        bump.xy *= normalScale;
        n = normalize(surfaceFrame(n, worldPosition, uv) * bump);
    }
    float occlusion = 1.0;
    if (hasOcclusionTexture == 1) occlusion = mix(1.0, texture(occlusionTexture, uv).r, occlusionStrength);
#ifdef CUSTOM_SURFACE
    Surface s = Surface(albedo, glow, metal, rough, n, worldPosition, uv, time);
    surface(s);
    albedo = s.color;
    glow = s.emission;
    metal = s.metallic;
    rough = s.roughness;
    n = normalize(s.normal);
#endif

    if (alphaMode == 1 && albedo.a < alphaCutoff) discard;
    float alpha = alphaMode == 2 ? albedo.a : 1.0;
    if (unlit == 1) {
        fragColor = vec4((albedo.rgb + glow) * alpha, alpha);
        return;
    }
    rough = clamp(rough, 0.04, 1.0);
    metal = clamp(metal, 0.0, 1.0);
    vec3 v = normalize(eye - worldPosition);
    vec3 f0 = mix(vec3(0.04), albedo.rgb, metal);

    vec3 light = shade(n, v, -sunDirection, sunColor * sunShadow(n), albedo.rgb, metal, rough, f0);
    for (int i = 0; i < 8; i++) {
        if (i >= lightCount) break;
        vec3 toLight = lightPositions[i].xyz - worldPosition;
        float distance = length(toLight);
        vec3 l = toLight / max(distance, 1e-4);
        float reach = distance / lightPositions[i].w;
        float fade = clamp(1.0 - reach * reach * reach * reach, 0.0, 1.0);
        float strength = fade * fade / max(distance * distance, 0.01);
        float inner = lightDirections[i].w, outer = lightColors[i].w;
        if (outer > -1.5) {
            float along = dot(-l, lightDirections[i].xyz);
            strength *= smoothstep(outer, inner, along);
        }
        light += shade(n, v, l, lightColors[i].rgb * strength, albedo.rgb, metal, rough, f0);
    }

    // The soft light of the sky and the ground, diffuse and reflected, the
    // reflection fading as the surface roughens.
    vec3 r = reflect(-v, n);
    vec3 around;
    vec3 mirrored;
    if (hasEnvironment == 1) {
        around = surrounding(n);
        mirrored = textureLod(environmentMap, panorama(r), rough * environmentLevels).rgb;
    } else {
        around = mix(groundColor, skyColor, n.y * 0.5 + 0.5);
        // A rough surface blurs its reflection into the light all around it.
        mirrored = mix(mix(groundColor, skyColor, smoothstep(-0.2, 0.2, r.y)), around, rough);
    }
#ifndef LIGHTMAPPED
    if (hasProbes == 1) {
        // The light baked in the level's probes in place of the sky's and
        // the ground's, and reflections dimmed where less of the sky reaches.
        vec4 probe = probeLight(worldPosition, n);
        if (probe.a > 0.0) {
            const vec3 luma = vec3(0.2126, 0.7152, 0.0722);
            float open = clamp(dot(probe.rgb, luma) / max(dot(around, luma), 1e-4), 0.0, 1.0);
            mirrored *= mix(1.0, open, probe.a);
            around = mix(around, probe.rgb, probe.a);
        }
    }
#endif
#ifdef LIGHTMAPPED
    // The light baked in Blender in place of the sky's and the ground's:
    // the sky's as much of it as reaches here, and what the sun, the lamps
    // and the sky bounced off everything around. Reflections of the sky
    // dim where less of it reaches, as in a room.
    vec3 baked = texture(lightmap, lightcoord).rgb;
    const vec3 luma = vec3(0.2126, 0.7152, 0.0722);
    mirrored *= clamp(dot(baked, luma) / max(dot(around, luma), 1e-4), 0.0, 1.0);
    around = baked;
#endif
    // Inside a reflection probe's box, the level reflects itself.
    vec4 boxed = reflectionAt(worldPosition, r, rough);
    if (boxed.a > 0.0) mirrored = boxed.rgb;
    float nv = max(dot(n, v), 0.0);
    vec3 fresnel = f0 + (max(vec3(1.0 - rough), f0) - f0) * pow(1.0 - nv, 5.0);
    vec3 ambient = (1.0 - fresnel) * (1.0 - metal) * albedo.rgb * around + fresnel * mirrored;
    light += ambient * occlusion + glow;

    if (fog.w > 0.0) {
        float distance = length(eye - worldPosition);
        float amount = clamp((distance - fog.w * 0.1) / (fog.w * 0.9), 0.0, 1.0);
        light = mix(light, fog.rgb, amount);
    }
    fragColor = vec4(light * alpha, alpha);
}
`
	// fullscreenVertexShader covers the view with one triangle, its corners
	// worked out from gl_VertexID, for the sky and the composing.
	fullscreenVertexShader = `#version 330
out vec2 texcoord;

void main()
{
    vec2 corner = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
    texcoord = corner;
    gl_Position = vec4(corner * 2.0 - 1.0, 1.0, 1.0);
}
`
	skyFragmentShader = `#version 330
in vec2 texcoord;
uniform mat4 inverseViewProjection;
uniform vec3 eye;
uniform vec3 skyTop;
uniform vec3 horizon;
uniform vec3 skyBottom;
uniform vec4 fog;
uniform sampler2D environmentMap;
uniform int hasEnvironment;
uniform vec2 environmentTurn;
out vec4 fragColor;

void main()
{
    vec4 far = inverseViewProjection * vec4(texcoord * 2.0 - 1.0, 1.0, 1.0);
    vec3 direction = normalize(far.xyz / far.w - eye);
    vec3 color = direction.y > 0.0
        ? mix(horizon, skyTop, pow(direction.y, 0.6))
        : mix(horizon, skyBottom, pow(-direction.y, 0.4));
    if (hasEnvironment == 1) {
        vec3 d = direction;
        d = vec3(d.x * environmentTurn.x - d.z * environmentTurn.y, d.y, d.x * environmentTurn.y + d.z * environmentTurn.x);
        vec2 uv = vec2(0.5 - atan(d.x, d.z) / 6.2831853, acos(clamp(d.y, -1.0, 1.0)) / 3.14159265);
        color = textureLod(environmentMap, uv, 0.0).rgb;
    }
    if (fog.w > 0.0) color = mix(color, fog.rgb, 1.0 - smoothstep(0.0, 0.25, abs(direction.y)));
    fragColor = vec4(color, 1.0);
}
`
	// composeFragmentShader brings the pass's picture, in linear light that
	// can be brighter than white, back to the screen's sRGB colors with
	// Khronos' PBR Neutral tonemapper, which leaves a color as it is up to
	// 0.76 and softens only what is brighter, so golib.Red lit by the sun is
	// still golib.Red. It gives colors premultiplied by their opacity, to go
	// over the 2D under the pass.
	composeFragmentShader = `#version 330
in vec2 texcoord;
uniform sampler2D picture;
uniform sampler2D glow;
uniform float glowStrength;
uniform float exposure;
uniform vec2 texel;
uniform int smoothEdges;
out vec4 fragColor;

vec3 neutral(vec3 color)
{
    const float startCompression = 0.8 - 0.04;
    const float desaturation = 0.15;
    float x = min(color.r, min(color.g, color.b));
    float offset = x < 0.08 ? x - 6.25 * x * x : 0.04;
    color -= offset;
    float peak = max(color.r, max(color.g, color.b));
    if (peak < startCompression) return color;
    const float d = 1.0 - startCompression;
    float newPeak = 1.0 - d * d / (peak + d - startCompression);
    color *= newPeak / peak;
    float g = 1.0 - 1.0 / (desaturation * (peak - newPeak) + 1.0);
    return mix(color, vec3(newPeak), g);
}

// The finished color at uv, glow added, tonemapped, in sRGB, premultiplied
// by its opacity.
vec4 finished(vec2 uv)
{
    vec4 c = texture(picture, uv);
    vec3 light = c.rgb;
    float a = c.a;
    if (glowStrength > 0.0) {
        vec3 g = texture(glow, uv).rgb * glowStrength;
        light += g;
        a = clamp(a + max(g.r, max(g.g, g.b)), 0.0, 1.0);
    }
    if (a <= 0.0) return vec4(0.0);
    vec3 color = neutral(max(light / a * exposure, vec3(0.0)));
    return vec4(pow(color, vec3(1.0 / 2.2)) * a, a);
}

float luma(vec4 c)
{
    return dot(c.rgb, vec3(0.299, 0.587, 0.114));
}

void main()
{
    vec4 middle = finished(texcoord);
    if (smoothEdges == 0) {
        fragColor = middle;
        return;
    }
    // FXAA: where the colors around differ, blend along the edge they make.
    vec4 nw = finished(texcoord + vec2(-1.0, -1.0) * texel);
    vec4 ne = finished(texcoord + vec2(1.0, -1.0) * texel);
    vec4 sw = finished(texcoord + vec2(-1.0, 1.0) * texel);
    vec4 se = finished(texcoord + vec2(1.0, 1.0) * texel);
    float lm = luma(middle), lnw = luma(nw), lne = luma(ne), lsw = luma(sw), lse = luma(se);
    float lowest = min(lm, min(min(lnw, lne), min(lsw, lse)));
    float highest = max(lm, max(max(lnw, lne), max(lsw, lse)));
    if (highest - lowest < max(0.0312, highest * 0.125)) {
        fragColor = middle;
        return;
    }
    vec2 dir = vec2(-((lnw + lne) - (lsw + lse)), (lnw + lsw) - (lne + lse));
    float reduce = max((lnw + lne + lsw + lse) * 0.25 * 0.125, 1.0 / 128.0);
    dir = clamp(dir / (min(abs(dir.x), abs(dir.y)) + reduce), vec2(-8.0), vec2(8.0)) * texel;
    vec4 a = 0.5 * (finished(texcoord + dir * (1.0 / 3.0 - 0.5)) + finished(texcoord + dir * (2.0 / 3.0 - 0.5)));
    vec4 b = a * 0.5 + 0.25 * (finished(texcoord - dir * 0.5) + finished(texcoord + dir * 0.5));
    float lb = luma(b);
    fragColor = (lb < lowest || lb > highest) ? a : b;
}
`
	lineVertexShader = `#version 330
layout(location = 0) in vec3 vertexPosition;
layout(location = 3) in vec4 vertexColor;
uniform mat4 model;
uniform mat4 viewProjection;
out vec4 tint;

void main()
{
    tint = vertexColor;
    gl_Position = viewProjection * model * vec4(vertexPosition, 1.0);
    // A hair nearer the camera, so a line on a surface, such as the grid on
    // the ground or a wireframe over its mesh, isn't half hidden by it.
    gl_Position.z -= 0.0001 * gl_Position.w;
}
`
	lineFragmentShader = `#version 330
in vec4 tint;
uniform vec4 color;
out vec4 fragColor;

void main()
{
    vec4 c = tint * color;
    fragColor = vec4(pow(c.rgb, vec3(2.2)) * c.a, c.a);
}
`
)

// animationShader bends a vertex by its skin's joints and its shape keys,
// for the vertex shaders of animated meshes: the joints' matrices and the
// shape keys' weights of each instance are a block of texels in the pass's
// animation texture, animationStride texels apart, and the shape keys' moves
// are in the mesh's own, two texels for each vertex of each.
const animationShader = `layout(location = 9) in vec4 vertexJoints;
layout(location = 10) in vec4 vertexWeights;
uniform highp sampler2D animation;
uniform highp sampler2D morphTargets;
uniform int animationBase;
uniform int animationStride;
uniform int jointCount;
uniform int morphCount;
uniform int morphVertices;

vec4 animationTexel(int k)
{
    return texelFetch(animation, ivec2(k % DATA_WIDTH, k / DATA_WIDTH), 0);
}

vec4 morphTexel(int k)
{
    return texelFetch(morphTargets, ivec2(k % DATA_WIDTH, k / DATA_WIDTH), 0);
}

mat4 jointMatrix(int block, float joint)
{
    int k = block + int(joint) * 4;
    return mat4(animationTexel(k), animationTexel(k + 1), animationTexel(k + 2), animationTexel(k + 3));
}

void animate(inout vec3 position, inout vec3 n)
{
    int block = animationBase + gl_InstanceID * animationStride;
    if (morphCount > 0) {
        int weights = block + jointCount * 4;
        for (int t = 0; t < morphCount; t++) {
            float w = animationTexel(weights + t / 4)[t % 4];
            if (w != 0.0) {
                int k = (t * morphVertices + gl_VertexID) * 2;
                position += w * morphTexel(k).xyz;
                n += w * morphTexel(k + 1).xyz;
            }
        }
    }
    if (jointCount > 0) {
        mat4 skin = vertexWeights.x * jointMatrix(block, vertexJoints.x)
            + vertexWeights.y * jointMatrix(block, vertexJoints.y)
            + vertexWeights.z * jointMatrix(block, vertexJoints.z)
            + vertexWeights.w * jointMatrix(block, vertexJoints.w);
        position = (skin * vec4(position, 1.0)).xyz;
        n = mat3(skin) * n;
    }
}
`

// litFragment fills in the sizes the lit fragment shader reads its probes
// with.
func litFragment(fragment string) string {
	fragment = strings.ReplaceAll(fragment, "DATA_WIDTH", strconv.Itoa(device.DataTextureWidth))
	return strings.ReplaceAll(fragment, "PROBE_TEXELS", strconv.Itoa(probeTexels))
}

// animated3D returns a vertex shader made to bend what it draws by bones and
// shape keys.
func animated3D(vertex string) string {
	code := strings.ReplaceAll(animationShader, "DATA_WIDTH", strconv.Itoa(device.DataTextureWidth))
	vertex = strings.Replace(vertex, "// ANIMATION", code, 1)
	return strings.Replace(vertex, "#version 330\n", "#version 330\n#define ANIMATED\n", 1)
}

// The texture units of the animation texture and of a mesh's shape keys,
// past the ones the materials, the shadows and the HDRI use, and the one
// the backends make textures on.
const (
	animationUnit = 8
	morphUnit     = 9
)

// litProgram is the program meshes are drawn with, and where its uniforms
// sit.
type litProgram struct {
	program                                     device.Program
	viewProjection, eye                         int32
	baseColor, baseTexture, hasBaseTexture      int32
	emissive, emissiveTexture, hasEmissive      int32
	metallic, roughness                         int32
	metalRoughTexture, hasMetalRoughTexture     int32
	normalTexture, hasNormalTexture             int32
	normalScale                                 int32
	occlusionTexture, hasOcclusionTexture       int32
	occlusionStrength                           int32
	alphaMode, alphaCutoff, unlit               int32
	tiling, time                                int32
	sunDirection, sunColor, skyColor, ground    int32
	fog, lightCount                             int32
	lightPositions, lightColors, lightDirection int32
	shadowMap, hasShadows                       int32
	environmentMap, hasEnvironment              int32
	environmentLevels, irradiance               int32
	environmentTurn                             int32
	shadowMatrices                              [shadowCascades]int32
	shadowSplits, shadowTexels, viewForward     int32
	lightmap                                    int32
	hasProbes, probes, probeGrid, probeSize     int32
	reflectionCount, levelFromWorld             int32
	reflectionCenters, reflectionLows           int32
	reflectionHighs                             int32
	reflections                                 [maxReflections]int32
	bending                                     bendingUniforms
}

// bendingUniforms are where an animated program's uniforms sit.
type bendingUniforms struct {
	animation, morphTargets        int32
	animationBase, animationStride int32
	jointCount, morphCount         int32
	morphVertices                  int32
}

func locateBending(program device.Program) bendingUniforms {
	at := func(name string) int32 { return device.UniformLocation(program, name) }
	return bendingUniforms{
		animation: at("animation"), morphTargets: at("morphTargets"),
		animationBase: at("animationBase"), animationStride: at("animationStride"),
		jointCount: at("jointCount"), morphCount: at("morphCount"), morphVertices: at("morphVertices"),
	}
}

// set gives an animated program what a group of instances is bent by.
func (b bendingUniforms) set(g groupAnimation) {
	device.SetUniformInt(b.animation, animationUnit)
	device.SetUniformInt(b.morphTargets, morphUnit)
	device.SetUniformInt(b.animationBase, int32(g.base))
	device.SetUniformInt(b.animationStride, int32(g.stride))
	device.SetUniformInt(b.jointCount, int32(g.joints))
	device.SetUniformInt(b.morphCount, int32(g.morphs))
	device.SetUniformInt(b.morphVertices, int32(g.vertices))
	if g.morphs > 0 {
		device.BindDataTexture(morphUnit, g.morphTexture)
	}
}

// groupAnimation is how the instances of one draw are bent: where their
// blocks start in the animation texture, how many texels each takes, and
// what they hold. The zero one isn't bent.
type groupAnimation struct {
	base, stride   int // in texels
	joints, morphs int
	vertices       int
	morphTexture   device.DataTexture
}

// groupAnimationOf returns the bending of a draw whose first instance is
// item, its blocks starting at texel base.
func groupAnimationOf(item *item3D, base int) groupAnimation {
	a := item.anim
	if a.floats == 0 {
		return groupAnimation{}
	}
	return groupAnimation{
		base: base, stride: a.floats / 4, joints: a.joints, morphs: a.morphs,
		vertices: a.vertices, morphTexture: a.morphTexture,
	}
}

// lineProgram is the program lines and wireframes are drawn with.
type lineProgram struct {
	program                      device.Program
	model, viewProjection, color int32
}

// skyProgram draws the sky behind everything.
type skyProgram struct {
	program                         device.Program
	inverseViewProjection, eye      int32
	skyTop, horizon, skyBottom, fog int32
	environmentMap, hasEnvironment  int32
	environmentTurn                 int32
}

// composeProgram puts a pass over the 2D.
type composeProgram struct {
	program            device.Program
	picture, exposure  int32
	glow, glowStrength int32
	texel, smoothEdges int32
}

// state3D is what every 3D pass draws with: made when the first one ends,
// and freed with the window.
type state3D struct {
	loaded    bool
	lit       litProgram
	lines     lineProgram
	sky       skyProgram
	compose   composeProgram
	shadow    shadowProgram
	shadowMap device.ShadowMap
	bloomDown bloomProgram
	bloomUp   bloomProgram
	bloom     bloomChain
	surfaces  map[programKind]*litProgram // programs with a game's own surface, bending or a lightmap, nil for one that didn't compile
	lineMesh  device.Mesh
	instances []float32 // the instances of the draw being built

	shadowBent    *shadowProgram     // the shadow program that bends, once a pass needs it
	animation     []float32          // the animated items' blocks, as the animation texture gets them
	animationData device.DataTexture // the animation texture
}

// programKind says which lit program a draw needs: with a game's own
// surface, bending what it draws by bones and shape keys, and lit by a
// lightmap.
type programKind struct {
	surface     string
	animated    bool
	lightmapped bool
}

var renderer3D state3D

func compile3D(name, vertex, fragment string) (device.Program, error) {
	program, err := device.NewProgram(vertex, fragment)
	if err != nil {
		return program, fmt.Errorf("golib: the 3D's %s shader doesn't compile on this graphics card: %w", name, err)
	}
	return program, nil
}

// load makes the 3D's programs, the first time a 3D pass ends.
func (r *state3D) load() error {
	if r.loaded {
		return nil
	}
	var made []device.Program
	build := func(name, vertex, fragment string) (device.Program, error) {
		p, err := compile3D(name, vertex, fragment)
		if err != nil {
			for _, m := range made {
				device.UnloadProgram(m)
			}
			return p, err
		}
		made = append(made, p)
		return p, nil
	}
	lit, err := build("lit", litVertexShader, litFragment(litFragmentShader))
	if err != nil {
		return err
	}
	lines, err := build("line", lineVertexShader, lineFragmentShader)
	if err != nil {
		return err
	}
	sky, err := build("sky", fullscreenVertexShader, skyFragmentShader)
	if err != nil {
		return err
	}
	compose, err := build("compose", fullscreenVertexShader, composeFragmentShader)
	if err != nil {
		return err
	}
	shadow, err := build("shadow", shadowVertexShader, shadowFragmentShader)
	if err != nil {
		return err
	}
	bloomDown, err := build("bloom", fullscreenVertexShader, bloomDownShader)
	if err != nil {
		return err
	}
	bloomUp, err := build("bloom", fullscreenVertexShader, bloomUpShader)
	if err != nil {
		return err
	}
	at := func(p device.Program, name string) int32 { return device.UniformLocation(p, name) }
	r.lit = locateLit(lit)
	r.lines = lineProgram{program: lines, model: at(lines, "model"), viewProjection: at(lines, "viewProjection"), color: at(lines, "color")}
	r.sky = skyProgram{
		program: sky, inverseViewProjection: at(sky, "inverseViewProjection"), eye: at(sky, "eye"),
		skyTop: at(sky, "skyTop"), horizon: at(sky, "horizon"), skyBottom: at(sky, "skyBottom"), fog: at(sky, "fog"),
	}
	r.compose = composeProgram{
		program: compose, picture: at(compose, "picture"), exposure: at(compose, "exposure"),
		glow: at(compose, "glow"), glowStrength: at(compose, "glowStrength"),
		texel: at(compose, "texel"), smoothEdges: at(compose, "smoothEdges"),
	}
	r.bloomDown = bloomProgram{program: bloomDown, source: at(bloomDown, "source"), texel: at(bloomDown, "texel"), first: at(bloomDown, "first")}
	r.bloomUp = bloomProgram{program: bloomUp, source: at(bloomUp, "source"), texel: at(bloomUp, "texel")}
	r.shadow = locateShadow(shadow)
	r.sky.environmentMap, r.sky.hasEnvironment = at(sky, "environmentMap"), at(sky, "hasEnvironment")
	r.sky.environmentTurn = at(sky, "environmentTurn")
	r.loaded = true
	return nil
}

// locateLit finds where the lit shader's uniforms sit in program, the
// plain one or one with a game's own surface, bending what it draws or not.
func locateLit(program device.Program) litProgram {
	at := func(name string) int32 { return device.UniformLocation(program, name) }
	p := litProgram{
		program: program, viewProjection: at("viewProjection"), eye: at("eye"),
		baseColor: at("baseColor"), baseTexture: at("baseTexture"), hasBaseTexture: at("hasBaseTexture"),
		emissive: at("emissive"), emissiveTexture: at("emissiveTexture"), hasEmissive: at("hasEmissiveTexture"),
		metallic: at("metallic"), roughness: at("roughness"),
		metalRoughTexture: at("metalRoughTexture"), hasMetalRoughTexture: at("hasMetalRoughTexture"),
		normalTexture: at("normalTexture"), hasNormalTexture: at("hasNormalTexture"), normalScale: at("normalScale"),
		occlusionTexture: at("occlusionTexture"), hasOcclusionTexture: at("hasOcclusionTexture"),
		occlusionStrength: at("occlusionStrength"),
		alphaMode:         at("alphaMode"), alphaCutoff: at("alphaCutoff"), unlit: at("unlit"),
		tiling: at("tiling"), time: at("time"),
		sunDirection: at("sunDirection"), sunColor: at("sunColor"), skyColor: at("skyColor"), ground: at("groundColor"),
		fog: at("fog"), lightCount: at("lightCount"),
		lightPositions: at("lightPositions"), lightColors: at("lightColors"), lightDirection: at("lightDirections"),
		shadowMap: at("shadowMap"), hasShadows: at("hasShadows"),
		environmentMap: at("environmentMap"), hasEnvironment: at("hasEnvironment"),
		environmentLevels: at("environmentLevels"), irradiance: at("irradiance"), environmentTurn: at("environmentTurn"),
		shadowSplits: at("shadowSplits"), shadowTexels: at("shadowTexels"), viewForward: at("viewForward"),
		lightmap: at("lightmap"),
		hasProbes: at("hasProbes"), probes: at("probes"), probeGrid: at("probeGrid"), probeSize: at("probeSize"),
		reflectionCount: at("reflectionCount"), levelFromWorld: at("levelFromWorld"),
		reflectionCenters: at("reflectionCenters"), reflectionLows: at("reflectionLows"), reflectionHighs: at("reflectionHighs"),
	}
	for k := range maxReflections {
		p.reflections[k] = at(fmt.Sprintf("reflection%d", k))
	}
	for k := range shadowCascades {
		p.shadowMatrices[k] = at(fmt.Sprintf("shadowMatrix%d", k))
	}
	p.bending = locateBending(program) // all -1 in a program that doesn't bend
	return p
}

// surfaceProgram returns the lit program for a material: the plain one, or,
// for one with a shader of the game's own, that shader's, compiled the first
// time a material uses it, one that bends what it draws for animated
// meshes, and one lit by a lightmap. A shader that doesn't compile is
// reported, and the material drawn with the plain program.
func (r *state3D) surfaceProgram(m *material3D, animated, lightmapped bool) *litProgram {
	kind := programKind{surface: m.surface, animated: animated, lightmapped: lightmapped}
	if kind == (programKind{}) {
		return &r.lit
	}
	if p, ok := r.surfaces[kind]; ok {
		if p == nil {
			return &r.lit
		}
		return p
	}
	vertex, fragment := litVertexShader, litFragment(litFragmentShader)
	if animated {
		vertex = animated3D(vertex)
	}
	if m.surface != "" {
		fragment = strings.Replace(fragment, "// SURFACE", m.surface, 1)
		fragment = strings.Replace(fragment, "#version 330\n", "#version 330\n#define CUSTOM_SURFACE\n", 1)
	}
	if lightmapped {
		vertex = strings.Replace(vertex, "#version 330\n", "#version 330\n#define LIGHTMAPPED\n", 1)
		fragment = strings.Replace(fragment, "#version 330\n", "#version 330\n#define LIGHTMAPPED\n", 1)
	}
	program, err := device.NewProgram(vertex, fragment)
	if r.surfaces == nil {
		r.surfaces = map[programKind]*litProgram{}
	}
	if err != nil {
		if m.surface != "" {
			reportError(fmt.Errorf("golib: a Material's Shader doesn't compile: %w", err))
		} else {
			reportError(fmt.Errorf("golib: the 3D's animated or lightmapped shader doesn't compile on this graphics card: %w", err))
		}
		r.surfaces[kind] = nil
		return &r.lit
	}
	p := locateLit(program)
	r.surfaces[kind] = &p
	return &p
}

// unload frees the programs and the lines' mesh. The window must still be
// open.
func (r *state3D) unload() {
	if !r.loaded {
		return
	}
	for _, p := range []device.Program{r.lit.program, r.lines.program, r.sky.program, r.compose.program, r.shadow.program, r.bloomDown.program, r.bloomUp.program} {
		device.UnloadProgram(p)
	}
	for _, p := range r.surfaces {
		if p != nil {
			device.UnloadProgram(p.program)
		}
	}
	if r.shadowBent != nil {
		device.UnloadProgram(r.shadowBent.program)
	}
	device.UnloadDataTexture(r.animationData)
	r.unloadBloom()
	device.UnloadShadowMap(r.shadowMap)
	device.UnloadMesh(r.lineMesh)
	*r = state3D{}
}

// shadowUnit is the texture unit the sun's shadow map is bound to, past the
// five a material uses, and environmentUnit the HDRI's.
const (
	shadowUnit      = 5
	environmentUnit = 6
	lightmapUnit    = 10 // past the animation's and the shape keys'
	probesUnit      = 11
	reflectionUnit  = 12 // and the three after it

	// maxReflections is how many reflection probes a pass uses: the nearest
	// to the camera.
	maxReflections = 4
)

// colorFloats returns c as four numbers from 0 to 1, as it is: sRGB.
func colorFloats(c Color) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

// frustum is the six planes around what a camera sees, each as a normal
// pointing inwards and a distance: a point p is inside a plane when
// normal.Dot(p) + distance >= 0.
type frustum [6]struct {
	normal   Vector3
	distance float32
}

// frustumOf takes the planes out of a view-projection matrix.
func frustumOf(m matrix) frustum {
	row := func(i int) [4]float32 { return [4]float32{m[i], m[4+i], m[8+i], m[12+i]} }
	r0, r1, r2, r3 := row(0), row(1), row(2), row(3)
	var f frustum
	for i, p := range [6][4]float32{
		{r3[0] + r0[0], r3[1] + r0[1], r3[2] + r0[2], r3[3] + r0[3]},
		{r3[0] - r0[0], r3[1] - r0[1], r3[2] - r0[2], r3[3] - r0[3]},
		{r3[0] + r1[0], r3[1] + r1[1], r3[2] + r1[2], r3[3] + r1[3]},
		{r3[0] - r1[0], r3[1] - r1[1], r3[2] - r1[2], r3[3] - r1[3]},
		{r3[0] + r2[0], r3[1] + r2[1], r3[2] + r2[2], r3[3] + r2[3]},
		{r3[0] - r2[0], r3[1] - r2[1], r3[2] - r2[2], r3[3] - r2[3]},
	} {
		normal := Vector3{X: p[0], Y: p[1], Z: p[2]}
		length := normal.Length()
		if length > 0 {
			f[i].normal, f[i].distance = normal.Scale(1/length), p[3]/length
		}
	}
	return f
}

// sees says whether any of the sphere at center, radius across, is inside.
func (f *frustum) sees(center Vector3, radius float32) bool {
	for _, p := range f {
		if p.normal.Dot(center)+p.distance < -radius {
			return false
		}
	}
	return true
}

// vec3 returns v as the first three of four uniform values.
func vec3(v Vector3) [4]float32 {
	return [4]float32{v.X, v.Y, v.Z}
}

// draw draws a pass.
func (r *state3D) draw(pass *pass3D) {
	view, projection := pass.camera.matrices()
	viewProjectionMatrix := projection.mul(view)
	viewProjection := [16]float32(viewProjectionMatrix)
	sight := frustumOf(viewProjectionMatrix)
	eye, _ := pass.camera.view()
	env := pass.environment.uniforms()
	lights := lightsFor(pass.lights, eye)

	// What is solid, grouped by mesh and material in the order first drawn,
	// so every copy of a mesh is one draw; what is see-through, one by one.
	type groupKey struct {
		mesh     uint32
		material *material3D
		floats   int // an animated item's block: items bent alike go together
		lightmap *texture3D
	}
	type group struct {
		gpu      device.Mesh
		material *material3D
		lightmap *texture3D
		items    []int
	}
	var groups []group
	byKey := map[groupKey]int{}
	var wires, seeThrough []int
	for i := range pass.items {
		item := &pass.items[i]
		if !sight.sees(item.center, item.radius) {
			continue
		}
		switch {
		case item.wire != nil:
			wires = append(wires, i)
		case item.material.alpha == alphaBlend || item.tint.A < 255:
			seeThrough = append(seeThrough, i)
		default:
			key := groupKey{item.gpu.ID, item.material, item.anim.floats, item.lightmap}
			at, ok := byKey[key]
			if !ok {
				at = len(groups)
				byKey[key] = at
				groups = append(groups, group{gpu: item.gpu, material: item.material, lightmap: item.lightmap})
			}
			groups[at].items = append(groups[at].items, i)
		}
	}

	distance := func(i int) float32 { return pass.items[i].center.Distance(eye) }
	slices.SortStableFunc(seeThrough, func(a, b int) int {
		return cmp.Compare(distance(b), distance(a)) // farthest first
	})

	device.Begin3D()

	// Each group's instances, built once for the shadows and the picture,
	// and the blocks of the animated ones, each draw's together, for the
	// animation texture.
	r.animation = r.animation[:0]
	built := make([]shadowGroup, len(groups))
	for k, g := range groups {
		var data []float32
		bending := groupAnimationOf(&pass.items[g.items[0]], len(r.animation)/4)
		for _, i := range g.items {
			item := &pass.items[i]
			tint := colorFloats(item.tint)
			data = append(data, item.model[:]...)
			data = append(data, tint[:]...)
			r.animation = append(r.animation, pass.animation[item.anim.at:item.anim.at+item.anim.floats]...)
		}
		built[k] = shadowGroup{gpu: g.gpu, material: g.material, instances: data, animation: bending, lightmap: g.lightmap}
	}
	seeThroughBending := make([]groupAnimation, len(seeThrough))
	for k, i := range seeThrough {
		item := &pass.items[i]
		seeThroughBending[k] = groupAnimationOf(item, len(r.animation)/4)
		r.animation = append(r.animation, pass.animation[item.anim.at:item.anim.at+item.anim.floats]...)
	}
	if len(r.animation) > 0 {
		if r.animationData.ID == 0 {
			r.animationData = device.NewDataTexture(r.animation)
		} else {
			device.UpdateDataTexture(&r.animationData, r.animation)
		}
		device.BindDataTexture(animationUnit, r.animationData)
	}
	shadows := env.shadows && len(built) > 0
	var sunViews cascades
	if shadows {
		sunViews = cascadesFor(pass.camera, env.sunDirection, env.shadowDistance)
		r.drawShadows(&sunViews, built)
	}

	// The HDRI, when there is one, lights the pass in place of Sky and
	// Ground.
	var environment device.Texture3D
	if env.hdri != nil {
		texture, err := env.hdri.upload()
		if err != nil {
			reportError(err)
		}
		environment = texture
	}
	hasEnvironment := environment.ID != 0
	if hasEnvironment {
		device.BindTexture3D(environmentUnit, environment)
	}
	if shadows {
		device.BindShadowMap(shadowUnit, r.shadowMap)
	}
	// A level's light probes light what has no lightmap, and its
	// reflection probes are reflected inside their boxes.
	var probeGrid, levelFromWorld [16]float32
	hasProbes := pass.probes != nil && pass.probes.hasGrid()
	if hasProbes {
		device.BindDataTexture(probesUnit, pass.probes.upload())
		probeGrid = [16]float32(pass.probes.fromWorld(pass.probesPlace))
	}
	var reflections []reflectionProbe
	var reflectionCenters, reflectionLows, reflectionHighs []float32
	if pass.probes != nil && len(pass.probes.reflections) > 0 {
		levelFromWorld = [16]float32(pass.probesPlace.inverse())
		reflections = nearestReflections(pass.probes.reflections, pass.probesPlace.inverse().transformPoint(eye))
		for k, r := range reflections {
			texture, err := r.hdri.upload()
			if err != nil {
				reportError(err)
			}
			device.BindTexture3D(reflectionUnit+k, texture)
			reflectionCenters = append(reflectionCenters, r.position.X, r.position.Y, r.position.Z, float32(len(r.hdri.levels)-1))
			reflectionLows = append(reflectionLows, r.lo.X, r.lo.Y, r.lo.Z, 0)
			reflectionHighs = append(reflectionHighs, r.hi.X, r.hi.Y, r.hi.Z, 0)
		}
	}
	var irradiance []float32
	if hasEnvironment {
		for _, c := range env.hdri.irradiance {
			irradiance = append(irradiance, c[:]...)
		}
	}
	// What every lit program reads the same this pass, given to each the
	// first time the pass uses it.
	framed := map[*litProgram]bool{}
	frame := func(lit *litProgram) {
		device.UseProgram(lit.program)
		if framed[lit] {
			return
		}
		framed[lit] = true
		device.SetUniformMatrix(lit.viewProjection, &viewProjection)
		device.SetUniform(lit.eye, 3, vec3(eye))
		device.SetUniform(lit.time, 1, [4]float32{pass.time})
		device.SetUniform(lit.sunDirection, 3, vec3(env.sunDirection))
		device.SetUniform(lit.sunColor, 3, env.sun)
		device.SetUniform(lit.skyColor, 3, env.sky)
		device.SetUniform(lit.ground, 3, env.ground)
		device.SetUniform(lit.fog, 4, env.fog)
		device.SetUniformInt(lit.lightCount, lights.count)
		if lights.count > 0 {
			device.SetUniformVec4s(lit.lightPositions, lights.positions)
			device.SetUniformVec4s(lit.lightColors, lights.colors)
			device.SetUniformVec4s(lit.lightDirection, lights.directions)
		}
		for unit, at := range []int32{lit.baseTexture, lit.emissiveTexture, lit.metalRoughTexture, lit.normalTexture, lit.occlusionTexture} {
			device.SetUniformInt(at, int32(unit))
		}
		// The shadow and environment samplers keep units of their own
		// even when unused: two kinds of sampler on one unit stop the draw.
		device.SetUniformInt(lit.environmentMap, environmentUnit)
		device.SetUniformInt(lit.hasEnvironment, boolInt(hasEnvironment))
		if hasEnvironment {
			device.SetUniform(lit.environmentLevels, 1, [4]float32{float32(len(env.hdri.levels) - 1)})
			device.SetUniformVec4s(lit.irradiance, irradiance)
			device.SetUniform(lit.environmentTurn, 2, env.hdriTurn)
		}
		device.SetUniformInt(lit.shadowMap, shadowUnit)
		device.SetUniformInt(lit.lightmap, lightmapUnit)
		device.SetUniformInt(lit.probes, probesUnit)
		for k := range maxReflections {
			device.SetUniformInt(lit.reflections[k], int32(reflectionUnit+k))
		}
		device.SetUniformInt(lit.reflectionCount, int32(len(reflections)))
		if len(reflections) > 0 {
			device.SetUniformMatrix(lit.levelFromWorld, &levelFromWorld)
			device.SetUniformVec4s(lit.reflectionCenters, reflectionCenters)
			device.SetUniformVec4s(lit.reflectionLows, reflectionLows)
			device.SetUniformVec4s(lit.reflectionHighs, reflectionHighs)
		}
		device.SetUniformInt(lit.hasProbes, boolInt(hasProbes))
		if hasProbes {
			device.SetUniformMatrix(lit.probeGrid, &probeGrid)
			size := pass.probes.size
			device.SetUniform(lit.probeSize, 3, [4]float32{float32(size[0]), float32(size[1]), float32(size[2])})
		}
		device.SetUniformInt(lit.hasShadows, boolInt(shadows))
		if shadows {
			for k := range shadowCascades {
				m := [16]float32(sunViews.viewProjections[k])
				device.SetUniformMatrix(lit.shadowMatrices[k], &m)
			}
			device.SetUniform(lit.shadowSplits, 4, sunViews.splits)
			texels := sunViews.texels
			texels[3] = shadowResolution
			device.SetUniform(lit.shadowTexels, 4, texels)
			device.SetUniform(lit.viewForward, 3, vec3(pass.camera.Forward()))
		}
	}
	var current *material3D
	var currentBent, currentLightmapped bool
	var program *litProgram
	var boundLightmap *texture3D
	culling := true
	use := func(m *material3D, bent bool, lightmap *texture3D) {
		if lightmap != nil && lightmap != boundLightmap {
			if gpu := lightmap.upload(); gpu.ID != 0 {
				device.BindTexture3D(lightmapUnit, gpu)
				boundLightmap = lightmap
			} else {
				lightmap = nil
			}
		}
		lightmapped := lightmap != nil
		if m == current && bent == currentBent && lightmapped == currentLightmapped {
			return
		}
		current, currentBent, currentLightmapped = m, bent, lightmapped
		if p := r.surfaceProgram(m, bent, lightmapped); p != program {
			program = p
			frame(p)
		}
		lit := program
		device.SetUniform(lit.baseColor, 4, m.baseColor)
		device.SetUniform(lit.emissive, 3, [4]float32{m.emissive[0], m.emissive[1], m.emissive[2]})
		device.SetUniform(lit.metallic, 1, [4]float32{m.metallic})
		device.SetUniform(lit.roughness, 1, [4]float32{m.roughness})
		device.SetUniform(lit.normalScale, 1, [4]float32{m.normalScale})
		device.SetUniform(lit.occlusionStrength, 1, [4]float32{m.occlusionStrength})
		device.SetUniformInt(lit.alphaMode, int32(m.alpha))
		device.SetUniform(lit.alphaCutoff, 1, [4]float32{m.alphaCutoff})
		device.SetUniformInt(lit.unlit, boolInt(m.unlit))
		tiling := m.tiling
		if tiling == [2]float32{} {
			tiling = [2]float32{1, 1}
		}
		device.SetUniform(lit.tiling, 2, [4]float32{tiling[0], tiling[1]})
		bind := func(unit int, texture *texture3D, has int32) {
			if gpu := texture.upload(); gpu.ID != 0 {
				device.BindTexture3D(unit, gpu)
				device.SetUniformInt(has, 1)
			} else {
				device.SetUniformInt(has, 0)
			}
		}
		bind(0, m.baseTexture, lit.hasBaseTexture)
		bind(1, m.emissiveTexture, lit.hasEmissive)
		bind(2, m.metalRoughTexture, lit.hasMetalRoughTexture)
		bind(3, m.normalTexture, lit.hasNormalTexture)
		bind(4, m.occlusionTexture, lit.hasOcclusionTexture)
		if m.twoSided == culling {
			culling = !m.twoSided
			device.SetCulling(culling)
		}
	}
	instance := func(item *item3D) {
		r.instances = append(r.instances, item.model[:]...)
		tint := colorFloats(item.tint)
		r.instances = append(r.instances, tint[:]...)
	}
	for _, g := range built {
		bent := g.animation.stride > 0
		use(g.material, bent, g.lightmap)
		if bent {
			program.bending.set(g.animation)
		}
		device.DrawMeshInstanced(g.gpu, g.instances)
	}

	if env.background {
		// Behind everything: drawn at the far plane, only where nothing is.
		if !culling {
			device.SetCulling(true)
			culling = true
		}
		sky := r.sky
		inverse := [16]float32(viewProjectionMatrix.inverse())
		device.UseProgram(sky.program)
		device.SetUniformMatrix(sky.inverseViewProjection, &inverse)
		device.SetUniform(sky.eye, 3, vec3(eye))
		device.SetUniform(sky.skyTop, 3, env.skyTop)
		device.SetUniform(sky.horizon, 3, env.horizon)
		device.SetUniform(sky.skyBottom, 3, env.skyBottom)
		device.SetUniform(sky.fog, 4, env.fog)
		device.SetUniformInt(sky.environmentMap, environmentUnit)
		device.SetUniformInt(sky.hasEnvironment, boolInt(hasEnvironment))
		device.SetUniform(sky.environmentTurn, 2, env.hdriTurn)
		device.SetCulling(false)
		device.DrawFullscreen()
		device.SetCulling(true)
	}

	if len(wires) > 0 || len(pass.lines) > 0 {
		lines := r.lines
		device.UseProgram(lines.program)
		device.SetUniformMatrix(lines.viewProjection, &viewProjection)
		for _, i := range wires {
			item := &pass.items[i]
			model := [16]float32(item.model)
			device.SetUniformMatrix(lines.model, &model)
			device.SetUniform(lines.color, 4, colorFloats(item.tint))
			device.DrawMesh(item.wire.uploadWire())
		}
		if len(pass.lines) > 0 {
			if r.lineMesh.ID == 0 {
				r.lineMesh = device.NewMesh(device.LayoutColored, pass.lines, pass.lineIndices, true)
			} else {
				device.UpdateMesh(&r.lineMesh, pass.lines, pass.lineIndices)
			}
			identity := [16]float32(identityMatrix())
			device.SetUniformMatrix(lines.model, &identity)
			device.SetUniform(lines.color, 4, [4]float32{1, 1, 1, 1})
			device.DrawMesh(r.lineMesh)
		}
	}

	if len(seeThrough) > 0 {
		current, program = nil, nil
		device.SetDepthWrite(false)
		device.SetBlend3D(true)
		for k, i := range seeThrough {
			item := &pass.items[i]
			material := item.material
			if material.alpha != alphaBlend {
				// A solid material drawn see-through by its tint.
				see := *material
				see.alpha = alphaBlend
				material = &see
			}
			bending := seeThroughBending[k]
			use(material, bending.stride > 0, item.lightmap)
			if bending.stride > 0 {
				program.bending.set(bending)
			}
			r.instances = r.instances[:0]
			instance(item)
			if item.material.alpha != alphaBlend {
				// A solid model fading out: its depth first, so that only
				// its front shows, not the faces inside it.
				device.SetColorWrite(false)
				device.SetDepthWrite(true)
				device.DrawMeshInstanced(item.gpu, r.instances)
				device.SetDepthWrite(false)
				device.SetColorWrite(true)
			}
			device.DrawMeshInstanced(item.gpu, r.instances)
		}
		device.SetDepthWrite(true)
		device.SetBlend3D(false)
	}

	width, height := pass.width, pass.height
	if env.bloom > 0 {
		r.glow(width, height)
	}
	compose := r.compose
	device.UseProgram(compose.program)
	device.SetUniformInt(compose.picture, 0)
	device.SetUniformInt(compose.glow, 1)
	device.SetUniform(compose.glowStrength, 1, [4]float32{env.bloom})
	if env.bloom > 0 {
		device.BindWorkTarget(1, r.bloom.levels[0])
	}
	device.SetUniform(compose.exposure, 1, [4]float32{env.exposure})
	device.SetUniform(compose.texel, 2, [4]float32{1 / float32(width), 1 / float32(height)})
	device.SetUniformInt(compose.smoothEdges, boolInt(smoothEdges3D))
	device.End3D(compose.program)
}

// nearestReflections returns the reflection probes a pass uses, the nearest
// to eye, in the level's space, keeping the smallest box first.
func nearestReflections(all []reflectionProbe, eye Vector3) []reflectionProbe {
	if len(all) <= maxReflections {
		return all
	}
	order := make([]int, len(all))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Compare(all[a].position.Distance(eye), all[b].position.Distance(eye))
	})
	order = order[:maxReflections]
	slices.Sort(order)
	chosen := make([]reflectionProbe, len(order))
	for k, i := range order {
		chosen[k] = all[i]
	}
	return chosen
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// largestScale returns how much m stretches lengths at most, along any of
// its axes: what a bounding sphere's radius grows by.
func (m matrix) largestScale() float32 {
	x := Vector3{X: m[0], Y: m[1], Z: m[2]}.Length()
	y := Vector3{X: m[4], Y: m[5], Z: m[6]}.Length()
	z := Vector3{X: m[8], Y: m[9], Z: m[10]}.Length()
	return float32(math.Max(float64(x), math.Max(float64(y), float64(z))))
}

// upload puts the texture on the graphics card, decoding it first if it
// hasn't been, the first time something with it is drawn. A nil texture, or
// one that didn't decode, gives none.
func (t *texture3D) upload() device.Texture3D {
	if t == nil || t.err != nil {
		return device.Texture3D{}
	}
	if t.gpu.ID == 0 && t.hdr != nil {
		t.gpu = device.NewHDRTexture(t.hdrWidth, t.hdrHeight, [][]uint16{t.hdr}, false)
		loadedTextures.add(t)
	}
	if t.gpu.ID == 0 {
		if t.decoded.Width == 0 {
			if t.decoded, t.err = device.DecodeImage(t.data); t.err != nil {
				return device.Texture3D{}
			}
		}
		t.gpu = device.NewTexture3D(t.decoded, t.options)
		t.decoded = device.DecodedImage{}
		loadedTextures.add(t)
	}
	return t.gpu
}

// unload frees the texture on the graphics card; the file stays, to decode
// it again if a game runs again.
func (t *texture3D) unload() {
	device.UnloadTexture3D(t.gpu)
	t.gpu = device.Texture3D{}
}

// loadedTextures are the 3D textures on the graphics card, for Run to free
// when it ends.
var loadedTextures textureList

type textureList struct {
	textures []*texture3D
}

func (l *textureList) add(t *texture3D) { l.textures = append(l.textures, t) }

func (l *textureList) unloadAll() {
	for _, t := range l.textures {
		t.unload()
	}
	l.textures = nil
}
