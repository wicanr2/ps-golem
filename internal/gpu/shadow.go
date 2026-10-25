package gpu

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"github.com/wicanr2/ps-golem/render/hd"
	"image"
	"sync/atomic"
)

// Shadow 的渲染與非同步佇列位於 render/hd，不持有機器指標。
type Shadow = hd.Shadow

func NewShadow(shift uint) *Shadow { return hd.NewShadow(shift) }

// SetShadow 在開機輸入送入前接上影子；nil 卸下影子，不負責 Close。
// 影子從上電狀態重播，不能在執行途中接上空白影子。
func (g *GPU) SetShadow(s *Shadow) { g.shadow = s }

func (g *GPU) PrecisionSupported() bool { return g.shadow != nil && g.shadow.Scale() >= 2 }

// WriteGP0Precision 的附件只送影子，原生處理只讀 v。
// 參照 beetle-psx-libretro mednafen/psx/gpu.c PGXP_WriteCB／PGXP_WriteFIFO，
// commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later。
func (g *GPU) WriteGP0Precision(v uint32, p pgxp.Value) {
	if g.shadow != nil {
		g.shadow.GP0Precision(v, p)
	}
	g.writeGP0(v)
}

// OutputScreen 只供呈現；Screen 與 VRAM 保持原生觀測語意。
func (g *GPU) OutputScreen() *image.RGBA {
	if g.shadow != nil {
		return g.shadow.Screen()
	}
	return g.Screen()
}

// OutputScreenInto 同 OutputScreen；重用相容的後備緩衝（016 §8）。
func (g *GPU) OutputScreenInto(dst *image.RGBA) *image.RGBA {
	if g.shadow != nil {
		return g.shadow.ScreenInto(dst)
	}
	return g.ScreenInto(dst)
}

// Clone 只複製原生機器狀態，不複製或共用影子。
func (g *GPU) Clone() *GPU {
	g.syncNativePixels()
	n := *g
	n.nativePlane = nil
	n.nativeSkipped = [6]uint64{}
	n.shadow = nil
	return &n
}

// HDShift 驗證使用者輸入；0 表示停用，1 用於轉送自我檢查。
func HDShift(n int) (uint, bool) {
	switch n {
	case 0, 1:
		return 0, true
	case 2:
		return 1, true
	case 4:
		return 2, true
	case 8:
		return 3, true
	}
	return 0, false
}

// TextureField 只將 field 編號附送呈現層，不改原生 GPU 狀態。
func (g *GPU) TextureField(field uint64) {
	if g.shadow != nil {
		g.shadow.TextureField(field)
	}
}

// RequestOutputScreen 非同步取得 HD 畫面；未掛影子時回傳 false，呼叫者改用同步原生路徑。
func (g *GPU) RequestOutputScreen(dst *image.RGBA, fn func(*image.RGBA)) bool {
	if g.shadow == nil {
		return false
	}
	if g.nativePlane == nil {
		g.shadow.RequestScreen(dst, fn)
		return true
	}
	var left atomic.Int32
	left.Store(2)
	var img *image.RGBA
	finish := func() {
		if left.Add(-1) == 0 {
			fn(img)
		}
	}
	g.nativePlane.Fence(finish)
	g.shadow.RequestScreen(dst, func(v *image.RGBA) { img = v; finish() })
	return true
}

// RequestFence 在影子命令邊界通知完成；原生同步路徑回傳 false，不呼叫 fn。
func (g *GPU) RequestFence(fn func()) bool {
	if g.shadow == nil && g.nativePlane == nil {
		return false
	}
	if g.nativePlane == nil {
		g.shadow.Fence(fn)
		return true
	}
	if g.shadow == nil {
		g.nativePlane.Fence(fn)
		return true
	}
	var left atomic.Int32
	left.Store(2)
	finish := func() {
		if left.Add(-1) == 0 {
			fn()
		}
	}
	g.nativePlane.Fence(finish)
	g.shadow.Fence(finish)
	return true
}
