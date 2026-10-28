// niebla is a game made with GoLib. DESIGN.md says what it is.
//
// Read it in this order:
//
//   - main.go (this file) starts the game: main works out who is
//     playing, then calls golib.Run with the menu scene. The screen's
//     size, the game's colors and the monitor filters are here.
//   - menu.go is the title screen: the player's number, and Play, which
//     carries them to the base as they left it.
//   - identity.go says who is playing: the machine's ID, hashed with the
//     game's salt into the number the menu shows — the identity a later
//     server hands tokens out by.
//   - store.go is the local database (SQLite, the schema a later server
//     keeps): players, saves, and the glue saveBase and resumeState.
//   - play.go is the play scene: Update turns input into actions and
//     sends one Tick per update, Draw draws the region through the
//     camera, and the camera, selection and roster live here, never
//     serialized.
//   - state.go holds the simulation's state, the one serializable value
//     the whole game is, and newGame, which deals the starting region.
//   - actions.go holds the actions (Tick, SendRobot, AssignRobot,
//     RecallRobot, MarkBuilding, QueueRobot) and Apply, the only door
//     into the state.
//   - sim_robots.go holds builders' and workers' rules and their tuning:
//     what each role claims from the robot day.
//   - sim_buildings.go holds the buildings' rules and their tuning:
//     blueprints, placement and safe zones, storage caps, refueling,
//     builders' and workers' factory production.
//   - sim_pipes.go holds the pumps and the pipes: the curve through the
//     player's clicks, its price by the section, the robots laying it
//     and the oil it carries; pipes.go draws them and lays them.
//   - sim_fog.go holds the fog's law: cycles, swells, exposure and oil
//     pools covered outside the colony's bubbles; sim_robots.go applies
//     stationary wear and sim_pipes.go stops covered pools' pumps.
//   - region.go holds the region's measures, the ground of the seed in
//     hand and the isometric projection, and worldgen.go generates that
//     ground from a seed - relief by wave function collapse, cover,
//     deposits -, both as plain Go with no drawing; ground.go paints it.
//   - things.go holds what a tile holds: the things' snapshot out of the
//     state and the SI units, as plain Go with no drawing.
//   - catalog.go is the entity database: per thing type, its name, its
//     color and its card's lines, with a stable-color fallback for types
//     it has no entry for yet.
//   - markup.go writes text in colors: the "[name]...[/]" markup.
//   - inspect.go lays out and paints the tile inspection panel, with the
//     cards' buttons.
//   - robots_panel.go lays out the colony roster and individual orders.
//   - draw.go paints the region and the robots.
//   - world_test.go drives the simulation directly, no window needed.
//
// examples/platformer is a complete example, with a title, pause and win scenes,
// and a world larger than the screen, seen through a golib.Camera.
package main

import (
	_ "embed"
	"log"

	"golib"
)

const retroMinWindowWidth = 1025

var (
	screenWidth  = 1280
	screenHeight = 720
	activeResize func(width, height int)
)

func resizeScreen(width, height int) {
	screenWidth, screenHeight = width, height
	if activeResize != nil {
		activeResize(width, height)
	}
}

// The game's colors, in one place so the look is easy to change: cold ground,
// amber oil, lilac veins, pale fog.
var (
	fogColor         = golib.Color{R: 186, G: 192, B: 200, A: 255}
	fogBandColor     = golib.Color{R: 225, G: 228, B: 234, A: 255}
	groundColor      = golib.Color{R: 74, G: 84, B: 98, A: 255}
	groundShadeColor = golib.Color{R: 66, G: 76, B: 90, A: 255}
	oilColor         = golib.Color{R: 224, G: 156, B: 58, A: 255}
	oilDarkColor     = golib.Color{R: 166, G: 112, B: 42, A: 255}
	lilacColor       = golib.Color{R: 186, G: 148, B: 255, A: 255}
	lilacDarkColor   = golib.Color{R: 138, G: 102, B: 208, A: 255}
	lilacLightColor  = golib.Color{R: 214, G: 188, B: 255, A: 255}
	rockColor        = golib.Color{R: 120, G: 126, B: 138, A: 255}
	rockLightColor   = golib.Color{R: 144, G: 150, B: 162, A: 255}
	bushColor        = golib.Color{R: 92, G: 106, B: 92, A: 255}
	bushLightColor   = golib.Color{R: 114, G: 130, B: 110, A: 255}
	scarColor        = golib.Color{R: 54, G: 60, B: 72, A: 255}
	robotColor       = golib.Color{R: 208, G: 216, B: 228, A: 255}
	robotDarkColor   = golib.Color{R: 58, G: 66, B: 80, A: 255}
	unitShadowTint   = golib.Color{R: 40, G: 46, B: 58, A: 110}
	coreColor        = golib.Color{R: 32, G: 36, B: 46, A: 255}
	coreFaceColor    = golib.Color{R: 60, G: 66, B: 84, A: 255}
	coreShadeColor   = golib.Color{R: 14, G: 16, B: 22, A: 255}
	coreGlowColor    = golib.Color{R: 255, G: 244, B: 214, A: 255}
	bubbleEdgeColor  = golib.Color{R: 190, G: 226, B: 255, A: 130}
	textColor        = golib.Color{R: 40, G: 44, B: 54, A: 255}

	// The buildings. Each kind has a light face for the sun side and a
	// dark one for the shade, in the colony's cold palette; the
	// protector keeps the bubble's blue.
	factoryColor   = golib.Color{R: 92, G: 188, B: 174, A: 255}
	factoryDark    = golib.Color{R: 52, G: 118, B: 110, A: 255}
	chargerColor   = golib.Color{R: 240, G: 202, B: 96, A: 255}
	chargerDark    = golib.Color{R: 150, G: 120, B: 44, A: 255}
	siloColor      = golib.Color{R: 204, G: 164, B: 100, A: 255}
	siloDark       = golib.Color{R: 128, G: 100, B: 56, A: 255}
	warehouseColor = golib.Color{R: 168, G: 156, B: 208, A: 255}
	warehouseDark  = golib.Color{R: 104, G: 96, B: 140, A: 255}

	protectorColor     = golib.Color{R: 150, G: 202, B: 246, A: 255}
	protectorDark      = golib.Color{R: 80, G: 120, B: 168, A: 255}
	protectorEdgeColor = golib.Color{R: 168, G: 216, B: 255, A: 90}

	// The pump wears the oil's trade in a darker rust, and its pipes a
	// cold steel inside a dark casing, so they read over the dark ground
	// and over the pale fog alike, and the oil reads inside them.
	pumpColor       = golib.Color{R: 214, G: 122, B: 72, A: 255}
	pumpDark        = golib.Color{R: 132, G: 70, B: 40, A: 255}
	pipeColor       = golib.Color{R: 124, G: 134, B: 152, A: 255}
	pipeDarkColor   = golib.Color{R: 36, G: 40, B: 52, A: 255}
	pipeShadowColor = golib.Color{R: 18, G: 22, B: 32, A: 110}

	// The bar a store wears to say how full it is: an empty well, dark,
	// inside a darker edge, so the oil and the lilac read over any body.
	fillBarColor     = golib.Color{R: 30, G: 34, B: 44, A: 255}
	fillBarEdgeColor = golib.Color{R: 12, G: 14, B: 20, A: 255}

	// The inspection panel: a dark plate with light text, so the cards'
	// colors read over any ground. The picked tile keeps the core's warm
	// white.
	panelColor       = golib.Color{R: 24, G: 27, B: 35, A: 235}
	panelEdgeColor   = golib.Color{R: 190, G: 226, B: 255, A: 70}
	panelTextColor   = golib.Color{R: 232, G: 236, B: 244, A: 255}
	panelDimColor    = golib.Color{R: 148, G: 156, B: 172, A: 255}
	pickedTileColor  = golib.Color{R: 255, G: 244, B: 214, A: 255}
	hoveredTileColor = golib.Color{R: 255, G: 255, B: 255, A: 90}
	techGhostColor   = golib.Color{R: 202, G: 208, B: 218, A: 255}
	buttonColor      = golib.Color{R: 34, G: 39, B: 50, A: 255}
	buttonEdgeColor  = golib.Color{R: 190, G: 226, B: 255, A: 120}
	buttonHoverColor = golib.Color{R: 52, G: 60, B: 76, A: 255}
	dangerColor      = golib.Color{R: 238, G: 96, B: 84, A: 255}
	blockedColor     = golib.Color{R: 78, G: 84, B: 98, A: 255}

	// The rivals wear rust under a red lamp, their repulsor's ring is
	// the colony's turned warm, and their scouts spray in a pink nothing
	// else in the region has. The guard post is field green, and its shot
	// bright enough for the monitor's glow.
	enemyColor      = golib.Color{R: 196, G: 84, B: 66, A: 255}
	enemyDark       = golib.Color{R: 104, G: 40, B: 36, A: 255}
	enemyLampColor  = golib.Color{R: 255, G: 120, B: 96, A: 255}
	enemyEdgeColor  = golib.Color{R: 255, G: 150, B: 120, A: 120}
	markColor       = golib.Color{R: 255, G: 70, B: 170, A: 255}
	guardColor      = golib.Color{R: 156, G: 176, B: 122, A: 255}
	guardDark       = golib.Color{R: 86, G: 102, B: 66, A: 255}
	shotColor       = golib.Color{R: 255, G: 244, B: 210, A: 255}
	warFactoryColor = golib.Color{R: 132, G: 150, B: 104, A: 255}
	warFactoryDark  = golib.Color{R: 70, G: 84, B: 54, A: 255}

	// A site reads in the scaffold's pale steel, and loose items in the
	// crates' worn wood.
	siteColor     = golib.Color{R: 176, G: 190, B: 208, A: 255}
	pileColor     = golib.Color{R: 196, G: 170, B: 128, A: 255}
	pileDarkColor = golib.Color{R: 124, G: 104, B: 74, A: 255}
)

// The monitor filters, run over the whole picture after every Draw, in this
// order: a glow, a whisper of a tube screen and a soft rounding of the
// pixels. Both the glow and the CRT are adapted from examples/asteroids, turned
// down. Every scene runs under them; the player switches all three with F2.
// They are made in main, once: a shader needs the window's GPU, so they can't
// be package variables, and the scenes share the one set.
var (
	//go:embed shaders/glow.fs
	glowSource string

	//go:embed shaders/crt.fs
	crtSource string

	//go:embed shaders/soft.fs
	softSource string
)

var monitor struct {
	glow, crt, soft *golib.Shader
	on              bool
}

// setFilters turns the monitor filters on and off.
func setFilters(on bool) {
	monitor.on = on
	if on {
		golib.SetPostProcess(monitor.glow, monitor.crt, monitor.soft)
	} else {
		golib.SetPostProcess()
	}
}

// The screen effect settings, sent to the shaders as uniforms. Softer than
// examples/asteroids' 2.4.
const glowStrength = 0.9 // how bright the halo around the core and bubble is

func main() {
	monitor.glow = golib.NewShader(glowSource)
	monitor.crt = golib.NewShader(crtSource)
	monitor.soft = golib.NewShader(softSource)
	monitor.glow.SetUniform("strength", glowStrength)
	monitor.soft.SetUniform("amount", 0.35)

	resolvePlayer()
	defer func() {
		if db != nil {
			db.close()
		}
	}()

	config := golib.Config{
		Title: "niebla", Width: screenWidth, Height: screenHeight,
		PixelArt: true, WindowScale: 2,
		WindowScaleMinWidth: retroMinWindowWidth,
		OnScreenResize:      resizeScreen,
		// The game waits while the player is in another program. Take it out
		// for a game that should keep playing in the background.
		PauseUnfocused: true,
	}
	if err := golib.Run(newMenuScene(), config); err != nil {
		log.Fatal(err)
	}
}
