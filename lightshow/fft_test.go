package lightshow

import (
	"math"
	"testing"
)

func TestBandsSeparate(t *testing.T) {
	const rate = 48000.0
	tone := func(f float64) []float64 {
		s := make([]float64, fftSize)
		for i := range s {
			s[i] = math.Sin(2 * math.Pi * f * float64(i) / rate)
		}
		return s
	}
	b, m, h := bands(tone(100), rate)
	if !(b > m*4 && b > h*4) {
		t.Fatalf("100 Hz should land in bass: %v %v %v", b, m, h)
	}
	b, m, h = bands(tone(1000), rate)
	if !(m > b*4 && m > h*4) {
		t.Fatalf("1 kHz should land in mid: %v %v %v", b, m, h)
	}
	b, m, h = bands(tone(5000), rate)
	if !(h > b*4 && h > m*4) {
		t.Fatalf("5 kHz should land in high: %v %v %v", b, m, h)
	}
}

func TestLoopbackOpens(t *testing.T) {
	lb, err := startLoopback()
	if err != nil {
		t.Skip(err)
	}
	defer lb.close()
	if lb.rate < 8000 {
		t.Fatalf("odd sample rate %v", lb.rate)
	}
	if len(lb.latest()) != fftSize {
		t.Fatal("bad window")
	}
}

func TestScreenSample(t *testing.T) {
	g := newGrabber(3)
	px, err := g.sample([4]float64{0, 0, 1, 1})
	if err != nil {
		t.Skip(err)
	}
	if len(px) != 3 {
		t.Fatalf("want 3 columns got %d", len(px))
	}
}
