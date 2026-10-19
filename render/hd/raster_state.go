package hd

import "image"

// 021：CPU鏡像與GPU各自的髒區，不接觸原生GPU或Machine。
type rasterState struct {
	device             RasterDevice
	cpuDirty, gpuDirty image.Rectangle
	words              []uint16
	bytes              []byte
	work               *RasterJob
	stats              RasterStats
}

func (g *Renderer) attachRaster(d RasterDevice) {
	if d != nil {
		g.raster = &rasterState{device: d, cpuDirty: image.Rect(0, 0, 1024<<g.shift, 512<<g.shift)}
	}
}

func (g *Renderer) uploadRaster() {
	r := g.raster
	if r == nil || r.cpuDirty.Empty() {
		return
	}
	b := r.cpuDirty
	n := b.Dx() * b.Dy()
	if cap(r.words) < n {
		r.words = make([]uint16, n)
	}
	words := r.words[:n]
	stride := 1024 << g.shift
	for y := 0; y < b.Dy(); y++ {
		copy(words[y*b.Dx():], g.VRAM[(y+b.Min.Y)*stride+b.Min.X:][:b.Dx()])
	}
	r.device.Upload(b, words)
	r.cpuDirty = image.Rectangle{}
}

func (g *Renderer) syncRaster() {
	r := g.raster
	if r == nil || r.gpuDirty.Empty() {
		return
	}
	g.uploadRaster()
	b := r.gpuDirty
	n := b.Dx() * b.Dy()
	if cap(r.words) < n {
		r.words = make([]uint16, n)
	}
	words := r.words[:n]
	r.device.Read(b, words)
	stride := 1024 << g.shift
	for y := 0; y < b.Dy(); y++ {
		copy(g.VRAM[(y+b.Min.Y)*stride+b.Min.X:][:b.Dx()], words[y*b.Dx():])
	}
	r.gpuDirty = image.Rectangle{}
	r.stats.Readbacks++
}

func (g *Renderer) rasterCPUWrite(b image.Rectangle) {
	r := g.raster
	if r == nil {
		return
	}
	merged := r.cpuDirty.Union(b)
	if !merged.Intersect(r.gpuDirty).Empty() {
		g.syncRaster()
	}
	r.cpuDirty = r.cpuDirty.Union(b)
}

func (g *Renderer) rasterCPURead(x, y int) {
	if r := g.raster; r != nil && image.Pt(x, y).In(r.gpuDirty) {
		g.syncRaster()
	}
}

func (g *Renderer) rasterFence() {
	if g.raster != nil {
		g.uploadRaster()
		g.raster.device.Flush()
	}
}

func (g *Renderer) rasterCPUPrimitive() {
	if g.raster == nil {
		return
	}
	g.syncRaster()
	maxY := int((g.clipY1 + 1) << g.shift)
	b := image.Rectangle{Min: image.Pt(max(0, int(g.clipX0<<g.shift)), max(0, int(g.clipY0<<g.shift))), Max: image.Pt(min(1024<<g.shift, int((g.clipX1+1)<<g.shift)), min(512<<g.shift, maxY))}
	if maxY > 512<<g.shift {
		b = image.Rect(0, 0, 1024<<g.shift, 512<<g.shift)
	}
	if !b.Empty() {
		g.raster.cpuDirty = g.raster.cpuDirty.Union(b)
	}
}

func (g *Renderer) rasterSafe(rect image.Rectangle, textured bool, tm uint32) bool {
	if g.raster == nil {
		return false
	}
	if rect.Empty() || g.maskEvalAND != 0 {
		return false
	}
	if rect.Min.X < 0 || rect.Min.Y < 0 || rect.Max.X > 1024<<g.shift || rect.Max.Y > 512<<g.shift {
		return false
	}
	// 跨到側畫布的圖元保留原CPU路徑；其餘中央圖元仍可用GPU。
	if g.margin != 0 && (rect.Min.X < int(g.clipX0<<g.shift) || rect.Max.X > int((g.clipX1+1)<<g.shift)) {
		return false
	}
	if textured {
		if tm > 2 {
			tm = 2
		}
		s := int(g.shift)
		x0, x1 := int(g.texPageX)<<s, (int(g.texPageX)+(256>>(2-tm)))<<s
		if x1 > 1024<<g.shift {
			x0, x1 = 0, 1024<<g.shift
		}
		page := image.Rect(x0, int(g.texPageY)<<s, x1, (int(g.texPageY)+256)<<s)
		if !page.Intersect(rect).Empty() {
			return false
		}
	}
	return true
}

func (g *Renderer) beginRaster(rect image.Rectangle, color, dx, dy [3]uint32, textured, modulate, dither bool, mode int) {
	r := g.raster
	n := rect.Dx() * rect.Dy() * 4
	if cap(r.bytes) < n {
		r.bytes = make([]byte, n)
	}
	b := r.bytes[:n]
	clear(b)
	r.work = &RasterJob{Rect: rect, Texels: b, Color: color, DX: dx, DY: dy, Textured: textured, Modulate: modulate, Dither: dither, Mode: mode, Shift: g.shift, MaskSet: g.maskSetOR, MaskEval: g.maskEvalAND}
}

func (g *Renderer) rasterPixel(x, y int32, t uint16, subpixel bool) {
	r := g.raster
	j := r.work
	i := ((int(y)-j.Rect.Min.Y)*j.Rect.Dx() + int(x) - j.Rect.Min.X) * 4
	j.Texels[i], j.Texels[i+1], j.Texels[i+3] = byte(t), byte(t>>8), 255
	if subpixel {
		j.Texels[i+2] = 255
	}
	if g.textures != nil {
		g.textures.setOrigin((uint32(y)>>g.shift)*1024+(uint32(x)>>g.shift), textureOrigin{})
	}
	r.stats.Pixels++
}

func (g *Renderer) endRaster() {
	r := g.raster
	j := r.work
	g.uploadRaster()
	r.device.Draw(*j)
	r.gpuDirty = r.gpuDirty.Union(j.Rect)
	r.work = nil
}

func (g *Renderer) beginRasterTriangle(vs [3]triVertex, ig iGroup, idl iDeltas, f polyFlags) bool {
	if g.raster == nil {
		return false
	}
	x0, x1, y0, y1 := vs[0].x, vs[0].x, vs[0].y, vs[0].y
	for _, v := range vs[1:] {
		x0 = min(x0, v.x)
		x1 = max(x1, v.x)
		y0 = min(y0, v.y)
		y1 = max(y1, v.y)
	}
	// 符號座標會環繞的圖元不送GPU，避免把xAdj誤當實際VRAM座標。
	if x0 < 0 || y0 < 0 || x1 >= 1024<<g.shift || y1 >= 512<<g.shift {
		g.raster.stats.Fallbacks++
		return false
	}
	b := image.Rectangle{Min: image.Pt(int(max(x0, g.clipLeft()<<g.shift)), int(max(y0, g.clipY0<<g.shift))), Max: image.Pt(int(min(x1+1, (g.clipRight()+1)<<g.shift)), int(min(y1+1, (g.clipY1+1)<<g.shift)))}
	if !g.rasterSafe(b, f.textured, f.tm) {
		g.raster.stats.Fallbacks++
		return false
	}
	dx, dy := [3]uint32{}, [3]uint32{}
	if f.gouraud {
		dx = [3]uint32{idl.drDx, idl.dgDx, idl.dbDx}
		dy = [3]uint32{idl.drDy, idl.dgDy, idl.dbDy}
	}
	g.beginRaster(b, [3]uint32{ig.r, ig.g, ig.b}, dx, dy, f.textured, f.modulate, g.dtd && (f.textured || f.gouraud), f.mode)
	g.raster.stats.Triangles++
	return true
}

func (g *Renderer) rasterSpan(x, y, w int32, ig iGroup, idl *iDeltas, f polyFlags) {
	const sh = coordFBS + coordPostPadding
	wrote := false
	for i := int32(0); i < w; i++ {
		t := uint16(0x8000)
		if f.textured {
			tu, tv := g.perspectiveUV(x+i, y, ig.u, ig.v)
			t = g.getTexel(f.tm, tu>>sh, tv>>sh)
			if g.textures != nil {
				if e, tx, ty := g.lookupTexture(f.tm, tu>>sh, tv>>sh); e != nil {
					t = g.texturePixel(e, tx, ty, tu&((1<<sh)-1), tv&((1<<sh)-1), sh)
				}
			}
			ig.u += idl.duDx
			ig.v += idl.dvDx
		}
		if !f.textured || t != 0 {
			g.rasterPixel(x+i, y, t, false)
			wrote = true
		}
	}
	if wrote {
		g.markSideDraw()
	}
}

// 精靈沿用原生UV／翻轉／快取順序，GPU執行HD子像素的色彩與目的寫入。
func (g *Renderer) rasterSprite(x0, y0, x1, y1 int32, u, v uint8, uInc, vInc int32, r, gg, b uint32, textured, modulate bool, mode int) bool {
	if g.raster == nil {
		return false
	}
	rect := image.Rectangle{Min: image.Pt(int(x0<<g.shift), int(y0<<g.shift)), Max: image.Pt(int(x1<<g.shift), int(y1<<g.shift))}
	if !g.rasterSafe(rect, textured, g.texMode) {
		g.raster.stats.Fallbacks++
		return false
	}
	g.beginRaster(rect, [3]uint32{r << 24, gg << 24, b << 24}, [3]uint32{}, [3]uint32{}, textured, modulate, false, mode)
	g.raster.work.Sprite = true
	n := int32(1) << g.shift
	wrote := false
	for y := y0; y < y1; y++ {
		ur := u
		if !g.lineSkip(uint32(y)) {
			for x := x0; x < x1; x++ {
				t := uint16(0x8000)
				var entry *textureImage
				var tx, ty int
				if textured {
					t = g.getTexel(g.texMode, uint32(ur), uint32(v))
					if g.textures != nil {
						entry, tx, ty = g.lookupTexture(g.texMode, uint32(ur), uint32(v))
					}
				}
				for yy := int32(0); yy < n; yy++ {
					for xx := int32(0); xx < n; xx++ {
						color := t
						if entry != nil {
							fx, fy := xx, yy
							if uInc < 0 {
								fx = n - 1 - xx
							}
							if vInc < 0 {
								fy = n - 1 - yy
							}
							color = g.texturePixel(entry, tx, ty, uint32(fx), uint32(fy), g.shift)
						}
						if !textured || color != 0 {
							g.rasterPixel((x<<g.shift)+xx, (y<<g.shift)+yy, color, entry != nil)
							wrote = true
						}
					}
				}
				if textured {
					ur = uint8(int32(ur) + uInc)
				}
			}
		}
		if textured {
			v = uint8(int32(v) + vInc)
		}
	}
	if wrote {
		g.markSideDraw()
	}
	g.endRaster()
	g.raster.stats.Sprites++
	return true
}
