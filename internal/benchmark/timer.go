//go:build !windows

package benchmark

import "time"

func highResNow() int64 {
	return time.Now().UnixNano()
}

func highResElapsed(from, to int64) time.Duration {
	return time.Duration(to - from)
}
