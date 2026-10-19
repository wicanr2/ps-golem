package ebiten

import (
	"image"
	"runtime"
	"sync"
	"testing"
)

// 慢速上傳仍持有前景；模擬端可畫後備，但交換後才能再次重用。
func TestFrameUploadOwnsPixels(t *testing.T) {
	e := &emulator{front: image.NewRGBA(image.Rect(0, 0, 64, 64)), back: image.NewRGBA(image.Rect(0, 0, 64, 64))}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 1; n <= 200; n++ {
			for i := range e.back.Pix {
				e.back.Pix[i] = byte(n)
			}
			e.mu.Lock()
			e.front, e.back = e.back, e.front
			e.frameNo++
			e.mu.Unlock()
			runtime.Gosched()
		}
	}()
	for n := 0; n < 200; n++ {
		e.useFrame(func(img *image.RGBA, _ uint64) {
			value := img.Pix[0]
			runtime.Gosched()
			for _, v := range img.Pix {
				if v != value {
					t.Error("front buffer changed during upload")
					break
				}
			}
		})
	}
	wg.Wait()
}
