package panel

import (
	"fmt"

	"moreno/tuyatray/homebase"
)

type groupBE struct {
	Backend
	ids []string
}

func (g groupBE) SetPower(_ string, on bool) {
	for _, id := range g.ids {
		g.Backend.SetPower(id, on)
	}
}

func (g groupBE) SetBright(_ string, v int) {
	for _, id := range g.ids {
		g.Backend.SetBright(id, v)
	}
}

func (g groupBE) SetTemp(_ string, v int) {
	for _, id := range g.ids {
		g.Backend.SetTemp(id, v)
	}
}

func (g groupBE) SetColour(_ string, h, s int) {
	for _, id := range g.ids {
		g.Backend.SetColour(id, h, s)
	}
}

func (g groupBE) SetWhite(string) {
	for _, id := range g.ids {
		g.Backend.SetWhite(id)
	}
}

func (g groupBE) SetEffect(_, name string) {
	for _, d := range g.devs() {
		if d.HasScene {
			g.Backend.SetEffect(d.ID, name)
		}
	}
}

func (g groupBE) devs() []homebase.Device {
	keep := map[string]bool{}
	for _, id := range g.ids {
		keep[id] = true
	}
	var out []homebase.Device
	for _, d := range g.Backend.Devices() {
		if keep[d.ID] {
			out = append(out, d)
		}
	}
	return out
}

func (p *Panel) groupView(top float32, lights []homebase.Device) {
	var scope []homebase.Device
	for _, d := range lights {
		if d.Online && (p.filter == "" || d.Room == p.filter) {
			scope = append(scope, d)
		}
	}
	name := "All lights"
	if p.filter != "" {
		name = p.filter
	}
	if len(scope) == 0 {
		p.text(16, top+2, name, 18, cWhite, semi)
		p.text(16, top+34, "No lights are reachable right now.", 13, cDim, reg)
		return
	}
	g := homebase.Device{ID: "*", Name: name, Room: fmt.Sprintf("%d lights", len(scope)), Kind: homebase.KindLight, Online: true, Mode: "white", Hue: 30, Sat: 100}
	var bright, temp, lit, colour int
	for _, d := range scope {
		g.HasTemp, g.HasColour, g.HasScene = g.HasTemp || d.HasTemp, g.HasColour || d.HasColour, g.HasScene || d.HasScene
		if d.On {
			lit++
			bright, temp = bright+d.Bright, temp+d.Temp
			if d.Mode == "colour" && d.HasColour {
				if colour == 0 {
					g.Hue, g.Sat = d.Hue, d.Sat
				}
				colour++
			}
		}
	}
	g.On = lit > 0
	g.Bright = 100
	if lit > 0 {
		g.Bright, g.Temp = bright/lit, temp/lit
		if colour*2 > lit {
			g.Mode = "colour"
		}
	}
	ids := make([]string, len(scope))
	for i, d := range scope {
		ids[i] = d.ID
	}
	real := p.be
	p.be = groupBE{real, ids}
	p.detailView(g, top)
	p.be = real
}
