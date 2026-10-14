package homebase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"moreno/tuyatray/lanlink"
	"moreno/tuyatray/tuyacloud"
)

type Kind uint8

const (
	KindLight Kind = iota
	KindSwitch
)

type Device struct {
	ID, Name, Room, Category     string
	Kind                         Kind
	Style                        byte
	HasTemp, HasColour, HasScene bool
	Scene                        string
	IP, Ver                      string
	Online, On                   bool
	Mode                         string
	Bright, Temp                 int
	Hue, Sat                     int
	Via                          string
	key, swCode, cloudRoom       string
	quiet                        time.Time
	lanFail                      int
}

const (
	pPower  uint8 = 1
	pWhite  uint8 = 2
	pColour uint8 = 4
	pScene  uint8 = 8
)

type Home struct {
	mu        sync.RWMutex
	devs      []*Device
	byID      map[string]*Device
	cl        *tuyacloud.Client
	changed   func()
	fast      atomic.Bool
	wmu       sync.Mutex
	pend      map[string]uint8
	busy      map[string]bool
	lastScan  time.Time
	lastCloud time.Time
	homeIDs   []string
	rooms     map[string]string
	streaming map[string]bool
}

func New(cl *tuyacloud.Client, changed func()) *Home {
	return &Home{cl: cl, changed: changed, byID: map[string]*Device{}, pend: map[string]uint8{}, busy: map[string]bool{}}
}

func (h *Home) notify() {
	if h.changed != nil {
		h.changed()
	}
}

func (h *Home) SetFast(f bool) { h.fast.Store(f) }

func (h *Home) SetRooms(m map[string]string) {
	h.mu.Lock()
	h.rooms = map[string]string{}
	for k, v := range m {
		h.rooms[k] = v
	}
	h.mu.Unlock()
}

func (h *Home) sortLocked() {
	sort.SliceStable(h.devs, func(i, j int) bool {
		a, b := h.devs[i], h.devs[j]
		if a.Room != b.Room {
			return a.Room < b.Room
		}
		if a.Kind != b.Kind {
			return a.Kind > b.Kind
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

func (h *Home) SetRoom(id, room string) {
	h.mu.Lock()
	if h.rooms == nil {
		h.rooms = map[string]string{}
	}
	if d, ok := h.byID[id]; ok {
		if room = strings.TrimSpace(room); room == "" {
			delete(h.rooms, id)
			d.Room = d.cloudRoom
		} else {
			h.rooms[id], d.Room = room, room
		}
		h.sortLocked()
	}
	h.mu.Unlock()
	h.notify()
}

func isLightCode(c string) bool {
	return strings.HasPrefix(c, "switch_led") || strings.HasPrefix(c, "bright_value") || strings.HasPrefix(c, "temp_value") || strings.HasPrefix(c, "colour_data") || c == "work_mode"
}

func (h *Home) Load(list []tuyacloud.Device) {
	devs, old, over := make([]*Device, 0, len(list)), map[string]Device{}, map[string]string{}
	h.mu.RLock()
	for id, d := range h.byID {
		old[id] = *d
	}
	for k, v := range h.rooms {
		over[k] = v
	}
	h.mu.RUnlock()
	for _, c := range list {
		d, light := &Device{ID: c.ID, Name: c.Name, Room: c.Room, Category: c.Category, key: c.LocalKey, Online: c.Online}, false
		for _, s := range c.Status {
			light = light || isLightCode(s.Code)
			if strings.HasSuffix(s.Code, "_v2") {
				d.Style = 'B'
			}
			switch {
			case strings.HasPrefix(s.Code, "temp_value"):
				d.HasTemp = true
			case strings.HasPrefix(s.Code, "colour_data"):
				d.HasColour = true
			case strings.HasPrefix(s.Code, "scene_data"):
				d.HasScene = true
			case s.Code == "switch_led" || s.Code == "switch_1" || s.Code == "switch":
				d.swCode = s.Code
			}
		}
		if light {
			d.Kind = KindLight
			if d.Style == 0 {
				d.Style = 'A'
			}
		} else {
			d.Kind = KindSwitch
		}
		if d.Room == "" {
			d.Room = "Other"
		}
		d.cloudRoom = d.Room
		if r, ok := over[c.ID]; ok && r != "" {
			d.Room = r
		}
		d.applyCloud(c.Status)
		if o, ok := old[c.ID]; ok {
			d.IP, d.Ver, d.Via = o.IP, o.Ver, o.Via
		}
		devs = append(devs, d)
	}
	sort.SliceStable(devs, func(i, j int) bool {
		a, b := devs[i], devs[j]
		if a.Room != b.Room {
			return a.Room < b.Room
		}
		if a.Kind != b.Kind {
			return a.Kind > b.Kind
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	h.mu.Lock()
	h.devs, h.byID = devs, map[string]*Device{}
	for _, d := range devs {
		h.byID[d.ID] = d
	}
	h.mu.Unlock()
	h.notify()
}

func (h *Home) Snapshot() []Device {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Device, len(h.devs))
	for i, d := range h.devs {
		out[i] = *d
	}
	return out
}

func (h *Home) Get(id string) (Device, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if d, ok := h.byID[id]; ok {
		return *d, true
	}
	return Device{}, false
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

func (d *Device) brightRange() (int, int) {
	if d.Style == 'B' {
		return 10, 1000
	}
	return 25, 255
}

func (d *Device) pctToRaw(p int) int {
	lo, hi := d.brightRange()
	return lo + clamp(p, 0, 100)*(hi-lo)/100
}

func (d *Device) rawToPct(r int) int {
	lo, hi := d.brightRange()
	return clamp((r-lo)*100/(hi-lo), 0, 100)
}

func (d *Device) tempRawMax() int {
	if d.Style == 'B' {
		return 1000
	}
	return 255
}

func hsv2rgb(h, s, v float64) (float64, float64, float64) {
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
	return r + m, g + m, b + m
}

func (d *Device) dpKeys() (sw, mode, bright, temp, colour string) {
	if d.Style == 'B' {
		return "20", "21", "22", "23", "24"
	}
	return "1", "2", "3", "4", "5"
}

func num(v any) (int, bool) {
	f, ok := v.(float64)
	return int(f), ok
}

func (d *Device) lanDps(mask uint8) lanlink.Dps {
	if d.Kind == KindSwitch {
		return lanlink.Dps{"1": d.On}
	}
	sw, mode, bright, temp, colour := d.dpKeys()
	out := lanlink.Dps{}
	if mask&pPower != 0 {
		out[sw] = d.On
	}
	if mask&pScene != 0 {
		if d.Style == 'B' {
			out[mode], out["25"] = "scene", effectsB[d.Scene]
		} else {
			e := effectsA[d.Scene]
			out[mode] = e[0]
			if e[1] != "" {
				out["6"] = e[1]
			}
		}
		return out
	}
	if mask&(pWhite|pColour) == 0 {
		return out
	}
	if d.Mode == "colour" && d.HasColour {
		out[mode] = "colour"
		v := max(d.Bright, 1)
		if d.Style == 'B' {
			out[colour] = fmt.Sprintf("%04x%04x%04x", d.Hue, d.Sat*10, d.pctToRaw(v))
		} else {
			r, g, b := hsv2rgb(float64(d.Hue), float64(d.Sat)/100, float64(v)/100)
			out[colour] = fmt.Sprintf("%02x%02x%02x%04x%02x%02x", int(r*255+.5), int(g*255+.5), int(b*255+.5), d.Hue, d.Sat*255/100, v*255/100)
		}
		return out
	}
	out[mode], out[bright] = "white", d.pctToRaw(d.Bright)
	if d.HasTemp {
		out[temp] = d.Temp * d.tempRawMax() / 100
	}
	return out
}

func (d *Device) cloudCmds(mask uint8) []tuyacloud.Command {
	sw := d.swCode
	if sw == "" {
		sw = "switch_led"
		if d.Kind == KindSwitch {
			sw = "switch_1"
		}
	}
	var out []tuyacloud.Command
	if mask&pPower != 0 || d.Kind == KindSwitch {
		out = append(out, tuyacloud.Command{Code: sw, Value: d.On})
	}
	if d.Kind != KindLight || mask&(pWhite|pColour) == 0 {
		return out
	}
	suf, sMax := "", 255
	if d.Style == 'B' {
		suf, sMax = "_v2", 1000
	}
	if d.Mode == "colour" && d.HasColour {
		v := max(d.Bright, 1)
		return append(out, tuyacloud.Command{Code: "work_mode", Value: "colour"}, tuyacloud.Command{Code: "colour_data" + suf, Value: map[string]int{"h": d.Hue, "s": d.Sat * sMax / 100, "v": v * sMax / 100}})
	}
	out = append(out, tuyacloud.Command{Code: "work_mode", Value: "white"}, tuyacloud.Command{Code: "bright_value" + suf, Value: d.pctToRaw(d.Bright)})
	if d.HasTemp {
		out = append(out, tuyacloud.Command{Code: "temp_value" + suf, Value: d.Temp * d.tempRawMax() / 100})
	}
	return out
}

func (d *Device) applyLan(dps lanlink.Dps) {
	if d.Kind == KindSwitch {
		if v, ok := dps["1"].(bool); ok {
			d.On = v
		}
		return
	}
	if _, ok := dps["20"]; ok {
		d.Style = 'B'
	}
	sw, mode, bright, temp, colour := d.dpKeys()
	if v, ok := dps[sw].(bool); ok {
		d.On = v
	}
	if v, ok := dps[mode].(string); ok {
		d.Mode = v
	}
	if v, ok := num(dps[bright]); ok {
		d.Bright = d.rawToPct(v)
	}
	if v, ok := num(dps[temp]); ok {
		d.HasTemp, d.Temp = true, clamp(v*100/d.tempRawMax(), 0, 100)
	}
	if s, ok := dps[colour].(string); ok {
		d.HasColour = true
		d.parseColour(s)
	}
	d.Scene = ""
	if d.InScene() {
		if d.Style == 'B' {
			d.Scene = effectNameB(dps["25"])
		} else {
			d.Scene = effectNameA(d.Mode, dps["6"])
		}
	}
}

func (d *Device) parseColour(s string) {
	hexn := func(a, b int) int { n, _ := strconv.ParseInt(s[a:b], 16, 32); return int(n) }
	switch {
	case d.Style == 'B' && len(s) == 12:
		d.Hue, d.Sat = hexn(0, 4), clamp(hexn(4, 8)/10, 0, 100)
		if d.Mode == "colour" {
			d.Bright = d.rawToPct(hexn(8, 12))
		}
	case d.Style == 'A' && len(s) == 14:
		d.Hue, d.Sat = hexn(6, 10), clamp(hexn(10, 12)*100/255, 0, 100)
		if d.Mode == "colour" {
			d.Bright = clamp(hexn(12, 14)*100/255, 0, 100)
		}
	}
}

func (d *Device) applyCloud(items []tuyacloud.StatusItem) {
	for _, it := range items {
		switch strings.TrimSuffix(it.Code, "_v2") {
		case "switch_led", "switch_1", "switch":
			if v, ok := it.Value.(bool); ok {
				d.On = v
			}
		case "work_mode":
			if v, ok := it.Value.(string); ok {
				d.Mode = v
			}
		case "bright_value":
			if v, ok := num(it.Value); ok {
				d.Bright = d.rawToPct(v)
			}
		case "temp_value":
			if v, ok := num(it.Value); ok {
				d.Temp = clamp(v*100/d.tempRawMax(), 0, 100)
			}
		case "colour_data":
			m := map[string]float64{}
			switch t := it.Value.(type) {
			case string:
				_ = json.Unmarshal([]byte(t), &m)
			case map[string]any:
				for k, v := range t {
					m[k], _ = v.(float64)
				}
			}
			sMax := 255.0
			if d.Style == 'B' {
				sMax = 1000
			}
			if len(m) == 0 {
				continue
			}
			d.Hue, d.Sat = int(m["h"]), clamp(int(m["s"]*100/sMax), 0, 100)
			if d.Mode == "colour" {
				d.Bright = clamp(int(m["v"]*100/sMax), 0, 100)
			}
		}
	}
}

func (h *Home) mutate(id string, f func(d *Device) uint8) {
	h.mu.Lock()
	d, ok := h.byID[id]
	var mask uint8
	if ok {
		mask, d.quiet = f(d), time.Now().Add(1500*time.Millisecond)
	}
	h.mu.Unlock()
	if !ok || mask == 0 {
		return
	}
	h.notify()
	h.wmu.Lock()
	h.pend[id] |= mask
	start := !h.busy[id]
	h.busy[id] = true
	h.wmu.Unlock()
	if start {
		go h.worker(id)
	}
}

func (h *Home) worker(id string) {
	for {
		h.wmu.Lock()
		mask := h.pend[id]
		h.pend[id] = 0
		if mask == 0 {
			h.busy[id] = false
			h.wmu.Unlock()
			return
		}
		h.wmu.Unlock()
		h.send(id, mask)
	}
}

func (h *Home) send(id string, mask uint8) {
	h.mu.RLock()
	d, ok := h.byID[id]
	var cp Device
	if ok {
		cp = *d
	}
	h.mu.RUnlock()
	if !ok {
		return
	}
	via := ""
	if cp.IP != "" && lanlink.NewConn(cp.ID, cp.IP, cp.Ver, cp.key).Set(cp.lanDps(mask)) == nil {
		via = "lan"
	}
	if via == "" && h.cl != nil && h.cl.Send(cp.ID, cp.cloudCmds(mask)) == nil {
		via = "cloud"
	}
	h.mu.Lock()
	if d, ok := h.byID[id]; ok {
		if via != "" {
			d.Via = via
		} else {
			d.Online = false
		}
	}
	h.mu.Unlock()
	h.notify()
}

func (h *Home) SetPower(id string, on bool) {
	h.mutate(id, func(d *Device) uint8 { d.On = on; return pPower })
}

func (h *Home) SetBright(id string, pct int) {
	h.mutate(id, func(d *Device) uint8 {
		d.Bright = clamp(pct, 1, 100)
		m := pWhite
		if d.Mode == "colour" {
			m = pColour
		}
		if !d.On {
			d.On, m = true, m|pPower
		}
		return m
	})
}

func (h *Home) SetTemp(id string, pct int) {
	h.mutate(id, func(d *Device) uint8 {
		d.Temp, d.Mode = clamp(pct, 0, 100), "white"
		m := pWhite
		if !d.On {
			d.On, m = true, m|pPower
		}
		return m
	})
}

func (h *Home) SetColour(id string, hue, sat int) {
	h.mutate(id, func(d *Device) uint8 {
		d.Hue, d.Sat, d.Mode = hue%360, clamp(sat, 0, 100), "colour"
		if d.Bright < 5 {
			d.Bright = 100
		}
		m := pColour
		if !d.On {
			d.On, m = true, m|pPower
		}
		return m
	})
}

func (h *Home) SetWhite(id string) {
	h.mutate(id, func(d *Device) uint8 { d.Mode = "white"; return pWhite })
}

func (h *Home) ids(lightsOnly bool) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []string
	for _, d := range h.devs {
		if !lightsOnly || d.Kind == KindLight {
			out = append(out, d.ID)
		}
	}
	return out
}

func (h *Home) AllLights(on bool) {
	for _, id := range h.ids(true) {
		h.SetPower(id, on)
	}
}

func (h *Home) RoomLights(room string, on bool) {
	for _, d := range h.Snapshot() {
		if d.Kind == KindLight && d.Room == room {
			h.SetPower(d.ID, on)
		}
	}
}

var SceneNames = []string{"Bright", "Cozy", "Movie", "Night", "Away"}

func (h *Home) Scene(name string) { h.sceneIn(h.ids(false), name) }

func (h *Home) sceneIn(scope []string, name string) {
	specs := map[string][2]int{"Bright": {100, 70}, "Cozy": {45, 6}, "Movie": {14, 4}, "Night": {6, 0}}
	in := map[string]bool{}
	for _, id := range scope {
		in[id] = true
	}
	if name == "Away" {
		for _, id := range scope {
			h.SetPower(id, false)
		}
		return
	}
	sp, ok := specs[name]
	if !ok {
		return
	}
	var lit, all []string
	for _, d := range h.Snapshot() {
		if d.Kind != KindLight || !in[d.ID] {
			continue
		}
		all = append(all, d.ID)
		if d.Online && d.On {
			lit = append(lit, d.ID)
		}
	}
	targets := lit
	if len(targets) == 0 {
		targets = all
	}
	for _, id := range targets {
		h.mutate(id, func(d *Device) uint8 {
			d.On, d.Mode, d.Bright = true, "white", sp[0]
			if d.HasTemp {
				d.Temp = sp[1]
			}
			return pPower | pWhite
		})
	}
}

func (h *Home) Run(ctx context.Context) {
	go h.Rescan()
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		h.pollOnce()
		iv := 30 * time.Second
		if h.fast.Load() {
			iv = 2500 * time.Millisecond
			h.wmu.Lock()
			stale := time.Since(h.lastScan) > 45*time.Second
			h.wmu.Unlock()
			if stale {
				go h.Rescan()
			}
		}
		t.Reset(iv)
	}
}

func (h *Home) Rescan() {
	h.wmu.Lock()
	if time.Since(h.lastScan) < 5*time.Second {
		h.wmu.Unlock()
		return
	}
	h.lastScan = time.Now()
	h.wmu.Unlock()
	found := lanlink.Discover(8 * time.Second)
	h.mu.Lock()
	for id, f := range found {
		if d, ok := h.byID[id]; ok {
			d.IP, d.Ver = f.IP, f.Version
		}
	}
	h.mu.Unlock()
	h.notify()
}

func (h *Home) pollOnce() {
	list := h.Snapshot()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	var needCloud atomic.Bool
	for _, c := range list {
		if h.isStreaming(c.ID) {
			continue
		}
		if c.IP == "" || (c.Ver != "3.3" && c.Ver != "3.1") {
			needCloud.Store(true)
			continue
		}
		wg.Add(1)
		go func(c Device) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dps, err := lanlink.NewConn(c.ID, c.IP, c.Ver, c.key).Status()
			h.mu.Lock()
			defer h.mu.Unlock()
			d, ok := h.byID[c.ID]
			if !ok {
				return
			}
			if err != nil {
				if d.lanFail++; d.lanFail >= 2 {
					d.IP = ""
					needCloud.Store(true)
				}
				return
			}
			d.lanFail, d.Via, d.Online = 0, "lan", true
			if time.Now().After(d.quiet) {
				d.applyLan(dps)
			}
		}(c)
	}
	wg.Wait()
	if needCloud.Load() {
		h.cloudSync()
	}
	h.notify()
}

func (h *Home) cloudSync() {
	if h.cl == nil {
		return
	}
	h.wmu.Lock()
	if time.Since(h.lastCloud) < 15*time.Second {
		h.wmu.Unlock()
		return
	}
	h.lastCloud = time.Now()
	ids := h.homeIDs
	h.wmu.Unlock()
	if ids == nil {
		homes, err := h.cl.Homes()
		if err != nil {
			return
		}
		for _, hm := range homes {
			ids = append(ids, hm.ID)
		}
		h.wmu.Lock()
		h.homeIDs = ids
		h.wmu.Unlock()
	}
	for _, hid := range ids {
		list, err := h.cl.HomeDevices(hid)
		if err != nil {
			continue
		}
		h.mu.Lock()
		for _, c := range list {
			d, ok := h.byID[c.ID]
			if !ok || (d.Via == "lan" && d.IP != "") || h.streaming[c.ID] {
				continue
			}
			d.Online, d.Via = c.Online, "cloud"
			if time.Now().After(d.quiet) {
				d.applyCloud(c.Status)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Home) SetEffect(id, name string) {
	if !ValidEffect(name) {
		return
	}
	h.mutate(id, func(d *Device) uint8 {
		if !d.HasScene {
			return 0
		}
		d.On, d.Mode, d.Scene = true, "scene", name
		return pPower | pScene
	})
}
