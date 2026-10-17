package hd

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"testing"
)

func TestPGXPBatchBoundary(t *testing.T) {
	s := NewShadow(2)
	defer s.Close()
	for range batchWords - 5 {
		s.GP1(0x02000000)
	}
	for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x200000ff} {
		s.GP0(w)
	}
	raw := uint32(10 | 10<<16)
	s.GP0Precision(raw, pgxp.Value{X: 10.75, Y: 10.75, Raw: raw, Flags: pgxp.ValidXY})
	for _, w := range []uint32{30 | 10<<16, 10 | 30<<16} {
		s.GP0(w)
	}
	h, v := s.PrecisionStats()
	if h != 1 || v != 3 {
		t.Fatalf("跨批次精度命中=%d/%d", h, v)
	}
}

func TestPGXPRejectChangedWord(t *testing.T) {
	g := newRenderer(2)
	g.WriteGP0(0x200000ff)
	g.nextPrecision = pgxp.Value{X: 10.75, Y: 10.75, Raw: 10 | 10<<16, Flags: pgxp.ValidXY}
	for _, w := range []uint32{11 | 10<<16, 30 | 10<<16, 10 | 30<<16} {
		g.WriteGP0(w)
	}
	if g.pgxpVertices != 0 || g.polygonVertices != 3 {
		t.Fatal("改字沒有回退")
	}
}
