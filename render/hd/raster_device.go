package hd

import "image"

// RasterJob 是已完成 PS1 覆蓋及貼圖快取處理的 HD 光柵工作。
// Texels 使用 RG 保存未調變的 RGB555/STP；A=0 表示沒有覆蓋，A=255 表示覆蓋。
// Color/DX/DY 仍為既有模 2^32 固定小數，座標為整張 HD VRAM 的座標。
// 裝置完成調變、抖色、混色、遮罩及寫入，不能接觸 Machine。
type RasterJob struct {
	Rect                               image.Rectangle
	Texels                             []byte
	Color, DX, DY                      [3]uint32
	Mode                               int
	Textured, Modulate, Dither, Sprite bool
	Shift                              uint
	MaskSet, MaskEval                  uint16
}

// RasterDevice 由 HD 消費端獨佔。影像資料只影響呈現，不回寫原生 GPU。
// Upload/Read 的 pixels 依矩形排列，沒有列間 padding。
// 實作不得保留呼叫端的切片。規格：021-hd-gpu-renderer.md。
type RasterDevice interface {
	Upload(rect image.Rectangle, pixels []uint16)
	Draw(job RasterJob)
	Read(rect image.Rectangle, pixels []uint16)
	Flush()
	Close()
}

// RasterStats 只描述呈現工作，不屬於原生機器狀態。
type RasterStats struct {
	Triangles, Sprites, Pixels, Fallbacks, Readbacks uint64
}
