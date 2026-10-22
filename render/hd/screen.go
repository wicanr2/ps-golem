// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

import (
	"encoding/binary"
	"image"
)

// RGB5 展開沿用既有 (c<<3)|(c>>2)，預先計算後以一次 32 位元寫入輸出。
var screenRGBA = func() [32768]uint32 {
	var table [32768]uint32
	for v := range table {
		r, g, b := uint32(v&31), uint32((v>>5)&31), uint32((v>>10)&31)
		table[v] = (r<<3 | r>>2) | (g<<3|g>>2)<<8 | (b<<3|b>>2)<<16 | 0xff000000
	}
	return table
}()

// Screen 每次交出獨立影像。逐像素契約見 016 §8。
func (g *Renderer) Screen() *image.RGBA { return g.ScreenInto(nil) }

// ScreenInto 重用尺寸與 stride 相容的後備緩衝。呼叫端保證它不再被讀取。
func (g *Renderer) ScreenInto(dst *image.RGBA) *image.RGBA {
	g.flushSpriteRun()
	defer g.rasterFence()
	g.syncRaster()
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
	outputW := int(w)
	if g.margin != 0 {
		outputW = g.wideWidth(int(w))
	}
	rect := image.Rect(0, 0, outputW, int(h))
	img := dst
	if img == nil || img.Rect != rect || img.Stride != outputW*4 || len(img.Pix) != outputW*int(h)*4 {
		img = image.NewRGBA(rect)
	}
	xoff := (outputW - int(w)) / 2
	rgb24 := g.displayMode&0x10 != 0
	stride := uint32(VRAMWidth) << g.shift
	for y := uint32(0); y < h; y++ {
		vy := ((g.displayYStart << g.shift) + y) & ((VRAMHeight << g.shift) - 1)
		row := g.VRAM[vy*stride : (vy+1)*stride]
		pix := img.Pix[int(y)*img.Stride : (int(y)+1)*img.Stride]
		if g.margin != 0 {
			for x := 0; x < xoff; x++ {
				binary.LittleEndian.PutUint32(pix[x*4:][:4], 0xff000000)
			}
			for x := xoff + int(w); x < outputW; x++ {
				binary.LittleEndian.PutUint32(pix[x*4:][:4], 0xff000000)
			}
		}
		pix = pix[xoff*4 : (xoff+int(w))*4]
		if rgb24 {
			// 每個原生 RGB24 像素只解碼兩個字，水平子像素沿用相同顏色。
			// 不合併垂直列，24bpp 畫布的不同 HD 列仍可有不同內容。
			n := uint32(1) << g.shift
			for x := uint32(0); x < w/n; x++ {
				offset := g.displayXStart*2 + x*3
				first := uint32(row[((offset/2)&(VRAMWidth-1))<<g.shift])
				second := uint32(row[(((offset+2)/2)&(VRAMWidth-1))<<g.shift])
				var rgba uint32
				if offset&1 == 0 {
					rgba = first | (second&255)<<16
				} else {
					rgba = first>>8 | second<<8
				}
				rgba |= 0xff000000
				for dx := uint32(0); dx < n; dx++ {
					binary.LittleEndian.PutUint32(pix[(x*n+dx)*4:][:4], rgba)
				}
			}
		} else {
			start := int(g.displayXStart<<g.shift) & (len(row) - 1)
			first := min(int(w), len(row)-start)
			screenRGB555Span(pix[:first*4], row[start:start+first])
			if first < int(w) {
				screenRGB555Span(pix[first*4:], row[:int(w)-first])
			}
		}
	}
	if g.margin != 0 && !rgb24 {
		g.paintSides(img, xoff, int(w))
	}
	return img
}

// 016-hd-output.md §8.4：連續區段不跨 VRAM 列邊界。
func screenRGB555Span(dst []byte, src []uint16) {
	if len(src) == 0 {
		return
	}
	_ = dst[len(src)*4-1]
	for len(src) >= 4 {
		a := screenRGBA[src[0]&32767]
		b := screenRGBA[src[1]&32767]
		c := screenRGBA[src[2]&32767]
		d := screenRGBA[src[3]&32767]
		_ = dst[15]
		binary.LittleEndian.PutUint64(dst, uint64(a)|uint64(b)<<32)
		binary.LittleEndian.PutUint64(dst[8:], uint64(c)|uint64(d)<<32)
		src = src[4:]
		dst = dst[16:]
	}
	for i, v := range src {
		binary.LittleEndian.PutUint32(dst[i*4:], screenRGBA[v&32767])
	}
}
