// psgolem 是玩家用的視窗執行檔（docs/spec/012-frontend-ebiten.md §2、§9）。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	// early 必須先於 Ebiten 初始化，才能在沒有顯示器時處理 -version。
	"github.com/wicanr2/ps-golem/cmd/psgolem/early"
	"github.com/wicanr2/ps-golem/frontend/ebiten"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
)

// settings 是 <使用者設定目錄>/ps-golem/settings.json（012 §9）。
type settings struct {
	BIOS   string `json:"bios"`
	Cue    string `json:"cue"`
	HD     int    `json:"hd"`
	Filter string `json:"filter"`
}

// version 見 early.Version。
var version = early.Version

func main() {
	biosPath := flag.String("bios", "", "BIOS 映像（512 KB）；省略時取設定檔")
	cuePath := flag.String("cue", "", "光碟映像的 cue 檔；省略時取設定檔")
	cardPath := flag.String("card", "", "1P 記憶卡檔案；省略時放在使用者設定目錄的 ps-golem/<開機檔名>.mcd")
	scale := flag.Int("scale", 3, "起始視窗倍率")
	frames := flag.Int("frames", 0, "跑這麼多次 Update 後結束（驗收用）")
	shot := flag.String("screenshot", "", "配合 -frames：結束前存下畫面")
	mute := flag.Bool("mute", false, "不開音訊裝置")
	pressSpec := flag.String("press", "", "按鍵腳本 FIELD:BTN[+BTN][:DUR],...，取代鍵盤與手把（驗收用）")
	untilField := flag.Uint64("fields", 0, "跑到第這麼多次 VBlank 後存記憶卡並結束（驗收用）")
	showVersion := flag.Bool("version", false, "印出版號後結束")
	stats := flag.Bool("stats", false, "每 600 個 field 印出實際 FPS 與模擬耗時")
	hdFlag := flag.Int("hd", 0, "影子 GPU 倍率：0、1、2、4、8；省略時取設定檔")
	filterFlag := flag.String("filter", "none", "輸出濾鏡：none、xbr；省略時取設定檔")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(1)
	}
	dir = filepath.Join(dir, "ps-golem")
	st := loadSettings(filepath.Join(dir, "settings.json"))
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["hd"] {
		*hdFlag = st.HD
	}
	if !set["filter"] && st.Filter != "" {
		*filterFlag = st.Filter
	}
	shift, ok := gpu.HDShift(*hdFlag)
	if !ok || (*filterFlag != "none" && *filterFlag != "xbr") {
		fmt.Fprintln(os.Stderr, "psgolem：hd 必須是 0、1、2、4、8；filter 必須是 none、xbr")
		os.Exit(2)
	}
	if *biosPath == "" {
		*biosPath = st.BIOS
	}
	if *cuePath == "" {
		*cuePath = st.Cue
	}
	press, err := padscript.Parse(*pressSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(2)
	}
	open := func(bios, cue string) (*ebiten.Session, error) {
		s, err := openSession(bios, cue, *cardPath, dir)
		if err != nil {
			return nil, err
		}
		if *hdFlag != 0 {
			shadow := gpu.NewShadow(shift)
			s.Machine.GPU.SetShadow(shadow)
			closeSession := s.Close
			s.Close = func() { s.Machine.GPU.SetShadow(nil); shadow.Close(); closeSession() }
		}
		if err := saveSettings(filepath.Join(dir, "settings.json"), settings{BIOS: bios, Cue: cue, HD: *hdFlag, Filter: *filterFlag}); err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：設定檔：", err)
		}
		return s, nil
	}
	err = ebiten.Run(ebiten.Config{
		Open: open, BIOSPath: *biosPath, CuePath: *cuePath, Scale: *scale,
		Title: "ps-golem " + version, Frames: *frames, Screenshot: *shot, Mute: *mute,
		Press: press, UntilField: *untilField, Stats: *stats,
		Filter: *filterFlag,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(1)
	}
}

func loadSettings(path string) settings {
	var st settings
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &st); err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：設定檔", path, "讀不懂：", err)
		}
	}
	return st
}

// saveSettings 先寫暫存檔再改名，避免寫到一半中斷留下壞檔。
func saveSettings(path string, st settings) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func openSession(biosPath, cuePath, cardPath, dir string) (*ebiten.Session, error) {
	bios, err := os.ReadFile(biosPath)
	if err != nil {
		return nil, err
	}
	m, err := machine.New(bios)
	if err != nil {
		return nil, err
	}
	d, err := disc.Open(cuePath)
	if err != nil {
		return nil, err
	}
	if d.Fallback != "" {
		fmt.Fprintln(os.Stderr, "psgolem：", d.Fallback)
	}
	m.InsertDisc(d)
	if cardPath == "" {
		name := d.BootFile()
		if name == "" {
			name = "card"
		}
		cardPath = filepath.Join(dir, name+".mcd")
	}
	if b, err := os.ReadFile(cardPath); err == nil {
		if !m.Card1.Load(b) {
			d.Close()
			return nil, fmt.Errorf("記憶卡 %s 不是 128 KB", cardPath)
		}
	} else if !os.IsNotExist(err) {
		d.Close()
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "psgolem：記憶卡", cardPath)
	return &ebiten.Session{
		Machine: m, CardPath: cardPath, Title: "ps-golem " + d.BootFile(),
		Close: func() { d.Close() },
	}, nil
}
