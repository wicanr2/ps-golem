package hd

import (
	"image"
	"testing"
	"time"
)

func TestAsyncSnapshotOrderAndClose(t *testing.T) {
	s := NewShadow(1)
	s.GP1(0x03000000)
	upload := func(v uint32) {
		for _, w := range []uint32{0xa0000000, 0, 1 | 1<<16, v} {
			s.GP0(w)
		}
	}
	upload(0x001f)
	var first *image.RGBA
	done := make(chan struct{})
	s.RequestScreen(nil, func(img *image.RGBA) { first = img; close(done) })
	upload(0x03e0)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("callback timeout")
	}
	last := s.Screen()
	if first.Pix[0] != 255 || first.Pix[1] != 0 || last.Pix[0] != 0 || last.Pix[1] != 255 {
		t.Fatalf("snapshot ordering changed: first=%v last=%v", first.Pix[:4], last.Pix[:4])
	}
	closed := make(chan struct{})
	s.RequestScreen(nil, func(*image.RGBA) { close(closed) })
	fenced := make(chan struct{})
	s.Fence(func() { close(fenced) })
	s.Close()
	select {
	case <-fenced:
	default:
		t.Fatal("Close returned before fence")
	}
	select {
	case <-closed:
	default:
		t.Fatal("Close returned before callback")
	}
}
