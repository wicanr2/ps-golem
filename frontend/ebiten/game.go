// Package ebiten 是 ps-golem 的視窗前端（docs/spec/012-frontend-ebiten.md）。
// 只讀畫面、寫手把按鍵、存記憶卡，機器的推進方式不變。
package ebiten

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	eb "github.com/hajimehoshi/ebiten/v2"

	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
)

// fieldCycleLimit 是一次 Update 最多跑的週期數（約 2 個 NTSC field），顯示關閉時也不會卡住（012 §3）。
const fieldCycleLimit = 1_200_000

// Config 是前端設定。
type Config struct {
	// Open 依 BIOS 與 cue 開機（012 §9）。BIOSPath、CuePath 是起始路徑，可以是空的。
	Open     OpenFunc
	BIOSPath string
	CuePath  string
	Scale    int
	Title    string
	// Frames 大於 0 時跑完這麼多次 Update 就結束，並把畫面存到 Screenshot（驗收用）。
	Frames     int
	Screenshot string
	// Mute 為 true 時不開音訊裝置（沒有音效卡的環境，例如容器內驗收）。
	Mute bool
	// Press 不為 nil 時用按鍵腳本取代鍵盤與手把（驗收用，012 §5）。
	Press []padscript.Press
	// UntilField 大於 0 時，機器跑到第這麼多次 VBlank 就存記憶卡、截圖並結束（驗收用）。
	UntilField uint64
	// Stats 為 true 時每 600 個 field 在標準錯誤印出實際 FPS 與每個 field 的模擬耗時。
	Stats bool
	// Filter 是 none 或 xbr，只影響呈現。
	Filter string
}

// Game 實作 ebiten.Game。
type Game struct {
	cfg  Config
	sess *Session
	m    *machine.Machine
	emu  *emulator
	// 設定畫面（012 §9）：m 為 nil 時顯示。
	biosPath, cuePath, setupErr string
	text                        *eb.Image
	card                        cardSaver
	frame                       *eb.Image
	frameNo                     uint64
	canvas                      *eb.Image
	updates                     int
	finishing                   bool
	err                         error
	stream                      *pcmStream
	xbr                         *eb.Shader
}

// Run 開視窗並執行到關閉。
func Run(cfg Config) error {
	if cfg.Scale <= 0 {
		cfg.Scale = 3
	}
	g := &Game{cfg: cfg, biosPath: cfg.BIOSPath, cuePath: cfg.CuePath}
	if cfg.Filter != "" && cfg.Filter != "none" && cfg.Filter != "xbr" {
		return fmt.Errorf("未知濾鏡 %q", cfg.Filter)
	}
	if cfg.Filter == "xbr" {
		var err error
		g.xbr, err = eb.NewShader(xbrSource)
		if err != nil {
			return fmt.Errorf("xBR shader：%w", err)
		}
		defer g.xbr.Deallocate()
	}
	if !cfg.Mute {
		g.stream = &pcmStream{}
		// 開不起音訊裝置時照常跑、不出聲：雙擊開啟的玩家看不到錯誤訊息，中止等於「點了沒反應」。
		if _, err := newAudio(g.stream); err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：音訊裝置開不起來，改為靜音：", err)
			g.stream = nil
		}
	}
	eb.SetWindowTitle(cfg.Title)
	eb.SetWindowSize(320*cfg.Scale, 240*cfg.Scale)
	eb.SetWindowResizingMode(eb.WindowResizingModeEnabled)
	eb.SetTPS(60)
	g.tryOpen()
	err := eb.RunGame(g)
	if g.emu != nil {
		g.emu.halt()
		if g.emu.err != nil && err == nil {
			err = g.emu.err
		}
	}
	if g.m != nil {
		if ferr := g.card.flush(g.m.Card1); ferr != nil && err == nil {
			err = ferr
		}
	}
	if g.sess != nil && g.sess.Close != nil {
		g.sess.Close()
	}
	if errors.Is(err, eb.Termination) {
		err = nil
	}
	if err == nil {
		err = g.err
	}
	return err
}

// Update 讀輸入交給模擬 goroutine；結束條件成立時多等一次 Draw 再存截圖並結束。
func (g *Game) Update() error {
	if g.finishing {
		if g.cfg.Screenshot != "" && g.canvas != nil {
			if err := savePNG(g.canvas, g.cfg.Screenshot); err != nil {
				g.err = err
			}
		}
		return eb.Termination
	}
	g.updates++
	if g.cfg.Frames > 0 && g.updates >= g.cfg.Frames {
		g.finishing = true
	}
	if g.emu == nil {
		if g.takeDropped() {
			g.tryOpen()
		}
		return nil
	}
	select {
	case <-g.emu.done:
		// 跑到 -fields 或記憶卡存檔失敗；錯誤在 Run 收尾時回報。
		g.finishing = true
	default:
	}
	g.emu.buttons.Store(uint32(buttons(eb.IsKeyPressed, eb.AppendGamepadIDs(nil), eb.IsStandardGamepadButtonPressed)))
	return nil
}

// Draw 把目前顯示的畫面依 4:3 縮放到視窗中央。
func (g *Game) Draw(screen *eb.Image) {
	if g.m == nil {
		g.drawSetup(screen)
		g.keepCanvas(screen)
		return
	}
	img, no := g.emu.frame()
	if img == nil {
		return
	}
	b := img.Bounds()
	if g.frame == nil || g.frame.Bounds().Size() != b.Size() {
		g.frame = eb.NewImage(b.Dx(), b.Dy())
		g.frameNo = 0
	}
	if no != g.frameNo {
		g.frame.WritePixels(img.Pix)
		g.frameNo = no
	}
	sb := screen.Bounds()
	r := fit(sb.Dx(), sb.Dy())
	op := &eb.DrawImageOptions{}
	op.GeoM.Scale(float64(r.Dx())/float64(b.Dx()), float64(r.Dy())/float64(b.Dy()))
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	op.Filter = eb.FilterNearest
	if g.xbr != nil {
		shaderOp := &eb.DrawRectShaderOptions{}
		shaderOp.GeoM = op.GeoM
		shaderOp.Images[0] = g.frame
		screen.DrawRectShader(b.Dx(), b.Dy(), g.xbr, shaderOp)
	} else {
		screen.DrawImage(g.frame, op)
	}
	if g.cfg.Screenshot != "" {
		g.keepCanvas(screen)
	}
}

// keepCanvas 把設定畫面也留一份給 -screenshot。
func (g *Game) keepCanvas(screen *eb.Image) {
	sb := screen.Bounds()
	if g.canvas == nil || g.canvas.Bounds().Size() != sb.Size() {
		g.canvas = eb.NewImage(sb.Dx(), sb.Dy())
	}
	g.canvas.Clear()
	g.canvas.DrawImage(screen, nil)
}

// Layout 用視窗的實際大小當畫布大小，縮放由 Draw 處理。
func (g *Game) Layout(w, h int) (int, int) { return w, h }

func savePNG(src *eb.Image, path string) error {
	b := src.Bounds()
	out := image.NewRGBA(b)
	src.ReadPixels(out.Pix)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}
