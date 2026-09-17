#version 330

// Bloom: bright things bleed light over everything around them. Each pixel
// gathers the part of its neighbours that is brighter than a threshold and
// adds it on top, over three widths at once: a tight core that thickens
// bullets and engines, a middle halo, and a wide haze stretched sideways, the
// way light spreads across a camera lens.
//
// The samples of each width sit on a spiral, one golden angle apart and spread
// evenly over its disc, which covers the disc better than rings of samples do
// for the same count. Every pixel turns its spirals by its own angle, so the
// gaps between the samples land somewhere else for each one: without that, a
// halo bands into blocks. Everything above white is eased towards it at the
// end, so a pile of glow keeps its color instead of turning into a white blob.
//
// The numbers that decide how the glow looks are the constants below. A debug
// build reads this file from the assets folder every time it starts, and F5
// reads it again without leaving the game, so they can be tuned by eye.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels

out vec4 finalColor;

// The look. Tune these.
const float threshold = 0.6;  // brightness where the glow starts: 0 is black, 1 is white
const float knee = 0.920;       // how far below the threshold the glow fades in
const float strength = 5.2;    // how much glow is added on top
const float radius = 8.0;     // pixels the widest glow reaches
const float anamorphic = 1.8;  // how much wider than tall that widest glow is
const float shoulder = 0.75;   // brightness where the roll-off towards white starts

const float goldenAngle = 2.39996323; // radians from one sample of a spiral to the next
const float fullTurn = 6.2831853;     // radians all the way round

// picture reads the picture, never past its edges: a sample outside them would
// wrap around and drag the other side's light in.
vec3 picture(vec2 at)
{
    vec2 edge = 0.5 / screenSize;
    return texture(texture0, clamp(at, edge, 1.0 - edge)).rgb;
}

// bright keeps what a color has above the threshold, with a soft knee, so a
// shape's glow grows as it brightens instead of switching on.
vec3 bright(vec3 color)
{
    float light = max(color.r, max(color.g, color.b));
    float soft = clamp(light - threshold + knee, 0.0, 2.0 * knee);
    soft = soft * soft / (4.0 * knee + 0.0001);
    return color * max(soft, light - threshold) / max(light, 0.0001);
}

// spin is the angle this pixel turns its spirals by: interleaved gradient
// noise, which gives neighbouring pixels well spread values, so what is left
// of the gaps between the samples looks like film grain instead of blocks.
float spin()
{
    vec2 at = fragTexCoord * screenSize;
    return fract(52.9829189 * fract(dot(at, vec2(0.06711056, 0.00583715)))) * fullTurn;
}

// halo averages the bright parts of count samples over a disc of discRadius
// pixels, widened sideways by stretch. turn starts each spiral somewhere else,
// so the three widths don't sample the same directions.
vec3 halo(float discRadius, int count, float stretch, float turn)
{
    vec2 pixel = 1.0 / screenSize;
    vec3 sum = vec3(0.0);
    float total = 0.0;
    for (int i = 0; i < count; i++) {
        float part = (float(i) + 0.5) / float(count);
        float away = discRadius * sqrt(part); // evenly over the disc, not bunched in the middle
        float angle = turn + float(i) * goldenAngle;
        vec2 offset = vec2(cos(angle) * stretch, sin(angle)) * away * pixel;
        float weight = exp(-2.0 * part); // far samples count less, as in a blur
        sum += bright(picture(fragTexCoord + offset)) * weight;
        total += weight;
    }
    return sum / total;
}

// rollOff eases colors brighter than white towards it, keeping their hue,
// where the screen would otherwise cut them off flat.
vec3 rollOff(vec3 color)
{
    float peak = max(color.r, max(color.g, color.b));
    if (peak <= shoulder) {
        return color;
    }
    float eased = shoulder + (1.0 - shoulder) * (1.0 - exp((shoulder - peak) / (1.0 - shoulder)));
    return color * eased / peak;
}

void main()
{
    vec3 color = picture(fragTexCoord);

    // The three widths start a third of a turn apart, so they don't all sample
    // the same directions, and this pixel's own angle on top of that.
    float turn = spin();
    vec3 glow = halo(radius * 0.20, 12, 1.0, turn) * 0.50
              + halo(radius * 0.45, 20, 1.0, turn + 2.09) * 0.32
              + halo(radius, 28, anamorphic, turn + 4.19) * 0.22;

    finalColor = vec4(rollOff(color + glow * strength), 1.0);
}
