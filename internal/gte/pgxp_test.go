package gte

import (
	"math"
	"testing"
)

type projectionLog struct{ points [][3]float32 }

func (p *projectionLog) Projected(x, y float32, raw uint32, vertex int, last bool) {
	p.points = append(p.points, [3]float32{x, y, float32(raw)})
}

func TestPGXPHardwareFuzzLog(t *testing.T) {
	cases := loadFuzzLog(t)
	if len(cases) != 1150 {
		t.Fatal("語料筆數改變")
	}
	for _, c := range cases {
		g := GTE{}
		p := &projectionLog{}
		g.SetProjectionObserver(p)
		for i, v := range c.In {
			g.Write(i, v)
		}
		if c.HasCmd {
			g.Execute(c.Cmd)
		}
		for i, want := range c.Out {
			if got := g.Read(i); got != want {
				t.Fatalf("Test %d r%d=%08x，預期 %08x", c.Number, i, got, want)
			}
		}
		for _, pt := range p.points {
			for _, x := range pt[:2] {
				if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) || x < -1024 || x > 1023 {
					t.Fatalf("無效投影 %v", pt)
				}
			}
		}
	}
	t.Log("啟用 PGXP，1150 筆各 64 個暫存器全部相符")
}

func TestPGXPProjection(t *testing.T) {
	g := GTE{}
	p := &projectionLog{}
	g.SetProjectionObserver(p)
	g.Write(32, 4096)
	g.Write(34, 4096)
	g.Write(36, 4096)
	g.Write(58, 100)
	for v := 0; v < 3; v++ {
		g.Write(v*2, uint32(32+v)|32<<16)
		g.Write(v*2+1, 300)
	}
	native := g
	native.SetProjectionObserver(nil)
	native.Execute(0x80030)
	g.Execute(0x80030)
	if len(p.points) != 3 {
		t.Fatal(p.points)
	}
	if math.Abs(float64(p.points[0][0]-float32(32.0/3))) > 0.00001 {
		t.Fatal(p.points)
	}
	for v := 0; v < 3; v++ {
		if got := g.Read(12 + v); got != native.Read(12+v) {
			t.Fatalf("RTPT FIFO %d=%x", v, got)
		}
	}
}
