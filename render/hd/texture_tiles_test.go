package hd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTextureTileExactSamplingAndCollision(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		im := &textureImage{width: 35, height: 33, scale: 1}
		if indexed {
			im.indices = make([]byte, 35*33)
			for i := range im.indices {
				im.indices[i] = byte((i*13 + i/35) & 15)
			}
		} else {
			im.colors = make([]uint16, 35*33)
			for i := range im.colors {
				im.colors[i] = uint16(i * 733)
			}
			im.colors[0] = 0
			im.colors[1] = 0x8000
		}
		flat := *im
		b := newTextureTileBuilder(1 << 20)
		// Deliberately force hash collisions: only exact blocks may share storage.
		b.hash = func([]byte) [32]byte { return [32]byte{} }
		if e := b.add(im); e != nil {
			t.Fatal(e)
		}
		a, g := newRenderer(0), newRenderer(0)
		a.textures = &textureTracking{}
		g.textures = &textureTracking{}
		for i := range a.clutCache {
			a.clutCache[i] = uint16(i*0x421) | 0x8000
			g.clutCache[i] = a.clutCache[i]
		}
		for y := 0; y < 33; y++ {
			for x := 0; x < 35; x++ {
				got, want := g.texturePixel(im, x, y, 0, 0, 0), a.texturePixel(&flat, x, y, 0, 0, 0)
				if got != want {
					t.Fatalf("indexed%v at%d,%d: %04x != %04x", indexed, x, y, got, want)
				}
			}
		}
		if g.textures.stats != a.textures.stats {
			t.Fatal("sampling statistics changed")
		}
		if im.colors != nil || im.indices != nil || im.tiles == nil {
			t.Fatal("linear storage retained")
		}
		duplicate := flat
		beforeColors, beforeIndices := len(b.bank.colors), len(b.bank.indices)
		if e := b.add(&duplicate); e != nil {
			t.Fatal(e)
		}
		if len(b.bank.colors) != beforeColors || len(b.bank.indices) != beforeIndices {
			t.Fatal("exact duplicate allocated new payload")
		}
	}
}

func TestTextureTileResidentLimit(t *testing.T) {
	b := newTextureTileBuilder(textureTilePixels + 4)
	im := &textureImage{width: 32, height: 32, scale: 1, indices: make([]byte, textureTilePixels)}
	if e := b.add(im); e != nil {
		t.Fatal(e)
	}
	if b.used != b.limit {
		t.Fatal("payload or uint32 index omitted from budget")
	}
	next := &textureImage{width: 32, height: 32, scale: 1, indices: make([]byte, textureTilePixels)}
	if e := b.add(next); e == nil || next.tiles != nil {
		t.Fatal("reference budget exceeded without whole-image rejection")
	}
	var block [textureTilePixels]uint16
	tight := newTextureTileBuilder(textureTilePixels*2 - 1)
	if _, e := tight.colorID(&block); e == nil {
		t.Fatal("color payload budget ignored")
	}
}

func TestTexturePackCompactsOnlyAtResidentBoundary(t *testing.T) {
	dir := t.TempDir()
	const w, h = 2048, 1024
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i] = 255
		im.Pix[i+1] = 255
		im.Pix[i+2] = 255
		im.Pix[i+3] = 255
	}
	write := func(name string, im image.Image) {
		t.Helper()
		f, e := os.Create(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		if e = png.Encode(f, im); e != nil {
			t.Fatal(e)
		}
		if e = f.Close(); e != nil {
			t.Fatal(e)
		}
	}
	write("color.png", im)
	write("mask.png", image.NewGray(image.Rect(0, 0, w, h)))
	entries := []map[string]any{}
	for i := 0; i < 65; i++ {
		entries = append(entries, map[string]any{"upload_sha256": fmt.Sprintf("%064x", i+1), "width_words": 256, "height": 512, "depth": 4, "scale": 2, "kind": "color", "clut_sha256": fmt.Sprintf("%064x", 1), "image": "color.png", "stp": "mask.png"})
	}
	manifest := func(rows []map[string]any) {
		t.Helper()
		b, e := json.Marshal(map[string]any{"version": 1, "entries": rows})
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	manifest(entries)
	before, e := os.ReadFile(filepath.Join(dir, "color.png"))
	if e != nil {
		t.Fatal(e)
	}
	pack, e := LoadTexturePack(dir)
	if e != nil {
		t.Fatal(e)
	}
	if pack.Len() != 65 {
		t.Fatal("entry lost")
	}
	var bank *textureTileBank
	refs := 0
	for _, im := range pack.entries {
		if im.tiles == nil {
			t.Fatal("logical >256MiB pack remained linear")
		}
		if bank == nil {
			bank = im.tiles.bank
		}
		if im.tiles.bank != bank {
			t.Fatal("pack did not share one immutable bank")
		}
		refs += len(im.tiles.ids)
		for _, p := range [][2]int{{0, 0}, {31, 31}, {32, 32}, {w - 1, h - 1}} {
			v, indexed := im.tiles.pixel(p[0], p[1])
			if indexed || v != 0x7fff {
				t.Fatal("color/STP or tile boundary changed")
			}
		}
	}
	if len(bank.colors) != 1 || len(bank.indices) != 0 || uint64(refs*4+textureTilePixels*2) >= textureMemoryLimit {
		t.Fatal("physical sharing or budget invalid")
	}
	after, e := os.ReadFile(filepath.Join(dir, "color.png"))
	if e != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("input modified")
	}
	// Still validate a later PNG after entering the compact representation.
	badImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	badImage.Set(0, 0, color.White)
	write("bad.png", badImage)
	bad := map[string]any{"upload_sha256": fmt.Sprintf("%064x", 66), "width_words": 256, "height": 512, "depth": 4, "scale": 2, "kind": "color", "clut_sha256": fmt.Sprintf("%064x", 1), "image": "bad.png", "stp": "mask.png"}
	manifest(append(entries, bad))
	if partial, e := LoadTexturePack(dir); e == nil || partial != nil {
		t.Fatal("late invalid PNG returned a partial pack")
	}
}
