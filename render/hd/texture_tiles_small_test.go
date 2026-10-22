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
	"strings"
	"testing"
)

func smallTilePNG(t *testing.T, dir, name string, im image.Image) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, im); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}
func smallTileManifest(t *testing.T, dir string, rows []map[string]any) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"version": 1, "entries": rows})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func smallTileFixture(t *testing.T) (string, []map[string]any, [][]uint16) {
	t.Helper()
	dir := t.TempDir()
	rows := []map[string]any{}
	words := [][]uint16{}
	for variant := 0; variant < 4; variant++ {
		im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		mask := image.NewGray(im.Rect)
		want := make([]uint16, 1024)
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				kind := (x/8 + 2*(y/8) + variant) % 4
				v := uint16(1 + kind*97 + (x%8)*17 + (y%8)*31)
				if (x%8+y%8+kind)%3 == 0 {
					v |= 0x8000
				}
				if x%8 == 0 && y%8 == 0 {
					v = 0
				}
				if x%8 == 7 && y%8 == 7 {
					v = 0x8000
				}
				want[y*32+x] = v
				a := uint8(255)
				if v == 0 {
					a = 0
				}
				im.SetNRGBA(x, y, color.NRGBA{uint8((v & 31) * 255 / 31), uint8((v >> 5 & 31) * 255 / 31), uint8((v >> 10 & 31) * 255 / 31), a})
				if v&0x8000 != 0 {
					mask.SetGray(x, y, color.Gray{255})
				}
			}
		}
		imageName, maskName := fmt.Sprintf("color%d.png", variant), fmt.Sprintf("stp%d.png", variant)
		smallTilePNG(t, dir, imageName, im)
		smallTilePNG(t, dir, maskName, mask)
		rows = append(rows, map[string]any{"upload_sha256": fmt.Sprintf("%064x", variant+1), "clut_sha256": fmt.Sprintf("%064x", 1), "width_words": 8, "height": 32, "depth": 4, "scale": 1, "kind": "color", "image": imageName, "stp": maskName})
		words = append(words, want)
	}
	smallTileManifest(t, dir, rows)
	return dir, rows, words
}
func checkSmallTileWords(t *testing.T, p *TexturePack, want [][]uint16, shift uint) {
	t.Helper()
	if p.Len() != len(want) {
		t.Fatal("entry lost")
	}
	clut, _ := textureDigest(fmt.Sprintf("%064x", 1))
	for i, words := range want {
		u, _ := textureDigest(fmt.Sprintf("%064x", i+1))
		im := p.lookup(u, 4, clut)
		if im == nil || im.tiles == nil || im.tiles.bank.shift != shift {
			t.Fatalf("entry%d representation", i)
		}
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				v, indexed := im.tiles.pixel(x, y)
				if indexed || v != words[y*32+x] {
					t.Fatalf("word %d,%d entry%d", x, y, i)
				}
			}
		}
	}
}

func TestTexturePackSmallTilesTransitions(t *testing.T) {
	dir, _, want := smallTileFixture(t)
	// First image; mixed linear/32 conversion; appended image after32 succeeded.
	for _, budget := range []uint64{1024, 4096, 5000} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			p, err := loadTexturePackWithMemoryLimit(dir, budget)
			if err != nil {
				t.Fatal(err)
			}
			checkSmallTileWords(t, p, want, 3)
		})
	}
	if p, err := loadTexturePackWithMemoryLimit(dir, 767); p != nil || !isTextureBudget(err) {
		t.Fatal("small payload/grid budget ignored", err)
	}
}

func TestTexturePackKeepsAccepted32Tiles(t *testing.T) {
	dir, rows, want := smallTileFixture(t)
	for _, r := range rows {
		r["image"] = rows[0]["image"]
		r["stp"] = rows[0]["stp"]
	}
	smallTileManifest(t, dir, rows)
	p, err := loadTexturePackWithMemoryLimit(dir, 5000)
	if err != nil {
		t.Fatal(err)
	}
	checkSmallTileWords(t, p, [][]uint16{want[0], want[0], want[0], want[0]}, 5)
}

func TestTexturePackSmallTilesLateContentRejection(t *testing.T) {
	for _, kind := range []string{"alpha", "STP", "索引"} {
		t.Run(kind, func(t *testing.T) {
			dir, rows, want := smallTileFixture(t)
			row := map[string]any{}
			for k, v := range rows[0] {
				row[k] = v
			}
			row["upload_sha256"] = fmt.Sprintf("%064x", 5)
			switch kind {
			case "alpha":
				im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
				for y := 0; y < 32; y++ {
					for x := 0; x < 32; x++ {
						im.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
					}
				}
				im.SetNRGBA(31, 31, color.NRGBA{255, 255, 255, 128})
				smallTilePNG(t, dir, "bad.png", im)
				row["image"] = "bad.png"
			case "STP":
				im := image.NewGray(image.Rect(0, 0, 32, 32))
				for y := 0; y < 32; y++ {
					for x := 0; x < 32; x++ {
						if want[0][y*32+x]&0x8000 != 0 {
							im.SetGray(x, y, color.Gray{255})
						}
					}
				}
				im.SetGray(31, 31, color.Gray{128})
				smallTilePNG(t, dir, "bad.png", im)
				row["stp"] = "bad.png"
			case "索引":
				palette := make(color.Palette, 17)
				for i := range palette {
					palette[i] = color.White
				}
				im := image.NewPaletted(image.Rect(0, 0, 32, 32), palette)
				im.SetColorIndex(31, 31, 16)
				smallTilePNG(t, dir, "bad.png", im)
				row["image"] = "bad.png"
				row["kind"] = "indexed"
				delete(row, "stp")
				delete(row, "clut_sha256")
			}
			smallTileManifest(t, dir, append(rows, row))
			p, err := loadTexturePackWithMemoryLimit(dir, 1024)
			if p != nil || err == nil || !strings.Contains(err.Error(), kind) {
				t.Fatal("late content guard bypassed", err)
			}
		})
	}
}

func TestTextureTileRetilesCollisionAndEdges(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		im := &textureImage{width: 35, height: 33, scale: 1}
		want := make([]uint16, 35*33)
		if indexed {
			im.indices = make([]byte, len(want))
		} else {
			im.colors = make([]uint16, len(want))
		}
		for i := range want {
			if indexed {
				im.indices[i] = byte((i*13 + i/35) & 15)
				want[i] = uint16(im.indices[i])
			} else {
				im.colors[i] = uint16(i * 733)
				want[i] = im.colors[i]
			}
		}
		coarse := newTextureTileBuilder(1 << 20)
		fine := newTextureTileBuilderWithShift(1<<20, 3)
		for _, b := range []*textureTileBuilder{coarse, fine} {
			b.hash = func([]byte) [32]byte { return [32]byte{} }
			if err := b.add(im); err != nil {
				t.Fatal(err)
			}
		}
		for y := 0; y < 33; y++ {
			for x := 0; x < 35; x++ {
				v, kind := im.tiles.pixel(x, y)
				if v != want[y*35+x] || kind != indexed {
					t.Fatal("retile changed word/type/edge")
				}
			}
		}
	}
}

func TestTexturePackRetilesForSpriteRunBudget(t *testing.T) {
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
	palette := color.Palette{color.Black, color.White, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}}
	im := image.NewPaletted(image.Rect(0, 0, 32, 32), palette)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			kind := (x/8 + 2*(y/8)) % 4
			im.SetColorIndex(x, y, byte((kind+x%8+3*(y%8))%4))
		}
	}
	smallTilePNG(t, dir, "index.png", im)
	smallTileManifest(t, dir, []map[string]any{{"upload_sha256": fmt.Sprintf("%064x", 1), "width_words": 8, "height": 32, "depth": 4, "scale": 1, "kind": "indexed", "image": "index.png"}})
	p, err := loadTexturePackWithMemoryLimit(dir, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range p.entries {
		if entry.tiles == nil || entry.tiles.bank.shift != 3 {
			t.Fatal("name budget did not retile")
		}
	}
	if !reflect.DeepEqual(p.spriteRuns, original.spriteRuns) {
		t.Fatal("name rules or pixels changed")
	}
	if p, err := loadTexturePackWithMemoryLimit(dir, 2367); p != nil || !isTextureBudget(err) {
		t.Fatal("combined name/grid/payload budget ignored", err)
	}
	badPalette := make(color.Palette, 17)
	for i := range badPalette {
		badPalette[i] = color.White
	}
	bad := image.NewPaletted(image.Rect(0, 0, 64, 32), badPalette)
	bad.SetColorIndex(63, 31, 16)
	smallTilePNG(t, dir, "run.png", bad)
	if p, err := loadTexturePackWithMemoryLimit(dir, 3000); p != nil || err == nil || !strings.Contains(err.Error(), "索引") {
		t.Fatal("name validation bypassed by retile", err)
	}
}
