// Package sio 實作 SIO0（手把與記憶卡埠）。規格：docs/spec/010-pad-and-memcard.md。
//
// 參照 beetle-psx-libretro mednafen/psx/frontio.c
// (commit e57860d82faa953dc0fc10f201fd583739bc2f7d)，GPL-2.0-or-later。
// 只實作兩個埠、不含 multitap；裝置只有數位手把與記憶卡。
package sio

// Device 是接在埠上的裝置（手把或記憶卡）。所有裝置的資料線並聯。
type Device interface {
	// SetDTR 是 /CS：選到這個埠時為真。
	SetDTR(on bool)
	// Clock 傳一個位元：txd 為主機送出的位元，回傳裝置送回的位元，
	// 以及這個位元組結束後要過多少週期拉 /ACK（0 表示不拉）。
	Clock(txd bool) (rxd bool, ackDelay int64)
}

// None 是空的埠：資料線維持 1、不回 /ACK。
type None struct{}

func (None) SetDTR(bool)              {}
func (None) Clock(bool) (bool, int64) { return true, 0 }

var scaleShift = [4]uint{0, 0, 4, 6}

// SIO 是 SIO0 的完整狀態。
type SIO struct {
	// Pads 與 Cards 各兩個埠（port 1、port 2）。
	Pads  [2]Device
	Cards [2]Device

	mode, control, baud uint32

	txBuf     uint8
	txPending bool
	txActive  bool
	txBit     uint
	rxBuf     uint8
	rxPending bool
	rxActive  bool
	rxAvail   bool
	rxBit     uint
	divider   int64

	ackDelay [4]int64 // 0、1：手把埠；2、3：記憶卡埠
	ackUntil [4]int64
	now      int64

	irq bool
}

// New 建立 SIO0；未接的埠用 None。
func New() *SIO {
	s := &SIO{}
	for i := 0; i < 2; i++ {
		s.Pads[i] = None{}
		s.Cards[i] = None{}
	}
	for i := range s.ackUntil {
		s.ackUntil[i] = -1
	}
	return s
}

// IRQ 回報中斷線（接 I_STAT bit 7）。
func (s *SIO) IRQ() bool { return s.irq }

func (s *SIO) clockDivider() int64 {
	d := int64(s.baud<<scaleShift[s.mode&3]) &^ 1
	if d < 0x20 {
		d = 0x20
	}
	return d
}

func (s *SIO) checkStartStop() {
	if (s.rxPending && s.control&0x4 != 0) || (s.txPending && s.control&0x1 != 0) {
		if s.rxPending {
			s.rxPending = false
			s.rxActive = true
			s.rxAvail = false
			s.rxBuf = 0
			s.rxBit = 0
		}
		if s.txPending {
			s.txPending = false
			s.txActive = true
			s.txBit = 0
		}
		s.divider = s.clockDivider()
	}
	if s.control&0x5 == 0 {
		s.rxActive = false
		s.txActive = false
	}
	if !s.rxActive && !s.txActive {
		s.divider = 0
	}
}

// Write 寫 1F801040+a（a 為 0、8、A、E）。
func (s *SIO) Write(a uint32, v uint32) {
	switch a & 0xF {
	case 0x0:
		s.txBuf = uint8(v)
		s.txPending = true
		s.txActive = false
	case 0x8:
		s.mode = v & 0x013F
	case 0xA:
		s.control = v & 0x3F2F
		if v&0x10 != 0 {
			s.irq = false
		}
		if v&0x40 != 0 {
			s.irq = false
			s.divider = 0
			s.rxPending, s.txPending = false, false
			s.rxActive, s.txActive = false, false
			s.rxAvail = false
			s.txBuf, s.rxBuf = 0, 0
			s.rxBit, s.txBit = 0, 0
			s.mode, s.control, s.baud = 0, 0, 0
		}
		sel1 := s.control&0x2 != 0 && s.control&0x2000 == 0
		sel2 := s.control&0x2 != 0 && s.control&0x2000 != 0
		s.Pads[0].SetDTR(sel1)
		s.Cards[0].SetDTR(sel1)
		s.Pads[1].SetDTR(sel2)
		s.Cards[1].SetDTR(sel2)
		if !sel1 {
			s.ackDelay[0], s.ackDelay[2] = 0, 0
			s.ackUntil[0], s.ackUntil[2] = -1, -1
		}
		if !sel2 {
			s.ackDelay[1], s.ackDelay[3] = 0, 0
			s.ackUntil[1], s.ackUntil[3] = -1, -1
		}
	case 0xE:
		s.baud = v & 0xFFFF
	}
	s.checkStartStop()
}

// Read 讀 1F801040+a。
func (s *SIO) Read(a uint32) uint32 {
	switch a & 0xF {
	case 0x0:
		r := uint32(s.rxBuf) * 0x01010101
		s.rxAvail = false
		s.rxPending = true
		s.rxActive = false
		s.checkStartStop()
		return r
	case 0x4:
		var r uint32
		if !s.txPending && !s.txActive {
			r |= 0x1
		}
		if s.rxAvail {
			r |= 0x2
		}
		for i := range s.ackUntil {
			if s.now < s.ackUntil[i] {
				r |= 0x80
				break
			}
		}
		if s.irq {
			r |= 0x200
		}
		return r
	case 0x8:
		return s.mode
	case 0xA:
		return s.control
	case 0xE:
		return s.baud
	}
	return 0
}

// Tick 推進 clocks 個 CPU 週期（Beetle FrontIO_Update）。
func (s *SIO) Tick(clocks int64) {
	s.now += clocks
	for i := range s.ackDelay {
		if s.ackDelay[i] > 0 {
			s.ackDelay[i] -= clocks
			if s.ackDelay[i] <= 0 {
				s.ackUntil[i] = s.now + 32 + s.ackDelay[i]
				if s.control&0x1000 != 0 {
					s.irq = true
				}
			}
		}
	}
	if s.divider <= 0 {
		return
	}
	s.divider -= clocks
	check := false
	for s.divider <= 0 {
		if !s.rxActive && !s.txActive {
			break
		}
		txd := true
		if s.txActive {
			txd = (s.txBuf>>s.txBit)&1 != 0
			s.txBit = (s.txBit + 1) & 7
			if s.txBit == 0 {
				check = true
				s.txActive = false
				if s.control&0x400 != 0 {
					s.irq = true
				}
			}
		}
		rxd := true
		devs := [4]Device{s.Pads[0], s.Pads[1], s.Cards[0], s.Cards[1]}
		for i, d := range devs {
			b, delay := d.Clock(txd)
			rxd = rxd && b
			if delay > 0 {
				s.ackDelay[i] = delay
			}
		}
		if s.rxActive {
			s.rxBuf &^= 1 << s.rxBit
			if rxd {
				s.rxBuf |= 1 << s.rxBit
			}
			s.rxBit = (s.rxBit + 1) & 7
			if s.rxBit == 0 {
				check = true
				s.rxActive = false
				s.rxAvail = true
				if s.control&0x800 != 0 {
					s.irq = true
				}
			}
		}
		s.divider += s.clockDivider()
	}
	if check {
		s.checkStartStop()
	}
}

// ---- 數位手把（Beetle InputDevice_Gamepad）----

// 按鍵位元（psx-spx controllersandmemorycards.md，標準手把），按下為 1。
const (
	BtnSelect   = 1 << 0
	BtnL3       = 1 << 1
	BtnR3       = 1 << 2
	BtnStart    = 1 << 3
	BtnUp       = 1 << 4
	BtnRight    = 1 << 5
	BtnDown     = 1 << 6
	BtnLeft     = 1 << 7
	BtnL2       = 1 << 8
	BtnR2       = 1 << 9
	BtnL1       = 1 << 10
	BtnR1       = 1 << 11
	BtnTriangle = 1 << 12
	BtnCircle   = 1 << 13
	BtnCross    = 1 << 14
	BtnSquare   = 1 << 15
)

// Gamepad 是數位手把（ID 5A41h）。
type Gamepad struct {
	// Buttons 是目前按著的鍵（Btn* 的組合）。
	Buttons uint16

	dtr     bool
	phase   int
	bitpos  uint
	rx      uint8
	tx      [3]uint8
	txPos   int
	txCount int
}

// SetDTR 實作 Device。
func (p *Gamepad) SetDTR(on bool) {
	if !p.dtr && on {
		p.phase = 0
		p.bitpos = 0
		p.txPos, p.txCount = 0, 0
	}
	p.dtr = on
}

// Clock 實作 Device。
func (p *Gamepad) Clock(txd bool) (bool, int64) {
	if !p.dtr {
		return true, 0
	}
	ret := true
	if p.txCount > 0 {
		ret = (p.tx[p.txPos]>>p.bitpos)&1 != 0
	}
	p.rx &^= 1 << p.bitpos
	if txd {
		p.rx |= 1 << p.bitpos
	}
	p.bitpos = (p.bitpos + 1) & 7
	if p.bitpos != 0 {
		return ret, 0
	}
	if p.txCount > 0 {
		p.txPos++
		p.txCount--
	}
	switch p.phase {
	case 0:
		if p.rx != 0x01 {
			p.phase = -1
		} else {
			p.tx[0] = 0x41
			p.txPos, p.txCount = 0, 1
			p.phase++
		}
	case 1:
		p.phase++
		p.tx[0] = 0x5A
		if p.rx == 0x42 {
			p.tx[1] = ^uint8(p.Buttons)
			p.tx[2] = ^uint8(p.Buttons >> 8)
			p.txPos, p.txCount = 0, 3
		} else {
			p.phase = -1
			p.txPos, p.txCount = 0, 0
		}
	}
	if p.txCount > 0 {
		return ret, 0x40
	}
	return ret, 0
}

// NextEvent 回傳最多再過多少週期，SIO 狀態就可能改變（不含程式存取暫存器）；-1 表示閒置。
func (s *SIO) NextEvent() int64 {
	next := int64(-1)
	for _, d := range s.ackDelay {
		if d > 0 && (next < 0 || d < next) {
			next = d
		}
	}
	if s.divider > 0 && (s.rxActive || s.txActive) && (next < 0 || s.divider < next) {
		next = s.divider
	}
	return next
}
