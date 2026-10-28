#version 330

// GoLib adds highp float precision when compiling this shader for WebGL 2.
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
uniform vec2 screenSize;
uniform vec2 frameSizeRCP; // 1 / screenSize, supplied once by the game
uniform float time;
out vec4 finalColor;

// Edit these values and press F5 in a desktop development build.
// KERNEL_SIZE is the half-width: 3 means a 7 x 7 square, or 49 samples.
#define KERNEL_SIZE 4
const int kernel_size = KERNEL_SIZE;
const float samplePosMult = 0.53;
const float bloomStrength = 0.5;
const float CHROMATIC_ABERRATION_PIXELS = 2.5;

vec4 sceneAt(vec2 uv) {
    vec2 halfTexel = frameSizeRCP * 0.5;
    return texture(texture0, clamp(uv, halfTexel, vec2(1.0) - halfTexel));
}

vec3 boxBloom(vec2 uv) {
    vec4 sum = vec4(0.0);
    const int size = 2 * kernel_size + 1;
    const float totalSamples = float(size * size);

    // Uniform square average, following the supplied blur kernel. Sample the
    // source colors directly: no highlight threshold or Kawase iterations.
    for (int y = -kernel_size; y <= kernel_size; ++y) {
        for (int x = -kernel_size; x <= kernel_size; ++x) {
            vec2 offset = vec2(float(x), float(y)) * samplePosMult * frameSizeRCP;
            sum += sceneAt(uv + offset);
        }
    }
    return (sum / totalSamples).rgb * bloomStrength;
}

void main() {
    vec2 uv = fragTexCoord;
    vec2 fromCenter = (uv - 0.5) * screenSize;
    float distancePixels = length(fromCenter);
    float edge = clamp(distancePixels / (0.5 * length(screenSize)), 0.0, 1.0);
    vec2 direction = fromCenter / max(distancePixels, 1.0);
    vec2 shift = direction * (CHROMATIC_ABERRATION_PIXELS * edge * edge) * frameSizeRCP;
    vec3 color = vec3(sceneAt(uv + shift).r,
                      sceneAt(uv).g,
                      sceneAt(uv - shift).b);

    // Composite the supplied blur as glow over the sharp scene and HUD.
    // To inspect the normalized blur alone, use: finalColor = vec4(boxBloom(uv), 1.0).
    vec3 glow = boxBloom(uv);
    float vignette = 1.0 - 0.20 * dot((uv - 0.5) * 1.3, (uv - 0.5) * 1.3);
    float grain = fract(sin(dot(uv * screenSize, vec2(12.9898, 78.233)) +
                            floor(time * 24.0)) * 43758.5453);
    color = (color + glow) * vignette + (grain - 0.5) * 0.006;
    finalColor = vec4(color, 1.0);
}
