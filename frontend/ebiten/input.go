package ebiten

import (
	eb "github.com/hajimehoshi/ebiten/v2"

	"github.com/wicanr2/ps-golem/internal/sio"
)

// keyMap 是鍵盤對應（docs/spec/012-frontend-ebiten.md §5）。
var keyMap = []struct {
	key eb.Key
	btn uint16
}{
	{eb.KeyArrowUp, sio.BtnUp}, {eb.KeyArrowDown, sio.BtnDown},
	{eb.KeyArrowLeft, sio.BtnLeft}, {eb.KeyArrowRight, sio.BtnRight},
	{eb.KeyZ, sio.BtnCross}, {eb.KeyX, sio.BtnCircle},
	{eb.KeyA, sio.BtnSquare}, {eb.KeyS, sio.BtnTriangle},
	{eb.KeyQ, sio.BtnL1}, {eb.KeyW, sio.BtnR1},
	{eb.Key1, sio.BtnL2}, {eb.Key3, sio.BtnR2},
	{eb.KeyEnter, sio.BtnStart}, {eb.KeyBackspace, sio.BtnSelect},
}

// padMap 是 Ebiten 標準手把對應（012 §5）。
var padMap = []struct {
	btn eb.StandardGamepadButton
	ps  uint16
}{
	{eb.StandardGamepadButtonLeftTop, sio.BtnUp}, {eb.StandardGamepadButtonLeftBottom, sio.BtnDown},
	{eb.StandardGamepadButtonLeftLeft, sio.BtnLeft}, {eb.StandardGamepadButtonLeftRight, sio.BtnRight},
	{eb.StandardGamepadButtonRightBottom, sio.BtnCross}, {eb.StandardGamepadButtonRightRight, sio.BtnCircle},
	{eb.StandardGamepadButtonRightLeft, sio.BtnSquare}, {eb.StandardGamepadButtonRightTop, sio.BtnTriangle},
	{eb.StandardGamepadButtonFrontTopLeft, sio.BtnL1}, {eb.StandardGamepadButtonFrontTopRight, sio.BtnR1},
	{eb.StandardGamepadButtonFrontBottomLeft, sio.BtnL2}, {eb.StandardGamepadButtonFrontBottomRight, sio.BtnR2},
	{eb.StandardGamepadButtonCenterRight, sio.BtnStart}, {eb.StandardGamepadButtonCenterLeft, sio.BtnSelect},
}

// buttons 回傳鍵盤與所有標準手把按住的鍵（聯集）。key、pad 由呼叫端提供，方便測試。
func buttons(key func(eb.Key) bool, pads []eb.GamepadID, pad func(eb.GamepadID, eb.StandardGamepadButton) bool) uint16 {
	var b uint16
	for _, m := range keyMap {
		if key(m.key) {
			b |= m.btn
		}
	}
	for _, id := range pads {
		for _, m := range padMap {
			if pad(id, m.btn) {
				b |= m.ps
			}
		}
	}
	return b
}
