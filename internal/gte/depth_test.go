package gte

import (
	"math"
	"testing"
)

type depthLog struct {
	projectionLog
	depths []float32
}

func (p *depthLog) ProjectedDepth(x, y, z float32, raw uint32, vertex int, last bool) {
	p.Projected(x, y, raw, vertex, last)
	p.depths = append(p.depths, z)
}
func TestPGXPDepthHardwareRegisters(t *testing.T) {
	cases := loadFuzzLog(t)
	if len(cases) != 1150 {
		t.Fatal("GTE corpus count changed")
	}
	for _, c := range cases {
		g := GTE{}
		p := &depthLog{}
		g.SetProjectionObserver(p)
		for i, v := range c.In {
			g.Write(i, v)
		}
		if c.HasCmd {
			g.Execute(c.Cmd)
		}
		for i, want := range c.Out {
			if got := g.Read(i); got != want {
				t.Fatalf("case %d reg%d=%x want=%x", c.Number, i, got, want)
			}
		}
		for _, z := range p.depths {
			if z < 0 || math.IsNaN(float64(z)) || math.IsInf(float64(z), 0) {
				t.Fatal(z)
			}
		}
	}
}
func TestPGXPDepthProjectionAndFallback(t *testing.T) {
	for _, tc := range []struct {
		x, z int
		want float32
	}{{32, 300, 300}, {32, 70000, 70000}, {32, 50, 0}, {32767, 300, 0}, {32, -1, 0}} {
		g := GTE{}
		p := &depthLog{}
		g.SetProjectionObserver(p)
		g.Write(32, 4096)
		g.Write(34, 4096)
		g.Write(36, 4096)
		g.Write(58, 100)
		g.Write(0, uint32(uint16(tc.x))|32<<16)
		g.Write(1, uint32(uint16(tc.z)))
		if tc.z == 70000 {
			g.Write(39, uint32(tc.z-300))
			g.Write(1, 300)
		}
		g.Execute(0x80001)
		if len(p.depths) != 1 || p.depths[0] != tc.want {
			t.Fatalf("%+v got=%v", tc, p.depths)
		}
	}
}
