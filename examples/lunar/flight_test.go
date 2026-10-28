package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"golib"
)

// An optional replay artifact uses the same keyboard controls as a player.
// golib go -C examples/lunar test -run TestContractRoutes -args -pilot-script=../../build/lunar/pilot-input.txt
var pilotScript = flag.String("pilot-script", "", "write the first contract's keyboard replay")

func pilot(w *world, burn *bool) controls {
	dx := w.pad().Center().X - w.pos.X
	desiredVX := golib.Clamp(dx*0.35, -34, 34)
	ax := golib.Clamp((desiredVX-w.vel.X)*1.1, -18, 18)
	height := w.pad().Y - footY - w.pos.Y
	desiredVY := golib.Clamp(height*0.35, 5, 34)
	if abs(dx) > 55 {
		desiredVY = min(desiredVY, 5)
	}
	desiredAngle := float32(math.Atan2(float64(ax), float64(regions[w.contract.Region].Gravity))) * 180 / math.Pi
	desiredAngle = golib.Clamp(desiredAngle, -42, 42)
	c := controls{}
	predicted := w.angle + w.angular*0.14
	if predicted < desiredAngle-1 {
		c.turn = 1
	}
	if predicted > desiredAngle+1 {
		c.turn = -1
	}
	if w.vel.Y > desiredVY+1 {
		*burn = true
	}
	if w.vel.Y < desiredVY-1 {
		*burn = false
	}
	if *burn {
		c.thrust = 1
	}
	if height < 75 && abs(dx) < 30 {
		c.stabilize = true
	}
	return c
}

func TestContractRoutes(t *testing.T) {
	if err := loadRegions(); err != nil {
		t.Fatal(err)
	}
	for r := range regions {
		for k := 0; k < 3; k++ {
			t.Run(fmt.Sprintf("region%d/cargo%d", r, k), func(t *testing.T) {
				w := newWorld(newCampaign(), contract{Region: r, Kind: k})
				burn := false
				var inputs []controls
				for frame := 0; frame < 60*150 && w.state == flying; frame++ {
					c := pilot(w, &burn)
					inputs = append(inputs, c)
					w.step(c, dt)
				}
				if w.state != landed {
					t.Fatalf("route failed at %.1fs, pos=%v vel=%v fuel=%.1f angle=%.1f: %s", w.elapsed, w.pos, w.vel, w.fuel, w.angle, w.reason)
				}
				if w.fuel <= 0 {
					t.Fatal("route requires more fuel than the starter ship carries")
				}
				t.Logf("landed in %.1fs with %.0f%% fuel and score %d", w.elapsed, w.fuel/w.spec.Fuel*100, w.landingScore)
				if r == 0 && k == 0 && *pilotScript != "" {
					if err := os.WriteFile(*pilotScript, []byte(replay(inputs)), 0644); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
func replay(frames []controls) string {
	// Enter the hub on frame 1, launch its selected contract on frame 3.
	tokens := []string{"Enter@1", "Enter@3"}
	for _, key := range []string{"A", "D", "W", "S"} {
		start := 0
		for i := 0; i <= len(frames); i++ {
			on := false
			if i < len(frames) {
				c := frames[i]
				on = (key == "A" && c.turn < 0) || (key == "D" && c.turn > 0) || (key == "W" && c.thrust > 0) || (key == "S" && c.stabilize)
			}
			if on && start == 0 {
				start = i + 4
			}
			if !on && start != 0 {
				tokens = append(tokens, fmt.Sprintf("%s@%d-%d", key, start, i+3))
				start = 0
			}
		}
	}
	return strings.Join(tokens, " ")
}

func TestCampaignSaveRoundTrip(t *testing.T) {
	if err := golib.DeleteData("campaign"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = golib.DeleteData("campaign") })
	p := newCampaign()
	p.Credits = 8000
	p.Deliveries = 9
	p.buyShip(1)
	p.buyUpgrade(2)
	p.buyBase(2)
	p.Muted = true
	if err := golib.SaveData("campaign", p); err != nil {
		t.Fatal(err)
	}
	var got campaign
	found, err := golib.LoadData("campaign", &got)
	if err != nil || !found || got != p {
		t.Fatalf("progress changed across a save: %v, %v", found, err)
	}
}
func TestMusicAndMaps(t *testing.T) {
	if err := theme.Err(); err != nil {
		t.Fatal(err)
	}
	if err := loadRegions(); err != nil {
		t.Fatal(err)
	}
	for i, l := range landscapes {
		for j, p := range l.pads {
			for _, x := range []float32{p.X, p.Center().X, p.X + p.Width} {
				if abs(l.height(x)-p.Y) > 0.01 {
					t.Fatalf("region %d pad %d is not flat on the terrain", i, j)
				}
			}
			if l.spawns[j].Y >= l.height(l.spawns[j].X)-100 {
				t.Fatalf("region %d spawn %d intersects terrain", i, j)
			}
		}
	}
}
func TestFailureAndLateDelivery(t *testing.T) {
	w := testWorld(t)
	p := newCampaign()
	w.fail("test impact")
	before := p.Credits
	p.settle(w)
	if p.Credits != before || p.Deliveries != 0 || p.Crashes != 1 {
		t.Fatal("insurance did not preserve credits")
	}
	w = testWorld(t)
	w.contract.Kind = 2
	w.elapsed = w.contract.deadline() + 1
	w.state = landed
	r := p.settle(w)
	if r.Express != 0 || r.Contract == 0 {
		t.Fatal("late delivery should lose only the time bonus")
	}
}

func TestUpgradedFleetCanLand(t *testing.T) {
	if err := loadRegions(); err != nil {
		t.Fatal(err)
	}
	for model := range ships {
		for _, level := range []int{0, 4} {
			for region := range regions {
				p := newCampaign()
				p.Selected = model
				p.Owned[model] = true
				p.Upgrades[model] = [4]int{level, level, level, level}
				w := newWorld(p, contract{Region: region, Kind: 1})
				burning := false
				for frame := 0; frame < 60*150 && w.state == flying; frame++ {
					w.step(pilot(w, &burning), dt)
				}
				if w.state != landed {
					t.Errorf("%s level %d in region %d: %s", ships[model].Name, level, region, w.reason)
				}
			}
		}
	}
}

func TestPadEdgeIsNotAValidLanding(t *testing.T) {
	w := testWorld(t)
	p := w.pad()
	w.pos = golib.Vector2{X: p.X + 4, Y: p.Y - footY}
	w.vel.Y = 5
	w.step(controls{}, dt)
	if w.state != crashed {
		t.Fatal("landing without both feet on the pad was accepted")
	}
}

func TestUpgradeAndColonyCaps(t *testing.T) {
	p := newCampaign()
	p.Credits = 100000
	p.Deliveries = 14
	for range 4 {
		if !p.buyUpgrade(0) {
			t.Fatal("upgrade below cap refused")
		}
	}
	before := p.Credits
	if p.buyUpgrade(0) || p.Credits != before {
		t.Fatal("upgrade beyond cap charged credits")
	}
	for range 3 {
		if !p.buyBase(0) {
			t.Fatal("colony below cap refused")
		}
	}
	before = p.Credits
	if p.buyBase(0) || p.Credits != before {
		t.Fatal("colony beyond cap charged credits")
	}
}
