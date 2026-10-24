package ebiten

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/ps-golem/machine"
)

func posterFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var buf bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 24, 108))
	im.Set(12, 50, color.RGBA{150, 100, 30, 255})
	if err := png.Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"left.png", "right.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":1,"posters":[{"id":"a","name":"A","left":"left.png","right":"right.png"},{"id":"b","name":"B","left":"right.png","right":"left.png"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPosterPackValidation(t *testing.T) {
	dir := posterFixture(t)
	pack, err := LoadPosterPack(dir)
	if err != nil || len(pack.entries) != 2 {
		t.Fatalf("pack: %v", err)
	}
	original, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{string(bytes.ReplaceAll(original, []byte(`"id":"b"`), []byte(`"id":"a"`))), string(bytes.ReplaceAll(original, []byte("left.png"), []byte("../left.png"))), string(original) + `{}`, `{"version":2,"posters":[]}`} {
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPosterPack(dir); err == nil {
			t.Fatal("accepted bad manifest")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "left.png"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPosterPack(dir); err == nil {
		t.Fatal("accepted corrupt PNG")
	}
}

func TestPosterRejectsSymbolicLinks(t *testing.T) {
	dir := posterFixture(t)
	if err := os.Remove(filepath.Join(dir, "left.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("right.png", filepath.Join(dir, "left.png")); err != nil {
		t.Skip(err)
	}
	if _, err := LoadPosterPack(dir); err == nil {
		t.Fatal("accepted symbolic link")
	}
}

func TestPosterZoomProtectsAnyVisiblePixel(t *testing.T) {
	im := image.NewRGBA(image.Rect(5, 7, 325, 247))
	im.Set(180, 100, color.RGBA{180, 90, 30, 255})
	before := sha256.Sum256(im.Pix)
	if !posterZoomSafe(im) {
		t.Fatal("black margin rejected")
	}
	if sha256.Sum256(im.Pix) != before {
		t.Fatal("mutated frame")
	}
	for _, pt := range []image.Point{{5, 7}, {324, 14}, {5, 239}, {324, 246}} {
		im.Set(pt.X, pt.Y, color.RGBA{1, 0, 0, 255})
		if posterZoomSafe(im) {
			t.Fatal("would crop visible pixel", pt)
		}
		im.Set(pt.X, pt.Y, color.RGBA{})
	}
	if posterZoomSafe(nil) {
		t.Fatal("nil image safe")
	}
}

func TestPosterGeometryAndAtomicSelection(t *testing.T) {
	canvas, r := posterGeometry(1920, 1080, true)
	if canvas != image.Rect(0, 0, 1920, 1080) || r != image.Rect(192, -36, 1728, 1116) {
		t.Fatal(canvas, r)
	}
	_, r = posterGeometry(1920, 1080, false)
	if r != image.Rect(240, 0, 1680, 1080) {
		t.Fatal(r)
	}
	dir := posterFixture(t)
	pack, err := LoadPosterPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved := []string{}
	g := &Game{cfg: Config{Posters: pack, SavePoster: func(s PosterSettings) error { saved = append(saved, s.ID); return nil }}, poster: PosterSettings{Soft: true, Zoom: true}}
	for _, want := range []string{"a", "b", ""} {
		g.cyclePoster()
		if g.poster.ID != want || !g.poster.Soft || !g.poster.Zoom {
			t.Fatal(g.poster)
		}
	}
	g.cfg.SavePoster = func(PosterSettings) error { return errors.New("write denied") }
	g.cyclePoster()
	if g.poster.ID != "" || g.posterError == "" {
		t.Fatal("failed save changed presentation")
	}
	if len(saved) != 3 {
		t.Fatal(saved)
	}
}

func TestPosterShaderAndNativeState(t *testing.T) {
	shader, err := eb.NewShader([]byte(posterBlendSource))
	if err != nil {
		t.Fatal(err)
	}
	shader.Deallocate()
	path := os.Getenv("PSGOLEM_BIOS")
	if path == "" {
		t.Skip("actual BIOS not supplied for state-neutrality check")
	}
	bios, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := machine.New(bios)
	if err != nil {
		t.Fatal(err)
	}
	m.RunUntilField(1, 1200000)
	before := m.Digest()
	card := append([]byte(nil), m.Card1.Data[:]...)
	im := m.GPU.OutputScreenInto(nil)
	posterZoomSafe(im)
	posterGeometry(1920, 1080, true)
	posterGeometry(800, 600, false)
	if m.Digest() != before || !bytes.Equal(card, m.Card1.Data[:]) {
		t.Fatal("presentation changed native state")
	}
}
