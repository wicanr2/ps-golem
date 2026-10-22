package hd

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func runFixture(t *testing.T) (*TexturePack, string, []uint16, []uint32) {
	t.Helper()
	dir := t.TempDir()
	raw := make([]uint16, 16)
	for i := range raw {
		raw[i] = 0x1111
	}
	key := textureUploadDigest(2, 8, raw)
	savePNG := func(name string, w, h int, pixels []uint8) {
		im := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}})
		copy(im.Pix, pixels)
		f, e := os.Create(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		e = png.Encode(f, im)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	font := make([]byte, 64)
	for i := range font {
		font[i] = 1
	}
	savePNG("font.png", 8, 8, font)
	pixels := make([]byte, 64*32)
	for y := 0; y < 32; y++ {
		for x := 4; x < 28; x++ {
			pixels[y*64+x] = 2
		}
	}
	savePNG("run.png", 64, 32, pixels)
	manifest := map[string]any{"version": 1, "entries": []any{map[string]any{"upload_sha256": fmtDigest(key), "width_words": 2, "height": 8, "depth": 4, "scale": 1, "kind": "indexed", "image": "font.png"}}}
	writeJSON := func(name string, v any) {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	writeJSON("manifest.json", manifest)
	steps := []any{map[string]any{"words": []uint32{0x65000000, 16 | 32<<16, 0x64000000, 0x80008}, "source": []int{0, 0}, "image_offset": []int{8, 0}}, map[string]any{"words": []uint32{0x65000000, 8 | 32<<16, 0x64000000, 0x80008}, "source": []int{0, 0}, "image_offset": []int{0, 0}}}
	writeJSON("sprite-runs.json", map[string]any{"version": 1, "runs": []any{map[string]any{"id": "synthetic", "upload_sha256": fmtDigest(key), "width_words": 2, "height": 8, "tpage": []int{128, 0}, "offset": []int{0, 0}, "buffer_y256": true, "image_width": 16, "image_height": 8, "scale": 4, "image": "run.png", "steps": steps}}})
	p, e := LoadTexturePack(dir)
	if e != nil {
		t.Fatal(e)
	}
	return p, dir, raw, []uint32{0x65000000, 16 | 32<<16, 0x64000000, 0x80008, 0x65000000, 8 | 32<<16, 0x64000000, 0x80008}
}
func fmtDigest(d [32]byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 64)
	for i, v := range d {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&15]
	}
	return string(out)
}
func runWords(g *Renderer, words []uint32) {
	for _, w := range words {
		g.WriteGP0(w)
	}
}
func runUpload(g *Renderer, x, y, w, h uint32, data []uint16) {
	runWords(g, []uint32{0xa0000000, x | y<<16, w | h<<16})
	for i := 0; i < len(data); i += 2 {
		v := uint32(data[i])
		if i+1 < len(data) {
			v |= uint32(data[i+1]) << 16
		}
		g.WriteGP0(v)
	}
}
func runRenderer(p *TexturePack, raw []uint16) *Renderer {
	g := newRenderer(2)
	g.configureTextures(p, nil)
	runWords(g, []uint32{0xe1000002, 0xe3000000, 0xe4000000 | 1023 | 511<<10})
	runUpload(g, 128, 0, 2, 8, raw)
	palette := make([]uint16, 16)
	palette[1] = 31
	palette[2] = 992
	runUpload(g, 0, 400, 16, 1, palette)
	return g
}

func TestSpriteRunsCompleteAndBufferPlane(t *testing.T) {
	p, _, raw, words := runFixture(t)
	for _, plane := range []uint32{0, 256} {
		g := runRenderer(p, raw)
		cb := append([]uint32(nil), words...)
		cb[1] += plane << 16
		cb[5] += plane << 16
		runWords(g, cb)
		if g.spriteRun.stats.Matches != 1 || g.runImage != nil || g.spriteRun.count != 0 {
			t.Fatal("run not committed or override escaped")
		}
		stride := 1024 << 2
		y := int(32+plane) << 2
		if g.VRAM[y*stride+9*4] != 992 || g.VRAM[y*stride+17*4] != 0 {
			t.Fatal("small replacement or cleared mark/cell incorrect")
		}
	}
}

func TestSpriteRunsFallbackAndBoundaries(t *testing.T) {
	p, _, raw, words := runFixture(t)
	parent := &TexturePack{entries: p.entries}
	for _, kind := range []string{"source", "wrong-second", "screen", "GP1", "field", "missing-tail"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]uint16(nil), raw...)
			cb := append([]uint32(nil), words...)
			if kind == "source" {
				data[0] ^= 1
			}
			if kind == "wrong-second" {
				cb[6]++
			}
			a, b := runRenderer(p, data), runRenderer(parent, data)
			if kind == "screen" || kind == "GP1" || kind == "field" || kind == "missing-tail" {
				runWords(a, cb[:4])
				runWords(b, cb[:4])
				switch kind {
				case "screen":
					a.Screen()
					b.Screen()
				case "GP1":
					a.WriteGP1(0x02000000)
					b.WriteGP1(0x02000000)
				case "field":
					a.SetField(true)
					b.SetField(true)
				case "missing-tail":
					a.flushSpriteRun()
				}
				if kind != "missing-tail" {
					runWords(a, cb[4:])
					runWords(b, cb[4:])
				}
			} else {
				runWords(a, cb)
				runWords(b, cb)
			}
			a.flushSpriteRun()
			b.flushSpriteRun()
			if !reflect.DeepEqual(a.VRAM, b.VRAM) || a.textures.stats != b.textures.stats || a.spriteRun.stats.Matches != 0 {
				t.Fatal("fallback changed native draw or texture accounting")
			}
			for i := range a.texCache {
				if a.texCache[i].tag != b.texCache[i].tag || a.texCache[i].data != b.texCache[i].data {
					t.Fatal("source probe mutated texture cache")
				}
			}
		})
	}
}

func TestSpriteRunsSchemaAndOptionalFile(t *testing.T) {
	_, dir, _, _ := runFixture(t)
	path := filepath.Join(dir, "sprite-runs.json")
	valid, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{`{"version":1,"runs":[],"Runs":[]}`, `{"version":1,"version":1,"runs":[]}`, `{"version":1,"runs":null}`, `{"version":1,"runs":[]} true`, `{"Version":1,"runs":[]}`} {
		if e = os.WriteFile(path, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		if p, e := LoadTexturePack(dir); e == nil || p != nil {
			t.Fatal("accepted malformed optional attachment")
		}
	}
	var doc map[string]any
	if e = json.Unmarshal(valid, &doc); e != nil {
		t.Fatal(e)
	}
	rule := doc["runs"].([]any)[0].(map[string]any)
	rule["image"] = "../escape.png"
	bad, _ := json.Marshal(doc)
	_ = os.WriteFile(path, bad, 0600)
	if p, e := LoadTexturePack(dir); e == nil || p != nil {
		t.Fatal("accepted escaping image")
	}
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	p, e := LoadTexturePack(dir)
	if e != nil || p.SpriteRunCount() != 0 || p.Len() != 1 {
		t.Fatal("legacy pack changed", e)
	}
}

func TestSpriteRunsRepeatedStateCommands(t *testing.T) {
	p, _, raw, words := runFixture(t)
	for _, state := range []uint32{0xe1000002, 0xe2000000, 0xe1000022, 0xe2000001} {
		parent := &TexturePack{entries: p.entries}
		a, b := runRenderer(p, raw), runRenderer(parent, raw)
		for _, g := range []*Renderer{a, b} {
			runWords(g, words[:4])
			g.WriteGP0(state)
			runWords(g, words[4:])
			g.flushSpriteRun()
		}
		noop := state == 0xe1000002 || state == 0xe2000000
		if noop {
			if a.spriteRun.stats.Matches != 1 || reflect.DeepEqual(a.VRAM, b.VRAM) {
				t.Fatalf("unchanged state interrupted sequence %x", state)
			}
		} else if a.spriteRun.stats.Matches != 0 || !reflect.DeepEqual(a.VRAM, b.VRAM) || a.textures.stats != b.textures.stats {
			t.Fatalf("changed state failed exact fallback %x", state)
		}
	}
}
