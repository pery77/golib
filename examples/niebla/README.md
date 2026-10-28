# niebla — technical notes

How the code is built. The game design lives in [DESIGN.md](./DESIGN.md); this
file is for whoever works on the code, human or agent.

## Commands

From the GoLib repository root:

```text
./golib run niebla          # play it
./golib test niebla         # go vet + go test
./golib shot niebla 30      # the menu, with the player's number
./golib shot niebla 120 --input "Enter@1"   # reach the region: Play is Enter
```

In a normal window the render buffer uses native resolution when the drawing
area is at most 1024 pixels wide. Above that, it uses half the width and height
for 2x retro pixels. The whole image, including text, follows this threshold
on resizes and fullscreen changes; the menu and camera follow the buffer too.
The squad boxes sit 12 screen pixels from the top, with the robots button
6 pixels below them; the resource HUD wraps to their left.
Rival notices sit 8 pixels below the wrapped resource HUD, or below the last
dev-tool row while those controls are open, with no resolution-specific
vertical offset.
Status and notice text grows from 15 to 18 pixels below 1000 drawing pixels
wide, then to a maximum of 20 below 750. Dev tools follow the wrapped HUD too.
`golib shot` keeps its fixed 1280x720 screen, so the scripted click coordinates
below remain valid.

The economy probe plays four headless opening policies over seeds 0, 1 and
2 for an hour, sampling the real state every game minute. It respects the
schematics each policy has actually earned. The plans are safe harvesting,
worker growth, a protected oil outpost, and a guard with a two-trooper
defense. The `oil` column is spendable oil; `protector_oil` tracks dedicated
fuel separately. Party size, city progress, stores and sortie counts are
recorded too, along with the first scout's core-bubble crossing, rival
building damage, repair milestone ticks, artillery's arrival tick, mechanic
count and remaining building damage. It writes only when given a path, so
normal tests leave no report:

```text
NIEBLA_ECONOMY_REPORT=../../build/niebla/economy.csv \
./golib go -C examples/niebla test \
  -run TestWriteEconomyReport -v
```

To inspect the first real attack just as it starts, without replaying the
minutes before it:

```text
NIEBLA_FIRST_RAID_SHOT_STATE=../../build/niebla/first-raid.json \
  ./golib go -C examples/niebla test \
  -run TestWriteFirstRaidShotState
./golib shot niebla 80 \
  --save build/niebla/first-raid.json --input "Enter@1"
```

To inspect a half-powered protector and its card:

```text
NIEBLA_PROTECTOR_SHOT_STATE=../../build/niebla/protector.json \
  ./golib go -C examples/niebla test \
  -run TestWriteProtectorShotState -v
./golib shot niebla 80 --save build/niebla/protector.json \
  --input "Enter@1 Mouse@2:760,302 MouseLeft@3"
```

The test prints the protector cell's click position for the current layout.

To inspect the floating oil costs from a repairing mechanic and a working
protector, zooming in on them:

```text
NIEBLA_COST_SHOT_STATE=../../build/niebla/costs.json \
  ./golib go -C examples/niebla test -run TestWriteCostShotState
./golib shot niebla 80 100 --save build/niebla/costs.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:4"
```

To inspect the joined outline of the core and two overlapping protectors:

```text
NIEBLA_BUBBLE_SHOT_STATE=../../build/niebla/bubbles.json \
  ./golib go -C examples/niebla test -run TestWriteBubbleShotState
./golib shot niebla 60 --save build/niebla/bubbles.json \
  --input "Enter@1"
```

To inspect the offscreen guides for a rival report and pending schematics:

```text
NIEBLA_GUIDE_SHOT_STATE=../../build/niebla/guides.json \
  ./golib go -C examples/niebla test -run TestWriteGuideShotState
./golib shot niebla 240 --save build/niebla/guides.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:4 D@10-240"
```

To inspect the first scout's live offscreen guide:

```text
NIEBLA_SCOUT_GUIDE_SHOT_STATE=../../build/niebla/scout-guide.json \
  ./golib go -C examples/niebla test \
  -run TestWriteFirstScoutGuideShotState
./golib shot niebla 40 80 --save build/niebla/scout-guide.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:4"
```

To inspect the schematics callout and its squares of what a drop brings:

```text
NIEBLA_TECH_SHOT_STATE=../../build/niebla/tech.json \
  ./golib go -C examples/niebla test -run TestWriteTechShotState
./golib shot niebla 60 --save build/niebla/tech.json \
  --input "Enter@1 Mouse@2:640,312 MouseLeft@3"
```

The test prints the badge and first square coordinates, plus a valid ground
cell for placing a blueprint. `NIEBLA_TECH_PIPES_SHOT_STATE` writes the
frontier kit with its informational pipes square. To inspect the repair
protocol's informational mechanic square instead:

```text
NIEBLA_TECH_REPAIR_SHOT_STATE=../../build/niebla/repair-tech.json \
  ./golib go -C examples/niebla test -run TestWriteTechShotState
./golib shot niebla 60 --save build/niebla/repair-tech.json \
  --input "Enter@1 Mouse@2:640,312 MouseLeft@3"
```

To see the scout inside the bubble, crossing out, and the guard callout in
one sequence of shots:

```text
NIEBLA_GUARD_SHOT_STATE=../../build/niebla/guard-timing.json \
  ./golib go -C examples/niebla test \
  -run TestWriteGuardTimingShotState -v
./golib shot niebla 1 22 24 25 \
  --save build/niebla/guard-timing.json \
  --input "Enter@1 Mouse@24:640,312 MouseLeft@25"
```

To compare the third normal attack in progress with the artillery schematic
after it ends:

```text
state_dir=../../build/niebla
NIEBLA_ARTILLERY_LOCKED_SHOT_STATE=$state_dir/artillery-locked.json \
NIEBLA_ARTILLERY_UNLOCKED_SHOT_STATE=$state_dir/artillery-unlocked.json \
  ./golib go -C examples/niebla test \
  -run TestWriteArtilleryTechShotStates -v
./golib shot niebla 60 --save build/niebla/artillery-locked.json \
  --input "Enter@1"
./golib shot niebla 60 --save build/niebla/artillery-unlocked.json \
  --input "Enter@1 Mouse@2:640,312 MouseLeft@3"
```

The test prints the artillery badge position for the current layout; use that
position instead of `640,312` if it differs.

To inspect mites on a stopped builder, a rival vehicle, an empty protector,
a pile and a laid pipe, alongside a charged protector bubble:

```text
NIEBLA_MITE_SHOT_STATE=../../build/niebla/mites.json \
  ./golib go -C examples/niebla test -run TestWriteMiteShotState
./golib shot niebla 120 --save build/niebla/mites.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:1" \
  --scale 2
```

To compare the war factory card before and after the protocol, and see its
disabled orders and red resource shortfalls, write the three states and use
the printed cell click coordinate in the shots:

```text
NIEBLA_REPAIR_LOCKED_SHOT_STATE=../../build/niebla/repair-locked.json \
NIEBLA_REPAIR_UNLOCKED_SHOT_STATE=../../build/niebla/repair-unlocked.json \
NIEBLA_REPAIR_SHORT_SHOT_STATE=../../build/niebla/repair-short.json \
  ./golib go -C examples/niebla test \
  -run TestWriteRepairCardShotStates -v
./golib shot niebla 60 --save build/niebla/repair-locked.json \
  --input "Enter@1 Mouse@2:664,362 MouseLeft@3"
./golib shot niebla 60 --save build/niebla/repair-unlocked.json \
  --input "Enter@1 Mouse@2:664,362 MouseLeft@3"
./golib shot niebla 60 --save build/niebla/repair-short.json \
  --input "Enter@1 Mouse@2:664,362 MouseLeft@3"
```

To inspect the remaining repair cost on a damaged guard post:

```text
NIEBLA_REPAIR_COST_SHOT_STATE=../../build/niebla/repair-cost.json \
  ./golib go -C examples/niebla test \
  -run TestWriteRepairCostShotState -v
./golib shot niebla 60 --save build/niebla/repair-cost.json \
  --input "Enter@1 Mouse@2:664,362 MouseLeft@3"
```

To inspect a pump being eaten outside every bubble, write its state and
take shots before and after the mites finish it:

```text
NIEBLA_PUMP_SHOT_STATE=../../build/niebla/pump.json \
  ./golib go -C examples/niebla test \
  -run TestWriteExternalPumpShotState
./golib shot niebla 120 600 --save build/niebla/pump.json \
  --input "Enter@1 Mouse@2:904,361 MouseWheel@3:3"
```

A shot starts on the menu; `Enter@1` presses Play (or click it: `Mouse@5:640,390
MouseLeft@6`). Inside the region:

```text
./golib shot niebla 60 --input "Enter@1 Mouse@5:568,372 MouseLeft@6"
```

The click picks a cell of the safe oil pool at tile (11, 14), whose card
opens by itself. (568, 372) is the tile's center on screen. A new game
starts with a builder, so open the robot roster at the top right, select
that builder and choose `assign deposit` to send it to the pool. The
deposit card's `send robot` button appears once an unassigned worker
exists; it never takes a worker from another post. The camera at rest centers
the view on world point (640, 372) — the middle of its bounds — so screen
and world differ by (0, -12) at rest zoom; click targets in scripted shots
aim at a tile's center (`projectTile` + half a tile, plus that offset),
never its corner, whose tile depends on float rounding.

To inspect the colony-wide roster after starting a game:

```text
./golib shot niebla 80 \
  --input "Enter@1 Mouse@5:1194,78 MouseLeft@6"
```

The roster button sits beneath the squad boxes, at screen position
1194, 78. It opens the panel with the starting builder and its current
task. The X or the roster button closes it.

To inspect the roster's builder, free-worker, deposit and mechanic groups:

```text
NIEBLA_ROBOT_ROSTER_SHOT_STATE=../../build/niebla/roster.json \
  ./golib go -C examples/niebla test \
  -run TestWriteRobotRosterShotState
./golib shot niebla 80 --save build/niebla/roster.json \
  --input "Enter@1 Mouse@5:1194,78 MouseLeft@6"
```

To inspect two pages of portraits, write a state with ten robots assigned
to the safe lilac vein and take a shot of its card:

```text
NIEBLA_ROBOT_SHOT_STATE=../../build/niebla/robots.json \
  ./golib go -C examples/niebla test \
  -run TestWriteRobotPortraitShotState
./golib shot niebla 60 120 180 \
  --save build/niebla/robots.json \
  --input "Enter@1 Mouse@5:616,372 MouseLeft@6 \
  Mouse@61:887,570 MouseLeft@62 \
  Mouse@121:682,502 MouseLeft@122"
```

The screenshots show the first page, the second page and the selected
worker's card in order. Click a portrait to inspect that robot, including
while it is on its way to the deposit, carrying a load or refueling. Its
card offers `recall robot` for that worker alone and `back to deposit` to
return.

## Dev tools

In the region, hold Control and click the game's name twice: a strip
opens under the HUD, in two rows. The first: **hold a swell** (the fog
presses in and stays until the button lets it go), **place robots**
(every click on the region drops a free built robot there; a right click
disarms), **reset world**, **replay seed**, **rivals: next visit**,
**rivals: stop waiting**, **fast forward x8** and **next schematics**.
The second row: **rivals: new city**, **finish city build**,
**finish battalion** and **send battalion**. In a scripted shot:

```text
./golib shot niebla 400 --input "Enter@1 Mouse@2:40,24 \
  LeftControl@3-12 MouseLeft@4 MouseLeft@8 \
  Mouse@14:76,77 MouseLeft@15 \
  Mouse@18:204,77 MouseLeft@19 Mouse@22:250,330 MouseLeft@23"
```

`reset world` deals the region again on a different seed; **replay seed**
resets the colony on the same map (`DevResetWorld`). Both save at once;
the seed stands beside `dev`. In a shot, click `reset world` at `332,77`
or `replay seed` at `460,77`.
`rivals: next visit` brings the next scheduled arrival in immediately
(`DevNextVisit`): the scout, an intro/construction attack, or a later city
crawler. It waits while a non-settled party is active. `rivals: stop
waiting` ends the preparation of a camped party or a city's current unit
build, force replacement or regroup wait (`DevHurryRivals`). `rivals: new city`
establishes a city now (`DevNewCity`). The next three buttons each act
on the oldest city: finish exactly one construction step
(`DevFinishCityBuilding`, replacing a missing crawler first), create and
launch its next complete battalion (`DevFinishCityBattalion`), or end a
damaged force's replacement wait or a full force's regroup wait
(`DevSendCityBattalion`). `fast forward x8`
sends `devFastTicks` ticks an update instead of one until pressed again;
the simulation advances identically, only sooner.

To inspect a city assembling its first battalion, with one vehicle built and
the next taking 30 seconds:

```text
NIEBLA_CITY_ASSEMBLY_SHOT_STATE=../../build/niebla/city-assembly.json \
  ./golib go -C examples/niebla test \
  -run TestWriteCityAssemblyShotState
./golib shot niebla 60 --save build/niebla/city-assembly.json \
  --input "Enter@1"
```

`cities_test.go` can also write a save with a complete city and its artillery
battalion for inspecting the visuals:

```text
NIEBLA_CITY_SHOT_STATE=../../build/niebla/city.json \
  ./golib go -C examples/niebla test -run TestWriteCityShotState
./golib shot niebla 80 --save build/niebla/city.json \
  --input "Enter@1 Mouse@3:880,492 MouseWheel@4:3"
```

To capture a building falling to a final bullet hit:

```text
NIEBLA_BUILDING_COLLAPSE_SHOT_STATE=../../build/niebla/collapse.json \
  ./golib go -C examples/niebla test \
  -run TestWriteBuildingCollapseShotState
./golib shot niebla 4 8 20 60 \
  --save build/niebla/collapse.json \
  --input "Enter@1 Mouse@2:664,374 MouseWheel@2:5"
```

To inspect a rebuilding city and a replacement crawler approaching a new
site:

```text
NIEBLA_CITY_REBUILD_SHOT_STATE=../../build/niebla/city-rebuild.json \
  ./golib go -C examples/niebla test -run TestWriteCityRebuildShotState
./golib shot niebla 60 --save build/niebla/city-rebuild.json \
  --input "Enter@1"
NIEBLA_CITY_REFOUNDING_SHOT_STATE=../../build/niebla/city-refounding.json \
  ./golib go -C examples/niebla test -run TestWriteCityRefoundingShotState
./golib shot niebla 60 --save build/niebla/city-refounding.json \
  --input "Enter@1"
```

## Files

| File | Holds |
| --- | --- |
| `main.go` | `main` — who is playing, then the menu scene —, screen size, every color of the game, the monitor filters (made once, shared by the scenes) |
| `menu.go` | The title screen: Play before the first save, then Continue, New, Load and Quit; shared menu buttons and input |
| `load.go` | The scrollable save-slot list, with date and simulated play time; choosing a colony or returning to the title |
| `identity.go` | Who is playing: the machine's ID (registry value, platform UUID or `/etc/machine-id`), hashed with the game's salt into `player`, the number the menu shows and a later server hands tokens out by |
| `store.go` | The local database (SQLite): players, saves and the machine table; `saveBase`/`resumeState`, the scenes' door into it; the DB path, `:memory:` under `golib shot` |
| `play.go` | The play scene: input to actions plus one `Tick` per update (more while the dev tools fast forward); camera, selection, open cards, robot roster, individual assignment and pointer modes live here, never serialized; unit cards follow a directly selected robot or rival vehicle, squad cards follow their pennant on free ground or target ring; a guard pennant on a building selects the building, and its cell selects instead of opening the build menu; the schematics callout and one-use building placement mode, rivals' HUD, reach overlays, floating cost numbers and autosave are view state too |
| `radial.go` | The build menu: the two rings a click on empty ground opens - the build groups (industry, military, logistics), then the group's blueprints - laid out around the cell every frame, and `backRadial`/`closeRadial`/`openRadial` for the scene |
| `glyphs.go` | The marks the build menu wears: a group's own glyph, a blueprint's body in miniature - the very `drawBuilding` the region draws, scaled into the menu's circle, so one graphic serves both - and the pipe's mark, the tube the region lifts on posts |
| `state.go` | The simulation's state and save-schema version 12: builders, workers, troopers, mechanics and rivals with saved hull, facing and stillness ticks; builders keep site, demolition or pipe-section reservations; rival-city crawler reconstruction and refounding timers; active older city forces gain a crawler when they lack antimist; the first scout's saved core-bubble crossing tick; buildings, jobs, piles and pipes with mite wear; stock, deposits, weather and rival tables; one-tick unit, building-death and resource-cost receipts feed view effects without being saved; `newGame` gives the colony one fueled builder |
| `costs.go` | Transient resource-cost receipts from placement, production, repairs, upkeep and weapons; never saved |
| `actions.go` | The actions (`Tick`, `SendRobot`, ID-specific `AssignRobot` and `RecallRobot`, `MarkBuilding`, typed `QueueRobot`, `QueueMechanic`, `Demolish`, `CancelJob`, `LayPipe`, `RemovePipe`, `AckTech`, the dev tools' city and visit actions, and `OrderSquad`) and `Apply`, the only door into the state |
| `sim_robots.go` | The robots' rules and tuning: `robotDay`, common priorities, builders working their reserved tasks, workers mining posts, builders and unassigned workers collecting piles, oil-pool waits under fog, stationary hull damage and tank wear, `postRobots` and worker-only `pickRobot`, movement and idle ranks |
| `sim_construction.go` | One construction queue for builder reservations: nearest available site, demolition or pipe section, protector sites first; claims persist across refueling and release on completion, cancellation or death |
| `sim_piles.go` | Demolition, unit wrecks and loose items: `canDemolish`, the 25% unit recovery (`dropRobotWreck`), the piles (`dropPile`, `pileOffer`, `nearestPile`, `takeFromPile`), the stores' free room and `storeSpot`, where a load is unloaded |
| `sim_buildings.go` | The buildings' rules and tuning: blueprints' costs, placement and safe zones, protector upkeep, storage, refueling and production eligibility for builders, workers, troopers and mechanics; `canQueueUnit` is shared by the cards and queue action |
| `sim_oil.go` | Oil's spendable tanks and dedicated protector reserves: `oilTotal`, `oilCap`, `payOil`, all physical tank capacity, and `haulTank` and `refuelTank`, where a robot carries oil to and refills from |
| `sim_pipes.go` | Pumps and pipes: the `Pipe`, its curve (`pipePath`, a centripetal Catmull-Rom spline through the bends), sections and cost, `canJoin` and the ports, the sections' remaining work, `stepPipes` and `pumpStatus`; fog-covered oil pools stop pumps without losing oil, protectors clear them, tanks fill from pipes at a shared 1.6 L/s limit and pass excess onward, while protectors keep their reserve and upkeep; each pipe records offered, moved and cumulative liters for the view |
| `sim_fog.go` | The fog's law and tuning: cycles, swells, local exposure (`fogExposureAt`), oil pools covered outside bubbles, where the line stands now (`fogLineNow`), and the drag a walker keeps (`fogDrag`) |
| `sim_enemies.go` | The rivals' timing and movement: `Enemy` with its saved facing octant and mite-stillness ticks, `Party`, `Raids`, `Mark` and `Report`; the one-minute scout and follow-up clocks, the first no-camp attack, raider growth by visit up to four, city arrivals, party stages, siphoning and return, the first scout's saved outward crossing tick of the core bubble, mite damage and wrecks, guard posts whose shots report oil costs, the shared 130 m small-arms reach, repair-protocol battle markers and legacy trigger migration |
| `sim_cities.go` | Rival cities: serializable production, crawler-first replacement, sequential rebuilding and refounding, construction-site mite damage, deterministic 45-second crawler and building steps and 30-second sortie-vehicle steps, finite local oil/mineral reserves, city arrival and old-save migration, city battalions assembled one vehicle at a time to five before artillery replaces the crawler, antimist replacement, unloading, 90-second rests and squad replacement; records the pressure city's first sortie |
| `sim_tech.go` | The schematics: the robot factory is the opening drop, first delivery unlocks infrastructure, the guard post follows the first scout's return past the core bubble, and the frontier kit follows one minute after that crossing (old saves keep the 5:30 trigger); the war factory follows the first-city founding; artillery follows three ended normal attacks (old saves keep the rival-factory trigger); the repair protocol waits for rival building damage and a city-force lull (minute 12 if no first force is produced); `stepTech`, `kindUnlocked`, `dropArrived` and `techPending` derive arrivals and `State.Tech` keeps which drops were opened |
| `sim_squads.go` | The military units' law and tuning: troopers (`RobotCombat`) and mechanics (`RobotRepair`), the `Squad` and its orders (`squadOf`, `stepSquads`), a trooper's line of the day (`stepSquad`) and its gun (`shoot`), which reports ammunition oil costs, the shared 130 m small-arms reach, the war factory's capacity (`squadRoom`, `mechanicRoom`), and rivals targeting defenders (`stepEnemyGuns`) |
| `squads.go` | The squads on the screen: the number keys that call one (`squadSlots`, 1 the oldest war factory), the boxes at the top right with a trooper icon, unit count and key (`drawSquadStrip`, `drawTrooperIcon`, `squadBoxRect`, `squadBoxAt`), selection of the pennant or target ring with priority for a building under a guard pennant (`pickSquadMark`, `selectSquad`, `squadMarkAt`, `squadMarkScreen`), guard-pennant cell hit-testing (`squadPennantInCell`), the pointer's mode that gives an order (`updateOrdering`, `enemyUnder`), the pennant and the ring (`drawSquadMarks`) and `squadWords` for the cards |
| `sim_shots.go` | Shots as state: bullets that follow their target and shells that burst on a spot (`fire`, `stepShots`, `land`), vulnerable colony units, rival-shot building damage markers, building health, proportional repair charges and mechanic fuel with cost receipts, what the colony sees (`seen`) and its artillery with shell cost receipts (`stepArtillery`) |
| `shots.go` | Shots and building collapses on the screen, for looks only: bullets as streaks, shells on their arc over a shadow, pools of light added over the ground and what stands on it (`lightPool`), guns' flashes, and bursts of sparks, embers, smoke and metal shards; unit and building death effects scale to their kind, and projectile impacts merge with nearby falls; `fxField` learns from transient events and shots; view, never state |
| `spending.go` | Floating oil and lilac costs above the building, pipe or robot that spent them; recurring expenses become one-second totals, while robot carrying/fog fuel burn and refills stay hidden; view, never state |
| `enemies.go` | The rivals on the screen: the scouts' marks on the ground, shadows and eight-view PNG models for the four mobile rival chassis plus the red-gray resident city crawler, damage bars and the words the player is told (`threatWords` for the HUD, `reportWords` and `drawReport` for the news) |
| `mist.go` | The fog on the screen: a haze outside every repulsor's circle and `mistLayers` layers that thicken it past the line, each the region minus the clear circles (`clearDiscs`: the core's, the protectors', the rivals'), cut in strips whose gaps join into quads (`drawMist`, `mistGaps`), so the circles are round at every zoom and the air inside them is clear |
| `bubble_edges.go` | The core's and protectors' joined clear ground: circle intersections divide each boundary into arcs, and only the arcs outside every other bubble are drawn, leaving one exterior outline |
| `swell.go` | How a pressing swell looks: waves of shade roll in to the line, stopping at clear circles; one-pixel static also appears in calm haze, at half strength, and grows darker with exposure |
| `region.go` | The region's measures, `land` (the generated ground of the seed in hand) and `useRegion`, the isometric `project` that lifts by the relief and its inverse `unproject`, tile helpers, `Deposit` and `depositAt`; pure Go, no drawing |
| `worldgen.go` | The generator, a pure function of the seed: relief by wave function collapse, ground cover, deposits as fields of richness; its own PRNG and noise; pure Go, no drawing |
| `ground.go` | The ground's painter: relief as lit slopes, cover colors with a grain, blocks sized to the zoom and culled to the view, rocks, bushes, tufts, and the deposits cell by cell (`oreCut` wears them from the rim in) |
| `things.go` | What a cell holds, the unit the player picks by: `Thing` snapshots out of layout plus state (a deposit's card spans its patch, while buildings and sites stay on their own cell; rival vehicles and bases standing on the cell have cards too), builders', workers', troopers' and mechanics' captions, `tileAtWorld`, the SI quantities; pure Go, no drawing |
| `catalog.go` | The entity database: per thing type its name, color, unit and card lines, plus the stable-color fallback |
| `markup.go` | The `[name]...[/]` colored-text markup: parser and drawer |
| `cost_view.go` | Shared resource price parts and drawing: resource colors, amounts, and red boxes only around unaffordable resources in radial tips and panel actions |
| `inspect.go` | The inspection panel: layout, hit testing, painting, clickable paginated worker portraits, remotely opened robot cards, squad cards anchored to their marks, individual recall and return buttons, other card actions and the integrity line of a damaged building; unlocked production and pump actions stay visible but dim and unclickable when unaffordable, with only short resource prices boxed in red; the cell's outline (`cellDiamond`) |
| `robots_panel.go` | The fixed colony roster under the squad strip: builders, available workers, workers grouped by deposit and mechanics; current activity, paging, individual deposit assignment and recall |
| `sim_mites.go` | Deterministic mite exposure and wear: common unit damage, colony and rival building sites, buildings, piles that lose material over three minutes at full exposure, and laid pipe sections that wear only where exposed; active repulsors shelter their bubbles |
| `mites.go` | The mites' view: volume- and exposure-scaled swarms around robots, rivals, buildings, sites, piles and laid pipe sections; city-owned hosts animate 60% of the usual particles, without changing damage; swarms double when stationary and again in a full swell, with a red halo around still hosts |
| `pipes.go` | Pipes on the screen (`drawPipes`: casing, body, the ghost of the unlaid part, orange bands sized by offered flow and animated by liters moved) and the pointer's mode that lays one (`pipeLaying`, `updateLaying`, the curve in hand and its price) |
| `dev.go` | The dev tools: Control and two clicks on the game's name open a strip of buttons — hold a swell, place free robots, reset/replay the world, next arrival, create city, finish one city construction step, finish/send a battalion, fast-forward and next schematics —; view only, acting through `Dev*` actions; `unitsAtWorld`, the inverse of `project` |
| `tech.go` | The schematics on the screen: the badge over the core - breathing halos around the drop's mark - while an unopened drop waits, the callout its click opens (`techWords`, `techWrap`) with a clickable square per building and informational squares for pipes and the mechanic (`techBrings`, `drawTechSquares`); only building squares arm placement, and an informational-only callout stays open until dismissed; view, never state |
| `guides.go` | Screen-edge arrows for the first live scout, an offscreen rival report or pending schematics, with the scout's theft report sharing its guide; view, never state |
| `audio.go` | The region's sound: wind, oil and mineral resonance loops, pool bubbles and crystal pings, gunfire and shell impacts, the CC0 metallic building-collapse recording, low interface clicks, a site-marking thump and a fanfare for rival reports. Ordinary world emitters fade with distance; cannon reports keep their longer range. Shell whistles follow their positions through descent. Gun reports capture their distance at firing (per-voice volume tracking is noted as debt in DESIGN.md). The field reads every simulation tick, even in fast-forward; view, never state |
| `tools/soundgen/` | The maker of the wind, oil and mineral sounds and `assets/sounds/alert.wav`: stdlib Go renders the noise beds as WAV for conversion to OGG, the mineral ring as a seamless WAV with irregular pitch drift of at most one semitone, the crystal ping with amplitude modulation, and the low alert fanfare with `--alert`; run it only when a sound changes |
| `draw.go` | The region painter: the core's monolith, buildings with their damage bars, the bubbles' outside edges, build-site wireframes and the gray schematic-placement ghost, the marking cursor, the stores' fill bars (`drawFillBar`) and the idle count by the core |
| `units.go` | The colony's four unit models on the ground, with Blender-rendered shadows, cargo, charge feedback, combat-unit health bars and workers' oil-tank bars below their feet |
| `worldsprites.go` | The eight eight-direction model and shadow sheets in the world and the scaled model icons; converts the world ground point to a screen pixel before drawing so moving sprites do not jump by the camera's zoom |
| `unit_picking.go` | Unit picking and inspection: each model's opaque PNG bounds become a screen-space body rectangle at its current facing and zoom; a small screen-pixel margin makes it clickable, frontmost overlapping units win, and selected unit cards follow their entity |
| `orientation.go` | The screen-space eight-way facing derived from an isometric movement vector, and the ground-plane yaw used for world details |
| `sources/models/studio.py` | One shared Blender authoring toolkit: geometry primitives, materials, 2:1 camera, light, world-yaw conversion, eight-view model sheets and isolated shadow masks |
| `sources/models/artillery.py`, `other_units.py`, `*.blend` | Geometry for the eight distinct mobile chassis and their editable Blender sources; not shipped with the game |
| `assets/sprites/*.png`, `assets/sprites/shadows/*.png` | Eight transparent eight-frame model sheets and their matching Blender-rendered shadow masks; loaded on desktop (the PNGs are web-compatible, but Niebla's SQLite driver does not build for web) |
| `assets/sounds/building-collapse.wav` | CC0 mechanical explosion with clattering metal; its source and license are in `assets/ATTRIBUTION.md` |
| `region_test.go` | Layout, projection, things, SI formatting, catalog tests |
| `guides_test.go` | Offscreen arrow placement and direction, the first scout while alive, report sharing, visible-target hiding, report timing, pending schematics and spacing; can write visual fixtures with `NIEBLA_GUIDE_SHOT_STATE` and `NIEBLA_SCOUT_GUIDE_SHOT_STATE` |
| `markup_test.go` | Markup parser, tooltip layout/button, portrait hit-testing, remote robot card and page-layout tests; can write a damaged-building repair-cost fixture with `NIEBLA_REPAIR_COST_SHOT_STATE` |
| `robots_panel_test.go` | Roster grouping, paging, row bounds, button placement and selected-unit details |
| `unit_picking_test.go` | Opaque bounds for every model and facing, frontmost sprite targeting, direct unit cards following movement, rival health details and closing cards for units that disappear |
| `shots_test.go` | Small-arms reach limits, the first raid against one guard post over three seeds, bullet and shell impacts, building damage and collapse effects, proportional repair costs and mechanic fuel, transient death receipts, explosion scaling/merging, defender damage and wrecks, and old war-factory saves |
| `repair_tech_test.go` | Repair-protocol triggers and lull boundaries, rival-fire markers that survive building destruction, rejected locked mechanic orders, and legacy-save compatibility |
| `squads_test.go` | Trooper production and squad behavior, mechanic limits, target selection, health and wrecks; pennants on free ground open squad cards, pennants on buildings open building cards, and both offer the order button; their ground cells select instead of opening construction; squads can attack the first incoming crawler without a camp delay |
| `world_test.go` | The simulation driven directly: the starting builder, explicit individual assignment, worker-only auto-assignment, role-specific carrying, loot and construction priorities, migration, dry deposits, determinism and JSON round trip |
| `sim_construction_test.go` | Parallel building reservations, protector priority without interrupting a current task, nearest pipe sections beside building sites, reclaiming cancelled or lost tasks, and continued work after loading a save |
| `economy_test.go` | The deterministic economy probe: four legal opening policies over three seeds, sampled each minute into an opt-in CSV with the scout's core crossing tick, protector fuel, party and city production, repair milestones, artillery's arrival tick, mechanic count and outstanding building damage |
| `buildings_test.go` | The buildings driven directly: five-second marked construction, payment, fog placement, factory robots, refueling, digestion, full stores, silos and the protector's bubble |
| `costs_test.go` | Placement and production receipts, mechanic repair oil, excluded robot fuel, one-second grouping of regular costs, four-second protector totals, and the optional `NIEBLA_COST_SHOT_STATE` visual fixture |
| `pipes_test.go` | Pumps and pipes driven directly: oil-pool fog stops robot loading and pump flow without draining the pool, a protector restores extraction, exposed pumps wear down unless sheltered, laid pipe sections wear in exposed air, pipes are paid and laid by sections, robots claim one section each, tanks share their pipe-fill limit across inlets and pass excess through a chain, protectors keep their reserve and upkeep, source outlets share flow, blocked tanks throttle pumps, bands show offered versus actual flow, pipes move oil between tanks, workers haul and refuel, illegal pipe actions are refused, pipe removal drops its cost as a pile, curves follow bends, and saves resume deterministically; can write a pump/protector flow fixture with `NIEBLA_PIPE_FLOW_SHOT_STATE` |
| `protector_test.go` | Protector fuel: upkeep drains its dedicated tank, radius fades below the configured threshold and vanishes empty, robots and pipes refill it, the reserve stays unavailable to other costs, old saves migrate once with starting charge, and an opt-in state fixture supports visual shots |
| `mites_test.go` | The mites' view without a window: volume and exposure counts, doubled still and swell swarms, half strength in haze, the red halo, orbit, trail, fade and falloff |
| `sim_mites_test.go` | Mite simulation rules: the same stationary unit damage across roles and rivals, no damage while moving, doubled swell damage, charged pylon immunity, colony and rival site wear, reduced city particle counts, pile lifetime, and exposed pipe wear |
| `dev_test.go` | The dev actions: a held swell stays up and doesn't count, a placed robot is built, rival camp/regroup/rebuild waits can be hurried, the city tool buttons have separate hit boxes, `unitsAtWorld` undoes `project` |
| `cities_test.go` | City founding beyond artillery range and on the scout's bearing, 45-second building steps, pylon-first construction, finite local economy, unit-by-unit 30-second sortie assembly and replacement, unloading/reuse, the full-squad rest, destroyed-force rebuild timing, dev actions, replay and JSON persistence |
| `fog_test.go` | The fog driven directly: cycles, swell timing, line and bubble margin, pushed-band drag, stationary wear by exposure and swell, movement and bubble resets, saved wear and the HUD forecast |
| `identity_test.go` | The identity derived from a machine ID: stable, distinct, and the parsers of what `reg query`, `ioreg` and the machine-id files say |
| `store_test.go` | The database driven directly: an identity kept across runs, the fallback one too, the token column waiting empty, a base saved and loaded back whole, a second save replacing the first, one player's save invisible to another, the DB path's rules |
| `tech_test.go`, `artillery_tech_test.go` | The schematics driven directly: factory at start, first delivery brings infrastructure before the scout, the first scout unlocks the guard only after crossing out of the core bubble, the frontier kit follows one minute later and keeps its 5:30 timing on old saves, old mark-or-oil saves keep earned guard access, the pressure city unlocks mobile units, artillery follows three ended normal attacks and old saves keep the factory trigger, locked actions are refused, `DevNextTech` brings the ladder in order, callout squares and one-use building selection, snapped pump placement and automatic dismissal; the ledger survives a round trip and optional shot states cover guard and artillery timing |

## Architecture

DESIGN.md's four laws (one serializable state, actions in/state out, the
game is a visualization, determinism) hold since slice 4, in a first,
robot-sized form:

- `State` (`state.go`) is the whole game: robots, buildings, stores,
  deposits' remaining resources, jobs, weather, rival vehicles and
  parties, rival cities and their production and construction wear, reports
  and projectiles.
  The ground itself is generated from `State.Seed` and never enters the
  state. It has no pointers, channels or functions, so it serializes as it
  is. `State.Version` 11 migrates saves: legacy `core` units become fueled
  builders, `built` units become workers, factories keep their selected
  product while it is in progress, builders and workers receive their
  initial hull, active city forces infer composition from survivors, and
  saves with a previously available mechanic keep that repair capability.
  An active older city force without an antimist vehicle gains a crawler.
  Guard schematics earned under the old mark-or-oil trigger stay unlocked
  without a new badge. Older saves retain the former rival-factory trigger
  for artillery and the former 5:30 frontier-kit trigger. The first scout's
  core-bubble crossing tick is saved for the one-minute frontier delay in new
  games. The new mite stillness and wear fields start clean in older saves;
  old enemy fog timers no longer carry into the mite rule.
- Actions (`actions.go`) are structs (`Tick`, `SendRobot`, `AssignRobot`,
  `RecallRobot`, typed `QueueRobot`, `QueueMechanic`);
  `Apply` mutates the state it is given — one owner, no copies — and is
  total and deterministic, so a seed plus an action log replays a game.
- `stepRobot` (`sim_robots.go`) derives each tick from `robotDay`. Everyone
  brings carried loads home and minds its tank first. Builders then finish
  loading and continue a reserved construction task or claim the nearest
  available one, prioritizing protector sites over other sites, demolitions
  and pipe sections; they collect piles and work an assigned post only
  when no construction remains. Workers skip construction, returning to
  a post or collecting piles when unassigned. Mechanics and troopers use
  their own task lines. The claim is saved on the robot and survives
  refueling, but not cancellation, death or an explicit assignment.
  Captions read the same priority (`Robot.taskNow`), and actions change
  saved assignments.
  Anything that iterates entities iterates them in sorted ID order.
- `Facing` is the last screen-space movement octant on robots and rival
  vehicles. It affects only their drawing, stays unchanged while still,
  and defaults to right when an older save has no field for it. World
  units turn that heading back into a ground-plane yaw before projection;
  portraits and squad icons keep the screen-space transform.
- The play scene sends input actions and one `Tick` per update; `Draw`
  only reads. View state — camera, picked cell, marked blueprint, open
  cards, pointer — lives in the scene and never serializes.
- The catalog and the markup palette are view-side metadata, not state:
  package-level tables, like a schema. Card lines read the state, so
  they can say what remains in a deposit and what a robot is doing.

### Buildings and built robots

The ground divides past its tiles: a tile of 200 u holds 8 by 8 **cells**
of `buildingCell` 25 u on a side (sim_buildings.go), and one cell is the
footprint of the smallest building — about four robots across — the
tile grid's last subdivision, so a tile may hold several buildings. A
building's `Col, Row` in the state are cell coordinates; `cellAtWorld`
undoes the projection onto the cell grid the way `tileAtWorld` does onto
tiles.

Eight blueprints in the radial menu, and the pump off it (see [Pumps and
pipes](#pumps-and-pipes)) (`BuildingKind` in `state.go`, rules and tuning in
`sim_buildings.go`): the **robot factory** builds robots from lilac and
oil, the **charger** refills a built robot's tank from the stores, the
**silo** and the **warehouse** add oil and lilac storage room, and the
**shadow protector** holds a small bubble of safe ground of its own while
its dedicated oil tank has fuel.
Marking is building, and building is earned: the blueprints arrive as
**remote schematics** (`sim_tech.go`, the ladder; `tech.go`, the badge
and the callout). The factory blueprint waits over the core from the
start. A drop lights a pulsing **badge over the core** while it waits —
the HUD adds `schematics at the core` —; its click opens a callout that
says what came in, and the blueprints join the menu. The first delivered
load brings infrastructure. The guard post comes when the first scout,
returning after its theft, crosses outward past the core's fixed 800 m
bubble; its oil and mark alone do not unlock it inside.
The menu offers only
what the cell could really take - what the schematics, the ground or
the fog refuse is not on the rings, and before the first drop the menu
doesn't open at all. The stores don't take options off: a blueprint
they can't pay stands washed out to gray (its group with it while
nothing in it could be paid), refuses the click, and the tip beside it
names and prices it, a red box around each resource that falls short
(`radialTipOf`, drawn by `drawRadialTip`). Marking itself takes
three clicks: a click on a free cell
of ground opens the **radial build menu** (`radial.go`) right on that
cell — the options lay out around the cell's projected center every
frame, so the menu follows the view —, a click on a **group** opens its
ring (industry: factory and war factory; military: guard post and artillery;
logistics: charger, silo, warehouse and protector) and a click on a
blueprint pays its cost from the stores and marks it on that very cell.
A right click goes back a ring, and closes the menu from the first. Each
option wears an icon: a group its own mark (`glyphs.go`), a blueprint
the very body the region draws (`drawBuilding`) in miniature, so one
graphic serves both. Options read their own place on the rings
(`radialOffered`: `kindUnlocked` plus `canPlace`); whether the stores
could pay shows as the wash and in the tip.
The drop's callout offers the same buildings directly: clicking a building
square hides the callout and arms a gray wireframe cursor on the cell grid.
The pump snaps to an oil pool's heart. A valid click marks the job; an
invalid cell leaves the mode and resources alone. After success the callout
returns with that square dimmed and disabled, and closes automatically when
its last building is used. Pipes are informational there and are still laid
from an oil tank's card.

The job joins the queue; each builder reserves a different site, demolition
or pipe section, choosing the nearest free task with protector sites first.
It completes its claim before choosing another, and stands on the cell's edge
(spread by ID) where the rising body can't swallow it. The site
shows the part already built in solid colors inside a **wireframe** of
the whole body, with the work's progress bar under the cell, drawn over
the fog so a site in the mist stays visible.

### Builder and worker roles

The core starts with one white `RobotBuilder`, already fueled. It can be
lost to the fog like any other builder. The robot factory's blueprint is
the opening schematic; the first delivered load unlocks logistics. Once
raised, the factory card offers `build builder` and `build worker`, each
costing 40 kg of lilac and 30 L of oil and taking 12 seconds.

Builders alone claim construction jobs and pipe sections. They also collect
loose piles and can work a deposit when assigned directly, but their cargo
capacity is one third of a worker's. Workers harvest and haul; only workers
without a deposit assignment collect loose piles, so a posted worker stays
on its mining work. Workers never claim a build job or pipe section. Both
roles share speed and tank size. `SendRobot` chooses only an unassigned
worker; it never silently changes another worker's post. `AssignRobot` names
a builder or worker ID, sets its deposit post, and cancels unfinished
loading while letting cargo already carried reach storage.

The roster button below the squad strip opens `robots_panel.go`. It groups
builders, unassigned workers, workers by deposit and mechanics, and shows
each unit's current activity. Selecting a builder or worker allows
individual assignment to an oil pool or lilac vein, or recall. The panel,
selection and armed assignment are view state; the resulting `PostCol` and
`PostRow` are simulation state changed only through `AssignRobot`.

The fog's law, in `canPlace` and `inSafeZone`: protectors and pumps
may be marked outside a bubble; a finished pump there is swarmed by mites
and digested in 10 seconds unless a protector shelters it. The loss is
reported and half the pump's cost falls as a pile. Other infrastructure
must be marked under a bubble. A protector is a dedicated 200 L tank
(`protectorOilCap`), initially charged with the 40 L paid for its blueprint;
it burns `protectorOilPerSecond` 0.125 L/s continuously. Its initial 40 L
lasts 5 min 20 s without a refill, and its floating cost totals 0.5 L every
four seconds. Its full 400 m radius starts fading below
`protectorRadiusFadeBelow` (5% of capacity), reaches zero when empty, and
returns as robots or pipes refill it. Protector oil is reserved:
`oilTotal`, costs, and rival raids exclude it. The bubbles also cancel the
fog's drag, which slows every robot to half its pace deep in the mist.

### Oil's tanks

`sim_oil.go`: oil has a place. `State.Stock.Oil` is the core's own tank
and `Building.Oil` a silo's, a charger's or a protector's (`tankCapOf`);
a tank is named by its building's ID, the core's by `coreTank`, 0.
`oilTotal` and `oilCap` sum only oil available for spending; the
protector's dedicated reserve is excluded from costs and raids. `payOil`
takes oil from available tanks, the core's first. `allOilTanks` also lists
protector tanks: robots haul to the nearest tank with room (`haulTank`,
`storeSpot` and `deposit`), and pipes can feed or drain a protector.
`refuelTank` still chooses the nearest charger or core with oil, where
`refuelSpot` walks and `refill` draws. `Demolish` drops a tank's held oil
in its pile. Lilac is still one stock under `lilacCap`. A save from before
the tanks loads with all its oil in the core, over its cap if need be: it
takes no more until it is used or piped away.

### Pumps and pipes

The **pump** (`BuildingPump`) is the one kind `canPlace` takes on oil
instead of ground: on a pool with oil left and no pump yet
(`patchPumped`), even outside a bubble. It isn't in the radial menu - a click on
a pool inspects it - so the pool's card carries `build pump`, which marks
it on `pumpCell`, the patch's middle. The pump and its site appear only on
that cell; the rest of the pool keeps its deposit card. The patch still
owns the pump for placement and extraction, so it takes only one. Once its
schematic has arrived, `build pump` remains visible on a placeable pool
even when the stores cannot pay; the button is gray, and only a missing
resource amount is boxed in red, like an unaffordable radial blueprint.
`lay pipe` appears when a pipe port is free and its schematic has arrived
(`LayPipe`'s frontier-kit guard); its changing total is shown while drawing.

A **pipe** (`State.Pipes`, by ID) carries oil one way, `From` a pump or
a tank `To` a tank, including a protector's dedicated tank, through the
player's `Bends`, in units. `canJoin` is
the network's law: two different ends, a port free on each
(`freePorts`: `pipePorts` 3, `corePipePorts` 6), no pipe between them
already. The curve is never stored: `pipeSpine` rebuilds it from the
ends and the bends with `pipePath` - a centripetal Catmull-Rom spline
cut into `pipeSpanSamples` pieces a span, with a `sagPoint` when there
are no bends - so the sim (length, sections, where the robots stand) and
the view (the drawing, the bands) read the same line. `Sections` is what
was paid and `Left` the robot work owed, which goes in a section at a
time, by many hands: `stepBuild` reserves the nearest available task,
including pipe sections, walks to its middle (`sectionSpot`) and stands
by it for its `pipeSectionWorkTicks`, then reserves another. The claim is
state - `Robot.Pipe`, `Robot.Section` - because the other robots read it;
it survives refueling but releases when the section is finished or
removed, or when the robot dies. `Pipe.SectionLeft` holds the
work left by section, and is nil before the first tick of work and once
the pipe is laid, when `Left` alone reads as work put in from the source
out (`sectionLeft`), which is how a save from before the sections
loads. `building` claims a robot only while its construction task remains
or another is free, so robots with nothing to build go on with their day.
`stepPipes` runs after the factories, in source-to-destination order. A
source divides its available oil equally among laid outlets that can
accept it; each pipe carries up to `pipeLitersPerSecond`. Every tank takes
at most `tankFillPerSecond` 1.6 L/s from pipes, shared across its inlets.
The tank stores what fits within that rate and passes excess through its
outlets in the same tick; a full tank can pass oil without storing it.
When no outlet can accept the excess, the source is throttled to the
network's capacity, so no oil disappears. Robot unloading is unchanged.
Non-protector tanks can also feed their outlets from oil already stored in
them. Pumps draw up to `pumpLitersPerSecond` from `State.Drain`. A protector
passes only incoming oil beyond its fill amount, never its stored reserve.
When full, it takes incoming oil to replace the
`protectorOilPerSecond` 0.125 L/s that upkeep drains at the end of the tick,
then shares any surplus equally among its outlets. A full protector with no
outlet takes only its upkeep from its incoming pipe.

`Pipe.Offered` records the source's share before the destination limits
it, `Pipe.Flow` the liters actually moved in the latest tick, and
`Pipe.Moved` the total moved since it started flowing. These are saved
state, so saves and replays keep the animation deterministic. The view
colors each band by `Offered`: 2 L/s fills 90% of the gap, leaving 10%
steel gray. Its phase follows `Moved`, so a destination that accepts less
slows the band without a phase jump, and a blocked or dry pipe stays gray.
Each full protector subtracts its upkeep from the offer to the next link;
the sixteenth pipe in a chain gets 0.125 L/s, and the seventeenth gets none.
`Demolish` calls `takePipesOf`, so a pipe never outlives an end.

To compare those two flow rates in consecutive shots:

```text
NIEBLA_PIPE_FLOW_SHOT_STATE=../../build/niebla/pipe-flow.json \
  ./golib go -C examples/niebla test \
  -run TestWritePipeFlowShotStates
./golib shot niebla 20 40 60 --save build/niebla/pipe-flow.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:2" --scale 2
```

The saved region shows a pump feeding a silo at full output and the core
feeding a full protector at its upkeep rate. The shots zoom in on both
pipes; the protector's bands cover one eighth the distance over 40 updates.

The laying mode is view (`pipeLaying` in the scene): it collects bends
and sends one `LayPipe` (`sendPipe`) on the click that lands on a tank
the pipe may end at (`layTargets`) - `layTarget` tests the pointer
against the tank's body on the screen, foot to top, a building winning
over the core - or on `connect` in the last node's menu
(`layMenuLayout`, `pickLayMenu`), which ends it at `nearestTarget`. The
panel lists an end's pipes as button rows (`pipeEndOf`, `pipeNote`; the
row's `ref` is the pipe `RemovePipe` takes). `drawPipes` runs twice,
like the piles: clear stretches under the buildings and robots with
their shadow, fogged ones and the unlaid ghost over the fog; the pipe is
drawn `lift` above its ground line, on posts.

### The fog breathes

The weather is state (`State.Fog` in `state.go`, law and tuning in
`sim_fog.go`): `Cycle` counts whole cycles of `fogCycleTicks` (30 s);
`NextIn` counts the cycles of calm left before the next **swell**;
`SwellLeft` is the ticks the current swell has left; `Swells` remembers
how many have passed, and every dial grows with that count — each swell
comes `fogSwellQuickener` times sooner (floor `fogSwellMinPeriod`),
lasts `fogSwellTicksGrowth` ticks longer (roof `fogSwellTicksMax`) and
presses `fogSwellGrowth` tiles deeper (`fogSwellReach` the first time),
but never past `fogSwellMargin` of the bubble: `swellReach` caps it, so
the core's ground is not negotiable whatever the swell count.

A swell starts and ends on its tick, but what the ground feels is its
`Pressure`, 0 to 1, which `stepPressure` walks up while a swell is up
and back down in the calm, `fogSwellRampTicks` either way: `fogLineNow`
is the calm line minus the reach times the pressure, and the extra burn
scales with it. `swell.go` draws the pressing - crests of shade rolling
in (`drawSwellWaves`, in the world, under the fog line) and static over
the mist (`drawSwellStatic`, in screen pixels) - from `State.Ticks`
alone, through `hashUnit`, with no randomness and no memory.

`fogExposureAt` is zero in the core or a colony protector bubble, 0.5 in
the clear haze, then rises with `fogAt` to 1 in deep fog. A tanked robot
that has not moved outside a colony bubble for `fogStillGraceTicks` (2 s)
loses oil at `fogStillBurnPerSecond` (1 L/s times exposure) and hull at
`fogStillDamagePerSecond` (2 points/s times exposure); movement resets
`Robot.StillTicks`, and old saves start that counter at zero. The ordinary
load burn remains separate. Mites and static specks also follow exposure:
the haze shows them at half strength. Rivals keep their existing
`fogAt` exposure timer.

Any oil pool outside colony bubbles is covered even in calm haze.
`oilPoolInFog` blocks robot loading and pump flow but never changes
`State.Drain`; a protector over the pool restores access. A worker assigned
to a covered pool keeps its post and waits at the core. Pool cards and pump
cards say when the fog blocks access. Lilac mining is not blocked by this
oil-pool rule.

A swell starts whole at a cycle's end — which is what makes the HUD's
forecast exact: while it says `swell next cycle` (the ghost line stands
where the fog will press in), the swell rises at that very boundary.
While it is up, `fogLineNow` returns the pressed line, the view paints
the band and the line there, and the sim gets meaner in the pushed
band: `fogDrag` keeps a quarter of the step where the calm fog would
leave clear ground (`fogSwellSpeedFactor`), and built robots outside a
bubble burn their tanks 1.5x (`fogSwellBurn`). Nothing else changes:
placement (`canPlace`) and the bubbles never read the swell.

`RobotKind` has four roles: builders (`RobotBuilder`) construct and lay
pipes, workers (`RobotWorker`) harvest and haul, troopers (`RobotCombat`)
fight in squads, and mechanics (`RobotRepair`) repair buildings. The core
gives one builder at the start; the factory builds either a builder or a
worker at the same cost. Both use the same speed and 120 L tank. Builders
carry one third of a worker's oil or lilac load and can be assigned to a
deposit for emergency hauling. Workers assigned to a deposit are never
pulled into construction; `SendRobot` chooses only an unassigned worker,
and `AssignRobot` retasks the selected unit by ID.

Builders and workers burn oil while carrying; mechanics spend it on
repairs. Troopers carry nothing and pay for their shots. Every unit has
hull: builders and workers have 60 points, troopers 80 and mechanics 60.
Standing outside a bubble for two seconds makes the fog drain oil and
damage hull by exposure; moving resets the counter. Under the low
line (`robotLowTankAt`) the tank claims its day and walks the unit to the
nearest charger or the core, where it stands until full — even past the
low line, so it does not dance between post and work. Full is
`tankFullSlack` short of the brim. Outside a bubble, an empty tank or hull
means the fog digests any of these units, including the opening builder.
Load burn and drag are dials in `sim_buildings.go`; stationary fuel and
hull wear are tuned in `sim_robots.go` and use `fogExposureAt`.

When the fog digests a built unit, `stepSim` leaves a wreck with
`unitWreckRefund` 0.25 of each resource: build cost, cargo and remaining
tank oil. Troopers and mechanics killed by enemy fire use the same
recovery in `dropRobotWreck`; builders and workers cannot be targeted.

The war factory can queue troopers with `QueueRobot` as soon as its own
schematics arrive. `QueueMechanic` and `QueueRobot{Kind: RobotRepair}` stay
locked until the repair protocol arrives: the first real attack ended, a
rival shot has damaged a colony building, and the first pressure-city force
has reached a lull. If that city produces no first force, the timing falls
back to minute 12, still requiring rival building damage. A mechanic costs
100 kg and 50 L, takes 15 seconds to build, has 60 health, and repairs at
6 damage/s. Each repaired point also costs 0.2 L from the mechanic's own
tank and a proportional part of the building's construction cost: a full
repair costs 50% of the blueprint's lilac and oil prices, deducted from
colony stores as the mechanic works. Repair does not top up the building's
own oil tank, which follows its ordinary store and logistics rules. The
mechanic chooses the oldest damaged building, and stops when it lacks fuel
or either repair resource; it can be hit by the same rival bullets and
shells as a trooper. Its repair task
replaces harvesting, construction, pipe and pile work; when there is nothing
damaged it waits by its factory. Old saves that already had mechanic
production keep it.

The stores have a roof: `oilCap`/`lilacCap` is the core's own room plus
every silo and warehouse. A robot hauling into a full store stands at
the store trying again each tick (its card says waiting for storage), and
deposits what fits when a silo opens room. The stores are one stock, but
a load's walk ends at the nearest store of its kind (`storeSpot`): a
warehouse or the core for lilac, a silo, a charger or the core for oil.
The core is a store like the others: a robot unloads `storeStandoff`
from its middle, never out at a parking spot.

Idle builders and workers rest by the core, in ranks before the monolith's
broad face (`parkSlot`: `parkSlots` 10 places, `parkRankSize` 5 to a rank,
`parkSpacing` 7 u). A worker's place is its `idleRank`, how many idle
workers come before it by ID, so the ranks close up when one leaves; past
ten the rest stand inside the first ones and the view writes how many
there are (`drawIdleCount`, also while the view is so far out that the
ranks are one dot).

Demolition (`sim_piles.go`): `Demolish` takes a building out of the
state at once and `CancelJob` a site out of the queue. What it was made
of falls on its cell as one `Pile` (`State.Piles`, by ID): the
blueprint's cost times `demolishRefund`, the cost of the unit a factory
was building, and what the stores lose the roof for (`spillOverflow`).
A pile holds its cell against `canPlace` until its last item leaves,
which deletes it. A protector can't go while it alone shelters another
building or a site (`canDemolish`, on `shelteredWithout`). Builders and
unassigned workers pick piles up after construction and before their posts:
the nearest pile that holds something the stores have free room for
(`freeRoom` counts what is already on its way home, so nobody loads what
won't fit), one kind per trip, lilac first, loading for `robotLoadTicks`;
assigned workers skip this task and stay at their posts. `Robot.Pile` says
which pile a loading robot stands at, 0 at its post.

### The rivals

`sim_enemies.go` owns the scout, the recurring construction attacks and
later city arrivals. Rival vehicles are in `State.Enemies`; parties carry
their stage, movement targets, wait/siphon timers, city ID, saved squad
size and artillery composition. `State.Raids` remembers the visit count,
next arrival, the scout's first bearing and the first pressure city's ID.
The scout appears after one minute. One minute after it leaves or is
destroyed, a crawler-led attack starts on its bearing as the first city is
founded 10 tiles (2 km) from the colony core, beyond the colony artillery's
1.5 km range. During that city's construction, another attack follows one
minute after each previous party ends. Later city crawlers arrive on the
city clock. Every traveling rival party has an antimist vehicle. Gameplay
randomness uses `State.roll`, so bearings and saves replay deterministically.
No city has a passive gun. City buildings are
`Enemy` entities tagged with their owning city ID, so existing selection,
guard-post targeting, squad orders, artillery and damage resolution can
address them. Their rules and serialization live in `sim_cities.go`.

Intro raid members move as one (`driveParty`), with the crawler as the
leader and `formationOffset` for their spacing. The first real attack has
one raider and does not camp; later construction attacks camp and add one
raider per visit, up to four. A raid chooses the nearest colony tank with oil
(`raidTarget`), moves to `siphonReachUnits`, draws
`siphonLitersPerSecond` from it until full/no oil/`raidSiphonTicks`, then
leaves for the entry point. City sorties use the same raid and return
rules, but their entry point is the city. The factory builds one vehicle
every `cityUnitBuildTicks` 1800 ticks (30 seconds), paying that vehicle's
share of the full sortie cost as it completes. The battalion stays at the
city until every vehicle is built, then attacks together. It starts with a
crawler and one raider, adding a raider per new battalion up to four. Once
the force reaches five vehicles, later battalions replace the crawler with
artillery and remain at five. A force that returns with oil unloads at
`cityUnloadPerSecond` 3 L/s per living vehicle. A full squad waits
`citySortieCooldownTicks` 90 seconds before the city starts its next
assembly. A damaged squad builds one replacement every 30 seconds; an empty
or destroyed force waits a minute before assembling its replacement.
`movingParty` permits one active party region-wide; other cities wait for it
to return, unload or be lost.

Each newly founded city saves an `AnnounceUntil` tick one minute ahead.
Until then, `threatWords` may show its status in the HUD. Its `ReportSettled`
news plate and offscreen arrow use the same deadline unless newer news
replaces them. Other reports and their arrows use the 15-second
`reportShowTicks` lifetime. Old saves without an announcement deadline do
not announce settled cities again.

The first scout gets a live screen-edge arrow while it is outside the view.
When its theft report appears, the report's red arrow takes precedence
while its location overlaps the live scout, avoiding a duplicate. Once
they separate, each can have a guide. Other current reports and the live
scout can each have a guide, alongside the pending-schematics guide.

Mites affect rival vehicles, the resident city crawler and city structures
under the same health rule as colony units. `stepExposure` compares each
enemy's position across ticks: movement resets `Enemy.StillTicks`; after two
stationary seconds in
exposed air, health falls at `miteDamagePerSecond`, doubled by full swell
pressure. Active repulsor bubbles clear mites. A charged city pylon is itself
immune; other city structures become vulnerable when no active bubble covers
them. However a vehicle dies, `killEnemy` leaves its loot and all it stole as
a pile on its cell, which then loses material to mites if exposed.

`stepMiteWear` (`sim_mites.go`) applies the same health rate to colony
buildings and sites. Depleted protectors are vulnerable; charged protectors
are not. A pile's `MiteTicks` advances by local exposure and swell pressure
until three minutes at full exposure, shrinking its oil and lilac to nothing.
Laid pipe sections contribute damage in proportion to the exposed part of
their length; a pipe disappears at ordinary building health. An exposed rival
city construction site also wears down; if consumed, its current 45-second
building step starts over. The core has no mite host and is not part of this
system.

The city progression (`sim_cities.go`) is stored in each `City`: stage,
construction timer, finite oil and lilac reserves, city stores, resident
crawler and structure IDs, next production tick and completed sorties. The
first city starts on the scout's bearing at the second attack; its crawler
is the initial constructor. Later crawlers arrive from the region edge.
Every city builds a pylon first, then a Nexus, oil extractor, lilac mine and
war factory. Each step takes `cityBuildTicks`; completed structures are
city-owned `Enemy` records. The resident crawler is required for construction,
extraction and sorties. If destroyed while a structure remains, it takes a
full 45-second build to replace before any missing structure; with no
structures left, the city is razed and later refounded. Oil and mineral
extraction stop if their building or the crawler is missing. The war factory
assembles a sortie as a saved `StageBuild` party, producing
and charging for one vehicle every 30 seconds; the party attacks only when
its full composition is ready. Intact returning squads are reused after
unloading; a missing unit, including the antimist crawler, is rebuilt one at
a time, 30 seconds per vehicle. Empty or destroyed squads wait one minute,
then assemble their replacement the same way. The city repulsor shelters
nearby city units; a sortie's crawler or mobile artillery carries its own
bubble.

A guard post (`BuildingGuard`, in the build menu) reloads in
`Building.Reload` and shoots the nearest vehicle within
`smallArmsRangeUnits` 130 m (`nearestEnemy`) with a bullet of
`guardShotDamage`, paid `guardShotOil` out of any tank. Troopers and the
rival crawlers and raiders use that same reach; artillery is separate.

Shots (`sim_shots.go`). No gun hits at once: `State.fire` puts a `Shot`
in `State.Shots` and `stepShots` flies it - a bullet at `bulletSpeed`
after its target, which it hurts on arrival if it still stands, a shell
at `shellSpeed` to the spot it was aimed at, where `land` hurts
everybody of the other side within `shellBlastUnits`: rival vehicles
for the colony's shells, and troopers, mechanics and buildings for the
rivals' (`hurtEnemy`, `hurtColonyUnit`, `hurtBuilding`). Guard posts,
troopers and
the rivals' guns fire bullets. A building counts what it has taken in
`Building.Damage`; at `buildingHealth` it goes through `takeDown`, the
door `Demolish` uses too, with `wreckRefund` of its cost, and
`ReportRazed` says so. Only a war-factory mechanic mends the oldest
damaged building (`damagedBuilding`, `mend`), at `repairPerSecond` 6
damage/s. Each point costs `repairOilPerPoint` 0.2 L from its own tank and
a proportional share of the building's construction cost from colony
stores; a full repair costs 50% in lilac and oil. Ordinary workers never
repair; the core is no building and takes nothing.

The arriving crawler keeps its ID as the city's construction vehicle. The
city first raises its pylon, then a Nexus. Old saves with a `StageSettled`
base migrate in `State.enterRegion` as a completed pylon and Nexus, then
build a resident crawler if they lack one. Destroying the crawler pauses the
city and puts its replacement first in the construction queue. Destroying a
structure queues its replacement while any structure remains; losing all
five structures razes the city and clears its stores. Remaining city forces
are left to their party and fog rules.

The colony's artillery (`BuildingArtillery`, `stepArtillery`) shells
visible rivals between `artilleryMinUnits` and `artilleryRangeUnits`, a
base or static city building before a vehicle, and pays every shell in
lilac and oil. The mobile rival artillery (`EnemyArtillery`) has its own
600 m range (`cityArtilleryRangeUnits`) and uses `fireCityArtillery` during
a sortie; it fires only at colony buildings, never the core. Its shots are
rival shells, so existing `land` and `hurtBuilding` apply.

Squads and mechanics (`sim_squads.go`). A war factory
(`BuildingWarFactory`) builds troopers through `QueueRobot`; after the
repair protocol arrives, `QueueMechanic` builds one repair unit per
factory. Both mechanic queue actions and the card button share the gate;
`squadRoom` and
`mechanicRoom` enforce their separate limits. `robotProduction` holds
each unit's cost and build time. A trooper is a `Robot` of kind
`RobotCombat` whose `Squad` is its war factory's ID. A mechanic is kind
`RobotRepair`, remembers its producing factory and has health like a
trooper, but is not in a squad. `tanked()` is every robot but the core's,
so the tank's laws - the burn, the refuel line, the fog's digestion -
cover both. The working lines of the day refuse both combat units. The
trooper's own line, `taskSquad`, comes after the tank's: `stepSquad`
walks it to its place around the guarded spot (`formationOffset`, where
it mends under a bubble) or after the squad's focus, to `squadStandoff`
of its reach. A mechanic's `taskRepair` seeks damaged buildings, repairs
only with oil in its tank, and waits by its factory when there is nothing
to mend. `shoot` runs every tick before the day, so a trooper fires on
the move: the focus when in reach, else the nearest, each shot paid from
its own tank. Orders are
`State.Squads`, by war factory ID, written by `OrderSquad` alone;
`squadOf` gives a squad with no entry the order of guarding its door,
and `stepSquads` drops an attack whose party is gone and passes a fallen
focus on to the party's leader. `stepEnemyGuns` is the rivals' side:
vehicles whose `enemySpec` has a gun fire a bullet at the nearest trooper,
mechanic or guard post in reach; `hurtColonyUnit` (`sim_shots.go`) is
where a trooper or mechanic falls and leaves 25% of its cost and remaining
tank in a pile, and
a bullet aimed at a building
(`Shot.Building`) lands in `hurtBuilding` like a shell's blast does. The
robots' loop in `stepSim` skips a robot
that fell earlier in the same tick.

In the play scene `ordering` holds the war factory whose squad the
pointer is ordering; while it is set, `updateOrdering` owns the clicks,
like `laying` does for pipes. The number keys 1-9 reach the same mode
without the card (`updateSquadKeys`): 1 arms the oldest war factory's
squad, the same key again takes it back, and the click that pressed the
key never orders by itself, since arming lands after the update's
inspection. A squad's box at the top right calls it too, by click
(`updateSquadBoxes`, taken with the dev tools' click before the region
sees the pointer). Clicking a pennant on free ground or a target ring
selects the squad and opens its tooltip beside the mark (`pickSquadMark`,
`selectSquad`, `tooltipLayoutForSquad`); it shows the current order and
troop count. The card follows a moving attack ring. A guard pennant on
a building selects that building instead, including its own war factory;
that card also offers `give order`. Clicking a mark alone never arms the
pointer.
The guard pennant's ground cell also selects as though a building occupied
it, so a click there does not open the build menu; the pennant icon opens
the squad card only where no building stands underneath it.
`drawSquadStrip` paints the squads' boxes at the top right.

On the screen (`enemies.go`) the marks lie on the ground under
everything, and the vehicles are drawn after the fog, so a party reads
from far out as a pocket moving through the mist. `compassWord` names a
bearing as the screen shows it, north up. `unit_picking.go` derives each
robot's and mobile rival's screen rectangle from the opaque pixels in its
current sprite frame, origin, scale and facing. Hover and clicks use the
same body hitboxes, with a small screen-space margin; overlapping units
resolve to the one drawn in front. Clicking a robot or mobile rival opens
only that unit's card, which follows its position and closes if it dies.
Squad attack orders use the same vehicle hitboxes, so the target ring and
the attack click agree. City buildings keep cell inspection and the
existing small target around their foot.

### The lifecycle, identity and the local database

The game boots on the **menu** (`menu.go`): the game's name and the player's
number, with Play and Quit before the first save. Once a colony has been
saved, the buttons become Continue, New, Load and Quit:

- **Continue** resumes the most recently saved colony.
- **New** starts the default region from scratch in a new numbered slot;
  every earlier colony remains available. It needs no developer tools.
- **Load** opens the scrollable list in `load.go`: Save 1, Save 2, and so
  on, each with its last save date (local time) and simulated play time.
  Arrows, W/S, the wheel, d-pad or pointer choose a row; Enter, Space,
  gamepad A/Start or a click loads it. Back or Esc returns to the title.

The play scene keeps its slot; autosave and Esc update only that colony.
The first save in a new slot happens on Esc or after `autosaveTicks`
(900, 15 s). A failed load shows an error without replacing the save with
a fresh game. The main menu is the only screen where Esc quits. The
four-button menu and the load list also fit the smaller resized screen.

**Identity** (`identity.go`): the machine says who is playing. Its
system ID — Windows' `MachineGuid`, macOS' `IOPlatformUUID`, Linux'
`/etc/machine-id` — hashed with `playerIDSalt`, is `player`: 64 hex
characters, stable across runs, and never shown or sent in raw form. The
menu shows its first eight as `#30E99076`; the database and a later
server use the whole thing. A machine that won't say who it is gets a
random identity, kept in the database's `machine` table, so it is still
stable from then on. Identity is per machine, not per human: two players
on one computer share a number, and an avatar picker is the later answer.

**The database** (`store.go`) is SQLite through `modernc.org/sqlite`
(pure Go — this project has no C compiler), one file at the player's
settings folder, in `GoLib examples/niebla/`, the same place a GoLib dist
build keeps its saves, so a debug build and a dist one share the base.
The schema is the schema a server keeps, on one machine for now:

- `players` — one row per identity, with `source` (`machine` or
  `random`), `created_at`, and a `token` column that stays empty until a
  server hands one out at first contact. The client is already shaped
  for that moment: resolve, register, then authenticate by identity.
- `saves` — the whole `State` as one JSON value per player and numbered
  slot, with the tick and the time it was written. On opening the database,
  the legacy `region` slot becomes `1` without changing its state. New
  slots use the next number; all stay in the same `niebla.db` file.
- `machine` — key-value for what belongs to this machine alone (the
  fallback identity lives here).

Under `golib shot` and `go test` the database is `:memory:`: shots and
tests never touch the player's base, and `golib shot --save` still
starts a game deep in a state — `resumeState` takes the seeded `state`
value over whatever the database has. The empty screenshot database keeps
Play first, so `Enter@1` still enters that seeded state or a fresh region.
Two caveats: the driver doesn't
build for the browser (`js/wasm`), so a web build of this game will get
its store from a server or the browser's own, not this file; and the
simulation itself never reads the clock — only the saves' timestamps do.

### Entity catalog

`catalog.go` holds one `ThingInfo` per `ThingType`: name, color, the SI unit
of its headline amount, an optional `Summary` (default: `si(amount, unit)`)
and `Details` for the expanded card. A type with no entry gets its name from
the type itself and a color from `stableColor`, a hash of the name through
HSV — the same type always prints in the same color, so new entities are
readable the moment they exist. To dress a type up later, add its catalog
entry; the markup palette picks the color up automatically.

### Text markup

`"[oil]900 L[/]"` prints `900 L` in the oil color. `[name]` switches to the
palette color `name`, `[/]` returns to the color before it, colors nest, and
unknown tags print as they are. The palette holds one entry per thing type
plus `dim`, `light` and `fog`. `drawMarkup` draws span by span, advancing by
`TextWidth` (which counts one letter gap per character, gaps included after
the last), so spans land where one `DrawText` call would put them.

### Units

The world speaks SI: one world unit is one meter (`unitMeters`), so a tile
is 200 m across (4 ha) and the region 5 km from side to side, the core's
monolith is 16 by 4 m and 36 m tall, and its bubble radius is 800 m. Oil is
liters, lilac is kilograms (`si` turns 12000 kg into `12.0 t`, so nobody
ever reads `kkg`). A deposit holds its cells' richness times the ore's
density (`oilPerRichCell` 70 L, `lilacPerRichCell` 240 kg, in
`worldgen.go`): about 3 kL and 10 t by the core, up to three times that
far out.
Robots are fast rovers with small arms (30 m/s, 30 L or 20 kg a trip), so
a worked deposit shows a constant coming and going. Amounts live at the
top of `things.go`.

### Blender mobile units

Each mobile chassis has an editable `.blend` in `sources/models/`, an
eight-frame transparent PNG in `assets/sprites/`, and an eight-frame shadow
mask in `assets/sprites/shadows/`. The crawler chassis has a second,
red-gray city-construction variant. `studio.py` supplies
the geometry primitives, material setup, light, isometric camera, direction
conversion, `render_sheet()` and `render_shadow_sheet()`. The shadow mask is
isolated from an EEVEE render of the model over a matte floor, by comparing
the floor with and without the shadow-casting daylight; the model's own
sprite alpha removes its silhouette from the mask. It is tinted and drawn
under the unit, not saved in game state.
`artillery.py` holds only the artillery model; `other_units.py` holds the
eight other models, including `worker-mechanic`: an armored service rover
with an amber tool deck and raised crane, without a weapon. After editing
a `.blend` in Blender, render it from the project root with its matching
script:

```text
blender -b examples/niebla/sources/models/artillery.blend \
  -P examples/niebla/sources/models/artillery.py
blender -b examples/niebla/sources/models/worker-core.blend \
  -P examples/niebla/sources/models/other_units.py
blender -b examples/niebla/sources/models/worker-mechanic.blend \
  -P examples/niebla/sources/models/other_units.py
blender -b examples/niebla/sources/models/rival-city-crawler.blend \
  -P examples/niebla/sources/models/other_units.py
```

Those commands refresh both the model and shadow sheets. To refresh only a
shadow mask while preserving its existing model PNG, pass `--shadows-only`:

```text
blender -b examples/niebla/sources/models/worker-core.blend \
  -P examples/niebla/sources/models/other_units.py \
  -- --shadows-only
blender -b examples/niebla/sources/models/artillery.blend \
  -P examples/niebla/sources/models/artillery.py \
  -- --shadows-only
```

To **discard manual edits** and recreate the `.blend` and PNG from Python,
use `--create`. This generates all eight other models at once, or only the
named model if you pass its basename:

```text
blender -b -P examples/niebla/sources/models/artillery.py \
  -- --create
blender -b -P examples/niebla/sources/models/other_units.py \
  -- --create
blender -b -P examples/niebla/sources/models/other_units.py \
  -- --create worker-core
blender -b -P examples/niebla/sources/models/other_units.py \
  -- --create worker-mechanic
blender -b -P examples/niebla/sources/models/other_units.py \
  -- --create rival-city-crawler
```

The shared camera matches the ground's 2:1 projection at 30 degrees;
each 128x192 frame puts its foot at (64, 160). The eight *screen* headings
become world yaws before rendering at reference zoom 32. `worldsprites.go`
scales both the model and its shadow at every zoom, down to a readable icon
minimum, and draws them in screen pixels after projecting the ground point:
GoLib rounds sprites before camera zoom, which otherwise caused 32-pixel
jumps. Portraits and squad boxes reuse the model PNGs at UI size, without
their world shadows. Building and shipping the game need neither Blender
nor the `.blend` files.

### Deposit patches

A vein is one thing however many tiles it reaches into. The generator
(`worldgen.go`) grows each deposit as a field of richness over cells,
1 at its heart and thinning out to specks at its rim, and keeps the
tiles that hold enough of it: `land.deposits` and `land.bodies` (static
data of the seed, never state), and `depositAt` maps any tile to its
deposit. `State.Drain` holds one entry per deposit, keyed by its heart's
tile. Every worker assigned from any patch tile shares that amount and
loads beside the heart (`postSpot`), with a slight offset to keep small
groups of robots visible. A patch remains one deposit and one card regardless
of how many workers it has; the state keeps no per-robot copy of its ore.
`ground.go` paints each deposit cell by cell, and a worked one wears from
the rim in.

### The generated region

`State.Seed` names the region and `generateRegion` makes it, the same on
every machine: relief, cover and deposits. `land` is the region of the
seed in hand; `newGameOn`, `State.enterRegion` (a save coming in) and
`Apply` keep it the state's own through `useRegion`. The relief is
decided by wave function collapse over blocks of four cells and lives
on the cells' corners, a level (4 m) at a time and never more than a
level to the cell; `flatCell` is what `canPlace` asks of a cell.
`project` lifts what stands on the ground by `heightAt`, `projectFlat`
is for the fog and the bubbles, and `unproject` finds the ground under
the pointer. DESIGN.md's Tuning has the numbers; `worldgen_test.go`
checks the laws over 40 seeds.

### Cell picking

The cell is the unit the player picks and counts by: the ground keeps
its tiles on the screen, but a click selects the cell under it
(`cellAtWorld`), the outline is the cell's (`cellDiamond`) and the panel
lists what stands on that cell alone (`thingsAt`) - its building, site or
pile and the robots on it -, so a silo's panel never shows the charger
beside it. Clicking a pump's visible body picks its own cell, and its
card replaces the deposit card there; the deposit remains one functional
patch, but its pump and site are not selectable from the other cells.
Deposits still show their whole card from any of their cells, as does the
core from any cell of its pad. A builder or worker assigned to a deposit
is listed there even while physically away from it. Clicking its portrait
focuses that unit's card at the deposit's panel anchor; `pickedRobot` is view
state, and the card is built from the robot ID instead of the robots
currently on the selected cell. Deposits stay tile-shaped, so the scene
hands their actions the cell's tile (`cellTile`).

`tileAtWorld` undoes `project`: a tile's diamond on the screen is the square
`[col, col+1) x [row, row+1)` in tiles, so the inverse is exact, and a point
maps to its tile with two `math.Floor` calls; `cellAtWorld` does the same
onto cells. The inspection panel anchors on the cell's projected middle
through `camera.ToScreen`, so it follows the cell while the view pans or
zooms, and flips to the cell's left when it would leave the screen.

### Inspection panel

One geometry, two users: `tooltipLayout` builds the row list, and `Update`
hit-tests it (`contains`, `trashAt`, `cardAt`, `buttonAt`) while `Draw`
paints it. A building's or a site's title row ends in a trash can
(`trashFor`; none on the core, dimmed on a protector that can't go): the
first press arms it — the scene's `armed` holds the card's ID and
`tooltip.arm` paints it red under `demolish?` —, the second applies
`Demolish` or `CancelJob`, and any other click disarms. A site and a
pile have cards of their own (`siteThing`, `pileThing`).
Cards start open on their own: a cell's primary thing (deposits, the
core, buildings — `Primary` in the catalog) and, on a cell with a single
thing, that thing. A click on a title folds or opens from where the card
stands (`cardOpen` gives the default, the scene's `expanded` map stores
the click). Clicking a cell selects it — unless it is free buildable
ground, which opens the build menu instead. An expanded deposit card
  offers `send robot` while an unassigned worker exists, and lists
  assigned units as clickable portraits, eight per page. Clicking a
  portrait opens that unit's card without moving the panel from the
  deposit; `recall robot` clears only its post, and `back to deposit`
  returns to the list. The factory card offers `build builder` and
  `build worker`. The
panel wins over what sits under it, so buttons work even where it covers
buildable ground; a right click that never moved more than 4 px deselects
(a drag is a pan, not a cancel). The
camera rests centered on world point (640, 372) — screen and world
differ by (0, -12) at rest zoom; click targets in scripted shots aim
at a tile's center (`projectTile` + half a tile), never its corner, whose
tile depends on float rounding.

### Colony robot roster

The button below the military squad strip opens a fixed panel on the
right. It groups builders, unassigned workers, workers by deposit and
mechanics; each row shows a model, ID and the task it is doing now. The
button or X closes the panel. Clicking a row selects it. For a builder or
worker, `assign deposit` arms the pointer; clicking an oil pool or lilac
vein sends that exact unit, and right-click cancels. `recall` clears its
post while any carried load is delivered first. A builder assigned to a
deposit remains a builder: it resumes construction and pipe work according
to its normal priority, and has only a third of a worker's carrying
capacity. The roster, page and selection are scene state; assignment uses
the deterministic `AssignRobot` action.

## Testing

`region_test.go` pins the layout's placement rules, the deposit patches
(shape and spread), the projection's round trip, the things a cell holds,
the SI formatter and the catalog's stability. `markup_test.go` covers the
parser (nesting, unknown tags, unclosed color), the tooltip layout's rows,
portrait selection and pagination, and the remote robot card and buttons.
`robots_panel_test.go` covers the global groups, pages and hit areas.
`world_test.go` drives the simulation with no window: the starting builder,
the idle ranks by the core, haul conservation (store + patch remaining =
patch full), workers sharing a deposit, explicit individual assignment,
worker-only automatic assignment, builder load size and migration,
build-job and protector-job priority, dry release, replay
determinism and the JSON
round trip. `buildings_test.go` does the same for the buildings slice: a
marking pays and its building rises, the fog refuses ground but not a
protector, one tile fits several buildings on its cells, a cell with a
job pending takes no second job, the factory queues and rolls out tanked
robots, a built robot refuels before it runs dry, the fog digests a dry
one outside the bubbles and leaves a quarter of its cost, cargo and tank
oil, deep fog
halves every walker's pace and a
protector's pocket cancels it, and full stores hold the cargo until a
silo opens room. `piles_test.go` pins the demolition: the cost falls as
a pile and comes home whole, a factory's pending unit (trooper or mechanic)
is cancelled and refunded,
a silo spills what loses its roof and the oil waits for room with nobody
holding it, a cancelled site drops its cost, a protector stays while it
alone shelters a building, a load goes to the nearest store of its kind,
the core one among them, a cell's panel leaves out the cell beside it,
piles survive a save (and a save from before them takes one), and the
cards carry their trash cans. `fog_test.go` does the same for the fog slice: the
cycles tick, the first swell rises on schedule and drains whole, the
line presses in and never reaches the bubble, the pushed band drags
more, stationary wear starts after two seconds and scales from haze to
deep fog, hull and tank damage are tested in haze and swell, and deep fog
digests a stationary builder by its hull. Movement and bubbles stop wear,
saves resume it, and the HUD forecasts.
`mites_test.go` pins the fog's mites, which are view but need no
window: their count follows the body's volume and exposure, half strength
in the haze and none under a bubble; they sit on a robot that stands still
and trail one that walks; they fade over a host that left the state; and
the layers they are drawn with stack into `miteFalloff`.
`enemies_test.go` pins the rivals' introduction: the scout arrives on
time, steals, leaves its mark and goes; the follow-up arrives after one
minute on the same bearing as the city begins building; construction
attacks repeat after a one-minute gap and grow by one raider per visit; the
raid camps, steals and leaves;
guard-post fire spends oil and wrecks drop loot; and a crawlerless intro
party is lost to the fog. It also pins deterministic replay, JSON saves
and a save from before the rivals. `cities_test.go` pins city buildings
and finite economy, antimist-led sorties that grow to artillery at the
five-vehicle cap, one-minute rebuilds,
unloading and immediate reuse of intact squads, development actions,
deterministic replay and JSON persistence.
`orientation_test.go` pins the eight screen octants, moving and stationary
facing for workers and rival vehicles, and the JSON round trip plus the
right-facing default for older saves.
`noRivals` keeps them out of a test that is about something else.
`shots_test.go` pins the shots and repair rules: a bullet is in the air
before it hurts; ordinary workers never mend buildings; a mechanic repairs
at a cost in oil but cannot out-repair continuing artillery fire; rival
bullets and shells can damage and kill it, leaving a wreck; and a war
factory in an old save still finishes its pending trooper. It also pins
that colony and rival structures emit transient collapse events, debris
merges with nearby projectile impacts, and manual demolition is quieter.
`audio_test.go` pins the collapse sound's distance and dismantling levels.
`shotstate_test.go` can write `NIEBLA_BUILDING_COLLAPSE_SHOT_STATE` for a
visual capture. The combat tests also pin
that a settled Nexus has no passive gun, the colony's artillery holds its
fire at a Nexus nobody sees, fires once a spotter stands within sight,
pays its shells, brings the Nexus down after shell flight and sends the
garrison away; and a shell misses who moved on. `squads_test.go` pins the
trooper squad and mechanic limits: the war factory charges for each unit,
rolls a trooper out whole into its squad and stops at `squadSize`, and a
mechanic stays outside the squad with one allowed per factory. Troopers
touch no job, pile or post; a squad walks to its guard spot and heals
under a bubble; enemy guns can target a mechanic but not a worker; a
squad sent after a camped crawler brings it down with every raider still
standing, burns its tanks and gets shot at, and returns to its door when
the party is gone. A lone trooper falls and leaves 25% of its cost and
remaining tank in its wreck, and a demolished war factory's trooper
rests by the core.
`TestTheNumberKeysCallTheWarFactoriesOldestFirst` pins the keys' order
(`squadSlots`), `TestASquadsMarkPicksItsSquad` the pennant's and the
ring's hit-testing (`squadMarkAt`), and
`TestSquadMarkOpensItsCardAndTheCardArmsOrdering` the mark selection,
card button and ordering mode; `TestSquadCardFollowsItsAttackRing` keeps
the card with a moving attack target. `TestTheSquadsBoxesLieApartAndPickTheirSquad`
pins the boxes' geometry (`squadBoxRect`, `squadBoxAt`). `shotstate_test.go` holds
`TestWriteShotSquadState`: with `NIEBLA_SHOT_STATE` naming a file, it
writes a region with two war factories and their squads in the shape
`golib shot --save` reads, and prints the first squad's mark and button
coordinates for a shot of the card and the armed ordering pointer:

```text
NIEBLA_SHOT_STATE=../../build/niebla/squad-tooltip.json \
  ./golib go -C examples/niebla test \
  -run TestWriteShotSquadState -v
./golib shot niebla 50 60 \
  --save build/niebla/squad-tooltip.json \
  --input "Enter@1 Mouse@45:586,354 MouseLeft@46 \
  Mouse@55:680,440 MouseLeft@56"
```

`TestWriteShotUnitState` puts one of each colony chassis by the core
for visual checks and prints the worker's screen position (it writes only
when requested). The first shot shows its hover rectangle; the second
clicks it, then zooms in to inspect the individual card and its outline:

```text
NIEBLA_UNIT_SHOT_STATE=../../build/niebla/units.json \
  ./golib go -C examples/niebla test \
  -run TestWriteShotUnitState -v
./golib shot niebla 2 --save build/niebla/units.json \
  --input "Enter@1 Mouse@2:618,347"
./golib shot niebla 60 --save build/niebla/units.json \
  --input "Enter@1 Mouse@2:618,347 MouseLeft@3 MouseWheel@4:5"
```

`TestWriteShotVehicleState` places the rival vehicle types, including
mobile artillery at several headings, near the core for a close view:

```text
NIEBLA_VEHICLE_SHOT_STATE=../../build/niebla/vehicles.json \
  ./golib go -C examples/niebla test -run TestWriteShotVehicleState
./golib shot niebla 60 --save build/niebla/vehicles.json \
  --input "Enter@1 Mouse@2:640,357 MouseWheel@3:4"
```

`TestWriteUnitDeathShotState` starts with a mechanic and a mobile artillery
unit dying on the first simulation tick, to compare their explosion sizes:

```text
NIEBLA_UNIT_DEATH_SHOT_STATE=../../build/niebla/unit-deaths.json \
  ./golib go -C examples/niebla test \
  -run TestWriteUnitDeathShotState
./golib shot niebla 2 20 60 \
  --save build/niebla/unit-deaths.json \
  --input "Enter@1 Mouse@2:640,360 MouseWheel@3:2"
```

`TestWriteShotMovingArtilleryState` sets a vehicle driving past the core
so frames a tick apart reveal whether its movement snaps to world pixels:

```text
NIEBLA_MOVING_ARTILLERY_SHOT_STATE=../../build/niebla/moving-artillery.json \
  ./golib go -C examples/niebla test -run TestWriteShotMovingArtilleryState
./golib shot niebla 60 61 62 63 \
  --save build/niebla/moving-artillery.json \
  --input "Enter@1 Mouse@2:640,357 MouseWheel@3:5"
```

`TestWriteShellTrailShotState` writes a shell just leaving an artillery
piece, for checking the muzzle, the growing trail and its smoke:

```text
NIEBLA_SHELL_SHOT_STATE=../../build/niebla/shell.json \
  ./golib go -C examples/niebla test -run TestWriteShellTrailShotState
./golib shot niebla 1 8 16 36 \
  --save build/niebla/shell.json \
  --input "Enter@1 Mouse@2:640,357 MouseWheel@3:4"
```

`identity_test.go` pins the identity: stable for a machine, distinct
between machines, 64 hex characters, and the three parsers of what the
systems report. `store_test.go` pins the database: an identity (and a
fallback one) kept across runs, a fresh player's token waiting empty, a
base saved and loaded back whole, a second save replacing the first,
one player's save invisible to another, and the DB path's rules
(`:memory:` under `golib shot`, the settings folder otherwise, a
missing one is an error).
`economy_test.go` is an opt-in probe, not a test that asserts a current
balance. With `NIEBLA_ECONOMY_REPORT` it runs four opening policies for one
hour on seeds 0, 1 and 2, and writes a CSV row each game minute: spendable
and protector oil, lilac, extraction, workers, troopers, buildings, swells,
visits, party composition, city construction and sorties. It uses only
schematics that have arrived and drives the same `Apply` actions as the
simulation. A rate or opening change is therefore measured in the real
simulation rather than a second, approximate model. `DESIGN.md` records the
results and the next balance decisions.
Visual checks are shots with scripted clicks; see the
command above.
