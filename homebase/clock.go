package homebase

import (
	"sync"
	"time"
)

type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
	At      string `json:"at"`
	Days    []int  `json:"days,omitempty"`
	Target  string `json:"target"`
	Action  string `json:"action"`
}

func (r Rule) due(t time.Time) bool {
	if !r.Enabled || r.At != t.Format("15:04") {
		return false
	}
	if len(r.Days) == 0 {
		return true
	}
	for _, d := range r.Days {
		if d == int(t.Weekday()) {
			return true
		}
	}
	return false
}

type Clock struct {
	mu      sync.Mutex
	rules   []Rule
	fired   map[string]string
	apply   func(target, action string) error
	timerAt time.Time
	timerTg string
	timerAc string
	stop    chan struct{}
}

func NewClock(apply func(target, action string) error) *Clock {
	return &Clock{apply: apply, fired: map[string]string{}, stop: make(chan struct{})}
}

func (c *Clock) Start() {
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-c.stop:
				return
			case now := <-t.C:
				c.tick(now)
			}
		}
	}()
}

func (c *Clock) Close() { close(c.stop) }

func (c *Clock) tick(now time.Time) {
	type job struct{ target, action string }
	var jobs []job
	key := now.Format("2006-01-02 15:04")
	c.mu.Lock()
	for _, r := range c.rules {
		if r.due(now) && c.fired[r.ID] != key {
			c.fired[r.ID] = key
			jobs = append(jobs, job{r.Target, r.Action})
		}
	}
	if !c.timerAt.IsZero() && !now.Before(c.timerAt) {
		jobs = append(jobs, job{c.timerTg, c.timerAc})
		c.timerAt = time.Time{}
	}
	c.mu.Unlock()
	for _, j := range jobs {
		_ = c.apply(j.target, j.action)
	}
}

func (c *Clock) SetRules(r []Rule) {
	c.mu.Lock()
	c.rules = append([]Rule(nil), r...)
	c.mu.Unlock()
}

func (c *Clock) Rules() []Rule {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Rule(nil), c.rules...)
}

func (c *Clock) StartTimer(d time.Duration, target, action string) {
	c.mu.Lock()
	c.timerAt, c.timerTg, c.timerAc = time.Now().Add(d), target, action
	c.mu.Unlock()
}

func (c *Clock) CancelTimer() {
	c.mu.Lock()
	c.timerAt = time.Time{}
	c.mu.Unlock()
}

func (c *Clock) TimerLeft() (time.Duration, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timerAt.IsZero() {
		return 0, "", ""
	}
	return max(0, time.Until(c.timerAt)), c.timerTg, c.timerAc
}
