package panel

import (
	"image/color"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/paint"
	"moreno/tuyatray/homebase"
)

func (p *Panel) background(lights []homebase.Device) {
	var r, g, b, ws float32
	on := 0
	for _, d := range lights {
		if d.On && d.Online {
			c, w := lightColour(d), 0.3+0.7*float32(d.Bright)/100
			r, g, b, ws = r+float32(c.R)*w, g+float32(c.G)*w, b+float32(c.B)*w, ws+w
			on++
		}
	}
	base := rgb(20, 23, 60)
	target := base
	if ws > 0 {
		f := float32(on) / float32(max(len(lights), 1))
		tint := rgb(uint8(r/ws*0.55), uint8(g/ws*0.42), uint8(b/ws*0.42))
		target = mix(base, tint, 0.3+0.4*f)
	}
	tr, tg, tb := p.ease("bgr", float32(target.R), 4), p.ease("bgg", float32(target.G), 4), p.ease("bgb", float32(target.B), 4)
	p.bgTop = rgb(uint8(tr), uint8(tg), uint8(tb))
	p.backdrop(0, 0, p.w, p.h)
}

func (p *Panel) backdrop(x, y, w, h float32) {
	defer p.clipTo(x, y, w, h).Pop()
	p.gradRR(0, 0, p.w, p.h, 9, p.bgTop, cBgBot, false)
	p.paintSpill(x, y, w, h)
}

func (p *Panel) fadeBand(y0, y1 float32, solidTop bool) {
	a, b := p.px(y0), p.px(y1)
	n := max((b-a)/3, 6)
	for i := 0; i < n; i++ {
		t := (float32(i) + 0.5) / float32(n)
		if !solidTop {
			t = 1 - t
		}
		s, e := a+(b-a)*i/n, a+(b-a)*(i+1)/n
		pop := paint.PushOpacity(p.ops, 1-t)
		p.backdrop(0, float32(s)/p.s, p.w, float32(e-s)/p.s)
		pop.Pop()
	}
}

func (p *Panel) bgAt(y float32) color.NRGBA { return mix(p.bgTop, cBgBot, clampf(y/p.h, 0, 1)) }

func (p *Panel) pressScale(tag string, x, y, w, h float32, down bool) func() {
	pr := p.ease("press:"+tag, b2f(down), 22)
	sc := 1 - 0.035*pr
	st := op.Affine(f32.Affine2D{}.Scale(f32.Pt((x+w/2)*p.s, (y+h/2)*p.s), f32.Pt(sc, sc))).Push(p.ops)
	return st.Pop
}

func (p *Panel) stagger(i int) (float32, float32) {
	d := min(float32(i), 8) * 0.06
	t := clampf((p.introV-d)/(1-d), 0, 1)
	return t, (1 - t) * 14
}

func (p *Panel) dragDelta(id string, ht hit) (float32, float32, bool) {
	l, ok := p.dg[id]
	if !ht.Down || !ht.Moved {
		if ok && !ht.Down {
			delete(p.dg, id)
		}
		return 0, 0, false
	}
	if !ok {
		p.dg[id] = &[2]float32{ht.X, ht.Y}
		return 0, 0, true
	}
	dx, dy := ht.X-l[0], ht.Y-l[1]
	l[0], l[1] = ht.X, ht.Y
	return dx, dy, true
}
