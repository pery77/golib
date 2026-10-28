#version 330

// Glitch: for a moment after the ship is hit, the picture breaks up like a
// screen losing its signal. Bands of it slide sideways and wrap around, the
// colors come apart, blocks of noise punch through, a bright bar rolls down
// and the whole thing jumps. Everything follows amount, which the game sets
// from the ship's damage and lets fade; at 0 the picture goes through
// untouched, so the shader can stay on all the time.
//
// Time is chopped into ticks, so the break-up jumps from one still picture to
// the next, the way broken hardware does, instead of wobbling smoothly.
//
// The numbers that decide how the break-up looks are the constants below. A
// debug build reads this file from the assets folder every time it starts, and
// F5 reads it again without leaving the game, so they can be tuned by eye.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels
uniform float time;      // set by GoLib: seconds of game time
uniform float amount;    // set by the game: 0 is off, 1 is the moment of the hit

out vec4 finalColor;

// The look. Tune these.
const float ticks = 18.0;       // still pictures a second the break-up jumps between
const float bandHeight = 22.0;  // pixels of a sliding band, at its thinnest
const float bandSlide = 0.22;   // how far across the screen a band slides, at the worst
const float blockWidth = 34.0;  // pixels of a block cut out of the picture
const float colorSplit = 0.035; // how far across the screen the colors come apart
const float barGlow = 0.35;     // how bright the bar rolling down the screen is

// noise returns the same number, from 0 to 1, for the same seed.
float noise(vec2 seed)
{
    return fract(sin(dot(seed, vec2(12.9898, 78.233))) * 43758.5453);
}

// picture reads the picture, wrapping sideways so a band that slides off one
// edge comes back in at the other, and never past the top or the bottom.
vec3 picture(vec2 at)
{
    float edge = 0.5 / screenSize.y;
    return texture(texture0, vec2(fract(at.x), clamp(at.y, edge, 1.0 - edge))).rgb;
}

void main()
{
    if (amount <= 0.0) {
        finalColor = vec4(texture(texture0, fragTexCoord).rgb, 1.0);
        return;
    }

    float tick = floor(time * ticks);
    vec2 uv = fragTexCoord;

    // The whole picture jumps, as if the screen lost its hold.
    uv.y += (noise(vec2(tick, 3.7)) - 0.5) * 0.02 * amount;

    // Bands slide sideways. Each one picks its own height and its own slide,
    // and only some of them move at all, so the picture tears instead of
    // smearing.
    float height = mix(3.0, 1.0, amount) * bandHeight;
    float band = floor(uv.y * screenSize.y / height);
    if (noise(vec2(band, tick)) < 0.12 + 0.45 * amount) {
        uv.x += (noise(vec2(band, tick + 11.0)) - 0.5) * bandSlide * amount * amount;
    }

    // The colors come apart along the tear, further than the lens ever pulls
    // them, and a different way from one tick to the next.
    vec2 split = vec2(noise(vec2(tick, 5.3)) - 0.5, noise(vec2(tick, 9.1)) - 0.5) * colorSplit * amount;
    vec3 color = vec3(
        picture(uv + split).r,
        picture(uv).g,
        picture(uv - split).b);

    // Blocks of the picture are cut out and filled with another part of it,
    // the way a decoder repeats what it still has when the rest is gone, and
    // the worst of them burn out into noise.
    vec2 block = vec2(floor(uv.x * screenSize.x / blockWidth), band);
    float picked = noise(block + tick * 0.37);
    if (picked > 1.0 - 0.14 * amount * amount) {
        vec2 stolen = vec2(noise(block + tick) - 0.5, noise(block - tick) - 0.5);
        color = picture(uv + stolen * 0.4 * amount);
        if (picked > 1.0 - 0.025 * amount * amount) {
            float grain = noise(floor(uv * screenSize) + tick);
            color = mix(color, vec3(0.7, 0.85, 1.0) * grain, 0.7 * amount);
        }
    }

    // A bright bar rolls down the screen, like a tape head crossing a picture.
    color += barGlow * vec3(1.0, 1.4, 2.0) * smoothstep(0.96, 1.0, fract(uv.y + time * 0.9)) * amount;

    // Scanlines, and the steps colors fall into when a signal runs out of room.
    float scanline = 0.5 + 0.5 * cos(uv.y * screenSize.y * 3.14159265);
    color *= 1.0 - 0.25 * amount * scanline;
    color = mix(color, floor(color * 12.0) / 12.0, 0.5 * amount);

    finalColor = vec4(color, 1.0);
}
