package machine

import "encoding/binary"

// 中斷來源（I_STAT 的位元），psx-spx interrupts.md。
const (
	irqVBlank = 0
	irqGPU    = 1
	irqCDROM  = 2
	irqDMA    = 3
	irqTimer0 = 4
	irqTimer1 = 5
	irqTimer2 = 6
	irqSIO0   = 7
	irqSIO1   = 8
	irqSPU    = 9
)

// ioState 是 006 §2 列出的最小 I/O。沒有特別處理的暫存器存在 regs，讀回寫入值。
type ioState struct {
	regs [0x1000]byte // 1F801000..1F801FFF

	iStat, iMask uint32

	dma    dmaState
	timers timerState
}

func (s *ioState) reset() {
	*s = ioState{}
	s.dma.dpcr = 0x07654321
	s.dma.chcr[6] = 2
}

func (s *ioState) raise(bit uint) {
	if IRQTrace != nil && s.iStat&(1<<bit) == 0 {
		IRQTrace(false, bit)
	}
	s.iStat |= 1 << bit
}

// IRQTrace 不為 nil 時記錄中斷：ack 為 false 是 I_STAT 某位元變 1，為 true 是程式寫 I_STAT（bit 為寫入值）。
var IRQTrace func(ack bool, bit uint)

func (s *ioState) irqPending() bool { return s.iStat&s.iMask != 0 }

// ioRead 讀 I/O 區，size 為 1、2、4。
func (m *Machine) ioRead(p uint32, size int) uint32 {
	if p >= 0x1F802000 {
		return 0xFFFFFFFF >> (32 - 8*size) // DEV8 擴充區
	}
	if p >= 0x1F801800 && p < 0x1F801804 {
		return uint32(m.cdRead(p))
	}
	if p >= 0x1F801040 && p < 0x1F801050 {
		v := m.SIO.Read(p & 0xE)
		if size < 4 {
			v &= 1<<(8*size) - 1
		}
		return v
	}
	al := p &^ 3
	v := m.ioRead32(al)
	v >>= (p & 3) * 8
	if size < 4 {
		v &= 1<<(8*size) - 1
	}
	return v
}

// ioWrite 寫 I/O 區。寬度小於 4 的寫入先與目前值合併成 32 位元再交給處理函式。
func (m *Machine) ioWrite(p uint32, v uint32, size int) {
	if p >= 0x1F802000 {
		return
	}
	if size < 4 && ioWritesFull32(p) {
		// (w32)：整個暫存器左移到位址對應的位元組後寫入整個字（psx-spx unpredictablethings.md）
		m.ioWrite32(p&^3, v<<((p&3)*8))
		return
	}
	if size < 4 {
		v &= 1<<(8*size) - 1
	}
	if p >= 0x1F801800 && p < 0x1F801804 {
		m.cdWrite(p, uint8(v))
		return
	}
	if p >= 0x1F801040 && p < 0x1F801050 {
		m.SIO.Write(p&0xE, v)
		return
	}
	al := p &^ 3
	if size < 4 {
		shift := (p & 3) * 8
		mask := uint32(1<<(8*size)-1) << shift
		v = m.ioPeek32(al)&^mask | (v<<shift)&mask
	}
	m.ioWrite32(al, v)
}

// ioWritesFull32 是 psx-spx「I/O Write Datasize」表中 8、16 位元寫入標為 (w32) 的區域：
// MEMCTRL、RAM_SIZE、中斷控制、DMA 的 ADDR／CTRL／DPCR／DICR、計時器。DMA 的 LEN 標為 OK，照一般合併。
func ioWritesFull32(p uint32) bool {
	switch {
	case p >= 0x1F801000 && p < 0x1F801024, p >= 0x1F801060 && p < 0x1F801064,
		p >= 0x1F801070 && p < 0x1F801078, p >= 0x1F8010F0 && p < 0x1F8010F8,
		p >= 0x1F801100 && p < 0x1F80112C:
		return true
	case p >= 0x1F801080 && p < 0x1F8010F0:
		return p&0xC != 0x4
	}
	return false
}

func (m *Machine) store32(al uint32, v uint32) {
	binary.LittleEndian.PutUint32(m.io.regs[al-0x1F801000:], v)
}

func (m *Machine) load32(al uint32) uint32 {
	return binary.LittleEndian.Uint32(m.io.regs[al-0x1F801000:])
}

// ioPeek32 是合併部分寫入用的「目前值」，不能有讀取副作用。
func (m *Machine) ioPeek32(al uint32) uint32 {
	switch {
	case al == 0x1F801070:
		return m.io.iStat
	case al == 0x1F801074:
		return m.io.iMask
	case al >= 0x1F801080 && al < 0x1F801100:
		return m.io.dma.read(al)
	case al >= 0x1F801100 && al < 0x1F801130:
		return m.io.timers.peek(al)
	case al == 0x1F801820:
		return 0
	case al == 0x1F801824:
		return m.MDEC.Read(al)
	}
	return m.load32(al)
}

func (m *Machine) ioRead32(al uint32) uint32 {
	switch {
	case al == 0x1F801054: // SIO1 狀態：傳送就緒（SIO1 未實作）
		return 0x00000005
	case al == 0x1F801070:
		return m.io.iStat
	case al == 0x1F801074:
		return m.io.iMask
	case al >= 0x1F801080 && al < 0x1F801100:
		return m.io.dma.read(al)
	case al >= 0x1F801100 && al < 0x1F801130:
		return m.io.timers.read(al)
	case al == 0x1F801810:
		return m.GPU.ReadData()
	case al == 0x1F801814:
		return m.GPU.Status()
	case al == 0x1F801820 || al == 0x1F801824:
		v := m.MDEC.Read(al)
		m.mdecPump()
		return v
	case al == 0x1F801DAC: // SPUSTAT 在 DAE，與 DAC（傳輸控制）同一個字
		spucnt := uint32(binary.LittleEndian.Uint16(m.io.regs[0xDAA:]))
		return m.load32(al)&^0x003F0000 | (spucnt&0x3F)<<16
	}
	return m.load32(al)
}

func (m *Machine) ioWrite32(al uint32, v uint32) {
	switch {
	case al == 0x1F801070:
		if IRQTrace != nil {
			IRQTrace(true, uint(v&0x7FF))
		}
		m.io.iStat &= v
	case al == 0x1F801074:
		m.io.iMask = v & 0x7FF
	case al >= 0x1F801080 && al < 0x1F801100:
		m.dmaWrite(al, v)
	case al >= 0x1F801100 && al < 0x1F801130:
		m.io.timers.write(al, v)
	case al == 0x1F801810:
		m.GPU.WriteGP0(v)
	case al == 0x1F801814:
		m.GPU.WriteGP1(v)
	case al == 0x1F801820 || al == 0x1F801824:
		m.MDEC.Write(al, v)
		m.mdecPump()
	default:
		m.store32(al, v)
	}
}

// ---- CD-ROM（009）----

// CDTrace 不為 nil 時記錄 CD-ROM 暫存器存取（除錯用）。
var CDTrace func(write bool, reg uint32, index uint8, v uint8)

func (m *Machine) cdRead(p uint32) uint8 {
	v := m.CD.Read(p)
	if CDTrace != nil {
		CDTrace(false, p&3, 0, v)
	}
	return v
}

func (m *Machine) cdWrite(p uint32, v uint8) {
	if CDTrace != nil {
		CDTrace(true, p&3, 0, v)
	}
	m.CD.Write(p, v)
}

// ---- DMA ----

type dmaState struct {
	madr, bcr, chcr [7]uint32
	dpcr, dicr      uint32
}

func (d *dmaState) read(al uint32) uint32 {
	off := al - 0x1F801080
	if off >= 0x70 {
		switch off {
		case 0x70:
			return d.dpcr
		case 0x74:
			return d.dicr
		}
		return 0
	}
	ch := off >> 4
	switch off & 0xF {
	case 0x0:
		return d.madr[ch]
	case 0x4:
		return d.bcr[ch]
	default: // 0x8 與其鏡像 0xC
		return d.chcr[ch]
	}
}

// updateMaster 重算 DICR bit 31：b15 或（b23 且任一旗標成立），不看通道遮罩（psx-spx dmachannels.md）。
// old 是修改 DICR 之前的 bit 31；只有 0→1 才算上升緣。
func (d *dmaState) updateMaster(old uint32) (rising bool) {
	old &= 1 << 31
	flags := (d.dicr >> 24) & 0x7F
	on := d.dicr&(1<<15) != 0 || (d.dicr&(1<<23) != 0 && flags != 0)
	d.dicr &^= 1 << 31
	if on {
		d.dicr |= 1 << 31
	}
	return old == 0 && on
}

// CHCR 可寫的位元。channel 6（OTC）只有 24、28、30 可寫，bit 1 恆為 1。
const (
	chcrWritable    = 0x71770703
	chcrOTCWritable = 1<<24 | 1<<28 | 1<<30
)

// DMATrace 不為 nil 時記錄 DMA 暫存器寫入（除錯用）。
var DMATrace func(off uint32, v uint32, dicrAfter uint32)

func (m *Machine) dmaWrite(al uint32, v uint32) {
	d := &m.io.dma
	if DMATrace != nil {
		defer func() { DMATrace(al-0x1F801080, v, d.dicr) }()
	}
	off := al - 0x1F801080
	if off >= 0x70 {
		switch off {
		case 0x70:
			d.dpcr = v
			m.mdecPump()
		case 0x74:
			// bit 0–5、15、16–23 可寫。psx-spx 寫 bit 0–6，但 ps1-tests
			// cpu/io-access-bitwidth 的實機紀錄是寫 0x78 讀回 0x38，bit 6 不可寫。
			keep := v & 0x00FF803F
			ack := v & 0x7F000000
			before := d.dicr
			d.dicr = keep | (d.dicr&0x7F000000)&^ack
			if d.updateMaster(before) {
				m.io.raise(irqDMA)
			}
		}
		return
	}
	ch := off >> 4
	switch off & 0xF {
	case 0x0:
		d.madr[ch] = v & 0x00FFFFFF
	case 0x4:
		d.bcr[ch] = v
	default:
		if ch == 6 {
			d.chcr[ch] = v&chcrOTCWritable | 2
		} else {
			d.chcr[ch] = v & chcrWritable
		}
		m.dmaMaybeStart(int(ch))
	}
}

// dmaMaybeStart：bit 24 成立、DPCR 的通道總開關開啟；同步模式 0 另需 bit 28（強制開始）。
// 其他模式視為裝置一直要求資料（DREQ），立即開始。
func (m *Machine) dmaMaybeStart(ch int) {
	d := &m.io.dma
	c := d.chcr[ch]
	if c&(1<<24) == 0 || d.dpcr&(1<<(3+4*ch)) == 0 {
		return
	}
	if (c>>9)&3 == 0 && c&(1<<28) == 0 {
		return
	}
	if ch <= 1 {
		m.mdecPump()
		return
	}
	m.dmaRun(ch)
}

// dmaActive：通道已啟動、DPCR 開啟；同步模式 0 另需 bit 28。
func (d *dmaState) active(ch int) bool {
	c := d.chcr[ch]
	if c&(1<<24) == 0 || d.dpcr&(1<<(3+4*ch)) == 0 {
		return false
	}
	return (c>>9)&3 != 0 || c&(1<<28) != 0
}

// mdecPump 推進 channel 0、1（011 §3）：MDEC 能收或能給時搬一個區塊，直到兩邊都沒有進展。
func (m *Machine) mdecPump() {
	d := &m.io.dma
	for progress := true; progress; {
		progress = false
		if d.active(0) && m.MDEC.DMACanWrite() {
			m.dmaMDECBlock(0)
			progress = true
		}
		if d.active(1) && m.MDEC.DMACanRead() {
			m.dmaMDECBlock(1)
			progress = true
		}
	}
}

// dmaMDECBlock 搬一個區塊（Beetle dma.c RunChannel、ChRW）。同步模式 0 整段算一個區塊。
func (m *Machine) dmaMDECBlock(ch int) {
	d := &m.io.dma
	c := d.chcr[ch]
	words := d.bcr[ch] & 0xFFFF
	if words == 0 {
		words = 0x10000
	}
	addr := d.madr[ch] & 0xFFFFFF
	toDev := c&1 != 0
	for i := uint32(0); i < words; i++ {
		a := addr & 0x1FFFFC
		switch {
		case ch == 0 && toDev:
			m.MDEC.DMAWrite(binary.LittleEndian.Uint32(m.ram[a:]))
		case ch == 0:
			binary.LittleEndian.PutUint32(m.ram[a:], 0)
		case !toDev:
			v, offs := m.MDEC.DMARead()
			binary.LittleEndian.PutUint32(m.ram[(addr+offs<<2)&0x1FFFFC:], v)
		}
		if c&2 != 0 {
			addr = (addr - 4) & 0xFFFFFF
		} else {
			addr = (addr + 4) & 0xFFFFFF
		}
	}
	d.madr[ch] = addr
	done := true
	if (c>>9)&3 == 1 {
		blocks := (d.bcr[ch]>>16 - 1) & 0xFFFF
		d.bcr[ch] = d.bcr[ch]&0xFFFF | blocks<<16
		done = blocks == 0
	}
	if done {
		m.dmaFinish(ch)
	}
}

// dmaRun 立即完成一次傳輸（006 §2、007 §1）：channel 2 送 GPU，channel 6 清表，其他不搬資料。
func (m *Machine) dmaRun(ch int) {
	d := &m.io.dma
	switch ch {
	case 2:
		m.dmaGPU()
	case 3:
		m.dmaCD()
	case 6:
		addr := d.madr[6] & 0x1FFFFC
		n := d.bcr[6] & 0xFFFF
		if n == 0 {
			n = 0x10000
		}
		for i := uint32(0); i < n; i++ {
			w := (addr - 4) & 0x1FFFFF
			if i == n-1 {
				w = 0xFFFFFF
			}
			binary.LittleEndian.PutUint32(m.ram[addr:], w)
			addr = (addr - 4) & 0x1FFFFC
		}
	}
	m.dmaFinish(ch)
}

// dmaFinish 結束一次傳輸：清 CHCR bit 24、28，依 DICR 設旗標與中斷。
func (m *Machine) dmaFinish(ch int) {
	d := &m.io.dma
	d.chcr[ch] &^= 1<<24 | 1<<28
	before := d.dicr
	if d.dicr&(1<<(16+ch)) != 0 && d.dicr&(1<<23) != 0 {
		d.dicr |= 1 << (24 + ch)
	}
	if d.updateMaster(before) {
		m.io.raise(irqDMA)
	}
}

// dmaCD：channel 3，CD-ROM → RAM。字數：同步模式 0 為 BCR 低 16 位元，1 為區塊大小 × 區塊數。
func (m *Machine) dmaCD() {
	d := &m.io.dma
	words := d.bcr[3] & 0xFFFF
	if (d.chcr[3]>>9)&3 == 1 {
		words *= d.bcr[3] >> 16
	}
	if words == 0 {
		words = 0x10000
	}
	addr := d.madr[3] & 0x1FFFFC
	for i := uint32(0); i < words; i++ {
		binary.LittleEndian.PutUint32(m.ram[addr:], m.CD.DMARead())
		addr = (addr + 4) & 0x1FFFFC
	}
	if (d.chcr[3]>>9)&3 == 1 {
		d.madr[3] = addr
		d.bcr[3] &= 0xFFFF
	}
}

// dmaLinkedListLimit 是 linked list 走訪的上限，防止自我參照的串列卡死模擬器。
const dmaLinkedListLimit = 1 << 20

// dmaGPU：channel 2。CHCR bit 0 為方向（1=RAM→GPU），bit 9–10 為同步模式。
func (m *Machine) dmaGPU() {
	d := &m.io.dma
	chcr := d.chcr[2]
	toDevice := chcr&1 != 0
	step := uint32(4)
	if chcr&2 != 0 {
		step = ^uint32(3) // -4
	}
	addr := d.madr[2] & 0x1FFFFC
	switch (chcr >> 9) & 3 {
	case 0, 1:
		var words uint32
		if (chcr>>9)&3 == 0 {
			words = d.bcr[2] & 0xFFFF
			if words == 0 {
				words = 0x10000
			}
		} else {
			bs := d.bcr[2] & 0xFFFF
			if bs == 0 {
				bs = 0x10000
			}
			words = bs * (d.bcr[2] >> 16)
		}
		for i := uint32(0); i < words; i++ {
			if toDevice {
				m.GPU.WriteGP0(binary.LittleEndian.Uint32(m.ram[addr:]))
			} else {
				binary.LittleEndian.PutUint32(m.ram[addr:], m.GPU.ReadData())
			}
			addr = (addr + step) & 0x1FFFFC
		}
		if (chcr>>9)&3 == 1 {
			d.madr[2] = addr
			d.bcr[2] &= 0xFFFF
		}
	case 2:
		for n := 0; n < dmaLinkedListLimit; n++ {
			hdr := binary.LittleEndian.Uint32(m.ram[addr:])
			cnt := hdr >> 24
			for i := uint32(0); i < cnt; i++ {
				a := (addr + 4 + 4*i) & 0x1FFFFC
				m.GPU.WriteGP0(binary.LittleEndian.Uint32(m.ram[a:]))
			}
			if hdr&0x800000 != 0 {
				break
			}
			addr = hdr & 0x1FFFFC
		}
		d.madr[2] = 0x00FFFFFF
	}
}

// ---- 計時器（本階段：計數、到達目標、溢位與中斷）----

type timer struct {
	count, mode, target uint32
}

type timerState struct {
	t       [3]timer
	divider uint64 // timer2 的 sysclock/8
}

func (ts *timerState) peek(al uint32) uint32 {
	n := (al - 0x1F801100) >> 4
	t := &ts.t[n]
	switch al & 0xF {
	case 0x0:
		return t.count
	case 0x4:
		return t.mode
	case 0x8:
		return t.target
	}
	return 0
}

func (ts *timerState) read(al uint32) uint32 {
	v := ts.peek(al)
	if al&0xF == 0x4 {
		n := (al - 0x1F801100) >> 4
		ts.t[n].mode &^= 3 << 11 // 讀模式暫存器清除「到達」旗標
	}
	return v
}

func (ts *timerState) write(al uint32, v uint32) {
	n := (al - 0x1F801100) >> 4
	t := &ts.t[n]
	switch al & 0xF {
	case 0x0:
		t.count = v & 0xFFFF
	case 0x4:
		t.mode = v&0x3FF | 1<<10 // bit10：IRQ 線未拉低
		t.count = 0
	case 0x8:
		t.target = v & 0xFFFF
	}
}

// tick 推進計時器。sys 為系統時脈週期，dots 為點時脈，lines 為 HBlank 次數（psx-spx timers.md）：
// timer0 bit 8 選點時脈，timer1 bit 8 選 HBlank，timer2 bit 9 選系統時脈 ÷8。
// 同步模式（bit 0–2）尚未實作。
func (ts *timerState) tick(s *ioState, sys, dots, lines uint64) {
	for i := range ts.t {
		steps := sys
		switch i {
		case 0:
			if ts.t[0].mode&(1<<8) != 0 {
				steps = dots
			}
		case 1:
			if ts.t[1].mode&(1<<8) != 0 {
				steps = lines
			}
		case 2:
			if ts.t[2].mode&(1<<9) != 0 {
				ts.divider += sys
				steps = ts.divider / 8
				ts.divider %= 8
			}
		}
		for k := uint64(0); k < steps; k++ {
			ts.inc(s, i)
		}
	}
}

func (ts *timerState) inc(s *ioState, i int) {
	t := &ts.t[i]
	t.count = (t.count + 1) & 0xFFFF
	irq := false
	if t.count == t.target {
		t.mode |= 1 << 11
		if t.mode&(1<<4) != 0 {
			irq = true
		}
		if t.mode&(1<<3) != 0 {
			t.count = 0
		}
	}
	if t.count == 0xFFFF {
		t.mode |= 1 << 12
		if t.mode&(1<<5) != 0 {
			irq = true
		}
	}
	if irq {
		s.raise(uint(irqTimer0 + i))
	}
}
