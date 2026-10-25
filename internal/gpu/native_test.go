package gpu

import (
	"github.com/wicanr2/ps-golem/render/hd"
	"reflect"
	"testing"
)

func TestNativePlaneDetach(t *testing.T) {
	a, b := New(), New()
	p := hd.NewNativePlane(nil)
	b.SetNativePlane(p)
	defer p.Close()
	send := func(words ...uint32) {
		for _, v := range words {
			a.WriteGP0(v)
			b.WriteGP0(v)
		}
	}
	send(0xe3000000, 0xe407ffff, 0xe1000600, 0xa0000000, 0, 0x00010003, 0x8000001f, 0x7fff,
		0x6000ff00, 20|20<<16, 8|8<<16, 0xc0000000, 0, 3|1<<16)
	for i := 0; i < 2; i++ {
		if a.ReadData() != b.ReadData() {
			t.Fatal("readback differs")
		}
	}
	held := b.VRAMSnapshot()
	held[0] ^= 0xffff
	if reflect.DeepEqual(held, a.VRAMSnapshot()) {
		t.Fatal("snapshot fixture invalid")
	}
	if !reflect.DeepEqual(a.Clone(), b.Clone()) {
		t.Fatal("attached state differs")
	}
	if b.NativeRasterWork()[0] == 0 {
		t.Fatal("native raster was not delegated")
	}
	done := make(chan struct{})
	if !b.RequestFence(func() { close(done) }) {
		t.Fatal("native-only fence missing")
	}
	<-done
	b.SetNativePlane(nil)
	p.Close()
	p.Close()
	// Continue a partially buffered command after detach, without resetting GPU.
	send(0xa0000000, 5|5<<16, 3|1<<16, 0x83e00001, 0x7c00)
	if !reflect.DeepEqual(a.Clone(), b.Clone()) {
		t.Fatal("detach lost pixels, cache or parser state")
	}
}

func TestNativePlaneRejectReplacement(t *testing.T) {
	g := New()
	p := hd.NewNativePlane(nil)
	g.SetNativePlane(p)
	defer func() { g.SetNativePlane(nil); p.Close() }()
	defer func() {
		if recover() == nil {
			t.Error("replaced an attached native plane")
		}
	}()
	g.SetNativePlane(p)
}
