package hd

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTextureColorTileModesAndExactWords(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 16, 17, 31, 32, 64} {
		words := make([]uint16, 64)
		for i := range words {
			v := i % n
			if v == 1 {
				words[i] = 0x8000
			} else if v > 1 {
				words[i] = uint16(v * 997)
			}
		}
		tile := encodeTextureColorTile(words)
		want := uint8(16)
		switch {
		case n == 1:
			want = 0
		case n <= 2:
			want = 1
		case n <= 4:
			want = 2
		case n <= 16:
			want = 4
		case n < 32:
			want = 8
		}
		if tile.bits != want {
			t.Fatalf("n%d mode%d want%d", n, tile.bits, want)
		}
		if tile.residentBytes() != uint64(len(tile.words)*2+len(tile.codes)+1) {
			t.Fatal("mode cost omitted")
		}
		for i, w := range words {
			if tile.pixel(i) != w {
				t.Fatalf("n%d at%d", n, i)
			}
		}
		words[0] ^= 0xffff
		if tile.equal(words) {
			t.Fatal("distinct words compared equal")
		}
	}
}

func TestTexturePaletteTileCollisionAndEdges(t *testing.T) {
	b := newTextureTileBuilderWithShift(1<<20, 3)
	b.bank.paletted = true
	b.hash = func([]byte) [32]byte { return [32]byte{} }
	im := &textureImage{width: 11, height: 9, scale: 1, colors: make([]uint16, 99)}
	for i := range im.colors {
		if i%3 == 0 {
			im.colors[i] = 0x8000
		} else {
			im.colors[i] = uint16(i * 37)
		}
	}
	flat := append([]uint16(nil), im.colors...)
	if err := b.add(im); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 9; y++ {
		for x := 0; x < 11; x++ {
			v, indexed := im.tiles.pixel(x, y)
			if indexed || v != flat[y*11+x] {
				t.Fatal("edge or collision differs")
			}
		}
	}
	n := len(b.bank.packedColors)
	duplicate := &textureImage{width: 11, height: 9, scale: 1, colors: flat}
	if err := b.add(duplicate); err != nil {
		t.Fatal(err)
	}
	if len(b.bank.packedColors) != n {
		t.Fatal("equal tile not shared")
	}
	var payload uint64
	for i := range b.bank.packedColors {
		payload += b.bank.packedColors[i].residentBytes()
	}
	if b.used != payload+uint64(len(im.tiles.ids)+len(duplicate.tiles.ids))*4 {
		t.Fatal("resident accounting")
	}
}

func TestTexturePaletteTileBudget(t *testing.T) {
	b := newTextureTileBuilderWithShift(7, 3)
	b.bank.paletted = true
	im := &textureImage{width: 8, height: 8, scale: 1, colors: make([]uint16, 64)}
	if err := b.add(im); err != nil || b.used != 7 {
		t.Fatalf("constant tile mode+word+reference: %v %d", err, b.used)
	}
	next := &textureImage{width: 8, height: 8, scale: 1, colors: make([]uint16, 64)}
	if err := b.add(next); !isTextureBudget(err) || next.tiles != nil {
		t.Fatal("reference boundary not enforced")
	}
}

func TestTexturePackPaletteFallbackAndLatePNG(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]any{}
	write := func(name string, im image.Image) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, im); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	originals := [][]uint16{}
	for i := 0; i < 4; i++ {
		im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		mask := image.NewGray(im.Bounds())
		words := make([]uint16, 1024)
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				tile := i*16 + (y/8)*4 + x/8
				at := (y%8)*8 + x%8
				white := (tile >> uint(at%6) & 1) != 0
				if white {
					im.SetNRGBA(x, y, color.NRGBA{A: 255})
					mask.SetGray(x, y, color.Gray{Y: 255})
					words[y*32+x] = 0x8000
				}
			}
		}
		imageName, maskName := fmt.Sprintf("image%d.png", i), fmt.Sprintf("mask%d.png", i)
		write(imageName, im)
		write(maskName, mask)
		originals = append(originals, words)
		entries = append(entries, map[string]any{"upload_sha256": fmt.Sprintf("%064x", i+1), "width_words": 32, "height": 32, "depth": 15, "scale": 1, "kind": "color", "image": imageName, "stp": maskName})
	}
	manifest := func() {
		data, err := json.Marshal(map[string]any{"version": 1, "entries": entries})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest()
	pack, err := loadTexturePackWithMemoryLimit(dir, 1400)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Len() != 4 {
		t.Fatal("partial pack")
	}
	for i, e := range entries {
		key, _, _, err := textureSpec{Upload: e["upload_sha256"].(string), Width: 32, Height: 32, Depth: 15, Scale: 1, Kind: "color", STP: e["stp"].(string)}.validate()
		if err != nil {
			t.Fatal(err)
		}
		im := pack.entries[key]
		if im.tiles == nil || !im.tiles.bank.paletted {
			t.Fatal("palette fallback not used")
		}
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				v, _ := im.tiles.pixel(x, y)
				if v != originals[i][y*32+x] {
					t.Fatal("PNG words differ")
				}
			}
		}
	}
	// A late content error must reject the entire pack even after compaction.
	bad := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	bad.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	write("bad.png", bad)
	entries = append(entries, map[string]any{"upload_sha256": fmt.Sprintf("%064x", 5), "width_words": 32, "height": 32, "depth": 15, "scale": 1, "kind": "color", "image": "bad.png", "stp": "mask0.png"})
	manifest()
	if partial, err := loadTexturePackWithMemoryLimit(dir, 1400); err == nil || partial != nil {
		t.Fatal("late invalid alpha accepted")
	}
}

func TestTexturePaletteFallbackAfterLinearPrefix(t *testing.T) {
	dir := t.TempDir()
	rows := []map[string]any{}
	for i := 0; i < 3; i++ {
		im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		mask := image.NewGray(im.Bounds())
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if (y*8+x)%(i+2) == 0 {
					im.SetNRGBA(x, y, color.NRGBA{A: 255})
					mask.SetGray(x, y, color.Gray{Y: 255})
				}
			}
		}
		name, stp := fmt.Sprintf("i%d.png", i), fmt.Sprintf("s%d.png", i)
		smallTilePNG(t, dir, name, im)
		smallTilePNG(t, dir, stp, mask)
		rows = append(rows, map[string]any{"upload_sha256": fmt.Sprintf("%064x", i+1), "width_words": 8, "height": 8, "depth": 15, "scale": 1, "kind": "color", "image": name, "stp": stp})
	}
	smallTileManifest(t, dir, rows)
	p, err := loadTexturePackWithMemoryLimit(dir, 200)
	if err != nil {
		t.Fatal(err)
	}
	if p.Len() != 3 {
		t.Fatal("lost prefix")
	}
	for _, im := range p.entries {
		if im.tiles == nil || !im.tiles.bank.paletted {
			t.Fatal("8 tile resource fallback missing")
		}
	}
}

func TestTexturePaletteFallbackForSpriteRuns(t *testing.T) {
	original, fixture, _, _ := runFixture(t)
	dir := t.TempDir()
	for _, name := range []string{"sprite-runs.json", "run.png"} {
		b, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	mask := image.NewGray(im.Bounds())
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			im.SetNRGBA(x, y, color.NRGBA{A: 255})
			mask.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	smallTilePNG(t, dir, "color.png", im)
	smallTilePNG(t, dir, "stp.png", mask)
	smallTileManifest(t, dir, []map[string]any{{"upload_sha256": fmt.Sprintf("%064x", 1), "width_words": 32, "height": 32, "depth": 15, "scale": 1, "kind": "color", "image": "color.png", "stp": "stp.png"}})
	p, err := loadTexturePackWithMemoryLimit(dir, 2200)
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range p.entries {
		if im.tiles == nil || !im.tiles.bank.paletted {
			t.Fatal("name budget did not choose palette fallback")
		}
	}
	if !reflect.DeepEqual(original.spriteRuns, p.spriteRuns) {
		t.Fatal("name rules or words changed")
	}
	if p, err := loadTexturePackWithMemoryLimit(dir, 2114); p != nil || !isTextureBudget(err) {
		t.Fatal("combined name budget bypassed", err)
	}
}
