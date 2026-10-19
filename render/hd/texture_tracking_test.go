package hd

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func textureTestPack(t *testing.T, words []uint16, depth, scale int) *TexturePack {
	t.Helper()
	root := t.TempDir()
	per := 4
	if depth == 8 {
		per = 2
	}
	if depth == 15 {
		per = 1
	}
	s := textureSpec{Upload: fmt.Sprintf("%x", textureUploadDigest(1, len(words), words)), Width: 1, Height: len(words), Depth: depth, Scale: scale, Kind: "indexed", Image: "image.png"}
	pal := color.Palette{color.Transparent, color.NRGBA{248, 0, 0, 255}, color.NRGBA{0, 248, 0, 255}}
	im := image.NewPaletted(image.Rect(0, 0, per*scale, len(words)*scale), pal)
	for y := 0; y < im.Bounds().Dy(); y++ {
		for x := 0; x < im.Bounds().Dx(); x++ {
			im.SetColorIndex(x, y, uint8(1+x%2))
		}
	}
	packTestPNG(t, root, s.Image, im)
	packTestManifest(t, root, s)
	p, err := LoadTexturePack(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func textureTestScene(g *Renderer) {
	send(g, 0xe3000000, 0xe407ffff, 0xe1000011)
	send(g, 0xa0000000, 0x01000040, 0x00020001, 0x11111111)
	send(g, 0xa0000000, 0x00010000, 0x00010010, 0x001f0000, 0x000003e0, 0, 0, 0, 0, 0, 0)
}

func TestTextureReplacementSpritePolyFlipAndWindow(t *testing.T) {
	for _, depth := range []int{4, 8} {
		for shift := uint(0); shift <= 3; shift++ {
			t.Run(fmt.Sprintf("depth%d/N%d", depth, 1<<shift), func(t *testing.T) {
				words := []uint16{0x1111, 0x1111}
				if depth == 8 {
					words = []uint16{0x0101, 0x0101}
				}
				p := textureTestPack(t, words, depth, 2)
				g := newRenderer(shift)
				g.configureTextures(p, nil)
				textureTestScene(g)
				if depth == 8 {
					send(g, 0xe1000091, 0xa0000000, 0x01000040, 0x00020001, 0x01010101)
					g.updateCLUT(0x40, 1)
				}
				send(g, 0x65808080, 20<<16|10, 0x00400000, 2<<16|2)
				if got := g.texel(10, 20); got != 31 {
					t.Fatalf("first sprite sample %04x", got)
				}
				n := uint32(1) << shift
				if n > 1 && g.VRAM[(20*n)*(1024*n)+10*n+n-1] != 0x3e0 {
					t.Fatal("sprite did not sample HD pattern")
				}
				if g.textures.stats.Hits == 0 || g.textures.stats.ReplacedSamples == 0 {
					t.Fatal("replacement bypassed renderer")
				}
				// E2 視窗將 U=8 映回 U=0，仍須帶入子貼素。
				send(g, 0xe2000001)
				if c := g.getTexelHD(g.texMode, 8, 0, 1, 0, 1); c != 0x3e0 {
					t.Fatalf("E2 subtexel %04x", c)
				}
				send(g, 0xe2000000, 0xe1001011)
				if depth == 4 && n > 1 {
					send(g, 0x65808080, 24<<16|10, 0x00400000, 1<<16|1)
					if got := g.texel(10, 24); got != 0x3e0 {
						t.Fatalf("flip must reverse subtexel order: %04x", got)
					}
				}
				if depth == 4 {
					before := g.textures.stats.ReplacedSamples
					send(g, 0x2d808080, 30<<16|10, 0x00400000, 30<<16|14, 0x00110004, 32<<16|10, 0x00000200, 32<<16|14, 0x00000204)
					if g.textures.stats.ReplacedSamples <= before {
						t.Fatal("polygon did not use replacement")
					}
				}
			})
		}
	}
}

func TestTextureReplacementCacheAndSourceInvalidation(t *testing.T) {
	g := newRenderer(1)
	g.configureTextures(textureTestPack(t, []uint16{0x1111, 0x1111}, 4, 2), nil)
	textureTestScene(g)
	g.updateCLUT(0x40, 0)
	if c := g.getTexelHD(0, 0, 0, 1, 0, 1); c != 0x3e0 {
		t.Fatal("initial replacement")
	}
	// 畫圖解除目前 VRAM 來源，舊貼圖快取仍持有同次來源。
	send(g, 0x6800ff00, 0x01000040)
	if g.textures.origins[256*1024+64].upload != nil {
		t.Fatal("draw did not clear source")
	}
	if c := g.getTexelHD(0, 0, 0, 1, 0, 1); c != 0x3e0 {
		t.Fatal("stale texture cache lost its old source")
	}
	send(g, 0x01000000)
	g.updateCLUT(0x40, 0)
	if c := g.getTexelHD(0, 0, 0, 1, 0, 1); c != 0 {
		t.Fatalf("flush should expose raw changed data: %04x", c)
	}
}

func TestTextureUploadOddPaddingAbortMaskAndCopy(t *testing.T) {
	d := NewTextureDump()
	g := newRenderer(1)
	g.configureTextures(nil, d)
	send(g, 0xa0000000, 0x01ff03ff, 0x00010001, 0xaaaa1111)
	o := g.textures.origins[511*1024+1023]
	if o.upload == nil || o.upload.key != textureUploadDigest(1, 1, []uint16{0x1111}) {
		t.Fatal("odd padding in key")
	}
	send(g, 0xa0000000, 0, 0x00010004, 0x22221111)
	g.WriteGP1(0x01000000)
	if g.textures.origins[0].upload != nil || g.textures.words != nil {
		t.Fatal("aborted upload registered")
	}
	send(g, 0xa0000000, 0, 0x00010002, 0x11118001)
	old := g.textures.origins[0]
	send(g, 0xe6000002, 0xa0000000, 0, 0x00010002, 0x33332222)
	if g.textures.origins[0] != old || g.textures.origins[1].word != 0x3333 {
		t.Fatal("mask acceptance source")
	}
	send(g, 0xe6000000, 0x80000000, 1, 8, 0x00010001)
	if g.textures.origins[8] != g.textures.origins[1] {
		t.Fatal("copy did not preserve source")
	}
	// 目的格部分被 mask 擋住，不得保留整格來源。
	g.VRAM[20] = 0x8000
	send(g, 0xe6000002, 0x80000000, 1, 10, 0x00010001)
	if g.textures.origins[10].upload != nil {
		t.Fatal("partial copy mask kept whole-cell source")
	}
	// 來源內容被 maskSet 改變，不能假裝仍是未改變的原始貼圖。
	send(g, 0xe6000001, 0xa0000000, 20, 0x00010001, 0x1111)
	if g.textures.origins[20].upload != nil {
		t.Fatal("maskSet changed source but kept identity")
	}
}

func TestTextureUnmatchedAndEmptyKeepExactShadow(t *testing.T) {
	for _, pack := range []*TexturePack{{entries: map[textureKey]*textureImage{}}, textureTestPack(t, []uint16{0x2222}, 4, 2)} {
		a, b := newRenderer(1), newRenderer(1)
		b.configureTextures(pack, nil)
		for _, g := range []*Renderer{a, b} {
			textureTestScene(g)
			g.VRAM[20*2*2048+10*2+1] = 0x8000
			send(g, 0xe6000002, 0x67808080, 20<<16|10, 0x00400000, 2<<16|4)
			send(g, 0x2f808080, 30<<16|10, 0x00400000, 30<<16|14, 0x00110004, 32<<16|10, 0x00000200, 32<<16|14, 0x00000204)
		}
		if !slices.Equal(a.VRAM, b.VRAM) {
			t.Fatal("empty/unmatched pack changed shadow output")
		}
	}
}

func TestTextureDumpRoundTripCLUTAndMemoryRelease(t *testing.T) {
	d := NewTextureDump()
	g := newRenderer(1)
	g.configureTextures(nil, d)
	textureTestScene(g)
	g.textures.field = 12
	g.updateCLUT(0x40, 0)
	g.getTexelHD(0, 0, 0, 0, 0, 1)
	// 同一上傳、不同有效 CLUT 各留預覽，但 manifest 只有一個 indexed 鍵。
	send(g, 0xa0000000, 0x00010001, 0x00010001, 0x7c00, 0x01000000)
	g.textures.field = 15
	g.updateCLUT(0x40, 0)
	g.getTexelHD(0, 0, 0, 0, 0, 1)
	root := filepath.Join(t.TempDir(), "dump")
	if err := d.WriteDirectory(root); err != nil {
		t.Fatal(err)
	}
	p, err := LoadTexturePack(root)
	if err != nil || p.Len() != 1 {
		t.Fatalf("roundtrip %v", err)
	}
	var meta struct {
		Uses []dumpUseJSON `json:"uses"`
	}
	b, err := os.ReadFile(filepath.Join(root, "dump.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Uses) != 2 || meta.Uses[0].First == 0 || len(meta.Uses[0].Words) != 16 {
		t.Fatal("CLUT/field metadata")
	}
	if err = d.WriteDirectory(root); err == nil {
		t.Fatal("overwrote dump directory")
	}
	if _, err = os.Stat(filepath.Join(root, "INCOMPLETE")); !os.IsNotExist(err) {
		t.Fatal("completed dump marker")
	}
	// 未使用上傳只由 VRAM／快取持有，覆寫及刷新後回收原始字。
	d2 := NewTextureDump()
	h := newRenderer(0)
	h.configureTextures(nil, d2)
	send(h, 0xa0000000, 0, 0x00010001, 0x1111)
	if d2.rawBytes != 2 {
		t.Fatal("raw bytes not accounted")
	}
	send(h, 0x680000ff, 0)
	if d2.rawBytes != 0 {
		t.Fatalf("unused upload retained: %d", d2.rawBytes)
	}
	d2.rawBytes = textureMemoryLimit
	u := &textureUpload{words: []uint16{1}}
	d2.holdRaw(u)
	if d2.err == nil || !strings.Contains(d2.err.Error(), "超過") {
		t.Fatal("raw budget ignored")
	}
}

func TestTextureDump15BitAndShadowField(t *testing.T) {
	d := NewTextureDump()
	s := NewShadowWithTextures(0, nil, d)
	s.TextureField(1<<33 | 19)
	for _, v := range []uint32{0xe3000000, 0xe407ffff, 0xe1000110, 0xa0000000, 0x01000000, 0x00010002, 0x801f8000, 0x65808080, 20<<16 | 10, 0, 1<<16 | 2} {
		s.GP0(v)
	}
	stats := s.TextureStats()
	s.Close()
	if stats.Uploads != 1 || len(d.used) != 1 {
		t.Fatal("shadow consumer bypassed dump")
	}
	root := filepath.Join(t.TempDir(), "dump")
	if err := d.WriteDirectory(root); err != nil {
		t.Fatal(err)
	}
	p, err := LoadTexturePack(root)
	if err != nil || p.Len() != 1 {
		t.Fatalf("15bpp roundtrip %v", err)
	}
	for _, u := range d.used {
		if u.first != 1<<33|19 {
			t.Fatal("64-bit field annotation truncated")
		}
	}
}

func TestTextureMemoMatchesDirectLookup(t *testing.T) {
	uploads := make([]*textureUpload, 16)
	for i := range uploads {
		uploads[i] = &textureUpload{w: 1, h: 1}
		uploads[i].key[0] = byte(i%8 + 1)
	}
	var a, b [32]byte
	a[0] = 11
	b[0] = 22
	p := &TexturePack{entries: map[textureKey]*textureImage{}}
	for i := 0; i < 8; i++ {
		u := uploads[i]
		if i%3 == 0 {
			for _, depth := range []int{4, 8} {
				p.entries[textureKey{upload: u.key, depth: depth}] = &textureImage{scale: 2}
			}
		}
		if i%4 == 0 {
			p.entries[textureKey{upload: u.key, depth: 4, clut: a, color: true}] = &textureImage{scale: 4}
		}
		if i%2 == 0 {
			p.entries[textureKey{upload: u.key, depth: 15, color: true}] = &textureImage{scale: 8}
		}
	}
	for _, pack := range []*TexturePack{nil, {entries: map[textureKey]*textureImage{}}, p} {
		tracker := &textureTracking{pack: pack}
		rng := rand.New(rand.NewSource(18018))
		for i := 0; i < 10000; i++ {
			u := uploads[rng.Intn(len(uploads))]
			depth := []int{4, 8, 15}[rng.Intn(3)]
			clut := [][32]byte{{}, a, b}[rng.Intn(3)]
			want := pack.lookup(u.key, depth, clut)
			for repeat := 0; repeat < 4; repeat++ {
				if got := tracker.resolveTexture(u, depth, clut); got != want {
					t.Fatalf("i=%d repeat=%d depth=%d", i, repeat, depth)
				}
			}
		}
	}
}
func TestTextureMemoPreservesDumpAndSampleCounts(t *testing.T) {
	dump := NewTextureDump()
	g := newRenderer(1)
	g.configureTextures(textureTestPack(t, []uint16{0x1111, 0x1111}, 4, 2), dump)
	textureTestScene(g)
	g.updateCLUT(0x40, 0)
	before := g.textures.stats
	for i := 0; i < 16; i++ {
		g.getTexelHD(0, 0, 0, 1, 0, 1)
	}
	after := g.textures.stats
	if after.Lookups-before.Lookups != 16 || after.Hits-before.Hits != 16 || after.ReplacedSamples-before.ReplacedSamples != 16 {
		t.Fatal("sampling stats changed")
	}
	var uses uint64
	for _, e := range dump.used {
		uses += e.count
	}
	if uses != 16 {
		t.Fatalf("dump missed memoized samples: %d", uses)
	}
}

var textureMemoBenchmarkResult *textureImage

func BenchmarkTextureResolverMemo(b *testing.B) {
	for _, kind := range []string{"indexed", "color", "miss", "alternating"} {
		for _, memo := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/memo=%v", kind, memo), func(b *testing.B) {
				u := &textureUpload{w: 1, h: 1}
				u.key[0] = 1
				other := &textureUpload{w: 1, h: 1}
				other.key[0] = 2
				var clut [32]byte
				clut[0] = 33
				p := &TexturePack{entries: map[textureKey]*textureImage{}}
				if kind == "indexed" {
					p.entries[textureKey{upload: u.key, depth: 4}] = &textureImage{scale: 2}
				}
				if kind == "color" {
					p.entries[textureKey{upload: u.key, depth: 4, clut: clut, color: true}] = &textureImage{scale: 2}
				}
				tracker := &textureTracking{pack: p}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					current := u
					if kind == "alternating" && i&1 != 0 {
						current = other
					}
					if memo {
						textureMemoBenchmarkResult = tracker.resolveTexture(current, 4, clut)
					} else {
						textureMemoBenchmarkResult = p.lookup(current.key, 4, clut)
					}
				}
			})
		}
	}
}
