// Package mdec 是 PS1 的動畫解碼器（docs/spec/011-mdec.md）。
//
// 移植自 beetle-psx-libretro mednafen/psx/mdec.c
// （commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later），只取純 C 路徑。
// Beetle 用 switch 跳進迴圈中段的 coroutine 寫法；這裡改成明確的狀態，並且不模擬解碼時間（011 §4）。
package mdec

const fifoSize = 0x20

type fifo struct {
	data       [fifoSize]uint32
	rpos, wpos uint32
	n          uint32
}

func (f *fifo) canWrite() uint32 { return fifoSize - f.n }
func (f *fifo) flush()           { f.rpos, f.wpos, f.n = 0, 0, 0 }
func (f *fifo) write(v uint32) {
	f.data[f.wpos] = v
	f.wpos = (f.wpos + 1) & (fifoSize - 1)
	f.n++
}
func (f *fifo) read() uint32 {
	v := f.data[f.rpos]
	f.rpos = (f.rpos + 1) & (fifoSize - 1)
	f.n--
	return v
}

// 解碼器目前在做的事。
const (
	stIdle   = iota // 等下一個命令
	stDecode        // 收巨集區塊資料
	stQuant         // 收量化表
	stIDCT          // 收 IDCT 表
)

// MDEC 不含指標，可直接複製（Machine.Clone）。
type MDEC struct {
	in, out fifo

	blockY, blockCb, blockCr [8][8]int8

	control, command uint32
	inCommand        bool
	state            int

	qMatrix [2][64]uint8
	qmIndex uint32

	idctMatrix [64]int16
	idctIndex  uint32

	qScale     uint8
	coeff      [64]int16
	coeffIndex uint32
	decodeWB   uint32

	pix          [192]byte // 48 個 32 位元字
	pixReadOff   uint32
	pixCount32   uint32
	pendingPixel bool

	inCounter   uint16
	lastEncoded uint32

	clock      int32 // 可用的週期（Beetle ClockCounter）
	waitClock  bool  // 在等 clock 變正
	cmdPending bool  // 命令已讀進、等 1 個週期後執行

	ramOffsetY, ramOffsetCounter, ramOffsetWWS uint8
}

// New 回傳上電狀態的 MDEC。
func New() *MDEC { return &MDEC{} }

var zigZag = [64]uint8{
	0x00, 0x08, 0x01, 0x02, 0x09, 0x10, 0x18, 0x11,
	0x0a, 0x03, 0x04, 0x0b, 0x12, 0x19, 0x20, 0x28,
	0x21, 0x1a, 0x13, 0x0c, 0x05, 0x06, 0x0d, 0x14,
	0x1b, 0x22, 0x29, 0x30, 0x38, 0x31, 0x2a, 0x23,
	0x1c, 0x15, 0x0e, 0x07, 0x0f, 0x16, 0x1d, 0x24,
	0x2b, 0x32, 0x39, 0x3a, 0x33, 0x2c, 0x25, 0x1e,
	0x17, 0x1f, 0x26, 0x2d, 0x34, 0x3b, 0x3c, 0x35,
	0x2e, 0x27, 0x2f, 0x36, 0x3d, 0x3e, 0x37, 0x3f,
}

func signX(bits uint, v int32) int32 { return v << (32 - bits) >> (32 - bits) }

func mask9ClampS8(v int32) int8 {
	v = signX(9, v)
	if v < -128 {
		v = -128
	}
	if v > 127 {
		v = 127
	}
	return int8(v)
}

func (d *MDEC) idctPass1(in *[64]int16, out *[64]int16) {
	for col := 0; col < 8; col++ {
		for x := 0; x < 8; x++ {
			var sum int32
			for u := 0; u < 8; u++ {
				sum += int32(in[col*8+u]) * int32(d.idctMatrix[x*8+u])
			}
			out[x*8+col] = int16((sum + 0x4000) >> 15)
		}
	}
}

func (d *MDEC) idctPass2(in *[64]int16, out *[8][8]int8) {
	for col := 0; col < 8; col++ {
		for x := 0; x < 8; x++ {
			var sum int32
			for u := 0; u < 8; u++ {
				sum += int32(in[col*8+u]) * int32(d.idctMatrix[x*8+u])
			}
			out[col][x] = mask9ClampS8((sum + 0x4000) >> 15)
		}
	}
}

func (d *MDEC) idct(out *[8][8]int8) {
	var tmp [64]int16
	d.idctPass1(&d.coeff, &tmp)
	d.idctPass2(&tmp, out)
}

func ycbcrToRGB(y, cb, cr int8) (r, g, b int) {
	Y, CB, CR := int32(y), int32(cb), int32(cr)
	r = int(uint8(mask9ClampS8(Y+((359*CR+0x80)>>8))) ^ 0x80)
	g = int(uint8(mask9ClampS8(Y+((((-88*CB)&^0x1F)+((-183*CR)&^0x07)+0x80)>>8))) ^ 0x80)
	b = int(uint8(mask9ClampS8(Y+((454*CB+0x80)>>8))) ^ 0x80)
	return
}

func rgb555(r, g, b int) uint16 {
	c := func(v int) uint16 {
		v = (v + 4) >> 3
		if v > 0x1F {
			v = 0x1F
		}
		return uint16(v)
	}
	return c(r) | c(g)<<5 | c(b)<<10
}

func (d *MDEC) encodeImage(ybn uint32) {
	d.pixCount32 = 0
	signed := d.command&(1<<26) != 0
	switch (d.command >> 27) & 3 {
	case 0: // 4bpp
		var xr uint8 = 0x88
		if signed {
			xr = 0
		}
		i := 0
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x += 2 {
				v0 := int(d.blockY[y][x]) + 8
				v1 := int(d.blockY[y][x+1]) + 8
				if v0 > 127 {
					v0 = 127
				}
				if v1 > 127 {
					v1 = 127
				}
				d.pix[i] = (uint8(v0)>>4 | uint8(v1)&0xF0) ^ xr
				i++
			}
		}
		d.pixCount32 = 8
	case 1: // 8bpp
		var xr uint8 = 0x80
		if signed {
			xr = 0
		}
		i := 0
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				d.pix[i] = uint8(d.blockY[y][x]) ^ xr
				i++
			}
		}
		d.pixCount32 = 16
	case 2: // 24bpp
		var xr uint8
		if signed {
			xr = 0x80
		}
		i := 0
		for y := 0; y < 8; y++ {
			cy := y>>1 | int(ybn&2)<<1
			cx := int(ybn&1) << 2
			for x := 0; x < 8; x++ {
				r, g, b := ycbcrToRGB(d.blockY[y][x], d.blockCb[cy][cx+x>>1], d.blockCr[cy][cx+x>>1])
				d.pix[i], d.pix[i+1], d.pix[i+2] = uint8(r)^xr, uint8(g)^xr, uint8(b)^xr
				i += 3
			}
		}
		d.pixCount32 = 48
	case 3: // 15bpp
		var xr uint16
		if d.command&0x02000000 != 0 {
			xr |= 0x8000
		}
		if signed {
			xr |= 0x4210
		}
		i := 0
		for y := 0; y < 8; y++ {
			cy := y>>1 | int(ybn&2)<<1
			cx := int(ybn&1) << 2
			for x := 0; x < 8; x++ {
				r, g, b := ycbcrToRGB(d.blockY[y][x], d.blockCb[cy][cx+x>>1], d.blockCr[cy][cx+x>>1])
				p := xr ^ rgb555(r, g, b)
				d.pix[i], d.pix[i+1] = uint8(p), uint8(p>>8)
				i += 2
			}
		}
		d.pixCount32 = 32
	}
}

func (d *MDEC) writeImageData(v uint16) (eat int32) {
	qmw := 0
	if d.decodeWB < 2 {
		qmw = 1
	}
	clamp := func(t int32) int16 {
		if t < -0x4000 {
			t = -0x4000
		}
		if t > 0x3FFF {
			t = 0x3FFF
		}
		return int16(t)
	}
	round := func(ci int32) int32 {
		switch {
		case ci < 0:
			return 8
		case ci > 0:
			return -8
		}
		return 0
	}
	if d.coeffIndex == 0 {
		if v == 0xFE00 {
			return
		}
		d.qScale = uint8(v >> 10)
		q := int32(d.qMatrix[qmw][0]) // DC 不乘 QScale
		ci := signX(10, int32(v&0x3FF))
		var t int32
		if q != 0 {
			t = int32(uint32(ci*q)<<4) + round(ci)
		} else {
			t = int32(uint32(ci*2) << 4)
		}
		d.coeff[zigZag[0]] = clamp(t)
		d.coeffIndex++
	} else if v == 0xFE00 {
		for d.coeffIndex < 64 {
			d.coeff[zigZag[d.coeffIndex]] = 0
			d.coeffIndex++
		}
	} else {
		for i := uint32(0); i < uint32(v>>10) && d.coeffIndex < 64; i++ {
			d.coeff[zigZag[d.coeffIndex]] = 0
			d.coeffIndex++
		}
		if d.coeffIndex < 64 {
			q := int32(d.qScale) * int32(d.qMatrix[qmw][d.coeffIndex])
			ci := signX(10, int32(v&0x3FF))
			var t int32
			if q != 0 {
				t = int32(uint32((ci*q)>>3)<<4) + round(ci)
			} else {
				t = int32(uint32(ci*2) << 4)
			}
			d.coeff[zigZag[d.coeffIndex]] = clamp(t)
			d.coeffIndex++
		}
	}
	if d.coeffIndex == 64 {
		d.coeffIndex = 0
		switch d.decodeWB {
		case 0:
			d.idct(&d.blockCr)
		case 1:
			d.idct(&d.blockCb)
		default:
			d.idct(&d.blockY)
		}
		eat = blockCycles
		if d.decodeWB >= 2 {
			d.lastEncoded = (d.decodeWB + 4) % 6
			d.encodeImage(d.lastEncoded)
		}
		d.decodeWB++
		color := (d.command>>27)&2 != 0
		if color && d.decodeWB == 6 {
			d.decodeWB = 0
		} else if !color && d.decodeWB == 3 {
			d.decodeWB = 2
		}
	}
	return eat
}

// clockCap 是解碼器最多能預存的週期數（Beetle 的 EventCycles，libretro.c 預設 128）。
const clockCap = 128

// blockCycles 是解碼一個 8×8 區塊的週期數（Beetle WriteImageData 的平均值 512）。
const blockCycles = 512

// WaitingClock 回傳解碼器是否在等週期。這時 Tick 會消耗週期，必須逐條指令推進（預存上限 128）。
func (d *MDEC) WaitingClock() bool { return d.waitClock }

// Tick 讓解碼器前進 n 個 CPU 週期（Beetle MDEC_Run）。
func (d *MDEC) Tick(n int32) {
	d.clock += n
	if d.clock > clockCap {
		d.clock = clockCap
	}
	d.run()
}

// run 盡量往前處理，直到要等輸入、等週期或等輸出 FIFO 有空間。
func (d *MDEC) run() {
	for {
		if d.waitClock {
			if d.clock <= 0 {
				return
			}
			d.waitClock = false
			if d.cmdPending {
				d.cmdPending = false
				d.startCommand(d.command)
				continue
			}
		}
		if d.pendingPixel {
			for d.pixReadOff < d.pixCount32 {
				if d.out.canWrite() == 0 {
					return
				}
				o := d.pixReadOff * 4
				d.out.write(uint32(d.pix[o]) | uint32(d.pix[o+1])<<8 | uint32(d.pix[o+2])<<16 | uint32(d.pix[o+3])<<24)
				d.pixReadOff++
			}
			d.pendingPixel = false
			if d.inCounter == 0xFFFF {
				d.endCommand()
			}
		}
		if d.in.n == 0 {
			return
		}
		switch d.state {
		case stIdle:
			// 讀到命令後花 1 個週期才開始執行
			d.command = d.in.read()
			d.inCommand = true
			d.clock--
			d.cmdPending, d.waitClock = true, true
		case stDecode:
			w := d.in.read()
			d.inCounter--
			d.pixCount32 = 0
			eat := d.writeImageData(uint16(w))
			eat += d.writeImageData(uint16(w >> 16))
			d.pixReadOff = 0
			d.pendingPixel = true
			d.clock -= eat
			d.waitClock = true
		case stQuant:
			w := d.in.read()
			d.inCounter--
			for i := 0; i < 4; i++ {
				d.qMatrix[d.qmIndex>>6][d.qmIndex&0x3F] = uint8(w)
				d.qmIndex = (d.qmIndex + 1) & 0x7F
				w >>= 8
			}
			if d.inCounter == 0xFFFF {
				d.endCommand()
			}
		case stIDCT:
			w := d.in.read()
			d.inCounter--
			for i := 0; i < 2; i++ {
				d.idctMatrix[(d.idctIndex&7)<<3|(d.idctIndex>>3)&7] = int16(uint16(w)) >> 3
				d.idctIndex = (d.idctIndex + 1) & 0x3F
				w >>= 16
			}
			if d.inCounter == 0xFFFF {
				d.endCommand()
			}
		}
	}
}

func (d *MDEC) endCommand() {
	d.state = stIdle
	d.inCommand = false
}

func (d *MDEC) startCommand(c uint32) {
	d.command = c
	d.inCommand = true
	switch (c >> 29) & 7 {
	case 1:
		d.inCounter = uint16(c)
		d.out.flush()
		d.pixCount32 = 0
		d.coeffIndex = 0
		if (c>>27)&2 != 0 {
			d.decodeWB = 0
		} else {
			d.decodeWB = 2
		}
		switch (c >> 27) & 3 {
		case 0, 1:
			d.ramOffsetWWS = 0
		case 2:
			d.ramOffsetWWS = 6
		case 3:
			d.ramOffsetWWS = 4
		}
		d.ramOffsetY = 0
		d.ramOffsetCounter = d.ramOffsetWWS
		d.inCounter--
		d.state = stDecode
	case 2:
		d.qmIndex = 0
		d.inCounter = 0x10
		if c&1 != 0 {
			d.inCounter = 0x20
		}
		d.inCounter--
		d.state = stQuant
	case 3:
		d.idctIndex = 0
		d.inCounter = 0x20 - 1
		d.state = stIDCT
	default:
		// 其他命令只記下參數字數，回到等待命令（Beetle 同樣在下一圈把 InCommand 清掉）。
		d.inCounter = uint16(c)
		d.inCommand = false
	}
}

// DMAWrite 是 channel 0 送進一個字。
func (d *MDEC) DMAWrite(v uint32) {
	if d.in.canWrite() == 0 {
		return
	}
	d.in.write(v)
	d.run()
}

// DMARead 是 channel 1 取一個字；offs 是寫入位址的字位移（Beetle MDEC_DMARead）。
func (d *MDEC) DMARead() (v uint32, offs uint32) {
	if d.out.n == 0 {
		return 0, 0
	}
	v = d.out.read()
	offs = uint32(d.ramOffsetY&7) * uint32(d.ramOffsetWWS)
	if d.ramOffsetY&8 != 0 {
		offs -= uint32(d.ramOffsetWWS) * 7
	}
	d.ramOffsetCounter--
	if d.ramOffsetCounter == 0 {
		d.ramOffsetCounter = d.ramOffsetWWS
		d.ramOffsetY++
	}
	d.run()
	return v, offs
}

// DMACanWrite：輸入 FIFO 空、允許 DMA 輸入、命令執行中且還有參數要收。
func (d *MDEC) DMACanWrite() bool {
	return d.in.canWrite() >= 0x20 && d.control&(1<<30) != 0 && d.inCommand && d.inCounter != 0xFFFF
}

// DMACanRead：輸出 FIFO 至少 32 字且允許 DMA 輸出。
func (d *MDEC) DMACanRead() bool {
	return d.out.n >= 0x20 && d.control&(1<<29) != 0
}

// Write 寫 1F801820（bit 2 為 0）或 1F801824。
func (d *MDEC) Write(addr uint32, v uint32) {
	if addr&4 != 0 {
		if v&0x80000000 != 0 { // 重置：清的欄位與 Beetle MDEC_Write 相同，量化表、IDCT 表留著
			d.state, d.inCounter, d.command, d.inCommand = stIdle, 0, 0, false
			d.pixCount32, d.pendingPixel = 0, false
			d.clock, d.waitClock, d.cmdPending = 0, false, false
			d.qmIndex, d.idctIndex, d.qScale = 0, 0, 0
			d.coeff, d.coeffIndex, d.decodeWB = [64]int16{}, 0, 0
			d.in.flush()
			d.out.flush()
		}
		d.control = v & 0x7FFFFFFF
		return
	}
	if d.in.canWrite() != 0 {
		d.in.write(v)
		if !d.inCommand && d.clock < 1 {
			d.clock = 1
		}
		d.run()
	}
}

// Read 讀 1F801820（輸出資料）或 1F801824（狀態）。
func (d *MDEC) Read(addr uint32) uint32 {
	if addr&4 == 0 {
		if d.out.n == 0 {
			return 0
		}
		v := d.out.read()
		d.run()
		return v
	}
	var r uint32
	if d.out.n == 0 {
		r |= 1 << 31
	}
	if d.in.canWrite() == 0 {
		r |= 1 << 30
	}
	if d.inCommand {
		r |= 1 << 29
	}
	if d.DMACanWrite() {
		r |= 1 << 28
	}
	if d.DMACanRead() {
		r |= 1 << 27
	}
	r |= ((d.command >> 25) & 0xF) << 23
	return r | d.currentBlock()<<16 | uint32(d.inCounter)
}

// currentBlock 是狀態 bit 18–16（psx-spx macroblockdecodermdec.md；Beetle 未實作）：
// 輸出 FIFO 有資料時為輸出中的區塊（0–3 = Y1–Y4，黑白為 4），否則為正在收的區塊（4 = Cr、5 = Cb）。
// 輸出中的區塊以「最後編碼的區塊」近似。
func (d *MDEC) currentBlock() uint32 {
	if (d.command>>27)&2 == 0 {
		return 4
	}
	if d.out.n != 0 || d.pendingPixel {
		return d.lastEncoded
	}
	return (d.decodeWB + 4) % 6
}
