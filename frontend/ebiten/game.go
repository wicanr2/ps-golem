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
	"github.com/wicanr2/ps-golem/render/hdgpu"
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
	// 明確輸出尺寸優先於Scale；零值沿用既有起始視窗（012 §11）。
	WindowWidth, WindowHeight int
	Title                     string
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
	Filter      string
	Widescreen  bool
	Gamepad     *GamepadSettings
	SaveGamepad func(GamepadSettings) error
	Posters     *PosterPack
	Poster      PosterSettings
	SavePoster  func(PosterSettings) error
}

// Game 實作 ebiten.Game。
type Game struct {
	closing  bool
	hasDrawn bool
	cfg      Config
	sess     *Session
	m        *machine.Machine
	emu      *emulator
	// 設定畫面（012 §9）：m 為 nil 時顯示。
	biosPath, cuePath, setupErr string
	text                        *eb.Image
	card                        cardSaver
	frame                       *eb.Image
	frameNo                     uint64
	shown                       image.Point // 上次畫到 screen 時的 screen 大小；零值代表要重畫
	canvas                      *eb.Image
	updates                     int
	finishing                   bool
	finishDrawn                 bool
	err                         error
	stream                      *pcmStream
	xbr                         *eb.Shader
	controller                  GamepadSettings
	waitInputRelease            bool
	pads                        []padSnapshot
	padUI                       controllerPanel
	uiFont                      *bitmapUIFont
	viewSize                    image.Point
	poster                      PosterSettings
	posterSafeZoom              bool
	posterImages                [][2]*eb.Image
	posterShader                *eb.Shader
	posterError                 string
}

// Run 開視窗並執行到關閉。
func Run(cfg Config) error {
	if cfg.Scale <= 0 {
		cfg.Scale = 3
	}
	g := &Game{cfg: cfg, biosPath: cfg.BIOSPath, cuePath: cfg.CuePath, poster: cfg.Poster}
	if err := g.poster.Validate(); err != nil {
		return err
	}
	if cfg.Posters != nil {
		var err error
		g.posterShader, err = eb.NewShader([]byte(posterBlendSource))
		if err != nil {
			return fmt.Errorf("海報融合：%w", err)
		}
		defer func() {
			g.posterShader.Deallocate()
			for _, pair := range g.posterImages {
				for _, im := range pair {
					im.Deallocate()
				}
			}
		}()
	}
	g.controller = DefaultGamepadSettings()
	if cfg.Gamepad != nil {
		if err := cfg.Gamepad.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：手把設定：", err)
		} else {
			g.controller = cfg.Gamepad.Clone()
		}
	}
	g.padUI.capture = -1
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
	if cfg.WindowWidth > 0 && cfg.WindowHeight > 0 {
		eb.SetWindowSize(cfg.WindowWidth, cfg.WindowHeight)
	} else if cfg.Widescreen {
		eb.SetWindowSize(1280*cfg.Scale/3, 720*cfg.Scale/3)
	} else {
		eb.SetWindowSize(320*cfg.Scale, 240*cfg.Scale)
	}
	eb.SetWindowClosingHandled(true)
	eb.SetWindowResizingMode(eb.WindowResizingModeEnabled)
	eb.SetTPS(60)
	// 畫面沒有新內容時 Draw 直接返回，保留上一次的畫面（012 §4）。
	eb.SetScreenClearedEveryFrame(false)
	g.tryOpen()
	err := eb.RunGame(g)
	hdgpu.Cancel()
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
	if cfg.Stats && g.m != nil && g.emu != nil {
		fmt.Fprintf(os.Stderr, "stats final_field=%d final_frame=%d shown_frame=%d final_drawn=%t\n", g.m.Field(), g.emu.frameNumber(), g.frameNo, g.finishDrawn)
	}
	if err == nil {
		err = g.err
	}
	return err
}

// Update 讀輸入交給模擬 goroutine；結束條件成立時多等一次 Draw 再存截圖並結束。
func (g *Game) Update() error {
	gpuWait := g.emu != nil && g.hasDrawn && !g.emu.parked.Load()
	if g.emu != nil {
		select {
		case <-g.emu.done:
			gpuWait = false
		default:
		}
	}
	hdgpu.Service(gpuWait)
	if eb.IsWindowBeingClosed() {
		g.closing = true
	}
	if g.closing {
		if g.emu != nil {
			g.emu.requestStop()
			select {
			case <-g.emu.done:
			default:
				return nil
			}
		}
		return eb.Termination
	}
	if g.finishing {
		if g.emu != nil {
			g.emu.requestStop()
			select {
			case <-g.emu.done:
			default:
				return nil
			}
		}
		if !g.finishDrawn {
			return nil
		}
		if g.cfg.Screenshot != "" && g.canvas != nil {
			if err := savePNG(g.canvas, g.cfg.Screenshot); err != nil {
				g.err = err
			}
		}
		return eb.Termination
	}
	g.updateControllerUI()
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
	if g.padUI.open {
		g.emu.buttons.Store(0)
	} else {
		g.emu.buttons.Store(uint32(g.filterGameplayInput(buttons(eb.IsKeyPressed, nil, nil) | controllerButtons(g.controller, g.pads))))
	}
	if g.hasDrawn && (g.stream == nil || playbackReady()) {
		g.emu.start()
	}
	return nil
}

// Draw 把目前顯示的畫面依 4:3 縮放到視窗中央。
func (g *Game) Draw(screen *eb.Image) {
	defer func() { g.hasDrawn = true }()
	g.viewSize = screen.Bounds().Size()
	if g.padUI.visible || g.padUI.open {
		g.shown = image.Point{}
	}
	g.drawGame(screen)
	g.drawControllerUI(screen)
	if g.cfg.Screenshot != "" && g.finishing {
		g.keepCanvas(screen)
	}
	if g.finishing {
		g.finishDrawn = true
	}
}

func (g *Game) drawGame(screen *eb.Image) {
	if g.m == nil {
		screen.Clear()
		g.drawSetup(screen)
		return
	}
	sb := screen.Bounds()
	var b image.Rectangle
	changed := false
	g.emu.useFrame(func(img *image.RGBA, no uint64) {
		if img == nil {
			return
		}
		b = img.Bounds()
		if g.frame == nil || g.frame.Bounds().Size() != b.Size() {
			g.frame = eb.NewImage(b.Dx(), b.Dy())
			g.frameNo = 0
		}
		if no == g.frameNo && sb.Size() == g.shown {
			return
		}
		if no != g.frameNo {
			if g.cfg.Posters != nil {
				g.posterSafeZoom = posterZoomSafe(img)
			}
			g.frame.WritePixels(img.Pix)
			g.frameNo = no
		}
		changed = true
	})
	if !changed {
		return
	}
	g.shown = sb.Size()
	screen.Clear()
	r := fit(sb.Dx(), sb.Dy())
	target := screen
	if g.cfg.Widescreen {
		r = fitWide(sb.Dx(), sb.Dy())
	} else if index := g.cfg.Posters.index(g.poster.ID); index >= 0 {
		r, target = g.drawPosters(screen, index)
	}
	op := &eb.DrawImageOptions{}
	op.GeoM.Scale(float64(r.Dx())/float64(b.Dx()), float64(r.Dy())/float64(b.Dy()))
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	op.Filter = eb.FilterNearest
	if g.xbr != nil {
		shaderOp := &eb.DrawRectShaderOptions{}
		shaderOp.GeoM = op.GeoM
		shaderOp.Images[0] = g.frame
		target.DrawRectShader(b.Dx(), b.Dy(), g.xbr, shaderOp)
	} else {
		target.DrawImage(g.frame, op)
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
