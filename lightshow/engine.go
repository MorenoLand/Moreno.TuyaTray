package lightshow

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Config struct {
	Mode      string     `json:"mode"`
	Devices   []string   `json:"devices,omitempty"`
	Region    [4]float64 `json:"region"`
	Split     bool       `json:"split"`
	Gain      int        `json:"gain"`
	MaxBright int        `json:"max_bright"`
}

type Output interface {
	Colour(id string, hue, sat, bright int)
	IDs() []string
	Close()
}

type Opener func(ids []string) (Output, []string)

type State struct {
	Running bool   `json:"running"`
	Mode    string `json:"mode"`
	Lights  int    `json:"lights"`
	Skipped int    `json:"skipped"`
	Error   string `json:"error,omitempty"`
}

type Engine struct {
	mu     sync.Mutex
	open   Opener
	cancel context.CancelFunc
	state  State
	onEnd  func()
}

func New(open Opener, onEnd func()) *Engine { return &Engine{open: open, onEnd: onEnd} }

func Normalize(c Config) Config {
	if c.Gain < 20 || c.Gain > 400 {
		c.Gain = 100
	}
	if c.MaxBright < 10 || c.MaxBright > 100 {
		c.MaxBright = 100
	}
	r := c.Region
	if r[2]-r[0] < 0.02 || r[3]-r[1] < 0.02 || r[0] < 0 || r[1] < 0 || r[2] > 1 || r[3] > 1 {
		c.Region = [4]float64{0, 0, 1, 1}
	}
	return c
}

func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) Start(cfg Config) error {
	cfg = Normalize(cfg)
	if cfg.Mode != "screen" && cfg.Mode != "sound" {
		return errors.New("mode must be screen or sound")
	}
	if len(cfg.Devices) == 0 {
		return errors.New("pick at least one light")
	}
	e.Stop()
	out, skipped := e.open(cfg.Devices)
	if out == nil || len(out.IDs()) == 0 {
		if out != nil {
			out.Close()
		}
		return errors.New("none of the chosen lights can be reached on the local network")
	}
	src, err := newSource(cfg, len(out.IDs()))
	if err != nil {
		out.Close()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.cancel = cancel
	e.state = State{Running: true, Mode: cfg.Mode, Lights: len(out.IDs()), Skipped: len(skipped)}
	e.mu.Unlock()
	go e.run(ctx, cfg, out, src)
	return nil
}

func (e *Engine) run(ctx context.Context, cfg Config, out Output, src source) {
	defer func() {
		src.Close()
		out.Close()
		e.mu.Lock()
		e.state.Running = false
		e.mu.Unlock()
		if e.onEnd != nil {
			e.onEnd()
		}
	}()
	ids := out.IDs()
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		frame, err := src.Next()
		if err != nil {
			e.mu.Lock()
			e.state.Error = err.Error()
			e.mu.Unlock()
			return
		}
		for i, id := range ids {
			c := frame[i*len(frame)/len(ids)]
			out.Colour(id, c.H, c.S, c.V)
		}
	}
}

type source interface {
	Next() ([]HSV, error)
	Close()
}

type HSV struct{ H, S, V int }

func newSource(cfg Config, lights int) (source, error) {
	if cfg.Mode == "screen" {
		return newScreenSource(cfg, lights)
	}
	return newSoundSource(cfg, lights)
}
