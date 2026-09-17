#version 330

// Chromatic aberration: a lens bends each color of light a little differently,
// so the picture's colors drift apart, more the further they are from the
// middle. Pulling the red, green and blue channels apart, as most games do,
// leaves hard red and cyan edges. This walks along the drift instead and
// gathers a whole spectrum of samples, each weighted by how much of it a red,
// a green and a blue sensor would see, which leaves the soft prism fringes a
// lens gives and keeps white white. The spread also blurs the corners a
// little, as it does through glass.
//
// The middle of the screen, where the ship is, always stays sharp, and the
// game tells the lens, in strain, how close the ship is to being destroyed,
// which pulls the colors further apart.
//
// The numbers that decide how the lens looks are the constants below. A debug
// build reads this file from the assets folder every time it starts, and F5
// reads it again without leaving the game, so they can be tuned by eye.

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec2 screenSize; // set by GoLib: the game's screen, in pixels
uniform float strain;    // set by the game: 0 while the ship is safe, 1 on its last hull point

out vec4 finalColor;

// The look. Tune these.
const float drift = 10.0;       // pixels the colors drift apart in the corners
const float strainDrift = 5.0; // pixels added to that on the ship's last hull point
const float falloff = 2.4;     // how much the drift keeps to the corners; 1 spreads it evenly
const int samples = 12;        // colors of the spectrum gathered along the drift
const float band = 0.2;       // how wide a sensor's range of wavelengths is

// response is how much of a wavelength, from 0 at the red end of the spectrum
// to 1 at the blue one, a red, green and blue sensor sees. The three ranges
// overlap, so the samples add up to a neutral white.
vec3 response(float wavelength)
{
    vec3 middle = vec3(0.18, 0.5, 0.82);
    vec3 offset = (vec3(wavelength) - middle) / band;
    return exp(-offset * offset);
}

// picture reads the picture, never past its edges, so a drifting sample can't
// wrap around and drag the other side's colors in.
vec3 picture(vec2 at)
{
    vec2 edge = 0.5 / screenSize;
    return texture(texture0, clamp(at, edge, 1.0 - edge)).rgb;
}

void main()
{
    // How far this pixel is from the middle of the screen, from 0 there to 1
    // in the corners, measured in pixels so a wide screen doesn't stretch it.
    vec2 fromMiddle = (fragTexCoord - 0.5) * screenSize;
    float away = min(length(fromMiddle) / (0.5 * length(screenSize)), 1.0);

    // It grows with a power of that distance, so the middle stays sharp and the
    // corners take the whole drift.
    vec2 way = fromMiddle / max(length(fromMiddle), 0.0001);
    vec2 apart = way * ((drift + strainDrift * strain) * pow(away, falloff)) / screenSize;

    vec3 sum = vec3(0.0);
    vec3 total = vec3(0.0);
    for (int i = 0; i < samples; i++) {
        float wavelength = (float(i) + 0.5) / float(samples);
        vec3 weight = response(wavelength);
        // Red lands short of where the pixel belongs and blue past it.
        sum += picture(fragTexCoord + apart * (wavelength - 0.5) * 2.0) * weight;
        total += weight;
    }

    finalColor = vec4(sum / total, 1.0);
}
