package gpu

import (
	"github.com/wicanr2/ps-golem/render/hd"
	"image"
)

// Shadow 的渲染與非同步佇列位於 render/hd，不持有機器指標。
type Shadow = hd.Shadow

func NewShadow(shift uint) *Shadow { return hd.NewShadow(shift) }

// SetShadow 在開機輸入送入前接上影子；nil 卸下影子，不負責 Close。
// 影子從上電狀態重播，不能在執行途中接上空白影子。
func (g *GPU) SetShadow(s *Shadow) { g.shadow = s }

// OutputScreen 只供呈現；Screen 與 VRAM 保持原生觀測語意。
func (g *GPU) OutputScreen() *image.RGBA {
	if g.shadow != nil {
		return g.shadow.Screen()
	}
	return g.Screen()
}

// OutputScreenInto 同 OutputScreen；沒有影子時重用 dst 的記憶體（見 ScreenInto）。
func (g *GPU) OutputScreenInto(dst *image.RGBA) *image.RGBA {
	if g.shadow != nil {
		return g.shadow.Screen()
	}
	return g.ScreenInto(dst)
}

// Clone 只複製原生機器狀態，不複製或共用影子。
func (g *GPU) Clone() *GPU {
	n := *g
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
