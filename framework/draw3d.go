package golib

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

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
	twoSided          bool // its back faces are drawn too
	unlit             bool // its color shows as it is, without the sun
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
}

// pass3D is what a Screen gathers between Begin3D and End3D.
type pass3D struct {
	open        bool
	camera      *Camera3D
	items       []item3D
	lines       []float32 // device.LayoutColored
	lineIndices []uint32
	environment Environment
	lights      []Light
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
		lights:      s.pass.lights[:0],
	}
}

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

void main()
{
    mat3 turn = mat3(instanceModel);
    normal = transpose(inverse(turn)) * vertexNormal;
    texcoord = vertexTexcoord;
    color = vertexColor * vec4(pow(instanceTint.rgb, vec3(2.2)), instanceTint.a);
    vec4 world = instanceModel * vec4(vertexPosition, 1.0);
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

uniform vec3 sunDirection;
uniform vec3 sunColor;
uniform vec3 skyColor;
uniform vec3 groundColor;
uniform vec4 fog;
uniform sampler2D environmentMap;
uniform int hasEnvironment;
uniform float environmentLevels;
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

// Where a direction is on an equirectangular panorama.
vec2 panorama(vec3 d)
{
    return vec2(0.5 - atan(d.x, d.z) / 6.2831853, acos(clamp(d.y, -1.0, 1.0)) / 3.14159265);
}

// The soft light a surface facing n gets from the HDRI, from its spherical
// harmonics.
vec3 surrounding(vec3 n)
{
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

void main()
{
    vec4 albedo = baseColor * color;
    if (hasBaseTexture == 1) albedo *= texture(baseTexture, texcoord);
    if (alphaMode == 1 && albedo.a < alphaCutoff) discard;
    float alpha = alphaMode == 2 ? albedo.a : 1.0;

    vec3 glow = emissive;
    if (hasEmissiveTexture == 1) glow *= texture(emissiveTexture, texcoord).rgb;
    if (unlit == 1) {
        fragColor = vec4((albedo.rgb + glow) * alpha, alpha);
        return;
    }

    float metal = metallic;
    float rough = roughness;
    if (hasMetalRoughTexture == 1) {
        vec4 mr = texture(metalRoughTexture, texcoord);
        rough *= mr.g;
        metal *= mr.b;
    }
    rough = clamp(rough, 0.04, 1.0);
    metal = clamp(metal, 0.0, 1.0);

    vec3 n = normalize(normal);
    if (!gl_FrontFacing) n = -n;
    if (hasNormalTexture == 1) {
        vec3 bump = texture(normalTexture, texcoord).xyz * 2.0 - 1.0;
        bump.xy *= normalScale;
        n = normalize(surfaceFrame(n, worldPosition, texcoord) * bump);
    }
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
    float occlusion = 1.0;
    if (hasOcclusionTexture == 1) occlusion = mix(1.0, texture(occlusionTexture, texcoord).r, occlusionStrength);
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
out vec4 fragColor;

void main()
{
    vec4 far = inverseViewProjection * vec4(texcoord * 2.0 - 1.0, 1.0, 1.0);
    vec3 direction = normalize(far.xyz / far.w - eye);
    vec3 color = direction.y > 0.0
        ? mix(horizon, skyTop, pow(direction.y, 0.6))
        : mix(horizon, skyBottom, pow(-direction.y, 0.4));
    if (hasEnvironment == 1) {
        vec2 uv = vec2(0.5 - atan(direction.x, direction.z) / 6.2831853, acos(clamp(direction.y, -1.0, 1.0)) / 3.14159265);
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
uniform float exposure;
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

void main()
{
    vec4 c = texture(picture, texcoord);
    if (c.a <= 0.0) {
        fragColor = vec4(0.0);
        return;
    }
    vec3 color = neutral(max(c.rgb / c.a * exposure, vec3(0.0)));
    fragColor = vec4(pow(color, vec3(1.0 / 2.2)) * c.a, c.a);
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
	sunDirection, sunColor, skyColor, ground    int32
	fog, lightCount                             int32
	lightPositions, lightColors, lightDirection int32
	shadowMap, hasShadows                       int32
	environmentMap, hasEnvironment              int32
	environmentLevels, irradiance               int32
	shadowMatrices                              [shadowCascades]int32
	shadowSplits, shadowTexels, viewForward     int32
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
}

// composeProgram puts a pass over the 2D.
type composeProgram struct {
	program           device.Program
	picture, exposure int32
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
	lineMesh  device.Mesh
	instances []float32 // the instances of the draw being built
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
	lit, err := build("lit", litVertexShader, litFragmentShader)
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
	at := func(p device.Program, name string) int32 { return device.UniformLocation(p, name) }
	r.lit = litProgram{
		program: lit, viewProjection: at(lit, "viewProjection"), eye: at(lit, "eye"),
		baseColor: at(lit, "baseColor"), baseTexture: at(lit, "baseTexture"), hasBaseTexture: at(lit, "hasBaseTexture"),
		emissive: at(lit, "emissive"), emissiveTexture: at(lit, "emissiveTexture"), hasEmissive: at(lit, "hasEmissiveTexture"),
		metallic: at(lit, "metallic"), roughness: at(lit, "roughness"),
		metalRoughTexture: at(lit, "metalRoughTexture"), hasMetalRoughTexture: at(lit, "hasMetalRoughTexture"),
		normalTexture: at(lit, "normalTexture"), hasNormalTexture: at(lit, "hasNormalTexture"), normalScale: at(lit, "normalScale"),
		occlusionTexture: at(lit, "occlusionTexture"), hasOcclusionTexture: at(lit, "hasOcclusionTexture"),
		occlusionStrength: at(lit, "occlusionStrength"),
		alphaMode:         at(lit, "alphaMode"), alphaCutoff: at(lit, "alphaCutoff"), unlit: at(lit, "unlit"),
		sunDirection: at(lit, "sunDirection"), sunColor: at(lit, "sunColor"), skyColor: at(lit, "skyColor"), ground: at(lit, "groundColor"),
		fog: at(lit, "fog"), lightCount: at(lit, "lightCount"),
		lightPositions: at(lit, "lightPositions"), lightColors: at(lit, "lightColors"), lightDirection: at(lit, "lightDirections"),
	}
	r.lines = lineProgram{program: lines, model: at(lines, "model"), viewProjection: at(lines, "viewProjection"), color: at(lines, "color")}
	r.sky = skyProgram{
		program: sky, inverseViewProjection: at(sky, "inverseViewProjection"), eye: at(sky, "eye"),
		skyTop: at(sky, "skyTop"), horizon: at(sky, "horizon"), skyBottom: at(sky, "skyBottom"), fog: at(sky, "fog"),
	}
	r.compose = composeProgram{program: compose, picture: at(compose, "picture"), exposure: at(compose, "exposure")}
	r.shadow = shadowProgram{
		program: shadow, lightViewProjection: at(shadow, "lightViewProjection"),
		baseTexture: at(shadow, "baseTexture"), hasBaseTexture: at(shadow, "hasBaseTexture"),
		baseOpacity: at(shadow, "baseOpacity"), cutout: at(shadow, "cutout"), alphaCutoff: at(shadow, "alphaCutoff"),
	}
	r.lit.shadowMap, r.lit.hasShadows = at(lit, "shadowMap"), at(lit, "hasShadows")
	r.lit.environmentMap, r.lit.hasEnvironment = at(lit, "environmentMap"), at(lit, "hasEnvironment")
	r.lit.environmentLevels, r.lit.irradiance = at(lit, "environmentLevels"), at(lit, "irradiance")
	r.sky.environmentMap, r.sky.hasEnvironment = at(sky, "environmentMap"), at(sky, "hasEnvironment")
	for k := range shadowCascades {
		r.lit.shadowMatrices[k] = at(lit, fmt.Sprintf("shadowMatrix%d", k))
	}
	r.lit.shadowSplits, r.lit.shadowTexels, r.lit.viewForward = at(lit, "shadowSplits"), at(lit, "shadowTexels"), at(lit, "viewForward")
	r.loaded = true
	return nil
}

// unload frees the programs and the lines' mesh. The window must still be
// open.
func (r *state3D) unload() {
	if !r.loaded {
		return
	}
	for _, p := range []device.Program{r.lit.program, r.lines.program, r.sky.program, r.compose.program, r.shadow.program} {
		device.UnloadProgram(p)
	}
	device.UnloadShadowMap(r.shadowMap)
	device.UnloadMesh(r.lineMesh)
	*r = state3D{}
}

// shadowUnit is the texture unit the sun's shadow map is bound to, past the
// five a material uses, and environmentUnit the HDRI's.
const (
	shadowUnit      = 5
	environmentUnit = 6
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
	}
	type group struct {
		gpu      device.Mesh
		material *material3D
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
			key := groupKey{item.gpu.ID, item.material}
			at, ok := byKey[key]
			if !ok {
				at = len(groups)
				byKey[key] = at
				groups = append(groups, group{gpu: item.gpu, material: item.material})
			}
			groups[at].items = append(groups[at].items, i)
		}
	}

	device.Begin3D()

	// Each group's instances, built once for the shadows and the picture.
	built := make([]shadowGroup, len(groups))
	for k, g := range groups {
		var data []float32
		for _, i := range g.items {
			item := &pass.items[i]
			tint := colorFloats(item.tint)
			data = append(data, item.model[:]...)
			data = append(data, tint[:]...)
		}
		built[k] = shadowGroup{gpu: g.gpu, material: g.material, instances: data}
	}
	shadows := env.shadows && len(built) > 0
	var sunViews cascades
	if shadows {
		sunViews = cascadesFor(pass.camera, env.sunDirection, env.shadowDistance)
		r.drawShadows(&sunViews, built)
	}

	lit := r.lit
	device.UseProgram(lit.program)
	device.SetUniformMatrix(lit.viewProjection, &viewProjection)
	device.SetUniform(lit.eye, 3, vec3(eye))
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
	device.SetUniformInt(lit.environmentMap, environmentUnit)
	device.SetUniformInt(lit.hasEnvironment, boolInt(hasEnvironment))
	if hasEnvironment {
		device.BindTexture3D(environmentUnit, environment)
		device.SetUniform(lit.environmentLevels, 1, [4]float32{float32(len(env.hdri.levels) - 1)})
		var sh []float32
		for _, c := range env.hdri.irradiance {
			sh = append(sh, c[:]...)
		}
		device.SetUniformVec4s(lit.irradiance, sh)
	}
	device.SetUniformInt(lit.hasShadows, boolInt(shadows))
	// The shadow sampler keeps a unit of its own even without shadows: two
	// kinds of sampler on one unit stop the draw.
	device.SetUniformInt(lit.shadowMap, shadowUnit)
	if shadows {
		device.BindShadowMap(shadowUnit, r.shadowMap)
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
	var current *material3D
	culling := true
	use := func(m *material3D) {
		if m == current {
			return
		}
		current = m
		device.SetUniform(lit.baseColor, 4, m.baseColor)
		device.SetUniform(lit.emissive, 3, [4]float32{m.emissive[0], m.emissive[1], m.emissive[2]})
		device.SetUniform(lit.metallic, 1, [4]float32{m.metallic})
		device.SetUniform(lit.roughness, 1, [4]float32{m.roughness})
		device.SetUniform(lit.normalScale, 1, [4]float32{m.normalScale})
		device.SetUniform(lit.occlusionStrength, 1, [4]float32{m.occlusionStrength})
		device.SetUniformInt(lit.alphaMode, int32(m.alpha))
		device.SetUniform(lit.alphaCutoff, 1, [4]float32{m.alphaCutoff})
		device.SetUniformInt(lit.unlit, boolInt(m.unlit))
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
		use(g.material)
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
		distance := func(i int) float32 { return pass.items[i].center.Distance(eye) }
		slices.SortStableFunc(seeThrough, func(a, b int) int {
			return cmp.Compare(distance(b), distance(a)) // farthest first
		})
		device.UseProgram(lit.program)
		current = nil
		device.SetDepthWrite(false)
		device.SetBlend3D(true)
		for _, i := range seeThrough {
			item := &pass.items[i]
			material := item.material
			if material.alpha != alphaBlend {
				// A solid material drawn see-through by its tint.
				see := *material
				see.alpha = alphaBlend
				material = &see
			}
			use(material)
			r.instances = r.instances[:0]
			instance(item)
			device.DrawMeshInstanced(item.gpu, r.instances)
		}
		device.SetDepthWrite(true)
		device.SetBlend3D(false)
	}

	compose := r.compose
	device.UseProgram(compose.program)
	device.SetUniformInt(compose.picture, 0)
	device.SetUniform(compose.exposure, 1, [4]float32{env.exposure})
	device.End3D(compose.program)
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
