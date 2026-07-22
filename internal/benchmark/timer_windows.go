package benchmark

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	qpcCounter    = kernel32.NewProc("QueryPerformanceCounter")
	qpcFreq       = kernel32.NewProc("QueryPerformanceFrequency")
	qpcTicksPerNs = func() float64 {
		var freq int64
		_, _, _ = qpcFreq.Call(uintptr(unsafe.Pointer(&freq)))
		return float64(freq) / 1e9
	}()
)

func highResNow() int64 {
	var count int64
	_, _, _ = qpcCounter.Call(uintptr(unsafe.Pointer(&count)))
	return count
}

func highResElapsed(from, to int64) time.Duration {
	return time.Duration(float64(to-from) / qpcTicksPerNs)
}
