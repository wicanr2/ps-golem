package pgxp

import "testing"

func TestDepthHalfwordProvenance(t *testing.T) {
	for _, op := range []uint32{0x21, 0x25} {
		p := New()
		raw := uint32(12 | 34<<16)
		p.ProjectedDepth(12.75, 34.5, 300, raw, 0, true)
		p.StoreCOP2(14, 0x1000, raw)
		p.Load(1, 0x1000, 12, op, 1, 2)
		p.Store(1, 0x2000, 12, 0x29, 3)
		p.Load(2, 0x1002, 34, op, 4, 5)
		p.Store(2, 0x2002, 34, 0x29, 6)
		if v := p.ReadMemory(0x2000, raw); v.Z != 300 || !v.Valid(raw) {
			t.Fatalf("same vertex halves lost: %+v", v)
		}
		p.ProjectedDepth(12.75, 34.5, 300, raw, 0, true)
		p.StoreCOP2(14, 0x3000, raw)
		p.Load(3, 0x3002, 34, op, 7, 8)
		p.Store(3, 0x2002, 34, 0x29, 9)
		if v := p.ReadMemory(0x2000, raw); v.Z != 0 {
			t.Fatal("mixed projected vertices accepted", v)
		}
		p.Load(4, 0x3000, raw, 0x23, 10, 11)
		p.Store(4, 0x4000, raw, 0x2b, 12)
		if v := p.ReadMemory(0x4000, raw); v.Z != 300 {
			t.Fatal("full word depth lost", v)
		}
		if v := p.ReadMemory(0x4000, raw+1); v.Z != 0 {
			t.Fatal("raw mutation kept depth", v)
		}
		p.Store(4, 0x3000, 12, 0x28, 13)
		if v := p.ReadMemory(0x3000, raw); v.Z != 0 {
			t.Fatal("byte overwrite kept depth", v)
		}
	}
}
