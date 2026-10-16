// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

// 混色模式（abr），-1 表示不透明。
const blendOpaque = -1

// blend 依 Beetle gpu_common.h PlotPixelBlend。
func blend(mode int, bg, fore uint16) uint16 {
	switch mode {
	case 0: // 平均
		b := uint32(bg | 0x8000)
		f := uint32(fore)
		return uint16(((f + b) - ((f ^ b) & 0x0421)) >> 1)
	case 1: // 相加
		b := uint32(bg &^ 0x8000)
		f := uint32(fore)
		sum := f + b
		carry := (sum - ((f ^ b) & 0x8421)) & 0x8420
		return uint16((sum - carry) | (carry - (carry >> 5)))
	case 2: // 相減
		b := uint32(bg | 0x8000)
		f := uint32(fore &^ 0x8000)
		diff := b - f + 0x108420
		borrow := (diff - ((b ^ f) & 0x108420)) & 0x108420
		return uint16((diff - borrow) & (borrow - (borrow >> 5)))
	case 3: // 加四分之一
		b := uint32(bg &^ 0x8000)
		f := uint32((fore>>2)&0x1CE7 | 0x8000)
		sum := f + b
		carry := (sum - ((f ^ b) & 0x8421)) & 0x8420
		return uint16((sum - carry) | (carry - (carry >> 5)))
	}
	return fore
}

// plot 畫一個像素（Beetle PlotPixel／PlotNativePixel）。
// textured 為假時寫入去掉 bit 15；mask 檢查依 maskEvalAND。
func (g *Renderer) plot(x, y int32, fore uint16, mode int, textured bool) {
	ux, uy := uint32(x)&1023, uint32(y)&511
	bg := g.texel(ux, uy)
	if mode >= 0 && fore&0x8000 != 0 {
		fore = blend(mode, bg, fore)
	}
	if g.maskEvalAND != 0 && bg&0x8000 != 0 {
		return
	}
	if !textured {
		fore &= 0x7FFF
	}
	g.put(ux, uy, fore|g.maskSetOR)
}

// updateCLUT 載入 CLUT 快取。tm 為 2 以上（15 位元）時不用 CLUT。
//
// 重載條件與 Beetle 不同：Beetle 在貼圖模式改變時一律重載；這裡依 ps1-tests
// gpu/clut-cache 的參考圖，只在 CLUT 位址不同、或已載入的筆數不夠（4 位元要 16 筆、
// 8 位元要 256 筆）時重載。8 位元切 4 位元不重載（同測試第 152 列），4 位元切 8 位元、
// 15 位元或保留模式切回 CLUT 模式要重載。
func (g *Renderer) updateCLUT(raw uint32, tm uint32) {
	if tm >= 2 {
		return
	}
	need := uint32(16)
	if tm == 1 {
		need = 256
	}
	addr := raw & 0x7FFF
	if g.clutCacheVB == addr && g.clutCacheN >= need {
		return
	}
	y := (raw >> 6) & 0x1FF
	cxo := (raw & 0x3F) << 4
	for i := uint32(0); i < need; i++ {
		g.clutCache[i] = g.texel((cxo+i)&0x3FF, y)
	}
	g.clutCacheVB = addr
	g.clutCacheN = need
}

// getTexel 取貼圖像素（Beetle GetTexel，含貼圖快取）。
func (g *Renderer) getTexel(tm uint32, u, v uint32) uint16 {
	if tm > 2 {
		tm = 2
	}
	uExt := (u & g.twxAnd) + g.twxAdd
	fx := (uExt >> (2 - tm)) & 1023
	fy := (v & g.twyAnd) + g.twyAdd
	gro := fy*1024 + fx
	line := gro &^ 3
	var c *texCacheLine
	switch tm {
	case 0:
		c = &g.texCache[(gro>>2)&0x3|(gro>>8)&0xFC]
	default:
		c = &g.texCache[(gro>>2)&0x7|(gro>>7)&0xF8]
	}
	if c.tag != line {
		cx := fx &^ 3
		for i := uint32(0); i < 4; i++ {
			c.data[i] = g.texel(cx+i, fy)
		}
		c.tag = line
	}
	w := c.data[gro&3]
	switch tm {
	case 0:
		return g.clutCache[(w>>((uExt&3)*4))&0xF]
	case 1:
		return g.clutCache[(w>>((uExt&1)*8))&0xFF]
	}
	return w
}

// modTexel 以頂點色調變貼圖色；off 是抖色表 [y][*][x] 的固定兩維。
func (g *Renderer) modTexel(dy, dx uint32, t uint16, r, gg, b uint32) uint16 {
	ir := (uint32(t&0x1F) * r) >> 4
	ig := (uint32(t&0x3E0) * gg) >> 9
	ib := (uint32(t&0x7C00) * b) >> 14
	return t&0x8000 | uint16(g.ditherLUT[dy][ir][dx]) | uint16(g.ditherLUT[dy][ig][dx])<<5 | uint16(g.ditherLUT[dy][ib][dx])<<10
}

// plotScaled 對應 Beetle gpu_common.h 的 PlotPixel，座標已經放大。
func (g *Renderer) plotScaled(x, y int32, fore uint16, mode int, textured bool) {
	ux := uint32(x) & ((1024 << g.shift) - 1)
	uy := uint32(y) & ((512 << g.shift) - 1)
	idx := uy<<(10+g.shift) | ux
	bg := g.VRAM[idx]
	if mode >= 0 && fore&0x8000 != 0 {
		fore = blend(mode, bg, fore)
	}
	if bg&g.maskEvalAND != 0 {
		return
	}
	if !textured {
		fore &= 0x7fff
	}
	g.VRAM[idx] = fore | g.maskSetOR
}
