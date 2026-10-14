package homebase

import (
	"errors"
	"strconv"
	"strings"
)

func (h *Home) Targets(target string) []string {
	var out []string
	for _, d := range h.Snapshot() {
		switch {
		case target == "" || target == "all":
			if d.Kind == KindLight {
				out = append(out, d.ID)
			}
		case target == "everything":
			out = append(out, d.ID)
		case strings.HasPrefix(target, "room:"):
			if d.Kind == KindLight && d.Room == strings.TrimPrefix(target, "room:") {
				out = append(out, d.ID)
			}
		case strings.HasPrefix(target, "device:"):
			if d.ID == strings.TrimPrefix(target, "device:") {
				out = append(out, d.ID)
			}
		}
	}
	return out
}

func (h *Home) Apply(target, action string) error {
	ids := h.Targets(target)
	if len(ids) == 0 {
		return errors.New("no devices match " + target)
	}
	kind, arg, _ := strings.Cut(action, ":")
	switch kind {
	case "on", "off":
		for _, id := range ids {
			h.SetPower(id, kind == "on")
		}
	case "bright":
		n, err := strconv.Atoi(arg)
		if err != nil {
			return errors.New("brightness must be a number")
		}
		for _, id := range ids {
			h.SetBright(id, n)
		}
	case "scene":
		h.sceneIn(ids, arg)
	case "effect":
		if !ValidEffect(arg) {
			return errors.New("unknown effect " + arg)
		}
		for _, id := range ids {
			h.SetEffect(id, arg)
		}
	default:
		return errors.New("unknown action " + action)
	}
	return nil
}
