package machine

import (
	"github.com/wicanr2/ps-golem/internal/gpu"
	"slices"
	"testing"
)

// TestPGXPTransport 以手寫指令走 RTPS → SWC2 → LW → SW → GP0。
// BIOS 為未執行的測試緩衝，不取代任何需要原版素材的驗收。
func TestPGXPTransport(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "保留小數", true: "改字回退"}[changed], func(t *testing.T) {
			m, _ := New(make([]byte, biosSize))
			s := gpu.NewShadow(2)
			m.GPU.SetShadow(s)
			defer func() { m.GPU.SetShadow(nil); s.Close() }()
			if err := m.EnablePGXP(); err != nil {
				t.Fatal(err)
			}
			g := &m.CPU.GTE
			g.Write(32, 4096)
			g.Write(34, 4096)
			g.Write(36, 4096)
			g.Write(58, 100)
			g.Write(0, 32|32<<16)
			g.Write(1, 300)
			// GP0 繪圖範圍與平面三角形的命令字。
			for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x200000ff} {
				m.Write32(0x1f801810, w)
			}
			m.CPU.SetGPR(2, 0x80020000)
			m.CPU.SetGPR(4, 0x1f801810)
			program := []uint32{
				0x3c014000, 0x40816000, // LUI、MTC0：啟用 COP2
				0x4a080001,    // RTPS sf=1
				0xe84e0000,    // SWC2 r14,0(r2)
				0x8c430000, 0, // LW r3,0(r2)、延遲槽
			}
			if changed {
				program = append(program, 0x24630001)
			} // ADDIU 改寫來源字
			program = append(program, 0xac830000) // SW r3,0(r4)
			for i, w := range program {
				m.Write32(0x80010000+uint32(4*i), w)
			}
			m.CPU.SetPC(0x80010000)
			for range program {
				m.Step()
			}
			for _, w := range []uint32{30 | 10<<16, 10 | 30<<16} {
				m.Write32(0x1f801810, w)
			}
			hits, vertices := s.PrecisionStats()
			want := uint64(1)
			if changed {
				want = 0
			}
			if hits != want || vertices != 3 {
				t.Fatalf("精度命中=%d/%d，預期 %d/3", hits, vertices, want)
			}
			// 用無附件的同字命令作影子對照；有效小數必須真的改變光柵化。
			raw := gpu.NewShadow(2)
			defer raw.Close()
			word := uint32(10 | 10<<16)
			if changed {
				word++
			}
			for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x200000ff, word, 30 | 10<<16, 10 | 30<<16} {
				raw.GP0(w)
			}
			equal := slices.Equal(s.VRAMCopy(), raw.VRAMCopy())
			if equal != changed {
				t.Fatalf("小數光柵化比較 equal=%v，改字=%v", equal, changed)
			}
			rtps, rtpt := m.PGXPTransforms()
			if rtps != 1 || rtpt != 0 {
				t.Fatalf("RTPS=%d RTPT=%d", rtps, rtpt)
			}
			clone := m.Clone()
			if clone.precision != nil || clone.GPU.PrecisionSupported() {
				t.Fatal("Clone 保留呈現追蹤器")
			}
			if clone.Digest() != m.Digest() {
				t.Fatal("Clone 原生狀態不同")
			}
		})
	}
}

func TestPGXPRequiresHD(t *testing.T) {
	m, _ := New(make([]byte, biosSize))
	if m.EnablePGXP() == nil {
		t.Fatal("沒有影子仍啟用 PGXP")
	}
	s := gpu.NewShadow(0)
	defer s.Close()
	m.GPU.SetShadow(s)
	if m.EnablePGXP() == nil {
		t.Fatal("N=1 仍啟用 PGXP")
	}
}

func TestPGXPDMA(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "區塊", true: "鏈結串列"}[linked], func(t *testing.T) {
			m, _ := New(make([]byte, biosSize))
			s := gpu.NewShadow(1)
			m.GPU.SetShadow(s)
			defer func() { m.GPU.SetShadow(nil); s.Close() }()
			if err := m.EnablePGXP(); err != nil {
				t.Fatal(err)
			}
			for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x200000ff} {
				m.Write32(0x1f801810, w)
			}
			addr := uint32(0x1000)
			start := addr
			if linked {
				m.Write32(addr, 3<<24|0x800000)
				addr += 4
			}
			raw := uint32(10 | 10<<16)
			m.precision.Projected(10.75, 10.75, raw, 0, true)
			m.precision.StoreCOP2(14, addr, raw)
			for i, w := range []uint32{raw, 30 | 10<<16, 10 | 30<<16} {
				m.Write32(addr+uint32(4*i), w)
			}
			m.Write32(0x1f8010f0, 1<<11)
			m.Write32(0x1f8010a0, start)
			m.Write32(0x1f8010a4, 3)
			chcr := uint32(1<<24 | 1)
			if linked {
				chcr |= 2 << 9
			}
			m.Write32(0x1f8010a8, chcr)
			h, v := s.PrecisionStats()
			if h != 1 || v != 3 {
				t.Fatalf("DMA 精度命中=%d/%d", h, v)
			}
		})
	}
}
