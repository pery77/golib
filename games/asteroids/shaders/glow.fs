#version 330

// Glow: adds a soft halo around bright shapes, by averaging rings of samples
// around every pixel and adding that back on top. Only the part of each sample
// above a threshold glows, so the dark background stays dark.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels
uniform float strength;  // set by the game: how bright the halo is

out vec4 finalColor;

void main()
{
    vec2 pixel = 1.0 / screenSize;
    vec3 color = texture(texture0, fragTexCoord).rgb;

    vec3 halo = vec3(0.0);
    float total = 0.0;
    for (int ring = 1; ring <= 3; ring++) {
        float radius = float(ring) * 2.5;
        float weight = 1.0 / float(ring); // inner rings count more
        for (int i = 0; i < 12; i++) {
            float angle = 6.2831853 * float(i) / 12.0;
            vec2 offset = vec2(cos(angle), sin(angle)) * radius * pixel;
            vec3 bright = max(texture(texture0, fragTexCoord + offset).rgb - vec3(0.15), vec3(0.0));
            halo += bright * weight;
            total += weight;
        }
    }
    halo /= total;

    finalColor = vec4(color + halo * strength, 1.0);
}
