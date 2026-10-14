package lightshow

import (
	"errors"
	"math"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32     = windows.NewLazySystemDLL("ole32.dll")
	pCoInit   = ole32.NewProc("CoInitializeEx")
	pCoCreate = ole32.NewProc("CoCreateInstance")
	pCoFree   = ole32.NewProc("CoTaskMemFree")
)

var (
	clsidEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidEnumerator   = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidAudioClient  = windows.GUID{Data1: 0x1CB9AD4C, Data2: 0xDBFA, Data3: 0x4C32, Data4: [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidCapture      = windows.GUID{Data1: 0xC8ADBD64, Data2: 0xE71E, Data3: 0x48A0, Data4: [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
)

func ptr(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

func vcall(obj uintptr, idx int, args ...uintptr) int32 {
	vt := *(*uintptr)(ptr(obj))
	fn := *(*uintptr)(ptr(vt + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return int32(r)
}

type loopback struct {
	mu   sync.Mutex
	ring []float64
	rate float64
	stop chan struct{}
	done chan struct{}
}

func startLoopback() (*loopback, error) {
	lb := &loopback{ring: make([]float64, fftSize*4), stop: make(chan struct{}), done: make(chan struct{})}
	ready := make(chan error, 1)
	go lb.run(ready)
	if err := <-ready; err != nil {
		<-lb.done
		return nil, err
	}
	return lb, nil
}

func (lb *loopback) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(lb.done)
	pCoInit.Call(0, 0)
	fail := func(msg string) { ready <- errors.New(msg) }
	var enum uintptr
	if r, _, _ := pCoCreate.Call(uintptr(unsafe.Pointer(&clsidEnumerator)), 0, 23, uintptr(unsafe.Pointer(&iidEnumerator)), uintptr(unsafe.Pointer(&enum))); int32(r) < 0 || enum == 0 {
		fail("audio system not available")
		return
	}
	defer vcall(enum, 2)
	var dev uintptr
	if vcall(enum, 4, 0, 0, uintptr(unsafe.Pointer(&dev))) < 0 || dev == 0 {
		fail("no audio output device")
		return
	}
	defer vcall(dev, 2)
	var ac uintptr
	if vcall(dev, 3, uintptr(unsafe.Pointer(&iidAudioClient)), 23, 0, uintptr(unsafe.Pointer(&ac))) < 0 || ac == 0 {
		fail("could not open the audio output")
		return
	}
	defer vcall(ac, 2)
	var wf uintptr
	if vcall(ac, 8, uintptr(unsafe.Pointer(&wf))) < 0 || wf == 0 {
		fail("could not read the audio format")
		return
	}
	defer pCoFree.Call(wf)
	hdr := unsafe.Slice((*byte)(ptr(wf)), 40)
	tag := uint16(hdr[0]) | uint16(hdr[1])<<8
	channels := int(uint16(hdr[2]) | uint16(hdr[3])<<8)
	rate := uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24
	bits := int(uint16(hdr[14]) | uint16(hdr[15])<<8)
	isFloat := tag == 3 || (tag == 0xFFFE && hdr[24] == 3)
	if channels < 1 || (!isFloat && bits != 16 && bits != 32) || (isFloat && bits != 32) {
		fail("unsupported audio format")
		return
	}
	if vcall(ac, 3, 0, 0x20000, 10000000, 0, wf, 0) < 0 {
		fail("could not capture the audio output")
		return
	}
	var cc uintptr
	if vcall(ac, 14, uintptr(unsafe.Pointer(&iidCapture)), uintptr(unsafe.Pointer(&cc))) < 0 || cc == 0 {
		fail("could not start audio capture")
		return
	}
	defer vcall(cc, 2)
	if vcall(ac, 10) < 0 {
		fail("could not start audio capture")
		return
	}
	defer vcall(ac, 11)
	lb.rate = float64(rate)
	ready <- nil
	frameBytes := channels * bits / 8
	for {
		select {
		case <-lb.stop:
			return
		case <-time.After(10 * time.Millisecond):
		}
		for {
			var n uint32
			if vcall(cc, 5, uintptr(unsafe.Pointer(&n))) < 0 || n == 0 {
				break
			}
			var data uintptr
			var frames, flags uint32
			var pos, qpc uint64
			if vcall(cc, 3, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&frames)), uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&pos)), uintptr(unsafe.Pointer(&qpc))) < 0 {
				break
			}
			mono := make([]float64, frames)
			if flags&2 == 0 && data != 0 {
				raw := unsafe.Slice((*byte)(ptr(data)), int(frames)*frameBytes)
				for i := range mono {
					var sum float64
					for c := 0; c < channels; c++ {
						o := (i*channels + c) * bits / 8
						switch {
						case isFloat:
							sum += float64(math.Float32frombits(uint32(raw[o]) | uint32(raw[o+1])<<8 | uint32(raw[o+2])<<16 | uint32(raw[o+3])<<24))
						case bits == 16:
							sum += float64(int16(uint16(raw[o])|uint16(raw[o+1])<<8)) / 32768
						default:
							sum += float64(int32(uint32(raw[o])|uint32(raw[o+1])<<8|uint32(raw[o+2])<<16|uint32(raw[o+3])<<24)) / 2147483648
						}
					}
					mono[i] = sum / float64(channels)
				}
			}
			vcall(cc, 4, uintptr(frames))
			lb.mu.Lock()
			if len(mono) >= len(lb.ring) {
				copy(lb.ring, mono[len(mono)-len(lb.ring):])
			} else {
				copy(lb.ring, lb.ring[len(mono):])
				copy(lb.ring[len(lb.ring)-len(mono):], mono)
			}
			lb.mu.Unlock()
		}
	}
}

func (lb *loopback) latest() []float64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return append([]float64(nil), lb.ring[len(lb.ring)-fftSize:]...)
}

func (lb *loopback) close() {
	close(lb.stop)
	<-lb.done
}

type soundSource struct {
	lb       *loopback
	cfg      Config
	lights   int
	peak     [3]float64
	avgBass  float64
	hueOff   float64
	level    float64
	lastBeat time.Time
}

func newSoundSource(cfg Config, lights int) (source, error) {
	lb, err := startLoopback()
	if err != nil {
		return nil, err
	}
	return &soundSource{lb: lb, cfg: cfg, lights: lights}, nil
}

func (s *soundSource) Close() { s.lb.close() }

func (s *soundSource) Next() ([]HSV, error) {
	b, m, h := bands(s.lb.latest(), s.lb.rate)
	v := [3]float64{b, m, h}
	var n [3]float64
	gain := float64(s.cfg.Gain) / 100
	for i := range v {
		s.peak[i] = math.Max(math.Max(v[i], s.peak[i]*0.99), 0.4)
		n[i] = clampF(v[i]/s.peak[i]*gain, 0, 1)
	}
	energy := clampF((n[0]*1.2+n[1]+n[2]*0.8)/3*1.6, 0, 1)
	beat := n[0] > 0.55 && n[0] > s.avgBass*1.45 && time.Since(s.lastBeat) > 220*time.Millisecond
	s.avgBass = s.avgBass*0.9 + n[0]*0.1
	if beat {
		s.lastBeat = time.Now()
		s.hueOff += 47
	}
	s.hueOff += 0.4
	target := 0.08 + 0.92*math.Pow(energy, 0.8)
	if beat {
		target = 1
	}
	k := 0.25
	if target > s.level {
		k = 0.75
	}
	s.level += (target - s.level) * k
	x := n[0] + n[1]*math.Cos(2.27) + n[2]*math.Cos(4.36)
	y := n[1]*math.Sin(2.27) + n[2]*math.Sin(4.36)
	base := math.Atan2(y, x) * 180 / math.Pi
	dom := clampF(math.Hypot(x, y)/math.Max(n[0]+n[1]+n[2], 0.001), 0, 1)
	sat := 0.65 + 0.35*dom
	out := make([]HSV, s.lights)
	for i := range out {
		hue := math.Mod(math.Mod(base+s.hueOff+float64(i)*35, 360)+360, 360)
		out[i] = HSV{int(hue), pct(sat, 100), max(2, pct(s.level*float64(s.cfg.MaxBright)/100, 100))}
	}
	return out, nil
}
