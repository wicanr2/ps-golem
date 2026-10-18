package sotn

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
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
	// 兩個新增陷阱也必須獨立正規化，不接受相同指令的錯誤 code。
	for _, p := range widescreenWords[7:] {
		m.Write32(p.addr, p.patched^0x40)
		before := m.ReadRAM(tilemapStart, tilemapSize)
		if err := w.check(m, h); err != nil || !bytes.Equal(before, m.ReadRAM(tilemapStart, tilemapSize)) {
			t.Fatalf("%08x 未拒絕未知改字或仍寫入", p.addr)
		}
		m.Write32(p.addr, p.original)
		if err := w.check(m, h); err != nil {
			t.Fatalf("%08x 原字與改字混用失配：%v", p.addr, err)
		}
	}
	m.Write32(tilemapStart, 0x1234)
	before := m.ReadRAM(tilemapStart, tilemapSize)
	if err := w.check(m, h); err != nil {
		t.Fatal("已套用後失配不應停止模擬", err)
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

func TestWidescreenMismatchWarning(t *testing.T) {
	m := testWideMachine(t)
	for _, p := range widescreenWords {
		m.Write32(p.addr, p.original)
	}
	h := sha256.Sum256(m.ReadRAM(tilemapStart, tilemapSize))
	w := &Widescreen{}
	if err := w.check(m, h); err != nil {
		t.Fatal(err)
	}
	m.Write32(tilemapStart, 123)
	before := m.ReadRAM(tilemapStart, tilemapSize)
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = out
	defer func() { os.Stderr = stderr; r.Close(); out.Close() }()
	for i := 0; i < 3; i++ {
		if err := w.check(m, h); err != nil {
			t.Fatal(err)
		}
	}
	out.Close()
	os.Stderr = stderr
	log, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(log), "簽章不符") != 1 {
		t.Fatalf("警告次數：%s", log)
	}
	if !bytes.Equal(before, m.ReadRAM(tilemapStart, tilemapSize)) {
		t.Fatal("失配改寫 RAM")
	}
	// 失配後原本三處陷阱仍能執行，沒有被卸下。
	for _, p := range widescreenWords[6:] {
		m.CPU.SetPC(p.addr)
		m.CPU.SetGPR(16, 0x80010000)
		m.Step()
		if m.CPU.PC() != p.addr+4 {
			t.Fatalf("%08x 陷阱被卸下", p.addr)
		}
	}
}

func TestWidescreenRowLoadDelay(t *testing.T) {
	for _, trap := range []struct{ pc, code uint32 }{
		{0x800ece80, widescreenForegroundCode}, {0x800ed260, widescreenBackgroundCode},
	} {
		// 載入目標、兩個來源、無關暫存器，均與原 ADDU 的延遲語意比較。
		for _, reg := range []uint32{2, 15, 12, 8} {
			t.Run(fmt.Sprintf("%08x/r%d", trap.pc, reg), func(t *testing.T) {
				m := testWideMachine(t)
				m.Write32(trap.pc-4, 0x8e000004|reg<<16) // lw reg,4(s0)
				m.Write32(trap.pc, trap.code<<6|0x0d)
				m.Write32(trap.pc+4, 0)
				m.Write32(0x80010004, 999)
				m.CPU.SetPC(trap.pc - 4)
				m.CPU.SetGPR(16, 0x80010000)
				m.CPU.SetGPR(15, 5)
				m.CPU.SetGPR(12, 0x120)
				m.CPU.SetBreakHandler(widescreenTrap)
				m.Step()
				m.Step()
				m.Step()
				if m.CPU.GPR(2) != 0x125 {
					t.Fatalf("v0=%x", m.CPU.GPR(2))
				}
				if reg != 2 && m.CPU.GPR(int(reg)) != 999 {
					t.Fatal("無關的延遲載入未提交")
				}
			})
		}
	}
}

func TestWidescreenRowBreakEXE(t *testing.T) {
	for _, trap := range []struct{ pc, code uint32 }{
		{0x800ece80, widescreenForegroundCode},
		{0x800ed260, widescreenBackgroundCode},
	} {
		for _, tc := range []struct {
			name          string
			tx, row, want uint32
		}{
			{"left-three", 0xfffffffd, 0x120, 0x120},
			{"left-one", 0xffffffff, 0x120, 0x120},
			{"signed-min", 0x80000000, 0x120, 0x120},
			{"zero", 0, 0x120, 0x120},
			{"positive", 5, 0x120, 0x125},
			{"unsigned-wrap", 5, 0xfffffffe, 3},
		} {
			t.Run(fmt.Sprintf("%08x/%s", trap.pc, tc.name), func(t *testing.T) {
				m := testWideMachine(t)
				exe := make([]byte, 0x808)
				copy(exe, "PS-X EXE")
				binary.LittleEndian.PutUint32(exe[0x10:], trap.pc)
				binary.LittleEndian.PutUint32(exe[0x18:], trap.pc)
				binary.LittleEndian.PutUint32(exe[0x1c:], 8)
				binary.LittleEndian.PutUint32(exe[0x800:], trap.code<<6|0x0d)
				if err := m.SideloadEXE(exe); err != nil {
					t.Fatal(err)
				}
				m.CPU.SetGPR(2, 0xdeadbeef)
				m.CPU.SetGPR(15, tc.tx)
				m.CPU.SetGPR(12, tc.row)
				m.CPU.SetBreakHandler(widescreenTrap)
				m.Step()
				if m.CPU.PC() != trap.pc+4 || m.CPU.GPR(2) != tc.want || m.CPU.GPR(15) != tc.tx || m.CPU.GPR(12) != tc.row {
					t.Fatalf("PC=%08x v0=%08x，應為 %08x；來源暫存器須不變", m.CPU.PC(), m.CPU.GPR(2), tc.want)
				}
			})
		}
		for _, mode := range []string{"absent", "address", "other-row-code", "store-code", "delay"} {
			t.Run(fmt.Sprintf("%08x/%s", trap.pc, mode), func(t *testing.T) {
				m := testWideMachine(t)
				pc, code := trap.pc, trap.code
				switch mode {
				case "address":
					pc += 4
				case "other-row-code":
					code = widescreenForegroundCode + widescreenBackgroundCode - code
				case "store-code":
					code = widescreenBreakCode
				}
				m.Write32(pc, code<<6|0x0d)
				m.CPU.SetPC(pc)
				m.CPU.SetGPR(2, 0xdeadbeef)
				if mode != "absent" {
					m.CPU.SetBreakHandler(widescreenTrap)
				}
				if mode == "delay" {
					m.Write32(pc-4, 0x10000001)
					m.CPU.SetPC(pc - 4)
					m.Step()
				}
				m.Step()
				if (m.CPU.COP0(cpu.CP0CAUSE)>>2)&31 != cpu.ExcBP || m.CPU.GPR(2) != 0xdeadbeef {
					t.Fatal("未匹配 BREAK 沒有保留 Bp 或改寫 v0")
				}
			})
		}
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
