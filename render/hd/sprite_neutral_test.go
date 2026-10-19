// 保留中性色最佳化前 drawSprite；render/hd/draw.go SHA-256 a5d6ae3dc7998bc105c89c620acee75c3500db5ae49b856250c07027e04ff30c。
// 參照來源與授權同原檔；007 §9 的獨立運算與完整精靈微基準。
package hd

import "testing"

func (g *Renderer) spriteBeforeNeutralSkip(xArg, yArg, w, h int32, uArg, vArg uint8, color uint32, textured, modulate bool, mode int) {
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
					hd := g.textures != nil && g.drawHDTexturePixel(x, y, uint32(ur), uint32(v), uInc, vInc, r, gg, b, modulate, mode)
					if !hd && t != 0 {
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

func TestSpriteNeutralModulationIdentity(t *testing.T) {
	g := newRenderer(1)
	for v := uint32(0); v < 65536; v++ {
		got := g.modTexel(2, 3, uint16(v), 128, 128, 128)
		if got != uint16(v) {
			t.Fatalf("color=%04x got=%04x", v, got)
		}
	}
}
func BenchmarkSpriteNeutral(b *testing.B) {
	for _, before := range []bool{true, false} {
		name := "current"
		if before {
			name = "before"
		}
		b.Run(name, func(b *testing.B) {
			g := newRenderer(1)
			g.clipX0, g.clipY0, g.clipX1, g.clipY1 = 0, 0, 1023, 511
			for i := range g.clutCache {
				g.clutCache[i] = uint16(i*733) | 1
			}
			for y := uint32(0); y < 128; y++ {
				for x := uint32(0); x < 64; x++ {
					g.put(x, y, uint16(x*999+y))
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if before {
					g.spriteBeforeNeutralSkip(128, 256, 192, 64, 0, 0, 0x808080, true, true, -1)
				} else {
					g.drawSprite(128, 256, 192, 64, 0, 0, 0x808080, true, true, -1)
				}
			}
		})
	}
}
