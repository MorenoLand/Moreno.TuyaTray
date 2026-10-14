package panel

import "moreno/tuyatray/homebase"

type cell struct {
	d          homebase.Device
	head       string
	x, y, w, h float32
}

type group struct {
	room string
	devs []homebase.Device
	half bool
}

const (
	gridPad  = 14
	gridGap  = 10
	tileStep = 72
	rowStep  = 62
	headStep = 26
)

func (p *Panel) groups() []group {
	var out []group
	at := map[string]int{}
	for _, d := range p.devs {
		if p.filter != "" && d.Room != p.filter {
			continue
		}
		i, ok := at[d.Room]
		if !ok {
			i = len(out)
			at[d.Room] = i
			out = append(out, group{room: d.Room})
		}
		out[i].devs = append(out[i].devs, d)
	}
	if p.filter != "" || len(out) < 2 {
		return out
	}
	for i := range out {
		g := &out[i]
		g.half = len(g.devs) == 1 && g.devs[0].Kind != homebase.KindSwitch
	}
	return out
}

func (p *Panel) arrange() ([]cell, float32) {
	tw := (p.w - 2*gridPad - gridGap) / 2
	colX := [2]float32{gridPad, gridPad + tw + gridGap}
	var cells []cell
	cy := float32(0)
	side := func(g group, c int) {
		cells = append(cells, cell{head: g.room, x: colX[c] + 2, y: cy})
		cells = append(cells, cell{d: g.devs[0], x: colX[c], y: cy + headStep, w: tw, h: 62})
	}
	full := func(g group) {
		if p.filter == "" {
			cells = append(cells, cell{head: g.room, x: colX[0] + 2, y: cy})
			cy += headStep
		}
		col := 0
		for _, d := range g.devs {
			if d.Kind == homebase.KindSwitch {
				if col == 1 {
					cy, col = cy+tileStep, 0
				}
				cells = append(cells, cell{d: d, x: gridPad, y: cy, w: p.w - 2*gridPad, h: 54})
				cy += rowStep
				continue
			}
			cells = append(cells, cell{d: d, x: colX[col], y: cy, w: tw, h: 62})
			if col++; col == 2 {
				cy, col = cy+tileStep, 0
			}
		}
		if col == 1 {
			cy += tileStep
		}
	}
	queue := p.groups()
	for len(queue) > 0 {
		g := queue[0]
		queue = queue[1:]
		if !g.half {
			full(g)
			continue
		}
		side(g, 0)
		for i, q := range queue {
			if q.half {
				side(q, 1)
				queue = append(queue[:i], queue[i+1:]...)
				break
			}
		}
		cy += headStep + tileStep
	}
	return cells, cy
}
