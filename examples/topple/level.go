package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"golib"
)

// Levels are JSON files in assets/levels/, played in the order of their
// names: level01.json, level02.json, and so on. Each places the platforms,
// the stacks of crates (C) and barrels (B) on them, the stone blocks and
// the loose crates on the ground, in meters, on a yard that runs from -10
// to 10 across X and Z. Open one in any text editor to change it.

const levelFolder = "levels"

// levelFile is a level as its JSON file has it.
type levelFile struct {
	Name      string      `json:"name"`
	Par       int         `json:"par"`   // balls a good player needs
	Start     spot        `json:"start"` // where the player stands
	Focus     spot        `json:"focus"` // what the camera keeps in view
	Platforms []platform  `json:"platforms"`
	Stones    []stone     `json:"stones"`
	Loose     []looseProp `json:"loose"`
}

// spot is a point on the ground.
type spot struct {
	X float32 `json:"x"`
	Z float32 `json:"z"`
}

// platform is a solid block on the ground with stacks on its top.
type platform struct {
	X      float32 `json:"x"` // the middle of its footprint
	Z      float32 `json:"z"`
	Width  float32 `json:"width"`  // along X
	Height float32 `json:"height"` // its top above the ground
	Depth  float32 `json:"depth"`  // along Z
	Stacks []stack `json:"stacks"`
}

// stack is a pile of crates and barrels on a platform: each layer a row of
// letters, from the bottom up, centered on the stack's place.
type stack struct {
	X      float32  `json:"x"` // from the platform's middle
	Z      float32  `json:"z"`
	Along  string   `json:"along"` // "x" or "z": which way the rows run
	Layers []string `json:"layers"`
}

// stone is a block of stone too heavy to move, standing on the ground.
type stone struct {
	X      float32 `json:"x"`
	Z      float32 `json:"z"`
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
	Depth  float32 `json:"depth"`
}

// looseProp is a crate or barrel on the ground, to push out of the way.
type looseProp struct {
	X    float32 `json:"x"`
	Z    float32 `json:"z"`
	Kind string  `json:"kind"` // "C" or "B"
}

// propKind says what a prop is.
type propKind int

const (
	crateKind propKind = iota
	barrelKind
)

// placedProp is a crate or barrel where it starts: the middle of its body.
type placedProp struct {
	kind     propKind
	position golib.Vector3
	target   bool // on a platform: it has to come down
}

// level is a level ready to build: its props placed.
type level struct {
	levelFile
	props []placedProp
}

// sizeOf is how big a prop of kind is, each way.
func sizeOf(kind propKind) golib.Vector3 {
	if kind == barrelKind {
		return golib.Vector3{X: barrelWidth, Y: barrelHeight, Z: barrelWidth}
	}
	return golib.Vector3{X: crateSize, Y: crateSize, Z: crateSize}
}

// kindOf reads a prop's letter.
func kindOf(letter rune) (propKind, error) {
	switch letter {
	case 'C', 'c':
		return crateKind, nil
	case 'B', 'b':
		return barrelKind, nil
	}
	return 0, fmt.Errorf("%q is not a crate (C) or a barrel (B)", letter)
}

// parseLevel reads a level's JSON and places its props.
func parseLevel(data []byte) (*level, error) {
	var f levelFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Par <= 0 {
		f.Par = 3
	}
	l := &level{levelFile: f}
	for pi, p := range f.Platforms {
		if p.Width <= 0 || p.Height <= 0 || p.Depth <= 0 {
			return nil, fmt.Errorf("platform %d has no size", pi+1)
		}
		for si, s := range p.Stacks {
			base := p.Height
			for li, layer := range s.Layers {
				letters := []rune(strings.TrimSpace(layer))
				var tallest float32
				for _, r := range letters {
					kind, err := kindOf(r)
					if err != nil {
						return nil, fmt.Errorf("platform %d, stack %d, layer %d: %v", pi+1, si+1, li+1, err)
					}
					tallest = max(tallest, sizeOf(kind).Y)
				}
				for i, r := range letters {
					kind, _ := kindOf(r)
					offset := (float32(i) - float32(len(letters)-1)/2) * propSpacing
					at := golib.Vector3{X: p.X + s.X, Y: base + sizeOf(kind).Y/2 + propGap, Z: p.Z + s.Z}
					if s.Along == "z" {
						at.Z += offset
					} else {
						at.X += offset
					}
					l.props = append(l.props, placedProp{kind: kind, position: at, target: true})
				}
				base += tallest + propGap
			}
		}
	}
	for i, p := range f.Loose {
		kind, err := kindOf([]rune(p.Kind + " ")[0])
		if err != nil {
			return nil, fmt.Errorf("loose prop %d: %v", i+1, err)
		}
		l.props = append(l.props, placedProp{kind: kind, position: golib.Vector3{X: p.X, Y: sizeOf(kind).Y/2 + propGap, Z: p.Z}})
	}
	if l.targets() == 0 {
		return nil, fmt.Errorf("no crates or barrels on the platforms")
	}
	return l, nil
}

// targets is how many props start on the platforms.
func (l *level) targets() int {
	n := 0
	for _, p := range l.props {
		if p.target {
			n++
		}
	}
	return n
}

// loadLevels reads every level in assets/levels/, in order.
func loadLevels() ([]*level, error) {
	names, err := golib.ListAssets(levelFolder)
	if err != nil {
		return nil, err
	}
	var levels []*level
	for _, name := range names {
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := golib.ReadAsset(name)
		if err != nil {
			return nil, err
		}
		l, err := parseLevel(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		levels = append(levels, l)
	}
	if len(levels) == 0 {
		return nil, fmt.Errorf("no levels in assets/%s", levelFolder)
	}
	return levels, nil
}
