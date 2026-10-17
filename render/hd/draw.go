// 參照 beetle-psx-libretro mednafen/psx/gpu.c、gpu_common.h、gpu_polygon.c、
// gpu_line.c、gpu_sprite.c，commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later。沿用 internal/gpu 的原生修正，另移植 upscale_shift 路徑。
// 本套光柵化只寫影子 VRAM，不參與機器狀態與時序。規格：016-hd-output §2–3。
package hd

// blendModeOf 回傳命令的混色模式：命令的半透明位元（bit 1）成立才用 abr。
func (g *Renderer) blendModeOf(cmd uint32) int {
	if cmd&0x02 != 0 {
		return int(g.abr)
	}
	return blendOpaque
}

func (g *Renderer) ditherEnabled() bool { return g.dtd }

// ---- Sprite（Beetle gpu_sprite.c）----

func (g *Renderer) drawSpriteCmd(cb []uint32) {
	cmd := cb[0] >> 24
	textured := cmd&0x04 != 0
	raw := cmd&0x01 != 0
	color := cb[0] & 0x00FFFFFF
	i := 1
	x := signExtend11(cb[i] & 0xFFFF)
	y := signExtend11(cb[i] >> 16)
	i++
	var u, v uint8
	var clutRaw uint32
	if textured {
		u = uint8(cb[i])
		v = uint8(cb[i] >> 8)
		clutRaw = (cb[i] >> 16) & 0xFFFF
		g.updateCLUT(clutRaw, g.texMode)
		i++
	}
	var w, h int32
	switch (cmd >> 3) & 3 {
	case 0:
		w = int32(cb[i] & 0x3FF)
		h = int32((cb[i] >> 16) & 0x1FF)
	case 1:
		w, h = 1, 1
	case 2:
		w, h = 8, 8
	case 3:
		w, h = 16, 16
	}
	x = signExtend11(uint32(x + g.offsX))
	y = signExtend11(uint32(y + g.offsY))
	g.drawSprite(x, y, w, h, u, v, color, textured, textured && !raw, g.blendModeOf(cmd))
}

func (g *Renderer) drawSprite(xArg, yArg, w, h int32, uArg, vArg uint8, color uint32, textured, modulate bool, mode int) {
	r := color & 0xFF
	gg := (color >> 8) & 0xFF
	b := (color >> 16) & 0xFF
	fill := uint16(0x8000 | r>>3 | (gg>>3)<<5 | (b>>3)<<10)
	var u, v uint8
	uInc, vInc := int32(1), int32(1)
	xStart, xBound := xArg, xArg+w
	yStart, yBound := yArg, yArg+h
	if textured {
		u, v = uArg, vArg
		if g.spriteFlip&0x1000 != 0 {
			uInc = -1
			u |= 1
		}
		if g.spriteFlip&0x2000 != 0 {
			vInc = -1
		}
	}
	if xStart < g.clipLeft() {
		if textured {
			u += uint8((g.clipLeft() - xStart) * uInc)
		}
		xStart = g.clipLeft()
	}
	if yStart < g.clipY0 {
		if textured {
			v += uint8((g.clipY0 - yStart) * vInc)
		}
		yStart = g.clipY0
	}
	if xBound > g.clipRight()+1 {
		xBound = g.clipRight() + 1
	}
	if yBound > g.clipY1+1 {
		yBound = g.clipY1 + 1
	}
	for y := yStart; y < yBound; y++ {
		ur := u
		if !g.lineSkip(uint32(y)) {
			for x := xStart; x < xBound; x++ {
				if textured {
					t := g.getTexel(g.texMode, uint32(ur), uint32(v))
					if t != 0 {
						if modulate {
							// sprite 不抖色：固定用 [2][*][3]，偏移為 0
							t = g.modTexel(2, 3, t, r, gg, b)
						}
						g.plot(x, y, t, mode, true)
					}
					ur = uint8(int32(ur) + uInc)
				} else {
					g.plot(x, y, fill, mode, false)
				}
			}
		}
		if textured {
			v = uint8(int32(v) + vInc)
		}
	}
}

// ---- 線段（Beetle gpu_line.c）----

type linePoint struct {
	x, y    int32
	r, g, b uint32
}

const lineXYFract = 32
const lineRGBFract = 12

func (g *Renderer) drawLineCmd(cb []uint32) {
	cmd := cb[0] >> 24
	gouraud := cmd&0x10 != 0
	var p [2]linePoint
	i := 0
	p[0].r = cb[i] & 0xFF
	p[0].g = (cb[i] >> 8) & 0xFF
	p[0].b = (cb[i] >> 16) & 0xFF
	i++
	p[0].x = signExtend11(cb[i]&0xFFFF) + g.offsX
	p[0].y = signExtend11(cb[i]>>16) + g.offsY
	i++
	if gouraud {
		p[1].r = cb[i] & 0xFF
		p[1].g = (cb[i] >> 8) & 0xFF
		p[1].b = (cb[i] >> 16) & 0xFF
		i++
	} else {
		p[1].r, p[1].g, p[1].b = p[0].r, p[0].g, p[0].b
	}
	p[1].x = signExtend11(cb[i]&0xFFFF) + g.offsX
	p[1].y = signExtend11(cb[i]>>16) + g.offsY
	if cmd&0x08 != 0 {
		g.plinePrev = p[1]
		g.plineCmd = cmd
		g.plineN = 0
		g.inCmd = inPolyline
	}
	g.drawLineChecked(p, gouraud, g.blendModeOf(cmd))
}

// polylineWord 收折線後續的字；遇到 0x5xxx5xxx 結束。
func (g *Renderer) polylineWord(v uint32) {
	gouraud := g.plineCmd&0x10 != 0
	if g.plineN == 0 && v&0xF000F000 == 0x50005000 {
		g.inCmd = inNone
		return
	}
	g.plineGroup[g.plineN] = v
	g.plineN++
	need := 1
	if gouraud {
		need = 2
	}
	if g.plineN < need {
		return
	}
	var p [2]linePoint
	p[0] = g.plinePrev
	k := 0
	if gouraud {
		c := g.plineGroup[0]
		p[1].r, p[1].g, p[1].b = c&0xFF, (c>>8)&0xFF, (c>>16)&0xFF
		k = 1
	} else {
		p[1].r, p[1].g, p[1].b = p[0].r, p[0].g, p[0].b
	}
	w := g.plineGroup[k]
	p[1].x = signExtend11(w&0xFFFF) + g.offsX
	p[1].y = signExtend11(w>>16) + g.offsY
	g.plineN = 0
	g.plinePrev = p[1]
	g.drawLineChecked(p, gouraud, g.blendModeOf(g.plineCmd))
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func (g *Renderer) drawLineChecked(p [2]linePoint, gouraud bool, mode int) {
	if abs32(p[1].x-p[0].x) >= 1024 || abs32(p[1].y-p[0].y) >= 512 {
		return
	}
	g.drawLine(p, gouraud, mode)
}

func lineDivide(delta int64, dk int32) int64 {
	delta = int64(uint64(delta) << lineXYFract)
	if delta < 0 {
		delta -= int64(dk) - 1
	}
	if delta > 0 {
		delta += int64(dk) - 1
	}
	return delta / int64(dk)
}

func (g *Renderer) drawLine(p [2]linePoint, gouraud bool, mode int) {
	dx := abs32(p[1].x - p[0].x)
	dy := abs32(p[1].y - p[0].y)
	k := dx
	if dy > k {
		k = dy
	}
	if p[0].x > p[1].x && k != 0 {
		p[0], p[1] = p[1], p[0]
	}
	var sx, sy int64
	var sr, sg, sb int32
	if k != 0 {
		sx = lineDivide(int64(p[1].x-p[0].x), k)
		sy = lineDivide(int64(p[1].y-p[0].y), k)
		if gouraud {
			sr = int32(uint32(p[1].r-p[0].r)<<lineRGBFract) / k
			sg = int32(uint32(p[1].g-p[0].g)<<lineRGBFract) / k
			sb = int32(uint32(p[1].b-p[0].b)<<lineRGBFract) / k
		}
	}
	cx := uint64(int64(p[0].x))<<lineXYFract | 1<<(lineXYFract-1)
	cy := uint64(int64(p[0].y))<<lineXYFract | 1<<(lineXYFract-1)
	cx -= 1024
	if sy < 0 {
		cy -= 1024
	}
	var cr, cg, cbl uint32
	if gouraud {
		cr = p[0].r<<lineRGBFract | 1<<(lineRGBFract-1)
		cg = p[0].g<<lineRGBFract | 1<<(lineRGBFract-1)
		cbl = p[0].b<<lineRGBFract | 1<<(lineRGBFract-1)
	}
	for i := int32(0); i <= k; i++ {
		x := int32(cx>>lineXYFract) & 2047
		if g.margin != 0 {
			x = signExtend11(uint32(x))
		}
		y := int32(cy>>lineXYFract) & 2047
		if !g.lineSkip(uint32(y)) {
			var r, gg, b uint32
			if gouraud {
				r, gg, b = (cr>>lineRGBFract)&0xFF, (cg>>lineRGBFract)&0xFF, (cbl>>lineRGBFract)&0xFF
			} else {
				r, gg, b = p[0].r, p[0].g, p[0].b
			}
			var pix uint16
			if g.ditherEnabled() {
				pix = 0x8000 | uint16(g.ditherLUT[y&3][r][x&3]) | uint16(g.ditherLUT[y&3][gg][x&3])<<5 | uint16(g.ditherLUT[y&3][b][x&3])<<10
			} else {
				pix = 0x8000 | uint16(r>>3) | uint16(gg>>3)<<5 | uint16(b>>3)<<10
			}
			if x >= g.clipLeft() && x <= g.clipRight() && y >= g.clipY0 && y <= g.clipY1 {
				g.plot(x, y, pix, mode, false)
			}
		}
		cx += uint64(sx)
		cy += uint64(sy)
		if gouraud {
			cr += uint32(sr)
			cg += uint32(sg)
			cbl += uint32(sb)
		}
	}
}
