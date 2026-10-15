package machine

import (
	"strings"
	"sync"
	"testing"
)

// 延後同步的等價性（docs/spec/008-cycle-model.md §2.5）：同一個起點分別以「逐條指令同步」與
// 「延後同步」執行，結束時機器狀態必須逐位元相同。
var syncEquivTests = []string{
	"timers/timers.exe", "timer-dump/timer-dump.exe", "cpu/access-time/access-time.exe",
	"dma/chopping/chopping.exe", "dma/chain-looping/chain-looping.exe",
	"gpu/rectangles/rectangles.exe", "gpu/animated-triangle/animated-triangle.exe",
	"mdec/movie/movie-15bit.exe", "input/pad/pad.exe", "cdrom/timing/timing.exe",
}

func TestDeferredSyncEquivalent(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	boot := bootSnapshot(t, bios)

	var wg sync.WaitGroup
	for _, path := range syncEquivTests {
		exe := files[path]
		if exe == nil {
			t.Errorf("tests.zip 裡缺 %s", path)
			continue
		}
		wg.Add(1)
		go func(path string, exe []byte) {
			defer wg.Done()
			var got [2]string
			for i, exact := range []bool{true, false} {
				m := boot.Clone()
				m.ExactTiming = exact
				if err := m.SideloadEXE(exe); err != nil {
					t.Errorf("%s：%v", path, err)
					return
				}
				m.Run(m3Cycles)
				got[i] = m.Digest()
			}
			if got[0] != got[1] {
				t.Errorf("%s：狀態不同\n逐條：%s\n延後：%s", strings.TrimSuffix(path, ".exe"), got[0], got[1])
			}
		}(path, exe)
	}
	wg.Wait()
}
