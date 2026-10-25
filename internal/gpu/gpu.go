// Package gpu 實作 PlayStation GPU 的原生解析度軟體光柵化。
//
// 規格：docs/spec/007-gpu-and-dma.md。
//
// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c（commit e57860d82faa953dc0fc10f201fd583739bc2f7d），
// GPL-2.0-or-later。只搬原生解析度路徑；放大、PGXP、硬體算繪介面與遊戲別的
// UV 修正（Calc_UVOffsets_Adjust_Verts）不搬。
package gpu

import "fmt"

// VRAM 尺寸。
const (
	VRAMWidth  = 1024
	VRAMHeight = 512
)

// 正在進行中的多字命令。
const (
	inNone = iota
	inFBWrite
	inFBRead
	inPolyline
)

// 顯示模式位元（GP1(08)）。
const (
	dispInterlaced = 0x20
	dispPAL        = 0x08
)

// 一條掃描線的影像週期數與每個 field 的掃描線數（NTSC），psx-spx GPU 章。
const (
	videoPerLine = 3413
)

// GPU 是一顆 GPU 的完整狀態。零值不可用，請用 New。
type GPU struct {
	vram [VRAMWidth * VRAMHeight]uint16

	// 命令緩衝
	buf    [16]uint32
	bufLen int
	need   int // 目前命令需要的字數，0 表示還沒開始

	inCmd int
	// FB 讀寫的矩形與游標
	fbX, fbY, fbW, fbH uint32
	fbCurX, fbCurY     uint32
	// 折線
	plineCmd   uint32
	plinePrev  linePoint
	plineGroup [2]uint32
	plineN     int

	// 繪圖環境
	texPageX, texPageY uint32
	texMode            uint32
	abr                uint32
	spriteFlip         uint32
	dtd, dfe           bool
	tww, twh, twx, twy uint32
	twxAnd, twxAdd     uint32
	twyAnd, twyAdd     uint32
	clipX0, clipY0     int32
	clipX1, clipY1     int32
	offsX, offsY       int32
	maskSetOR          uint16
	maskEvalAND        uint16
	texDisable         bool
	texDisableAllow    bool

	// CLUT 與貼圖快取
	clutCache   [256]uint16
	clutCacheVB uint32 // 已載入的 CLUT 位址；^0 表示無效
	clutCacheN  uint32 // 已載入的筆數
	texCache    [256]texCacheLine

	// 顯示控制
	displayOff    bool
	dmaControl    uint32
	displayXStart uint32
	displayYStart uint32
	horizStart    uint32
	horizEnd      uint32
	vertStart     uint32
	vertEnd       uint32
	displayMode   uint32

	irqPending bool

	readBuffer uint32

	// 掃描
	lineClock       uint32
	scanline        uint32
	linesPerField   uint32
	field           bool
	fieldRAMReadout bool
	inVBlank        bool
	curYOffset      uint32
	curLineReadout  uint32

	ditherLUT [4][512][4]uint8

	// Trace 不為 nil 時，每個 GP0 字都先交給它（除錯用）。
	Trace  func(v uint32)
	shadow *Shadow
}

type texCacheLine struct {
	tag  uint32
	data [4]uint16
}

// New 建立上電狀態的 GPU。
func New() *GPU {
	g := &GPU{}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			for v := 0; v < 512; v++ {
				val := (v + int(ditherTable[y][x])) >> 3
				if val < 0 {
					val = 0
				}
				if val > 0x1F {
					val = 0x1F
				}
				g.ditherLUT[y][v][x] = uint8(val)
			}
		}
	}
	g.power()
	return g
}

var ditherTable = [4][4]int8{
	{-4, 0, -3, 1},
	{2, -2, 3, -1},
	{-3, 1, -4, 0},
	{3, -1, 2, -2},
}

func (g *GPU) power() {
	for i := range g.vram {
		g.vram[i] = 0
	}
	g.clutCacheVB = ^uint32(0)
	g.clutCacheN = 0
	for i := range g.texCache {
		g.texCache[i].tag = ^uint32(0)
	}
	g.readBuffer = 0
	g.inVBlank = true
	g.linesPerField = 263
	g.lineClock = 0
	g.scanline = 0
	g.field = false
	g.fieldRAMReadout = false
	g.softReset()
}

// softReset 是 GP1(00)。
func (g *GPU) softReset() {
	g.irqPending = false
	g.invalidateCache()
	g.dmaControl = 0
	g.bufLen, g.need = 0, 0
	g.inCmd = inNone
	g.displayOff = true
	g.displayXStart, g.displayYStart = 0, 0
	g.displayMode = 0
	g.horizStart, g.horizEnd = 0x200, 0xC00
	g.vertStart, g.vertEnd = 0x10, 0x100
	g.texPageX, g.texPageY = 0, 0
	g.spriteFlip = 0
	g.abr = 0
	g.texMode = 0
	g.dtd, g.dfe = false, false
	g.tww, g.twh, g.twx, g.twy = 0, 0, 0, 0
	g.recalcTexWindow()
	g.clipX0, g.clipY0, g.clipX1, g.clipY1 = 0, 0, 0, 0
	g.offsX, g.offsY = 0, 0
	g.maskSetOR, g.maskEvalAND = 0, 0
	g.texDisable = false
	g.texDisableAllow = false
}

func (g *GPU) invalidateTexCache() {
	for i := range g.texCache {
		g.texCache[i].tag = ^uint32(0)
	}
}

func (g *GPU) invalidateCache() {
	g.clutCacheVB = ^uint32(0)
	g.clutCacheN = 0
	g.invalidateTexCache()
}

// IRQ 回傳 GPU 中斷（GP0(1F)）是否成立。
func (g *GPU) IRQ() bool { return g.irqPending }

// ---- 暫存器 ----

// WriteGP1 處理控制命令。
func (g *GPU) WriteGP1(v uint32) {
	if g.shadow != nil {
		g.shadow.GP1(v)
	}
	cmd := v >> 24
	v &= 0x00FFFFFF
	switch cmd {
	case 0x00:
		g.softReset()
	case 0x01:
		g.bufLen, g.need = 0, 0
		g.inCmd = inNone
	case 0x02:
		g.irqPending = false
	case 0x03:
		g.displayOff = v&1 != 0
	case 0x04:
		g.dmaControl = v & 3
	case 0x05:
		g.displayXStart = v & 0x3FE
		g.displayYStart = (v >> 10) & 0x1FF
	case 0x06:
		g.horizStart = v & 0xFFF
		g.horizEnd = (v >> 12) & 0xFFF
	case 0x07:
		g.vertStart = v & 0x3FF
		g.vertEnd = (v >> 10) & 0x3FF
	case 0x08:
		g.displayMode = v & 0xFF
	case 0x09:
		g.texDisableAllow = v&1 != 0
	case 0x10:
		switch v & 0xF {
		case 0x2:
			g.readBuffer = g.readBuffer&0xFFF00000 | g.tww | g.twh<<5 | g.twx<<10 | g.twy<<15
		case 0x3:
			g.readBuffer = g.readBuffer&0xFFF00000 | uint32(g.clipY0)<<10 | uint32(g.clipX0)
		case 0x4:
			g.readBuffer = g.readBuffer&0xFFF00000 | uint32(g.clipY1)<<10 | uint32(g.clipX1)
		case 0x5:
			g.readBuffer = g.readBuffer&0xFFC00000 | uint32(g.offsX)&2047 | (uint32(g.offsY)&2047)<<11
		case 0x7:
			g.readBuffer = 2
		case 0x8:
			g.readBuffer = 0
		}
	}
}

// Status 回傳 GPUSTAT。
func (g *GPU) Status() uint32 {
	r := ((g.displayMode<<1)&0x7F | (g.displayMode>>6)&1) << 16
	r |= (g.displayMode & 0x80) << 7
	r |= g.dmaControl << 29
	r |= (g.curLineReadout & 1) << 31
	if !g.field {
		r |= 1 << 13
	}
	if g.dmaControl&2 != 0 {
		r |= 1 << 25
	}
	if g.irqPending {
		r |= 1 << 24
	}
	if g.displayOff {
		r |= 1 << 23
	}
	if g.inCmd == inNone && g.bufLen == 0 {
		r |= 1 << 26
	}
	if g.inCmd == inFBRead {
		r |= 1 << 27
	}
	if g.fifoReady() {
		r |= 1 << 28
	}
	r |= g.texPageX >> 6
	r |= g.texPageY >> 4
	r |= g.abr << 5
	r |= g.texMode << 7
	if g.dtd {
		r |= 1 << 9
	}
	if g.dfe {
		r |= 1 << 10
	}
	if g.maskSetOR != 0 {
		r |= 1 << 11
	}
	if g.maskEvalAND != 0 {
		r |= 1 << 12
	}
	if g.texDisable {
		r |= 1 << 15
	}
	return r
}

// fifoReady 是 GPUSTAT bit 28（GP0 寫入 FIFO 空），照 Beetle CalcFIFOReadyBit 的順序：
// 折線進行中為 0；FIFO 空就是 1（即使 VRAM 傳輸還在進行）；VRAM 讀寫中為 0；
// 其餘情況命令還沒收齊，FIFO 未滿，為 1。
func (g *GPU) fifoReady() bool {
	switch {
	case g.inCmd == inPolyline:
		return false
	case g.bufLen == 0:
		return true
	case g.inCmd == inFBRead || g.inCmd == inFBWrite:
		return false
	}
	return true
}

// ReadData 讀 GPUREAD（VRAM→CPU 傳輸中就讀 VRAM）。
func (g *GPU) ReadData() uint32 {
	if g.inCmd != inFBRead {
		return g.readBuffer
	}
	var r uint32
	for i := 0; i < 2; i++ {
		r |= uint32(g.texel(g.fbCurX&1023, g.fbCurY&511)) << (16 * i)
		g.fbCurX++
		if g.fbCurX == g.fbX+g.fbW {
			if g.fbCurY+1 == g.fbY+g.fbH {
				g.inCmd = inNone
			} else {
				g.fbCurY++
				g.fbCurX = g.fbX
			}
		}
	}
	g.readBuffer = r
	return r
}

// ---- 掃描 ----

// DotDivider 回傳點時脈的除數（影像時脈 ÷ 除數 = 點時脈），依 GP1(08) 的水平解析度。
func (g *GPU) DotDivider() uint32 {
	if g.displayMode&0x40 != 0 {
		return 7
	}
	return [4]uint32{10, 8, 5, 4}[g.displayMode&3]
}

// TickVideo 推進 n 個影像週期。回傳期間 VBlank 開始的次數與經過的掃描線數（給 timer1）。
func (g *GPU) TickVideo(n uint32) (vblankStarts, lines int) {
	g.lineClock += n
	for g.lineClock >= videoPerLine {
		g.lineClock -= videoPerLine
		lines++
		if g.endOfLine() {
			vblankStarts++
		}
	}
	return
}

// endOfLine 照 Beetle gpu.c 的掃描線更新；回傳這一線是否進入 VBlank。
func (g *GPU) endOfLine() bool {
	if !g.inVBlank {
		g.curYOffset++
	}
	g.scanline++
	if g.scanline >= g.linesPerField {
		g.scanline = 0
	}
	if g.scanline == g.linesPerField-1 {
		if g.displayMode&dispInterlaced != 0 {
			g.field = !g.field
		} else {
			g.field = false
		}
	}
	if g.scanline == 0 {
		if g.displayMode&dispInterlaced != 0 {
			if g.displayMode&dispPAL != 0 {
				g.linesPerField = 313 - b2u(g.field)
			} else {
				g.linesPerField = 263 - b2u(g.field)
			}
		} else {
			g.field = false
			if g.displayMode&dispPAL != 0 {
				g.linesPerField = 314
			} else {
				g.linesPerField = 263
			}
		}
	}
	started := false
	if g.scanline == g.vertEnd && !g.inVBlank {
		g.inVBlank = true
		started = true
		g.curYOffset = 0
		oldField := g.fieldRAMReadout
		if g.displayMode&0x24 == 0x24 {
			g.fieldRAMReadout = !g.field
		} else {
			g.fieldRAMReadout = false
		}
		if g.shadow != nil && oldField != g.fieldRAMReadout {
			g.shadow.Field(g.fieldRAMReadout)
		}
	}
	if g.scanline == g.vertStart && g.inVBlank {
		g.inVBlank = false
	}
	off := g.curYOffset
	if g.displayMode&0x24 == 0x24 {
		off = g.curYOffset << 1
		if !g.inVBlank && g.fieldRAMReadout {
			off++
		}
	}
	g.curLineReadout = (g.displayYStart + off) & 0x1FF
	return started
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// lineSkip：隔行 480 線且不畫顯示中的 field 時跳過（Beetle LineSkipTest）。
func (g *GPU) lineSkip(y uint32) bool {
	if g.displayMode&0x24 != 0x24 || g.dfe {
		return false
	}
	return y&1 == (g.displayYStart+b2u(g.fieldRAMReadout))&1
}

// ---- VRAM ----

func (g *GPU) texel(x, y uint32) uint16 { return g.vram[(y&511)<<10|(x&1023)] }

func (g *GPU) put(x, y uint32, v uint16) { g.vram[(y&511)<<10|(x&1023)] = v }

// DebugState 回傳命令處理的內部狀態（除錯用）。
func (g *GPU) DebugState() string {
	return fmt.Sprintf("inCmd=%d bufLen=%d need=%d buf0=%08x fb=(%d,%d %dx%d) cur=(%d,%d)",
		g.inCmd, g.bufLen, g.need, g.buf[0], g.fbX, g.fbY, g.fbW, g.fbH, g.fbCurX, g.fbCurY)
}

// VideoToNextLine 回傳到下一條掃描線結束還差多少影像週期（機器據此決定何時同步，008）。
func (g *GPU) VideoToNextLine() uint32 { return videoPerLine - g.lineClock }
