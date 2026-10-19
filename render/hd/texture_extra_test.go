package hd

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestTextureColorUsesCachedCLUTAndSTPBlend(t *testing.T) {
	root := t.TempDir()
	words := []uint16{0x1111, 0x1111}
	palette := make([]uint16, 16)
	palette[1] = 31
	palette[2] = 0x3e0
	s := textureSpec{Upload: fmt.Sprintf("%x", textureUploadDigest(1, 2, words)), Width: 1, Height: 2, Depth: 4, Scale: 2, Kind: "color", CLUT: fmt.Sprintf("%x", textureCLUTDigest(palette)), Image: "color.png", STP: "stp.png"}
	im := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	mask := image.NewGray(im.Bounds())
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			im.SetNRGBA(x, y, color.NRGBA{0, 0, 248, 255})
		}
	}
	im.SetNRGBA(1, 0, color.NRGBA{})
	mask.SetGray(0, 0, color.Gray{255})
	packTestPNG(t, root, s.Image, im)
	packTestPNG(t, root, s.STP, mask)
	packTestManifest(t, root, s)
	p, err := LoadTexturePack(root)
	if err != nil {
		t.Fatal(err)
	}
	g := newRenderer(1)
	g.configureTextures(p, nil)
	textureTestScene(g)
	g.put(10, 20, 31)
	send(g, 0x67808080, 20<<16|10, 0x00400000, 1<<16|2)
	if c := g.VRAM[40*2048+20] & 0x7fff; c != 0x3c0f {
		t.Fatalf("PS1 STP average RGB5=%04x", c)
	}
	if c := g.VRAM[40*2048+21]; c != 31 {
		t.Fatalf("transparent replacement wrote background: %04x", c)
	}
	if c := g.VRAM[40*2048+22]; c != 0x7c00 {
		t.Fatalf("STP=0 should be opaque even with ABE: %04x", c)
	}
	send(g, 0xa0000000, 0x00010001, 0x00010001, 0x7c00)
	if c := g.getTexelHD(0, 0, 0, 1, 0, 1); c != 0 {
		t.Fatal("live VRAM palette changed cached CLUT key")
	}
	send(g, 0x01000000)
	g.updateCLUT(0x40, 0)
	if c := g.getTexelHD(0, 0, 0, 1, 0, 1); c != 0x7c00 {
		t.Fatalf("missing color CLUT did not fall back: %04x", c)
	}
}

func TestTextureCopy128OverlapAndCachedRawLifetime(t *testing.T) {
	d := NewTextureDump()
	g := newRenderer(1)
	g.configureTextures(nil, d)
	send(g, 0xa0000000, 0, 0x00010104)
	for i := 0; i < 260; i += 2 {
		g.WriteGP0(uint32(i+1) | uint32(i+2)<<16)
	}
	send(g, 0x80000000, 0, 1, 0x00010104)
	if g.textures.origins[128].offset != 127 || g.textures.origins[129].offset != 127 || g.textures.origins[257].offset != 255 {
		t.Fatal("source copy did not preserve 128-word overlap order")
	}
	h := newRenderer(1)
	d2 := NewTextureDump()
	h.configureTextures(nil, d2)
	textureTestScene(h)
	h.updateCLUT(0x40, 0)
	h.getTexel(0, 0, 0)
	u := h.textures.origins[256*1024+64].upload
	send(h, 0xe3000000, 0xe407ffff, 0x6000ff00, 0x01000040, 0x00020001)
	if len(u.words) == 0 {
		t.Fatal("stale texture cache did not retain raw source")
	}
	send(h, 0x01000000)
	if len(u.words) != 0 {
		t.Fatal("unused overwritten source retained after cache flush")
	}
}
