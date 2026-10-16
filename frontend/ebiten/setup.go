package ebiten

import (
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/wicanr2/ps-golem/machine"
)

// biosSize 是 BIOS 映像的大小；拖進來的檔案用它認 BIOS（012 §9）。
const biosSize = 512 * 1024

// Session 是開好的一台機器與它的記憶卡位置。
type Session struct {
	Machine  *machine.Machine
	CardPath string
	Title    string
	Close    func()
}

// OpenFunc 依 BIOS 與 cue 的路徑開機；失敗時回傳錯誤，前端改顯示設定畫面。
type OpenFunc func(biosPath, cuePath string) (*Session, error)

// dropKind 依副檔名與大小判斷拖進的檔案是什麼（012 §9）。
func dropKind(name string, size int64) string {
	switch {
	case strings.EqualFold(filepath.Ext(name), ".cue"):
		return "cue"
	case size == biosSize:
		return "bios"
	}
	return ""
}

// takeDropped 讀這一次 Update 拖進的檔案，更新 BIOS 與 cue 的路徑；回傳是否有變動。
func (g *Game) takeDropped() bool {
	fsys := eb.DroppedFiles()
	if fsys == nil {
		return false
	}
	ents, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return false
	}
	changed := false
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		f, err := fsys.Open(e.Name())
		if err != nil {
			continue
		}
		osf, ok := f.(*os.File)
		info, serr := f.Stat()
		f.Close()
		if !ok || serr != nil {
			continue
		}
		switch dropKind(e.Name(), info.Size()) {
		case "cue":
			g.cuePath, changed = osf.Name(), true
		case "bios":
			g.biosPath, changed = osf.Name(), true
		}
	}
	return changed
}

// tryOpen 兩個路徑都有時開機；失敗把錯誤留給設定畫面顯示。
func (g *Game) tryOpen() {
	if g.biosPath == "" || g.cuePath == "" {
		return
	}
	s, err := g.cfg.Open(g.biosPath, g.cuePath)
	if err != nil {
		g.setupErr = err.Error()
		return
	}
	g.setupErr = ""
	g.start(s)
}

func (g *Game) start(s *Session) {
	g.sess = s
	g.m = s.Machine
	g.card = cardSaver{path: s.CardPath}
	if s.Title != "" {
		eb.SetWindowTitle(s.Title)
	}
}

// drawSetup 畫設定畫面。文字只用 ASCII：內建除錯字型沒有中文字。
func (g *Game) drawSetup(screen *eb.Image) {
	show := func(p string) string {
		if p == "" {
			return "(not set)"
		}
		return wrap(asciiPath(p), 44)
	}
	msg := fmt.Sprintf("ps-golem\n\n"+
		"Drag and drop onto this window:\n"+
		"  - your PlayStation BIOS file (512 KB)\n"+
		"  - the disc .cue file\n\n"+
		"BIOS: %s\nDisc: %s\n", show(g.biosPath), show(g.cuePath))
	if g.setupErr != "" {
		msg += "\nError:\n" + wrap(asciiPath(g.setupErr), 50) + "\n"
	}
	// 字放在 320×240 的小畫布上，再依 4:3 規則放大到視窗，跟遊戲畫面一樣大。
	if g.text == nil {
		g.text = eb.NewImage(320, 240)
	}
	g.text.Fill(color.Black)
	ebitenutil.DebugPrintAt(g.text, msg, 8, 8)
	screen.Fill(color.Black)
	r := fit(screen.Bounds().Dx(), screen.Bounds().Dy())
	op := &eb.DrawImageOptions{}
	op.GeoM.Scale(float64(r.Dx())/320, float64(r.Dy())/240)
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	op.Filter = eb.FilterNearest
	screen.DrawImage(g.text, op)
}

// asciiPath 把非 ASCII 字元換成 '?'，內建字型畫不出來。
func asciiPath(s string) string {
	b := []byte{}
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			r = '?'
		}
		b = append(b, byte(r))
	}
	return string(b)
}

// wrap 每 n 個字元換行，避免長路徑超出 320 像素寬的畫布。
func wrap(s string, n int) string {
	var b strings.Builder
	for len(s) > n {
		b.WriteString(s[:n])
		b.WriteString("\n      ")
		s = s[n:]
	}
	b.WriteString(s)
	return b.String()
}
