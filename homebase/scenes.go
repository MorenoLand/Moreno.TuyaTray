package homebase

import "strings"

var Effects = []string{"Night", "Reading", "Cocktail", "Leisure", "Soft", "Blinking", "Rainbow", "Rave", "Nature"}

var effectsB = map[string]string{
	"Night":    "00e5e500002c006403e800000000",
	"Reading":  "01fcfc000000000003e700000000",
	"Cocktail": "02fbfb00012100dc03d400000000",
	"Leisure":  "03bebe00003c00fa03e800000000",
	"Soft":     "03bbbb00002d001a03e800000000",
	"Blinking": "05505001000003e803e800000000505001007803e803e80000000050500100f003e803e800000000",
	"Rainbow":  "07646402000003e803e800000000646402007803e803e800000000646402003c03e803e800000000646402012c03e803e80000000064640200f003e803e800000000",
	"Rave":     "06565601000003e803e80000000056560100000000000000000000",
	"Nature":   "04444402007803e803e800000000444402007803e8000a00000000",
}

var effectsA = map[string][2]string{
	"Night":    {"scene", "fff8e5002b0a64"},
	"Reading":  {"scene", "fcfcfc00000063"},
	"Cocktail": {"scene", "f1c3fb01211662"},
	"Leisure":  {"scene", "ffffbe003c1964"},
	"Soft":     {"scene", "feedbb002c1a64"},
	"Blinking": {"scene_2", ""},
	"Rainbow":  {"scene_4", ""},
	"Rave":     {"scene_3", ""},
	"Nature":   {"scene_1", ""},
}

func ValidEffect(n string) bool { _, ok := effectsB[n]; return ok }

func effectNameB(v any) string {
	s, _ := v.(string)
	for n, d := range effectsB {
		if d == s {
			return n
		}
	}
	return ""
}

func effectNameA(mode string, v any) string {
	s, _ := v.(string)
	for n, e := range effectsA {
		if e[0] == mode && (e[1] == "" || e[1] == s) {
			return n
		}
	}
	return ""
}

func (d *Device) InScene() bool { return strings.HasPrefix(d.Mode, "scene") }
