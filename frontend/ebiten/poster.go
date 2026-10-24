package ebiten

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	eb "github.com/hajimehoshi/ebiten/v2"
)

// PosterSettings affects presentation only (024-poster-presentation).
type PosterSettings struct {
	ID   string `json:"id"`
	Soft bool   `json:"soft"`
	Zoom bool   `json:"zoom"`
}

type posterArt struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Left   string `json:"left"`
	Right  string `json:"right"`
	images [2]image.Image
}

// PosterPack contains complete, validated external artwork; no game assets
// are embedded in the public executor.
type PosterPack struct{ entries []posterArt }

var posterID = regexp.MustCompile(`^[A-Za-z0-9-]{1,32}$`)

func (s PosterSettings) Validate() error {
	if s.ID != "" && !posterID.MatchString(s.ID) {
		return fmt.Errorf("無效海報代號")
	}
	return nil
}

func LoadPosterPack(dir string) (*PosterPack, error) {
	read := func(name string, limit int64) ([]byte, error) {
		if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
			return nil, fmt.Errorf("無效海報檔名")
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > limit {
			return nil, fmt.Errorf("海報檔案超限或不是一般檔案：%s", name)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, limit+1))
		if int64(len(b)) > limit {
			return nil, fmt.Errorf("海報檔案超限")
		}
		return b, err
	}
	b, err := read("manifest.json", 64<<10)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Version int         `json:"version"`
		Posters []posterArt `json:"posters"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&manifest); err != nil {
		return nil, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("海報清單尾端資料無效")
	}
	if manifest.Version != 1 || len(manifest.Posters) < 1 || len(manifest.Posters) > 8 {
		return nil, fmt.Errorf("海報清單版本或數量無效")
	}
	seen := map[string]bool{}
	total := 0
	for n := range manifest.Posters {
		a := &manifest.Posters[n]
		if !posterID.MatchString(a.ID) || seen[a.ID] || utf8.RuneCountInString(a.Name) > 64 {
			return nil, fmt.Errorf("海報代號重複或名稱超限")
		}
		seen[a.ID] = true
		for side, name := range []string{a.Left, a.Right} {
			if filepath.Ext(name) != ".png" {
				return nil, fmt.Errorf("海報須使用PNG")
			}
			data, e := read(name, 8<<20)
			if e != nil {
				return nil, e
			}
			cfg, e := png.DecodeConfig(bytes.NewReader(data))
			if e != nil {
				return nil, e
			}
			if cfg.Width < 1 || cfg.Width > 2048 || cfg.Height < 1 || cfg.Height > 2160 {
				return nil, fmt.Errorf("海報尺寸超限")
			}
			total += cfg.Width * cfg.Height * 4
			if total > 32<<20 {
				return nil, fmt.Errorf("海報解碼圖像超過32MiB")
			}
			a.images[side], e = png.Decode(bytes.NewReader(data))
			if e != nil {
				return nil, e
			}
		}
	}
	return &PosterPack{entries: manifest.Posters}, nil
}

func (p *PosterPack) index(id string) int {
	if p != nil {
		for n, a := range p.entries {
			if a.ID == id {
				return n
			}
		}
	}
	return -1
}

func posterZoomSafe(img *image.RGBA) bool {
	if img == nil || img.Bounds().Empty() {
		return false
	}
	b := img.Bounds()
	rows := (b.Dy() + 31) / 32
	for _, part := range []image.Rectangle{image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+rows), image.Rect(b.Min.X, b.Max.Y-rows, b.Max.X, b.Max.Y)} {
		for y := part.Min.Y; y < part.Max.Y; y++ {
			for x := part.Min.X; x < part.Max.X; x++ {
				i := img.PixOffset(x, y)
				if img.Pix[i]|img.Pix[i+1]|img.Pix[i+2] != 0 {
					return false
				}
			}
		}
	}
	return true
}

func posterGeometry(w, h int, zoom bool) (canvas, game image.Rectangle) {
	canvas = fitWide(w, h)
	game = fit(canvas.Dx(), canvas.Dy()).Add(canvas.Min)
	if zoom {
		size := image.Pt(game.Dx()*16/15, game.Dy()*16/15)
		center := canvas.Min.Add(canvas.Size().Div(2))
		game = image.Rectangle{Min: center.Sub(size.Div(2)), Max: center.Sub(size.Div(2)).Add(size)}
	}
	return
}

func (g *Game) initPosterImages() {
	if g.posterImages != nil || g.cfg.Posters == nil {
		return
	}
	g.posterImages = make([][2]*eb.Image, len(g.cfg.Posters.entries))
	for n, a := range g.cfg.Posters.entries {
		for side, img := range a.images {
			g.posterImages[n][side] = eb.NewImageFromImage(img)
		}
	}
}

func drawPosterCover(dst, src *eb.Image, r image.Rectangle, brightness float32) {
	if r.Empty() {
		return
	}
	b := src.Bounds()
	sx, sy := float64(r.Dx())/float64(b.Dx()), float64(r.Dy())/float64(b.Dy())
	scale := max(sx, sy)
	op := &eb.DrawImageOptions{Filter: eb.FilterLinear}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(r.Min.X)+(float64(r.Dx())-float64(b.Dx())*scale)/2, float64(r.Min.Y)+(float64(r.Dy())-float64(b.Dy())*scale)/2)
	op.ColorScale.Scale(brightness, brightness, brightness, 1)
	dst.SubImage(r).(*eb.Image).DrawImage(src, op)
}

// drawPosters never reads or writes the emulated machine. Destination clipping
// keeps both blend bands strictly outside the central game viewport.
func (g *Game) drawPosters(screen *eb.Image, index int) (image.Rectangle, *eb.Image) {
	g.initPosterImages()
	size := screen.Bounds().Size()
	canvas, r := posterGeometry(size.X, size.Y, g.poster.Zoom && g.posterSafeZoom)
	left := image.Rect(canvas.Min.X, canvas.Min.Y, r.Min.X, canvas.Max.Y)
	right := image.Rect(r.Max.X, canvas.Min.Y, canvas.Max.X, canvas.Max.Y)
	light := float32(1)
	if g.poster.Soft {
		light = .9
	}
	drawPosterCover(screen, g.posterImages[index][0], left, light)
	drawPosterCover(screen, g.posterImages[index][1], right, light)
	if g.poster.Soft && g.posterShader != nil {
		b := g.frame.Bounds()
		sx, sy := float64(r.Dx())/float64(b.Dx()), float64(r.Dy())/float64(b.Dy())
		band := min(left.Dx(), max(1, canvas.Dx()*48/1920))
		for side, clip := range []image.Rectangle{image.Rect(r.Min.X-band, canvas.Min.Y, r.Min.X, canvas.Max.Y), image.Rect(r.Max.X, canvas.Min.Y, r.Max.X+band, canvas.Max.Y)} {
			if clip.Empty() {
				continue
			}
			op := &eb.DrawRectShaderOptions{Uniforms: map[string]any{"Edge": []float32{float32(float64(band) / sx), float32(side)}, "Blur": float32(5 / sx)}}
			op.Images[0] = g.frame
			op.GeoM.Scale(-sx, sy)
			x := float64(r.Min.X)
			if side == 1 {
				x = float64(r.Max.X + r.Dx())
			}
			op.GeoM.Translate(x, float64(r.Min.Y))
			screen.SubImage(clip).(*eb.Image).DrawRectShader(b.Dx(), b.Dy(), g.posterShader, op)
		}
	}
	return r, screen.SubImage(canvas).(*eb.Image)
}

func (g *Game) changePoster(next PosterSettings) {
	if err := next.Validate(); err != nil {
		g.posterError = err.Error()
		return
	}
	if g.cfg.SavePoster != nil {
		if err := g.cfg.SavePoster(next); err != nil {
			g.posterError = "儲存失敗：" + err.Error()
			g.shown = image.Point{}
			return
		}
	}
	g.poster = next
	g.posterError = ""
	g.shown = image.Point{}
}

func (g *Game) cyclePoster() {
	if g.cfg.Posters == nil {
		return
	}
	next := g.poster
	i := g.cfg.Posters.index(next.ID) + 1
	if i >= len(g.cfg.Posters.entries) {
		next.ID = ""
	} else {
		next.ID = g.cfg.Posters.entries[i].ID
	}
	g.changePoster(next)
}

const posterBlendSource = `//kage:unit pixels
package main
var Edge vec2
var Blur float
func Fragment(dstPos vec4,srcPos vec2,color vec4) vec4 {
 p:=srcPos-imageSrc0Origin()
 distance:=p.x
 if Edge.y>0 { distance=imageSrc0Size().x-p.x }
 alpha:=clamp(1-distance/Edge.x,0,1)
 sum:=vec4(0)
 for y:=-1;y<=1;y++ {for x:=-1;x<=1;x++ {sum+=imageSrc0At(srcPos+vec2(float(x),float(y))*Blur)}}
 return sum*(alpha/9)
}`
