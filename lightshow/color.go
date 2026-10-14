package lightshow

import "math"

type rgbF struct{ R, G, B float64 }

func (c rgbF) mixTo(o rgbF, k float64) rgbF {
	return rgbF{c.R + (o.R-c.R)*k, c.G + (o.G-c.G)*k, c.B + (o.B-c.B)*k}
}

func toHSV(c rgbF) (h, s, v float64) {
	mx, mn := math.Max(c.R, math.Max(c.G, c.B)), math.Min(c.R, math.Min(c.G, c.B))
	v = mx
	d := mx - mn
	if mx > 0 {
		s = d / mx
	}
	switch {
	case d == 0:
		h = 0
	case mx == c.R:
		h = math.Mod((c.G-c.B)/d, 6)
	case mx == c.G:
		h = (c.B-c.R)/d + 2
	default:
		h = (c.R-c.G)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return
}

func fromHSV(h, s, v float64) rgbF {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x, m := c*(1-math.Abs(math.Mod(h/60, 2)-1)), v-c
	switch {
	case h < 60:
		return rgbF{c + m, x + m, m}
	case h < 120:
		return rgbF{x + m, c + m, m}
	case h < 180:
		return rgbF{m, c + m, x + m}
	case h < 240:
		return rgbF{m, x + m, c + m}
	case h < 300:
		return rgbF{x + m, m, c + m}
	}
	return rgbF{c + m, m, x + m}
}

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func pct(v float64, max int) int {
	return int(clampF(v, 0, 1)*float64(max) + 0.5)
}
