package panel

import (
	"image/color"

	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"moreno/tuyatray/homebase"
)

var effectColours = map[string][]color.NRGBA{
	"Night":    {rgb(18, 26, 64), rgb(120, 96, 120), rgb(236, 170, 100)},
	"Reading":  {rgb(246, 226, 196), rgb(255, 248, 236)},
	"Cocktail": {rgb(238, 70, 190), rgb(120, 70, 240), rgb(70, 130, 250)},
	"Leisure":  {rgb(60, 110, 150), rgb(170, 150, 150), rgb(242, 170, 120)},
	"Soft":     {rgb(255, 205, 186), rgb(255, 232, 214), rgb(240, 214, 236)},
	"Blinking": {rgb(255, 60, 70), rgb(60, 240, 100), rgb(70, 110, 255)},
	"Rainbow":  {rgb(255, 50, 50), rgb(255, 220, 40), rgb(60, 230, 80), rgb(50, 220, 240), rgb(70, 90, 255), rgb(230, 60, 230)},
	"Rave":     {rgb(255, 40, 70), rgb(24, 20, 36), rgb(255, 40, 70)},
	"Nature":   {rgb(30, 130, 70), rgb(120, 200, 90), rgb(40, 150, 90)},
}

func (p *Panel) sceneView(d homebase.Device) bool {
	return d.HasScene && (d.InScene() || p.sceneTab == d.ID)
}

func (p *Panel) discGrad(cx, cy, r float32, cols []color.NRGBA) {
	defer clip.UniformRRect(p.rect(cx-r, cy-r, 2*r, 2*r), p.px(r)).Push(p.ops).Pop()
	n := len(cols)
	seg := 2 * r / float32(n-1)
	for i := 0; i < n-1; i++ {
		x0 := cx - r + seg*float32(i)
		st := clip.Rect(p.rect(x0, cy-r, seg+1, 2*r)).Push(p.ops)
		paint.LinearGradientOp{Stop1: p.pt(x0, cy), Stop2: p.pt(x0+seg, cy), Color1: cols[i], Color2: cols[i+1]}.Add(p.ops)
		paint.PaintOp{}.Add(p.ops)
		st.Pop()
	}
}

func (p *Panel) effectsGrid(d homebase.Device, y float32) float32 {
	cw := (p.w - 32) / 3
	for i, n := range homebase.Effects {
		cx, cy := 16+cw*(float32(i%3)+0.5), y+float32(i/3)*88+30
		sel := p.ease("fx:"+d.ID+n, b2f(d.InScene() && d.Scene == n), 12)
		ht := p.area("fxhit:"+d.ID+n, cx-cw/2, cy-30, cw, 82)
		cols := effectColours[n]
		dim := make([]color.NRGBA, len(cols))
		for j, c := range cols {
			dim[j] = mix(c, rgb(26, 30, 46), 0.4*(1-sel))
		}
		p.glow(cx, cy, 50, cols[0], 0.5*sel)
		p.discGrad(cx, cy, 26, dim)
		if sel > 0.01 {
			p.ring(cx, cy, 30, 2, rgba(255, 255, 255, uint8(235*sel)))
		}
		p.textC(cx, cy+34, n, 12, mix(cDim, cWhite, sel), reg)
		if ht.Clicked && d.Online {
			p.be.SetEffect(d.ID, n)
		}
	}
	return y + 3*88
}
