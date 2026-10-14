package homebase

import (
	"testing"
	"time"
)

func TestRuleDue(t *testing.T) {
	mon := time.Date(2026, 10, 12, 7, 30, 0, 0, time.Local)
	r := Rule{Enabled: true, At: "07:30"}
	if !r.due(mon) {
		t.Fatal("every-day rule should fire")
	}
	r.Days = []int{0, 6}
	if r.due(mon) {
		t.Fatal("weekend rule must not fire on Monday")
	}
	r.Days = []int{1}
	r.Enabled = false
	if r.due(mon) {
		t.Fatal("disabled rule must not fire")
	}
}

func TestClockFiresOncePerMinuteAndTimer(t *testing.T) {
	var got []string
	c := NewClock(func(target, action string) error { got = append(got, target+" "+action); return nil })
	c.SetRules([]Rule{{ID: "a", Enabled: true, At: "22:00", Target: "all", Action: "off"}})
	at := time.Date(2026, 10, 12, 22, 0, 5, 0, time.Local)
	c.tick(at)
	c.tick(at.Add(10 * time.Second))
	if len(got) != 1 {
		t.Fatalf("want one firing, got %v", got)
	}
	c.StartTimer(time.Minute, "everything", "off")
	c.tick(time.Now())
	if len(got) != 1 {
		t.Fatal("timer fired early")
	}
	c.tick(time.Now().Add(2 * time.Minute))
	if len(got) != 2 || got[1] != "everything off" {
		t.Fatalf("timer should fire: %v", got)
	}
	if left, _, _ := c.TimerLeft(); left != 0 {
		t.Fatal("timer should clear")
	}
}
