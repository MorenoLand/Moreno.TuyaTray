package panel

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	gBg   = rgb(244, 247, 252)
	gCard = rgb(255, 255, 255)
	gText = rgb(34, 38, 52)
	gSub  = rgb(150, 156, 172)
	gHi   = rgb(255, 170, 70)
)

func (p *Panel) gbar(x, y, w float32, c color.NRGBA) { p.fillRR(x, y, w, 3.5, 1.75, c) }

func (p *Panel) gring(x, y, w, h float32) {
	p.fillRR(x, y, w, h, 6, rgba(255, 170, 70, 40))
	p.strokeRR(x, y, w, h, 6, 1.6, gHi)
}

func (p *Panel) gnav(x, y, w float32, active int) {
	for i := 0; i < 4; i++ {
		c := gSub
		if i == active {
			c = gText
		}
		p.disc(x+w*(float32(i)+0.5)/4, y+4, 3.2, c)
	}
}

func (p *Panel) guideMe(x, y, w, h float32) {
	p.gbar(x+w-36, y+11, 8, gText)
	p.gring(x+w-24, y+4, 18, 18)
	p.ring(x+w-15, y+13, 4, 1.4, gText)
	p.disc(x+20, y+38, 11, rgb(206, 214, 240))
	p.text(x+36, y+32, "Tap to Set Nickname", 7, gText, semi)
	p.fillRR(x+8, y+58, w-16, 34, 6, rgb(52, 56, 76))
	p.text(x+14, y+68, "Smart Video Doorbell", 7, cWhite, semi)
	p.fillRR(x+6, y+98, w-12, h-132, 8, gCard)
	for i := 0; i < 5; i++ {
		ry := y + 106 + float32(i)*(h-148)/5
		p.fillRR(x+13, ry, 8, 8, 2, []color.NRGBA{rgb(240, 120, 90), rgb(110, 200, 120), rgb(250, 190, 70), rgb(80, 190, 230), rgb(230, 120, 90)}[i])
		p.gbar(x+28, ry+2, w-60-float32(i)*6, gText)
	}
	p.gnav(x, y+h-18, w, 3)
	p.textC(x+w*0.875, y+h-12, "Me", 6, gText, semi)
}

func (p *Panel) guideSettings(x, y, w, h float32) {
	p.textC(x+w/2, y+8, "Settings", 8, gText, semi)
	p.fillRR(x+6, y+26, w-12, 62, 8, gCard)
	p.text(x+13, y+31, "Personal Information", 7, gText, semi)
	p.gring(x+9, y+46, w-18, 18)
	p.text(x+13, y+51, "Account and Security", 7, gText, semi)
	p.text(x+13, y+71, "Device Update", 7, gText, semi)
	p.fillRR(x+6, y+96, w-12, h-132, 8, gCard)
	for i := 0; i < 5; i++ {
		ry := y + 106 + float32(i)*(h-146)/5
		p.gbar(x+13, ry, 38+float32(i%3)*10, gText)
		if i == 0 || i == 3 {
			p.fillRR(x+w-30, ry-3, 18, 9, 4.5, rgb(210, 214, 226))
		} else {
			p.gbar(x+w-28, ry, 12, gSub)
		}
	}
	p.gnav(x, y+h-18, w, -1)
}

func (p *Panel) guideAccount(x, y, w, h float32) {
	p.textC(x+w/2, y+8, "Account and Security", 7.5, gText, semi)
	p.fillRR(x+6, y+26, w-12, 20, 7, gCard)
	p.text(x+12, y+31, "Region", 7, gText, semi)
	p.fillRR(x+6, y+52, w-12, 26, 7, gCard)
	p.text(x+12, y+55, "Email Address", 7, gText, semi)
	p.text(x+12, y+66, "you****@example.com", 6, gSub, reg)
	p.gbar(x+8, y+86, 50, gSub)
	p.fillRR(x+6, y+94, w-12, 20, 7, gCard)
	p.gbar(x+12, y+102, 56, gText)
	p.fillRR(x+6, y+120, w-12, 36, 7, gCard)
	for i := 0; i < 3; i++ {
		p.gbar(x+12, y+126+float32(i)*10, 40+float32(i)*8, gText)
	}
	p.gring(x+4, y+h-48, w-8, 26)
	p.fillRR(x+6, y+h-46, w-12, 22, 6, gCard)
	p.text(x+12, y+h-40, "User Code", 7, gText, semi)
	p.textR(x+w-12, y+h-40, "XXXXXXX", 7, gSub, reg)
}

func (p *Panel) scaled(x, y, z float32, f func()) {
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(x*p.s, y*p.s), f32.Pt(z, z))).Push(p.ops).Pop()
	f()
}

func (p *Panel) guide(y float32) {
	steps := []struct {
		tab  string
		text [2]string
		draw func(x, y, w, h float32)
	}{
		{"1  Me", [2]string{"Open the Me tab, then tap the", "gear icon at the top right."}, p.guideMe},
		{"2  Account", [2]string{"In Settings, open", "Account and Security."}, p.guideSettings},
		{"3  User Code", [2]string{"Copy the User Code shown", "near the bottom of that page."}, p.guideAccount},
	}
	tw := (p.w - 32 - 16) / 3
	for i, s := range steps {
		x := 16 + float32(i)*(tw+8)
		sel := p.ease(strings.Repeat("g", i+1), b2f(p.guideStep == i), 16)
		p.fillRR(x, y, tw, 34, 17, mix(rgba(255, 255, 255, 5), rgb(48, 56, 94), sel))
		p.strokeRR(x, y, tw, 34, 17, 1.1, mix(rgba(255, 255, 255, 12), rgb(90, 150, 255), sel))
		p.textC(x+tw/2, y+8, s.tab, 12.5, mix(cDim, cWhite, sel), semi)
		if p.area(strings.Repeat("gt", i+1), x, y, tw, 34).Clicked {
			p.guideStep = i
		}
	}
	const z, bw, bh = 1.75, 120, 204
	mx, my := (p.w-bw*z)/2, y+48
	p.fillRR(mx, my, bw*z, bh*z, 22, gBg)
	p.scaled(mx, my, z, func() { steps[p.guideStep].draw(mx, my, bw, bh) })
	p.strokeRR(mx, my, bw*z, bh*z, 22, 1.2, rgba(255, 255, 255, 18))
	if p.area("guidepic", mx, my, bw*z, bh*z).Clicked {
		p.guideStep = (p.guideStep + 1) % 3
	}
	t := steps[p.guideStep].text
	p.textC(p.w/2, my+bh*z+14, t[0], 13, cWhite, reg)
	p.textC(p.w/2, my+bh*z+34, t[1], 13, cWhite, reg)
}

func (p *Panel) linkView() {
	payload, status := p.be.LinkState()
	p.text(16, 20, "Link your Tuya account", 18, cWhite, semi)
	p.text(16, 50, "Enter your Tuya User Code, then scan the QR it shows.", 12, cDim, reg)
	p.glass(16, 84, p.w-140, 46, 14)
	if !p.focused {
		p.gtx.Execute(key.FocusCmd{Tag: &p.edCode})
		p.focused = true
	}
	p.edCode.SingleLine, p.edCode.Submit, p.edCode.Mask = true, true, '•'
	if p.showCode {
		p.edCode.Mask = 0
	}
	submit := false
	for {
		ev, ok := p.edCode.Update(p.gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			submit = true
		}
	}
	g := p.gtx
	g.Constraints = layout.Constraints{Max: image.Pt(p.px(p.w-210), p.px(30))}
	st := op.Offset(image.Pt(p.px(32), p.px(96))).Push(p.ops)
	e := material.Editor(p.th, &p.edCode, "User Code")
	e.Color, e.HintColor, e.TextSize = cWhite, cFaint, unit.Sp(16)
	e.Layout(g)
	st.Pop()
	eye := p.area("eye", p.w-170, 88, 42, 38)
	p.icoEye(p.w-149, 107, mix(cDim, cWhite, p.ease("eyehv", b2f(eye.Hover || p.showCode), 14)), p.showCode)
	if eye.Clicked {
		p.showCode = !p.showCode
	}
	bx := p.w - 116
	p.gradRR(bx, 84, 100, 46, 14, rgb(88, 150, 255), rgb(44, 96, 230), false)
	p.textC(bx+50, 97, "Show QR", 13.5, cWhite, semi)
	code := strings.TrimSpace(p.edCode.Text())
	if (p.area("qrbtn", bx, 84, 100, 46).Clicked || submit) && code != "" {
		p.be.StartLink(code)
	}
	bottom := float32(630)
	if payload == "" {
		p.guide(150)
	} else if bits := p.qr(payload); len(bits) > 0 {
		size := float32(250)
		m := size / float32(len(bits))
		x0, y0 := (p.w-size-24)/2, float32(160)
		p.fillRR(x0, y0, size+24, size+24, 18, rgb(255, 255, 255))
		for r := range bits {
			for c := range bits[r] {
				if bits[r][c] {
					p.fillRR(x0+12+float32(c)*m, y0+12+float32(r)*m, m+0.4, m+0.4, 0, rgb(10, 12, 24))
				}
			}
		}
		p.textC(p.w/2, y0+size+44, "Scan with the Tuya app: Me, then the scan icon.", 12.5, cDim, reg)
		bottom = y0 + size + 76
	}
	p.reach(bottom + 24)
	if status != "" {
		col := cAmber
		if strings.HasPrefix(status, "Error") {
			col = rgb(255, 150, 130)
		}
		p.textC(p.w/2, bottom, status, 12.5, col, reg)
	}
}
