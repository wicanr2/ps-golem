// psgolem 是玩家用的視窗執行檔（docs/spec/012-frontend-ebiten.md §2）。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/ps-golem/frontend/ebiten"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/machine"
)

func main() {
	biosPath := flag.String("bios", "", "BIOS 映像（512 KB）")
	cuePath := flag.String("cue", "", "光碟映像的 cue 檔")
	cardPath := flag.String("card", "", "1P 記憶卡檔案；省略時放在使用者設定目錄的 ps-golem/<開機檔名>.mcd")
	scale := flag.Int("scale", 3, "起始視窗倍率")
	frames := flag.Int("frames", 0, "跑這麼多次 Update 後結束（驗收用）")
	shot := flag.String("screenshot", "", "配合 -frames：結束前存下畫面")
	flag.Parse()
	if err := run(*biosPath, *cuePath, *cardPath, *scale, *frames, *shot); err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(1)
	}
}

func run(biosPath, cuePath, cardPath string, scale, frames int, shot string) error {
	if biosPath == "" || cuePath == "" {
		return fmt.Errorf("需要 -bios 與 -cue")
	}
	bios, err := os.ReadFile(biosPath)
	if err != nil {
		return err
	}
	m, err := machine.New(bios)
	if err != nil {
		return err
	}
	d, err := disc.Open(cuePath)
	if err != nil {
		return err
	}
	defer d.Close()
	if d.Fallback != "" {
		fmt.Fprintln(os.Stderr, "psgolem：", d.Fallback)
	}
	m.InsertDisc(d)
	if cardPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		name := d.BootFile()
		if name == "" {
			name = "card"
		}
		cardPath = filepath.Join(dir, "ps-golem", name+".mcd")
	}
	if b, err := os.ReadFile(cardPath); err == nil {
		if !m.Card1.Load(b) {
			return fmt.Errorf("記憶卡 %s 不是 128 KB", cardPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintln(os.Stderr, "psgolem：記憶卡", cardPath)
	return ebiten.Run(ebiten.Config{
		Machine: m, CardPath: cardPath, Scale: scale,
		Title: "ps-golem " + d.BootFile(), Frames: frames, Screenshot: shot,
	})
}
