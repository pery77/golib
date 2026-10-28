#version 330

// Soft: the monitor filter. Every screen pixel takes a little of its four
// neighbors (a small tent blur), so the hard corners of shapes and letters
// round off a little. Flat areas read the same color from every tap and stay
// exactly as they were, so patterns like the ground's checker do not move,
// and no grain is added anywhere.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels
uniform float amount;    // set by the game: 0 is the untouched picture

out vec4 finalColor;

void main()
{
    vec2 pixel = 1.0 / screenSize;
    vec3 center = texture(texture0, fragTexCoord).rgb;

    vec3 around = texture(texture0, fragTexCoord + vec2(pixel.x, 0.0)).rgb;
    around += texture(texture0, fragTexCoord - vec2(pixel.x, 0.0)).rgb;
    around += texture(texture0, fragTexCoord + vec2(0.0, pixel.y)).rgb;
    around += texture(texture0, fragTexCoord - vec2(0.0, pixel.y)).rgb;
    around *= 0.25;

    finalColor = vec4(mix(center, around, amount), 1.0);
}
