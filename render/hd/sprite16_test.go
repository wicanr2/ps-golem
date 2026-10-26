package hd

import (
	"reflect"
	"testing"
)

func TestSprite16ConsumerFallbackMatchesGP0(t *testing.T) {
	for _, page := range []uint32{8, 9} {
		a, b := NewShadow(1), NewShadow(1)
		func() {
			defer a.Close()
			defer b.Close()
			for _, s := range []*Shadow{a, b} {
				for _, v := range []uint32{0xe1000000 | page, 0xe3000000, 0xe4000000 | 31 | 31<<10, 0x600000ff, 0, 16 | 16<<16} {
					s.GP0(v)
				}
			}
			j := Sprite16{X: 2, Y: 3, Page: 8}
			a.GP0(0x7d000000)
			a.GP0(2 | 3<<16)
			a.GP0(0)
			b.Sprite16(j)
			if !reflect.DeepEqual(a.VRAMCopy(), b.VRAMCopy()) {
				t.Fatal("typed or consumer fallback differs")
			}
		}()
	}
}
