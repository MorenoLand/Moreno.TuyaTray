package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"moreno/tuyatray/flyout"
	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeicons"
	"moreno/tuyatray/homeplan"
	"moreno/tuyatray/lightshow"
	"moreno/tuyatray/localapi"
	"moreno/tuyatray/tuyacloud"
)

type live struct {
	mu          sync.Mutex
	home        *homebase.Home
	acc         homebase.Account
	cancel      context.CancelFunc
	settings    homebase.Settings
	qr, status  string
	invalidate  func()
	quit        func()
	api         *localapi.Server
	apiStatus   string
	cl          *tuyacloud.Client
	lastRefresh time.Time
	clock       *homebase.Clock
	show        *lightshow.Engine
	plan        *homeplan.Plan
	planName    string
	owner       func() uintptr
}

func newLive(invalidate, quit func()) *live {
	l := &live{settings: homebase.LoadSettings(), invalidate: invalidate, quit: quit}
	l.clock = homebase.NewClock(l.Apply)
	l.clock.SetRules(l.settings.Rules)
	l.clock.Start()
	l.show = lightshow.New(func(ids []string) (lightshow.Output, []string) {
		h := l.h()
		if h == nil {
			return nil, ids
		}
		st, skipped := h.OpenStream(ids)
		return st, skipped
	}, invalidate)
	if acc, ok := homebase.LoadAccount(); ok {
		l.attach(acc)
	}
	l.applyAPI()
	return l
}

func (l *live) attach(acc homebase.Account) {
	cl := tuyacloud.NewClient(acc.Session)
	cl.OnToken = func(s tuyacloud.Session) {
		l.mu.Lock()
		l.acc.Session = s
		a := l.acc
		l.mu.Unlock()
		_ = homebase.SaveAccount(a)
	}
	h := homebase.New(cl, l.invalidate)
	l.mu.Lock()
	rooms := l.settings.Rooms
	l.mu.Unlock()
	h.SetRooms(rooms)
	h.Load(acc.Devices)
	ctx, cancel := context.WithCancel(context.Background())
	l.mu.Lock()
	l.home, l.acc, l.cancel, l.cl = h, acc, cancel, cl
	l.mu.Unlock()
	go h.Run(ctx)
	l.Refresh()
}

func (l *live) Refresh() {
	l.mu.Lock()
	cl, h := l.cl, l.home
	l.lastRefresh = time.Now()
	l.mu.Unlock()
	if cl == nil || h == nil {
		return
	}
	go func() {
		if devs, err := cl.AllDevices(); err == nil && len(devs) > 0 {
			l.mu.Lock()
			l.acc.Devices = devs
			a := l.acc
			l.mu.Unlock()
			_ = homebase.SaveAccount(a)
			h.Load(devs)
		}
		h.Rescan()
	}()
}

func (l *live) RefreshIfStale(d time.Duration) {
	l.mu.Lock()
	stale := time.Since(l.lastRefresh) > d
	l.mu.Unlock()
	if stale {
		l.Refresh()
	}
}

func (l *live) SetRoom(id, room string) {
	room = strings.TrimSpace(room)
	l.mu.Lock()
	if l.settings.Rooms == nil {
		l.settings.Rooms = map[string]string{}
	}
	if room == "" {
		delete(l.settings.Rooms, id)
	} else {
		l.settings.Rooms[id] = room
	}
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.do(func(h *homebase.Home) { h.SetRoom(id, room) })
}

func (l *live) h() *homebase.Home {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.home
}

func (l *live) Devices() []homebase.Device {
	if h := l.h(); h != nil {
		return h.Snapshot()
	}
	return nil
}

func (l *live) HomeName() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.acc.Devices) > 0 {
		return l.acc.Devices[0].HomeName
	}
	return ""
}

func (l *live) SetPower(id string, on bool) { l.do(func(h *homebase.Home) { h.SetPower(id, on) }) }

func (l *live) SetBright(id string, p int) { l.do(func(h *homebase.Home) { h.SetBright(id, p) }) }

func (l *live) SetTemp(id string, p int) { l.do(func(h *homebase.Home) { h.SetTemp(id, p) }) }

func (l *live) SetColour(id string, hue, sat int) {
	l.do(func(h *homebase.Home) { h.SetColour(id, hue, sat) })
}

func (l *live) SetWhite(id string) { l.do(func(h *homebase.Home) { h.SetWhite(id) }) }

func (l *live) AllLights(on bool) { l.do(func(h *homebase.Home) { h.AllLights(on) }) }

func (l *live) RoomLights(room string, on bool) {
	l.do(func(h *homebase.Home) { h.RoomLights(room, on) })
}

func (l *live) Scene(name string) { l.do(func(h *homebase.Home) { h.Scene(name) }) }

func (l *live) Rescan() { l.do(func(h *homebase.Home) { go h.Rescan() }) }

func (l *live) do(f func(*homebase.Home)) {
	if h := l.h(); h != nil {
		f(h)
	}
}

func (l *live) Linked() bool { return l.h() != nil }

func (l *live) StartLink(code string) {
	l.mu.Lock()
	l.qr, l.status = "", "Contacting Tuya…"
	l.mu.Unlock()
	go func() {
		fail := func(m string) {
			l.mu.Lock()
			l.qr, l.status = "", "Error: "+m
			l.mu.Unlock()
			l.invalidate()
		}
		tok, err := tuyacloud.StartLogin(code)
		if err != nil {
			fail(err.Error())
			return
		}
		l.mu.Lock()
		l.qr, l.status = tuyacloud.QRPayload(tok), "Waiting for approval…"
		l.mu.Unlock()
		l.invalidate()
		for i := 0; i < 120; i++ {
			time.Sleep(2 * time.Second)
			s, ok, err := tuyacloud.PollLogin(tok, code)
			if err != nil || !ok {
				continue
			}
			cl := tuyacloud.NewClient(s)
			devs, err := cl.AllDevices()
			if err != nil {
				fail(err.Error())
				return
			}
			acc := homebase.Account{Session: s, Devices: devs}
			_ = homebase.SaveAccount(acc)
			l.mu.Lock()
			l.qr, l.status = "", ""
			l.mu.Unlock()
			l.attach(acc)
			l.invalidate()
			return
		}
		fail("approval timed out")
	}()
}

func (l *live) LinkState() (string, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.qr, l.status
}

func (l *live) Unlink() {
	l.mu.Lock()
	if l.cancel != nil {
		l.cancel()
	}
	l.home, l.acc, l.cancel = nil, homebase.Account{}, nil
	l.mu.Unlock()
	homebase.ForgetAccount()
	l.invalidate()
}

func (l *live) Settings() homebase.Settings {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.settings
}

func (l *live) SaveSettings(s homebase.Settings) {
	l.mu.Lock()
	prev := l.settings
	l.settings = s
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	if s.StartWithWindows != prev.StartWithWindows {
		_ = flyout.SetAutostart(s.StartWithWindows)
	}
	if apiChanged(prev, s) {
		l.applyAPI()
	}
	l.invalidate()
}

func (l *live) Quit() { l.quit() }

func (l *live) SetFast(f bool) { l.do(func(h *homebase.Home) { h.SetFast(f) }) }

func apiChanged(a, b homebase.Settings) bool {
	return a.LocalAPI != b.LocalAPI || a.APIPort != b.APIPort || strings.Join(a.APIAllow, ",") != strings.Join(b.APIAllow, ",") || a.APIAuth != b.APIAuth || a.APIUser != b.APIUser || a.APIHash != b.APIHash
}

func (l *live) APIStatus() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.apiStatus
}

func (l *live) SetAPIPassword(user, pass string) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	salt := hex.EncodeToString(b)
	l.mu.Lock()
	s := l.settings
	l.mu.Unlock()
	s.APIUser, s.APISalt, s.APIHash = user, salt, localapi.Hash(salt, pass)
	l.SaveSettings(s)
}

func (l *live) applyAPI() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.api != nil {
		_ = l.api.Close()
		l.api = nil
	}
	s := l.settings
	l.apiStatus = ""
	if !s.LocalAPI {
		return
	}
	if s.APIAuth && s.APIHash == "" {
		l.apiStatus = "Set a username and password first"
		return
	}
	srv, err := localapi.Start(localapi.Config{Port: s.APIPort, Allow: s.APIAllow, Auth: s.APIAuth, User: s.APIUser, Salt: s.APISalt, Hash: s.APIHash}, l)
	if err != nil {
		l.apiStatus = "Error: " + err.Error()
		return
	}
	l.api = srv
	l.apiStatus = fmt.Sprintf("Listening on port %d", s.APIPort)
}

func (l *live) SetSlot(id string, slot int) {
	l.mu.Lock()
	if l.settings.Slots == nil {
		l.settings.Slots = map[string]int{}
	}
	l.settings.Slots[id] = slot
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
}

func (l *live) Layout() (bool, map[string]int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]int{}
	for k, v := range l.settings.Slots {
		out[k] = v
	}
	return l.settings.ShowHouse, out
}

func (l *live) SetEffect(id, name string) { l.do(func(h *homebase.Home) { h.SetEffect(id, name) }) }

func (l *live) Apply(target, action string) error {
	h := l.h()
	if h == nil {
		return errors.New("no Tuya account linked")
	}
	return h.Apply(target, action)
}

func (l *live) Clock24() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.settings.Clock24
}

func (l *live) SetClock24(on bool) {
	l.mu.Lock()
	l.settings.Clock24 = on
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
}

func (l *live) Rules() []homebase.Rule { return l.clock.Rules() }

func (l *live) SaveRules(r []homebase.Rule) {
	l.clock.SetRules(r)
	l.mu.Lock()
	l.settings.Rules = r
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
}

func (l *live) StartTimer(minutes int, target, action string) {
	l.clock.StartTimer(time.Duration(minutes)*time.Minute, target, action)
	l.invalidate()
}

func (l *live) CancelTimer() {
	l.clock.CancelTimer()
	l.invalidate()
}

func (l *live) TimerState() (time.Duration, string, string) { return l.clock.TimerLeft() }

func (l *live) ShowState() lightshow.State { return l.show.State() }

func (l *live) ShowConfig() lightshow.Config {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.settings.Show
}

func (l *live) StartShow(c lightshow.Config) error {
	c = lightshow.Normalize(c)
	if err := l.show.Start(c); err != nil {
		return err
	}
	l.mu.Lock()
	l.settings.Show = c
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
	return nil
}

func (l *live) StopShow() {
	l.show.Stop()
	l.invalidate()
}

func (l *live) Plan() *homeplan.Plan {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.plan == nil || l.planName != l.settings.HomePlan {
		l.plan, l.planName = homeplan.Load(l.settings.HomePlan, homebase.ConfigDir()), l.settings.HomePlan
	}
	return l.plan
}

func (l *live) PlanName() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.settings.HomePlan == "" {
		return "apartment"
	}
	return l.settings.HomePlan
}

func (l *live) HasCustomPlan() bool {
	_, err := os.Stat(homeplan.CustomPath(homebase.ConfigDir()))
	return err == nil
}

func (l *live) SetPlan(name string) error {
	if name == "custom" {
		if !l.HasCustomPlan() {
			return errors.New("no custom design saved yet")
		}
	} else if _, err := homeplan.Preset(name); err != nil {
		return err
	}
	l.mu.Lock()
	l.settings.HomePlan = name
	l.plan = nil
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
	return nil
}

func (l *live) SetCustomPlan(b []byte) error {
	if _, err := homeplan.Parse(b); err != nil {
		return err
	}
	if err := os.WriteFile(homeplan.CustomPath(homebase.ConfigDir()), b, 0o600); err != nil {
		return err
	}
	l.mu.Lock()
	l.plan, l.settings.HomePlan = nil, "custom"
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
	return nil
}

func (l *live) CustomPlanJSON() []byte {
	b, _ := os.ReadFile(homeplan.CustomPath(homebase.ConfigDir()))
	return b
}

func (l *live) Icons() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]string, len(l.settings.Icons))
	for k, v := range l.settings.Icons {
		out[k] = v
	}
	return out
}

func (l *live) SetIcon(key, value string) error {
	if !homeicons.ValidKey(key) {
		return errors.New("key must be d:DEVICE or r:ROOM")
	}
	if !homeicons.ValidValue(homebase.ConfigDir(), value) {
		return errors.New("unknown icon")
	}
	l.mu.Lock()
	if l.settings.Icons == nil {
		l.settings.Icons = map[string]string{}
	}
	if value == "" {
		delete(l.settings.Icons, key)
	} else {
		l.settings.Icons[key] = value
	}
	s := l.settings
	l.mu.Unlock()
	_ = homebase.SaveSettings(s)
	l.invalidate()
	return nil
}

func (l *live) ImportIcon(key string, data []byte) (string, error) {
	name, err := homeicons.Import(homebase.ConfigDir(), data)
	if err != nil {
		return "", err
	}
	return "i:" + name, l.SetIcon(key, "i:"+name)
}

func (l *live) IconPath(file string) string { return homeicons.Path(homebase.ConfigDir(), file) }

func (l *live) ChooseIconImage(key string) {
	go func() {
		var owner uintptr
		if l.owner != nil {
			owner = l.owner()
		}
		path, ok := flyout.PickImage(owner)
		if !ok {
			return
		}
		if b, err := os.ReadFile(path); err == nil {
			_, _ = l.ImportIcon(key, b)
		}
	}()
}
