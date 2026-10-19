// Package hdgpu 以 Ebitengine/Kage 執行 HD 像素光柵工作。
// 規格：docs/spec/021-hd-gpu-renderer.md。
// API 契約來源：Ebitengine v2.9.9 image.go、shader.go（Apache-2.0）。
// 著色器依本專案 render/hd/raster.go 的整數運算獨立撰寫。
package hdgpu

import (
	_ "embed"
	"fmt"
	"image"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/ps-golem/render/hd"
)

//go:embed raster.kage
var shaderSource []byte

type Device struct {
	front                     *eb.Image
	shader                    *eb.Shader
	input, background, output *eb.Image
	size                      image.Point
	bytes                     []byte
	closed                    bool
	Draws, Pixels, Readbacks  uint64
}

// New 配置呈現裝置；Draw/Read 僅能在 Ebitengine 主迴圈運作期間使用。
func New(shift uint) (*Device, error) {
	if shift > 3 {
		return nil, fmt.Errorf("GPU HD shift 超出範圍：%d", shift)
	}
	s, err := eb.NewShader(shaderSource)
	if err != nil {
		return nil, fmt.Errorf("GPU HD shader：%w", err)
	}
	d := &Device{front: eb.NewImageWithOptions(image.Rect(0, 0, 1024<<shift, 512<<shift), &eb.NewImageOptions{Unmanaged: true}), shader: s}
	activeDevices.Add(1)
	return d, nil
}

func (d *Device) scratch(w, h int) {
	if d.size.X >= w && d.size.Y >= h {
		return
	}
	for _, i := range []*eb.Image{d.input, d.background, d.output} {
		if i != nil {
			i.Deallocate()
		}
	}
	pow2 := func(n int) int {
		p := 1
		for p < n {
			p *= 2
		}
		return p
	}
	d.size = image.Pt(pow2(max(w, d.size.X)), pow2(max(h, d.size.Y)))
	d.input = eb.NewImageWithOptions(image.Rect(0, 0, d.size.X, d.size.Y), &eb.NewImageOptions{Unmanaged: true})
	d.background = eb.NewImageWithOptions(image.Rect(0, 0, d.size.X, d.size.Y), &eb.NewImageOptions{Unmanaged: true})
	d.output = eb.NewImageWithOptions(image.Rect(0, 0, d.size.X, d.size.Y), &eb.NewImageOptions{Unmanaged: true})
}

func (d *Device) upload(rect image.Rectangle, words []uint16) {
	n := rect.Dx() * rect.Dy() * 4
	if cap(d.bytes) < n {
		d.bytes = make([]byte, n)
	}
	b := d.bytes[:n]
	for i, v := range words {
		b[i*4], b[i*4+1], b[i*4+2], b[i*4+3] = byte(v), byte(v>>8), 0, 255
	}
	d.front.SubImage(rect).(*eb.Image).WritePixels(b)
}

func split(v [3]uint32, high bool) []float32 {
	r := make([]float32, 3)
	for i, w := range v {
		if high {
			w >>= 16
		}
		r[i] = float32(w & 65535)
	}
	return r
}

func (d *Device) draw(j hd.RasterJob) {
	w, h := j.Rect.Dx(), j.Rect.Dy()
	if w <= 0 || h <= 0 {
		return
	}
	d.scratch(w, h)
	r := image.Rect(0, 0, w, h)
	in := d.input.SubImage(r).(*eb.Image)
	bg := d.background.SubImage(r).(*eb.Image)
	out := d.output.SubImage(r).(*eb.Image)
	in.WritePixels(j.Texels)
	copyOp := &eb.DrawImageOptions{Blend: eb.BlendCopy}
	bg.DrawImage(d.front.SubImage(j.Rect).(*eb.Image), copyOp)
	bit := func(b bool) float32 {
		if b {
			return 1
		}
		return 0
	}
	op := &eb.DrawRectShaderOptions{Blend: eb.BlendCopy}
	op.Images[0], op.Images[1] = in, bg
	op.Uniforms = map[string]any{
		"Origin":  []float32{float32(j.Rect.Min.X), float32(j.Rect.Min.Y)},
		"ColorLo": split(j.Color, false), "ColorHi": split(j.Color, true),
		"DXLo": split(j.DX, false), "DXHi": split(j.DX, true),
		"DYLo": split(j.DY, false), "DYHi": split(j.DY, true),
		"Mode": float32(j.Mode), "Textured": bit(j.Textured), "Modulate": bit(j.Modulate), "Dither": bit(j.Dither), "Sprite": bit(j.Sprite),
		"Scale": float32(uint32(1) << j.Shift), "MaskSet": float32(j.MaskSet), "MaskEval": float32(j.MaskEval),
	}
	out.DrawRectShader(w, h, d.shader, op)
	put := &eb.DrawImageOptions{Blend: eb.BlendCopy}
	put.GeoM.Translate(float64(j.Rect.Min.X), float64(j.Rect.Min.Y))
	d.front.DrawImage(out, put)
	d.Draws++
	d.Pixels += uint64(w * h)
}

func (d *Device) read(rect image.Rectangle, words []uint16) {
	n := rect.Dx() * rect.Dy() * 4
	if cap(d.bytes) < n {
		d.bytes = make([]byte, n)
	}
	b := d.bytes[:n]
	d.front.SubImage(rect).(*eb.Image).ReadPixels(b)
	for i := range words {
		words[i] = uint16(b[i*4]) | uint16(b[i*4+1])<<8
	}
	d.Readbacks++
}

func (d *Device) Close() {
	if d.closed {
		return
	}
	d.closed = true
	activeDevices.Add(-1)
	for _, i := range []*eb.Image{d.front, d.input, d.background, d.output} {
		if i != nil {
			i.Deallocate()
		}
	}
	d.shader.Deallocate()
}

// flush 是提交界線。CPU需要像素時由Read同步，不為每場額外強制回讀。
func (d *Device) flush() {}

func (d *Device) Upload(r image.Rectangle, p []uint16) { d.call(false, func() { d.upload(r, p) }) }
func (d *Device) Draw(j hd.RasterJob)                  { d.call(false, func() { d.draw(j) }) }
func (d *Device) Read(r image.Rectangle, p []uint16)   { d.call(false, func() { d.read(r, p) }) }
func (d *Device) Flush()                               { d.call(true, func() { d.flush() }) }
