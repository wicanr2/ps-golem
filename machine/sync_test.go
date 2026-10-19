package machine

import (
	"github.com/wicanr2/ps-golem/internal/cpu"
	"reflect"
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
	"spu/test/test.exe", "spu/memory-transfer/memory-transfer.exe", "spu/stereo/stereo.exe",
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

func TestRAMCountdownMachineBoundaries(t *testing.T) {
	bios := loadBIOS(t)
	iword := func(op, rs, rt uint32, imm int16) uint32 { return op<<26 | rs<<21 | rt<<16 | uint32(uint16(imm)) }
	for _, budget := range []uint64{1, 25, 26, 27, 52, 1000, 10000, 1000000} {
		for _, exact := range []bool{false, true} {
			m, err := New(bios)
			if err != nil {
				t.Fatal(err)
			}
			start := uint32(0x80000200)
			first := iword(0x23, 29, 5, 20)
			head := []uint32{first, 0, iword(9, 5, 5, -1), iword(0x2b, 29, 5, 20), first, 0, iword(5, 5, 0, 9), 0}
			tail := []uint32{iword(15, 0, 5, -32768), iword(0x23, 5, 5, 0x1800), 0, 5<<21 | 6<<16 | 5<<11 | 0x2a, iword(5, 5, 0, -21), 0}
			for i, v := range head {
				m.Write32(start+uint32(i)*4, v)
			}
			for i, v := range tail {
				m.Write32(start+64+uint32(i)*4, v)
			}
			m.Write32(0x1000, 1000000)
			m.Write32(0x1800, 7)
			m.biu = 0x800
			m.CPU.SetPC(start)
			m.CPU.SetGPR(29, 0x80000fec)
			m.CPU.SetGPR(6, 9)
			m.ExactTiming = exact
			ref := m.Clone()
			reached := m.RunUntilField(1, budget)
			end := ref.cycles + budget
			for ref.field < 1 && ref.cycles < end {
				ref.Step()
			}
			ref.sync()
			if reached != (ref.field >= 1) || m.Digest() != ref.Digest() || !reflect.DeepEqual(m, ref) {
				t.Fatalf("budget=%d exact=%v complete machine differs; digest equal=%v cycles=%d/%d", budget, exact, m.Digest() == ref.Digest(), m.cycles, ref.cycles)
			}
		}
	}
}

func TestRAMCountdownScheduledIRQ(t *testing.T) {
	bios := loadBIOS(t)
	m, err := New(bios)
	if err != nil {
		t.Fatal(err)
	}
	iword := func(op, rs, rt uint32, imm int16) uint32 { return op<<26 | rs<<21 | rt<<16 | uint32(uint16(imm)) }
	start := uint32(0x80000200)
	first := iword(0x23, 29, 5, 20)
	head := []uint32{first, 0, iword(9, 5, 5, -1), iword(0x2b, 29, 5, 20), first, 0, iword(5, 5, 0, 9), 0}
	tail := []uint32{iword(15, 0, 5, -32768), iword(0x23, 5, 5, 0x1800), 0, 5<<21 | 6<<16 | 5<<11 | 0x2a, iword(5, 5, 0, -21), 0}
	for i, w := range head {
		m.Write32(start+uint32(i)*4, w)
	}
	for i, w := range tail {
		m.Write32(start+64+uint32(i)*4, w)
	}
	// ORI t0,zero,0x401; MTC0 t0,SR; NOP。開 VBlank 硬體中斷。
	m.Write32(start-12, iword(13, 0, 8, 0x401))
	m.Write32(start-8, 0x10<<26|4<<21|8<<16|12<<11)
	m.Write32(start-4, 0)
	m.Write32(0x1000, 1000000)
	m.Write32(0x1800, 7)
	m.Write32(0x1f801074, 1)
	m.biu = 0x800
	m.CPU.SetPC(start - 12)
	m.CPU.SetGPR(29, 0x80000fec)
	m.CPU.SetGPR(6, 9)
	ref := m.Clone()
	reached := m.RunUntilField(2, 1300000)
	for ref.field < 2 && ref.cycles < 1300000 {
		ref.Step()
	}
	ref.sync()
	if m.io.iStat&1 == 0 || m.CPU.COP0(cpu.CP0EPC) == 0 {
		t.Fatal("VBlank IRQ not delivered")
	}
	if reached != (ref.field >= 2) || m.Digest() != ref.Digest() || !reflect.DeepEqual(m, ref) {
		t.Fatal("scheduled IRQ full machine differs")
	}
}
