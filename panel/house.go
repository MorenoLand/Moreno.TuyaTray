package panel

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/op"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeplan"
)

func tagFor(pl *homeplan.Plan, d homebase.Device) string {
	rt, nt := pl.Tag(d.Room), pl.Tag(d.Name)
	if rt != "" && rt != "living" {
		return rt
	}
	if nt != "" {
		return nt
	}
	return rt
}

func assignSlots(pl *homeplan.Plan, lights []homebase.Device, over map[string]int) map[string]int {
	slots := pl.Slots
	used, out := make([]bool, len(slots)), map[string]int{}
	pick := func(tag string) int {
		for i, s := range slots {
			if !used[i] && ((tag == "" && s.Room != pl.Upper) || s.Room == tag) {
				return i
			}
		}
		return -1
	}
	for _, d := range lights {
		if i, ok := over[d.ID]; ok && i >= 0 && i < len(slots) && !used[i] {
			used[i], out[d.ID] = true, i
		}
	}
	for _, d := range lights {
		if _, done := out[d.ID]; done {
			continue
		}
		i := pick(tagFor(pl, d))
		if i < 0 {
			i = pick("")
		}
		if i >= 0 {
			used[i], out[d.ID] = true, i
		}
	}
	return out
}

type iso struct {
	ox, oy, a float32
	p         *Panel
}

func (m iso) at(x, y, z float32) f32.Point {
	return m.p.pt(m.ox+(x-y)*m.a, m.oy+(x+y)*m.a*0.5-z*m.a*0.9)
}

func (m iso) scr(x, y, z float32) (float32, float32) {
	q := m.at(x, y, z)
	return q.X / m.p.s, q.Y / m.p.s
}

func (m iso) quad(a, b, c, d [3]float32, col color.NRGBA) {
	m.p.fillPoly([]f32.Point{m.at(a[0], a[1], a[2]), m.at(b[0], b[1], b[2]), m.at(c[0], c[1], c[2]), m.at(d[0], d[1], d[2])}, col)
}

func (m iso) floor(x0, y0, x1, y1, z float32, c color.NRGBA) {
	m.quad([3]float32{x0, y0, z}, [3]float32{x1, y0, z}, [3]float32{x1, y1, z}, [3]float32{x0, y1, z}, c)
}

func (m iso) box(x, y, z, w, d, h float32, top, left, right color.NRGBA) {
	m.quad([3]float32{x, y + d, z}, [3]float32{x + w, y + d, z}, [3]float32{x + w, y + d, z + h}, [3]float32{x, y + d, z + h}, left)
	m.quad([3]float32{x + w, y, z}, [3]float32{x + w, y + d, z}, [3]float32{x + w, y + d, z + h}, [3]float32{x + w, y, z + h}, right)
	m.quad([3]float32{x, y, z + h}, [3]float32{x + w, y, z + h}, [3]float32{x + w, y + d, z + h}, [3]float32{x, y + d, z + h}, top)
}

func (m iso) solid(x, y, z, w, d, h float32, c color.NRGBA) {
	m.box(x, y, z, w, d, h, shade(c, 1.12), shade(c, 0.86), shade(c, 0.68))
}

func (m iso) disc(x, y, z, r float32, c color.NRGBA) {
	pts := make([]f32.Point, 0, 16)
	for i := 0; i < 16; i++ {
		t := float64(i) * math.Pi / 8
		pts = append(pts, m.at(x+r*float32(math.Cos(t)), y+r*float32(math.Sin(t)), z))
	}
	m.p.fillPoly(pts, c)
}

func (m iso) post(x, y, z, r, h float32, c color.NRGBA) {
	for i := 0; i <= 3; i++ {
		m.disc(x, y, z+h*float32(i)/3, r, shade(c, 0.78+0.07*float32(i)))
	}
	m.disc(x, y, z+h, r, shade(c, 1.15))
}

func (m iso) line(x0, y0, z0, x1, y1, z1, w float32, c color.NRGBA) {
	m.p.strokePoly([]f32.Point{m.at(x0, y0, z0), m.at(x1, y1, z1)}, w, c, false)
}

func (m iso) planks(x0, y0, x1, y1, z, step float32, c color.NRGBA) {
	for y := y0 + step; y < y1-0.01; y += step {
		m.line(x0, y, z, x1, y, z, 0.6, c)
	}
}

func (m iso) tiles(x0, y0, x1, y1, z, step float32, c color.NRGBA) {
	for x := x0 + step; x < x1-0.01; x += step {
		m.line(x, y0, z, x, y1, z, 0.6, c)
	}
	m.planks(x0, y0, x1, y1, z, step, c)
}

func shade(c color.NRGBA, k float32) color.NRGBA {
	f := func(v uint8) uint8 { return uint8(clampf(float32(v)*k, 0, 255)) }
	return color.NRGBA{f(c.R), f(c.G), f(c.B), c.A}
}

type piece struct {
	key  float32
	draw func()
}

func (p *Panel) house(cx, top, h float32, lights []homebase.Device, sel string) {
	pl := p.be.Plan()
	slots := pl.Slots
	W, D, H := pl.Size[0], pl.Size[1], pl.Size[2]
	if h < 8 {
		return
	}
	asg := assignSlots(pl, lights, p.settings.Slots)
	upperUsed := p.placing
	for _, i := range asg {
		upperUsed = upperUsed || (pl.Upper != "" && slots[i].Room == pl.Upper)
	}
	extra := float32(0)
	if upperUsed && pl.Upper != "" {
		extra = 1.44
	}
	ext, lift := (W+D)/2+0.9*H+0.32, 0.9*H
	a := clampf((h-8)/(ext+extra), 14, 30)
	m := iso{ox: cx, a: a, p: p}
	m.oy = top + (h-(ext+extra)*a)/2 + (lift+extra)*a
	zt := p.ease("zoom", b2f(sel != "" && !p.placing), 4.5)
	uz := p.ease("camz", p.camZ, 10)
	z := (1 + 0.95*zt + 0.1*(1-p.introV)) * uz
	cxp, cyp := cx*p.s, (top+h/2)*p.s
	panx, pany := p.ease("camx", p.camX, 10)*p.s, p.ease("camy", p.camY, 10)*p.s
	fxp, fyp := cxp, cyp
	if i, ok := asg[sel]; ok && sel != "" {
		q := m.at(slots[i].X, slots[i].Y, slots[i].Z)
		fxp, fyp = cxp+(q.X-cxp)*zt, cyp+(q.Y-cyp)*zt
	}
	cl := p.clipTo(0, top, p.w, h)
	tr := op.Affine(f32.Affine2D{}.Offset(f32.Pt(-fxp, -fyp)).Scale(f32.Point{}, f32.Pt(z, z)).Offset(f32.Pt(cxp+panx, cyp+pany))).Push(p.ops)
	p.glowE(cx*p.s/p.s, top+h*0.5+a*3.6, a*7.5, a*2.4, rgb(2, 3, 14), 0.55)
	const base = 0.4
	k := map[string]float32{}
	for _, r := range pl.Rooms {
		k[r] = base
	}
	for _, d := range lights {
		if i, ok := asg[d.ID]; ok && d.On && d.Online {
			t := slots[i].Room
			k[t] = max(k[t], base+(1-base)*(0.62+0.38*float32(d.Bright)/100))
		}
	}
	var kg float32
	for _, t := range pl.Rooms {
		k[t] = p.ease("room:"+t, k[t], 5)
		kg += k[t] / float32(len(pl.Rooms))
	}
	night := rgb(16, 22, 54)
	col := func(room string, c [3]uint8) color.NRGBA {
		if room == "" {
			return mix(night, rgb(c[0], c[1], c[2]), kg)
		}
		return mix(night, rgb(c[0], c[1], c[2]), k[room])
	}
	m.box(-0.14, -0.14, -0.4, W+0.14, D+0.14, 0.4, col("", [3]uint8{70, 64, 86}), col("", [3]uint8{44, 40, 60}), col("", [3]uint8{34, 30, 48}))
	for _, f := range pl.Floors {
		m.floor(f.R[0], f.R[1], f.R[2], f.R[3], f.Z, col(f.Rm, f.C))
		kind, step, _ := strings.Cut(f.Pat, ":")
		st := float32(0.3)
		if step != "" {
			if v, err := strconv.ParseFloat(step, 32); err == nil && v > 0.05 {
				st = float32(v)
			}
		}
		switch kind {
		case "planks":
			m.planks(f.R[0], f.R[1], f.R[2], f.R[3], f.Z, st, rgba(0, 0, 0, 38))
		case "tiles":
			m.tiles(f.R[0], f.R[1], f.R[2], f.R[3], f.Z, st, rgba(0, 0, 40, 44))
		}
	}
	wt := col("", [3]uint8{206, 198, 218})
	for _, w := range pl.Walls {
		if w.Up && !upperUsed {
			continue
		}
		c := [3]uint8{196, 182, 204}
		if w.Side == "R" {
			c = [3]uint8{168, 154, 188}
		}
		wc := col(w.Rm, c)
		m.box(w.B[0], w.B[1], w.B[2], w.B[3], w.B[4], w.B[5], wt, wc, wc)
	}
	glass := mix(rgb(22, 34, 82), rgb(130, 160, 225), kg*0.45)
	for _, w := range pl.Windows {
		if w.Side == "L" {
			m.quad([3]float32{0, w.A, w.Z0}, [3]float32{0, w.B, w.Z0}, [3]float32{0, w.B, w.Z1}, [3]float32{0, w.A, w.Z1}, glass)
			m.line(0, (w.A+w.B)/2, w.Z0, 0, (w.A+w.B)/2, w.Z1, 1.2, wt)
		} else {
			m.quad([3]float32{w.A, 0, w.Z0}, [3]float32{w.B, 0, w.Z0}, [3]float32{w.B, 0, w.Z1}, [3]float32{w.A, 0, w.Z1}, glass)
			m.line((w.A+w.B)/2, 0, w.Z0, (w.A+w.B)/2, 0, w.Z1, 1.2, wt)
		}
	}
	var items []piece
	for _, it := range pl.Items {
		if it.Up && !upperUsed {
			continue
		}
		parts := it.P
		items = append(items, piece{it.K[0] + it.K[2]/2 + it.K[1] + it.K[3]/2, func() {
			for _, pt := range parts {
				p.drawPart(m, pt, k, night, col)
			}
		}})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].key < items[j].key })
	for _, it := range items {
		it.draw()
	}
	tr.Pop()
	cl.Pop()
	p.fadeBand(top, top+22, true)
	p.fadeBand(top+h-44, top+h, false)
	move := op.Affine(f32.Affine2D{}.Offset(f32.Pt(-fxp, -fyp)).Scale(f32.Point{}, f32.Pt(z, z)).Offset(f32.Pt(cxp+panx, cyp+pany)))
	type lamp struct {
		d              homebase.Device
		gx, gy, fx, fy float32
		br             float32
	}
	var lamps []lamp
	var fresh []spillPt
	for _, d := range lights {
		i, ok := asg[d.ID]
		if !ok {
			continue
		}
		s := slots[i]
		gx, gy := m.scr(s.X, s.Y, s.Z)
		fx, fy := m.scr(s.X, s.Y, 0)
		br := p.ease("gl:"+d.ID, b2f(d.On && d.Online)*(0.45+0.55*float32(d.Bright)/100), 7)
		lamps = append(lamps, lamp{d, gx, gy, fx, fy, br})
	}
	gl := move.Push(p.ops)
	for _, l := range lamps {
		if l.br <= 0.02 {
			continue
		}
		col, spill := lightColour(l.d), float32(1)
		if l.d.Mode == "colour" && l.d.HasColour {
			spill = 0.62
		}
		warm := mix(col, rgb(255, 238, 205), 0.35)
		tx := (cxp + panx + z*(l.gx*p.s-fxp)) / p.s
		ty := (cyp + pany + z*(l.gy*p.s-fyp)) / p.s
		fresh = append(fresh, spillPt{tx, ty, col, l.br, a * z})
		p.glowE(l.fx, l.fy, a*4.2*spill, a*2.2*spill, col, 0.55*l.br)
		p.glow(l.gx, l.gy, a*3.0*spill, col, 0.5*l.br*spill)
		p.glow(l.gx, l.gy, a*1.5*spill, warm, 0.8*l.br)
		p.glow(l.gx, l.gy, a*0.6, rgb(255, 252, 240), 1.0*l.br)
	}
	gl.Pop()
	p.spill = fresh
	cl = p.clipTo(0, top, p.w, h)
	tr = move.Push(p.ops)
	for _, l := range lamps {
		if l.d.ID == sel {
			p.ring(l.gx, l.gy, 13, 1.8, rgba(255, 255, 255, 220))
		}
		if l.br < 0.5 {
			p.disc(l.gx, l.gy, 2.2, rgba(150, 156, 190, uint8(110*(1-l.br*2))))
		}
		id := l.d.ID
		if p.area("house:"+id, l.gx-14, l.gy-14, 28, 28).Clicked && l.d.Online {
			p.be.SetPower(id, !l.d.On)
			p.scene = ""
		}
	}
	if p.placing {
		for i, s := range slots {
			gx, gy := m.scr(s.X, s.Y, s.Z)
			cur := asg[sel] == i
			p.ring(gx, gy, 9, 1.6, mix(rgba(255, 255, 255, 140), rgb(255, 190, 90), b2f(cur)))
			p.disc(gx, gy, 3, mix(rgba(255, 255, 255, 120), rgb(255, 190, 90), b2f(cur)))
			if p.area(fmt.Sprint("slot", i), gx-13, gy-13, 26, 26).Clicked {
				p.be.SetSlot(sel, i)
				p.placing = false
			}
		}
	}
	tr.Pop()
	cl.Pop()
}

func (p *Panel) paintSpill(x, y, w, h float32) {
	defer p.clipTo(x, y, w, h).Pop()
	sp := 0.3 + 0.7*clampf(p.heroNow/150, 0, 1)
	for _, s := range p.spill {
		p.glowE(s.x, s.y, s.r*9, s.r*6, s.c, 0.2*s.k*sp)
		p.glow(s.x, s.y, s.r*4, s.c, 0.14*s.k*sp)
	}
}

func (p *Panel) drawPart(m iso, pt homeplan.Part, k map[string]float32, night color.NRGBA, col func(string, [3]uint8) color.NRGBA) {
	base := col(pt.Rm, pt.C)
	if pt.Sh != 0 {
		base = shade(base, pt.Sh)
	}
	switch pt.T {
	case "solid", "box":
		f := [3]float32{1.12, 0.86, 0.68}
		for i := 0; i < 3 && i < len(pt.F); i++ {
			f[i] = pt.F[i]
		}
		top, left, right := shade(base, f[0]), shade(base, f[1]), shade(base, f[2])
		if pt.Top != nil {
			g := pt.Rm
			if pt.TopG {
				g = ""
			}
			top = col(g, *pt.Top)
		}
		if pt.Left != nil {
			left = col(pt.Rm, *pt.Left)
		}
		if pt.Right != nil {
			right = col(pt.Rm, *pt.Right)
		}
		m.box(pt.B[0], pt.B[1], pt.B[2], pt.B[3], pt.B[4], pt.B[5], top, left, right)
	case "part":
		hh := pt.H
		if hh <= 0 {
			hh = 1.35
		}
		lit := float32(0)
		for _, r := range pt.Rooms {
			lit = max(lit, k[r])
		}
		pc := mix(night, rgb(190, 180, 198), lit)
		m.box(pt.B[0], pt.B[1], 0, pt.B[2], pt.B[3], hh, shade(pc, 1.06), shade(pc, 0.86), shade(pc, 0.66))
	case "disc":
		m.disc(pt.P[0], pt.P[1], pt.P[2], pt.R, base)
	case "post":
		m.post(pt.P[0], pt.P[1], pt.P[2], pt.R, pt.H, base)
	case "quad":
		c := base
		if pt.Screen != nil {
			c = mix(base, rgb(pt.Screen[0], pt.Screen[1], pt.Screen[2]), 0.5*(k[pt.Rm]-0.4))
		}
		m.quad(pt.Pts[0], pt.Pts[1], pt.Pts[2], pt.Pts[3], c)
	case "line":
		w := pt.W
		if w <= 0 {
			w = 1.2
		}
		m.line(pt.A[0], pt.A[1], pt.A[2], pt.B[0], pt.B[1], pt.B[2], w, base)
	}
}
