// Package gte 實作 PlayStation 的幾何轉換引擎（COP2）。
//
// 規格：docs/spec/003-gte.md。驗收語料是 ps1-tests gte-fuzz 的實機紀錄。
//
// 參照 beetle-psx-libretro mednafen/psx/gte.c
// (commit e57860d82faa953dc0fc10f201fd583739bc2f7d)，GPL-2.0-or-later。
// 只搬架構狀態的計算；PGXP 與寬螢幕的影子路徑屬 HD，不在這裡。
package gte

import "math/bits"

// 控制暫存器索引。
const (
	crTR   = 5  // 平移向量 TRX..TRZ（cr5..cr7）
	crBK   = 13 // 背景色 RBK..BBK（cr13..cr15）
	crFC   = 21 // 遠景色 RFC..BFC（cr21..cr23）
	crOFX  = 24
	crOFY  = 25
	crH    = 26
	crDQA  = 27
	crDQB  = 28
	crZSF3 = 29
	crZSF4 = 30
	crFLAG = 31
)

// MVMVA 的平移向量選擇（tx 欄位）。
const (
	cvTR = iota
	cvBK
	cvFC
	cvNone
)

// MVMVA 的矩陣選擇（mx 欄位）。
const (
	mxRotation = iota
	mxLight
	mxColor
	mxReserved
)

const vecIR = 3 // vx=3：以 [IR1,IR2,IR3] 為乘數向量

// FLAG 裡會讓 bit 31（錯誤總和）成立的位元。
const flagErrorMask = 0x7f87e000

// GTE 是一顆 GTE 的完整狀態。零值等同上電後的狀態。
type GTE struct {
	dr    [32]uint32
	cr    [32]uint32
	flags uint32 // 命令執行中累積的旗標，結束時寫回 cr[31]
}

// unrTable 是除法用的牛頓法初值表（UNR table），由公式產生。
var unrTable [0x101]uint8

func init() {
	for divisor := uint32(0x8000); divisor < 0x10000; divisor += 0x80 {
		xa := uint32(512)
		for i := 1; i < 5; i++ {
			xa = (xa * (1024*512 - ((divisor >> 7) * xa))) >> 18
		}
		unrTable[(divisor>>7)&0xFF] = uint8(((xa + 1) >> 1) - 0x101)
	}
	unrTable[0x100] = unrTable[0xFF]
}

// ---- 暫存器欄位 ----

func (g *GTE) ir(i int) int32          { return int32(int16(g.dr[8+i])) }
func (g *GTE) setIR(i int, v int32)    { g.dr[8+i] = uint32(int32(int16(v))) }
func (g *GTE) mac(i int) int32         { return int32(g.dr[24+i]) }
func (g *GTE) setMAC(i int, v int32)   { g.dr[24+i] = uint32(v) }
func (g *GTE) zfifo(i int) uint32      { return uint32(uint16(g.dr[16+i])) }
func (g *GTE) setZFIFO(i int, v int32) { g.dr[16+i] = uint32(uint16(v)) }
func (g *GTE) setOTZ(v uint16)         { g.dr[7] = uint32(v) }

func (g *GTE) rgbR() int32   { return int32(uint8(g.dr[6])) }
func (g *GTE) rgbG() int32   { return int32(uint8(g.dr[6] >> 8)) }
func (g *GTE) rgbB() int32   { return int32(uint8(g.dr[6] >> 16)) }
func (g *GTE) rgbCD() uint32 { return uint32(uint8(g.dr[6] >> 24)) }

// vector 取 V0..V2 的第 elm 個分量（0=X, 1=Y, 2=Z）。
func (g *GTE) vector(v, elm int) int32 {
	switch elm {
	case 0:
		return int32(int16(g.dr[v*2]))
	case 1:
		return int32(int16(g.dr[v*2] >> 16))
	default:
		return int32(int16(g.dr[v*2+1]))
	}
}

func lo16(v uint32) int32 { return int32(int16(v)) }
func hi16(v uint32) int32 { return int32(int16(v >> 16)) }

// ---- 讀寫 ----

// Write 寫入第 reg 個暫存器：0..31 為資料暫存器（MTC2），32..63 為控制暫存器（CTC2）。
func (g *GTE) Write(reg int, v uint32) {
	if reg >= 32 {
		g.writeCR(reg-32, v)
		return
	}
	g.writeDR(reg, v)
}

// Read 讀出第 reg 個暫存器：0..31 為資料暫存器（MFC2），32..63 為控制暫存器（CFC2）。
func (g *GTE) Read(reg int) uint32 {
	if reg >= 32 {
		return g.readCR(reg - 32)
	}
	return g.readDR(reg)
}

var crWriteMask = [32]uint32{
	0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0x0000FFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF,
	0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0x0000FFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF,
	0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0x0000FFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF,
	0xFFFFFFFF, 0xFFFFFFFF, 0x0000FFFF, 0x0000FFFF, 0xFFFFFFFF, 0x0000FFFF, 0x0000FFFF, 0xFFFFFFFF,
}

func (g *GTE) writeCR(which int, v uint32) {
	m := crWriteMask[which]
	v &= m
	g.cr[which] = v | (g.cr[which] &^ m)
	if which == crFLAG {
		g.cr[crFLAG] = v & 0x7ffff000
		if v&flagErrorMask != 0 {
			g.cr[crFLAG] |= 1 << 31
		}
	}
}

func (g *GTE) readCR(which int) uint32 {
	r := g.cr[which]
	switch which {
	case 4, 12, 20, 26, 27, 29, 30:
		r = uint32(int32(int16(r)))
	}
	return r
}

func (g *GTE) writeDR(which int, v uint32) {
	switch which {
	case 1, 3, 5:
		g.dr[which] = uint32(int32(int16(v)))
	case 7:
		g.setOTZ(uint16(v))
	case 8, 9, 10, 11:
		g.setIR(which-8, int32(v))
	case 14:
		g.dr[14] = v
		g.dr[15] = v
	case 15:
		g.dr[15] = v
		g.dr[12] = g.dr[13]
		g.dr[13] = g.dr[14]
		g.dr[14] = g.dr[15]
	case 16, 17, 18, 19:
		g.setZFIFO(which-16, int32(v))
	case 28:
		g.setIR(1, int32((v>>0)&0x1F)<<7)
		g.setIR(2, int32((v>>5)&0x1F)<<7)
		g.setIR(3, int32((v>>10)&0x1F)<<7)
	case 29, 31: // 唯讀
	case 30:
		g.dr[30] = v
		g.dr[31] = uint32(bits.LeadingZeros32(v ^ (0 - (v >> 31))))
	default:
		g.dr[which] = v
	}
}

func sat5(v int32) uint32 {
	if v < 0 {
		return 0
	}
	if v > 0x1F {
		return 0x1F
	}
	return uint32(v)
}

func (g *GTE) readDR(which int) uint32 {
	switch which {
	case 1, 3, 5, 8, 9, 10, 11:
		return uint32(int32(int16(g.dr[which])))
	case 7, 16, 17, 18, 19:
		return uint32(uint16(g.dr[which]))
	case 28, 29:
		return sat5(g.ir(1)>>7) | sat5(g.ir(2)>>7)<<5 | sat5(g.ir(3)>>7)<<10
	default:
		return g.dr[which]
	}
}

// ---- 飽和與旗標 ----

func signExtend44(v int64) int64 { return (v << 20) >> 20 }

// aMV 是 MVMVA 系列累加時的 44 位元溢位檢查。
func (g *GTE) aMV(which int, v int64) int64 {
	if v >= 1<<43 {
		g.flags |= 1 << (30 - which)
	}
	if v < -(1 << 43) {
		g.flags |= 1 << (27 - which)
	}
	return signExtend44(v)
}

// i64ToI44 與 aMV 相同，但正向門檻是 2^43-1（照參照實作）。
func (g *GTE) i64ToI44(which int, v int64) int64 {
	if v >= 0x7ffffffffff {
		g.flags |= 1 << (30 - which)
	}
	if v < -0x80000000000 {
		g.flags |= 1 << (27 - which)
	}
	return signExtend44(v)
}

// f 是 MAC0 的 32 位元溢位檢查。
func (g *GTE) f(v int64) int64 {
	if v < -2147483648 {
		g.flags |= 1 << 15
	}
	if v > 2147483647 {
		g.flags |= 1 << 16
	}
	return v
}

// limB 把值飽和到 IR 的範圍；lm 時下限為 0。
func (g *GTE) limB(which int, v int32, lm bool) int32 {
	lo := int32(-32768)
	if lm {
		lo = 0
	}
	if v < lo {
		g.flags |= 1 << (24 - which)
		return lo
	}
	if v > 32767 {
		g.flags |= 1 << (24 - which)
		return 32767
	}
	return v
}

// limBPTZ 是 RTPS／RTPT 的 IR3：旗標看未位移的 ftv，飽和看 value。
func (g *GTE) limBPTZ(which int, v, ftv int32, lm bool) int32 {
	lo := int32(-32768)
	if lm {
		lo = 0
	}
	if ftv < -32768 || ftv > 32767 {
		g.flags |= 1 << (24 - which)
	}
	if v < lo {
		v = lo
	}
	if v > 32767 {
		v = 32767
	}
	return v
}

func (g *GTE) limC(which int, v int32) uint32 {
	if v&^0xFF != 0 {
		g.flags |= 1 << (21 - which)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
	}
	return uint32(v)
}

// limD 飽和 SZ3／OTZ。unchained 為 false 時，先看 MAC0 的溢位旗標。
func (g *GTE) limD(v int32, unchained bool) int32 {
	if !unchained {
		if g.flags&(1<<15) != 0 {
			g.flags |= 1 << 18
			return 0
		}
		if g.flags&(1<<16) != 0 {
			g.flags |= 1 << 18
			return 0xFFFF
		}
	}
	if v < 0 {
		g.flags |= 1 << 18
		return 0
	}
	if v > 65535 {
		g.flags |= 1 << 18
		return 65535
	}
	return v
}

func (g *GTE) limG(which int, v int32) int32 {
	if v < -1024 {
		g.flags |= 1 << (14 - which)
		return -1024
	}
	if v > 1023 {
		g.flags |= 1 << (14 - which)
		return 1023
	}
	return v
}

// limH 飽和 IR0 到 0..4096（上限是 4096，不是 4095）。
func (g *GTE) limH(v int32) int32 {
	if v < 0 {
		g.flags |= 1 << 12
		return 0
	}
	if v > 4096 {
		g.flags |= 1 << 12
		return 4096
	}
	return v
}

func (g *GTE) macToIR(lm bool) {
	g.setIR(1, g.limB(0, g.mac(1), lm))
	g.setIR(2, g.limB(1, g.mac(2), lm))
	g.setIR(3, g.limB(2, g.mac(3), lm))
}

func (g *GTE) macToRGBFIFO() {
	packed := g.limC(0, g.mac(1)>>4) | g.limC(1, g.mac(2)>>4)<<8 |
		g.limC(2, g.mac(3)>>4)<<16 | g.rgbCD()<<24
	g.dr[20] = g.dr[21]
	g.dr[21] = g.dr[22]
	g.dr[22] = packed
}

// ---- 矩陣運算 ----

// matrixRow 回傳矩陣 mx 第 row 列的三個係數。
func (g *GTE) matrixRow(mx, row int) [3]int32 {
	if mx == mxReserved {
		if row == 0 {
			return [3]int32{-(g.rgbR() << 4), g.rgbR() << 4, g.ir(0)}
		}
		c := lo16(g.cr[row])
		return [3]int32{c, c, c}
	}
	m := mx * 8
	switch row {
	case 0:
		return [3]int32{lo16(g.cr[m]), hi16(g.cr[m]), lo16(g.cr[m+1])}
	case 1:
		return [3]int32{hi16(g.cr[m+1]), lo16(g.cr[m+2]), hi16(g.cr[m+2])}
	default:
		return [3]int32{lo16(g.cr[m+3]), hi16(g.cr[m+3]), lo16(g.cr[m+4])}
	}
}

func (g *GTE) mulVector(v int) [3]int32 {
	if v == vecIR {
		return [3]int32{g.ir(1), g.ir(2), g.ir(3)}
	}
	return [3]int32{g.vector(v, 0), g.vector(v, 1), g.vector(v, 2)}
}

func (g *GTE) translation(cv, i int) int64 {
	if cv == cvNone {
		return 0
	}
	return int64(int32(g.cr[cv*8+crTR+i])) << 12
}

func (g *GTE) multiplyMatrixByVector(mx, v, cv int, sf uint, lm bool) {
	vec := g.mulVector(v)
	for i := 0; i < 3; i++ {
		row := g.matrixRow(mx, i)
		mul := [3]int32{row[0] * vec[0], row[1] * vec[1], row[2] * vec[2]}
		tmp := g.translation(cv, i)
		tmp = g.aMV(i, tmp+int64(mul[0]))
		if cv == cvFC {
			// 遠景色向量的硬體缺陷：第一項只留下旗標，結果被丟掉。
			g.limB(i, int32(tmp>>sf), false)
			tmp = 0
		}
		tmp = g.aMV(i, tmp+int64(mul[1]))
		tmp = g.aMV(i, tmp+int64(mul[2]))
		g.setMAC(1+i, int32(tmp>>sf))
	}
	g.macToIR(lm)
}

// multiplyMatrixByVectorPT 是 RTPS／RTPT 的透視轉換前半段。
func (g *GTE) multiplyMatrixByVectorPT(v int, sf uint, lm bool) {
	var tmp [3]int64
	for i := 0; i < 3; i++ {
		row := g.matrixRow(mxRotation, i)
		tmp[i] = g.translation(cvTR, i)
		tmp[i] = g.aMV(i, tmp[i]+int64(row[0]*g.vector(v, 0)))
		tmp[i] = g.aMV(i, tmp[i]+int64(row[1]*g.vector(v, 1)))
		tmp[i] = g.aMV(i, tmp[i]+int64(row[2]*g.vector(v, 2)))
		g.setMAC(1+i, int32(tmp[i]>>sf))
	}
	g.setIR(1, g.limB(0, g.mac(1), lm))
	g.setIR(2, g.limB(1, g.mac(2), lm))
	g.setIR(3, g.limBPTZ(2, g.mac(3), int32(tmp[2]>>12), lm))

	g.dr[16] = g.dr[17]
	g.dr[17] = g.dr[18]
	g.dr[18] = g.dr[19]
	g.setZFIFO(3, g.limD(int32(tmp[2]>>12), true))
}

func calcRecip(divisor uint16) int32 {
	x := int32(0x101 + int32(unrTable[((uint32(divisor)&0x7FFF)+0x40)>>7]))
	tmp := ((int32(divisor) * -x) + 0x80) >> 8
	return ((x * (131072 + tmp)) + 0x80) >> 8
}

// divide 是 H／SZ3 的 UNR 除法，結果飽和到 17 位元。
func (g *GTE) divide(dividend, divisor uint32) uint32 {
	if divisor*2 > dividend {
		shift := uint(bits.LeadingZeros16(uint16(divisor)))
		dividend <<= shift
		divisor <<= shift
		r := uint32((uint64(dividend)*uint64(int64(calcRecip(uint16(divisor|0x8000)))) + 32768) >> 16)
		if r > 0x1FFFF {
			r = 0x1FFFF
		}
		return r
	}
	g.flags |= 1 << 17
	return 0x1FFFF
}

func (g *GTE) transformXY(hDivSZ int64) {
	g.setMAC(0, int32(g.f(int64(int32(g.cr[crOFX]))+int64(g.ir(1))*hDivSZ)>>16))
	x := g.limG(0, g.mac(0))
	g.setMAC(0, int32(g.f(int64(int32(g.cr[crOFY]))+int64(g.ir(2))*hDivSZ)>>16))
	y := g.limG(1, g.mac(0))
	g.dr[15] = uint32(uint16(x)) | uint32(uint16(y))<<16
	g.dr[12] = g.dr[13]
	g.dr[13] = g.dr[14]
	g.dr[14] = g.dr[15]
}

func (g *GTE) transformDQ(hDivSZ int64) {
	dqa := int64(int16(g.cr[crDQA]))
	dqb := int64(int32(g.cr[crDQB]))
	g.setMAC(0, int32(g.f(dqb+dqa*hDivSZ)))
	g.setIR(0, g.limH(int32((dqb+dqa*hDivSZ)>>12)))
}

// ---- 命令 ----

type fields struct {
	sf    uint // 0 或 12
	mx, v int
	cv    int
	lm    bool
}

func decode(cmd uint32) fields {
	f := fields{
		mx: int(cmd>>17) & 3,
		v:  int(cmd>>15) & 3,
		cv: int(cmd>>13) & 3,
		lm: (cmd>>10)&1 != 0,
	}
	if cmd&(1<<19) != 0 {
		f.sf = 12
	}
	return f
}

// Execute 執行一個 COP2 命令。只看低 25 位。回傳參照實作的週期數。
func (g *GTE) Execute(cmd uint32) int {
	g.flags = 0
	cycles := g.execute(cmd&0x3F, decode(cmd))
	if g.flags&flagErrorMask != 0 {
		g.flags |= 1 << 31
	}
	g.cr[crFLAG] = g.flags
	return cycles
}

func (g *GTE) execute(op uint32, f fields) int {
	switch op {
	case 0x00, 0x01:
		g.rtp(0, f, true)
		return 15
	case 0x06:
		g.nclip()
		return 8
	case 0x0C:
		g.op(f)
		return 6
	case 0x10:
		g.dpcs(f, false)
		return 8
	case 0x11:
		g.intpl(f)
		return 8
	case 0x12:
		g.multiplyMatrixByVector(f.mx, f.v, f.cv, f.sf, f.lm)
		return 8
	case 0x13:
		g.ncd(0, f)
		return 19
	case 0x14:
		g.multiplyMatrixByVector(mxColor, vecIR, cvBK, f.sf, f.lm)
		g.dcpl(f)
		return 13
	case 0x16:
		g.ncd(0, f)
		g.ncd(1, f)
		g.ncd(2, f)
		return 44
	case 0x1A, 0x29:
		g.dcpl(f)
		return 8
	case 0x1B:
		g.ncc(0, f)
		return 17
	case 0x1C:
		g.cc(f)
		return 11
	case 0x1E:
		g.nc(0, f)
		return 14
	case 0x20:
		g.nc(0, f)
		g.nc(1, f)
		g.nc(2, f)
		return 30
	case 0x28:
		g.sqr(f)
		return 5
	case 0x2A:
		g.dpcs(f, true)
		g.dpcs(f, true)
		g.dpcs(f, true)
		return 17
	case 0x2D:
		g.avsz(g.zfifo(1)+g.zfifo(2)+g.zfifo(3), int64(int16(g.cr[crZSF3])))
		return 5
	case 0x2E:
		g.avsz(g.zfifo(0)+g.zfifo(1)+g.zfifo(2)+g.zfifo(3), int64(int16(g.cr[crZSF4])))
		return 5
	case 0x30:
		g.rtp(0, f, false)
		g.rtp(1, f, false)
		g.rtp(2, f, true)
		return 23
	case 0x3D:
		g.gpf(f)
		return 5
	case 0x3E:
		g.gpl(f)
		return 5
	case 0x3F:
		g.ncc(0, f)
		g.ncc(1, f)
		g.ncc(2, f)
		return 39
	}
	return 1
}

// rtp 是 RTPS／RTPT 的一個頂點；dq 為真時另算深度提示。
func (g *GTE) rtp(v int, f fields, dq bool) {
	g.multiplyMatrixByVectorPT(v, f.sf, f.lm)
	h := int64(g.divide(uint32(uint16(g.cr[crH])), g.zfifo(3)))
	g.transformXY(h)
	if dq {
		g.transformDQ(h)
	}
}

func (g *GTE) nclip() {
	x0, y0 := lo16(g.dr[12]), hi16(g.dr[12])
	x1, y1 := lo16(g.dr[13]), hi16(g.dr[13])
	x2, y2 := lo16(g.dr[14]), hi16(g.dr[14])
	sum := int64(x0)*int64(y1-y2) + int64(x1)*int64(y2-y0) + int64(x2)*int64(y0-y1)
	g.setMAC(0, int32(g.f(sum)))
}

func (g *GTE) op(f fields) {
	ir1, ir2, ir3 := g.ir(1), g.ir(2), g.ir(3)
	r0, r1, r2 := lo16(g.cr[0]), lo16(g.cr[2]), lo16(g.cr[4])
	g.setMAC(1, (r1*ir3-r2*ir2)>>f.sf)
	g.setMAC(2, (r2*ir1-r0*ir3)>>f.sf)
	g.setMAC(3, (r0*ir2-r1*ir1)>>f.sf)
	g.macToIR(f.lm)
}

func (g *GTE) sqr(f fields) {
	for i := 1; i < 4; i++ {
		ir := g.ir(i)
		g.setMAC(i, (ir*ir)>>f.sf)
	}
	g.macToIR(f.lm)
}

func (g *GTE) avsz(sum uint32, zsf int64) {
	avg := zsf * int64(sum)
	g.f(avg)
	g.setMAC(0, int32(avg))
	g.setOTZ(uint16(g.limD(g.mac(0)>>12, false)))
}

// depthCue 是 DPCS／DPCT／DCPL／INTPL 共用的「往遠景色內插」：
// MAC = base + IR0 × sat(FC − base)，base 以 <<12 的定點表示。
func (g *GTE) depthCue(base [3]int64, f fields) {
	for i := 0; i < 3; i++ {
		fc := int64(int32(g.cr[crFC+i])) << 12
		g.setMAC(1+i, int32(g.i64ToI44(i, fc-base[i])>>f.sf))
		g.setMAC(1+i, int32(g.i64ToI44(i, base[i]+int64(g.ir(0)*g.limB(i, g.mac(1+i), false)))>>f.sf))
	}
	g.macToIR(f.lm)
	g.macToRGBFIFO()
}

// dpcs 用 RGBC（fifo 為假）或 RGB FIFO 的第一筆（fifo 為真，DPCT）當基色。
func (g *GTE) dpcs(f fields, fifo bool) {
	c := g.dr[6]
	if fifo {
		c = g.dr[20]
	}
	var base [3]int64
	for i := 0; i < 3; i++ {
		base[i] = int64(int32(uint8(c>>(8*i)))<<4) << 12
	}
	g.depthCue(base, f)
}

func (g *GTE) dcpl(f fields) {
	rgb := [3]int32{g.rgbR() << 4, g.rgbG() << 4, g.rgbB() << 4}
	var base [3]int64
	for i := 0; i < 3; i++ {
		base[i] = int64(rgb[i] * g.ir(1+i))
	}
	g.depthCue(base, f)
}

func (g *GTE) intpl(f fields) {
	var base [3]int64
	for i := 0; i < 3; i++ {
		base[i] = int64(g.ir(1+i)) << 12
	}
	g.depthCue(base, f)
}

func (g *GTE) colorTimesIR(f fields) {
	g.setMAC(1, ((g.rgbR()<<4)*g.ir(1))>>f.sf)
	g.setMAC(2, ((g.rgbG()<<4)*g.ir(2))>>f.sf)
	g.setMAC(3, ((g.rgbB()<<4)*g.ir(3))>>f.sf)
	g.macToIR(f.lm)
	g.macToRGBFIFO()
}

func (g *GTE) lightAndColor(v int, f fields) {
	g.multiplyMatrixByVector(mxLight, v, cvNone, f.sf, f.lm)
	g.multiplyMatrixByVector(mxColor, vecIR, cvBK, f.sf, f.lm)
}

func (g *GTE) nc(v int, f fields) {
	g.lightAndColor(v, f)
	g.macToRGBFIFO()
}

func (g *GTE) ncc(v int, f fields) {
	g.lightAndColor(v, f)
	g.colorTimesIR(f)
}

func (g *GTE) ncd(v int, f fields) {
	g.lightAndColor(v, f)
	g.dcpl(f)
}

func (g *GTE) cc(f fields) {
	g.multiplyMatrixByVector(mxColor, vecIR, cvBK, f.sf, f.lm)
	g.colorTimesIR(f)
}

func (g *GTE) gpf(f fields) {
	g.setMAC(1, (g.ir(0)*g.ir(1))>>f.sf)
	g.setMAC(2, (g.ir(0)*g.ir(2))>>f.sf)
	g.setMAC(3, (g.ir(0)*g.ir(3))>>f.sf)
	g.macToIR(f.lm)
	g.macToRGBFIFO()
}

func (g *GTE) gpl(f fields) {
	for i := 1; i < 4; i++ {
		v := (int64(g.mac(i)) << f.sf) + int64(g.ir(0)*g.ir(i))
		g.setMAC(i, int32(g.i64ToI44(i-1, v)>>f.sf))
	}
	g.macToIR(f.lm)
	g.macToRGBFIFO()
}
