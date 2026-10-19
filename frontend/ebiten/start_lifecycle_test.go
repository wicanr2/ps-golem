package ebiten

import (
	"testing"
	"time"

	"github.com/wicanr2/ps-golem/machine"
)

// 只驗證生命週期，零值機器不會執行指令，不代替 BIOS 整合測試。
func TestHaltBeforeFirstUpdate(t *testing.T) {
	m := &machine.Machine{}
	e := newEmulator(m, Config{}, nil, nil)
	done := make(chan struct{})
	go func() { e.halt(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("closing before startup deadlocked")
	}
	e.start()
	e.halt()
	if m.Cycles() != 0 || e.frameNumber() != 0 {
		t.Fatal("closed emulator executed work")
	}
}
