package homeplan

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed presets/*.json
var presetFS embed.FS

type Slot struct {
	Room string  `json:"room"`
	X    float32 `json:"x"`
	Y    float32 `json:"y"`
	Z    float32 `json:"z"`
}

type Floor struct {
	R   [4]float32 `json:"r"`
	Z   float32    `json:"z,omitempty"`
	Rm  string     `json:"room"`
	C   [3]uint8   `json:"c"`
	Pat string     `json:"pat,omitempty"`
}

type Wall struct {
	B    [6]float32 `json:"b"`
	Rm   string     `json:"room"`
	Side string     `json:"side"`
	Up   bool       `json:"upper,omitempty"`
}

type Window struct {
	Side string  `json:"side"`
	A    float32 `json:"a"`
	B    float32 `json:"b"`
	Z0   float32 `json:"z0"`
	Z1   float32 `json:"z1"`
}

type Part struct {
	T      string       `json:"t"`
	B      []float32    `json:"b,omitempty"`
	P      []float32    `json:"p,omitempty"`
	A      []float32    `json:"a,omitempty"`
	Pts    [][3]float32 `json:"pts,omitempty"`
	R      float32      `json:"r,omitempty"`
	H      float32      `json:"h,omitempty"`
	W      float32      `json:"w,omitempty"`
	Rm     string       `json:"room,omitempty"`
	Rooms  []string     `json:"rooms,omitempty"`
	C      [3]uint8     `json:"c,omitempty"`
	Sh     float32      `json:"sh,omitempty"`
	F      []float32    `json:"f,omitempty"`
	Top    *[3]uint8    `json:"top,omitempty"`
	Left   *[3]uint8    `json:"left,omitempty"`
	Right  *[3]uint8    `json:"right,omitempty"`
	TopG   bool         `json:"topg,omitempty"`
	Screen *[3]uint8    `json:"screen,omitempty"`
}

type Item struct {
	K  [4]float32 `json:"k"`
	Up bool       `json:"upper,omitempty"`
	P  []Part     `json:"p"`
}

type Plan struct {
	Name     string              `json:"name"`
	Size     [3]float32          `json:"size"`
	Upper    string              `json:"upper,omitempty"`
	Words    map[string][]string `json:"words,omitempty"`
	Slots    []Slot              `json:"slots"`
	Floors   []Floor             `json:"floors"`
	Walls    []Wall              `json:"walls"`
	Windows  []Window            `json:"windows,omitempty"`
	Items    []Item              `json:"items"`
	Rooms    []string            `json:"-"`
	Fallback string              `json:"-"`
}

var defaultWords = map[string][]string{
	"loft":    {"loft", "attic", "upstairs", "mezzanine"},
	"bed":     {"bed", "master", "nursery"},
	"bath":    {"bath", "toilet", "wc", "shower"},
	"kitchen": {"kitchen", "cook", "pantry"},
	"dining":  {"dining", "dinner"},
	"hall":    {"hall", "corridor", "entry", "foyer", "stair"},
	"living":  {"living", "lounge", "lamp", "overhead", "fan", "tv", "couch", "sofa", "tree"},
}

var wordOrder = []string{"loft", "bed", "bath", "kitchen", "dining", "hall", "living"}

func Parse(b []byte) (*Plan, error) {
	var p Plan
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.Size[0] < 2 || p.Size[1] < 2 || p.Size[2] < 0.5 || p.Size[0] > 40 || p.Size[1] > 40 || p.Size[2] > 6 {
		return nil, errors.New("size must be [width, depth, wall height] with sensible values")
	}
	if len(p.Slots) == 0 || len(p.Slots) > 64 {
		return nil, errors.New("slots must hold between 1 and 64 light spots")
	}
	if len(p.Floors) > 64 || len(p.Walls) > 64 || len(p.Windows) > 64 || len(p.Items) > 300 {
		return nil, errors.New("plan is too large")
	}
	seen := map[string]bool{}
	var rooms []string
	add := func(r string) {
		if r != "" && !seen[r] {
			seen[r] = true
			rooms = append(rooms, r)
		}
	}
	for _, s := range p.Slots {
		if s.Room == "" {
			return nil, errors.New("every slot needs a room")
		}
		add(s.Room)
	}
	for _, f := range p.Floors {
		add(f.Rm)
	}
	for _, w := range p.Walls {
		if w.Side != "L" && w.Side != "R" {
			return nil, errors.New("wall side must be L or R")
		}
		add(w.Rm)
	}
	for i, it := range p.Items {
		n := 0
		for _, pt := range it.P {
			n++
			switch pt.T {
			case "solid", "box":
				if len(pt.B) != 6 {
					return nil, fmt.Errorf("item %d: %s needs b with 6 numbers", i, pt.T)
				}
			case "part":
				if len(pt.B) != 4 {
					return nil, fmt.Errorf("item %d: part needs b with 4 numbers", i)
				}
			case "disc":
				if len(pt.P) != 3 || pt.R <= 0 {
					return nil, fmt.Errorf("item %d: disc needs p [x,y,z] and r", i)
				}
			case "post":
				if len(pt.P) != 3 || pt.R <= 0 || pt.H <= 0 {
					return nil, fmt.Errorf("item %d: post needs p, r and h", i)
				}
			case "quad":
				if len(pt.Pts) != 4 {
					return nil, fmt.Errorf("item %d: quad needs 4 points", i)
				}
			case "line":
				if len(pt.A) != 3 || len(pt.B) != 3 {
					return nil, fmt.Errorf("item %d: line needs a and b with 3 numbers", i)
				}
			default:
				return nil, fmt.Errorf("item %d: unknown type %q", i, pt.T)
			}
			add(pt.Rm)
		}
		if n == 0 || n > 64 {
			return nil, fmt.Errorf("item %d must contain between 1 and 64 parts", i)
		}
	}
	p.Rooms = rooms
	if p.Words == nil {
		p.Words = map[string][]string{}
	}
	for k, v := range defaultWords {
		if _, ok := p.Words[k]; !ok && seen[k] {
			p.Words[k] = v
		}
	}
	return &p, nil
}

func (p *Plan) Tag(s string) string {
	s = strings.ToLower(s)
	keys := make([]string, 0, len(p.Words))
	for k := range p.Words {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return order(keys[i]) < order(keys[j]) || (order(keys[i]) == order(keys[j]) && keys[i] < keys[j])
	})
	for _, k := range keys {
		for _, w := range p.Words[k] {
			if w != "" && strings.Contains(s, strings.ToLower(w)) {
				return k
			}
		}
	}
	return ""
}

func order(k string) int {
	for i, o := range wordOrder {
		if o == k {
			return i
		}
	}
	return len(wordOrder)
}

func Presets() []string {
	ents, _ := presetFS.ReadDir("presets")
	var out []string
	for _, e := range ents {
		out = append(out, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(out)
	return out
}

func Preset(name string) (*Plan, error) {
	b, err := presetFS.ReadFile("presets/" + name + ".json")
	if err != nil {
		return nil, errors.New("no such home design: " + name)
	}
	return Parse(b)
}

func CustomPath(dir string) string { return filepath.Join(dir, "house.json") }

func Load(name, dir string) *Plan {
	if name == "custom" {
		if b, err := os.ReadFile(CustomPath(dir)); err == nil {
			if p, err := Parse(b); err == nil {
				return p
			}
		}
	}
	if p, err := Preset(name); err == nil {
		return p
	}
	p, _ := Preset("twostory")
	return p
}
