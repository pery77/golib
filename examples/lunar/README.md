# SELENE: Frontier

A complete 2D lunar lander and incremental logistics campaign, built using only
GoLib's public API. Fly freight, perfect your landings and build a lunar economy.

From the repository root:

```powershell
.\golib run lunar
```

Choose **Begin operations**, then **Launch contract**. Both accept Enter.
Tilt right with D and thrust toward the beacon with W. Release rotation to keep
your attitude; hold S to straighten the hull and brake sideways. Use short engine
burns to reduce descent before reaching the lit pad. All three landing indicators
must be green. Failed contracts have free recovery and retries.

- **15 contracts across five regions**, with supply, fragile research and express
  deliveries. Region access grows with completed deliveries.
- **Three ships**, with different engine power, handling, fuel, landing tolerance
  and cargo payouts. Four upgrade tracks with four levels each, per ship.
- **Five colonies**, each expandable through three levels. Their dividends stack
  and pay after every successful delivery. Establish all five to finish the
  campaign; continue afterward.
- Cinematic automatic/manual zoom, square-kernel glow, radial chromatic aberration,
  atmospheric planets, faceted terrain, engine particles, landing dust, impact
  effects and camera shake. F2 toggles post-processing for comparison.
- Original synthesized ambient music and effects. Persistent progress, settings,
  flight manual, pause, retry, debrief and campaign reset with confirmation.
- Keyboard, gamepad and touch flight controls. All menus support mouse/touch and
  keyboard/gamepad. Settings provides fullscreen, effects and audio buttons.

| Action | Keyboard | Gamepad |
| --- | --- | --- |
| Thrust | W / Up / Space | A / RT |
| Rotate | A / D / Left / Right | Left stick / d-pad |
| Stabilize and brake drift | S / Down / Left Shift | X |
| Zoom | Wheel / Q / E | LB / RB |
| Automatic zoom | C | Y |
| Pause | Esc / P | Start |
| Menus | Arrows + Enter / mouse | d-pad + A |
| Mission control tabs | 1 / 2 / 3 / Tab | LB / RB |
| Effects / mute / fullscreen | F2 / M / F11 | Settings menu |
| Reload shader (desktop development) | F5 | — |

## Builds

```powershell
.\golib web lunar
.\golib web lunar --lan
.\golib dist lunar
.\golib dist lunar --web
.\golib run lunar --dist
```

The desktop zip is in `build/lunar/dist/`. The web zip has `index.html` at its
root and can be uploaded to itch.io. Touch buttons appear when playing with
fingers. Open Settings to enter fullscreen on a phone.

Debug saves live in `build/lunar/save/`; desktop distribution saves use GoLib's
normal per-game settings folder. Browser saves stay with that browser and address.
Contracts are free to launch. There are no real-time or offline income timers:
the incremental economy is driven by successful flights.

## Verification and editing

```powershell
.\golib test lunar
.\golib go -C examples/lunar test -run TestLiveShaderReload -args -shader-reload-check
.\golib shot lunar 60
.\golib go -C examples/lunar test -run TestContractRoutes -args '-pilot-script=../../build/lunar/pilot-input.txt'
$pilot = Get-Content build/lunar/pilot-input.txt -Raw
.\golib shot lunar 1150 1650 --input $pilot
```

The route tests fly every contract with player-equivalent controls; the optional
script reproduces the first complete landing through the real input system.
Tests also cover collision limits, fuel, purchases, payouts and save/load.
The optional shader check opens a hidden graphics window, edits temporary shader
sources and presses F5. It checks the rendered pixels after successful reloads,
invalid GLSL, a missing file and a reload with effects disabled.

While running `golib run lunar`, edit and save `shaders/cinema.fs`, then press
**F5 in the game window**. The new shader takes effect without restarting the
flight. A read or compilation error keeps the previous effect and displays a
notice; compiler details appear in the console. Reload also works while paused
or with effects disabled (F2). Web and distribution builds embed the shader and
need rebuilding to pick up source edits.

The glow uses the normalized square average of the source image. In `cinema.fs`,
`KERNEL_SIZE` controls its half-width (3 means 7 × 7 samples), `samplePosMult`
controls sample spacing in game pixels and `bloomStrength` controls the added
glow. `CHROMATIC_ABERRATION_PIXELS` controls radial red/blue separation.

Open `assets/maps/*.tmx` in Tiled to edit the terrain polyline, destination pads
and insertion points. These are object maps, so no tile art is required.
Keep terrain points ordered left to right and the surface flat under each pad.
All prices and progression live in `campaign.go`; physics lives in `world.go`.

The original planet illustrations and icon can be regenerated with:

```powershell
.\golib go -C examples/lunar run ./sources/art
```

The music is an original synthesized GoLib tune. To substitute a licensed track
provided by the player, put it in `assets/` and replace `golib.NewTune(themeSpec)`
in `sounds.go` with `golib.NewMusic("your-track.ogg")`.

See [DESIGN.md](DESIGN.md) for the design and tuning notes.
