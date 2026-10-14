package panel

import (
	"testing"

	"moreno/tuyatray/homebase"
)

func TestArrangePairsSingleLightRooms(t *testing.T) {
	p := &Panel{w: 408}
	p.devs = []homebase.Device{
		{ID: "1", Room: "Bath"}, {ID: "2", Room: "Hall"},
		{ID: "3", Room: "Living"}, {ID: "4", Room: "Living"}, {ID: "5", Room: "Living"},
		{ID: "6", Room: "Kitchen"},
	}
	cells, h := p.arrange()
	pos := map[string][2]float32{}
	for _, c := range cells {
		if c.head == "" {
			pos[c.d.ID] = [2]float32{c.x, c.y}
		}
	}
	if pos["1"][1] != pos["2"][1] || pos["1"][0] == pos["2"][0] {
		t.Fatalf("Bath and Hall should share a row: %v %v", pos["1"], pos["2"])
	}
	if pos["3"][1] != pos["4"][1] || pos["5"][1] <= pos["3"][1] {
		t.Fatalf("Living tiles should fill two columns: %v", pos)
	}
	if pos["6"][1] <= pos["5"][1] || h <= pos["6"][1] {
		t.Fatalf("Kitchen should sit below Living: %v h=%v", pos, h)
	}
}

func TestArrangeFilterUsesFullGrid(t *testing.T) {
	p := &Panel{w: 408, filter: "Bath"}
	p.devs = []homebase.Device{{ID: "1", Room: "Bath"}, {ID: "2", Room: "Hall"}}
	cells, _ := p.arrange()
	if len(cells) != 1 || cells[0].d.ID != "1" || cells[0].x != gridPad {
		t.Fatalf("filtered view should show only Bath at the left edge: %+v", cells)
	}
}
