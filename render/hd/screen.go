// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

import "image"

// Screen 依顯示設定把目前顯示的區域取成 RGB 影像（docs/spec/007-gpu-and-dma.md §8）。
func (g *Renderer) Screen() *image.RGBA {
	hres := [4]uint32{256, 320, 512, 640}[g.displayMode&3]
	if g.displayMode&0x40 != 0 {
		hres = 368
	}
	w := hres
	if g.horizEnd > g.horizStart {
		if cw := ((g.horizEnd-g.horizStart)/g.DotDivider() + 2) &^ 3; cw > 0 && cw <= hres {
			w = cw
		}
	}
	h := uint32(240)
	if g.vertEnd > g.vertStart {
		h = g.vertEnd - g.vertStart
	}
	if g.displayMode&0x04 != 0 && g.displayMode&dispInterlaced != 0 {
		h *= 2
	}
	w <<= g.shift
	h <<= g.shift
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	rgb24 := g.displayMode&0x10 != 0
	for y := uint32(0); y < h; y++ {
		vy := ((g.displayYStart << g.shift) + y) & ((VRAMHeight << g.shift) - 1)
		stride := uint32(VRAMWidth) << g.shift
		row := g.VRAM[vy*stride : (vy+1)*stride]
		for x := uint32(0); x < w; x++ {
			o := img.PixOffset(int(x), int(y))
			if rgb24 {
				b := g.displayXStart*2 + (x>>g.shift)*3 // 原生位元組位移，MDEC 不插值
				for i := uint32(0); i < 3; i++ {
					p := b + i
					word := row[((p/2)&(VRAMWidth-1))<<g.shift]
					img.Pix[o+int(i)] = uint8(word >> (8 * (p & 1)))
				}
			} else {
				v := row[((g.displayXStart<<g.shift)+x)&(stride-1)]
				r, gg, bb := v&31, (v>>5)&31, (v>>10)&31
				img.Pix[o] = uint8(r<<3 | r>>2)
				img.Pix[o+1] = uint8(gg<<3 | gg>>2)
				img.Pix[o+2] = uint8(bb<<3 | bb>>2)
			}
			img.Pix[o+3] = 0xFF
		}
	}
	if g.margin != 0 {
		return g.wideScreen(img)
	}
	return img
}
