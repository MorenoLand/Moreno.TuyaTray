package lightshow

import (
	"errors"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	gdi32           = windows.NewLazySystemDLL("gdi32.dll")
	pGetDC          = user32.NewProc("GetDC")
	pReleaseDC      = user32.NewProc("ReleaseDC")
	pGetMetrics     = user32.NewProc("GetSystemMetrics")
	pCreateCompDC   = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompBmp  = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject   = gdi32.NewProc("SelectObject")
	pDeleteObject   = gdi32.NewProc("DeleteObject")
	pDeleteDC       = gdi32.NewProc("DeleteDC")
	pStretchBlt     = gdi32.NewProc("StretchBlt")
	pSetStretchMode = gdi32.NewProc("SetStretchBltMode")
	pGetDIBits      = gdi32.NewProc("GetDIBits")
)

type bitmapInfo struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	SizeImage     uint32
	XPels, YPels  int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type zone struct {
	avg      rgbF
	hue, sat float64
	vivid    float64
	value    float64
}

type grabber struct {
	cols, w, h int
	buf        []byte
}

func newGrabber(cols int) *grabber {
	w := max(cols*16, 32)
	return &grabber{cols: cols, w: w, h: 18, buf: make([]byte, w*18*4)}
}

func (g *grabber) sample(region [4]float64) ([]zone, error) {
	sw, _, _ := pGetMetrics.Call(0)
	sh, _, _ := pGetMetrics.Call(1)
	if sw == 0 || sh == 0 {
		return nil, errors.New("no screen")
	}
	screen, _, _ := pGetDC.Call(0)
	if screen == 0 {
		return nil, errors.New("screen not available")
	}
	defer pReleaseDC.Call(0, screen)
	mem, _, _ := pCreateCompDC.Call(screen)
	if mem == 0 {
		return nil, errors.New("capture failed")
	}
	defer pDeleteDC.Call(mem)
	bmp, _, _ := pCreateCompBmp.Call(screen, uintptr(g.w), uintptr(g.h))
	if bmp == 0 {
		return nil, errors.New("capture failed")
	}
	defer pDeleteObject.Call(bmp)
	old, _, _ := pSelectObject.Call(mem, bmp)
	defer pSelectObject.Call(mem, old)
	pSetStretchMode.Call(mem, 4)
	x0, y0 := int(region[0]*float64(sw)), int(region[1]*float64(sh))
	cw, ch := max(int((region[2]-region[0])*float64(sw)), 1), max(int((region[3]-region[1])*float64(sh)), 1)
	if r, _, _ := pStretchBlt.Call(mem, 0, 0, uintptr(g.w), uintptr(g.h), screen, uintptr(x0), uintptr(y0), uintptr(cw), uintptr(ch), 0x00CC0020); r == 0 {
		return nil, errors.New("capture failed")
	}
	bi := bitmapInfo{Size: uint32(unsafe.Sizeof(bitmapInfo{})), Width: int32(g.w), Height: -int32(g.h), Planes: 1, Bits: 32}
	if r, _, _ := pGetDIBits.Call(mem, bmp, 0, uintptr(g.h), uintptr(unsafe.Pointer(&g.buf[0])), uintptr(unsafe.Pointer(&bi)), 0); r == 0 {
		return nil, errors.New("capture failed")
	}
	out := make([]zone, g.cols)
	per := g.w / g.cols
	for c := 0; c < g.cols; c++ {
		var r, gr, b, n, wsum, hx, hy, ss, vs float64
		for y := 0; y < g.h; y++ {
			for x := c * per; x < (c+1)*per; x++ {
				i := (y*g.w + x) * 4
				pb, pg, pr := float64(g.buf[i])/255, float64(g.buf[i+1])/255, float64(g.buf[i+2])/255
				b, gr, r, n = b+pb, gr+pg, r+pr, n+1
				h, s, v := toHSV(rgbF{pr, pg, pb})
				vs += v
				if s > 0.25 && v > 0.2 {
					w := s * v
					a := h * math.Pi / 180
					hx, hy, ss, wsum = hx+w*math.Cos(a), hy+w*math.Sin(a), ss+w*s, wsum+w
				}
			}
		}
		z := zone{avg: rgbF{r / n, gr / n, b / n}, value: vs / n, vivid: wsum / n}
		if wsum > 0 {
			z.hue = math.Mod(math.Atan2(hy, hx)*180/math.Pi+360, 360)
			z.sat = ss / wsum
		}
		out[c] = z
	}
	return out, nil
}

type screenSource struct {
	g      *grabber
	cfg    Config
	col    []rgbF
	val    []float64
	primed bool
}

func newScreenSource(cfg Config, lights int) (source, error) {
	cols := 1
	if cfg.Split {
		cols = min(max(lights, 1), 8)
	}
	g := newGrabber(cols)
	if _, err := g.sample(cfg.Region); err != nil {
		return nil, err
	}
	return &screenSource{g: g, cfg: cfg, col: make([]rgbF, cols), val: make([]float64, cols)}, nil
}

func (s *screenSource) Next() ([]HSV, error) {
	zs, err := s.g.sample(s.cfg.Region)
	if err == nil {
		for i, z := range zs {
			var c rgbF
			if z.vivid > 0.04 {
				c = fromHSV(z.hue, clampF(z.sat*1.3, 0.6, 1), 1)
			} else {
				h, sat, _ := toHSV(z.avg)
				c = fromHSV(h, clampF(sat*1.5, 0, 1), 1)
			}
			v := math.Pow(clampF(z.value*1.7, 0, 1), 0.8)
			if !s.primed {
				s.col[i], s.val[i] = c, v
			} else {
				s.col[i], s.val[i] = s.col[i].mixTo(c, 0.4), s.val[i]+(v-s.val[i])*0.5
			}
		}
		s.primed = true
	}
	out := make([]HSV, len(s.col))
	for i, c := range s.col {
		h, sat, _ := toHSV(c)
		out[i] = HSV{int(h), pct(sat, 100), max(2, pct(s.val[i]*float64(s.cfg.MaxBright)/100, 100))}
	}
	return out, nil
}

func (s *screenSource) Close() {}
