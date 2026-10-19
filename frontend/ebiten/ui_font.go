package ebiten

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"

	eb "github.com/hajimehoshi/ebiten/v2"
)

// 字圖集來源、生成方式與 OFL-1.1 授權見 020-gamepad-toolbar §6。
//
//go:embed ui-font.png
var uiFontPNG []byte

//go:embed ui-font.json
var uiFontJSON []byte

type uiGlyph struct{ X, Y, W, H, Advance int }
type bitmapUIFont struct {
	title  *bitmapUIFont
	atlas  *eb.Image
	height int
	glyphs map[string]uiGlyph
}

func loadUIFont() *bitmapUIFont {
	var meta struct {
		TitleHeight int                `json:"title_height"`
		TitleGlyphs map[string]uiGlyph `json:"title_glyphs"`
		Height      int
		Glyphs      map[string]uiGlyph
	}
	if err := json.Unmarshal(uiFontJSON, &meta); err != nil {
		panic(err)
	}
	im, err := png.Decode(bytes.NewReader(uiFontPNG))
	if err != nil {
		panic(err)
	}
	if gray, ok := im.(*image.Gray); ok {
		im = &image.Alpha{Pix: gray.Pix, Stride: gray.Stride, Rect: gray.Rect}
	}
	atlas := eb.NewImageFromImage(im)
	font := &bitmapUIFont{atlas: atlas, height: meta.Height, glyphs: meta.Glyphs}
	font.title = &bitmapUIFont{atlas: atlas, height: meta.TitleHeight, glyphs: meta.TitleGlyphs}
	return font
}

func (f *bitmapUIFont) glyph(r rune) uiGlyph {
	if g, ok := f.glyphs[string(r)]; ok {
		return g
	}
	return f.glyphs["?"]
}

func (f *bitmapUIFont) width(text string) int {
	w := 0
	for _, r := range text {
		w += f.glyph(r).Advance
	}
	return w
}

func (f *bitmapUIFont) bounded(text string, width int) string {
	if f.width(text) <= width {
		return text
	}
	ellipsis := "…"
	room := width - f.width(ellipsis)
	if room < 0 {
		return ""
	}
	w := 0
	out := []rune{}
	for _, r := range text {
		a := f.glyph(r).Advance
		if w+a > room {
			break
		}
		out = append(out, r)
		w += a
	}
	return string(out) + ellipsis
}

// label 先依實際 advance 測量再省略，整行高度與點擊區共用 r。
func (f *bitmapUIFont) label(dst *eb.Image, r image.Rectangle, text string, c color.Color, center bool, scale float64) {
	if r.Empty() || float64(f.height)*scale > float64(r.Dy())+0.01 {
		return
	}
	text = f.bounded(text, int(math.Floor(float64(r.Dx())/scale)))
	x := float64(r.Min.X)
	if center {
		x += (float64(r.Dx()) - float64(f.width(text))*scale) / 2
	}
	y := float64(r.Min.Y) + (float64(r.Dy())-float64(f.height)*scale)/2
	for _, ch := range text {
		g := f.glyph(ch)
		op := &eb.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(x, y)
		op.ColorScale.ScaleWithColor(c)
		if scale != 1 {
			op.Filter = eb.FilterLinear
		}
		dst.DrawImage(f.atlas.SubImage(image.Rect(g.X, g.Y, g.X+g.W, g.Y+g.H)).(*eb.Image), op)
		x += float64(g.Advance) * scale
	}
}
