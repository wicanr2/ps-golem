package hd

import (
	"image"
	"image/draw"
)

// sideCanvas 以完整 E3／E4 矩形作索引。像素不在 VRAM，貼圖與傳輸不會讀寫它。
type sideCanvas struct {
	rect        [4]int32
	left, right []uint16
}

// SetSideMargin 只供呈現端設定，單位為原生像素；0 關閉。
// 在送入第一筆機器輸入之前設定。
func (s *Shadow) SetSideMargin(m int) {
	if m < 0 || m > 512 {
		panic("側邊寬度超出範圍")
	}
	s.read(func(g *Renderer) {
		g.margin = int32(m)
		g.sides = make(map[[4]int32]*sideCanvas)
		g.side = nil
	})
}

func (g *Renderer) clipLeft() int32  { return g.clipX0 - g.margin }
func (g *Renderer) clipRight() int32 { return g.clipX1 + g.margin }

// prepareSides 在繪圖命令開始時執行，避免 E3／E4 尚未成對時配置暫時矩形。
func (g *Renderer) prepareSides() {
	if g.margin == 0 {
		return
	}
	k := [4]int32{g.clipX0, g.clipY0, g.clipX1, g.clipY1}
	if k[2] < k[0] || k[3] < k[1] {
		g.side = nil
		return
	}
	a := g.sides[k]
	if a == nil {
		// 同一起點的新尺寸取代舊矩形，顯示端不從 map 任選過期尺寸。
		for old := range g.sides {
			if old[0] == k[0] && old[1] == k[1] {
				delete(g.sides, old)
			}
		}
		n := int(g.margin*(k[3]-k[1]+1)) << (2 * g.shift)
		a = &sideCanvas{rect: k, left: make([]uint16, n), right: make([]uint16, n)}
		g.sides[k] = a
	}
	if g.clearSide {
		clear(a.left)
		clear(a.right)
		g.clearSide = false
	}
	g.side = a
}

func (g *Renderer) sidePixel(x, y int32) *uint16 {
	a := g.side
	if a == nil {
		return nil
	}
	x0, x1 := a.rect[0]<<g.shift, (a.rect[2]+1)<<g.shift
	y -= a.rect[1] << g.shift
	w := g.margin << g.shift
	if y < 0 || y >= (a.rect[3]-a.rect[1]+1)<<g.shift {
		return nil
	}
	if x >= x0-w && x < x0 {
		return &a.left[y*w+x-x0+w]
	}
	if x >= x1 && x < x1+w {
		return &a.right[y*w+x-x1]
	}
	return nil
}

func (g *Renderer) plotSide(x, y int32, fore uint16, mode int, textured bool) {
	p := g.sidePixel(x, y)
	if p == nil {
		return
	}
	if *p&g.maskEvalAND != 0 {
		return
	}
	if mode >= 0 && fore&0x8000 != 0 {
		fore = blend(mode, *p, fore)
	}
	if !textured {
		fore &= 0x7fff
	}
	*p = fore | g.maskSetOR
}

func (g *Renderer) fillSides(x, y, w, h uint32, fill uint16) {
	for _, a := range g.sides {
		if int32(x) > a.rect[0] || int32(x+w) <= a.rect[2] {
			continue
		}
		stride := g.margin << g.shift
		for dy := uint32(0); dy < h; dy++ {
			ny := int32((y + dy) & 511)
			if ny < a.rect[1] || ny > a.rect[3] || g.lineSkip(uint32(ny)) {
				continue
			}
			start := (ny - a.rect[1]) << g.shift
			for sy := int32(0); sy < 1<<g.shift; sy++ {
				for sx := int32(0); sx < stride; sx++ {
					i := (start+sy)*stride + sx
					a.left[i], a.right[i] = fill, fill
				}
			}
		}
	}
}

// wideScreen 在原畫面外接上側邊，再取中央 16:9 範圍。
// 動畫或找不到對應繪圖區時，左右維持黑色。
func (g *Renderer) wideScreen(src *image.RGBA) *image.RGBA {
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
	for _, a := range g.sides {
		if uint32(a.rect[0]) != g.displayXStart || uint32(a.rect[1]) != g.displayYStart || int(a.rect[2]-a.rect[0]+1)*n != w {
			continue
		}
		stride := int(g.margin) * n
		for y := 0; y < min(h, int(a.rect[3]-a.rect[1]+1)*n); y++ {
			for x := 0; x < xoff && x < stride; x++ {
				putRGB(out, xoff-1-x, y, a.left[y*stride+stride-1-x])
				putRGB(out, xoff+w+x, y, a.right[y*stride+x])
			}
		}
		break
	}
	return out
}

func putRGB(img *image.RGBA, x, y int, v uint16) {
	o := img.PixOffset(x, y)
	for i := 0; i < 3; i++ {
		c := uint8(v>>(5*i)) & 31
		img.Pix[o+i] = c<<3 | c>>2
	}
}
