package localapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/homeplan"
	"moreno/tuyatray/lightshow"
)

//go:embed swagger/*
var swaggerAssets embed.FS

//go:embed web/*
var webAssets embed.FS

var swaggerFS, _ = fs.Sub(swaggerAssets, "swagger")

var webFS, _ = fs.Sub(webAssets, "web")

type Control interface {
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
	Layout() (bool, map[string]int)
	Linked() bool
	StartLink(userCode string)
	LinkState() (payload, status string)
	Unlink()
	Plan() *homeplan.Plan
	PlanName() string
	HasCustomPlan() bool
	SetPlan(name string) error
	SetCustomPlan([]byte) error
	CustomPlanJSON() []byte
	Icons() map[string]string
	SetIcon(key, value string) error
	ImportIcon(key string, data []byte) (string, error)
	IconPath(file string) string
	Clock24() bool
	SetClock24(bool)
	Rules() []homebase.Rule
	SaveRules([]homebase.Rule)
	Apply(target, action string) error
	StartTimer(minutes int, target, action string)
	CancelTimer()
	TimerState() (time.Duration, string, string)
	ShowState() lightshow.State
	ShowConfig() lightshow.Config
	StartShow(lightshow.Config) error
	StopShow()
}

type Config struct {
	Port             int
	Allow            []string
	Auth             bool
	User, Salt, Hash string
}

type Server struct {
	c     Control
	cfg   Config
	srv   *http.Server
	allow []netip.Prefix
}

func Hash(salt, password string) string {
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:])
}

type Device struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Room                string `json:"room"`
	Kind                string `json:"kind"`
	Online              bool   `json:"online"`
	On                  bool   `json:"on"`
	Mode                string `json:"mode"`
	Brightness          int    `json:"brightness"`
	Temperature         int    `json:"temperature"`
	Hue                 int    `json:"hue"`
	Saturation          int    `json:"saturation"`
	SupportsTemperature bool   `json:"supportsTemperature"`
	SupportsColour      bool   `json:"supportsColour"`
	Via                 string `json:"via"`
	LanAddress          string `json:"lanAddress"`
	Effect              string `json:"effect"`
	SupportsEffects     bool   `json:"supportsEffects"`
}

type problem struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func toDevice(d homebase.Device) Device {
	k := "light"
	if d.Kind == homebase.KindSwitch {
		k = "switch"
	}
	return Device{d.ID, d.Name, d.Room, k, d.Online, d.On, d.Mode, d.Bright, d.Temp, d.Hue, d.Sat, d.HasTemp, d.HasColour, d.Via, d.IP, d.Scene, d.HasScene}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, problem{http.StatusText(status), status, detail})
}

func body(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func Start(cfg Config, c Control) (*Server, error) {
	s := &Server{c: c, cfg: cfg}
	bind := "127.0.0.1"
	for _, e := range cfg.Allow {
		if strings.TrimSpace(e) == "" {
			continue
		}
		e = strings.TrimSpace(e)
		pf, err := netip.ParsePrefix(e)
		if err != nil {
			a, err2 := netip.ParseAddr(e)
			if err2 != nil {
				return nil, fmt.Errorf("invalid allowlist entry %q", e)
			}
			pf = netip.PrefixFrom(a.Unmap(), a.BitLen())
		}
		s.allow = append(s.allow, pf)
		if !pf.Addr().IsLoopback() {
			bind = "0.0.0.0"
		}
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(bind, fmt.Sprint(cfg.Port)))
	if err != nil {
		return nil, err
	}
	s.srv = &http.Server{Handler: s.guard(s.routes()), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

func (s *Server) Close() error { return s.srv.Close() }

func (s *Server) permitted(ip netip.Addr) bool {
	if ip.IsLoopback() {
		return true
	}
	for _, p := range s.allow {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ap, err := netip.ParseAddrPort(r.RemoteAddr)
		if err != nil || !s.permitted(ap.Addr().Unmap()) {
			fail(w, http.StatusForbidden, "address not in the allowlist")
			return
		}
		if s.cfg.Auth {
			u, pw, ok := r.BasicAuth()
			good := ok && subtle.ConstantTimeCompare([]byte(u), []byte(s.cfg.User)) == 1 && subtle.ConstantTimeCompare([]byte(Hash(s.cfg.Salt, pw)), []byte(s.cfg.Hash)) == 1
			if !good {
				w.Header().Set("WWW-Authenticate", `Basic realm="TuyaTray"`)
				fail(w, http.StatusUnauthorized, "sign-in required")
				return
			}
		}
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			fail(w, http.StatusForbidden, "cross-origin request refused")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) device(w http.ResponseWriter, id string) (homebase.Device, bool) {
	for _, d := range s.c.Devices() {
		if d.ID == id {
			return d, true
		}
	}
	fail(w, http.StatusNotFound, "device not found")
	return homebase.Device{}, false
}

func (s *Server) act(w http.ResponseWriter, r *http.Request, in any, f func(id string)) {
	id := r.PathValue("id")
	if _, ok := s.device(w, id); !ok {
		return
	}
	if in != nil && !body(w, r, in) {
		return
	}
	f(id)
	time.Sleep(60 * time.Millisecond)
	d, _ := s.device(w, id)
	writeJSON(w, http.StatusOK, toDevice(d))
}

type onReq struct {
	On bool `json:"on"`
}

type pctReq struct {
	Percent int `json:"percent"`
}

type colourReq struct {
	Hue        int `json:"hue"`
	Saturation int `json:"saturation"`
}

type nameReq struct {
	Name string `json:"name"`
}

type slotReq struct {
	Slot int `json:"slot"`
}

type roomReq struct {
	Room string `json:"room"`
}

type linkReq struct {
	UserCode string `json:"userCode"`
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "TuyaTray", "version": "v1", "openapi": "/swagger/v1/swagger.json", "swagger": "/swagger/"})
	})
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		out := []Device{}
		for _, d := range s.c.Devices() {
			out = append(out, toDevice(d))
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("GET /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if d, ok := s.device(w, r.PathValue("id")); ok {
			writeJSON(w, 200, toDevice(d))
		}
	})
	mux.HandleFunc("POST /devices/{id}/power", func(w http.ResponseWriter, r *http.Request) {
		var in onReq
		s.act(w, r, &in, func(id string) { s.c.SetPower(id, in.On) })
	})
	mux.HandleFunc("POST /devices/{id}/brightness", func(w http.ResponseWriter, r *http.Request) {
		var in pctReq
		s.act(w, r, &in, func(id string) { s.c.SetBright(id, in.Percent) })
	})
	mux.HandleFunc("POST /devices/{id}/temperature", func(w http.ResponseWriter, r *http.Request) {
		var in pctReq
		s.act(w, r, &in, func(id string) { s.c.SetTemp(id, in.Percent) })
	})
	mux.HandleFunc("POST /devices/{id}/colour", func(w http.ResponseWriter, r *http.Request) {
		var in colourReq
		s.act(w, r, &in, func(id string) { s.c.SetColour(id, in.Hue, in.Saturation) })
	})
	mux.HandleFunc("POST /devices/{id}/effect", func(w http.ResponseWriter, r *http.Request) {
		var in nameReq
		s.act(w, r, &in, func(id string) { s.c.SetEffect(id, in.Name) })
	})
	mux.HandleFunc("GET /effects", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, homebase.Effects) })
	mux.HandleFunc("POST /devices/{id}/white", func(w http.ResponseWriter, r *http.Request) {
		s.act(w, r, nil, func(id string) { s.c.SetWhite(id) })
	})
	mux.HandleFunc("GET /rooms", func(w http.ResponseWriter, r *http.Request) {
		out, seen := []string{}, map[string]bool{}
		for _, d := range s.c.Devices() {
			if !seen[d.Room] {
				seen[d.Room], out = true, append(out, d.Room)
			}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("POST /rooms/{room}/power", func(w http.ResponseWriter, r *http.Request) {
		var in onReq
		if body(w, r, &in) {
			s.c.RoomLights(r.PathValue("room"), in.On)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("POST /lights/power", func(w http.ResponseWriter, r *http.Request) {
		var in onReq
		if body(w, r, &in) {
			s.c.AllLights(in.On)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("GET /scenes", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, homebase.SceneNames) })
	mux.HandleFunc("POST /scenes/{name}", func(w http.ResponseWriter, r *http.Request) {
		for _, n := range homebase.SceneNames {
			if strings.EqualFold(n, r.PathValue("name")) {
				s.c.Scene(n)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		fail(w, http.StatusNotFound, "scene not found")
	})
	mux.HandleFunc("PUT /devices/{id}/room", func(w http.ResponseWriter, r *http.Request) {
		var in roomReq
		s.act(w, r, &in, func(id string) { s.c.SetRoom(id, in.Room) })
	})
	mux.HandleFunc("PUT /devices/{id}/slot", func(w http.ResponseWriter, r *http.Request) {
		var in slotReq
		s.act(w, r, &in, func(id string) { s.c.SetSlot(id, in.Slot) })
	})
	mux.HandleFunc("GET /layout", func(w http.ResponseWriter, r *http.Request) {
		show, slots := s.c.Layout()
		if slots == nil {
			slots = map[string]int{}
		}
		writeJSON(w, 200, map[string]any{"showHouse": show, "slots": slots, "homeName": s.c.HomeName(), "clock24": s.c.Clock24(), "plan": s.c.PlanName(), "icons": s.c.Icons()})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, _ := fs.ReadFile(webFS, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	mux.Handle("GET /web/", http.StripPrefix("/web", noStore(http.FileServer(http.FS(webFS)))))
	mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
		s.c.Refresh()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /network/rescan", func(w http.ResponseWriter, r *http.Request) {
		s.c.Rescan()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /account", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"linked": s.c.Linked(), "homeName": s.c.HomeName()})
	})
	mux.HandleFunc("POST /account/link", func(w http.ResponseWriter, r *http.Request) {
		var in linkReq
		if body(w, r, &in) && strings.TrimSpace(in.UserCode) != "" {
			s.c.StartLink(strings.TrimSpace(in.UserCode))
			w.WriteHeader(http.StatusAccepted)
		}
	})
	mux.HandleFunc("GET /account/link", func(w http.ResponseWriter, r *http.Request) {
		qr, st := s.c.LinkState()
		writeJSON(w, 200, map[string]any{"qrPayload": qr, "status": st})
	})
	mux.HandleFunc("DELETE /account", func(w http.ResponseWriter, r *http.Request) {
		s.c.Unlink()
		w.WriteHeader(http.StatusNoContent)
	})
	s.automation(mux)
	s.planRoutes(mux)
	s.iconRoutes(mux)
	mux.HandleFunc("PUT /preferences", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Clock24 bool `json:"clock24"`
		}
		if body(w, r, &in) {
			s.c.SetClock24(in.Clock24)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /swagger/v1/swagger.json", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, spec(s.cfg.Auth)) })
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger", http.FileServer(http.FS(swaggerFS))))
	return mux
}

func ref(n string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + n} }

func pathParam(n string) map[string]any {
	return map[string]any{"name": n, "in": "path", "required": true, "schema": map[string]any{"type": "string"}}
}

func response(n string) map[string]any {
	switch n {
	case "":
		return map[string]any{"description": "OK"}
	case "Names":
		return map[string]any{"description": "OK", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}}
	}
	return map[string]any{"description": "OK", "content": map[string]any{"application/json": map[string]any{"schema": ref(n)}}}
}

func op(tag, summary string, params []any, req, resp string) map[string]any {
	o := map[string]any{"tags": []string{tag}, "summary": summary, "responses": map[string]any{"200": response(resp), "400": response("ProblemDetails"), "403": response("ProblemDetails"), "404": response("ProblemDetails")}}
	if params != nil {
		o["parameters"] = params
	}
	if req != "" {
		o["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": ref(req)}}}
	}
	return o
}

func spec(auth bool) map[string]any {
	id := []any{pathParam("id")}
	prop := func(t string) map[string]any { return map[string]any{"type": t} }
	obj := func(p map[string]any) map[string]any { return map[string]any{"type": "object", "properties": p} }
	out := map[string]any{
		"openapi": "3.0.1",
		"info":    map[string]any{"title": "TuyaTray API", "version": "v1"},
		"paths": map[string]any{
			"/devices":                  map[string]any{"get": op("Device", "List lights and plugs with live state", nil, "", "DeviceArray")},
			"/devices/{id}":             map[string]any{"get": op("Device", "Get one device", id, "", "Device")},
			"/devices/{id}/power":       map[string]any{"post": op("Device", "Turn a device on or off", id, "PowerRequest", "Device")},
			"/devices/{id}/brightness":  map[string]any{"post": op("Device", "Set brightness in percent", id, "PercentRequest", "Device")},
			"/devices/{id}/temperature": map[string]any{"post": op("Device", "Set colour temperature (0 warm, 100 cool)", id, "PercentRequest", "Device")},
			"/devices/{id}/colour":      map[string]any{"post": op("Device", "Set colour by hue and saturation", id, "ColourRequest", "Device")},
			"/devices/{id}/white":       map[string]any{"post": op("Device", "Switch a light to white mode", id, "", "Device")},
			"/devices/{id}/room":        map[string]any{"put": op("Device", "Move a device to a room (empty string restores the Tuya room)", id, "RoomRequest", "Device")},
			"/devices/{id}/effect":      map[string]any{"post": op("Device", "Run a built-in light effect: Night, Reading, Cocktail, Leisure, Soft, Blinking, Rainbow, Rave or Nature", id, "NameRequest", "Device")},
			"/effects":                  map[string]any{"get": op("Device", "List light effects", nil, "", "Names")},
			"/devices/{id}/slot":        map[string]any{"put": op("Device", "Place a light at a house slot (0-14)", id, "SlotRequest", "Device")},
			"/layout":                   map[string]any{"get": op("Network", "Home view settings and light placements", nil, "", "Layout")},
			"/plan":                     map[string]any{"get": op("Home design", "The active home design (isometric plan) as JSON", nil, "", ""), "put": op("Home design", "Switch to a built-in design by id", nil, "PlanChoice", "")},
			"/plans":                    map[string]any{"get": op("Home design", "List built-in designs and whether a custom one exists", nil, "", "")},
			"/plan/custom":              map[string]any{"get": op("Home design", "The saved custom design JSON (or the active design to start from)", nil, "", ""), "put": op("Home design", "Validate and save a custom design JSON document and switch to it", nil, "", "")},
			"/icons":                    map[string]any{"get": op("Icons", "Icon choices: keys d:DEVICEID or r:ROOM, values b:NAME (built-in) or i:FILE (custom image)", nil, "", ""), "put": op("Icons", "Set or reset one icon (value empty resets). Built-in names: bulb lamp strip plug tv fan bed sofa kitchen bath desk plant sun moon door home", nil, "IconRequest", "")},
			"/icons/image":              map[string]any{"post": op("Icons", "Upload a PNG, JPEG or GIF as the icon for query parameter key (d:DEVICEID or r:ROOM); it is cropped square and stored in the app data folder", nil, "", "")},
			"/preferences":              map[string]any{"put": op("Network", "Set display preferences (clock24: use 24-hour time)", nil, "PreferencesRequest", "")},
			"/refresh":                  map[string]any{"post": op("Network", "Refresh devices, rooms and names from Tuya and rescan the LAN", nil, "", "")},
			"/rooms":                    map[string]any{"get": op("Room", "List rooms", nil, "", "Names")},
			"/rooms/{room}/power":       map[string]any{"post": op("Room", "Turn every light in a room on or off", []any{pathParam("room")}, "PowerRequest", "")},
			"/lights/power":             map[string]any{"post": op("Room", "Turn every light on or off", nil, "PowerRequest", "")},
			"/scenes":                   map[string]any{"get": op("Scene", "List scenes", nil, "", "Names")},
			"/scenes/{name}":            map[string]any{"post": op("Scene", "Apply a scene", []any{pathParam("name")}, "", "")},
			"/network/rescan":           map[string]any{"post": op("Network", "Rediscover devices on the LAN", nil, "", "")},
			"/account":                  map[string]any{"get": op("Account", "Link state of the Tuya account", nil, "", "AccountState"), "delete": op("Account", "Unlink the Tuya account", nil, "", "")},
			"/account/link":             map[string]any{"get": op("Account", "Poll the link flow", nil, "", "LinkState"), "post": op("Account", "Start linking with a User Code", nil, "LinkRequest", "")},
		},
		"components": map[string]any{"schemas": map[string]any{
			"Device":             obj(map[string]any{"id": prop("string"), "name": prop("string"), "room": prop("string"), "kind": map[string]any{"type": "string", "enum": []string{"light", "switch"}}, "online": prop("boolean"), "on": prop("boolean"), "mode": prop("string"), "brightness": prop("integer"), "temperature": prop("integer"), "hue": prop("integer"), "saturation": prop("integer"), "supportsTemperature": prop("boolean"), "supportsColour": prop("boolean"), "via": prop("string"), "lanAddress": prop("string")}),
			"DeviceArray":        map[string]any{"type": "array", "items": ref("Device")},
			"PowerRequest":       obj(map[string]any{"on": prop("boolean")}),
			"PercentRequest":     obj(map[string]any{"percent": map[string]any{"type": "integer", "minimum": 0, "maximum": 100}}),
			"ColourRequest":      obj(map[string]any{"hue": map[string]any{"type": "integer", "minimum": 0, "maximum": 360}, "saturation": map[string]any{"type": "integer", "minimum": 0, "maximum": 100}}),
			"NameRequest":        obj(map[string]any{"name": prop("string")}),
			"SlotRequest":        obj(map[string]any{"slot": map[string]any{"type": "integer", "minimum": 0}}),
			"PlanChoice":         obj(map[string]any{"preset": prop("string")}),
			"IconRequest":        obj(map[string]any{"key": prop("string"), "value": prop("string")}),
			"PreferencesRequest": obj(map[string]any{"clock24": prop("boolean")}),
			"Layout":             obj(map[string]any{"showHouse": prop("boolean"), "clock24": prop("boolean"), "homeName": prop("string"), "slots": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}}}),
			"RoomRequest":        obj(map[string]any{"room": prop("string")}),
			"LinkRequest":        obj(map[string]any{"userCode": prop("string")}),
			"LinkState":          obj(map[string]any{"qrPayload": prop("string"), "status": prop("string")}),
			"AccountState":       obj(map[string]any{"linked": prop("boolean"), "homeName": prop("string")}),
			"ProblemDetails":     obj(map[string]any{"title": prop("string"), "status": prop("integer"), "detail": prop("string")}),
		}},
	}
	ap, as := automationSpec()
	for k, v := range ap {
		out["paths"].(map[string]any)[k] = v
	}
	for k, v := range as {
		out["components"].(map[string]any)["schemas"].(map[string]any)[k] = v
	}
	if auth {
		out["security"] = []any{map[string]any{"basic": []any{}}}
		out["components"].(map[string]any)["securitySchemes"] = map[string]any{"basic": map[string]any{"type": "http", "scheme": "basic"}}
	}
	return out
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
