package localapi

import (
	"io"
	"net/http"
	"os"
	"strings"

	"moreno/tuyatray/homeicons"
)

type iconReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s *Server) iconRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /icons", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"icons": s.c.Icons(), "builtin": homeicons.Builtin})
	})
	mux.HandleFunc("PUT /icons", func(w http.ResponseWriter, r *http.Request) {
		var in iconReq
		if !body(w, r, &in) {
			return
		}
		if err := s.c.SetIcon(in.Key, in.Value); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /icons/image", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if !homeicons.ValidKey(key) {
			fail(w, http.StatusBadRequest, "key must be d:DEVICE or r:ROOM")
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 9<<20))
		if err != nil {
			fail(w, http.StatusBadRequest, "image is too large")
			return
		}
		v, err := s.c.ImportIcon(key, b)
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"value": v})
	})
	mux.HandleFunc("GET /icons/img/{file}", func(w http.ResponseWriter, r *http.Request) {
		f := r.PathValue("file")
		if !strings.HasSuffix(f, ".png") || !homeicons.ValidFile(f) {
			fail(w, http.StatusNotFound, "no such icon")
			return
		}
		b, err := os.ReadFile(s.c.IconPath(f))
		if err != nil {
			fail(w, http.StatusNotFound, "no such icon")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=86400")
		_, _ = w.Write(b)
	})
}
