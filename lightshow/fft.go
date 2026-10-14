package lightshow

import (
	"math"
	"math/cmplx"
)

const fftSize = 1024

var hann = func() []float64 {
	w := make([]float64, fftSize)
	for i := range w {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))
	}
	return w
}()

func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(l)))
		for i := 0; i < n; i += l {
			wn := complex(1, 0)
			for k := 0; k < l/2; k++ {
				u, v := a[i+k], a[i+k+l/2]*wn
				a[i+k], a[i+k+l/2] = u+v, u-v
				wn *= w
			}
		}
	}
}

func bands(samples []float64, rate float64) (bass, mid, high float64) {
	buf := make([]complex128, fftSize)
	for i := 0; i < fftSize && i < len(samples); i++ {
		buf[i] = complex(samples[i]*hann[i], 0)
	}
	fft(buf)
	hz := rate / fftSize
	var n [3]float64
	for k := 1; k < fftSize/2; k++ {
		f := float64(k) * hz
		m := cmplx.Abs(buf[k])
		switch {
		case f < 40:
		case f < 160:
			bass, n[0] = bass+m, n[0]+1
		case f < 2000:
			mid, n[1] = mid+m, n[1]+1
		case f < 10000:
			high, n[2] = high+m, n[2]+1
		}
	}
	if n[0] > 0 {
		bass /= n[0]
	}
	if n[1] > 0 {
		mid /= n[1]
	}
	if n[2] > 0 {
		high /= n[2]
	}
	return
}
