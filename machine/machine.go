// Package machine 把 CPU、記憶體與週邊組成一台 PlayStation。
//
// 規格：docs/spec/006-minimal-machine.md。
package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/ps-golem/internal/cdrom"
	"github.com/wicanr2/ps-golem/internal/cpu"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/internal/mdec"
	"github.com/wicanr2/ps-golem/internal/sio"
)

const (
	ramSize        = 2 << 20
	biosSize       = 512 << 10
	scratchpadSize = 1 << 10
)

// 影像時鐘 = CPU 時鐘 × 11/7；NTSC 一個 field 263 線 × 3413 影像週期（006 §3）。
const (
	videoNum      = 11
	videoDen      = 7
	videoPerLine  = 3413
	linesPerField = 263
	videoPerField = videoPerLine * linesPerField
)

// Machine 是一台 PlayStation。時間以 CPU 週期計。
type Machine struct {
	CPU *cpu.CPU
	GPU *gpu.GPU
	CD  *cdrom.CDROM
	SIO *sio.SIO
	// MDEC 是動畫解碼器（011）。
	MDEC *mdec.MDEC
	// Pad1 是插在 1P 的數位手把；改 Buttons 就是按鍵。
	Pad1 *sio.Gamepad
	// Card1 是插在 1P 的記憶卡。
	Card1 *sio.Memcard

	ram        [ramSize]byte
	bios       [biosSize]byte
	scratchpad [scratchpadSize]byte
	biu        uint32

	io ioState

	cycles uint64

	// 影像時鐘累加器：每個 CPU 週期加 videoNum，滿 videoDen 進一個影像週期。
	videoAcc uint64
	dotAcc   uint64
	field    uint64
	gpuIRQ   bool
	cdIRQ    bool
	sioIRQ   bool

	tty []byte
}

// New 建立一台機器並載入 BIOS（512 KB）。
func New(bios []byte) (*Machine, error) {
	if len(bios) != biosSize {
		return nil, fmt.Errorf("BIOS 大小 %d，應為 %d", len(bios), biosSize)
	}
	m := &Machine{}
	copy(m.bios[:], bios)
	m.io.reset()
	m.GPU = gpu.New()
	m.CD = cdrom.New()
	m.SIO = sio.New()
	m.Pad1 = &sio.Gamepad{}
	m.SIO.Pads[0] = m.Pad1
	m.MDEC = mdec.New()
	m.Card1 = sio.NewMemcard()
	m.SIO.Cards[0] = m.Card1
	m.CPU = cpu.New(m)
	return m, nil
}

// InsertDisc 放入光碟（009 §5）。
func (m *Machine) InsertDisc(d *disc.Disc) { m.CD.Insert(d) }

// Clone 深拷貝整台機器（即時存檔的雛形；測試用來共用開機進 shell 的狀態）。
func (m *Machine) Clone() *Machine {
	n := *m
	n.tty = append([]byte(nil), m.tty...)
	g := *m.GPU
	n.GPU = &g
	c := *m.CPU
	n.CPU = &c
	cd := m.CD.Clone()
	n.CD = cd
	pad := *m.Pad1
	n.Pad1 = &pad
	sioCopy := *m.SIO
	n.SIO = &sioCopy
	n.SIO.Pads[0] = n.Pad1
	md := *m.MDEC
	n.MDEC = &md
	card := *m.Card1
	n.Card1 = &card
	n.SIO.Cards[0] = n.Card1
	n.CPU.Rebind(&n)
	return &n
}

// Cycles 回傳開機以來的 CPU 週期數。
func (m *Machine) Cycles() uint64 { return m.cycles }

// VRAM 回傳 GPU 的 VRAM（1024×512，觀測用，勿改）。
func (m *Machine) VRAM() []uint16 { return m.GPU.VRAM[:] }

// Field 回傳開機以來經過的 VBlank 次數。
func (m *Machine) Field() uint64 { return m.field }

// TTY 回傳目前攔到的 TTY 輸出。
func (m *Machine) TTY() string { return string(m.tty) }

// Step 執行一條指令並推進它花費的週期（008）。
func (m *Machine) Step() {
	m.ttyHook()
	m.tick(uint64(m.CPU.Step()))
}

// Run 執行 n 個週期。
func (m *Machine) Run(n uint64) {
	end := m.cycles + n
	for m.cycles < end {
		m.Step()
	}
}

// RunUntilField 執行到經過的 VBlank 次數達到 f，最多 maxCycles 個週期；回傳是否達到。
func (m *Machine) RunUntilField(f uint64, maxCycles uint64) bool {
	end := m.cycles + maxCycles
	for m.field < f {
		if m.cycles >= end {
			return false
		}
		m.Step()
	}
	return true
}

// RunUntilPC 執行到 PC 的實體位址等於 pc 為止（三個區段都算），最多 maxCycles 個週期。
func (m *Machine) RunUntilPC(pc uint32, maxCycles uint64) error {
	want := physical(pc)
	end := m.cycles + maxCycles
	for m.cycles < end {
		if physical(m.CPU.PC()) == want {
			return nil
		}
		m.Step()
	}
	return fmt.Errorf("跑了 %d 個週期仍未到達 PC %08x（目前 %08x）", maxCycles, pc, m.CPU.PC())
}

// ttyHook 攔 putchar：A(3Ch)、B(3Dh)（006 §6）。
func (m *Machine) ttyHook() {
	p := physical(m.CPU.PC())
	if p != 0xA0 && p != 0xB0 {
		return
	}
	fn := m.CPU.GPR(9)
	if (p == 0xA0 && fn == 0x3C) || (p == 0xB0 && fn == 0x3D) {
		m.tty = append(m.tty, byte(m.CPU.GPR(4)))
	}
}

func (m *Machine) tick(n uint64) {
	m.cycles += n
	var dots, lines uint64
	m.videoAcc += n * videoNum
	if m.videoAcc >= videoDen {
		v := m.videoAcc / videoDen
		m.videoAcc -= v * videoDen
		vb, l := m.GPU.TickVideo(uint32(v))
		lines = uint64(l)
		for ; vb > 0; vb-- {
			m.field++
			m.io.raise(irqVBlank)
		}
		m.dotAcc += v
		div := uint64(m.GPU.DotDivider())
		dots = m.dotAcc / div
		m.dotAcc -= dots * div
	}
	m.io.timers.tick(&m.io, n, dots, lines)
	m.CD.Tick(int64(n))
	m.MDEC.Tick(int32(n))
	if m.io.dma.active(0) || m.io.dma.active(1) {
		m.mdecPump()
	}
	m.SIO.Tick(int64(n))
	if irq := m.SIO.IRQ(); irq != m.sioIRQ {
		if irq {
			m.io.raise(irqSIO0)
		}
		m.sioIRQ = irq
	}
	if irq := m.CD.IRQ(); irq != m.cdIRQ {
		if irq {
			m.io.raise(irqCDROM)
		}
		m.cdIRQ = irq
	}
	if irq := m.GPU.IRQ(); irq != m.gpuIRQ {
		if irq {
			m.io.raise(irqGPU)
		}
		m.gpuIRQ = irq
	}
	m.CPU.SetIRQ(m.io.irqPending())
}

// physical 把虛擬位址轉成實體位址（006 §1，照 Beetle cpu.c 的 addr_mask）。
func physical(a uint32) uint32 {
	switch a >> 29 {
	case 4:
		return a & 0x7FFFFFFF
	case 5:
		return a & 0x1FFFFFFF
	}
	return a
}

func isScratchpad(virt, phys uint32) bool {
	// Scratchpad 只在 KUSEG 與 KSEG0。
	return phys >= 0x1F800000 && phys < 0x1F800400 && virt>>29 != 5
}

// ---- cpu.Bus ----

func (m *Machine) Read8(a uint32) uint8 {
	p := physical(a)
	switch {
	case p < 0x00800000:
		return m.ram[p&(ramSize-1)]
	case isScratchpad(a, p):
		return m.scratchpad[p&0x3FF]
	case p >= 0x1FC00000 && p < 0x1FC00000+biosSize:
		return m.bios[p-0x1FC00000]
	case p >= 0x1F801000 && p < 0x1F803000:
		return uint8(m.ioRead(p, 1))
	case p >= 0x1F000000 && p < 0x1F800000:
		return 0xFF
	}
	return 0
}

func (m *Machine) Read16(a uint32) uint16 {
	p := physical(a)
	switch {
	case p < 0x00800000:
		return binary.LittleEndian.Uint16(m.ram[p&(ramSize-1):])
	case isScratchpad(a, p):
		return binary.LittleEndian.Uint16(m.scratchpad[p&0x3FF:])
	case p >= 0x1FC00000 && p < 0x1FC00000+biosSize:
		return binary.LittleEndian.Uint16(m.bios[p-0x1FC00000:])
	case p >= 0x1F801000 && p < 0x1F803000:
		return uint16(m.ioRead(p, 2))
	case p >= 0x1F000000 && p < 0x1F800000:
		return 0xFFFF
	}
	return 0
}

func (m *Machine) Read32(a uint32) uint32 {
	p := physical(a)
	switch {
	case p < 0x00800000:
		return binary.LittleEndian.Uint32(m.ram[p&(ramSize-1):])
	case isScratchpad(a, p):
		return binary.LittleEndian.Uint32(m.scratchpad[p&0x3FF:])
	case p >= 0x1FC00000 && p < 0x1FC00000+biosSize:
		return binary.LittleEndian.Uint32(m.bios[p-0x1FC00000:])
	case p >= 0x1F801000 && p < 0x1F803000:
		return m.ioRead(p, 4)
	case p >= 0x1F000000 && p < 0x1F800000:
		return 0xFFFFFFFF
	case p == 0xFFFE0130:
		return m.biu
	}
	return 0
}

// Write8 的 v 是整個來源暫存器（cpu.Bus）；記憶體只取低 8 位元。
func (m *Machine) Write8(a uint32, v uint32) {
	p := physical(a)
	switch {
	case p < 0x00800000:
		m.ram[p&(ramSize-1)] = uint8(v)
	case isScratchpad(a, p):
		m.scratchpad[p&0x3FF] = uint8(v)
	case p >= 0x1F801000 && p < 0x1F803000:
		m.ioWrite(p, v, 1)
	}
}

// Write16 的 v 是整個來源暫存器（cpu.Bus）；記憶體只取低 16 位元。
func (m *Machine) Write16(a uint32, v uint32) {
	p := physical(a)
	switch {
	case p < 0x00800000:
		binary.LittleEndian.PutUint16(m.ram[p&(ramSize-1):], uint16(v))
	case isScratchpad(a, p):
		binary.LittleEndian.PutUint16(m.scratchpad[p&0x3FF:], uint16(v))
	case p >= 0x1F801000 && p < 0x1F803000:
		m.ioWrite(p, v, 2)
	}
}

func (m *Machine) Write32(a uint32, v uint32) {
	p := physical(a)
	switch {
	case p < 0x00800000:
		binary.LittleEndian.PutUint32(m.ram[p&(ramSize-1):], v)
	case isScratchpad(a, p):
		binary.LittleEndian.PutUint32(m.scratchpad[p&0x3FF:], v)
	case p >= 0x1F801000 && p < 0x1F803000:
		m.ioWrite(p, v, 4)
	case p == 0xFFFE0130:
		m.biu = v &^ 0x440
	}
}

// BIU 回傳快取控制暫存器。
func (m *Machine) BIU() uint32 { return m.biu }

// ReadCost 依 008 §2.1 回傳讀取的額外週期（數值來自 ps1-tests cpu/access-time 實機紀錄）。
func (m *Machine) ReadCost(a uint32, size int) int {
	p := physical(a)
	w := 0
	switch size {
	case 2:
		w = 1
	case 4:
		w = 2
	}
	switch {
	case p < 0x00800000:
		return 4
	case isScratchpad(a, p):
		return 0
	case p >= 0x1FC00000 && p < 0x1FC00000+biosSize:
		return [3]int{6, 12, 24}[w]
	case p >= 0x1F801800 && p < 0x1F801804:
		return [3]int{7, 13, 25}[w]
	case p >= 0x1F801C00 && p < 0x1F802000:
		return [3]int{17, 17, 38}[w]
	case p >= 0x1F801000 && p < 0x1F802000:
		return 2
	case p >= 0x1F802000 && p < 0x1F804000:
		return [3]int{10, 25, 55}[w]
	case p >= 0x1FA00000 && p < 0x1FC00000:
		return [3]int{5, 5, 9}[w]
	case p >= 0x1F000000 && p < 0x1F800000:
		return [3]int{6, 12, 24}[w]
	case p == 0xFFFE0130:
		return 0
	}
	return 0
}

// IsolatedWrite：SR.IsC 成立時的寫入不進記憶體；(BIU & 0x81) == 0x80 時寫 scratchpad（006 §4）。
func (m *Machine) IsolatedWrite(a uint32, v uint32, size int) {
	if m.biu&0x081 != 0x080 {
		return
	}
	o := a & 0x3FF
	switch size {
	case 1:
		m.scratchpad[o] = uint8(v)
	case 2:
		binary.LittleEndian.PutUint16(m.scratchpad[o&^1:], uint16(v))
	default:
		binary.LittleEndian.PutUint32(m.scratchpad[o&^3:], v)
	}
}

// InstructionBusError：從這些區域取指令會發生匯流排錯誤（006 §4.1）。
// 依據是 ps1-tests cpu/code-in-io 的實機紀錄：scratchpad、MDEC、中斷控制器出錯；
// SPU、DMA 不出錯。其餘 I/O 區域沒有證據，維持可執行。
func (m *Machine) InstructionBusError(a uint32) bool {
	p := physical(a)
	switch {
	case isScratchpad(a, p):
		return true
	case p >= 0x1F801070 && p < 0x1F801078:
		return true
	case p >= 0x1F801820 && p < 0x1F801828:
		return true
	}
	return false
}

// ReadRAM 複製一段主記憶體（觀測用）。
func (m *Machine) ReadRAM(phys uint32, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = m.ram[(phys+uint32(i))&(ramSize-1)]
	}
	return out
}

// DebugIO 回傳中斷與 DMA 的狀態（除錯用）。
func (m *Machine) DebugIO() string {
	d := &m.io.dma
	return fmt.Sprintf("I_STAT=%03x I_MASK=%03x DICR=%08x DPCR=%08x CHCR3=%08x CHCR2=%08x CDIRQ=%v",
		m.io.iStat, m.io.iMask, d.dicr, d.dpcr, d.chcr[3], d.chcr[2], m.CD.IRQ())
}
