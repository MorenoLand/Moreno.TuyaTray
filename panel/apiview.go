package panel

import (
	"fmt"
	"image"
	"net/netip"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"moreno/tuyatray/homebase"
)

type confirmDlg struct {
	title string
	lines []string
	ok    string
	f     func()
}

func (p *Panel) field(ed *widget.Editor, x, y, w, h float32, hint string, mask rune) {
	p.glass(x, y, w, h, 14)
	ed.SingleLine, ed.Mask = true, mask
	g := p.gtx
	g.Constraints = layout.Exact(image.Pt(p.px(w-24), p.px(h-14)))
	st := op.Offset(image.Pt(p.px(x+12), p.px(y+10))).Push(p.ops)
	e := material.Editor(p.th, ed, hint)
	e.Color, e.HintColor, e.TextSize = cWhite, cFaint, unit.Sp(15)
	e.Layout(g)
	st.Pop()
}

func (p *Panel) navRow(y float32, label, value string, f func()) {
	p.glass(16, y, p.w-32, 50, 16)
	p.text(32, y+14, label, 14, cWhite, reg)
	p.strokePoly([]f32.Point{p.pt(p.w-40, y+19), p.pt(p.w-34, y+25), p.pt(p.w-40, y+31)}, 1.8, cDim, false)
	p.textR(p.w-52, y+15, value, 12.5, cDim, reg)
	if p.area("nav:"+label, 16, y, p.w-32, 50).Clicked {
		f()
	}
}

func (p *Panel) button(tag string, x, y, w, h float32, label string, primary bool) bool {
	ht := p.area("btn:"+tag, x, y, w, h)
	hv := p.ease("btn:"+tag, b2f(ht.Hover), 16)
	if primary {
		p.gradRR(x, y, w, h, 14, mix(rgb(88, 150, 255), rgb(120, 175, 255), hv), mix(rgb(44, 96, 230), rgb(60, 116, 245), hv), false)
	} else {
		p.fillRR(x, y, w, h, 14, mix(rgba(255, 255, 255, 6), rgba(255, 255, 255, 12), hv))
		p.strokeRR(x, y, w, h, 14, 1, rgba(255, 255, 255, 14))
	}
	p.textC(x+w/2, y+h/2-10, label, 13.5, cWhite, semi)
	return ht.Clicked
}

func (p *Panel) askConfirm(title string, lines []string, ok string, f func()) {
	p.confirm = &confirmDlg{title, lines, ok, f}
}

func (p *Panel) drawConfirm() {
	c := p.confirm
	if c == nil {
		return
	}
	p.fillRR(0, 0, p.w, p.h, 9, rgba(3, 5, 16, 225))
	p.area("scrim", 0, 0, p.w, p.h)
	h := float32(150 + 20*len(c.lines))
	p.reach(h + 40)
	x, y, w := float32(20), (p.h-h)/2, p.w-40
	p.gradRR(x, y, w, h, 20, rgb(34, 38, 70), rgb(18, 22, 44), false)
	p.strokeRR(x, y, w, h, 20, 1.2, rgb(110, 90, 60))
	p.strokePoly([]f32.Point{p.pt(x+34, y+20), p.pt(x+49, y+46), p.pt(x+19, y+46)}, 2, gHi, true)
	p.line(x+34, y+29, x+34, y+38, 2, gHi)
	p.disc(x+34, y+42, 1.3, gHi)
	p.text(x+62, y+22, c.title, 17, cWhite, semi)
	for i, l := range c.lines {
		p.text(x+20, y+64+float32(i)*20, l, 12.5, cDim, reg)
	}
	by, bw := y+h-58, (w-48)/2
	if p.button("c-cancel", x+16, by, bw, 42, "Cancel", false) {
		p.confirm = nil
	}
	if p.button("c-ok", x+32+bw, by, bw, 42, c.ok, true) {
		f := c.f
		p.confirm = nil
		f()
	}
}

func (p *Panel) apiView(top float32) {
	s := p.settings
	if !p.apiInit {
		p.apiInit, p.apiMsg = true, ""
		p.edPort.SetText(strconv.Itoa(s.APIPort))
		p.edAllow.SetText(strings.Join(s.APIAllow, ", "))
		p.edUser.SetText(s.APIUser)
		p.edPass.SetText("")
	}
	p.text(16, top+6, "Local API", 18, cWhite, semi)
	y := top + 46
	p.toggleRow(y, "Enabled", s.LocalAPI, func() {
		if s.LocalAPI {
			s.LocalAPI = false
			p.be.SaveSettings(s)
			return
		}
		p.askConfirm("Enable the local API?", apiWarning, "Enable API", func() { s.LocalAPI = true; p.be.SaveSettings(s) })
	})
	if s.LocalAPI {
		st := p.be.APIStatus()
		col := cDim
		if strings.HasPrefix(st, "Error") || strings.HasPrefix(st, "Set") {
			col = rgb(255, 150, 130)
		}
		p.text(18, y+58, fmt.Sprintf("http://127.0.0.1:%d/swagger/", s.APIPort), 12, cAmber, reg)
		p.text(18, y+76, st, 11.5, col, reg)
	}
	p.text(18, y+104, "Port", 12, cDim, reg)
	p.field(&p.edPort, 16, y+124, 120, 44, "7480", 0)
	p.text(18, y+180, "Also allow (comma separated IPs or CIDR ranges)", 12, cDim, reg)
	p.field(&p.edAllow, 16, y+200, p.w-32, 44, "192.168.0.0/24, 10.0.0.5", 0)
	p.toggleRow(y+256, "Require sign-in", s.APIAuth, func() { s.APIAuth = !s.APIAuth; p.be.SaveSettings(s) })
	by := y + 256 + 58
	if s.APIAuth {
		fw := (p.w - 32 - 8) / 2
		p.field(&p.edUser, 16, by, fw, 44, "Username", 0)
		hint := "New password"
		if s.APIHash != "" {
			hint = "Password set"
		}
		p.field(&p.edPass, 24+fw, by, fw, 44, hint, '•')
		by += 56
	}
	if p.button("apply", 16, by, p.w-32, 46, "Apply", true) {
		p.applyAPI(s)
	}
	if p.apiMsg != "" {
		col := cGreen
		if p.apiMsg != "Applied" {
			col = rgb(255, 150, 130)
		}
		p.text(18, by+58, p.apiMsg, 12.5, col, reg)
	}
	p.reach(by + 96)
}

func (p *Panel) applyAPI(s homebase.Settings) {
	port, err := strconv.Atoi(strings.TrimSpace(p.edPort.Text()))
	if err != nil || port < 1024 || port > 65535 {
		p.apiMsg = "Port must be between 1024 and 65535"
		return
	}
	var allow []string
	for _, e := range strings.Split(p.edAllow.Text(), ",") {
		if e = strings.TrimSpace(e); e == "" {
			continue
		}
		if _, err := netip.ParsePrefix(e); err != nil {
			if _, err := netip.ParseAddr(e); err != nil {
				p.apiMsg = "Not an IP address or CIDR: " + e
				return
			}
		}
		allow = append(allow, e)
	}
	user, pass := strings.TrimSpace(p.edUser.Text()), p.edPass.Text()
	if s.APIAuth {
		if user == "" {
			p.apiMsg = "Enter a username"
			return
		}
		if pass == "" && s.APIHash == "" {
			p.apiMsg = "Enter a password"
			return
		}
	}
	s.APIPort, s.APIAllow = port, allow
	if s.APIAuth {
		s.APIUser = user
	}
	p.be.SaveSettings(s)
	if s.APIAuth && pass != "" {
		p.be.SetAPIPassword(user, pass)
		p.edPass.SetText("")
	}
	p.apiMsg = "Applied"
}

var apiWarning = []string{
	"Anything allowed to reach this port can switch your",
	"lights and unlink your Tuya account. The API uses",
	"plain HTTP, so traffic is not encrypted.",
	"Only this PC can connect unless you add addresses.",
	"Requiring a username and password is recommended.",
}

func (p *Panel) ShowAPI(confirm bool) {
	p.view, p.apiInit = viewAPI, false
	if confirm {
		p.askConfirm("Enable the local API?", apiWarning, "Enable API", func() {})
	}
}

func (p *Panel) drawPicker() {
	if p.picker == "" {
		return
	}
	d, ok := p.find(p.picker)
	if !ok {
		p.picker = ""
		return
	}
	p.fillRR(0, 0, p.w, p.h, 9, rgba(3, 5, 16, 225))
	p.area("scrim2", 0, 0, p.w, p.h)
	type chip struct {
		label     string
		x, y, w   float32
		reset, on bool
	}
	var chips []chip
	cx, cy, cw := float32(16), float32(0), p.w-40
	for _, r := range append(p.rooms(), "") {
		label := r
		if r == "" {
			label = "Reset to Tuya room"
		}
		w := p.textW(label, 12.5, reg) + 26
		if cx+w > cw-16 {
			cx, cy = 16, cy+38
		}
		chips = append(chips, chip{label, cx, cy, w, r == "", r != "" && r == d.Room})
		cx += w + 8
	}
	h := 76 + cy + 38 + 16 + 44 + 12 + 42 + 20
	p.reach(h + 40)
	x, y := float32(20), (p.h-h)/2
	p.gradRR(x, y, cw, h, 20, rgb(34, 38, 70), rgb(18, 22, 44), false)
	p.strokeRR(x, y, cw, h, 20, 1.2, rgb(70, 82, 130))
	p.text(x+18, y+16, "Move to room", 17, cWhite, semi)
	p.textFit(x+18, y+44, cw-36, d.Name, 12, cDim, reg)
	for i, c := range chips {
		bx, by := x+c.x, y+76+c.y
		sel := p.ease(fmt.Sprint("chip", i, d.ID), b2f(c.on), 16)
		p.fillRR(bx, by, c.w, 30, 15, mix(rgba(255, 255, 255, 5), rgb(48, 56, 94), sel))
		p.strokeRR(bx, by, c.w, 30, 15, 1.1, mix(rgba(255, 255, 255, 14), rgb(90, 150, 255), sel))
		p.textC(bx+c.w/2, by+5, c.label, 12.5, mix(cDim, cWhite, sel), reg)
		if p.area(fmt.Sprint("chiphit", i), bx, by, c.w, 30).Clicked {
			if c.reset {
				p.be.SetRoom(d.ID, "")
			} else {
				p.be.SetRoom(d.ID, c.label)
			}
			p.picker = ""
		}
	}
	fy := y + 76 + cy + 38 + 16
	p.field(&p.edRoom, x+16, fy, cw-32-86, 44, "New room name", 0)
	add := p.button("room-add", x+cw-16-78, fy, 78, 44, "Add", true)
	name := strings.TrimSpace(p.edRoom.Text())
	if add && name != "" {
		p.be.SetRoom(d.ID, name)
		p.picker = ""
	}
	if p.button("room-cancel", x+16, fy+56, cw-32, 42, "Cancel", false) {
		p.picker = ""
	}
}
