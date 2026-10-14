package panel

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeicons"
)

type iconCache struct {
	imgs map[string]paint.ImageOp
	bad  map[string]bool
}

func (p *Panel) builtinIcon(name string) func(float32, float32, color.NRGBA) {
	switch name {
	case "bulb":
		return p.icoBulb
	case "lamp":
		return p.icoLamp
	case "strip":
		return p.icoSpark
	case "plug":
		return p.icoPlug
	case "tv":
		return p.icoFilm
	case "fan":
		return p.icoFan
	case "bed":
		return p.icoBed
	case "sofa":
		return p.icoSofa
	case "kitchen":
		return p.icoPot
	case "bath":
		return p.icoTub
	case "desk":
		return p.icoDesk
	case "plant":
		return p.icoPlant
	case "sun":
		return p.icoSun
	case "moon":
		return p.icoMoon
	case "door":
		return p.icoDoor
	case "home":
		return p.icoHome
	}
	return nil
}

func (p *Panel) iconImage(file string) (paint.ImageOp, bool) {
	if p.icons.imgs == nil {
		p.icons.imgs, p.icons.bad = map[string]paint.ImageOp{}, map[string]bool{}
	}
	if im, ok := p.icons.imgs[file]; ok {
		return im, true
	}
	if p.icons.bad[file] || !homeicons.ValidFile(file) {
		return paint.ImageOp{}, false
	}
	path := p.be.IconPath(file)
	f, err := os.Open(path)
	if err != nil {
		p.icons.bad[file] = true
		return paint.ImageOp{}, false
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		p.icons.bad[file] = true
		return paint.ImageOp{}, false
	}
	if len(p.icons.imgs) > 48 {
		p.icons.imgs = map[string]paint.ImageOp{}
	}
	im := paint.NewImageOp(img)
	p.icons.imgs[file] = im
	return im, true
}

func (p *Panel) pic(file string, x, y, w, h, r float32) bool {
	im, ok := p.iconImage(file)
	if !ok {
		return false
	}
	sz := im.Size()
	rr := int(r*p.s + 0.5)
	defer clip.RRect{Rect: p.rect(x, y, w, h), NE: rr, NW: rr, SE: rr, SW: rr}.Push(p.ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(w*p.s/float32(sz.X), h*p.s/float32(sz.Y))).Offset(f32.Pt(x*p.s, y*p.s))).Push(p.ops).Pop()
	im.Add(p.ops)
	paint.PaintOp{}.Add(p.ops)
	return true
}

func (p *Panel) iconChoice(key string) (kind, val string) {
	v := p.settings.Icons[key]
	switch {
	case strings.HasPrefix(v, "b:"):
		return "b", v[2:]
	case strings.HasPrefix(v, "i:"):
		return "i", v[2:]
	}
	return "", ""
}

func (p *Panel) scaleAbout(cx, cy, k float32, f func()) {
	st := op.Affine(f32.Affine2D{}.Scale(f32.Pt(cx*p.s, cy*p.s), f32.Pt(k, k))).Push(p.ops)
	f()
	st.Pop()
}

func (p *Panel) roomMark(room string, x, y, size float32, c color.NRGBA) bool {
	kind, val := p.iconChoice("r:" + room)
	switch kind {
	case "i":
		return p.pic(val, x, y, size, size, size/4)
	case "b":
		if fn := p.builtinIcon(val); fn != nil {
			k := size / 20
			p.scaleAbout(x+size/2, y+size/2, k, func() { fn(x+size/2, y+size/2, c) })
			return true
		}
	}
	return false
}

func (p *Panel) iconPreview(key string, def func(float32, float32, color.NRGBA), x, y, size float32, c color.NRGBA) {
	p.fillRR(x, y, size, size, size/3.2, rgba(255, 255, 255, 10))
	kind, val := p.iconChoice(key)
	switch {
	case kind == "i" && p.pic(val, x, y, size, size, size/3.2):
	case kind == "b" && p.builtinIcon(val) != nil:
		p.scaleAbout(x+size/2, y+size/2, size/38, func() { p.builtinIcon(val)(x+size/2, y+size/2, c) })
	default:
		p.scaleAbout(x+size/2, y+size/2, size/38, func() { def(x+size/2, y+size/2, c) })
	}
}

func (p *Panel) iconsView(top float32) {
	vh := p.dh() - top
	p.text(16, top+6, "Icons", 18, cWhite, semi)
	type row struct{ key, label, sub string }
	var rows []row
	rooms := p.rooms()
	for _, r := range rooms {
		rows = append(rows, row{"r:" + r, r, "Room"})
	}
	for _, d := range p.devs {
		rows = append(rows, row{"d:" + d.ID, d.Name, d.Room})
	}
	y0 := top + 44
	region := vh - 44
	content := float32(len(rows))*56 + 10
	p.autoState.iscroll = clampf(p.autoState.iscroll+p.wheel("iconwheel", 0, y0, p.w, region), 0, max(0, content-region))
	defer p.clipTo(0, y0, p.w, region).Pop()
	y := y0 - p.autoState.iscroll
	for i, r := range rows {
		if y+52 > y0 && y < y0+region {
			p.glass(16, y, p.w-32, 50, 16)
			def := p.icoBulb
			if strings.HasPrefix(r.key, "d:") {
				if d, ok := p.find(r.key[2:]); ok {
					def = func(cx, cy float32, c color.NRGBA) { p.defaultIcon(d, cx, cy, c) }
				}
			} else {
				def = p.icoHome
			}
			p.iconPreview(r.key, def, 26, y+7, 36, cWhite)
			p.textFit(76, y+8, p.w-76-90, r.label, 13.5, cWhite, semi)
			kind, val := p.iconChoice(r.key)
			state := "Default icon"
			if kind == "b" {
				state = strings.ToUpper(val[:1]) + val[1:]
			} else if kind == "i" {
				state = "Custom image"
			}
			p.textFit(76, y+28, p.w-76-90, r.sub+" · "+state, 11.5, cDim, reg)
			p.strokePoly([]f32.Point{p.pt(p.w-40, y+19), p.pt(p.w-34, y+25), p.pt(p.w-40, y+31)}, 1.8, cDim, false)
			if p.area(fmt.Sprint("icrow", i), 16, y, p.w-32, 50).Clicked {
				p.iconKey = r.key
			}
		}
		y += 56
	}
	p.reach(top + 44 + content)
}

func (p *Panel) drawIconPicker() {
	if p.iconKey == "" {
		return
	}
	key := p.iconKey
	p.fillRR(0, 0, p.w, p.h, 9, rgba(3, 5, 16, 225))
	p.area("scrim3", 0, 0, p.w, p.h)
	cols := float32(4)
	cw := (p.w - 40 - 32 - (cols-1)*8) / cols
	rows := (len(homeicons.Builtin) + 3) / 4
	h := 64 + float32(rows)*(56+8) + 12 + 44 + 8 + 44 + 16
	p.reach(h + 40)
	x, y, w := float32(20), (p.h-h)/2, p.w-40
	p.gradRR(x, y, w, h, 20, rgb(34, 38, 70), rgb(18, 22, 44), false)
	p.strokeRR(x, y, w, h, 20, 1.2, rgb(70, 82, 130))
	title := strings.TrimPrefix(strings.TrimPrefix(key, "d:"), "r:")
	if strings.HasPrefix(key, "d:") {
		if d, ok := p.find(key[2:]); ok {
			title = d.Name
		}
	}
	p.text(x+18, y+14, "Choose an icon", 17, cWhite, semi)
	p.textFit(x+18, y+40, w-36, title, 12, cDim, reg)
	_, cur := p.iconChoice(key)
	for i, n := range homeicons.Builtin {
		bx := x + 16 + float32(i%4)*(cw+8)
		by := y + 64 + float32(i/4)*(56+8)
		sel := p.ease(fmt.Sprint("ic", n), b2f(cur == n), 16)
		p.fillRR(bx, by, cw, 56, 14, mix(rgba(255, 255, 255, 5), rgb(48, 56, 94), sel))
		p.strokeRR(bx, by, cw, 56, 14, 1.1, mix(rgba(255, 255, 255, 14), rgb(90, 150, 255), sel))
		p.builtinIcon(n)(bx+cw/2, by+22, mix(cDim, cWhite, sel))
		p.textC(bx+cw/2, by+36, n, 10.5, mix(cDim, cWhite, sel), reg)
		if p.area(fmt.Sprint("icpick", n), bx, by, cw, 56).Clicked {
			p.be.SetIcon(key, "b:"+n)
			p.iconKey = ""
		}
	}
	by := y + 64 + float32(rows)*(56+8) + 4
	if p.button("ic-image", x+16, by, w-32, 44, "Choose an image from this PC…", true) {
		p.be.ChooseIconImage(key)
		p.iconKey = ""
	}
	bw := (w - 40) / 2
	if p.button("ic-reset", x+16, by+52, bw, 44, "Use default", false) {
		p.be.SetIcon(key, "")
		p.iconKey = ""
	}
	if p.button("ic-cancel", x+24+bw, by+52, bw, 44, "Cancel", false) {
		p.iconKey = ""
	}
}

func (p *Panel) icoFan(cx, cy float32, c color.NRGBA) {
	for i := 0; i < 3; i++ {
		a := float64(i)*2*math.Pi/3 - math.Pi/2
		ex, ey := cx+8*float32(math.Cos(a)), cy+8*float32(math.Sin(a))
		mx, my := cx+4.5*float32(math.Cos(a+0.55)), cy+4.5*float32(math.Sin(a+0.55))
		p.strokePoly([]f32.Point{p.pt(cx, cy), p.pt(mx, my), p.pt(ex, ey)}, 1.8, c, false)
	}
	p.disc(cx, cy, 2.2, c)
}

func (p *Panel) icoBed(cx, cy float32, c color.NRGBA) {
	p.line(cx-9, cy-6, cx-9, cy+7, 1.6, c)
	p.line(cx+9, cy+1, cx+9, cy+7, 1.6, c)
	p.strokeRR(cx-9, cy, 18, 5, 1.5, 1.6, c)
	p.strokeRR(cx-7, cy-4, 6, 4, 1.5, 1.4, c)
}

func (p *Panel) icoPot(cx, cy float32, c color.NRGBA) {
	p.strokeRR(cx-7, cy-1, 14, 9, 2, 1.6, c)
	p.line(cx-9, cy-2, cx+9, cy-2, 1.6, c)
	p.line(cx-2.5, cy-5, cx+2.5, cy-5, 1.6, c)
	p.line(cx-9, cy+2, cx-7, cy+2, 1.6, c)
	p.line(cx+7, cy+2, cx+9, cy+2, 1.6, c)
}

func (p *Panel) icoTub(cx, cy float32, c color.NRGBA) {
	p.strokePoly([]f32.Point{p.pt(cx-9, cy), p.pt(cx+9, cy), p.pt(cx+7, cy+6), p.pt(cx-7, cy+6)}, 1.6, c, true)
	p.line(cx-5, cy+6, cx-6, cy+9, 1.6, c)
	p.line(cx+5, cy+6, cx+6, cy+9, 1.6, c)
	p.strokePoly([]f32.Point{p.pt(cx-6, cy), p.pt(cx-6, cy-7), p.pt(cx-2, cy-7)}, 1.6, c, false)
}

func (p *Panel) icoDesk(cx, cy float32, c color.NRGBA) {
	p.line(cx-10, cy-3, cx+10, cy-3, 2, c)
	p.line(cx-8, cy-3, cx-8, cy+8, 1.6, c)
	p.line(cx+8, cy-3, cx+8, cy+8, 1.6, c)
	p.strokeRR(cx+1, cy-1, 6, 5, 1.2, 1.4, c)
}

func (p *Panel) icoPlant(cx, cy float32, c color.NRGBA) {
	p.strokePoly([]f32.Point{p.pt(cx-5, cy+2), p.pt(cx+5, cy+2), p.pt(cx+3.5, cy+9), p.pt(cx-3.5, cy+9)}, 1.6, c, true)
	p.line(cx, cy+2, cx, cy-8, 1.6, c)
	p.line(cx, cy+1, cx-6, cy-5, 1.6, c)
	p.line(cx, cy+1, cx+6, cy-5, 1.6, c)
}

func (p *Panel) defaultIcon(d homebase.Device, cx, cy float32, c color.NRGBA) {
	n := strings.ToLower(d.Name)
	switch {
	case d.Kind == homebase.KindSwitch:
		p.icoPlug(cx, cy, c)
	case strings.Contains(n, "strip") || strings.Contains(n, "tv") || strings.Contains(n, "back") || strings.Contains(n, "led") || strings.Contains(n, "tree"):
		p.icoSpark(cx, cy, c)
	case strings.Contains(n, "lamp") || strings.Contains(n, "floor") || strings.Contains(n, "desk"):
		p.icoLamp(cx, cy, c)
	default:
		p.icoBulb(cx, cy, c)
	}
}

var _ = image.Pt
