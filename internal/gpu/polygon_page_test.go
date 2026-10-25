package gpu

import (
	"fmt"
	"testing"
)

// 007-gpu-and-dma §10：期望值由逐分量整數公式獨立計算。
// 不使用被測的 blend，也不以原生與影子互相比較取代公式。
func TestPolygonOwnTPageBlend(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		t.Run(fmt.Sprint(1<<shift), func(t *testing.T) {
			g := New()
			s := NewShadow(shift)
			s.SetSideMargin(48)
			g.SetShadow(s)
			defer func() { g.SetShadow(nil); s.Close() }()
			send := func(words ...uint32) {
				for _, w := range words {
					g.WriteGP0(w)
				}
			}
			xy := func(x, y int) uint32 { return uint32(uint16(x)) | uint32(uint16(y))<<16 }
			g.WriteGP1(0x03000000)
			g.WriteGP1(0x08000000)
			send(0xe3000000, 0xe4000000|255|239<<10)
			cases := 0
			for previous := 0; previous < 4; previous++ {
				for current := 0; current < 4; current++ {
					for _, texel := range []uint16{0xa108, 0x2108, 0x8000, 0} {
						for _, cmd := range []uint32{0x25, 0x27, 0x2d, 0x2f, 0x35, 0x37, 0x3d, 0x3f} {
							// 中央及左右側邊同底色。15bpp 貼圖直接提供 STP 與零貼素。
							send(0x60606060, 0xffe0, 336|32<<16)
							send(0xa0000000, xy(640, 256), xy(8, 8))
							for i := 0; i < 32; i++ {
								send(uint32(texel) | uint32(texel)<<16)
							}
							for _, x := range []int{-8, 8, 260} {
								send(0xe1000000 | 0x11a | uint32(previous)<<5)
								nv := 3
								if cmd&8 != 0 {
									nv = 4
								}
								words := []uint32{}
								for i := 0; i < nv; i++ {
									if i == 0 || cmd&16 != 0 {
										c := uint32(0x808080)
										if i == 0 {
											c |= cmd << 24
										}
										words = append(words, c)
									}
									px, py := x, 8
									if i&1 != 0 {
										px += 8
									}
									if i&2 != 0 {
										py += 8
									}
									uv := uint32((px - x) | ((py - 8) << 8))
									if i == 1 {
										uv |= (0x11a | uint32(current)<<5) << 16
									}
									words = append(words, xy(px, py), uv)
								}
								send(words...)
							}
							want := uint16(0x318c)
							if texel != 0 {
								want = texel
								if cmd&2 != 0 && texel&0x8000 != 0 {
									want = 0x8000
									for component := 0; component < 3; component++ {
										f, b := int(texel>>(5*component)&31), 12
										var c int
										/* psx-spx Semi-transparency：逐通道截斷及飽和。 */
										switch current {
										case 0:
											c = b/2 + f/2
										case 1:
											c = min(31, b+f)
										case 2:
											c = max(0, b-f)
										case 3:
											c = min(31, b+f/4)
										}
										want |= uint16(c) << (5 * component)
									}
								}
							}
							if got := g.vram[10*1024+10]; got != want {
								t.Fatalf("native prev=%d current=%d cmd=%x tex=%x got=%x want=%x", previous, current, cmd, texel, got, want)
							}
							im := s.Screen()
							n := 1 << shift
							for _, x := range []int{-6, 10, 262} {
								for dy := 0; dy < n; dy++ {
									for dx := 0; dx < n; dx++ {
										got := im.RGBAAt((43+x)*n+dx, 10*n+dy)
										for component, c := range []uint8{got.R, got.G, got.B} {
											v := uint8(want >> (5 * component) & 31)
											if c != v<<3|v>>2 {
												t.Fatalf("HD x=%d prev=%d current=%d cmd=%x tex=%x got=%v word=%x", x, previous, current, cmd, texel, got, want)
											}
										}
									}
								}
							}
							cases++
						}
					}
				}
			}
			t.Logf("%d cases, native and HD center/left/right", cases)
		})
	}
}
