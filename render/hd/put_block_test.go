// 保留優化前 put；render/hd/gpu.go SHA-256 35ba1efda7026e2d17b80ef1b79818c6223896936eeda0172516cec1a2a40be9。
// 參照來源與授權同 gpu.go；用於 016 §8.2 的獨立逐像素 oracle。
package hd

import (
	"slices"
	"testing"
)

func (g *Renderer) putBeforeBlockOptimization(x, y uint32, v uint16) {
	if g.textures != nil {
		g.textures.setOrigin((y&511)*1024+(x&1023), textureOrigin{})
	}
	n := uint32(1) << g.shift
	x, y = (x&1023)<<g.shift, (y&511)<<g.shift
	for dy := uint32(0); dy < n; dy++ {
		row := g.VRAM[(y+dy)<<(10+g.shift):]
		for dx := uint32(0); dx < n; dx++ {
			row[x+dx] = v
		}
	}
}

func TestPutBlockPreservesVRAM(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g, want := newRenderer(shift), newRenderer(shift)
		seed := uint32(0x7481)
		next := func() uint32 { seed ^= seed << 13; seed ^= seed >> 17; seed ^= seed << 5; return seed }
		for i := 0; i < 4096; i++ {
			x, y, v := next(), next(), uint16(next())
			g.put(x, y, v)
			want.putBeforeBlockOptimization(x, y, v)
		}
		for _, xy := range [][2]uint32{{0, 0}, {1023, 511}, {1024, 512}, {0xffffffff, 0xffffffff}, {1023, 0}, {0, 511}} {
			g.put(xy[0], xy[1], 0x8a51)
			want.putBeforeBlockOptimization(xy[0], xy[1], 0x8a51)
		}
		if !slices.Equal(g.VRAM, want.VRAM) {
			t.Fatalf("shift=%d: VRAM differs", shift)
		}
	}
}
func BenchmarkPutBlock(b *testing.B) {
	for _, before := range []bool{true, false} {
		name := "current"
		if before {
			name = "before"
		}
		b.Run(name, func(b *testing.B) {
			g := newRenderer(1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x, y := uint32(i)&1023, uint32(i>>10)&511
				if before {
					g.putBeforeBlockOptimization(x, y, uint16(i))
				} else {
					g.put(x, y, uint16(i))
				}
			}
		})
	}
}
