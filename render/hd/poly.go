// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

// 多邊形光柵化，照 Beetle gpu_polygon.c 的原生解析度路徑（不含 PGXP 透視修正）。

const (
	coordFBS         = 12
	coordPostPadding = 12
)

type triVertex struct {
	x, y    int32
	u, v    int32
	r, g, b int32
}

type iGroup struct {
	u, v    uint32
	r, g, b uint32
}

type iDeltas struct {
	duDx, dvDx       uint32
	drDx, dgDx, dbDx uint32
	duDy, dvDy       uint32
	drDy, dgDy, dbDy uint32
}

type polyFlags struct {
	gouraud, textured, modulate bool
	mode                        int
	tm                          uint32
}

func (g *Renderer) drawPolygon(cb []uint32) {
	cmd := cb[0] >> 24
	f := polyFlags{
		gouraud:  cmd&0x10 != 0,
		textured: cmd&0x04 != 0,
		mode:     g.blendModeOf(cmd),
	}
	f.modulate = f.textured && cmd&0x01 == 0
	nv := 3
	if cmd&0x08 != 0 {
		nv = 4
	}
	// 貼圖頁在第二個頂點的 UV 字上半（Beetle ProcessFIFO 的 SetTPage）。
	if f.textured {
		idx := 4
		if f.gouraud {
			idx = 5
		}
		g.setTPage(cb[idx] >> 16)
	}
	f.tm = g.texMode

	var vs [4]triVertex
	i := 0
	for n := 0; n < nv; n++ {
		if n == 0 || f.gouraud {
			c := cb[i] & 0xFFFFFF
			vs[n].r, vs[n].g, vs[n].b = int32(c&0xFF), int32((c>>8)&0xFF), int32((c>>16)&0xFF)
			i++
		} else {
			vs[n].r, vs[n].g, vs[n].b = vs[0].r, vs[0].g, vs[0].b
		}
		x := signExtend11(uint32(int16(cb[i] & 0xFFFF)))
		y := signExtend11(uint32(int16(cb[i] >> 16)))
		vs[n].x = (x + g.offsX) << g.shift
		vs[n].y = (y + g.offsY) << g.shift
		g.polygonVertices++
		// 參照 beetle-psx-libretro mednafen/psx/gpu_polygon.c PGXP_LIT，
		// commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later。
		// 與軟體渲染器相同，浮點座標加偏移及放大後轉為整數子像素。
		if p := g.precise[i]; p.Valid(cb[i]) {
			vs[n].x = int32((p.X + float32(g.offsX)) * float32(uint32(1)<<g.shift))
			vs[n].y = int32((p.Y + float32(g.offsY)) * float32(uint32(1)<<g.shift))
			g.pgxpVertices++
		}
		i++
		if f.textured {
			vs[n].u = int32(cb[i] & 0xFF)
			vs[n].v = int32((cb[i] >> 8) & 0xFF)
			if n == 0 {
				g.updateCLUT((cb[i]>>16)&0xFFFF, f.tm)
			}
			i++
		}
	}
	g.drawTriangleChecked([3]triVertex{vs[0], vs[1], vs[2]}, f)
	if nv == 4 {
		g.drawTriangleChecked([3]triVertex{vs[1], vs[2], vs[3]}, f)
	}
}

func (g *Renderer) drawTriangleChecked(v [3]triVertex, f polyFlags) {
	if abs32(v[2].y-v[0].y) >= 512<<g.shift || abs32(v[2].y-v[1].y) >= 512<<g.shift || abs32(v[1].y-v[0].y) >= 512<<g.shift {
		return
	}
	if abs32(v[2].x-v[0].x) >= 1024<<g.shift || abs32(v[2].x-v[1].x) >= 1024<<g.shift || abs32(v[1].x-v[0].x) >= 1024<<g.shift {
		return
	}
	g.drawTriangle(v, f)
}

func makePolyXFP(x int32) int64 {
	return int64(uint64(uint32(x))<<32) + ((1 << 32) - (1 << 11))
}

func makePolyXFPStep(dx, dy int32) int64 {
	dxEx := int64(dx) << 32
	if dxEx < 0 {
		dxEx -= int64(dy) - 1
	}
	if dxEx > 0 {
		dxEx += int64(dy) - 1
	}
	return dxEx / int64(dy)
}

// calcis 照 Beetle 的 CALCIS 巨集，以 int32 運算（溢位行為同 C 的環繞）。
func calcis(ax, ay, bx, by, cx, cy int32) int32 {
	return (bx-ax)*(cy-by) - (cx-bx)*(by-ay)
}

func calcIDeltas(idl *iDeltas, a, b, c *triVertex, f polyFlags) bool {
	denom := calcis(a.x, a.y, b.x, b.y, c.x, c.y)
	if denom == 0 {
		return false
	}
	q := func(n int32) uint32 { return uint32(int64(n)*(1<<coordFBS)/int64(denom)) << coordPostPadding }
	if f.gouraud {
		idl.drDx = q(calcis(a.r, a.y, b.r, b.y, c.r, c.y))
		idl.drDy = q(calcis(a.x, a.r, b.x, b.r, c.x, c.r))
		idl.dgDx = q(calcis(a.g, a.y, b.g, b.y, c.g, c.y))
		idl.dgDy = q(calcis(a.x, a.g, b.x, b.g, c.x, c.g))
		idl.dbDx = q(calcis(a.b, a.y, b.b, b.y, c.b, c.y))
		idl.dbDy = q(calcis(a.x, a.b, b.x, b.b, c.x, c.b))
	}
	if f.textured {
		idl.duDx = q(calcis(a.u, a.y, b.u, b.y, c.u, c.y))
		idl.duDy = q(calcis(a.x, a.u, b.x, b.u, c.x, c.u))
		idl.dvDx = q(calcis(a.v, a.y, b.v, b.y, c.v, c.y))
		idl.dvDy = q(calcis(a.x, a.v, b.x, b.v, c.x, c.v))
	}
	return true
}

func addDX(ig *iGroup, idl *iDeltas, count uint32, f polyFlags) {
	if f.textured {
		ig.u += idl.duDx * count
		ig.v += idl.dvDx * count
	}
	if f.gouraud {
		ig.r += idl.drDx * count
		ig.g += idl.dgDx * count
		ig.b += idl.dbDx * count
	}
}

func addDY(ig *iGroup, idl *iDeltas, count uint32, f polyFlags) {
	if f.textured {
		ig.u += idl.duDy * count
		ig.v += idl.dvDy * count
	}
	if f.gouraud {
		ig.r += idl.drDy * count
		ig.g += idl.dgDy * count
		ig.b += idl.dbDy * count
	}
}

func (g *Renderer) drawTriangle(vs [3]triVertex, f polyFlags) {
	var cvtemp uint32
	if vs[1].x <= vs[0].x {
		if vs[2].x <= vs[1].x {
			cvtemp = 1 << 2
		} else {
			cvtemp = 1 << 1
		}
	} else if vs[2].x < vs[0].x {
		cvtemp = 1 << 2
	} else {
		cvtemp = 1 << 0
	}
	if vs[2].y < vs[1].y {
		vs[2], vs[1] = vs[1], vs[2]
		cvtemp = (cvtemp>>1)&0x2 | (cvtemp<<1)&0x4 | cvtemp&0x1
	}
	if vs[1].y < vs[0].y {
		vs[1], vs[0] = vs[0], vs[1]
		cvtemp = (cvtemp>>1)&0x1 | (cvtemp<<1)&0x2 | cvtemp&0x4
	}
	if vs[2].y < vs[1].y {
		vs[2], vs[1] = vs[1], vs[2]
		cvtemp = (cvtemp>>1)&0x2 | (cvtemp<<1)&0x4 | cvtemp&0x1
	}
	core := cvtemp >> 1

	if vs[0].y == vs[2].y {
		return
	}
	var idl iDeltas
	if !calcIDeltas(&idl, &vs[0], &vs[1], &vs[2], f) {
		return
	}

	var ig iGroup
	cv := &vs[core]
	if f.textured {
		ig.u = (uint32(cv.u)<<coordFBS + 1<<(coordFBS-1-g.shift)) << coordPostPadding
		ig.v = (uint32(cv.v)<<coordFBS + 1<<(coordFBS-1-g.shift)) << coordPostPadding
	}
	ig.r = (uint32(cv.r)<<coordFBS + 1<<(coordFBS-1)) << coordPostPadding
	ig.g = (uint32(cv.g)<<coordFBS + 1<<(coordFBS-1)) << coordPostPadding
	ig.b = (uint32(cv.b)<<coordFBS + 1<<(coordFBS-1)) << coordPostPadding
	addDX(&ig, &idl, uint32(-cv.x), f)
	addDY(&ig, &idl, uint32(-cv.y), f)

	baseCoord := makePolyXFP(vs[0].x)
	baseStep := makePolyXFPStep(vs[2].x-vs[0].x, vs[2].y-vs[0].y)
	var boundUS, boundLS int64
	var rightFacing bool
	if vs[1].y == vs[0].y {
		boundUS = 0
		rightFacing = vs[1].x > vs[0].x
	} else {
		boundUS = makePolyXFPStep(vs[1].x-vs[0].x, vs[1].y-vs[0].y)
		rightFacing = boundUS > baseStep
	}
	if vs[2].y == vs[1].y {
		boundLS = 0
	} else {
		boundLS = makePolyXFPStep(vs[2].x-vs[1].x, vs[2].y-vs[1].y)
	}
	var vo, vp uint32
	if core != 0 {
		vo = 1
	}
	if core == 2 {
		vp = 3
	}
	rf := 0
	if rightFacing {
		rf = 1
	}

	type tripart struct {
		xCoord  [2]uint64
		xStep   [2]uint64
		yCoord  int32
		yBound  int32
		decMode bool
	}
	var tp [2]tripart
	{
		t := &tp[vo]
		t.yCoord = vs[0^vo].y
		t.yBound = vs[1^vo].y
		t.xCoord[rf] = uint64(makePolyXFP(vs[0^vo].x))
		t.xStep[rf] = uint64(boundUS)
		t.xCoord[1-rf] = uint64(baseCoord + int64(vs[vo].y-vs[0].y)*baseStep)
		t.xStep[1-rf] = uint64(baseStep)
		t.decMode = vo != 0
	}
	{
		t := &tp[vo^1]
		t.yCoord = vs[1^vp].y
		t.yBound = vs[2^vp].y
		t.xCoord[rf] = uint64(makePolyXFP(vs[1^vp].x))
		t.xStep[rf] = uint64(boundLS)
		t.xCoord[1-rf] = uint64(baseCoord + int64(vs[1^vp].y-vs[0].y)*baseStep)
		t.xStep[1-rf] = uint64(baseStep)
		t.decMode = vp != 0
	}

	for i := 0; i < 2; i++ {
		yi := tp[i].yCoord
		yb := tp[i].yBound
		lc, ls := tp[i].xCoord[0], tp[i].xStep[0]
		rc, rs := tp[i].xCoord[1], tp[i].xStep[1]
		if tp[i].decMode {
			for yi > yb {
				yi--
				lc -= ls
				rc -= rs
				y := g.signCoord(uint32(yi))
				if y < (g.clipY0 << g.shift) {
					break
				}
				if y > (g.clipY1 << g.shift) {
					continue
				}
				g.drawSpan(yi, int32(int64(lc)>>32), int32(int64(rc)>>32), ig, &idl, f)
			}
		} else {
			for yi < yb {
				y := g.signCoord(uint32(yi))
				if y > (g.clipY1 << g.shift) {
					break
				}
				if y >= (g.clipY0 << g.shift) {
					g.drawSpan(yi, int32(int64(lc)>>32), int32(int64(rc)>>32), ig, &idl, f)
				}
				yi++
				lc += ls
				rc += rs
			}
		}
	}
}

func (g *Renderer) drawSpan(y, xStart, xBound int32, ig iGroup, idl *iDeltas, f polyFlags) {
	if g.lineSkip(uint32(y) >> g.shift) {
		return
	}
	xAdj := xStart
	w := xBound - xStart
	x := g.signCoord(uint32(xStart))
	if x < (g.clipX0 << g.shift) {
		d := (g.clipX0 << g.shift) - x
		xAdj += d
		x += d
		w -= d
	}
	if x+w > (g.clipX1<<g.shift)+1 {
		w = (g.clipX1 << g.shift) + 1 - x
	}
	if w <= 0 {
		return
	}
	addDX(&ig, idl, uint32(xAdj), f)
	addDY(&ig, idl, uint32(y), f)
	const sh = coordFBS + coordPostPadding
	for ; w > 0; w-- {
		r := (ig.r >> sh) & 0xFF
		gg := (ig.g >> sh) & 0xFF
		b := (ig.b >> sh) & 0xFF
		dx := (uint32(x) >> g.shift) & 3
		dy := (uint32(y) >> g.shift) & 3
		if f.textured {
			t := g.getTexel(f.tm, ig.u>>sh, ig.v>>sh)
			if t != 0 {
				if f.modulate {
					if !g.ditherEnabled() {
						dx, dy = 3, 2
					}
					t = g.modTexel(dy, dx, t, r, gg, b)
				}
				g.plotScaled(x, y, t, f.mode, true)
			}
		} else {
			var pix uint16 = 0x8000
			if f.gouraud && g.ditherEnabled() {
				pix |= uint16(g.ditherLUT[dy][r][dx]) | uint16(g.ditherLUT[dy][gg][dx])<<5 | uint16(g.ditherLUT[dy][b][dx])<<10
			} else {
				pix |= uint16(r>>3) | uint16(gg>>3)<<5 | uint16(b>>3)<<10
			}
			g.plotScaled(x, y, pix, f.mode, false)
		}
		x++
		addDX(&ig, idl, 1, f)
	}
}

func (g *Renderer) signCoord(v uint32) int32 {
	shift := 21 - g.shift
	return int32(v<<shift) >> shift
}
