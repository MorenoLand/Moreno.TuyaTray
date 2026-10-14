package homeplan

import "testing"

func TestPresetsParse(t *testing.T) {
	names := Presets()
	if len(names) == 0 {
		t.Fatal("no presets")
	}
	for _, n := range names {
		p, err := Preset(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if len(p.Rooms) == 0 || len(p.Slots) == 0 {
			t.Fatalf("%s has no rooms or slots", n)
		}
	}
}

func TestTagMatchesRoomNames(t *testing.T) {
	p, err := Preset("apartment")
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{"Master Bedroom": "bed", "Kitchen": "kitchen", "Loft": "loft", "Garage": ""} {
		if got := p.Tag(in); got != want {
			t.Fatalf("Tag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRejectsBadPlans(t *testing.T) {
	for _, in := range []string{`{}`, `{"size":[7,5,2],"slots":[]}`, `{"size":[7,5,2],"slots":[{"room":"a"}],"walls":[{"b":[0,0,0,1,1,1],"side":"X"}]}`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Fatalf("expected an error for %s", in)
		}
	}
}
