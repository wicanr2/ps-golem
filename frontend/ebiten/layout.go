package ebiten

import "image"

// fit 回傳把 4:3 畫面放進 w×h 視窗時的目標矩形（置中，012 §4）。
func fit(w, h int) image.Rectangle {
	dw, dh := w, w*3/4
	if dh > h {
		dw, dh = h*4/3, h
	}
	x, y := (w-dw)/2, (h-dh)/2
	return image.Rect(x, y, x+dw, y+dh)
}

// fitWide 依 16:9 比例置中。
func fitWide(w, h int) image.Rectangle {
	dw, dh := w, w*9/16
	if dh > h {
		dw, dh = h*16/9, h
	}
	x, y := (w-dw)/2, (h-dh)/2
	return image.Rect(x, y, x+dw, y+dh)
}
