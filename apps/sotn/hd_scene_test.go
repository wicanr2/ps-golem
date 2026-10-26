package sotn

import (
	"github.com/wicanr2/ps-golem/machine"
	"github.com/wicanr2/ps-golem/render/hd"
	"reflect"
	"testing"
)

func TestHDSceneOriginalAndUnknownContext(t *testing.T) {
	m, e := machine.New(make([]byte, 512<<10))
	if e != nil {
		t.Fatal(e)
	}
	before := m.Clone()
	if _, e = AttachHDSceneRenderer(m, false); e == nil {
		t.Fatal("original no-HD machine accepted renderer")
	}
	if !reflect.DeepEqual(before, m.Clone()) {
		t.Fatal("original state changed")
	}
	s := hd.NewShadow(1)
	m.GPU.SetShadow(s)
	defer func() { m.GPU.SetShadow(nil); s.Close() }()
	renderer, e := AttachHDSceneRenderer(m, false)
	if e != nil {
		t.Fatal(e)
	}
	defer renderer.Close()
	before = m.Clone()
	ok, _ := tryHDLayerCall(m, 100)
	if ok || !reflect.DeepEqual(before, m.Clone()) {
		t.Fatal("unknown game context mutated machine")
	}
}
