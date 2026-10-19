package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// 020-gamepad-toolbar。所有 UI 幾何都從 644×588 的定版面板座標推出。
const toolbarHeight = 32

type controllerPanel struct {
	pinned, visible, open bool
	draft                 GamepadSettings
	capture               int
	before                padSnapshot
	status                string
}

var (
	uiWhite  = color.RGBA{243, 246, 250, 255}
	uiMuted  = color.RGBA{174, 187, 208, 255}
	uiPanel  = color.RGBA{32, 38, 49, 255}
	uiButton = color.RGBA{52, 65, 85, 255}
	uiAccent = color.RGBA{71, 123, 187, 255}
)

func panelGeometry(w, h int) (x, y, scale float64) {
	scale = math.Min(1, math.Min(float64(max(1, w-24))/644, float64(max(1, h-48))/588))
	x = (float64(w) - 644*scale) / 2
	y = (float64(h) - 588*scale) / 2
	if y < toolbarHeight {
		y = toolbarHeight
	}
	return
}

func panelRow(i int) image.Rectangle {
	col, row := i/7, i%7
	x, y := 24+col*302, 162+row*33
	return image.Rect(x+76, y, x+278, y+28)
}

func (g *Game) openControllerPanel() {
	g.padUI.open = true
	g.padUI.draft = g.controller.Clone()
	g.padUI.capture = -1
	g.padUI.status = ""
	if g.emu != nil {
		g.emu.buttons.Store(0)
		g.emu.paused.Store(true)
	}
	g.shown = image.Point{}
}

func (g *Game) closeControllerPanel() {
	g.waitInputRelease = true
	g.padUI.open = false
	g.padUI.capture = -1
	if g.emu != nil {
		g.emu.buttons.Store(0)
		g.emu.paused.Store(false)
	}
	g.shown = image.Point{}
}

func toolbarShown(pinned, open bool, cursorX, cursorY, width int) bool {
	return pinned || open || cursorX >= 0 && cursorX < width && cursorY >= 0 && cursorY < toolbarHeight
}

func (g *Game) updateControllerUI() {
	g.pads = connectedPads()
	x, y := eb.CursorPosition()
	w, h := g.viewSize.X, g.viewSize.Y
	if w == 0 {
		w, h = 960, 720
	}
	oldVisible := g.padUI.visible
	if inpututil.IsKeyJustPressed(eb.KeyF1) {
		g.padUI.pinned = !g.padUI.pinned
	}
	g.padUI.visible = toolbarShown(g.padUI.pinned, g.padUI.open, x, y, w)
	if oldVisible != g.padUI.visible {
		g.shown = image.Point{}
	}
	clicked := inpututil.IsMouseButtonJustPressed(eb.MouseButtonLeft)
	if !g.padUI.open {
		if clicked && g.padUI.visible && image.Pt(x, y).In(image.Rect(116, 2, 222, 30)) {
			g.openControllerPanel()
		}
		return
	}
	if inpututil.IsKeyJustPressed(eb.KeyEscape) {
		g.closeControllerPanel()
		return
	}
	px, py, scale := panelGeometry(w, h)
	local := image.Pt(int(math.Floor((float64(x)-px)/scale)), int(math.Floor((float64(y)-py)/scale)))
	if clicked {
		switch {
		case local.In(image.Rect(544, 82, 574, 120)):
			g.cycleController(-1)
		case local.In(image.Rect(574, 82, 620, 120)):
			g.cycleController(1)
		case local.In(image.Rect(24, 526, 140, 564)):
			g.padUI.draft.DeadZone = 0.2
			if p, ok := findPad(g.pads, g.padUI.draft.Device); ok {
				delete(g.padUI.draft.Profiles, p.Model)
				g.padUI.status = "已恢復目前手把的預設配置"
			}
			g.padUI.capture = -1
		case local.In(image.Rect(422, 526, 510, 564)):
			g.closeControllerPanel()
			return
		case local.In(image.Rect(522, 526, 620, 564)):
			g.saveControllerPanel()
			return
		default:
			for i := range actionNames {
				if !local.In(panelRow(i)) {
					continue
				}
				p, ok := findPad(g.pads, g.padUI.draft.Device)
				if !ok {
					g.padUI.status = "請先選擇單一已連線手把"
					break
				}
				g.padUI.capture = i
				g.padUI.before = p
				g.padUI.status = "按手把按鍵或推動搖桿；Delete 可清除指派"
				break
			}
		}
	}
	if eb.IsMouseButtonPressed(eb.MouseButtonLeft) && local.In(image.Rect(160, 400, 474, 438)) {
		t := math.Max(0, math.Min(1, float64(local.X-170)/292))
		g.padUI.draft.DeadZone = math.Round((0.05+t*0.85)*20) / 20
	}
	if g.padUI.capture >= 0 {
		if inpututil.IsKeyJustPressed(eb.KeyDelete) || inpututil.IsKeyJustPressed(eb.KeyBackspace) {
			g.assignControllerBinding(GamepadBinding{Kind: "none"})
			return
		}
		p, ok := findPad(g.pads, g.padUI.draft.Device)
		if !ok {
			g.padUI.capture = -1
			g.padUI.status = "手把已離線，請重新連線"
			return
		}
		if binding, ok := freshBinding(g.padUI.before, p); ok {
			g.assignControllerBinding(binding)
		}
		g.padUI.before.Buttons = p.Buttons
		g.padUI.before.RawButtons = p.RawButtons
	}
}

// 同一條指派路徑處理新綁定與清除，保留衝突及草稿隔離。
func (g *Game) assignControllerBinding(binding GamepadBinding) {
	i := g.padUI.capture
	p, ok := findPad(g.pads, g.padUI.draft.Device)
	if !ok || i < 0 || i >= len(actionNames) {
		return
	}
	profile := g.padUI.draft.profile(p)
	profile.Bindings = append([]GamepadBinding(nil), profile.Bindings...)
	for n, b := range profile.Bindings {
		if n != i && b == binding && b.Kind != "none" {
			g.padUI.status = "此按鍵已用於 " + actionNames[n]
			return
		}
	}
	profile.Bindings[i] = binding
	g.padUI.draft.Profiles[p.Model] = profile
	g.padUI.status = "已指派 " + actionNames[i] + "：" + bindingName(binding)
	if binding.Kind == "none" {
		g.padUI.status = "已清除 " + actionNames[i] + " 的指派"
	}
	g.padUI.capture = -1
}

func rawControllerInput(p padSnapshot) string {
	parts := []string{}
	for i, down := range p.RawButtons {
		if down {
			parts = append(parts, fmt.Sprintf("B%d", i))
		}
	}
	for i, v := range p.Axes {
		parts = append(parts, fmt.Sprintf("A%d:%+.2f", i, v))
	}
	if len(parts) == 0 {
		return "原始輸入：無按鍵或軸"
	}
	return "原始輸入：" + strings.Join(parts, " ")
}

func (g *Game) cycleController(direction int) {
	choices := []string{"auto", "keyboard"}
	for _, p := range g.pads {
		choices = append(choices, p.Device)
	}
	i := 0
	for n, c := range choices {
		if c == g.padUI.draft.Device {
			i = n
			break
		}
	}
	i = (i + direction + len(choices)) % len(choices)
	g.padUI.draft.Device = choices[i]
	g.padUI.capture = -1
	g.padUI.status = ""
}

func (g *Game) saveControllerPanel() {
	if err := g.padUI.draft.Validate(); err != nil {
		g.padUI.status = err.Error()
		return
	}
	next := g.padUI.draft.Clone()
	if g.cfg.SaveGamepad != nil {
		if err := g.cfg.SaveGamepad(next.Clone()); err != nil {
			g.padUI.status = "儲存失敗：" + err.Error()
			return
		}
	}
	g.controller = next
	g.closeControllerPanel()
}

func roundedUI(dst *eb.Image, r image.Rectangle, radius float64, c color.Color) {
	if r.Empty() {
		return
	}
	x, y, w, h := float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy())
	rr := float32(math.Min(radius, math.Min(float64(w), float64(h))/2))
	vector.DrawFilledRect(dst, x+rr, y, w-2*rr, h, c, true)
	vector.DrawFilledRect(dst, x, y+rr, w, h-2*rr, c, true)
	for _, cx := range []float32{x + rr, x + w - rr} {
		for _, cy := range []float32{y + rr, y + h - rr} {
			vector.DrawFilledCircle(dst, cx, cy, rr, c, true)
		}
	}
}

func (g *Game) drawControllerUI(screen *eb.Image) {
	if !g.padUI.visible && !g.padUI.open {
		return
	}
	if g.uiFont == nil {
		g.uiFont = loadUIFont()
	}
	font := g.uiFont
	if g.padUI.visible || g.padUI.open {
		vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), toolbarHeight, color.RGBA{37, 42, 53, 255}, false)
		font.label(screen, image.Rect(14, 2, 108, 30), "ps-golem", uiWhite, false, 1)
		roundedUI(screen, image.Rect(116, 2, 222, 30), 4, color.RGBA{62, 72, 89, 255})
		font.label(screen, image.Rect(124, 2, 214, 30), "手把設定", uiWhite, true, 1)
		if screen.Bounds().Dx() >= 500 {
			font.label(screen, image.Rect(screen.Bounds().Dx()-170, 2, screen.Bounds().Dx()-12, 30), "F1 設定工具列", uiMuted, true, 1)
		}
	}
	if !g.padUI.open {
		return
	}
	x, y, scale := panelGeometry(screen.Bounds().Dx(), screen.Bounds().Dy())
	tr := func(r image.Rectangle) image.Rectangle {
		return image.Rect(int(math.Round(x+float64(r.Min.X)*scale)), int(math.Round(y+float64(r.Min.Y)*scale)), int(math.Round(x+float64(r.Max.X)*scale)), int(math.Round(y+float64(r.Max.Y)*scale)))
	}
	label := func(r image.Rectangle, text string, c color.Color, center bool, size float64) {
		if size > 1 {
			font.title.label(screen, tr(r), text, c, center, scale)
		} else {
			font.label(screen, tr(r), text, c, center, scale*size)
		}
	}
	inset := func(r image.Rectangle) image.Rectangle { r.Min.X += 8; r.Max.X -= 8; return r }
	button := func(r image.Rectangle, text string, c color.Color) {
		roundedUI(screen, tr(r), 5*scale, c)
		label(inset(r), text, uiWhite, true, 1)
	}
	// 單次重畫前先恢復遊戲背景，不讓半透明遮罩跨 Draw 累積。
	vector.DrawFilledRect(screen, 0, toolbarHeight, float32(screen.Bounds().Dx()), float32(max(0, screen.Bounds().Dy()-toolbarHeight)), color.RGBA{0, 0, 0, 120}, false)
	roundedUI(screen, tr(image.Rect(0, 0, 644, 588)), 12*scale, color.RGBA{88, 104, 126, 255})
	roundedUI(screen, tr(image.Rect(1, 1, 643, 587)), 11*scale, uiPanel)
	label(image.Rect(24, 12, 620, 52), "手把設定", uiWhite, false, 1.4)
	label(image.Rect(24, 50, 620, 82), "USB 與已配對的藍芽手把", uiMuted, false, 1)
	label(image.Rect(24, 82, 85, 120), "裝置", uiWhite, false, 1)
	name := "自動（全部手把）"
	if g.padUI.draft.Device == "keyboard" {
		name = "僅鍵盤"
	} else if g.padUI.draft.Device != "auto" {
		name = "選定手把未連線"
		if p, ok := findPad(g.pads, g.padUI.draft.Device); ok {
			name = p.Name
		}
	}
	roundedUI(screen, tr(image.Rect(91, 82, 620, 120)), 5*scale, color.RGBA{48, 59, 76, 255})
	label(image.Rect(105, 82, 536, 120), name, uiWhite, false, 1)
	label(image.Rect(544, 82, 574, 120), "◀", uiWhite, true, 1)
	label(image.Rect(574, 82, 620, 120), "▶", uiWhite, true, 1)
	label(image.Rect(24, 126, 620, 158), "點選按鍵後，按下手把上要使用的按鈕。", uiMuted, false, 0.875)
	p, selected := findPad(g.pads, g.padUI.draft.Device)
	if !selected && len(g.pads) > 0 && g.padUI.draft.Device == "auto" {
		p = g.pads[0]
	}
	profile := g.padUI.draft.profile(p)
	live := controllerButtons(g.padUI.draft, g.pads)
	for i, n := range actionNames {
		r := panelRow(i)
		label(image.Rect(r.Min.X-76, r.Min.Y, r.Min.X-4, r.Max.Y), n, uiWhite, false, 1)
		c := uiButton
		if live&padMap[i].ps != 0 {
			c = uiAccent
		}
		text := bindingName(profile.Bindings[i])
		if g.padUI.capture == i {
			c = color.RGBA{134, 96, 44, 255}
			text = "等待按鍵…"
		}
		roundedUI(screen, tr(r), 4*scale, c)
		label(inset(r), text, uiWhite, false, 0.875)
	}
	label(image.Rect(24, 400, 168, 438), fmt.Sprintf("搖桿死區  %d%%", int(math.Round(g.padUI.draft.DeadZone*100))), uiWhite, false, 1)
	line := tr(image.Rect(170, 417, 462, 423))
	roundedUI(screen, line, 3*scale, color.RGBA{82, 98, 122, 255})
	knob := 170 + (g.padUI.draft.DeadZone-0.05)/0.85*292
	fill := tr(image.Rect(170, 417, int(knob), 423))
	roundedUI(screen, fill, 3*scale, color.RGBA{109, 163, 237, 255})
	vector.DrawFilledCircle(screen, float32(x+knob*scale), float32(y+420*scale), float32(8*scale), color.RGBA{216, 232, 255, 255}, true)
	pressed := []string{}
	for i, m := range padMap {
		if live&m.ps != 0 {
			pressed = append(pressed, actionNames[i])
		}
	}
	text := "即時測試：無按鍵"
	if len(pressed) > 0 {
		text = "即時測試：" + strings.Join(pressed, " ")
	}
	if selected {
		label(image.Rect(24, 438, 620, 455), text, color.RGBA{184, 214, 255, 255}, false, 0.7)
		label(image.Rect(24, 455, 620, 472), rawControllerInput(p), uiMuted, false, 0.7)
	} else {
		label(image.Rect(24, 438, 620, 472), text, color.RGBA{184, 214, 255, 255}, false, 0.875)
	}
	status := g.padUI.status
	if status == "" {
		status = "藍芽配對由系統處理。設定時遊戲暫停。"
	}
	label(image.Rect(24, 474, 620, 510), status, uiMuted, false, 0.875)
	button(image.Rect(24, 526, 140, 564), "恢復預設", uiButton)
	button(image.Rect(422, 526, 510, 564), "取消", uiButton)
	button(image.Rect(522, 526, 620, 564), "儲存", uiAccent)
}

// 面板內按住的綁定按鍵必須先放開，恢復遊戲後才接受新的輸入。
func (g *Game) filterGameplayInput(b uint16) uint16 {
	if g.waitInputRelease {
		if b == 0 {
			g.waitInputRelease = false
		}
		return 0
	}
	return b
}
