package ebiten

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"
)

// 僅測試注入裝置快照與實際 GUI，不能代替實體 USB／藍芽連線驗收。
type controllerCaptureGame struct {
	t      *testing.T
	out    string
	g      *Game
	scene  int
	saved  bool
	canvas *eb.Image
}

func (c *controllerCaptureGame) Update() error {
	if c.saved {
		c.scene++
		c.saved = false
	}
	if c.scene >= 5 {
		return eb.Termination
	}
	return nil
}

func (c *controllerCaptureGame) Layout(w, h int) (int, int) { return 960, 720 }

func (c *controllerCaptureGame) Draw(screen *eb.Image) {
	if c.saved {
		return
	}
	if c.canvas == nil {
		c.canvas = eb.NewImage(960, 720)
	}
	c.canvas.Fill(color.RGBA{16, 18, 25, 255})
	g := c.g
	g.controller = DefaultGamepadSettings()
	g.padUI = controllerPanel{capture: -1}
	g.pads = nil
	name := "hidden"
	if c.scene == 1 {
		g.padUI.visible = true
		name = "toolbar"
	}
	if c.scene >= 2 {
		g.padUI.visible = true
		g.padUI.open = true
		g.padUI.draft = g.controller.Clone()
		p := testPad()
		p.Name = "Xbox Wireless Controller"
		p.Buttons[eb.StandardGamepadButtonRightBottom] = true
		p.LeftX = 0.25
		p.RawButtons = []bool{false, false, true}
		p.Axes = []float64{0.25, -0.75, 0, 0, -1, 0}
		g.pads = []padSnapshot{p}
		g.padUI.draft.Device = p.Device
		name = "standard"
		if c.scene == 3 {
			g.pads[0].Standard = false
			g.pads[0].Name = strings.Repeat("繁體中文超長手把名稱", 8)
			g.padUI.capture = 4
			g.padUI.status = "請按下要使用的按鈕，或推動搖桿"
			name = "raw-long-name"
		}
		if c.scene == 4 {
			g.pads = nil
			g.padUI.status = "手把已離線，請重新連線"
			name = "disconnected"
		}
	}
	g.drawControllerUI(c.canvas)
	if c.scene == 2 || c.scene == 3 {
		// 直接檢查兩行真的有字，避免行高不符而整行跳過仍通過邏輯測試。
		pix := make([]byte, 960*720*4)
		c.canvas.ReadPixels(pix)
		for _, r := range []image.Rectangle{image.Rect(182, 504, 778, 521), image.Rect(182, 521, 778, 538)} {
			ink := false
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					i := (y*960 + x) * 4
					ink = ink || pix[i] != 32 || pix[i+1] != 38 || pix[i+2] != 49
				}
			}
			if !ink {
				c.t.Errorf("missing controller diagnostic text in %v", r)
			}
		}
	}
	if err := savePNG(c.canvas, filepath.Join(c.out, "gamepad-"+name+".png")); err != nil {
		c.t.Error(err)
	}
	if c.scene == 0 {
		pix := make([]byte, 960*720*4)
		c.canvas.ReadPixels(pix)
		if pix[0] != 16 || pix[1] != 18 || pix[2] != 25 {
			c.t.Error("hidden toolbar changed viewport")
		}
	}
	if c.scene == 2 {
		// 以真實字圖集測試長中文文字的墨跡沒有跨出安全區。
		test := eb.NewImage(320, 80)
		test.Fill(color.Black)
		r := image.Rect(32, 20, 200, 52)
		g.uiFont.label(test, r, strings.Repeat("繁體中文手把名稱", 20), color.White, false, 1)
		pix := make([]byte, 320*80*4)
		test.ReadPixels(pix)
		for y := 0; y < 80; y++ {
			for x := 0; x < 320; x++ {
				i := (y*320 + x) * 4
				if (pix[i] != 0 || pix[i+1] != 0 || pix[i+2] != 0) && !image.Pt(x, y).In(r) {
					c.t.Errorf("text ink escaped at %d,%d", x, y)
					c.saved = true
					return
				}
			}
		}
	}
	screen.DrawImage(c.canvas, nil)
	c.saved = true
}

func TestControllerUICaptures(t *testing.T) {
	out := os.Getenv("PSGOLEM_CONTROLLER_EVIDENCE")
	if out == "" {
		t.Skip("需要 PSGOLEM_CONTROLLER_EVIDENCE；只輸出自製 UI fixture")
	}
	eb.SetWindowSize(960, 720)
	c := &controllerCaptureGame{t: t, out: out, g: &Game{}}
	if err := eb.RunGame(c); err != nil {
		t.Fatal(err)
	}
}
