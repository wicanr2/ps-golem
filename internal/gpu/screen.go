package gpu

import "image"

// Screen 依顯示設定把目前顯示的區域取成 RGB 影像（docs/spec/007-gpu-and-dma.md §8）。
func (g *GPU) Screen() *image.RGBA { return g.ScreenInto(nil) }

// ScreenInto 同 Screen，dst 尺寸相同時重用它的記憶體（前端每個 field 呼叫，避免配置）。
func (g *GPU) ScreenInto(dst *image.RGBA) *image.RGBA {
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
	img := dst
	if img == nil || img.Bounds() != image.Rect(0, 0, int(w), int(h)) {
		img = image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	}
	rgb24 := g.displayMode&0x10 != 0
	for y := uint32(0); y < h; y++ {
		vy := (g.displayYStart + y) & (VRAMHeight - 1)
		row := g.vram[vy*VRAMWidth : (vy+1)*VRAMWidth]
		for x := uint32(0); x < w; x++ {
			o := img.PixOffset(int(x), int(y))
			if rgb24 {
				b := g.displayXStart*2 + x*3 // 位元組位移
				for i := uint32(0); i < 3; i++ {
					p := b + i
					word := row[(p/2)&(VRAMWidth-1)]
					img.Pix[o+int(i)] = uint8(word >> (8 * (p & 1)))
				}
			} else {
				v := row[(g.displayXStart+x)&(VRAMWidth-1)]
				r, gg, bb := v&31, (v>>5)&31, (v>>10)&31
				img.Pix[o] = uint8(r<<3 | r>>2)
				img.Pix[o+1] = uint8(gg<<3 | gg>>2)
				img.Pix[o+2] = uint8(bb<<3 | bb>>2)
			}
			img.Pix[o+3] = 0xFF
		}
	}
	return img
}
