package panel

import (
	"fmt"
	"image/color"
	"math/rand"
	"strings"
	"time"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeplan"
	"moreno/tuyatray/lightshow"
)

type autoState struct {
	rule     homebase.Rule
	idx      int
	scroll   float32
	sscroll  float32
	iscroll  float32
	show     lightshow.Config
	showInit bool
	msg      string
}

type chipSpec struct {
	label string
	on    bool
	f     func()
}

var dayNames = []string{"S", "M", "T", "W", "T", "F", "S"}

var dayLong = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

var actionChips = []struct{ label, action string }{
	{"Turn on", "on"}, {"Turn off", "off"}, {"Dim 25%", "bright:25"}, {"Dim 50%", "bright:50"}, {"Full 100%", "bright:100"},
	{"Cozy", "scene:Cozy"}, {"Movie", "scene:Movie"}, {"Night", "scene:Night"}, {"Bright", "scene:Bright"}, {"Away", "scene:Away"},
}

func (p *Panel) chips(tag string, x, y, w float32, items []chipSpec) float32 {
	cx, cy := x, y
	for i, c := range items {
		cw := p.textW(c.label, 12.5, reg) + 26
		if cx+cw > x+w && cx > x {
			cx, cy = x, cy+38
		}
		sel := p.ease(fmt.Sprint("ch:", tag, i), b2f(c.on), 16)
		p.fillRR(cx, cy, cw, 30, 15, mix(rgba(255, 255, 255, 5), rgb(48, 56, 94), sel))
		p.strokeRR(cx, cy, cw, 30, 15, 1.1, mix(rgba(255, 255, 255, 14), rgb(90, 150, 255), sel))
		p.textC(cx+cw/2, cy+5, c.label, 12.5, mix(cDim, cWhite, sel), reg)
		if p.area(fmt.Sprint("chit:", tag, i), cx, cy, cw, 30).Clicked {
			c.f()
		}
		cx += cw + 8
	}
	return cy + 38
}

func (p *Panel) stepper(tag string, cx, y float32, val string, dec, inc func()) {
	p.fillRR(cx-60, y, 36, 36, 18, rgba(255, 255, 255, 8))
	p.line(cx-51, y+18, cx-33, y+18, 2, cWhite)
	p.fillRR(cx+24, y, 36, 36, 18, rgba(255, 255, 255, 8))
	p.line(cx+33, y+18, cx+51, y+18, 2, cWhite)
	p.line(cx+42, y+9, cx+42, y+27, 2, cWhite)
	p.textC(cx, y-2, val, 30, cWhite, reg)
	if p.area("st-"+tag+"-", cx-60, y, 36, 36).Clicked {
		dec()
	}
	if p.area("st+"+tag+"+", cx+24, y, 36, 36).Clicked {
		inc()
	}
}

func (p *Panel) dayText(days []int) string {
	if len(days) == 0 || len(days) == 7 {
		return "Every day"
	}
	has := map[int]bool{}
	for _, d := range days {
		has[d] = true
	}
	if len(days) == 5 && !has[0] && !has[6] {
		return "Weekdays"
	}
	if len(days) == 2 && has[0] && has[6] {
		return "Weekends"
	}
	var out []string
	for d := 0; d < 7; d++ {
		if has[d] {
			out = append(out, dayLong[d])
		}
	}
	return strings.Join(out, ", ")
}

func (p *Panel) targetText(t string) string {
	switch {
	case t == "all" || t == "":
		return "All lights"
	case t == "everything":
		return "Everything"
	case strings.HasPrefix(t, "room:"):
		return strings.TrimPrefix(t, "room:")
	case strings.HasPrefix(t, "device:"):
		if d, ok := p.find(strings.TrimPrefix(t, "device:")); ok {
			return d.Name
		}
	}
	return t
}

func actionText(a string) string {
	for _, c := range actionChips {
		if c.action == a {
			return c.label
		}
	}
	if k, v, ok := strings.Cut(a, ":"); ok && k == "effect" {
		return v + " effect"
	}
	return a
}

func (p *Panel) ruleLine(r homebase.Rule) string {
	return p.dayText(r.Days) + " · " + p.targetText(r.Target) + " · " + actionText(r.Action)
}

func (p *Panel) schedulesView(top float32) {
	vh := p.dh() - top
	rules := p.be.Rules()
	p.text(16, top+6, "Schedules", 18, cWhite, semi)
	left, _, _ := p.be.TimerState()
	y := top + 46
	p.glass(16, y, p.w-32, 112, 16)
	if left > 0 {
		m, sec := int(left.Minutes()), int(left.Seconds())%60
		p.text(32, y+14, "Everything turns off in", 13, cDim, reg)
		p.text(32, y+34, fmt.Sprintf("%d:%02d", m, sec), 30, cWhite, reg)
		p.animating = true
		if p.button("tm-cancel", 32, y+72, 120, 30, "Cancel timer", false) {
			p.be.CancelTimer()
		}
	} else {
		p.text(32, y+14, "Sleep timer · turn everything off in", 13, cDim, reg)
		bw := (p.w - 32 - 32 - 24) / 4
		for i, m := range []int{15, 30, 60, 120} {
			label := fmt.Sprintf("%d min", m)
			if m >= 60 {
				label = fmt.Sprintf("%d h", m/60)
			}
			if p.button(fmt.Sprint("tm", m), 32+float32(i)*(bw+8), y+50, bw, 44, label, false) {
				p.be.StartTimer(m, "everything", "off")
			}
		}
	}
	ly := y + 128
	vy := ly
	region := vh - (vy - top)
	contentH := float32(len(rules)*66) + 62
	maxS := max(0, contentH-region)
	p.autoState.scroll = clampf(p.autoState.scroll+p.wheel("autowheel", 0, vy, p.w, region), 0, maxS)
	defer p.clipTo(0, vy, p.w, region).Pop()
	ry := vy - p.autoState.scroll
	for i, r := range rules {
		p.glass(16, ry, p.w-32, 58, 16)
		col := cWhite
		if !r.Enabled {
			col = cFaint
		}
		p.timeText(30, ry+13, r.At, col)
		name := r.Name
		if name == "" {
			name = actionText(r.Action)
		}
		nx := float32(130)
		if p.settings.Clock24 {
			nx = 104
		}
		p.textFit(nx, ry+10, p.w-nx-84, name, 13.5, col, semi)
		p.textFit(nx, ry+31, p.w-nx-84, p.ruleLine(r), 11.5, cDim, reg)
		t := p.ease(fmt.Sprint("rule-on", r.ID), b2f(r.Enabled), 14)
		p.fillRR(p.w-76, ry+16, 44, 26, 13, mix(rgb(52, 58, 86), rgb(88, 150, 255), t))
		p.disc(p.w-63+t*18, ry+29, 10, cWhite)
		if p.area(fmt.Sprint("rule-tg", r.ID), p.w-80, ry+10, 52, 38).Clicked {
			rs := append([]homebase.Rule(nil), rules...)
			rs[i].Enabled = !rs[i].Enabled
			p.be.SaveRules(rs)
		}
		if p.area(fmt.Sprint("rule-ed", r.ID), 16, ry, p.w-100, 58).Clicked {
			p.autoState.rule, p.autoState.idx, p.autoState.msg = r, i, ""
			p.view = viewRule
		}
		ry += 66
	}
	if p.button("rule-add", 16, ry+4, p.w-32, 46, "Add a schedule", true) {
		p.autoState.rule = homebase.Rule{Enabled: true, At: time.Now().Add(time.Hour).Format("15:04"), Target: "all", Action: "off"}
		p.autoState.idx, p.autoState.msg = -1, ""
		p.view = viewRule
	}
	p.reach(top + 46 + 128 + float32(len(rules))*66 + 70)
}

func (p *Panel) ruleView(top float32) {
	r := &p.autoState.rule
	hh, mm := 7, 0
	fmt.Sscanf(r.At, "%d:%d", &hh, &mm)
	set := func(h, m int) { r.At = fmt.Sprintf("%02d:%02d", (h+24)%24, (m+60)%60) }
	title := "New schedule"
	if p.autoState.idx >= 0 {
		title = "Edit schedule"
	}
	p.text(16, top+6, title, 18, cWhite, semi)
	y := top + 52
	p.glass(16, y, p.w-32, 64, 16)
	if p.settings.Clock24 {
		p.stepper("h", p.w/2-80, y+14, fmt.Sprintf("%02d", hh), func() { set(hh-1, mm) }, func() { set(hh+1, mm) })
		p.textC(p.w/2, y+10, ":", 28, cDim, reg)
		p.stepper("m", p.w/2+80, y+14, fmt.Sprintf("%02d", mm), func() { set(hh, mm-5) }, func() { set(hh, mm+5) })
	} else {
		pm, h12 := hh >= 12, hh%12
		if h12 == 0 {
			h12 = 12
		}
		to24 := func(n int) int { return n%12 + 12*map[bool]int{true: 1, false: 0}[pm] }
		p.stepper("h", 96, y+14, fmt.Sprintf("%d", h12), func() { set(to24((h12+10)%12+1), mm) }, func() { set(to24(h12%12+1), mm) })
		p.textC(168, y+10, ":", 28, cDim, reg)
		p.stepper("m", 240, y+14, fmt.Sprintf("%02d", mm), func() { set(hh, mm-5) }, func() { set(hh, mm+5) })
		label := map[bool]string{true: "PM", false: "AM"}[pm]
		p.fillRR(p.w-86, y+14, 56, 36, 18, rgb(48, 56, 94))
		p.strokeRR(p.w-86, y+14, 56, 36, 18, 1.1, rgb(90, 150, 255))
		p.textC(p.w-58, y+21, label, 14, cWhite, semi)
		if p.area("ampm", p.w-86, y+14, 56, 36).Clicked {
			set(hh+12, mm)
		}
	}
	y += 80
	p.text(18, y, "Repeat", 12, cDim, reg)
	dw := (p.w - 32 - 6*6) / 7
	has := map[int]bool{}
	for _, d := range r.Days {
		has[d] = true
	}
	for d := 0; d < 7; d++ {
		x := 16 + float32(d)*(dw+6)
		sel := p.ease(fmt.Sprint("day", d), b2f(has[d] || len(r.Days) == 0), 16)
		p.fillRR(x, y+20, dw, 36, 12, mix(rgba(255, 255, 255, 5), rgb(48, 56, 94), sel))
		p.strokeRR(x, y+20, dw, 36, 12, 1.1, mix(rgba(255, 255, 255, 14), rgb(90, 150, 255), sel))
		p.textC(x+dw/2, y+28, dayNames[d], 13, mix(cDim, cWhite, sel), reg)
		if p.area(fmt.Sprint("dayhit", d), x, y+20, dw, 36).Clicked {
			if len(r.Days) == 0 {
				for k := 0; k < 7; k++ {
					r.Days = append(r.Days, k)
				}
			}
			var nd []int
			for k := 0; k < 7; k++ {
				if (k == d) != has[k] {
					nd = append(nd, k)
				}
			}
			if len(nd) == 7 {
				nd = nil
			}
			r.Days = nd
		}
	}
	y += 70
	p.text(18, y, "Lights", 12, cDim, reg)
	targets := []chipSpec{{"Everything", r.Target == "everything", func() { r.Target = "everything" }}, {"All lights", r.Target == "all", func() { r.Target = "all" }}}
	for _, room := range p.rooms() {
		rm := room
		targets = append(targets, chipSpec{rm, r.Target == "room:"+rm, func() { r.Target = "room:" + rm }})
	}
	y = p.chips("tg", 16, y+20, p.w-32, targets) + 6
	p.text(18, y, "Do", 12, cDim, reg)
	var acts []chipSpec
	for _, c := range actionChips {
		a := c.action
		acts = append(acts, chipSpec{c.label, r.Action == a, func() { r.Action = a }})
	}
	for _, e := range homebase.Effects {
		n, label := e, e
		if n == "Night" {
			label = "Night glow"
		}
		acts = append(acts, chipSpec{label, r.Action == "effect:"+n, func() { r.Action = "effect:" + n }})
	}
	y = p.chips("ac", 16, y+20, p.w-32, acts) + 8
	bw := (p.w - 32 - 8) / 2
	rules := p.be.Rules()
	if p.button("rule-save", 16+bw+8, y, bw, 44, "Save", true) {
		out := append([]homebase.Rule(nil), rules...)
		if r.ID == "" {
			r.ID = fmt.Sprintf("%x", rand.Int63())
		}
		if p.autoState.idx >= 0 && p.autoState.idx < len(out) {
			out[p.autoState.idx] = *r
		} else {
			out = append(out, *r)
		}
		p.be.SaveRules(out)
		p.view = viewSchedules
	}
	if p.autoState.idx >= 0 {
		if p.button("rule-del", 16, y, bw, 44, "Delete", false) {
			out := append([]homebase.Rule(nil), rules[:p.autoState.idx]...)
			out = append(out, rules[p.autoState.idx+1:]...)
			p.be.SaveRules(out)
			p.view = viewSchedules
		}
	} else if p.button("rule-cancel", 16, y, bw, 44, "Cancel", false) {
		p.view = viewSchedules
	}
	p.reach(y + 64)
}

func (p *Panel) showView(top float32) {
	st := &p.autoState
	if !st.showInit {
		st.showInit, st.msg = true, ""
		st.show = lightshow.Normalize(p.be.ShowConfig())
		if st.show.Mode == "" {
			st.show.Mode = "screen"
		}
	}
	c := &st.show
	state := p.be.ShowState()
	p.text(16, top+6, "Light show", 18, cWhite, semi)
	y := top + 48
	modes := []chipSpec{{"Screen colours", c.Mode == "screen", func() { c.Mode = "screen" }}, {"PC sound", c.Mode == "sound", func() { c.Mode = "sound" }}}
	y = p.chips("md", 16, y, p.w-32, modes) + 4
	p.text(18, y, "Lights to drive (network-connected bulbs only)", 12, cDim, reg)
	picked := map[string]bool{}
	for _, id := range c.Devices {
		picked[id] = true
	}
	var lights []chipSpec
	lights = append(lights, chipSpec{"All", len(c.Devices) > 0 && len(c.Devices) == p.lightCount(), func() {
		c.Devices = nil
		for _, d := range p.devs {
			if d.Kind == homebase.KindLight {
				c.Devices = append(c.Devices, d.ID)
			}
		}
	}})
	for _, d := range p.devs {
		if d.Kind != homebase.KindLight {
			continue
		}
		id := d.ID
		lights = append(lights, chipSpec{d.Name, picked[id], func() {
			var out []string
			found := false
			for _, x := range c.Devices {
				if x == id {
					found = true
				} else {
					out = append(out, x)
				}
			}
			if !found {
				out = append(out, id)
			}
			c.Devices = out
		}})
	}
	y = p.chips("lt", 16, y+20, p.w-32, lights) + 4
	if c.Mode == "screen" {
		p.text(18, y, "Screen area", 12, cDim, reg)
		areas := []struct {
			label string
			r     [4]float64
		}{{"Whole screen", [4]float64{0, 0, 1, 1}}, {"Left half", [4]float64{0, 0, 0.5, 1}}, {"Right half", [4]float64{0.5, 0, 1, 1}}, {"Top half", [4]float64{0, 0, 1, 0.5}}, {"Bottom half", [4]float64{0, 0.5, 1, 1}}, {"Centre", [4]float64{0.25, 0.25, 0.75, 0.75}}}
		var chips []chipSpec
		for _, a := range areas {
			ar := a
			chips = append(chips, chipSpec{ar.label, c.Region == ar.r || (c.Region == [4]float64{} && ar.r == [4]float64{0, 0, 1, 1}), func() { c.Region = ar.r }})
		}
		y = p.chips("rg", 16, y+20, p.w-32, chips) + 4
		p.toggleRow(y, "Left to right across lights", c.Split, func() { c.Split = !c.Split })
		y += 58
	} else {
		p.text(18, y, fmt.Sprintf("Sensitivity %d%%", max(c.Gain, 20)), 12, cDim, reg)
		p.slider("sens", 16, y+20, p.w-32, 30, (max(c.Gain, 20)-20)*100/380, rgb(30, 34, 54), rgb(120, 170, 255), func(v int) { c.Gain = 20 + v*380/100 })
		y += 62
	}
	mb := c.MaxBright
	if mb < 10 {
		mb = 100
	}
	p.text(18, y, fmt.Sprintf("Maximum brightness %d%%", mb), 12, cDim, reg)
	p.slider("maxb", 16, y+20, p.w-32, 30, mb, rgb(30, 34, 54), rgb(255, 214, 150), func(v int) { c.MaxBright = max(v, 10) })
	y += 66
	if state.Running {
		if p.button("show-stop", 16, y, p.w-32, 46, "Stop light show", false) {
			p.be.StopShow()
		}
		p.text(18, y+54, fmt.Sprintf("Running · %d lights", state.Lights), 12.5, cGreen, reg)
		if state.Skipped > 0 {
			p.text(18, y+72, fmt.Sprintf("%d skipped, not reachable on the network", state.Skipped), 11.5, cAmber, reg)
		}
		y += 80
	} else {
		if p.button("show-start", 16, y, p.w-32, 46, "Start light show", true) {
			if err := p.be.StartShow(*c); err != nil {
				st.msg = err.Error()
			} else {
				st.msg = ""
			}
		}
		msg := st.msg
		if msg == "" && state.Error != "" {
			msg = state.Error
		}
		if msg != "" {
			p.text(18, y+54, msg, 12, rgb(255, 150, 130), reg)
		}
		y += 76
	}
	p.reach(y + 12)
}

func (p *Panel) lightCount() int {
	n := 0
	for _, d := range p.devs {
		if d.Kind == homebase.KindLight {
			n++
		}
	}
	return n
}

func (p *Panel) ShowSchedules() { p.view = viewSchedules }

func (p *Panel) ShowRule(i int) {
	rs := p.be.Rules()
	if i >= 0 && i < len(rs) {
		p.autoState.rule, p.autoState.idx = rs[i], i
	}
	p.view = viewRule
}

func (p *Panel) ShowLightShow() { p.view, p.autoState.showInit = viewShow, false }

func (p *Panel) timeText(x, y float32, at string, c color.NRGBA) float32 {
	var h, m int
	fmt.Sscanf(at, "%d:%d", &h, &m)
	if p.settings.Clock24 {
		return p.text(x, y, fmt.Sprintf("%02d:%02d", h, m), 21, c, reg)
	}
	suf, h12 := "AM", h%12
	if h >= 12 {
		suf = "PM"
	}
	if h12 == 0 {
		h12 = 12
	}
	w := p.text(x, y, fmt.Sprintf("%d:%02d", h12, m), 21, c, reg)
	return w + 3 + p.text(x+w+3, y+8, suf, 11, c, semi)
}

func (p *Panel) designView(top float32) {
	s := p.settings
	cur := s.HomePlan
	if cur == "" {
		cur = "apartment"
	}
	p.text(16, top+6, "Home design", 18, cWhite, semi)
	var chips []chipSpec
	for _, id := range homeplan.Presets() {
		name := id
		if pl, err := homeplan.Preset(id); err == nil {
			name = pl.Name
		}
		pid := id
		chips = append(chips, chipSpec{name, cur == pid, func() { s.HomePlan = pid; p.be.SaveSettings(s) }})
	}
	if p.be.HasCustomPlan() {
		chips = append(chips, chipSpec{"Custom (house.json)", cur == "custom", func() { s.HomePlan = "custom"; p.be.SaveSettings(s) }})
	}
	y := p.chips("plan", 16, top+44, p.w-32, chips)
	p.text(18, y+4, "Make your own: save a design as house.json next to the", 11.5, cDim, reg)
	p.text(18, y+22, "settings, or paste it in the web panel under Home design.", 11.5, cDim, reg)
	p.reach(y + 56)
}

func (p *Panel) ShowDesign() { p.view = viewDesign }

func (p *Panel) ShowIcons() { p.view = viewIcons }

func (p *Panel) ShowIconPicker(key string) { p.iconKey = key }

func (p *Panel) ShowGroup() { p.view = viewGroup }
