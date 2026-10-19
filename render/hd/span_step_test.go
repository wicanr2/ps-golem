// 保留最佳化前 drawSpan，來源 poly.go SHA-256 39896477e5d09d8957b35af9678e4a0a82cdb0a12b8151b85e14d1a6af302ed4。
// 參照來源及授權同 poly.go；016 §8.5。僅作獨立逐像素對照。
package hd

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

func (g *Renderer) spanBeforeInvariantHoist(y, xStart, xBound int32, ig iGroup, idl *iDeltas, f polyFlags) {
	if g.lineSkip(uint32(y) >> g.shift) {
		return
	}
	xAdj := xStart
	w := xBound - xStart
	x := g.signCoord(uint32(xStart))
	if x < (g.clipLeft() << g.shift) {
		d := (g.clipLeft() << g.shift) - x
		xAdj += d
		x += d
		w -= d
	}
	if x+w > (g.clipRight()<<g.shift)+1 {
		w = (g.clipRight() << g.shift) + 1 - x
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
			if g.textures != nil {
				if e, tx, ty := g.lookupTexture(f.tm, ig.u>>sh, ig.v>>sh); e != nil {
					t = g.texturePixel(e, tx, ty, ig.u&((1<<sh)-1), ig.v&((1<<sh)-1), sh)
				}
			}
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

func TestSpanInvariantHoistExactState(t *testing.T) {
	const seed = int64(0x19c057)
	for shift := uint(0); shift <= 3; shift++ {
		for _, tracked := range []bool{false, true} {
			t.Run(fmt.Sprintf("N%d/tracked%v", 1<<shift, tracked), func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))
				a, b := newRenderer(shift), newRenderer(shift)
				for i := range a.VRAM {
					v := uint16(rng.Uint32())
					a.VRAM[i] = v
					b.VRAM[i] = v
				}
				var pack *TexturePack
				if tracked {
					pack = textureTestPack(t, []uint16{0x1111, 0x1111}, 4, 2)
				}
				for _, g := range []*Renderer{a, b} {
					if tracked {
						g.configureTextures(pack, NewTextureDump())
					}
					textureTestScene(g)
					g.margin = 64
					g.sides = make(map[[4]int32]*sideCanvas)
					g.clipX0, g.clipX1, g.clipY0, g.clipY1 = 32, 287, 0, 255
					g.prepareSides()
				}
				for i := 0; i < 256; i++ {
					f := polyFlags{gouraud: i&1 != 0, textured: i&2 != 0, modulate: i&4 != 0, mode: i%5 - 1, tm: uint32(i / 8 % 4)}
					ig := iGroup{rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32()}
					idl := iDeltas{rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32()}
					x := int32(rng.Intn(500)-100) << shift
					y := int32(rng.Intn(520)-4) << shift
					bound := x + int32(rng.Intn(96)-8)*(1<<shift)
					if i%16 == 0 {
						// 真正替換命中及重複子貼素。
						f = polyFlags{textured: true, modulate: i&16 != 0, gouraud: i&32 != 0, tm: 0, mode: i%5 - 1}
						ig = iGroup{r: 128 << 24, g: 128 << 24, b: 128 << 24}
						idl = iDeltas{duDx: 1 << (24 - shift), dvDy: 1 << (24 - shift)}
						x, y, bound = 0, 0, 8<<shift
					}
					for _, g := range []*Renderer{a, b} {
						g.dtd = i&8 != 0
						g.maskSetOR = uint16(i&16) << 11
						g.maskEvalAND = uint16(i&32) << 10
						g.displayMode = uint32(i&64) >> 1
						g.field = i&128 != 0
						if i%16 == 0 {
							g.clipX0 = 0
							g.clipY1 = 511
						} else {
							g.clipX0 = 32
							g.clipY1 = 255
						}
						g.prepareSides()
						window := uint32(i*733) & 0xfffff
						if i%16 == 0 {
							window = 0
						}
						g.WriteGP0(0xe2000000 | window)
					}
					a.drawSpan(y, x, bound, ig, &idl, f)
					b.spanBeforeInvariantHoist(y, x, bound, ig, &idl, f)
					if i%32 == 31 {
						if !slices.Equal(a.VRAM, b.VRAM) {
							t.Fatalf("VRAM differs at case %d", i)
						}
						ca, cb := *a, *b
						ca.VRAM = nil
						cb.VRAM = nil
						if !reflect.DeepEqual(ca, cb) {
							t.Fatalf("cache/origin/dump/side state differs at case %d", i)
						}
					}
				}
				if tracked && a.textures.stats.ReplacedSamples == 0 {
					t.Fatal("no replacement coverage")
				}
			})
		}
	}
}

func BenchmarkSpanInvariantHoist(b *testing.B) {
	for _, gouraud := range []bool{false, true} {
		for _, before := range []bool{true, false} {
			name := "current"
			if before {
				name = "before"
			}
			b.Run(fmt.Sprintf("gouraud%v/%s", gouraud, name), func(b *testing.B) {
				g := newRenderer(1)
				g.clipX0, g.clipX1, g.clipY0, g.clipY1 = 0, 1023, 0, 511
				for i := range g.VRAM {
					g.VRAM[i] = 0x1357
				}
				for i := range g.clutCache {
					g.clutCache[i] = uint16(i*733) | 1
				}
				ig := iGroup{r: 128 << 24, g: 128 << 24, b: 128 << 24}
				idl := iDeltas{duDx: 1 << 23, dvDy: 1 << 23, drDx: 1 << 16, dgDx: 1 << 17, dbDx: 1 << 18}
				f := polyFlags{textured: true, gouraud: gouraud, modulate: true, tm: 0, mode: -1}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if before {
						g.spanBeforeInvariantHoist(128, 256, 768, ig, &idl, f)
					} else {
						g.drawSpan(128, 256, 768, ig, &idl, f)
					}
				}
			})
		}
	}
}

// 重複取樣與目的／貼圖重疊時，仍保持硬體的舊快取資料。
func TestSpanRepeatedUVFeedback(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		for _, tm := range []uint32{0, 1, 2, 3} {
			a, b := newRenderer(shift), newRenderer(shift)
			for _, g := range []*Renderer{a, b} {
				g.clipX0, g.clipX1, g.clipY0, g.clipY1 = 0, 1023, 0, 511
				for i := range g.VRAM {
					g.VRAM[i] = uint16(i*733) | 1
				}
				for i := range g.clutCache {
					g.clutCache[i] = uint16(i*913) | 1
				}
			}
			for _, du := range []uint32{0, 1, 1 << 20, 1 << 23, 1 << 24, ^uint32(0)} {
				ig := iGroup{r: 160 << 24, g: 128 << 24, b: 96 << 24}
				idl := iDeltas{duDx: du}
				f := polyFlags{textured: true, modulate: true, tm: tm, mode: 1}
				a.drawSpan(0, 0, 128, ig, &idl, f)
				b.spanBeforeInvariantHoist(0, 0, 128, ig, &idl, f)
				if !slices.Equal(a.VRAM, b.VRAM) || !reflect.DeepEqual(a.texCache, b.texCache) {
					t.Fatalf("feedback shift=%d tm=%d du=%d", shift, tm, du)
				}
			}
		}
	}
}

func TestSpanRawAndNeutralExact(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		a, b := newRenderer(shift), newRenderer(shift)
		for _, g := range []*Renderer{a, b} {
			g.clipX0, g.clipX1, g.clipY0, g.clipY1 = 0, 1023, 0, 511
			for i := range g.VRAM {
				g.VRAM[i] = uint16(i*733) | 1
			}
			for i := range g.clutCache {
				g.clutCache[i] = uint16(i*913) | 1
			}
		}
		for i := 0; i < 80; i++ {
			f := polyFlags{textured: true, modulate: i&1 != 0, gouraud: i&2 != 0, tm: uint32(i / 4 % 4), mode: i%5 - 1}
			ig := iGroup{r: 128 << 24, g: 128 << 24, b: 128 << 24}
			idl := iDeltas{duDx: 1 << 23, drDx: 1 << 20, dgDx: 1 << 21, dbDx: 1 << 22}
			a.dtd = i&4 != 0
			b.dtd = a.dtd
			a.drawSpan(0, 0, 128, ig, &idl, f)
			b.spanBeforeInvariantHoist(0, 0, 128, ig, &idl, f)
			if !slices.Equal(a.VRAM[:128], b.VRAM[:128]) || !reflect.DeepEqual(a.texCache, b.texCache) {
				t.Fatalf("raw/neutral shift=%d case=%d", shift, i)
			}
		}
		if !slices.Equal(a.VRAM, b.VRAM) {
			t.Fatalf("VRAM outside sampled row differs shift=%d", shift)
		}
	}
}
