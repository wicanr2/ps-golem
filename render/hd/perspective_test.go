package hd

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"math"
	"testing"
)

func TestTexturePerspectiveInverseHomography(t *testing.T) {
	g := newRenderer(1)
	g.texturePerspective = true
	v := [4]triVertex{{x: 368, y: 48, z: 400, u: 0, v: 0}, {x: 560, y: 48, z: 400, u: 127, v: 0}, {x: 336, y: 304, z: 300, u: 0, v: 127}, {x: 592, y: 304, z: 300, u: 127, v: 127}}
	for _, tri := range [][3]triVertex{{v[0], v[1], v[2]}, {v[1], v[2], v[3]}} {
		g.setPerspectiveTriangle(tri, polyFlags{textured: true})
		if g.perspectiveTriangle == nil {
			t.Fatal("no perspective plane")
		}
		for y := 60; y < 290; y += 7 {
			for x := 380; x < 540; x += 9 {
				px, py := (float64(x)+0.5)/2, (float64(y)+0.5)/2
				fraction := (py - 24) / 128
				width := 96 + 32*fraction
				expectedU := 127 * (px - (232 - width/2)) / width
				expectedV := 127 * (fraction / 300) / ((1-fraction)/400 + fraction/300)
				u, w := g.perspectiveUV(int32(x), int32(y), 0, 0)
				for i, pair := range [][2]float64{{float64(u), math.Floor(expectedU * 16777216)}, {float64(w), math.Floor(expectedV * 16777216)}} {
					if math.Abs(pair[0]-pair[1]) > 1 {
						t.Fatalf("inverse homography xy=%d,%d channel=%d got=%v want=%v", x, y, i, pair[0], pair[1])
					}
				}
			}
		}
	}
	// 缺深度、相同深度及無效深度保持既有仿射取樣。
	for _, z := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		tri := [3]triVertex{v[0], v[1], v[2]}
		tri[0].z = z
		g.setPerspectiveTriangle(tri, polyFlags{textured: true})
		u, w := g.perspectiveUV(400, 200, 123, 456)
		if u != 123 || w != 456 {
			t.Fatal("invalid depth did not fall back")
		}
	}
	tri := [3]triVertex{v[0], v[1], v[2]}
	for i := range tri {
		tri[i].z = 300
	}
	g.setPerspectiveTriangle(tri, polyFlags{textured: true})
	u, w := g.perspectiveUV(400, 200, 123, 456)
	if u != 123 || w != 456 {
		t.Fatal("constant depth changed affine UV")
	}
}

func TestTexturePerspectiveDepthBatchBoundary(t *testing.T) {
	s := NewShadow(1)
	defer s.Close()
	s.SetTexturePerspective(true)
	for i := 0; i < batchWords-3; i++ {
		s.GP1(0x02000000)
	}
	for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x25000000} {
		s.GP0(w)
	}
	for i, p := range [][2]uint32{{10, 10}, {100, 10}, {10, 100}} {
		xy := p[0] | p[1]<<16
		s.GP0Precision(xy, pgxp.Value{X: float32(p[0]), Y: float32(p[1]), Z: float32(300 + i*100), Raw: xy, Flags: pgxp.ValidXY})
		uv := uint32(0)
		if i == 1 {
			uv = 100 | 0x100<<16
		}
		if i == 2 {
			uv = 100 << 8
		}
		s.GP0(uv)
	}
	stats := s.TexturePerspectiveStatistics()
	if stats.ValidDepthTriangles != 1 || stats.CorrectedTriangles != 1 {
		t.Fatalf("depth lost across batches: %+v", stats)
	}
}
