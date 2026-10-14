package flyout

import (
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32        = windows.NewLazySystemDLL("comdlg32.dll")
	pGetOpenFileNam = comdlg32.NewProc("GetOpenFileNameW")
	modal           atomic.Bool
)

type openFileName struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	PvReserved    uintptr
	DwReserved    uint32
	FlagsEx       uint32
}

func Modal() bool { return modal.Load() }

func PickImage(owner uintptr) (string, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	modal.Store(true)
	defer modal.Store(false)
	filter, _ := windows.UTF16FromString("Images\x00*.png;*.jpg;*.jpeg;*.gif\x00All files\x00*.*\x00")
	filter = append(filter, 0)
	title, _ := windows.UTF16PtrFromString("Choose an icon image")
	buf := make([]uint16, 1024)
	ofn := openFileName{Owner: owner, Filter: &filter[0], FilterIndex: 1, File: &buf[0], MaxFile: uint32(len(buf)), Title: title, Flags: 0x1000 | 0x800 | 0x8 | 0x80000}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	r, _, _ := pGetOpenFileNam.Call(uintptr(unsafe.Pointer(&ofn)))
	if owner != 0 {
		pSetForeground.Call(owner)
	}
	if r == 0 {
		return "", false
	}
	return windows.UTF16ToString(buf), true
}
