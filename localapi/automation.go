package localapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"moreno/tuyatray/homebase"
	"moreno/tuyatray/lightshow"
)

var clockText = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type timerReq struct {
	Minutes int    `json:"minutes"`
	Target  string `json:"target"`
	Action  string `json:"action"`
}

type actionReq struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

func validAction(a string) bool {
	kind, arg, _ := strings.Cut(a, ":")
	switch kind {
	case "on", "off":
		return arg == ""
	case "bright":
		n, err := strconv.Atoi(arg)
		return err == nil && n >= 1 && n <= 100
	case "scene":
		for _, n := range homebase.SceneNames {
			if n == arg {
				return true
			}
		}
	case "effect":
		return homebase.ValidEffect(arg)
	}
	return false
}

func validTarget(t string) bool {
	return t == "all" || t == "everything" || (strings.HasPrefix(t, "room:") && len(t) > 5) || (strings.HasPrefix(t, "device:") && len(t) > 7)
}

func cleanRules(in []homebase.Rule) ([]homebase.Rule, string) {
	if len(in) > 64 {
		return nil, "too many schedules"
	}
	out := make([]homebase.Rule, 0, len(in))
	for _, r := range in {
		if !clockText.MatchString(r.At) {
			return nil, "time must look like 07:30"
		}
		if !validTarget(r.Target) {
			return nil, "target must be all, everything, room:NAME or device:ID"
		}
		if !validAction(r.Action) {
			return nil, "unknown action " + r.Action
		}
		for _, d := range r.Days {
			if d < 0 || d > 6 {
				return nil, "days must be 0 (Sunday) to 6 (Saturday)"
			}
		}
		if r.ID == "" {
			b := make([]byte, 6)
			_, _ = rand.Read(b)
			r.ID = hex.EncodeToString(b)
		}
		r.Name = strings.TrimSpace(r.Name)
		out = append(out, r)
	}
	return out, ""
}

func (s *Server) automation(mux *http.ServeMux) {
	mux.HandleFunc("GET /schedules", func(w http.ResponseWriter, r *http.Request) {
		out := s.c.Rules()
		if out == nil {
			out = []homebase.Rule{}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("PUT /schedules", func(w http.ResponseWriter, r *http.Request) {
		var in []homebase.Rule
		if !body(w, r, &in) {
			return
		}
		out, msg := cleanRules(in)
		if msg != "" {
			fail(w, http.StatusBadRequest, msg)
			return
		}
		s.c.SaveRules(out)
		if out == nil {
			out = []homebase.Rule{}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("POST /actions", func(w http.ResponseWriter, r *http.Request) {
		var in actionReq
		if !body(w, r, &in) {
			return
		}
		if !validTarget(in.Target) || !validAction(in.Action) {
			fail(w, http.StatusBadRequest, "invalid target or action")
			return
		}
		if err := s.c.Apply(in.Target, in.Action); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	timer := func() map[string]any {
		left, tg, ac := s.c.TimerState()
		return map[string]any{"active": left > 0, "seconds": int(left / time.Second), "target": tg, "action": ac}
	}
	mux.HandleFunc("GET /timer", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, timer()) })
	mux.HandleFunc("POST /timer", func(w http.ResponseWriter, r *http.Request) {
		var in timerReq
		if !body(w, r, &in) {
			return
		}
		if in.Target == "" {
			in.Target = "everything"
		}
		if in.Action == "" {
			in.Action = "off"
		}
		if in.Minutes < 1 || in.Minutes > 24*60 || !validTarget(in.Target) || !validAction(in.Action) {
			fail(w, http.StatusBadRequest, "minutes must be 1 to 1440 with a valid target and action")
			return
		}
		s.c.StartTimer(in.Minutes, in.Target, in.Action)
		writeJSON(w, 200, timer())
	})
	mux.HandleFunc("DELETE /timer", func(w http.ResponseWriter, r *http.Request) {
		s.c.CancelTimer()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /show", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"state": s.c.ShowState(), "config": s.c.ShowConfig()})
	})
	mux.HandleFunc("POST /show/start", func(w http.ResponseWriter, r *http.Request) {
		var in lightshow.Config
		if !body(w, r, &in) {
			return
		}
		if err := s.c.StartShow(in); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"state": s.c.ShowState(), "config": s.c.ShowConfig()})
	})
	mux.HandleFunc("POST /show/stop", func(w http.ResponseWriter, r *http.Request) {
		s.c.StopShow()
		w.WriteHeader(http.StatusNoContent)
	})
}

func automationSpec() (map[string]any, map[string]any) {
	prop := func(t string) map[string]any { return map[string]any{"type": t} }
	obj := func(p map[string]any) map[string]any { return map[string]any{"type": "object", "properties": p} }
	paths := map[string]any{
		"/schedules":  map[string]any{"get": op("Automation", "List schedules", nil, "", "RuleArray"), "put": op("Automation", "Replace all schedules. target is all, everything, room:NAME or device:ID. action is on, off, bright:N, scene:NAME or effect:NAME. days is 0 (Sunday) to 6, empty means every day", nil, "RuleArray", "RuleArray")},
		"/actions":    map[string]any{"post": op("Automation", "Run an action once on a target", nil, "ActionRequest", "")},
		"/timer":      map[string]any{"get": op("Automation", "Current sleep timer", nil, "", "TimerState"), "post": op("Automation", "Start a timer that runs an action after N minutes (default: everything off)", nil, "TimerRequest", "TimerState"), "delete": op("Automation", "Cancel the timer", nil, "", "")},
		"/show":       map[string]any{"get": op("Light show", "State and last used configuration of the screen/sound light show", nil, "", "ShowStatus")},
		"/show/start": map[string]any{"post": op("Light show", "Start syncing lights to the screen or to the sound playing on this PC (mode screen or sound)", nil, "ShowConfig", "ShowStatus")},
		"/show/stop":  map[string]any{"post": op("Light show", "Stop the light show", nil, "", "")},
	}
	rule := obj(map[string]any{"id": prop("string"), "name": prop("string"), "enabled": prop("boolean"), "at": prop("string"), "days": map[string]any{"type": "array", "items": prop("integer")}, "target": prop("string"), "action": prop("string")})
	cfg := obj(map[string]any{"mode": map[string]any{"type": "string", "enum": []string{"screen", "sound"}}, "devices": map[string]any{"type": "array", "items": prop("string")}, "region": map[string]any{"type": "array", "items": prop("number"), "minItems": 4, "maxItems": 4}, "split": prop("boolean"), "gain": prop("integer"), "max_bright": prop("integer")})
	schemas := map[string]any{
		"RuleArray":     map[string]any{"type": "array", "items": ref("Rule")},
		"Rule":          rule,
		"ActionRequest": obj(map[string]any{"target": prop("string"), "action": prop("string")}),
		"TimerRequest":  obj(map[string]any{"minutes": prop("integer"), "target": prop("string"), "action": prop("string")}),
		"TimerState":    obj(map[string]any{"active": prop("boolean"), "seconds": prop("integer"), "target": prop("string"), "action": prop("string")}),
		"ShowConfig":    cfg,
		"ShowStatus":    obj(map[string]any{"state": obj(map[string]any{"running": prop("boolean"), "mode": prop("string"), "lights": prop("integer"), "skipped": prop("integer"), "error": prop("string")}), "config": ref("ShowConfig")}),
	}
	return paths, schemas
}
