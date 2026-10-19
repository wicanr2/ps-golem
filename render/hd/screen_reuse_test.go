package hd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"testing"
)

// 保留優化前的逐像素實作作為獨立 oracle；不使用正式 RGB 查表。
func (g *Renderer) referenceScreen() *image.RGBA {
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
		return g.referenceWideScreen(img)
	}
	return img
}

func (g *Renderer) referenceWideScreen(src *image.RGBA) *image.RGBA {
	n := 1 << g.shift
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	// 256 寬時取規格的 342；其他解析度按相同比例補黑。
	target := ((w/n*4 + 2) / 3) * n
	if w/n == 256 {
		target = 342 * n
	}
	out := image.NewRGBA(image.Rect(0, 0, target, h))
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	xoff := (target - w) / 2
	draw.Draw(out, image.Rect(xoff, 0, xoff+w, h), src, image.Point{}, draw.Src)
	if g.displayMode&0x10 != 0 {
		return out
	}
	var selected *sideCanvas
	for _, a := range g.sides {
		yoff := (int(a.rect[1]) - int(g.displayYStart)) * n
		if uint32(a.rect[0]) != g.displayXStart || yoff < 0 || yoff >= h || int(a.rect[2]-a.rect[0]+1)*n != w || a.lastDraw == 0 {
			continue
		}
		if selected == nil || a.lastDraw > selected.lastDraw {
			selected = a
		}
	}
	if a := selected; a != nil {
		yoff := (int(a.rect[1]) - int(g.displayYStart)) * n
		stride := int(g.margin) * n
		for y := 0; y < min(h-yoff, int(a.rect[3]-a.rect[1]+1)*n); y++ {
			for x := 0; x < xoff && x < stride; x++ {
				referencePutRGB(out, xoff-1-x, y+yoff, a.left[y*stride+stride-1-x])
				referencePutRGB(out, xoff+w+x, y+yoff, a.right[y*stride+x])
			}
		}
	}
	return out
}

func referencePutRGB(img *image.RGBA, x, y int, v uint16) {
	o := img.PixOffset(x, y)
	for i := 0; i < 3; i++ {
		c := uint8(v>>(5*i)) & 31
		img.Pix[o+i] = c<<3 | c>>2
	}
}

func TestScreenIntoMatchesPreviousRenderer(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := newRenderer(shift)
		seed := uint32(18026)
		for i := range g.VRAM {
			seed = seed*1664525 + 1013904223
			g.VRAM[i] = uint16(seed >> 8)
		}
		var dst *image.RGBA
		for _, mode := range []uint32{0, 1, 2, 3, 0x40, 0x10, 0x14, 0x24, 0x34} {
			for _, margin := range []int32{0, 43} {
				for _, start := range [][2]uint32{{0, 0}, {1019, 509}} {
					g.displayMode, g.margin = mode, margin
					g.horizStart, g.horizEnd = 0, 32*g.DotDivider()
					g.vertStart, g.vertEnd = 0, 9
					g.displayXStart, g.displayYStart = start[0], start[1]
					rect := [4]int32{int32(start[0]), int32(start[1]), int32(start[0]) + 31, int32(start[1]) + 8}
					n := int(margin*9) << (2 * shift)
					a := &sideCanvas{rect: rect, left: make([]uint16, n), right: make([]uint16, n), lastDraw: 2}
					for i := range a.left {
						a.left[i], a.right[i] = uint16(i*31+7), uint16(i*13+17)
					}
					g.sides = map[[4]int32]*sideCanvas{rect: a}
					want := g.referenceScreen()
					dst = g.ScreenInto(dst)
					if dst.Rect != want.Rect || !bytes.Equal(dst.Pix, want.Pix) {
						t.Fatalf("shift=%d mode=%x margin=%d start=%v", shift, mode, margin, start)
					}
					reuse := g.ScreenInto(dst)
					if reuse != dst || !bytes.Equal(reuse.Pix, want.Pix) {
						t.Fatal("buffer not reused exactly")
					}
					// 失去側邊選區後，重用輸出也必須清掉舊像素。
					g.sides = nil
					dst = g.ScreenInto(dst)
					if !bytes.Equal(dst.Pix, g.referenceScreen().Pix) {
						t.Fatal("stale side pixels")
					}
				}
			}
		}
		g.margin, g.displayMode = 43, 0
		g.horizEnd = 256 * g.DotDivider()
		if got, want := g.Screen(), g.referenceScreen(); !bytes.Equal(got.Pix, want.Pix) || got.Rect != want.Rect {
			t.Fatal("256-wide special case changed")
		}
	}
}

func TestScreenIntoOwnershipAndLayout(t *testing.T) {
	g := newRenderer(1)
	a, b := g.Screen(), g.Screen()
	a.Pix[0] ^= 255
	if bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("Screen shares returned pixels")
	}
	bad := image.NewRGBA(image.Rect(0, 0, b.Rect.Dx()+1, b.Rect.Dy())).SubImage(b.Rect).(*image.RGBA)
	if got := g.ScreenInto(bad); got == bad || !bytes.Equal(got.Pix, b.Pix) {
		t.Fatal("incompatible stride accepted")
	}
	if n := testing.AllocsPerRun(10, func() { g.ScreenInto(b) }); n != 0 {
		t.Fatalf("same-size conversion allocates: %g", n)
	}
}

func BenchmarkScreenIntoHD2(b *testing.B) {
	g := newRenderer(1)
	dst := g.Screen()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = g.ScreenInto(dst)
	}
}

func BenchmarkScreenPreviousHD2(b *testing.B) {
	g := newRenderer(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.referenceScreen()
	}
}

func (g *Renderer) screenIntoBaseline(dst *image.RGBA) *image.RGBA {
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
			start := g.displayXStart << g.shift
			for x := uint32(0); x < w; x++ {
				binary.LittleEndian.PutUint32(pix[x*4:][:4], screenRGBA[row[(start+x)&(stride-1)]&32767])
			}
		}
	}
	if g.margin != 0 && !rgb24 {
		g.paintSides(img, xoff, int(w))
	}
	return img
}

func TestRGB555SpanAllWordsAndTails(t *testing.T) {
	src := make([]uint16, 65541)
	for i := range src {
		src[i] = uint16(i)
	}
	for _, n := range []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 17, 319, 320, 641, 65536, 65541} {
		for _, offset := range []int{0, 1, 3} {
			all := bytes.Repeat([]byte{0x5a}, offset+n*4+7)
			dst := all[offset : offset+n*4]
			screenRGB555Span(dst, src[:n])
			want := make([]byte, n*4)
			for i, v := range src[:n] {
				r, g, b := byte(v&31), byte((v>>5)&31), byte((v>>10)&31)
				want[i*4], want[i*4+1], want[i*4+2], want[i*4+3] = r<<3|r>>2, g<<3|g>>2, b<<3|b>>2, 255
			}
			if !bytes.Equal(dst, want) || !bytes.Equal(all[:offset], bytes.Repeat([]byte{0x5a}, offset)) ||
				!bytes.Equal(all[offset+n*4:], bytes.Repeat([]byte{0x5a}, 7)) {
				t.Fatalf("n=%d offset=%d", n, offset)
			}
		}
	}
}
func TestScreenSpanFullWidths(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := newRenderer(shift)
		seed := uint32(18028)
		for i := range g.VRAM {
			seed = seed*1664525 + 1013904223
			g.VRAM[i] = uint16(seed >> 8)
		}
		for _, mode := range []uint32{0, 1, 2, 3, 0x40} {
			for _, start := range [][2]uint32{{0, 0}, {733, 503}, {1023, 511}} {
				for _, margin := range []int32{0, 43} {
					g.displayMode, g.margin = mode, margin
					g.horizStart, g.horizEnd = 0, 0
					g.vertStart, g.vertEnd = 0, 19
					g.displayXStart, g.displayYStart = start[0], start[1]
					got, want := g.Screen(), g.referenceScreen()
					if got.Rect != want.Rect || !bytes.Equal(got.Pix, want.Pix) {
						t.Fatalf("shift=%d mode=%x start=%v margin=%d", shift, mode, start, margin)
					}
				}
			}
		}
	}
}

var screenSpanBenchmarkImage *image.RGBA

func BenchmarkScreenSpanConversion(b *testing.B) {
	for _, wrap := range []bool{false, true} {
		for _, optimized := range []bool{false, true} {
			b.Run(fmt.Sprintf("wrap=%v/new=%v", wrap, optimized), func(b *testing.B) {
				g := newRenderer(1)
				g.displayMode = 1
				g.horizStart, g.horizEnd = 0, 0
				g.vertStart, g.vertEnd = 0, 240
				if wrap {
					g.displayXStart = 733
				}
				for i := range g.VRAM {
					g.VRAM[i] = uint16(i * 31)
				}
				dst := g.Screen()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if optimized {
						screenSpanBenchmarkImage = g.ScreenInto(dst)
					} else {
						screenSpanBenchmarkImage = g.screenIntoBaseline(dst)
					}
				}
			})
		}
	}
}
