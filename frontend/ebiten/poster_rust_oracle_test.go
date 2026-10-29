package ebiten

// 024 section6: read the same real paused frame as the Rust frontend. This
// oracle reuses the existing Go geometry/loader/shader, never starts a Machine.
// GPL-3.0-or-later. Original/HD artwork and screenshots stay outside this repo.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"
)

type rustPosterCase struct {
	Name, Source, ID string
	Soft, Zoom       bool
	Width, Height    int
}
type rustPosterOracle struct {
	g       Game
	cases   []rustPosterCase
	at      int
	root    string
	records []map[string]any
	err     error
}

func (p *rustPosterOracle) Update() error {
	if p.err != nil {
		return p.err
	}
	if p.at >= len(p.cases) {
		return eb.Termination
	}
	return nil
}
func (p *rustPosterOracle) Layout(int, int) (int, int) { return 1920, 1080 }
func (p *rustPosterOracle) Draw(screen *eb.Image) {
	if p.at >= len(p.cases) || p.err != nil {
		return
	}
	c := p.cases[p.at]
	screen = eb.NewImage(c.Width, c.Height)
	defer screen.Dispose()
	f, e := os.Open(filepath.Join(p.root, c.Source))
	if e != nil {
		p.err = e
		return
	}
	decoded, e := png.Decode(f)
	f.Close()
	if e != nil {
		p.err = e
		return
	}
	b := decoded.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), decoded, b.Min, draw.Src)
	p.g.frame = eb.NewImageFromImage(img)
	p.g.poster = PosterSettings{ID: c.ID, Soft: c.Soft, Zoom: c.Zoom}
	p.g.posterSafeZoom = posterZoomSafe(img)
	screen.Clear()
	canvas, rect := posterGeometry(c.Width, c.Height, false)
	if index := p.g.cfg.Posters.index(c.ID); index >= 0 {
		var target *eb.Image
		rect, target = p.g.drawPosters(screen, index)
		_ = target
		canvas, _ = posterGeometry(c.Width, c.Height, c.Zoom && p.g.posterSafeZoom)
	}
	op := &eb.DrawImageOptions{Filter: eb.FilterNearest}
	op.GeoM.Scale(float64(rect.Dx())/float64(b.Dx()), float64(rect.Dy())/float64(b.Dy()))
	op.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.SubImage(canvas).(*eb.Image).DrawImage(p.g.frame, op)
	rgba := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
	screen.ReadPixels(rgba.Pix)
	out, e := os.Create(filepath.Join(p.root, "go-"+c.Name+".png"))
	if e == nil {
		e = png.Encode(out, rgba)
		out.Close()
	}
	if e != nil {
		p.err = e
		return
	}
	p.records = append(p.records, map[string]any{"name": c.Name, "source_sha256": fmt.Sprintf("%x", sha256.Sum256(img.Pix)), "safe_zoom": p.g.posterSafeZoom, "canvas": []int{canvas.Min.X, canvas.Min.Y, canvas.Dx(), canvas.Dy()}, "game": []int{rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy()}})
	p.g.frame.Dispose()
	p.at++
}
func runRustPosterOracle() error {
	root := os.Getenv("PSGOLEM_RUST_POSTER_ORACLE_ROOT")
	packPath := os.Getenv("PSGOLEM_RUST_POSTER_PACK")
	data, err := os.ReadFile(filepath.Join(root, "oracle-cases.json"))
	if err != nil {
		return err
	}
	p := &rustPosterOracle{root: root}
	if err = json.Unmarshal(data, &p.cases); err != nil {
		return err
	}
	if len(p.cases) < 1 || len(p.cases) > 32 {
		return fmt.Errorf("oracle case count")
	}
	for _, c := range p.cases {
		if filepath.Base(c.Name) != c.Name || filepath.Base(c.Source) != c.Source || c.Width < 320 || c.Width > 7680 || c.Height < 240 || c.Height > 4320 {
			return fmt.Errorf("oracle input bounds")
		}
	}
	p.g.cfg.Posters, err = LoadPosterPack(packPath)
	if err != nil {
		return err
	}
	p.g.posterShader, err = eb.NewShader([]byte(posterBlendSource))
	if err != nil {
		return err
	}
	eb.SetWindowSize(1920, 1080)
	eb.SetWindowTitle("ps-golem poster oracle")
	if err = eb.RunGame(p); err != nil {
		return err
	}
	if p.err != nil {
		return p.err
	}
	if p.at != len(p.cases) {
		return fmt.Errorf("rendered %d / %d", p.at, len(p.cases))
	}
	data, err = json.MarshalIndent(p.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "go-oracle.json"), data, 0600)
}

// Ebitengine requires RunGame on the main goroutine. Ordinary tests retain the
// existing m.Run path; only this explicitly selected fixture runs a window.
func TestMain(m *testing.M) {
	if os.Getenv("PSGOLEM_RUST_POSTER_ORACLE_ROOT") != "" {
		if err := runRustPosterOracle(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}
func TestRustPosterPresentationOracle(t *testing.T) {
	root := os.Getenv("PSGOLEM_RUST_POSTER_ORACLE_ROOT")
	if root == "" {
		t.Skip("select real local paused frames and adopted poster pack")
	}
	data, err := os.ReadFile(filepath.Join(root, "go-oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	if err = json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) < 1 {
		t.Fatal("no rendered oracle frames")
	}
}
