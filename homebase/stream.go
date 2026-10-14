package homebase

import (
	"sync"
	"time"

	"moreno/tuyatray/lanlink"
)

type streamDev struct {
	id   string
	s    *lanlink.Stream
	ch   chan lanlink.Dps
	done chan struct{}
	cp   Device
	live bool
}

type Stream struct {
	h        *Home
	mu       sync.Mutex
	dev      map[string]*streamDev
	order    []string
	lastNote time.Time
}

func (h *Home) isStreaming(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.streaming[id]
}

func (h *Home) OpenStream(ids []string) (*Stream, []string) {
	st := &Stream{h: h, dev: map[string]*streamDev{}}
	var skipped []string
	h.mu.Lock()
	if h.streaming == nil {
		h.streaming = map[string]bool{}
	}
	for _, id := range ids {
		d, ok := h.byID[id]
		if !ok || d.Kind != KindLight || d.IP == "" || (d.Ver != "3.3" && d.Ver != "3.1") || !d.Online {
			skipped = append(skipped, id)
			continue
		}
		sd := &streamDev{id: id, cp: *d, ch: make(chan lanlink.Dps, 1), done: make(chan struct{})}
		sd.s = lanlink.NewStream(lanlink.NewConn(d.ID, d.IP, d.Ver, d.key))
		st.dev[id] = sd
		st.order = append(st.order, id)
		h.streaming[id] = true
		go sd.run()
	}
	h.mu.Unlock()
	return st, skipped
}

func (sd *streamDev) run() {
	for {
		select {
		case <-sd.done:
			sd.s.Close()
			return
		case dps := <-sd.ch:
			_ = sd.s.Send(dps)
		}
	}
}

func (s *Stream) IDs() []string { return append([]string(nil), s.order...) }

func (s *Stream) Colour(id string, hue, sat, bright int) {
	sd, ok := s.dev[id]
	if !ok {
		return
	}
	d := sd.cp
	mask := pColour
	d.Bright = clamp(bright, 1, 100)
	if d.HasColour {
		d.Mode, d.Hue, d.Sat = "colour", ((hue%360)+360)%360, clamp(sat, 0, 100)
	} else {
		d.Mode, mask = "white", pWhite
	}
	if !sd.live {
		sd.live, mask = true, mask|pPower
		d.On = true
	}
	dps := d.lanDps(mask)
	select {
	case sd.ch <- dps:
	default:
		select {
		case <-sd.ch:
		default:
		}
		select {
		case sd.ch <- dps:
		default:
		}
	}
	s.h.mu.Lock()
	if hd, ok := s.h.byID[id]; ok {
		hd.On, hd.Mode, hd.Bright, hd.Hue, hd.Sat = true, d.Mode, d.Bright, d.Hue, d.Sat
		hd.quiet = time.Now().Add(2 * time.Second)
	}
	s.h.mu.Unlock()
	s.mu.Lock()
	note := time.Since(s.lastNote) > 400*time.Millisecond
	if note {
		s.lastNote = time.Now()
	}
	s.mu.Unlock()
	if note {
		s.h.notify()
	}
}

func (s *Stream) Close() {
	s.h.mu.Lock()
	for id, sd := range s.dev {
		close(sd.done)
		delete(s.h.streaming, id)
	}
	s.h.mu.Unlock()
	s.dev = map[string]*streamDev{}
}
