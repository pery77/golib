#version 330

// CRT: a whisper of a tube screen, adapted from examples/asteroids' crt.fs with
// the flicker removed, the bulge removed, the scanlines softened and the dark
// border much lighter, so the effect stays subtle. The colors split half a
// screen pixel apart, straight across the picture.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels
uniform vec2 outputSize; // set by GoLib: the picture on the window, in pixels

out vec4 finalColor;

void main()
{
    // Red and blue read half a screen pixel apart: a tube's tint rather
    // than a fringe a letter wears.
    vec2 shift = vec2(0.5 / screenSize.x, 0.0);
    vec3 color = vec3(
        texture(texture0, fragTexCoord + shift).r,
        texture(texture0, fragTexCoord).g,
        texture(texture0, fragTexCoord - shift).b);

    // A scanline every 3 pixels of the window, so they stay sharp at any
    // size, a breath darker than the picture: the pale fog would show any
    // deeper one as stripes.
    float scanline = sin(fragTexCoord.y * outputSize.y * 3.14159265 / 1.5) * 0.5 + 0.5;
    color *= mix(0.96, 1.0, scanline);

    // A touch darker towards the corners.
    vec2 centered = fragTexCoord - 0.5;
    color *= 1.0 - dot(centered, centered) * 0.3;

    finalColor = vec4(color, 1.0);
}
