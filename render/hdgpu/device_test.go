package hdgpu_test

import (
	"fmt"
	"image"
	"os"
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/ps-golem/render/hd"
	"github.com/wicanr2/ps-golem/render/hdgpu"
)

type testLoop struct {
	started     bool
	ready, done chan struct{}
}

func (g *testLoop) Update() error {
	if !g.started {
		close(g.ready)
		g.started = true
	}
	hdgpu.Service(g.started)
	select {
	case <-g.done:
		return eb.Termination
	default:
		return nil
	}
}
func (*testLoop) Draw(*eb.Image)             {}
func (*testLoop) Layout(int, int) (int, int) { return 32, 32 }
func TestMain(m *testing.M) {
	g := &testLoop{ready: make(chan struct{}), done: make(chan struct{})}
	code := 1
	go func() { <-g.ready; code = m.Run(); close(g.done) }()
	if err := eb.RunGame(g); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// CPU oracle 保留既有 render/hd/raster.go 的 bit 運算，不從 shader 導出期望值。
func blend(mode int, bg, fore uint16) uint16 {
	switch mode {
	case 0:
		b := uint32(bg | 0x8000)
		f := uint32(fore)
		return uint16(((f + b) - ((f ^ b) & 0x0421)) >> 1)
	case 1:
		b := uint32(bg &^ 0x8000)
		f := uint32(fore)
		s := f + b
		c := (s - ((f ^ b) & 0x8421)) & 0x8420
		return uint16((s - c) | (c - (c >> 5)))
	case 2:
		b := uint32(bg | 0x8000)
		f := uint32(fore &^ 0x8000)
		d := b - f + 0x108420
		c := (d - ((b ^ f) & 0x108420)) & 0x108420
		return uint16((d - c) & (c - (c >> 5)))
	case 3:
		b := uint32(bg &^ 0x8000)
		f := uint32((fore>>2)&0x1ce7 | 0x8000)
		s := f + b
		c := (s - ((f ^ b) & 0x8421)) & 0x8420
		return uint16((s - c) | (c - (c >> 5)))
	}
	return fore
}
func reference(j hd.RasterJob, bg uint16, src []byte, x, y int) uint16 {
	if src[3] == 0 || bg&j.MaskEval != 0 {
		return bg
	}
	rgb := [3]uint32{}
	for c := range rgb {
		rgb[c] = ((j.Color[c] + j.DX[c]*uint32(x) + j.DY[c]*uint32(y)) >> 24) & 255
	}
	d := 0
	if j.Dither {
		a := [4][4]int{{-4, 0, -3, 1}, {2, -2, 3, -1}, {-3, 1, -4, 0}, {3, -1, 2, -2}}
		d = a[(y>>j.Shift)&3][(x>>j.Shift)&3]
	}
	q := func(v int) uint16 {
		v = (v + d) >> 3
		if v < 0 {
			return 0
		}
		if v > 31 {
			return 31
		}
		return uint16(v)
	}
	f := uint16(src[0]) | uint16(src[1])<<8
	if j.Textured {
		if j.Modulate {
			f = f&0x8000 | q(int((uint32(f&31)*rgb[0])>>4)) | q(int((uint32((f>>5)&31)*rgb[1])>>4))<<5 | q(int((uint32((f>>10)&31)*rgb[2])>>4))<<10
		}
	} else {
		f = 0x8000 | q(int(rgb[0])) | q(int(rgb[1]))<<5 | q(int(rgb[2]))<<10
	}
	if j.Mode >= 0 && f&0x8000 != 0 {
		f = blend(j.Mode, bg, f)
	}
	if !j.Textured {
		f &= 0x7fff
	}
	return f | j.MaskSet
}

func TestRGB555GPUConformance(t *testing.T) {
	d, err := hdgpu.New(1)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	r := image.Rect(37, 29, 293, 285)
	n := r.Dx() * r.Dy()
	bg := make([]uint16, n)
	input := make([]byte, n*4)
	got := make([]uint16, n)
	for i := range bg {
		bg[i] = uint16(i*4051 + 0x2949)
		input[i*4], input[i*4+1], input[i*4+3] = byte(i), byte(i>>8), 255
		if i%53 == 0 {
			input[i*4+3] = 0
		}
	}
	for mode := -1; mode < 4; mode++ {
		for mask := 0; mask < 4; mask++ {
			for flags := 0; flags < 8; flags++ {
				j := hd.RasterJob{Rect: r, Texels: input, Color: [3]uint32{128 << 24, 0x029451ff, 0xfc1234ff}, DX: [3]uint32{0x1f1007, 0xfedcba98, 0x80ff1234}, DY: [3]uint32{0xff110002, 0x01988475, 0xffff1331}, Mode: mode, Textured: flags&1 != 0, Modulate: flags&2 != 0, Dither: flags&4 != 0, Shift: 1, MaskSet: uint16(mask&1) << 15, MaskEval: uint16(mask>>1) << 15}
				d.Upload(r, bg)
				d.Draw(j)
				d.Read(r, got)
				for i, v := range got {
					x, y := r.Min.X+i%r.Dx(), r.Min.Y+i/r.Dx()
					want := reference(j, bg[i], input[i*4:i*4+4], x, y)
					if v != want {
						t.Fatalf("mode=%d mask=%d flags=%d (%d,%d): GPU=%04x CPU=%04x", mode, mask, flags, x, y, v, want)
					}
				}
			}
		}
	}
}

func TestGPUIdentityAndSubrect(t *testing.T) {
	for shift := uint(0); shift < 4; shift++ {
		d, err := hdgpu.New(shift)
		if err != nil {
			t.Fatal(err)
		}
		r := image.Rect((1024<<shift)-256, (512<<shift)-256, 1024<<shift, 512<<shift)
		n := 256 * 256
		bg := make([]uint16, n)
		in := make([]byte, n*4)
		out := make([]uint16, n)
		for i := range bg {
			bg[i] = uint16(i)
			in[i*4], in[i*4+1], in[i*4+3] = byte(i), byte(i>>8), 255
		}
		d.Upload(r, bg)
		j := hd.RasterJob{Rect: r, Texels: in, Color: [3]uint32{128 << 24, 128 << 24, 128 << 24}, Textured: true, Modulate: true, Mode: -1, Shift: shift}
		d.Draw(j)
		d.Read(r, out)
		for i, v := range out {
			if v != uint16(i) {
				t.Fatalf("shift%d word%04x ->%04x", shift, i, v)
			}
		}
		d.Close()
	}
}
