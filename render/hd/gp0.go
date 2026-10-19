// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"image"
)

// cmdLen 回傳 GP0 命令的字數（含命令字）。折線回傳第一段的長度。
func cmdLen(cmd uint32) int {
	switch {
	case cmd == 0x02:
		return 3
	case cmd >= 0x20 && cmd < 0x40:
		nv := 3
		if cmd&0x08 != 0 {
			nv = 4
		}
		per := 1
		if cmd&0x04 != 0 {
			per++
		}
		if cmd&0x10 != 0 {
			per++
		}
		g := 0
		if cmd&0x10 != 0 {
			g = 1
		}
		return 1 + nv*per - g
	case cmd >= 0x40 && cmd < 0x60:
		if cmd&0x10 != 0 {
			return 4
		}
		return 3
	case cmd >= 0x60 && cmd < 0x80:
		n := 2
		if cmd&0x04 != 0 {
			n++
		}
		if cmd&0x18 == 0 {
			n++
		}
		return n
	case cmd >= 0x80 && cmd < 0xA0:
		return 4
	case cmd >= 0xA0 && cmd < 0xE0:
		return 3
	}
	return 1
}

// WriteGP0 寫一個字到 GP0（CPU 或 DMA）。
func (g *Renderer) WriteGP0(v uint32) {
	p := g.nextPrecision
	g.nextPrecision = pgxp.Value{}
	if g.Trace != nil {
		g.Trace(v)
	}
	switch g.inCmd {
	case inFBWrite:
		g.fbWriteWord(v)
		return
	case inPolyline:
		g.polylineWord(v)
		return
	}
	if g.bufLen == 0 {
		g.need = cmdLen(v >> 24)
	}
	g.precise[g.bufLen] = p
	g.buf[g.bufLen] = v
	g.bufLen++
	if g.bufLen < g.need {
		return
	}
	cb := g.buf[:g.bufLen]
	g.bufLen, g.need = 0, 0
	g.execute(cb)
}

func (g *Renderer) execute(cb []uint32) {
	cmd := cb[0] >> 24
	if cmd == 0x02 || cmd >= 0x20 && cmd < 0x80 {
		g.prepareSides()
	}
	switch {
	case cmd == 0x01:
		g.invalidateCache()
	case cmd == 0x02:
		g.fbFill(cb)
	case cmd == 0x1F:
		g.irqPending = true
	case cmd >= 0x20 && cmd < 0x40:
		g.drawPolygon(cb)
	case cmd >= 0x40 && cmd < 0x60:
		g.drawLineCmd(cb)
	case cmd >= 0x60 && cmd < 0x80:
		g.drawSpriteCmd(cb)
	case cmd >= 0x80 && cmd < 0xA0:
		g.fbCopy(cb)
	case cmd >= 0xA0 && cmd < 0xC0:
		g.fbWriteStart(cb)
	case cmd >= 0xC0 && cmd < 0xE0:
		g.invalidateTexCache()
	case cmd == 0xE1:
		g.setTPage(cb[0])
		g.spriteFlip = cb[0] & 0x3000
		g.dtd = (cb[0]>>9)&1 != 0
		g.dfe = (cb[0]>>10)&1 != 0
	case cmd == 0xE2:
		g.tww = cb[0] & 0x1F
		g.twh = (cb[0] >> 5) & 0x1F
		g.twx = (cb[0] >> 10) & 0x1F
		g.twy = (cb[0] >> 15) & 0x1F
		g.recalcTexWindow()
	case cmd == 0xE3:
		g.clipX0 = int32(cb[0] & 1023)
		g.clipY0 = int32((cb[0] >> 10) & 1023)
	case cmd == 0xE4:
		g.clipX1 = int32(cb[0] & 1023)
		g.clipY1 = int32((cb[0] >> 10) & 1023)
	case cmd == 0xE5:
		g.offsX = signExtend11(cb[0] & 2047)
		g.offsY = signExtend11((cb[0] >> 11) & 2047)
	case cmd == 0xE6:
		g.maskSetOR = 0
		if cb[0]&1 != 0 {
			g.maskSetOR = 0x8000
		}
		g.maskEvalAND = 0
		if cb[0]&2 != 0 {
			g.maskEvalAND = 0x8000
		}
	}
}

func signExtend11(v uint32) int32 { return int32(v<<21) >> 21 }

func (g *Renderer) setTPage(w uint32) {
	newX := (w & 0xF) * 64
	newY := (w & 0x10) * 16
	newMode := (w >> 7) & 3
	g.abr = (w >> 5) & 3
	if (newMode == 0) != (g.texMode == 0) || newX != g.texPageX || newY != g.texPageY {
		g.invalidateTexCache()
	}
	// 「貼圖停用」位元：每次寫入貼圖頁都改寫，不允許時清成 0。
	// Beetle 在不允許時保留舊值；ps1-tests gpu/gp0-e1 的實機紀錄
	// （testTexturedPolygons、testUnsetAllowTextureDisableClearsBitAfterWrite）顯示會清掉。
	nd := g.texDisableAllow && (w>>11)&1 != 0
	if nd != g.texDisable {
		g.invalidateTexCache()
	}
	g.texDisable = nd
	g.texPageX, g.texPageY, g.texMode = newX, newY, newMode
	g.recalcTexWindow()
}

func (g *Renderer) recalcTexWindow() {
	tm := g.texMode
	if tm > 2 {
		tm = 2
	}
	g.twxAnd = ^(g.tww << 3)
	g.twxAdd = ((g.twx & g.tww) << 3) + (g.texPageX << (2 - tm))
	g.twyAnd = ^(g.twh << 3)
	g.twyAdd = ((g.twy & g.twh) << 3) + g.texPageY
}

// ---- VRAM 填色、複製、上傳、下載 ----

func (g *Renderer) fbFill(cb []uint32) {
	r := cb[0] & 0xFF
	gg := (cb[0] >> 8) & 0xFF
	b := (cb[0] >> 16) & 0xFF
	fill := uint16(r>>3 | (gg>>3)<<5 | (b>>3)<<10)
	destX := cb[1] & 0x3F0
	destY := (cb[1] >> 16) & 0x3FF
	w := ((cb[2] & 0x3FF) + 0xF) &^ 0xF
	h := (cb[2] >> 16) & 0x1FF
	if g.margin != 0 {
		g.fillSides(destX, destY, w, h, fill)
	}
	for y := uint32(0); y < h; y++ {
		dy := (y + destY) & 511
		if g.lineSkip(dy) {
			continue
		}
		for x := uint32(0); x < w; x++ {
			g.put((x+destX)&1023, dy, fill)
		}
	}
}

func (g *Renderer) fbCopy(cb []uint32) {
	if g.raster != nil {
		g.syncRaster()
		g.rasterCPUWrite(image.Rect(0, 0, 1024<<g.shift, 512<<g.shift))
	}
	sx := cb[1] & 0x3FF
	sy := (cb[1] >> 16) & 0x3FF
	dx := cb[2] & 0x3FF
	dy := (cb[2] >> 16) & 0x3FF
	w := cb[3] & 0x3FF
	h := (cb[3] >> 16) & 0x1FF
	if w == 0 {
		w = 0x400
	}
	if h == 0 {
		h = 0x200
	}
	g.invalidateTexCache()
	g.copyTextureOrigins(sx, sy, dx, dy, w, h)
	sx <<= g.shift
	sy <<= g.shift
	dx <<= g.shift
	dy <<= g.shift
	w <<= g.shift
	h <<= g.shift
	stride := uint32(1024) << g.shift
	maskY := (uint32(512) << g.shift) - 1
	chunk := uint32(128) << g.shift
	var tmp [1024]uint16
	for y := uint32(0); y < h; y++ {
		for x := uint32(0); x < w; x += chunk {
			n := min(chunk, w-x)
			for c := uint32(0); c < n; c++ {
				tmp[c] = g.VRAM[((y+sy)&maskY)*stride+((x+c+sx)&(stride-1))]
			}
			for c := uint32(0); c < n; c++ {
				i := ((y+dy)&maskY)*stride + ((x + c + dx) & (stride - 1))
				if g.VRAM[i]&g.maskEvalAND == 0 {
					g.VRAM[i] = tmp[c] | g.maskSetOR
				}
			}
		}
	}
}

func (g *Renderer) fbWriteStart(cb []uint32) {
	g.fbX = cb[1] & 0x3FF
	g.fbY = (cb[1] >> 16) & 0x3FF
	g.fbW = cb[2] & 0x3FF
	g.fbH = (cb[2] >> 16) & 0x1FF
	if g.fbW == 0 {
		g.fbW = 0x400
	}
	if g.fbH == 0 {
		g.fbH = 0x200
	}
	g.fbCurX, g.fbCurY = g.fbX, g.fbY
	g.invalidateTexCache()
	g.inCmd = inFBWrite
	g.startTextureUpload()
}

func (g *Renderer) fbWriteWord(v uint32) {
	for i := 0; i < 2; i++ {
		x, y := g.fbCurX&1023, g.fbCurY&511
		accepted := g.texel(x, y)&g.maskEvalAND == 0
		if accepted {
			g.put(x, y, uint16(v)|g.maskSetOR)
		}
		g.observeTextureWord(uint16(v), accepted && uint16(v)|g.maskSetOR == uint16(v))
		g.fbCurX++
		if g.fbCurX == g.fbX+g.fbW {
			g.fbCurX = g.fbX
			g.fbCurY++
			if g.fbCurY == g.fbY+g.fbH {
				g.finishTextureUpload()
				g.inCmd = inNone
				return
			}
		}
		v >>= 16
	}
}

func (g *Renderer) fbReadStart(cb []uint32) {
	g.fbX = cb[1] & 0x3FF
	g.fbY = (cb[1] >> 16) & 0x3FF
	g.fbW = cb[2] & 0x3FF
	g.fbH = (cb[2] >> 16) & 0x3FF
	if g.fbW == 0 {
		g.fbW = 0x400
	}
	if g.fbH > 0x200 {
		g.fbH &= 0x1FF
	}
	g.fbCurX, g.fbCurY = g.fbX, g.fbY
	g.invalidateTexCache()
	if g.fbW != 0 && g.fbH != 0 {
		g.inCmd = inFBRead
	}
}
