package ebiten

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/ps-golem/internal/sio"
	"github.com/wicanr2/ps-golem/machine"
)

func TestControllerResumeWaitsForRelease(t *testing.T) {
	g := &Game{controller: DefaultGamepadSettings()}
	g.openControllerPanel()
	g.closeControllerPanel()
	if g.filterGameplayInput(sio.BtnCross) != 0 || !g.waitInputRelease {
		t.Fatal("held mapping input leaked after close")
	}
	if g.filterGameplayInput(0) != 0 || g.waitInputRelease {
		t.Fatal("neutral did not rearm input")
	}
	if g.filterGameplayInput(sio.BtnCross) != sio.BtnCross {
		t.Fatal("new input lost after release")
	}
}

func TestControllerBIOSPauseResume(t *testing.T) {
	biosPath := os.Getenv("PSGOLEM_BIOS")
	if biosPath == "" {
		t.Skip("需要實際 BIOS；不使用代製 BIOS")
	}
	b, err := os.ReadFile(biosPath)
	if os.IsNotExist(err) {
		t.Skip("缺少實際 BIOS")
	}
	if err != nil {
		t.Fatal(err)
	}
	m, err := machine.New(b)
	if err != nil {
		t.Fatal(err)
	}
	card := &cardSaver{path: filepath.Join(t.TempDir(), "card.mcd")}
	e := newEmulator(m, Config{}, nil, card)
	e.start()
	defer e.halt()
	wait := func(pred func(uint64) bool) uint64 {
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			n := e.frameNumber()
			if pred(n) {
				return n
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("BIOS 前端排程未達預期狀態")
		return 0
	}
	wait(func(n uint64) bool { return n > 0 })
	e.paused.Store(true)
	// 容許正在執行的 field 完成，之後發布計數需停止。
	wait(func(_ uint64) bool { return e.parked.Load() })
	n := e.frameNumber()
	time.Sleep(100 * time.Millisecond)
	after := e.frameNumber()
	if n != after {
		t.Fatalf("paused frame advanced: %d -> %d", n, after)
	}
	e.paused.Store(false)
	wait(func(next uint64) bool { return next > after })
	t.Log("real BIOS: pause at field boundary, publication stops, resume continues")
}
