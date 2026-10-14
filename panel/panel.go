package panel

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/skip2/go-qrcode"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeplan"
	"moreno/tuyatray/lightshow"
)

const (
	viewMain = iota
	viewDetail
	viewSettings
	viewLink
	viewAPI
	viewSchedules
	viewRule
	viewShow
	viewDesign
	viewIcons
	viewGroup
	semi = font.SemiBold
	reg  = font.Normal
)

type Backend interface {
	Devices() []homebase.Device
	HomeName() string
	SetPower(id string, on bool)
	SetBright(id string, pct int)
	SetTemp(id string, pct int)
	SetColour(id string, hue, sat int)
	SetWhite(id string)
	AllLights(on bool)
	RoomLights(room string, on bool)
	Scene(name string)
	Rescan()
	Refresh()
	SetRoom(id, room string)
	SetSlot(id string, slot int)
	SetEffect(id, name string)
	Linked() bool
	StartLink(userCode string)
	LinkState() (payload, status string)
	Unlink()
	Settings() homebase.Settings
	SaveSettings(homebase.Settings)
	SetAPIPassword(user, pass string)
	APIStatus() string
	Quit()
	Plan() *homeplan.Plan
	HasCustomPlan() bool
	Icons() map[string]string
	SetIcon(key, value string) error
	ChooseIconImage(key string)
	IconPath(file string) string
	Rules() []homebase.Rule
	SaveRules([]homebase.Rule)
	StartTimer(minutes int, target, action string)
	CancelTimer()
	TimerState() (time.Duration, string, string)
	ShowState() lightshow.State
	ShowConfig() lightshow.Config
	StartShow(lightshow.Config) error
	StopShow()
}

type Panel struct {
	Live, Static                            bool
	be                                      Backend
	th                                      *material.Theme
	gtx                                     layout.Context
	ops                                     *op.Ops
	s, w, h, dt                             float32
	ptr                                     map[any]*pstate
	an                                      map[any]*float32
	glows                                   map[uint32]paint.ImageOp
	devs                                    []homebase.Device
	settings                                homebase.Settings
	view                                    int
	sel, filter, scene                      string
	scroll, contentH, pill                  float32
	last                                    time.Time
	animating, focused                      bool
	qrFor                                   string
	qrBits                                  [][]bool
	edPort, edUser, edPass, edAllow, edCode widget.Editor
	apiInit                                 bool
	apiMsg                                  string
	confirm                                 *confirmDlg
	guideStep                               int
	showCode                                bool
	heroNow, introV                         float32
	prevView                                int
	bgTop                                   color.NRGBA
	tileN                                   int
	MaxH, WantH, need, HomeH                float32
	customOpen                              bool
	placing, panDown                        bool
	sceneTab                                string
	camZ, camX, camY, panLX, panLY          float32
	scrollIn, pillIn, scrollVel             float32
	listDrag, pillDrag                      bool
	dg                                      map[string]*[2]float32
	heroClick                               time.Time
	picker                                  string
	edRoom                                  widget.Editor
	spill                                   []spillPt
	tileGlow                                []spillPt
	icons                                   iconCache
	iconKey                                 string
	autoState                               autoState
}

type spillPt struct {
	x, y float32
	c    color.NRGBA
	k    float32
	r    float32
}

func New(be Backend) *Panel {
	th := material.NewTheme()
	th.Shaper = loadShaper()
	return &Panel{be: be, th: th, camZ: 1, dg: map[string]*[2]float32{}, ptr: map[any]*pstate{}, an: map[any]*float32{}, glows: map[uint32]paint.ImageOp{}}
}

func (p *Panel) Reset() {
	p.view, p.sel, p.scroll = viewMain, "", 0
	p.picker, p.confirm, p.placing, p.customOpen, p.iconKey = "", nil, false, false, ""
	p.restart("intro")
}

func (p *Panel) ShowDetail(id string) { p.view, p.sel = viewDetail, id }

func (p *Panel) ShowSettings() { p.view = viewSettings }

func (p *Panel) Filter(room string) { p.filter = room }

func hsvc(h, s, v float64) color.NRGBA {
	h = math.Mod(h, 360)
	c := v * s
	x, m := c*(1-math.Abs(math.Mod(h/60, 2)-1)), v-c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return rgb(uint8((r+m)*255), uint8((g+m)*255), uint8((b+m)*255))
}

func tempColour(t float32) color.NRGBA {
	if t < 0.5 {
		return mix(rgb(255, 170, 90), rgb(255, 247, 235), t*2)
	}
	return mix(rgb(255, 247, 235), rgb(170, 205, 255), (t-0.5)*2)
}

func lightColour(d homebase.Device) color.NRGBA {
	if d.Mode == "colour" && d.HasColour {
		return hsvc(float64(d.Hue), float64(max(d.Sat, 40))/100, 1)
	}
	return tempColour(float32(d.Temp) / 100)
}

func accent(d homebase.Device) color.NRGBA {
	if d.Mode == "colour" && d.HasColour {
		return hsvc(float64(d.Hue), float64(max(d.Sat, 50))/100, 1)
	}
	return cAmberLine
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func (p *Panel) Layout(gtx layout.Context) layout.Dimensions {
	p.gtx, p.ops, p.s = gtx, gtx.Ops, gtx.Metric.PxPerDp
	p.w, p.h = float32(gtx.Constraints.Max.X)/p.s, float32(gtx.Constraints.Max.Y)/p.s
	p.dt = 0.016
	if !p.last.IsZero() {
		p.dt = clampf(float32(gtx.Now.Sub(p.last).Seconds()), 0, 0.05)
	}
	p.last, p.animating, p.need = gtx.Now, false, 0
	p.devs, p.settings = p.be.Devices(), p.be.Settings()
	if !p.be.Linked() {
		p.view = viewLink
	} else if p.view == viewLink {
		p.view, p.showCode, p.focused = viewMain, false, false
		p.edCode.SetText("")
	}
	if p.view == viewDetail {
		if _, ok := p.find(p.sel); !ok {
			p.view = viewMain
		}
	}
	if p.view != p.prevView {
		p.prevView = p.view
		p.restart("vt")
		p.camZ, p.camX, p.camY, p.placing = 1, 0, 0, false
	}
	var lights []homebase.Device
	for _, d := range p.devs {
		if d.Kind == homebase.KindLight {
			lights = append(lights, d)
		}
	}
	if p.view != viewMain {
		p.tileGlow = nil
	}
	p.background(lights)
	p.heroNow = p.ease("hero", p.heroTarget(lights), 8)
	if p.heroNow > 8 {
		p.hero(lights)
	}
	if p.view != viewLink {
		p.header(lights)
	}
	p.introV = p.ease("intro", 1, 4.5)
	k := clampf(p.introV*1.4, 0, 1) * p.ease("vt", 1, 10)
	st := op.Offset(image.Pt(0, p.px((1-k)*12))).Push(p.ops)
	fade := paint.PushOpacity(p.ops, k)
	switch p.view {
	case viewLink:
		p.linkView()
	case viewSettings:
		p.settingsView(p.top(lights))
	case viewAPI:
		p.apiView(p.top(lights))
	case viewSchedules:
		p.schedulesView(p.top(lights))
	case viewRule:
		p.ruleView(p.top(lights))
	case viewShow:
		p.showView(p.top(lights))
	case viewDesign:
		p.designView(p.top(lights))
	case viewIcons:
		p.iconsView(p.top(lights))
	case viewGroup:
		p.groupView(p.top(lights), lights)
	case viewDetail:
		d, _ := p.find(p.sel)
		p.detailView(d, p.top(lights))
	default:
		p.mainView(lights)
	}
	fade.Pop()
	st.Pop()
	p.drawPicker()
	p.drawIconPicker()
	p.drawConfirm()
	p.WantH = clampf(p.need, 380, p.dh())
	if p.view == viewMain {
		p.HomeH = p.WantH
	}
	if p.animating && p.Live {
		gtx.Execute(op.InvalidateCmd{})
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (p *Panel) restart(key string) {
	if p.Static {
		return
	}
	a := p.an[key]
	if a == nil {
		a = new(float32)
		p.an[key] = a
	}
	*a = 0
}
func (p *Panel) find(id string) (homebase.Device, bool) {
	for _, d := range p.devs {
		if d.ID == id {
			return d, true
		}
	}
	return homebase.Device{}, false
}

func (p *Panel) dh() float32 {
	if p.MaxH > 0 {
		return p.MaxH
	}
	return p.h
}

func (p *Panel) reach(y float32) {
	if y > p.need {
		p.need = y
	}
}

func (p *Panel) heroH() float32 { return clampf(p.dh()-440, 120, 232) }

func (p *Panel) heroTarget(lights []homebase.Device) float32 {
	if !p.settings.ShowHouse || len(lights) == 0 {
		return 0
	}
	switch p.view {
	case viewMain, viewDesign:
		return p.heroH()
	case viewDetail, viewGroup:
		return clampf(p.dh()-470, 120, 252)
	}
	return 0
}

func (p *Panel) hero(lights []homebase.Device) {
	sel := ""
	if p.view == viewDetail {
		for _, d := range lights {
			if d.ID == p.sel {
				sel = d.ID
			}
		}
	}
	p.camPan()
	p.house(p.w/2, 70, p.heroNow, lights, sel)
	p.camWheel()
	if p.placing && sel != "" {
		p.textC(p.w/2, 78, "Tap a spot to place this light", 12.5, cAmber, reg)
	}
}

func (p *Panel) camWheel() {
	if dy := p.wheel("herowheel", 0, 70, p.w, p.heroNow); dy != 0 {
		p.camZ = clampf(p.camZ*(1-dy*0.0016), 1, 3.2)
	}
}

func (p *Panel) camPan() {
	ht := p.area("heropan", 0, 70, p.w, p.heroNow)
	if ht.Down && !p.panDown {
		p.panLX, p.panLY = ht.X, ht.Y
	}
	if ht.Down && p.panDown && ht.Moved {
		p.camX += ht.X - p.panLX
		p.camY += ht.Y - p.panLY
		p.panLX, p.panLY = ht.X, ht.Y
	}
	p.panDown = ht.Down
	if ht.Clicked {
		if p.gtx.Now.Sub(p.heroClick) < 380*time.Millisecond {
			p.camZ, p.camX, p.camY = 1, 0, 0
		}
		p.heroClick = p.gtx.Now
	}
	lim := (p.camZ - 1) * p.w * 0.42
	p.camX, p.camY = clampf(p.camX, -lim, lim), clampf(p.camY, -lim*0.8, lim*0.8)
}

func (p *Panel) top(lights []homebase.Device) float32 {
	if p.heroNow > 8 {
		return 70 + p.heroNow + 6
	}
	return 70
}
func (p *Panel) glass(x, y, w, h, r float32) {
	p.fillRR(x, y, w, h, r, rgba(255, 255, 255, 6))
	p.strokeRR(x, y, w, h, r, 1, rgba(255, 255, 255, 9))
}

func (p *Panel) header(lights []homebase.Device) {
	on, off := 0, 0
	for _, d := range lights {
		if d.On && d.Online {
			on++
		}
		if !d.Online {
			off++
		}
	}
	back := p.view != viewMain
	p.glass(16, 16, 36, 36, 12)
	if back {
		p.icoBack(34, 34, cWhite)
	} else {
		p.icoHome(34, 34, rgb(220, 224, 240))
	}
	if p.area("home", 16, 16, 36, 36).Clicked && back {
		switch p.view {
		case viewAPI, viewSchedules, viewShow, viewDesign, viewIcons:
			p.view = viewSettings
		case viewRule:
			p.view = viewSchedules
		default:
			p.view, p.sel = viewMain, ""
		}
	}
	name := p.be.HomeName()
	if name == "" {
		name = "My Home"
	}
	p.textFit(62, 15, 210, name, 16, cWhite, semi)
	sub := "No lights on"
	if on > 0 {
		sub = fmt.Sprintf("%d of %d lights on", on, len(lights))
	}
	p.disc(67, 45, 3, map[bool]color.NRGBA{true: cGreen, false: cFaint}[on > 0])
	w := p.text(76, 36, sub, 12, cDim, reg)
	if off > 0 {
		p.text(76+w, 36, fmt.Sprintf(" · %d offline", off), 12, cAmber, reg)
	}
	sg := p.ease("hdr:set", b2f(p.view == viewSettings), 14)
	p.fillRR(290, 16, 36, 36, 18, mix(rgba(255, 255, 255, 6), rgba(90, 150, 255, 40), sg))
	p.icoSliders(308, 34, rgb(210, 214, 232))
	if p.area("gear", 290, 16, 36, 36).Clicked {
		p.view = map[bool]int{true: viewMain, false: viewSettings}[p.view == viewSettings]
	}
	pw := p.ease("hdr:pow", b2f(on > 0), 12)
	p.glow(362, 34, 38, cBlue, 0.8*pw)
	p.gradRR(342, 14, 40, 40, 20, mix(rgb(52, 58, 86), rgb(88, 150, 255), pw), mix(rgb(40, 45, 70), rgb(44, 96, 230), pw), false)
	p.icoPower(362, 34, 7, cWhite)
	if p.area("power", 342, 14, 40, 40).Clicked {
		p.be.AllLights(on == 0)
		p.scene = ""
	}
}

func (p *Panel) mainView(lights []homebase.Device) {
	y := p.top(lights)
	cl := p.clipTo(0, y-4, p.w, p.h-(y-4))
	for _, g := range p.tileGlow {
		p.glow(g.x, g.y, 130, g.c, g.k)
	}
	cl.Pop()
	p.tileGlow = p.tileGlow[:0]
	p.scenes(y)
	y += 66
	p.pills(y)
	y += 40
	p.wholeRow(y, lights)
	y += 34
	p.list(y)
	p.reach(y + p.contentH + 14)
}
func (p *Panel) scenes(y float32) {
	cw := (p.w - 28 - 4*8) / 5
	icons := []func(float32, float32, color.NRGBA){p.icoSun, p.icoSofa, p.icoFilm, p.icoMoon, p.icoDoor}
	for i, n := range homebase.SceneNames {
		x := 14 + float32(i)*(cw+8)
		ht := p.area("scene:"+n, x, y, cw, 56)
		pop := p.pressScale("scene:"+n, x, y, cw, 56, ht.Down)
		sel := p.ease("sc:"+n, b2f(p.scene == n), 14)
		p.glow(x+cw/2, y+28, 56, cBlue, 0.5*sel)
		p.gradRR(x, y, cw, 56, 16, mix(rgb(24, 29, 50), rgb(24, 44, 92), sel), mix(rgb(18, 22, 40), rgb(20, 36, 78), sel), false)
		p.strokeRR(x, y, cw, 56, 16, 1.1, mix(rgb(46, 54, 84), rgb(90, 150, 255), sel))
		col := mix(cDim, rgb(210, 226, 255), sel)
		icons[i](x+cw/2, y+20, col)
		p.textC(x+cw/2, y+34, n, 11.5, col, reg)
		pop()
		if ht.Clicked {
			p.be.Scene(n)
			p.scene = n
		}
	}
}

func (p *Panel) rooms() []string {
	var out []string
	seen := map[string]bool{}
	for _, d := range p.devs {
		if !seen[d.Room] {
			seen[d.Room], out = true, append(out, d.Room)
		}
	}
	return out
}

func (p *Panel) pills(y float32) {
	items := append([]string{""}, p.rooms()...)
	defer p.clipTo(0, y, p.w, 30).Pop()
	total := float32(0)
	for _, r := range items {
		l := r
		if l == "" {
			l = "All"
		}
		total += p.textW(l, 12.5, reg) + 36
	}
	p.pill = clampf(p.pill-p.pillIn, 0, max(0, total-p.w+14))
	p.pillIn = 0
	pr := float32(16)
	if p.pillDrag {
		pr = 70
	}
	p.pillDrag = false
	bg := p.area("pillsbg", 0, y, p.w, 30)
	if dx, _, on := p.dragDelta("pillsbg", bg); on {
		p.pill = clampf(p.pill-dx, 0, max(0, total-p.w+14))
		p.pillDrag = true
	}
	x := 14 - p.ease("pillx", p.pill, pr)
	for _, r := range items {
		l := r
		if l == "" {
			l = "All"
		}
		tw := p.textW(l, 12.5, reg)
		on := 0
		for _, d := range p.devs {
			if d.Kind == homebase.KindLight && d.On && d.Online && (r == "" || d.Room == r) {
				on++
			}
		}
		w := tw + 24
		mark := r != "" && p.settings.Icons["r:"+r] != ""
		if mark {
			w += 18
		}
		if on > 0 && r != "" {
			w += 24
		}
		sel := p.ease("pill:"+r, b2f(p.filter == r), 14)
		p.fillRR(x, y, w, 28, 14, mix(rgba(255, 255, 255, 1), rgb(48, 54, 78), sel))
		tx := x + 12
		if mark {
			p.roomMark(r, x+10, y+7, 14, mix(cDim, cWhite, sel))
			tx += 18
		}
		p.text(tx, y+4, l, 12.5, mix(cDim, cWhite, sel), map[bool]font.Weight{true: semi, false: reg}[p.filter == r])
		if on > 0 && r != "" {
			p.disc(x+w-15, y+14, 9, rgb(240, 160, 80))
			p.textC(x+w-15, y+6, fmt.Sprint(on), 11, rgb(40, 22, 8), semi)
		}
		ht := p.area("pillhit:"+r, x, y, w, 28)
		if dx, _, on := p.dragDelta("pill:"+r, ht); on {
			p.pill = clampf(p.pill-dx, 0, max(0, total-p.w+14))
			p.pillDrag = true
		}
		if ht.Clicked {
			p.filter, p.scroll = r, 0
		}
		x += w + 8
	}
	p.pillIn += p.wheel("pillwheel", 0, y, p.w, 30)
}

func (p *Panel) wholeRow(y float32, lights []homebase.Device) {
	on, n := 0, 0
	for _, d := range lights {
		if p.filter == "" || d.Room == p.filter {
			n++
			if d.On && d.Online {
				on++
			}
		}
	}
	title := "Whole home"
	if p.filter != "" {
		title = p.filter
	}
	w := p.textFit(14, y, 190, title, 13, cWhite, semi)
	p.text(14+w+6, y+1, fmt.Sprintf("· %d of %d on", on, n), 12, cDim, reg)
	cur := p.w - 14
	for _, b := range []string{"All off", "All on", "Style"} {
		bw := p.textW(b, 11, reg) + 20
		x := cur - bw
		cur = x - 8
		p.fillRR(x, y-3, bw, 24, 12, rgba(255, 255, 255, 4))
		p.strokeRR(x, y-3, bw, 24, 12, 1, rgba(255, 255, 255, 12))
		p.textC(x+bw/2, y, b, 11, cWhite, reg)
		if p.area("all:"+b, x, y-3, bw, 24).Clicked {
			if b == "Style" {
				p.view, p.customOpen, p.sceneTab = viewGroup, false, ""
				continue
			}
			if p.filter == "" {
				p.be.AllLights(b == "All on")
			} else {
				p.be.RoomLights(p.filter, b == "All on")
			}
			p.scene = ""
		}
	}
}

func (p *Panel) list(top float32) {
	vh := p.h - top
	p.tileN = 0
	bg := p.area("listbg", 0, top, p.w, vh)
	if _, dy, on := p.dragDelta("listbg", bg); on {
		p.scrollIn -= dy
		p.listDrag = true
	}
	if p.listDrag {
		p.scrollVel = 0.75*p.scrollVel + 0.25*(p.scrollIn/max(p.dt, 0.008))
	} else if math.Abs(float64(p.scrollVel)) > 8 {
		p.scrollIn += p.scrollVel * p.dt
		p.scrollVel *= float32(math.Exp(float64(-p.dt * 3.6)))
		p.animating = true
	} else {
		p.scrollVel = 0
	}
	drag := p.listDrag
	p.listDrag = false
	p.scroll = clampf(p.scroll+p.scrollIn, 0, max(0, p.contentH-vh+12))
	p.scrollIn = 0
	sr := float32(18)
	if drag {
		sr = 90
	}
	sc := p.ease("scroll", p.scroll, sr)
	defer p.clipTo(0, top, p.w, vh).Pop()
	cells, ch := p.arrange()
	for _, c := range cells {
		y := top + c.y - sc
		switch {
		case c.head != "":
			tx := c.x
			if p.settings.Icons["r:"+c.head] != "" && p.roomMark(c.head, c.x, y+1, 12, cFaint) {
				tx += 17
			}
			p.text(tx, y+2, strings.ToUpper(c.head), 10, cFaint, semi)
		case y+c.h <= top || y >= p.h:
		case c.d.Kind == homebase.KindSwitch:
			p.switchRow(c.d, c.x, y, c.w, c.h)
		default:
			p.lightTile(c.d, c.x, y, c.w, c.h)
		}
	}
	p.contentH = ch
	if p.contentH > vh {
		bh := vh * vh / p.contentH
		p.fillRR(p.w-5, top+(vh-bh)*sc/max(1, p.contentH-vh), 2.5, bh, 1.25, rgba(255, 255, 255, 12))
	}
	p.gradRR(0, p.h-22, p.w, 22, 0, rgba(8, 11, 28, 0), rgb(8, 11, 28), false)
	p.scrollIn += p.wheel("listwheel", 0, top, p.w, vh)
}

func (p *Panel) deviceIcon(d homebase.Device, cx, cy, size float32, c color.NRGBA) {
	kind, val := p.iconChoice("d:" + d.ID)
	switch {
	case kind == "i" && p.pic(val, cx-size/2, cy-size/2-0, size, size, size/3.2):
	case kind == "b" && p.builtinIcon(val) != nil:
		p.builtinIcon(val)(cx, cy, c)
	default:
		p.defaultIcon(d, cx, cy, c)
	}
}

func (p *Panel) sub(d homebase.Device) string {
	switch {
	case !d.Online:
		return "Offline · " + d.Room
	case !d.On:
		return "Off · " + d.Room
	case d.Kind == homebase.KindSwitch:
		return "On · " + d.Room
	}
	return fmt.Sprintf("%d%% · %s", d.Bright, d.Room)
}

func (p *Panel) lightTile(d homebase.Device, x, y, w, h float32) {
	on, id := d.On && d.Online, d.ID
	body := p.area("tile:"+id, x, y, w, h)
	ico := p.area("tog:"+id, x+10, y+12, 38, 38)
	al, dy := p.stagger(p.tileN)
	p.tileN++
	defer paint.PushOpacity(p.ops, al).Pop()
	if !d.Online {
		defer paint.PushOpacity(p.ops, 0.5).Pop()
	}
	y += dy
	defer p.pressScale("tile:"+id, x, y, w, h, body.Down && !body.Moved)()
	acc := accent(d)
	hv := p.ease("t:"+id, b2f(on), 14)
	hover := p.ease("th:"+id, b2f(body.Hover || body.Down), 16)
	p.tileGlow = append(p.tileGlow, spillPt{x + w/2, y + h/2, acc, 0.3 * hv * al, 0})
	p.gradRR(x, y, w, h, 18, mix(mix(rgb(21, 26, 44), rgb(30, 36, 58), hover), mix(rgb(52, 40, 38), mix(rgb(20, 20, 30), acc, 0.22), 0.5), hv), mix(rgb(17, 21, 38), mix(rgb(24, 18, 28), acc, 0.1), hv), false)
	p.strokeRR(x, y, w, h, 18, 1.2, mix(rgb(40, 48, 74), acc, hv))
	p.fillRR(x+10, y+12, 38, 38, 12, mix(rgb(30, 36, 58), mix(acc, rgb(255, 235, 210), 0.45), hv))
	p.deviceIcon(d, x+29, y+31, 38, mix(cDim, rgb(60, 36, 20), hv))
	p.textFit(x+58, y+11, w-66, d.Name, 13.5, cWhite, semi)
	p.textFit(x+58, y+32, w-66, p.sub(d), 11.5, mix(cFaint, rgb(190, 160, 140), hv), reg)
	if hv > 0.02 {
		p.fillRR(x+14, y+h-6, w-28, 3, 1.5, rgba(255, 255, 255, uint8(6*hv)))
		p.gradRR(x+14, y+h-6, max(3, (w-28)*float32(d.Bright)/100), 3, 1.5, acc, mix(acc, rgb(255, 215, 150), 0.6), true)
	}
	if !d.Online {
		return
	}
	switch {
	case ico.Clicked:
		p.be.SetPower(id, !d.On)
		p.scene = ""
	case body.Clicked:
		p.view, p.sel, p.scroll = viewDetail, id, 0
	case body.Moved && math.Abs(float64(body.Y-body.SY)) > math.Abs(float64(body.X-body.SX)):
		if _, dy, on := p.dragDelta("tile:"+id, body); on {
			p.scrollIn -= dy
			p.listDrag = true
		}
	case body.Moved && (body.Down || body.Released) && !ico.Down:
		p.throttled("dim:"+id, int(clampf((body.X-x)/w, 0.01, 1)*100), body.Released, func(v int) { p.be.SetBright(id, v) })
	}
}

func (p *Panel) switchRow(d homebase.Device, x, y, w, h float32) {
	on, id := d.On && d.Online, d.ID
	body := p.area("row:"+id, x, y, w, h)
	ico := p.area("rowtog:"+id, x+12, y+12, 30, 30)
	pow := p.area("rowpow:"+id, x+w-48, y+10, 36, 34)
	al, dy := p.stagger(p.tileN)
	p.tileN++
	defer paint.PushOpacity(p.ops, al).Pop()
	if !d.Online {
		defer paint.PushOpacity(p.ops, 0.5).Pop()
	}
	y += dy
	defer p.pressScale("row:"+id, x, y, w, h, body.Down)()
	hv := p.ease("t:"+id, b2f(on), 14)
	acc := cAmberLine
	p.glow(x+w/2, y+h/2, 110, acc, 0.12*hv)
	p.gradRR(x, y, w, h, 18, mix(rgb(20, 25, 42), rgb(54, 42, 38), hv), mix(rgb(17, 21, 38), rgb(34, 26, 30), hv), false)
	p.strokeRR(x, y, w, h, 18, 1.1, mix(rgb(44, 52, 80), acc, hv))
	p.fillRR(x+12, y+12, 30, 30, 10, mix(rgb(34, 40, 62), mix(acc, rgb(255, 235, 210), 0.45), hv))
	p.deviceIcon(d, x+27, y+27, 30, mix(cDim, rgb(60, 36, 20), hv))
	p.textFit(x+54, y+8, w-120, d.Name, 14, cWhite, semi)
	p.textFit(x+54, y+29, w-120, p.sub(d), 11.5, mix(cFaint, rgb(190, 160, 140), hv), reg)
	p.fillRR(x+w-48, y+10, 36, 34, 17, mix(rgb(34, 40, 62), rgb(88, 150, 255), hv))
	p.icoPower(x+w-30, y+27, 6, mix(cDim, cWhite, hv))
	switch {
	case (ico.Clicked || pow.Clicked) && d.Online:
		p.be.SetPower(id, !d.On)
		p.scene = ""
	case body.Clicked:
		p.view, p.sel, p.scroll = viewDetail, id, 0
	case body.Moved:
		if _, dy, on := p.dragDelta("row:"+id, body); on {
			p.scrollIn -= dy
			p.listDrag = true
		}
	}
}

func (p *Panel) slider(tag string, x, y, w, h float32, v int, c1, c2 color.NRGBA, set func(int)) {
	p.sliderDraw(tag, x, y, w, h, v, func() { p.gradRR(x, y, w, h, 14, c1, c2, true) }, set)
}

func (p *Panel) rainbow(x, y, w, h float32) {
	defer clip.UniformRRect(p.rect(x, y, w, h), p.px(12)).Push(p.ops).Pop()
	for i := 0; i < 6; i++ {
		sx := x + w*float32(i)/6
		st := clip.Rect(p.rect(sx, y, w/6+1, h)).Push(p.ops)
		paint.LinearGradientOp{Stop1: p.pt(sx, y), Stop2: p.pt(sx+w/6, y), Color1: hsvc(float64(i*60), 1, 1), Color2: hsvc(float64((i+1)*60), 1, 1)}.Add(p.ops)
		paint.PaintOp{}.Add(p.ops)
		st.Pop()
	}
}

func (p *Panel) sliderDraw(tag string, x, y, w, h float32, v int, track func(), set func(int)) {
	ht := p.area(tag, x, y, w, h)
	track()
	dn := p.ease("knob:"+tag, b2f(ht.Down), 18)
	kw := 16 + 6*dn
	kx := x + 2 + (w-20)*float32(v)/100 - 3*dn
	p.glow(kx+kw/2, y+h/2, 30, rgb(255, 255, 255), 0.35*dn)
	p.fillRR(kx, y+3, kw, h-6, 11, rgb(250, 250, 252))
	p.fillRR(kx+kw/2-1, y+h/2-7, 2, 14, 1, rgb(190, 194, 210))
	if ht.Down {
		p.throttled(tag, int(clampf((ht.X-x-10)/(w-20), 0, 1)*100), false, set)
	}
	if ht.Released {
		p.throttled(tag, int(clampf((ht.X-x-10)/(w-20), 0, 1)*100), true, set)
	}
}

var swatchHue = []float64{0, 28, 52, 125, 180, 230, 300}

func (p *Panel) detailView(d homebase.Device, top float32) {
	id := d.ID
	p.textFit(16, top+2, 270, d.Name, 18, cWhite, semi)
	st := "Off"
	switch {
	case !d.Online:
		st = "Offline"
	case d.On:
		st = "On"
	}
	tw := p.text(16, top+26, d.Room+" · "+st, 12, cDim, reg)
	if id != "*" {
		px := 16 + tw + 12
		p.fillRR(px, top+23, 98, 22, 11, rgba(255, 255, 255, 5))
		p.strokeRR(px, top+23, 98, 22, 11, 1, rgba(255, 255, 255, 14))
		p.textC(px+49, top+27, "Change room", 11, cDim, reg)
		if p.area("chroom", px, top+23, 98, 22).Clicked {
			p.picker = d.ID
			p.edRoom.SetText("")
		}
		if p.settings.ShowHouse && d.Kind == homebase.KindLight {
			lbl := map[bool]string{true: "Cancel", false: "Place in house"}[p.placing]
			qx, qw := px+98+8, p.textW(lbl, 11, reg)+24
			if qx+qw < p.w-70 {
				p.fillRR(qx, top+23, qw, 22, 11, mix(rgba(255, 255, 255, 5), rgba(255, 190, 90, 40), b2f(p.placing)))
				p.strokeRR(qx, top+23, qw, 22, 11, 1, mix(rgba(255, 255, 255, 14), rgb(255, 190, 90), b2f(p.placing)))
				p.textC(qx+qw/2, top+27, lbl, 11, mix(cDim, cAmber, b2f(p.placing)), reg)
				if p.area("place", qx, top+23, qw, 22).Clicked {
					p.placing = !p.placing
				}
			}
		}
		ix := px + 98 + 8
		if p.settings.ShowHouse && d.Kind == homebase.KindLight {
			ix += p.textW(map[bool]string{true: "Cancel", false: "Place in house"}[p.placing], 11, reg) + 24 + 8
		}
		if ix+50 < p.w-70 {
			p.fillRR(ix, top+23, 50, 22, 11, rgba(255, 255, 255, 5))
			p.strokeRR(ix, top+23, 50, 22, 11, 1, rgba(255, 255, 255, 14))
			p.textC(ix+25, top+27, "Icon", 11, cDim, reg)
			if p.area("dicon", ix, top+23, 50, 22).Clicked {
				p.iconKey = "d:" + d.ID
			}
		}
	}
	hv := p.ease("t:"+id, b2f(d.On && d.Online), 14)
	p.glow(p.w-38, top+16, 34, cBlue, 0.7*hv)
	p.gradRR(p.w-58, top-4, 40, 40, 20, mix(rgb(52, 58, 86), rgb(88, 150, 255), hv), mix(rgb(40, 45, 70), rgb(44, 96, 230), hv), false)
	p.icoPower(p.w-38, top+16, 7, cWhite)
	if p.area("dpow", p.w-58, top-4, 40, 40).Clicked && d.Online {
		p.be.SetPower(id, !d.On)
	}
	if d.Kind == homebase.KindSwitch {
		word := map[bool]string{true: "On", false: "Off"}[d.On && d.Online]
		p.text(14, top+50, word, 56, map[bool]color.NRGBA{true: cWhite, false: cFaint}[d.On && d.Online], font.Light)
		return
	}
	shown := p.ease("num:"+id, float32(d.Bright), 14)
	num := fmt.Sprint(int(shown + 0.5))
	if !d.On {
		num = "0"
	}
	nw := p.text(14, top+46, num, 52, map[bool]color.NRGBA{true: cWhite, false: cFaint}[d.On], font.Light)
	p.text(14+nw+4, top+78, "%", 18, cBlue, reg)
	modes := []string{"White"}
	if d.HasColour {
		modes = append(modes, "Colour")
	}
	if d.HasScene {
		modes = append(modes, "Scene")
	}
	if len(modes) > 1 {
		for i, m := range modes {
			bw := float32(66)
			bx := p.w - 16 - float32(len(modes)-i)*bw - float32(len(modes)-1-i)*6
			on := (m == "White" && d.Mode != "colour" && !p.sceneView(d)) || (m == "Colour" && d.Mode == "colour" && !p.sceneView(d)) || (m == "Scene" && p.sceneView(d))
			sel := p.ease(fmt.Sprint("mode", m, id), b2f(on), 14)
			p.fillRR(bx, top+70, bw, 28, 14, mix(rgba(255, 255, 255, 4), rgb(48, 54, 78), sel))
			p.textC(bx+bw/2, top+74, m, 12.5, mix(cDim, cWhite, sel), reg)
			if p.area("mode:"+m, bx, top+70, bw, 28).Clicked && d.Online {
				switch m {
				case "White":
					p.sceneTab = ""
					p.be.SetWhite(id)
				case "Colour":
					p.sceneTab = ""
					p.be.SetColour(id, d.Hue, max(d.Sat, 100))
				case "Scene":
					p.sceneTab = id
				}
			}
		}
	}
	y := top + 112
	if p.sceneView(d) {
		y = p.effectsGrid(d, y)
		p.reach(y + 16)
		return
	}
	acc := lightColour(d)
	p.slider("bright:"+id, 16, y, p.w-32, 44, d.Bright, rgb(30, 34, 54), mix(acc, rgb(255, 255, 255), 0.35), func(v int) { p.be.SetBright(id, max(v, 1)) })
	y += 58
	if d.HasTemp && d.Mode != "colour" {
		p.text(16, y, "Temperature", 12, cDim, reg)
		p.slider("temp:"+id, 16, y+20, p.w-32, 30, d.Temp, rgb(255, 170, 90), rgb(170, 205, 255), func(v int) { p.be.SetTemp(id, v) })
		y += 62
	}
	if d.HasColour {
		p.text(16, y, "Colour", 12, cDim, reg)
		if p.customOpen {
			bw := float32(76)
			bx := p.w - 16 - bw
			p.fillRR(bx, y-4, bw, 24, 12, rgba(255, 255, 255, 5))
			p.strokeRR(bx, y-4, bw, 24, 12, 1, rgba(255, 255, 255, 14))
			p.textC(bx+bw/2, y-1, "Presets", 11.5, cDim, reg)
			if p.area("presets", bx, y-4, bw, 24).Clicked {
				p.customOpen = false
			}
			sat := max(d.Sat, 60)
			p.sliderDraw("hue:"+id, 16, y+22, p.w-32, 30, d.Hue*100/360, func() { p.rainbow(16, y+22, p.w-32, 30) }, func(v int) { p.be.SetColour(id, v*359/100, sat) })
			hue := d.Hue
			p.sliderDraw("sat:"+id, 16, y+62, p.w-32, 30, d.Sat, func() { p.gradRR(16, y+62, p.w-32, 30, 14, rgb(255, 255, 255), hsvc(float64(hue), 1, 1), true) }, func(v int) { p.be.SetColour(id, hue, v) })
			y += 100
		} else {
			n := float32(len(swatchHue) + 1)
			sw := (p.w - 32 - (n-1)*8) / n
			for i, h := range swatchHue {
				x := 16 + float32(i)*(sw+8)
				c := hsvc(h, 1, 1)
				on := d.Mode == "colour" && math.Abs(float64(d.Hue)-h) < 14
				sel := p.ease(fmt.Sprint("sw", i, id), b2f(on), 14)
				p.glow(x+sw/2, y+20+sw/2, sw*1.1, c, 0.45*sel)
				p.fillRR(x, y+20-2*sel, sw, sw, 12, c)
				p.strokeRR(x-2, y+18-2*sel, sw+4, sw+4, 14, 2, rgba(255, 255, 255, uint8(255*sel)))
				if p.area(fmt.Sprint("swh:", id, i), x, y+20, sw, sw).Clicked && d.Online {
					p.be.SetColour(id, int(h), 100)
				}
			}
			cx := 16 + float32(len(swatchHue))*(sw+8)
			p.rainbow(cx, y+20, sw, sw)
			p.strokeRR(cx, y+20, sw, sw, 12, 1.4, rgba(255, 255, 255, 60))
			p.text(cx+sw/2-4, y+20+sw/2-9, "+", 18, rgba(20, 22, 40, 230), semi)
			if p.area("customswatch", cx, y+20, sw, sw).Clicked {
				p.customOpen = true
			}
			y += 20 + sw
		}
	}
	if p.settings.ShowHouse && y+14+44+16 <= p.dh() {
		p.glass(16, y+14, p.w-32, 44, 14)
		p.icoSpark(38, y+36, cDim)
		p.text(58, y+21, "Scroll the home to zoom, drag to pan, double-click to reset.", 11, cDim, reg)
		p.text(58, y+36, "Tap a glow to toggle that light.", 11, cDim, reg)
		y += 58
	}
	p.reach(y + 16)
}
func (p *Panel) toggleRow(y float32, label string, on bool, f func()) {
	p.glass(16, y, p.w-32, 50, 16)
	p.text(32, y+14, label, 14, cWhite, reg)
	t := p.ease("tg:"+label, b2f(on), 14)
	p.fillRR(p.w-76, y+12, 44, 26, 13, mix(rgb(52, 58, 86), rgb(88, 150, 255), t))
	p.disc(p.w-63+t*18, y+25, 10, cWhite)
	if p.area("trow:"+label, 16, y, p.w-32, 50).Clicked {
		f()
	}
}

func (p *Panel) buttonRow(y float32, label string, c color.NRGBA, f func()) {
	p.glass(16, y, p.w-32, 50, 16)
	p.text(32, y+14, label, 14, c, reg)
	if p.area("brow:"+label, 16, y, p.w-32, 50).Clicked {
		f()
	}
}

func (p *Panel) settingsView(top float32) {
	s := p.settings
	p.text(16, top+8, "Settings", 18, cWhite, semi)
	y0 := top + 44
	vh := p.dh() - y0
	const pitch, rows = 54, 12
	content := float32(rows*pitch) + 12
	p.autoState.sscroll = clampf(p.autoState.sscroll+p.wheel("setwheel", 0, y0, p.w, vh), 0, max(0, content-vh))
	defer p.clipTo(0, y0, p.w, vh).Pop()
	y := y0 + 6 - p.autoState.sscroll
	row := func(i int) float32 { return y + float32(i*pitch) }
	p.toggleRow(row(0), "Home view", s.ShowHouse, func() { s.ShowHouse = !s.ShowHouse; p.be.SaveSettings(s) })
	p.toggleRow(row(1), "Start with Windows", s.StartWithWindows, func() { s.StartWithWindows = !s.StartWithWindows; p.be.SaveSettings(s) })
	p.toggleRow(row(2), "24-hour time", s.Clock24, func() { s.Clock24 = !s.Clock24; p.be.SaveSettings(s) })
	p.navRow(row(3), "Schedules", p.ruleSummary(), func() { p.view, p.autoState.scroll = viewSchedules, 0 })
	p.navRow(row(4), "Light show", p.showSummary(), func() { p.view, p.autoState.showInit = viewShow, false })
	p.navRow(row(5), "Home design", p.be.Plan().Name, func() { p.view = viewDesign })
	p.navRow(row(6), "Icons", p.iconSummary(), func() { p.view, p.autoState.iscroll = viewIcons, 0 })
	p.navRow(row(7), "Local API", map[bool]string{true: fmt.Sprintf("On, port %d", s.APIPort), false: "Off"}[s.LocalAPI], func() { p.view, p.apiInit = viewAPI, false })
	p.buttonRow(row(8), "Refresh devices", cWhite, p.be.Refresh)
	p.buttonRow(row(9), "Unlink Tuya account", rgb(255, 150, 130), p.be.Unlink)
	p.buttonRow(row(10), "Quit", cWhite, p.be.Quit)
	p.reach(y0 + content)
}

func (p *Panel) iconSummary() string {
	if n := len(p.settings.Icons); n > 0 {
		return fmt.Sprintf("%d set", n)
	}
	return "Defaults"
}

func (p *Panel) qr(payload string) [][]bool {
	if payload != p.qrFor {
		p.qrFor, p.qrBits = payload, nil
		if q, err := qrcode.New(payload, qrcode.Medium); err == nil {
			p.qrBits = q.Bitmap()
		}
	}
	return p.qrBits
}

func (p *Panel) PreviewCode(code string, show bool) {
	p.edCode.SetText(code)
	p.showCode = show
}

func (p *Panel) ShowPicker(id string) { p.view, p.sel, p.picker = viewDetail, id, id }

func (p *Panel) PreviewState(custom, placing bool) {
	p.customOpen, p.placing = custom, placing
}

func (p *Panel) ShowScene(id string) { p.view, p.sel, p.sceneTab = viewDetail, id, id }

func (p *Panel) ruleSummary() string {
	n := 0
	for _, r := range p.be.Rules() {
		if r.Enabled {
			n++
		}
	}
	if n == 0 {
		return "None"
	}
	return fmt.Sprintf("%d active", n)
}

func (p *Panel) showSummary() string {
	if st := p.be.ShowState(); st.Running {
		return map[string]string{"screen": "Screen", "sound": "Sound"}[st.Mode] + " on"
	}
	return "Off"
}
