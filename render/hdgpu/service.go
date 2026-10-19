package hdgpu

import (
	"sync"
	"sync/atomic"
	"time"
)

type gpuCall struct {
	run  func()
	done chan struct{}
	end  bool
}

var calls = make(chan gpuCall, 1)
var activeDevices atomic.Int32
var stopped = make(chan struct{})
var stopOnce sync.Once

// Cancel 在圖形主迴圈結束後釋放等待中的呈現要求，包含裝置錯誤的收尾。
// 不再執行回讀；這些資料只屬於正在結束的HD影子。
func Cancel() { stopOnce.Do(func() { close(stopped) }) }

// Service 必須由每次 Game.Update 呼叫。回讀在圖形frame內執行，避免
// Ebitengine v2.9.9 internal/ui/ui.go:readPixels 的跨frame等待。
// 一個Flush結束本次服務；25ms預算讓高負載時仍能處理視窗事件。
// wait只在模擬工作仍執行時使用，暫停及CPU後端不等待空佇列。
func Service(wait bool) {
	if activeDevices.Load() == 0 {
		return
	}
	var c gpuCall
	timer := time.NewTimer(25 * time.Millisecond)
	defer timer.Stop()
	if wait {
		select {
		case c = <-calls:
		case <-stopped:
			return
		case <-timer.C:
			return
		}
	} else {
		select {
		case c = <-calls:
		default:
			return
		}
	}
	for {
		c.run()
		close(c.done)
		if c.end {
			return
		}
		select {
		case c = <-calls:
		case <-stopped:
			return
		case <-timer.C:
			return
		}
	}
}

func (d *Device) call(end bool, fn func()) {
	done := make(chan struct{})
	select {
	case calls <- gpuCall{run: fn, done: done, end: end}:
	case <-stopped:
		return
	}
	select {
	case <-done:
	case <-stopped:
	}
}
