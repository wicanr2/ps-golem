package gpu

import (
	"github.com/wicanr2/ps-golem/render/hd"
	"slices"
	"testing"
)

func TestNativePlaneTraceBeforeCommand(t *testing.T) {
	a, b := New(), New()
	p := hd.NewNativePlane(nil)
	b.SetNativePlane(p)
	defer func() { b.SetNativePlane(nil); p.Close() }()
	var seenA, seenB []uint16
	a.Trace = func(uint32) { seenA = append(seenA, a.VRAMSnapshot()[0]) }
	b.Trace = func(uint32) { seenB = append(seenB, b.VRAMSnapshot()[0]) }
	for _, v := range []uint32{0xa0000000, 0, 0x00010001, 0x001f} {
		a.WriteGP0(v)
		b.WriteGP0(v)
	}
	if !slices.Equal(seenA, seenB) || len(seenA) != 4 || seenA[3] != 0 {
		t.Fatalf("Trace observed new pixels early: %v/%v", seenA, seenB)
	}
	if a.VRAMSnapshot()[0] != 0x001f || b.VRAMSnapshot()[0] != 0x001f {
		t.Fatal("new command missing after Trace")
	}
}
