package localapi

import (
	"encoding/json"
	"io"
	"net/http"

	"moreno/tuyatray/homeplan"
)

type planChoice struct {
	Preset string `json:"preset"`
}

func (s *Server) planRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /plan", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"id": s.c.PlanName(), "plan": s.c.Plan()})
	})
	mux.HandleFunc("GET /plans", func(w http.ResponseWriter, r *http.Request) {
		type entry struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		out := []entry{}
		for _, id := range homeplan.Presets() {
			if p, err := homeplan.Preset(id); err == nil {
				out = append(out, entry{id, p.Name})
			}
		}
		if s.c.HasCustomPlan() {
			out = append(out, entry{"custom", "Custom (house.json)"})
		}
		writeJSON(w, 200, map[string]any{"active": s.c.PlanName(), "plans": out})
	})
	mux.HandleFunc("PUT /plan", func(w http.ResponseWriter, r *http.Request) {
		var in planChoice
		if !body(w, r, &in) {
			return
		}
		if err := s.c.SetPlan(in.Preset); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"id": s.c.PlanName(), "plan": s.c.Plan()})
	})
	mux.HandleFunc("GET /plan/custom", func(w http.ResponseWriter, r *http.Request) {
		b := s.c.CustomPlanJSON()
		if len(b) == 0 {
			b, _ = json.MarshalIndent(s.c.Plan(), "", "  ")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("PUT /plan/custom", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			fail(w, http.StatusBadRequest, "body too large")
			return
		}
		if err := s.c.SetCustomPlan(b); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"id": "custom", "plan": s.c.Plan()})
	})
}
