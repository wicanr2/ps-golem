package hdgpu_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wicanr2/ps-golem/render/hd"
	"github.com/wicanr2/ps-golem/render/hdgpu"
)

func syntheticPack(t *testing.T, kind string) *hd.TexturePack {
	t.Helper()
	root := t.TempDir()
	var b [12]byte
	binary.LittleEndian.PutUint32(b[:4], 1)
	binary.LittleEndian.PutUint32(b[4:8], 2)
	binary.LittleEndian.PutUint16(b[8:10], 0x1111)
	binary.LittleEndian.PutUint16(b[10:12], 0x1111)
	im := image.NewPaletted(image.Rect(0, 0, 8, 4), color.Palette{color.Transparent, color.NRGBA{248, 0, 0, 255}, color.NRGBA{0, 248, 0, 255}})
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			im.SetColorIndex(x, y, uint8(1+x%2))
		}
	}
	entry := map[string]any{"upload_sha256": fmt.Sprintf("%x", sha256.Sum256(b[:])), "width_words": 1, "height": 2, "depth": 4, "scale": 2, "kind": kind, "image": "tile.png"}
	var artwork image.Image = im
	if kind == "color" {
		rgba := image.NewNRGBA(im.Bounds())
		stp := image.NewGray(im.Bounds())
		for y := 0; y < 4; y++ {
			for x := 0; x < 8; x++ {
				colors := [4]color.NRGBA{{R: 255, G: 37, B: 211, A: 255}, {G: 199, B: 80, A: 255}, {}, {A: 255}}
				rgba.SetNRGBA(x, y, colors[x%4])
				if x%2 != 0 {
					stp.SetGray(x, y, color.Gray{Y: 255})
				}
			}
		}
		artwork = rgba
		var clut [32]byte
		binary.LittleEndian.PutUint16(clut[2:4], 0x801f)
		binary.LittleEndian.PutUint16(clut[4:6], 0x03e0)
		entry["clut_sha256"] = fmt.Sprintf("%x", sha256.Sum256(clut[:]))
		entry["stp"] = "stp.png"
		f, err := os.Create(filepath.Join(root, "stp.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, stp); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	f, e := os.Create(filepath.Join(root, "tile.png"))
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(f, artwork); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	manifest := map[string]any{"version": 1, "entries": []any{entry}}
	data, e := json.Marshal(manifest)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "manifest.json"), data, 0644); e != nil {
		t.Fatal(e)
	}
	p, e := hd.LoadTexturePack(root)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestGPUReplacementAndSTP(t *testing.T) {
	for _, kind := range []string{"indexed", "color"} {
		t.Run(kind, func(t *testing.T) {
			pack := syntheticPack(t, kind)
			for _, shift := range []uint{1, 2} {
				t.Run(fmt.Sprintf("N%d", 1<<shift), func(t *testing.T) {
					d, e := hdgpu.New(shift)
					if e != nil {
						t.Fatal(e)
					}
					a, b := hd.NewShadowWithTextures(shift, pack, hd.NewTextureDump()), hd.NewShadowWithDevice(shift, pack, hd.NewTextureDump(), d)
					defer a.Close()
					defer b.Close()
					for _, s := range []*hd.Shadow{a, b} {
						send(s, 0xe3000000, 0xe4000000|319|239<<10, 0xe1000011)
						send(s, 0xa0000000, xy(64, 256), xy(1, 2), 0x11111111)
						send(s, 0xa0000000, xy(0, 1), xy(16, 1), 0x801f0000, 0x000003e0, 0, 0, 0, 0, 0, 0)
						send(s, 0x30a0e019, xy(8, 18), 0x001221f1, xy(72, 20), 0x00fcb329, xy(10, 70))
						for i := uint32(0); i < 4; i++ {
							send(s, 0x66808080, xy(10+i*8, 20+i*4), 0x00400000, xy(4, 2))
							send(s, 0x2e808080, xy(10+i*8, 34+i*4), 0x00400000, xy(14+i*8, 34+i*4), 0x00110004, xy(10+i*8, 36+i*4), 0x00000200, xy(14+i*8, 36+i*4), 0x00000204)
						}
					}
					if av, bv := a.VRAMCopy(), b.VRAMCopy(); !slices.Equal(av, bv) {
						for i, v := range av {
							if v != bv[i] {
								t.Fatalf("replacement VRAM %d CPU=%04x GPU=%04x", i, v, bv[i])
							}
						}
					}
					as, bs := a.TextureStats(), b.TextureStats()
					if as != bs || bs.ReplacedSamples == 0 {
						t.Fatalf("replacement stats CPU=%+v GPU=%+v", as, bs)
					}
				})
			}
		})
	}
}
