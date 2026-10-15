package machine

import (
	"bytes"
	"image"
	"image/png"
)

// VRAMPNG 依 007 §2.1 把 VRAM 存成 PNG。
func VRAMPNG(vram []uint16) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1024, 512))
	for i, p := range vram {
		img.Pix[i*4+0] = uint8(p&31) << 3
		img.Pix[i*4+1] = uint8((p>>5)&31) << 3
		img.Pix[i*4+2] = uint8((p>>10)&31) << 3
		img.Pix[i*4+3] = 255
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
