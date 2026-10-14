package main

import (
	"flag"
	"math"
	"os"
	"runtime/debug"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/energye/systray"

	"moreno/tuyatray/flyout"
	"moreno/tuyatray/homebase"
	"moreno/tuyatray/panel"
)

const panelW = 408

func main() {
	shot := flag.String("shot", "", "render demo screens to this folder and exit")
	flag.Parse()
	debug.SetGCPercent(40)
	if *shot != "" {
		renderShots(*shot)
		return
	}
	if !flyout.Single() {
		return
	}
	var (
		wp                 atomic.Pointer[app.Window]
		hwnd               atomic.Uintptr
		reset, focused     atomic.Bool
		closing            atomic.Bool
		shownAt, lastOn    atomic.Int64
		blurAt             atomic.Int64
		hNow, homeH, scale atomic.Int32
		trayReady          atomic.Bool
	)
	lastOn.Store(-1)
	scale.Store(1000)
	sizeCh := make(chan int32, 1)
	go func() {
		for px := range sizeCh {
			if h := hwnd.Load(); h != 0 {
				flyout.SetHeight(h, px)
			}
		}
	}()
	setHeight := func(dp float32) {
		hNow.Store(int32(dp * 100))
		px := int32(dp*float32(scale.Load())/1000 + 0.5)
		select {
		case sizeCh <- px:
		default:
			select {
			case <-sizeCh:
			default:
			}
			select {
			case sizeCh <- px:
			default:
			}
		}
	}
	var lv *live
	invalidate := func() {
		if w := wp.Load(); w != nil {
			w.Invalidate()
		}
		if lv == nil || !trayReady.Load() {
			return
		}
		on, n := 0, 0
		for _, d := range lv.Devices() {
			if d.Kind == homebase.KindLight {
				n++
				if d.On && d.Online {
					on++
				}
			}
		}
		if int64(on) != lastOn.Swap(int64(on)) {
			systray.SetIcon(flyout.Icon(on > 0))
			systray.SetTooltip(map[bool]string{true: "Lights on: " + itoa(on) + " of " + itoa(n), false: "No lights on"}[on > 0])
		}
	}
	trim := func() {
		debug.FreeOSMemory()
		flyout.TrimMemory()
	}
	hide := func() {
		h := hwnd.Load()
		if h == 0 || !flyout.Visible(h) || !closing.CompareAndSwap(false, true) {
			return
		}
		done := flyout.Close(h, 220*time.Millisecond, scale.Load()*10/1000)
		closing.Store(false)
		if !done {
			return
		}
		lv.SetFast(false)
		go func() {
			time.Sleep(600 * time.Millisecond)
			trim()
		}()
	}
	open := func() {
		h := hwnd.Load()
		if h == 0 {
			return
		}
		reset.Store(true)
		focused.Store(false)
		shownAt.Store(time.Now().UnixNano())
		lv.SetFast(true)
		lv.RefreshIfStale(90 * time.Second)
		if hh := float32(homeH.Load()) / 100; hh > 100 {
			setHeight(hh)
			time.Sleep(40 * time.Millisecond)
		}
		flyout.Show(h)
		invalidate()
	}
	toggle := func() {
		if h := hwnd.Load(); h != 0 && flyout.Visible(h) {
			go hide()
			return
		}
		if time.Since(time.Unix(0, blurAt.Load())) < 450*time.Millisecond {
			return
		}
		open()
	}
	lv = newLive(invalidate, func() { os.Exit(0) })
	lv.owner = hwnd.Load
	go func() {
		for {
			time.Sleep(45 * time.Second)
			if h := hwnd.Load(); h != 0 && !flyout.Visible(h) {
				trim()
			}
		}
	}()
	go systray.Run(func() {
		systray.SetIcon(flyout.Icon(false))
		systray.SetTooltip("Smart lights")
		systray.SetOnClick(func(systray.IMenu) { toggle() })
		systray.SetOnRClick(func(m systray.IMenu) { _ = m.ShowMenu() })
		systray.AddMenuItem("Open", "").Click(open)
		systray.AddMenuItem("Refresh devices", "").Click(lv.Refresh)
		systray.AddMenuItem("Stop light show", "").Click(lv.StopShow)
		systray.AddSeparator()
		systray.AddMenuItem("Quit", "").Click(func() { os.Exit(0) })
		trayReady.Store(true)
	}, nil)
	go func() {
		_, wh := flyout.WorkAreaDp()
		h := min(720, wh-24)
		pnl := panel.New(lv)
		pnl.Live, pnl.MaxH = true, h
		hNow.Store(int32(h * 100))
		w := new(app.Window)
		w.Option(app.Title("Smart lights"), app.Size(unit.Dp(panelW), unit.Dp(h)), app.MinSize(unit.Dp(panelW), unit.Dp(340)), app.MaxSize(unit.Dp(panelW), unit.Dp(h)), app.Decorated(false))
		wp.Store(w)
		var ops op.Ops
		last := time.Now()
		for {
			switch e := w.Event().(type) {
			case app.Win32ViewEvent:
				hwnd.Store(e.HWND)
				if e.HWND != 0 {
					go flyout.Prepare(e.HWND)
				}
			case app.ConfigEvent:
				if e.Config.Focused {
					focused.Store(true)
				} else if focused.Swap(false) && !flyout.Modal() && time.Since(time.Unix(0, shownAt.Load())) > 300*time.Millisecond {
					if h := hwnd.Load(); h != 0 && flyout.Visible(h) {
						blurAt.Store(time.Now().UnixNano())
						go hide()
					}
				}
			case app.DestroyEvent:
				os.Exit(0)
			case app.FrameEvent:
				if reset.Swap(false) {
					pnl.Reset()
				}
				gtx := app.NewContext(&ops, e)
				pnl.Layout(gtx)
				e.Frame(gtx.Ops)
				scale.Store(int32(e.Metric.PxPerDp * 1000))
				if pnl.HomeH > 0 {
					homeH.Store(int32(pnl.HomeH * 100))
				}
				now := time.Now()
				dt := math.Min(math.Max(now.Sub(last).Seconds(), 0.001), 0.05)
				last = now
				if want, cur := pnl.WantH, float32(hNow.Load())/100; want > 100 && cur != want {
					if math.Abs(float64(want-cur)) < 0.4 {
						cur = want
					} else {
						cur += (want - cur) * float32(1-math.Exp(-dt*13))
						w.Invalidate()
					}
					setHeight(cur)
				}
			}
		}
	}()
	app.Main()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
