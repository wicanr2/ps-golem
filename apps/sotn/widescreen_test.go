package sotn

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/ps-golem/internal/cpu"
	"github.com/wicanr2/ps-golem/machine"
)

func testWideMachine(t *testing.T) *machine.Machine {
	t.Helper()
	m, err := machine.New(make([]byte, 512<<10))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// 合成函式只供驗證簽章機制，不冒充原版資料或生產簽章。
func TestWidescreenSignature(t *testing.T) {
	m := testWideMachine(t)
	for _, p := range widescreenWords {
		m.Write32(p.addr, p.original)
	}
	original := m.ReadRAM(tilemapStart, tilemapSize)
	w := &Widescreen{}
	if err := w.Check(m); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, m.ReadRAM(tilemapStart, tilemapSize)) || w.applied {
		t.Fatal("不符生產簽章仍改寫 RAM")
	}
	h := sha256.Sum256(original)
	if err := w.check(m, h); err != nil {
		t.Fatal(err)
	}
	for _, p := range widescreenWords {
		if m.Read32(p.addr) != p.patched {
			t.Fatalf("%08x 未套用", p.addr)
		}
	}
	if err := w.check(m, h); err != nil || w.Applications != 1 {
		t.Fatal("正規化後重複套用或失配", err)
	}
	// 模擬 DRA 重載，下一個 field 必須重新套用。
	for _, p := range widescreenWords {
		m.Write32(p.addr, p.original)
	}
	if err := w.check(m, h); err != nil || w.Applications != 2 {
		t.Fatal("重載後未套用", err)
	}
	m.Write32(tilemapStart, 0x1234)
	before := m.ReadRAM(tilemapStart, tilemapSize)
	if err := w.check(m, h); err == nil {
		t.Fatal("已套用後失配沒有回報")
	}
	if !bytes.Equal(before, m.ReadRAM(tilemapStart, tilemapSize)) {
		t.Fatal("失配仍寫入")
	}
	w.Disable(m)
	for _, p := range widescreenWords {
		if m.Read32(p.addr) != p.original {
			t.Fatalf("%08x 未還原", p.addr)
		}
	}
	// 解除後原 BREAK 必須回到 Bp。
	m.Write32(0x800eccf0, widescreenWords[6].patched)
	m.CPU.SetPC(0x800eccf0)
	m.Step()
	if (m.CPU.COP0(cpu.CP0CAUSE)>>2)&31 != cpu.ExcBP {
		t.Fatal("解除後仍有陷阱")
	}
}

func TestWidescreenBreakEXE(t *testing.T) {
	for _, mode := range []string{"handled", "absent", "address", "code", "delay", "load-delay"} {
		t.Run(mode, func(t *testing.T) {
			m := testWideMachine(t)
			pc := uint32(0x800eccf0)
			code := uint32(widescreenBreakCode)
			if mode == "address" {
				pc += 4
			}
			if mode == "code" {
				code++
			}
			start := pc
			words := []uint32{code<<6 | 0x0d, 0}
			if mode == "delay" {
				start -= 4
				words = append([]uint32{0x10000001}, words...)
			}
			if mode == "load-delay" {
				start -= 4
				words = append([]uint32{0x8e030004}, words...)
			}
			exe := make([]byte, 0x800+4*len(words))
			copy(exe, "PS-X EXE")
			binary.LittleEndian.PutUint32(exe[0x10:], start)
			binary.LittleEndian.PutUint32(exe[0x18:], start)
			binary.LittleEndian.PutUint32(exe[0x1c:], uint32(4*len(words)))
			for i, w := range words {
				binary.LittleEndian.PutUint32(exe[0x800+4*i:], w)
			}
			if err := m.SideloadEXE(exe); err != nil {
				t.Fatal(err)
			}
			m.CPU.SetGPR(3, 32)
			m.CPU.SetGPR(16, 0x80010000)
			m.Write32(0x80010004, 999)
			if mode != "absent" {
				m.CPU.SetBreakHandler(widescreenTrap)
			}
			if mode == "delay" || mode == "load-delay" {
				m.Step()
			}
			m.Step()
			if mode == "handled" || mode == "load-delay" {
				if m.CPU.PC() != pc+4 || m.Read16(0x80010000) != 0xfff0 {
					t.Fatalf("陷阱結果 PC=%08x x=%04x", m.CPU.PC(), m.Read16(0x80010000))
				}
				if mode == "load-delay" && m.CPU.GPR(3) != 999 {
					t.Fatal("延遲載入未提交")
				}
			} else {
				if (m.CPU.COP0(cpu.CP0CAUSE)>>2)&31 != cpu.ExcBP || m.Read16(0x80010000) != 0 {
					t.Fatal("未匹配 BREAK 沒有保留 Bp")
				}
			}
		})
	}
}
