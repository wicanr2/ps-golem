package hd

import "testing"

func TestNativePlaneRoleAndBound(t *testing.T) {
	p := NewNativePlane(nil)
	defer p.Close()
	display := NewShadow(2)
	defer display.Close()
	for i := 0; i < nativePendingLimit+1; i++ {
		p.GP0(0xe1000000)
	}
	if p.pending != 1 {
		t.Fatalf("pending input is not bounded: %d", p.pending)
	}
	p.GP1(0x03000000)
	p.Field(true)
	for _, v := range []uint32{0xa0000000, 0, 0x00010003, 0x00008000, 0xffff001f, 0xc0000000, 0, 0x00010003} {
		p.GP0(v)
		display.GP0(v)
	}
	var native, shown bool
	p.shadow.read(func(g *Renderer) { native = g.inCmd == inFBRead && g.nativeCompatibility })
	display.read(func(g *Renderer) { shown = g.inCmd == inFBRead || g.nativeCompatibility })
	if !native || shown {
		t.Fatal("C0 role isolation failed")
	}
	if p.ReadData() != 0x00008000 || p.ReadData() != 0x0000001f {
		t.Fatal("odd readback differs")
	}
	snapshot := p.Snapshot()
	snapshot.VRAM[0] = 0xffff
	if p.Snapshot().VRAM[0] != 0x8000 {
		t.Fatal("snapshot owns core memory")
	}
	p.GP1(0)
	p.shadow.read(func(g *Renderer) {
		if !g.nativeCompatibility {
			t.Error("reset discarded native role")
		}
	})
}
