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

	"github.com/wicanr2/ps-golem/machine"
)

// fieldCycleLimit 是一次 Update 最多跑的週期數（約 2 個 NTSC field），顯示關閉時也不會卡住（012 §3）。
const fieldCycleLimit = 1_200_000

// Config 是前端設定。
type Config struct {
	// Open 依 BIOS 與 cue 開機（012 §10）。BIOSPath、CuePath 是起始路徑，可以是空的。
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
}

// Game 實作 ebiten.Game。
type Game struct {
	cfg  Config
	sess *Session
	m    *machine.Machine
	// 設定畫面（012 §10）：m 為 nil 時顯示。
	biosPath, cuePath, setupErr string
	text                        *eb.Image
	card                        cardSaver
	frame                       *eb.Image
	canvas                      *eb.Image
	updates                     int
	err                         error
	stream                      *pcmStream
	pcm                         []int16
}

// Run 開視窗並執行到關閉。
func Run(cfg Config) error {
	if cfg.Scale <= 0 {
		cfg.Scale = 3
	}
	g := &Game{cfg: cfg, biosPath: cfg.BIOSPath, cuePath: cfg.CuePath}
	if !cfg.Mute {
		g.stream = &pcmStream{}
		if _, err := newAudio(g.stream); err != nil {
			return fmt.Errorf("音訊：%w", err)
		}
	}
	eb.SetWindowTitle(cfg.Title)
	eb.SetWindowSize(320*cfg.Scale, 240*cfg.Scale)
	eb.SetWindowResizingMode(eb.WindowResizingModeEnabled)
	eb.SetTPS(60)
	g.tryOpen()
	err := eb.RunGame(g)
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

// Update 讀輸入、跑到下一次 VBlank、必要時存記憶卡。
func (g *Game) Update() error {
	if g.cfg.Frames > 0 && g.updates >= g.cfg.Frames {
		if g.cfg.Screenshot != "" && g.canvas != nil {
			if err := savePNG(g.canvas, g.cfg.Screenshot); err != nil {
				g.err = err
			}
		}
		return eb.Termination
	}
	g.updates++
	if g.m == nil {
		if g.takeDropped() {
			g.tryOpen()
		}
		return nil
	}
	g.m.Pad1.Buttons = buttons(eb.IsKeyPressed, eb.AppendGamepadIDs(nil), eb.IsStandardGamepadButtonPressed)
	g.m.RunUntilField(g.m.Field()+1, fieldCycleLimit)
	g.pcm = g.m.TakeAudio(g.pcm[:0])
	if g.stream != nil {
		g.stream.push(g.pcm)
	}
	if _, err := g.card.tick(g.m.Card1, g.m.Field()); err != nil {
		return fmt.Errorf("記憶卡存檔：%w", err)
	}
	return nil
}

// Draw 把目前顯示的畫面依 4:3 縮放到視窗中央。
func (g *Game) Draw(screen *eb.Image) {
	if g.m == nil {
		g.drawSetup(screen)
		g.keepCanvas(screen)
		return
	}
	img := g.m.GPU.Screen()
	b := img.Bounds()
	if g.frame == nil || g.frame.Bounds().Size() != b.Size() {
		g.frame = eb.NewImage(b.Dx(), b.Dy())
	}
	g.frame.WritePixels(img.Pix)
	sb := screen.Bounds()
	if g.canvas == nil || g.canvas.Bounds().Size() != sb.Size() {
		g.canvas = eb.NewImage(sb.Dx(), sb.Dy())
	}
	g.canvas.Clear()
	r := fit(sb.Dx(), sb.Dy())
	op := &eb.DrawImageOptions{}
	op.GeoM.Scale(float64(r.Dx())/float64(b.Dx()), float64(r.Dy())/float64(b.Dy()))
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	op.Filter = eb.FilterNearest
	g.canvas.DrawImage(g.frame, op)
	screen.DrawImage(g.canvas, nil)
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
