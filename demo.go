package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeplan"
	"moreno/tuyatray/lightshow"
	"moreno/tuyatray/panel"
)

type demo struct {
	unlinked bool
	plan     string
}

func (demo) Devices() []homebase.Device {
	l := func(id, name, room string, on bool, bright, temp int) homebase.Device {
		return homebase.Device{ID: id, Name: name, Room: room, Online: true, On: on, Mode: "white", Bright: bright, Temp: temp, HasTemp: true, HasColour: true, HasScene: true, Style: 'B'}
	}
	tv := l("tv", "TV backlight", "Living Room", true, 90, 0)
	tv.Mode, tv.Hue, tv.Sat = "colour", 265, 100
	hall := l("hall", "Hallway", "Hallway", false, 100, 0)
	hall.Online = false
	return []homebase.Device{
		{ID: "ac", Name: "Living AC", Room: "Living Room", Kind: homebase.KindSwitch, Online: true},
		l("ceil", "Ceiling", "Living Room", true, 100, 40), l("lamp", "Floor lamp", "Living Room", true, 60, 5), tv,
		l("strip", "Counter strip", "Kitchen", true, 80, 60), l("pend", "Kitchen pendant", "Kitchen", false, 100, 30),
		l("bed", "Bedside lamp", "Bedroom", true, 35, 0), l("bath", "Bathroom", "Bathroom", true, 100, 70),
		l("din", "Dining pendant", "Dining", true, 70, 20), l("loft", "Loft light", "Loft", true, 55, 8), hall,
	}
}

func (demo) HomeName() string            { return "My Home" }
func (demo) SetPower(string, bool)       {}
func (demo) SetBright(string, int)       {}
func (demo) SetTemp(string, int)         {}
func (demo) SetColour(string, int, int)  {}
func (demo) SetWhite(string)             {}
func (demo) AllLights(bool)              {}
func (demo) RoomLights(string, bool)     {}
func (demo) Scene(string)                {}
func (demo) Refresh()                    {}
func (demo) SetRoom(string, string)      {}
func (demo) SetSlot(string, int)         {}
func (demo) SetEffect(string, string)    {}
func (demo) Rescan()                     {}
func (d demo) Linked() bool              { return !d.unlinked }
func (demo) StartLink(string)            {}
func (demo) LinkState() (string, string) { return "", "" }
func (demo) Unlink()                     {}
func (demo) Settings() homebase.Settings {
	return homebase.Settings{ShowHouse: true, LocalAPI: true, APIPort: 7480, APIAuth: true, APIUser: "admin", APIHash: "x", APIAllow: []string{"192.168.0.0/24"}, Icons: map[string]string{"r:Kitchen": "b:kitchen", "r:Bedroom": "b:bed", "d:lamp": "b:fan", "d:strip": "b:plant"}}
}
func (demo) SaveSettings(homebase.Settings) {}
func (demo) SetAPIPassword(string, string)  {}
func (demo) APIStatus() string              { return "" }
func (d demo) Plan() *homeplan.Plan {
	id := d.plan
	if id == "" {
		id = "twostory"
	}
	p, _ := homeplan.Preset(id)
	return p
}
func (demo) HasCustomPlan() bool          { return false }
func (demo) Icons() map[string]string     { return nil }
func (demo) SetIcon(string, string) error { return nil }
func (demo) ChooseIconImage(string)       {}
func (demo) IconPath(string) string       { return "" }
func (demo) Quit()                        {}
func (demo) Rules() []homebase.Rule {
	return []homebase.Rule{
		{ID: "a", Name: "Lights out", Enabled: true, At: "23:30", Target: "everything", Action: "off"},
		{ID: "b", Name: "Morning light", Enabled: true, At: "07:15", Days: []int{1, 2, 3, 4, 5}, Target: "room:Bedroom", Action: "scene:Bright"},
		{ID: "c", Name: "Movie night", Enabled: false, At: "20:00", Days: []int{5, 6}, Target: "room:Living Room", Action: "scene:Movie"},
	}
}
func (demo) SaveRules([]homebase.Rule)                   {}
func (demo) StartTimer(int, string, string)              {}
func (demo) CancelTimer()                                {}
func (demo) TimerState() (time.Duration, string, string) { return 0, "", "" }
func (demo) ShowState() lightshow.State                  { return lightshow.State{} }
func (demo) ShowConfig() lightshow.Config {
	return lightshow.Config{Mode: "screen", Devices: []string{"ceil", "lamp"}}
}
func (demo) StartShow(lightshow.Config) error { return nil }
func (demo) StopShow()                        {}

func renderShots(dir string) {
	_ = os.MkdirAll(dir, 0o755)
	p := panel.New(demo{})
	p.Static = true
	const s = 2
	for _, v := range []struct {
		name  string
		tall  bool
		setup func()
	}{
		{"main", false, func() { p.Reset() }},
		{"list", true, func() { p.Reset() }},
		{"detail", false, func() { p.ShowDetail("tv") }},
		{"settings", false, func() { p.Reset(); p.ShowSettings() }},
		{"api", false, func() { p.Reset(); p.ShowAPI(false) }},
		{"picker", false, func() { p.Reset(); p.ShowPicker("ceil") }},
		{"scene", false, func() { p.Reset(); p.ShowScene("tv") }},
		{"custom", false, func() { p.Reset(); p.ShowDetail("tv"); p.PreviewState(true, false) }},
		{"placing", false, func() { p.Reset(); p.ShowDetail("ceil"); p.PreviewState(false, true) }},
		{"switch", false, func() { p.Reset(); p.ShowDetail("ac") }},
		{"schedules", false, func() { p.Reset(); p.ShowSchedules() }},
		{"rule", false, func() { p.Reset(); p.ShowRule(0) }},
		{"show", false, func() { p.Reset(); p.ShowLightShow() }},
		{"icons", false, func() { p.Reset(); p.ShowIcons() }},
		{"iconpick", false, func() { p.Reset(); p.ShowIcons(); p.ShowIconPicker("d:lamp") }},
		{"group", false, func() { p.Reset(); p.ShowGroup() }},
		{"design", false, func() { p.Reset(); p.ShowDesign() }},
		{"studio", false, func() { p = panel.New(demo{plan: "studio"}); p.Static = true }},
		{"cottage", false, func() { p = panel.New(demo{plan: "cottage"}); p.Static = true }},
		{"confirm", false, func() { p.ShowAPI(true) }},
		{"link", false, func() { p = panel.New(demo{unlinked: true}); p.Static = true }},
		{"linkhidden", false, func() { p.PreviewCode("XXXXXXX", false) }},
		{"linkshown", false, func() { p.PreviewCode("XXXXXXX", true) }},
	} {
		v.setup()
		w, h := 408*s, 720*s
		if v.tall {
			h = 1500 * s
		}
		var warm op.Ops
		p.Layout(layout.Context{Ops: &warm, Constraints: layout.Exact(image.Pt(w, h)), Metric: unit.Metric{PxPerDp: s, PxPerSp: s}})
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Exact(image.Pt(w, h)), Metric: unit.Metric{PxPerDp: s, PxPerSp: s}}
		p.Layout(gtx)
		win, err := headless.NewWindow(w, h)
		if err != nil {
			fmt.Println(err)
			return
		}
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		if err := win.Frame(&ops); err == nil {
			err = win.Screenshot(img)
		}
		win.Release()
		f, err := os.Create(filepath.Join(dir, v.name+".png"))
		if err != nil {
			fmt.Println(err)
			return
		}
		_ = png.Encode(f, img)
		f.Close()
	}
}
