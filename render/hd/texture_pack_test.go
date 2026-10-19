package hd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packTestSpec() textureSpec {
	return textureSpec{Upload: strings.Repeat("a", 64), Width: 1, Height: 1, Depth: 4, Scale: 1, Kind: "indexed", Image: "index.png"}
}

func packTestPNG(t *testing.T, root, name string, img image.Image) {
	t.Helper()
	f, err := os.Create(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func packTestIndex(t *testing.T, root string, depth int, indices []byte) {
	t.Helper()
	pal := make(color.Palette, 256)
	for i := range pal {
		pal[i] = color.NRGBA{R: 255, A: 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, len(indices), 1), pal)
	copy(img.Pix, indices)
	packTestPNG(t, root, "index.png", img)
}

func packTestManifest(t *testing.T, root string, specs ...textureSpec) {
	t.Helper()
	b, err := json.Marshal(textureManifest{Version: 1, Entries: specs})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "manifest.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestTexturePackIndexedValuesAndCLUTFallback(t *testing.T) {
	for _, depth := range []int{4, 8} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			root := t.TempDir()
			s := packTestSpec()
			s.Depth = depth
			indices := []byte{0, 1, 2, 15}
			if depth == 8 {
				indices = []byte{1, 255}
			}
			packTestIndex(t, root, depth, indices)
			packTestManifest(t, root, s)
			before, _ := os.ReadFile(filepath.Join(root, "index.png"))
			p, err := LoadTexturePack(root)
			if err != nil {
				t.Fatal(err)
			}
			key, _ := textureDigest(s.Upload)
			e := p.lookup(key, depth, [32]byte{1})
			if p.Len() != 1 || e == nil || !bytes.Equal(e.indices, indices) {
				t.Fatal("lost index identity or fallback")
			}
			if p.lookup([32]byte{}, depth, [32]byte{}) != nil {
				t.Fatal("missing upload should not match")
			}
			after, _ := os.ReadFile(filepath.Join(root, "index.png"))
			if !bytes.Equal(before, after) {
				t.Fatal("loader modified source")
			}
		})
	}
}

func TestTexturePackColorSTPAndPriority(t *testing.T) {
	root := t.TempDir()
	indexed := packTestSpec()
	packTestIndex(t, root, 4, []byte{0, 1, 2, 3})
	colored := indexed
	colored.Kind = "color"
	colored.Image = "color.png"
	colored.STP = "stp.png"
	colored.CLUT = strings.Repeat("b", 64)
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{R: 255, A: 0})
	img.SetNRGBA(3, 0, color.NRGBA{A: 255})
	stp := image.NewGray(img.Bounds())
	stp.Pix[1] = 255
	stp.Pix[3] = 255
	packTestPNG(t, root, colored.Image, img)
	packTestPNG(t, root, colored.STP, stp)
	packTestManifest(t, root, indexed, colored)
	p, err := LoadTexturePack(root)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := textureDigest(indexed.Upload)
	clut, _ := textureDigest(colored.CLUT)
	e := p.lookup(key, 4, clut)
	want := []uint16{31, 0x83e0, 0, 0x8000}
	if e == nil || len(e.colors) != 4 {
		t.Fatal("missing color image")
	}
	for i, v := range want {
		if e.colors[i] != v {
			t.Fatalf("color[%d]=%04x want %04x", i, e.colors[i], v)
		}
	}
	if e := p.lookup(key, 4, [32]byte{}); e == nil || len(e.indices) == 0 {
		t.Fatal("missing CLUT should fall back to indexed")
	}
}

func TestTexturePack15BitAndScaledDimensions(t *testing.T) {
	root := t.TempDir()
	s := packTestSpec()
	s.Kind = "color"
	s.Depth = 15
	s.Scale = 2
	s.Image = "color.png"
	s.STP = "stp.png"
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:], []byte{248, 0, 0, 255})
	}
	packTestPNG(t, root, s.Image, img)
	packTestPNG(t, root, s.STP, image.NewGray(img.Bounds()))
	packTestManifest(t, root, s)
	p, err := LoadTexturePack(root)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := textureDigest(s.Upload)
	e := p.lookup(key, 15, [32]byte{})
	if e == nil || e.scale != 2 || e.width != 2 || e.height != 2 || e.colors[3] != 31 {
		t.Fatal("15bpp scale")
	}
}

func TestTexturePackEmpty(t *testing.T) {
	root := t.TempDir()
	packTestManifest(t, root, []textureSpec{}...)
	p, err := LoadTexturePack(root)
	if err != nil || p == nil || p.Len() != 0 {
		t.Fatalf("empty: %v", err)
	}
	var nilPack *TexturePack
	if nilPack.Len() != 0 || nilPack.lookup([32]byte{}, 4, [32]byte{}) != nil {
		t.Fatal("nil pack")
	}
}

func TestTexturePackInvalidManifest(t *testing.T) {
	valid := `{"version":1,"entries":[]}`
	tests := map[string]string{
		"unknown":          `{"version":1,"entries":[],"typo":1}`,
		"version":          `{"version":2,"entries":[]}`,
		"missing entries":  `{"version":1}`,
		"null entries":     `{"version":1,"entries":null}`,
		"trailing":         valid + valid,
		"duplicate":        `{"version":1,"version":1,"entries":[]}`,
		"nested duplicate": `{"version":1,"entries":[{"depth":4,"depth":8}]}`,
		"bad type":         `{"version":"1","entries":[]}`,
		"invalid UTF8":     valid[:len(valid)-1] + ",\"x\":\"\xff\"}",
		"too large":        strings.Repeat(" ", textureManifestLimit+1),
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "manifest.json"), []byte(b), 0644)
			p, err := LoadTexturePack(root)
			if err == nil || p != nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestTexturePackInvalidEntries(t *testing.T) {
	tests := map[string]func(*textureSpec){
		"hash":               func(s *textureSpec) { s.Upload = "bad" },
		"uppercase hash":     func(s *textureSpec) { s.Upload = strings.Repeat("A", 64) },
		"width":              func(s *textureSpec) { s.Width = 1025 },
		"height":             func(s *textureSpec) { s.Height = 0 },
		"depth":              func(s *textureSpec) { s.Depth = 16 },
		"scale":              func(s *textureSpec) { s.Scale = 3 },
		"kind":               func(s *textureSpec) { s.Kind = "rgb" },
		"indexed15":          func(s *textureSpec) { s.Depth = 15 },
		"indexed CLUT":       func(s *textureSpec) { s.CLUT = strings.Repeat("b", 64) },
		"indexed STP":        func(s *textureSpec) { s.STP = "mask.png" },
		"color missing STP":  func(s *textureSpec) { s.Kind = "color"; s.CLUT = strings.Repeat("b", 64) },
		"color missing CLUT": func(s *textureSpec) { s.Kind = "color"; s.STP = "stp.png" },
		"15 CLUT": func(s *textureSpec) {
			s.Kind = "color"
			s.Depth = 15
			s.STP = "stp.png"
			s.CLUT = strings.Repeat("b", 64)
		},
		"pixel budget": func(s *textureSpec) { s.Width = 1024; s.Height = 512; s.Scale = 8 },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			s := packTestSpec()
			change(&s)
			packTestManifest(t, root, s)
			p, err := LoadTexturePack(root)
			if err == nil || p != nil {
				t.Fatal("accepted invalid entry")
			}
		})
	}
	root := t.TempDir()
	s := packTestSpec()
	packTestIndex(t, root, 4, []byte{0, 1, 2, 3})
	packTestManifest(t, root, s, s)
	if p, err := LoadTexturePack(root); err == nil || p != nil {
		t.Fatal("accepted duplicate entry or partial pack")
	}
	s2 := s
	s2.Depth = 8
	s2.Width = 2
	packTestManifest(t, root, s, s2)
	if p, err := LoadTexturePack(root); err == nil || p != nil {
		t.Fatal("accepted same hash with different dimensions")
	}
}

func TestTexturePackPNGFailures(t *testing.T) {
	for _, name := range []string{"rgb index", "index overflow", "dimensions", "broken PNG", "huge header"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			s := packTestSpec()
			packTestManifest(t, root, s)
			switch name {
			case "rgb index":
				packTestPNG(t, root, s.Image, image.NewNRGBA(image.Rect(0, 0, 4, 1)))
			case "index overflow":
				packTestIndex(t, root, 4, []byte{0, 1, 16, 3})
			case "dimensions":
				packTestIndex(t, root, 4, []byte{0, 1})
			case "broken PNG":
				os.WriteFile(filepath.Join(root, s.Image), []byte("broken"), 0644)
			case "huge header":
				// 一張只有巨大 IHDR 的 PNG；必須先拒絕尺寸，不能進入分配／解碼。
				b := []byte("\x89PNG\r\n\x1a\n")
				chunk := make([]byte, 25)
				binary.BigEndian.PutUint32(chunk[:4], 13)
				copy(chunk[4:8], "IHDR")
				binary.BigEndian.PutUint32(chunk[8:12], 1<<25)
				binary.BigEndian.PutUint32(chunk[12:16], 1<<25)
				chunk[16] = 8
				chunk[17] = 6
				binary.BigEndian.PutUint32(chunk[21:], crc32.ChecksumIEEE(chunk[4:21]))
				b = append(b, chunk...)
				os.WriteFile(filepath.Join(root, s.Image), b, 0644)
			}
			if p, err := LoadTexturePack(root); err == nil || p != nil {
				t.Fatal("accepted invalid PNG")
			}
		})
	}
	for _, name := range []string{"alpha", "mask gray", "mask color", "opaque black"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			s := packTestSpec()
			s.Kind = "color"
			s.CLUT = strings.Repeat("b", 64)
			s.Image = "color.png"
			s.STP = "stp.png"
			img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
			for x := 0; x < 4; x++ {
				img.SetNRGBA(x, 0, color.NRGBA{R: 255, A: 255})
			}
			mask := image.NewNRGBA(img.Bounds())
			for x := 0; x < 4; x++ {
				mask.SetNRGBA(x, 0, color.NRGBA{A: 255})
			}
			switch name {
			case "alpha":
				img.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 128})
			case "mask gray":
				mask.SetNRGBA(1, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
			case "mask color":
				mask.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 255})
			case "opaque black":
				img.SetNRGBA(1, 0, color.NRGBA{R: 7, G: 7, B: 7, A: 255})
			}
			packTestPNG(t, root, s.Image, img)
			packTestPNG(t, root, s.STP, mask)
			packTestManifest(t, root, s)
			if p, err := LoadTexturePack(root); err == nil || p != nil {
				t.Fatal("accepted invalid color/mask")
			}
		})
	}
}

func TestTexturePackPathContainment(t *testing.T) {
	for _, name := range []string{"../outside.png", "/tmp/outside.png", "x/../index.png", "x\\index.png", "C:/index.png", ".", ""} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			s := packTestSpec()
			s.Image = name
			packTestManifest(t, root, s)
			if p, err := LoadTexturePack(root); err == nil || p != nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
	root := t.TempDir()
	outside := t.TempDir()
	packTestIndex(t, outside, 4, []byte{0, 1, 2, 3})
	if err := os.Symlink(filepath.Join(outside, "index.png"), filepath.Join(root, "index.png")); err != nil {
		t.Skip(err)
	}
	packTestManifest(t, root, packTestSpec())
	if p, err := LoadTexturePack(root); err == nil || p != nil {
		t.Fatal("accepted outside symlink")
	}
}

func TestTextureUploadAndCLUTDigest(t *testing.T) {
	w := []uint16{0x1234, 0xabcd, 0}
	a := textureUploadDigest(3, 1, w)
	b := textureUploadDigest(1, 3, w)
	if a == b {
		t.Fatal("dimensions missing from key")
	}
	if a != textureUploadDigest(3, 1, append([]uint16(nil), w...)) {
		t.Fatal("unstable key")
	}
	if textureCLUTDigest(w) == textureCLUTDigest(w[:2]) {
		t.Fatal("CLUT length missing")
	}
}

func TestTexturePackForbiddenEmptyFieldsAndAliases(t *testing.T) {
	for _, field := range []string{"stp", "clut_sha256", "Depth", "STP"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			packTestIndex(t, root, 4, []byte{0, 1, 2, 3})
			b, err := json.Marshal(packTestSpec())
			if err != nil {
				t.Fatal(err)
			}
			var entry map[string]any
			if err := json.Unmarshal(b, &entry); err != nil {
				t.Fatal(err)
			}
			if field == "Depth" {
				delete(entry, "depth")
				entry[field] = 4
			} else {
				entry[field] = ""
			}
			b, err = json.Marshal(map[string]any{"version": 1, "entries": []any{entry}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "manifest.json"), b, 0644); err != nil {
				t.Fatal(err)
			}
			if pack, err := LoadTexturePack(root); err == nil || pack != nil {
				t.Fatalf("accepted forbidden field %s", field)
			}
		})
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"Version":1,"entries":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if pack, err := LoadTexturePack(root); err == nil || pack != nil {
		t.Fatal("accepted root field alias")
	}
}
