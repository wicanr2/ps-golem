package hd

import (
	"fmt"
	"image"
	"reflect"
	"slices"
	"testing"
)

// 記憶體裝置只用來測CPU／GPU相依與完整狀態；實際Kage另由hdgpu測試。
type memoryRaster struct{ g *Renderer }

func (d *memoryRaster) Upload(b image.Rectangle, w []uint16) {
	stride := 1024 << d.g.shift
	for y := 0; y < b.Dy(); y++ {
		copy(d.g.VRAM[(y+b.Min.Y)*stride+b.Min.X:][:b.Dx()], w[y*b.Dx():])
	}
}
func (d *memoryRaster) Read(b image.Rectangle, w []uint16) {
	stride := 1024 << d.g.shift
	for y := 0; y < b.Dy(); y++ {
		copy(w[y*b.Dx():], d.g.VRAM[(y+b.Min.Y)*stride+b.Min.X:][:b.Dx()])
	}
}
func (*memoryRaster) Flush() {}
func (*memoryRaster) Close() {}
func (d *memoryRaster) Draw(j RasterJob) {
	bg := make([]uint16, j.Rect.Dx()*j.Rect.Dy())
	d.Read(j.Rect, bg)
	stride := 1024 << d.g.shift
	for y := j.Rect.Min.Y; y < j.Rect.Max.Y; y++ {
		for x := j.Rect.Min.X; x < j.Rect.Max.X; x++ {
			i := (y-j.Rect.Min.Y)*j.Rect.Dx() + x - j.Rect.Min.X
			src := j.Texels[i*4 : i*4+4]
			if src[3] == 0 {
				continue
			}
			b := bg[i]
			if j.Sprite && src[2] == 0 {
				nx, ny := (x-j.Rect.Min.X)>>j.Shift<<j.Shift, (y-j.Rect.Min.Y)>>j.Shift<<j.Shift
				b = bg[ny*j.Rect.Dx()+nx]
			}
			if b&j.MaskEval != 0 {
				continue
			}
			c := [3]uint32{}
			for k := range c {
				c[k] = (j.Color[k] + j.DX[k]*uint32(x) + j.DY[k]*uint32(y)) >> 24
			}
			dx, dy := uint32(3), uint32(2)
			if j.Dither {
				dx, dy = (uint32(x)>>j.Shift)&3, (uint32(y)>>j.Shift)&3
			}
			f := uint16(src[0]) | uint16(src[1])<<8
			if j.Textured {
				if j.Modulate {
					f = d.g.modTexel(dy, dx, f, c[0], c[1], c[2])
				}
			} else {
				f = 0x8000 | uint16(d.g.ditherLUT[dy][c[0]][dx]) | uint16(d.g.ditherLUT[dy][c[1]][dx])<<5 | uint16(d.g.ditherLUT[dy][c[2]][dx])<<10
			}
			if j.Mode >= 0 && f&0x8000 != 0 {
				f = blend(j.Mode, b, f)
			}
			if !j.Textured {
				f &= 0x7fff
			}
			d.g.VRAM[y*stride+x] = f | j.MaskSet
		}
	}
}

func TestRasterSourceCacheFullState(t *testing.T) {
	for _, shift := range []uint{0, 1, 2} {
		for _, tracked := range []bool{false, true} {
			t.Run(fmt.Sprintf("N%d/tracked%v", 1<<shift, tracked), func(t *testing.T) {
				a, b := newRenderer(shift), newRenderer(shift)
				var pack *TexturePack
				if tracked {
					pack = textureTestPack(t, []uint16{0x1111, 0x1111}, 4, 2)
				}
				for _, g := range []*Renderer{a, b} {
					if tracked {
						g.configureTextures(pack, NewTextureDump())
					}
					g.margin = 48
					g.sides = make(map[[4]int32]*sideCanvas)
				}
				a.attachRaster(&memoryRaster{g: newRenderer(shift)})
				for _, g := range []*Renderer{a, b} {
					textureTestScene(g)
					send(g, 0xe407ffff, 0xe1000011)
					for mode := 0; mode < 4; mode++ {
						send(g, 0xe1000011|uint32(mode)<<5|1<<9)
						for i := uint32(0); i < 5; i++ {
							x, y := uint32(10+i*17), uint32(20+i*9)
							send(g, 0x34f2197c, y<<16|x, 0x00400000, 0x0084a1b3, (y+11)<<16|x+24, 0x00110004|uint32(mode)<<21, 0x00ab291c, (y+21)<<16|x+3, 0x00000200)
							send(g, 0x66888888, (y+2)<<16|x+2, 0x00400000, 2<<16|4)
							send(g, 0xe1001011|uint32(mode)<<5, 0x65808080, (y+6)<<16|x, 0x00400000, 2<<16|1)
						}
						// 對已被GPU寫過的區域上傳、複製、遮罩，測髒區相依及來源釋放。
						send(g, 0xa0000000, 20<<16|10, 1<<16|2, 0x90128457)
						send(g, 0x80000000, 20<<16|10, 24<<16|15, 2<<16|4)
						send(g, 0xe6000003, 0x60102030, 30<<16|10, 2<<16|3, 0xe6000000)
					}
					// 寫貼圖區不失效CLUT；保留快取與來源存活的相同次序。
					send(g, 0x02001122, 256<<16|64, 2<<16|16)
					send(g, 0x65808080, 90<<16|10, 0x00400000, 2<<16|4)
				}
				a.syncRaster()
				if !slices.Equal(a.VRAM, b.VRAM) {
					for i, v := range a.VRAM {
						if v != b.VRAM[i] {
							t.Fatalf("VRAM %d GPU=%04x CPU=%04x", i, v, b.VRAM[i])
						}
					}
				}
				if a.raster.stats.Triangles == 0 || a.raster.stats.Sprites == 0 {
					t.Fatal("GPU path not covered")
				}
				a.raster = nil
				if !reflect.DeepEqual(a, b) {
					if !reflect.DeepEqual(a.texCache, b.texCache) {
						t.Fatal("texture cache differs")
					}
					if !reflect.DeepEqual(a.textures, b.textures) {
						t.Fatal("texture tracking/dump/reference state differs")
					}
					t.Fatal("renderer state differs")
				}
			})
		}
	}
}
