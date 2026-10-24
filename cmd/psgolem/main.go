// psgolem 是玩家用的視窗執行檔（docs/spec/012-frontend-ebiten.md §2、§9）。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	// early 必須先於 Ebiten 初始化，才能在沒有顯示器時處理 -version。
	"github.com/wicanr2/ps-golem/apps/sotn"
	"github.com/wicanr2/ps-golem/cmd/psgolem/early"
	"github.com/wicanr2/ps-golem/frontend/ebiten"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
	"github.com/wicanr2/ps-golem/render/hd"
	"github.com/wicanr2/ps-golem/render/hdgpu"
)

// settings 是 <使用者設定目錄>/ps-golem/settings.json（012 §9）。
type settings struct {
	Resolution         string                  `json:"resolution,omitempty"`
	TexturePerspective bool                    `json:"texture_perspective,omitempty"`
	HDRenderer         string                  `json:"hd_renderer,omitempty"`
	BIOS               string                  `json:"bios"`
	Cue                string                  `json:"cue"`
	HD                 int                     `json:"hd"`
	Filter             string                  `json:"filter"`
	Widescreen         bool                    `json:"widescreen"`
	PGXP               bool                    `json:"pgxp"`
	Gamepad            *ebiten.GamepadSettings `json:"gamepad,omitempty"`
	Textures           string                  `json:"textures,omitempty"`
	Posters            string                  `json:"posters,omitempty"`
	Poster             *ebiten.PosterSettings  `json:"poster,omitempty"`
}

// version 見 early.Version。
var version = early.Version

func main() {
	biosPath := flag.String("bios", "", "BIOS 映像（512 KB）；省略時取設定檔")
	cuePath := flag.String("cue", "", "光碟映像的 cue 檔；省略時取設定檔")
	cardPath := flag.String("card", "", "1P 記憶卡檔案；省略時放在使用者設定目錄的 ps-golem/<開機檔名>.mcd")
	scale := flag.Int("scale", 3, "起始視窗倍率")
	resolutionFlag := flag.String("resolution", "", "起始視窗解析度，例如1920x1080；省略取設定檔，auto清除")
	frames := flag.Int("frames", 0, "跑這麼多次 Update 後結束（驗收用）")
	shot := flag.String("screenshot", "", "配合 -frames：結束前存下畫面")
	mute := flag.Bool("mute", false, "不開音訊裝置")
	pressSpec := flag.String("press", "", "按鍵腳本 FIELD:BTN[+BTN][:DUR],...，取代鍵盤與手把（驗收用）")
	untilField := flag.Uint64("fields", 0, "跑到第這麼多次 VBlank 後存記憶卡並結束（驗收用）")
	showVersion := flag.Bool("version", false, "印出版號後結束")
	showStats := flag.Bool("stats", false, "每 600 個 field 印出實際 FPS 與模擬耗時")
	hdFlag := flag.Int("hd", 0, "影子 GPU 倍率：0、1、2、4、8；省略時取設定檔")
	rendererFlag := flag.String("hd-renderer", "cpu", "HD繪圖後端：cpu（預設）、gpu、auto；省略時取設定檔")
	perspectiveFlag := flag.Bool("texture-perspective", false, "HD 3D貼圖透視校正；需要 -hd 2以上及 -pgxp，省略時取設定檔")
	pgxpFlag := flag.Bool("pgxp", false, "PGXP 次像素精度；省略時取設定檔，需要 -hd 2 以上")
	filterFlag := flag.String("filter", "none", "輸出濾鏡：none、xbr；省略時取設定檔")
	wideFlag := flag.Bool("widescreen", false, "月下寬螢幕遊戲修改模式；預設關閉")
	texturesFlag := flag.String("textures", "", "HD 素材包目錄；省略時取設定檔，-textures= 清除")
	postersFlag := flag.String("posters", "", "海報目錄；省略取設定檔，空值清除")
	posterFlag := flag.String("poster", "", "海報代號；空值關閉")
	posterSoft := flag.Bool("poster-soft", true, "海報外側柔和融合")
	posterZoom := flag.Bool("poster-zoom", true, "黑邊足夠時等比放大中央")
	dumpFlag := flag.String("dump-textures", "", "本次傾印到新的本機目錄，不保存此路徑")
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
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(1)
	}
	location := locateSettings(portableDirectory(runtime.GOOS, exe, os.Getenv("APPIMAGE")), dir)
	st := loadSettings(location.path)
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	resolution, width, height, err := resolveResolution(*resolutionFlag, set["resolution"], st.Resolution)
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem：", err)
		os.Exit(2)
	}
	if !set["hd"] {
		*hdFlag = st.HD
	}
	if !set["pgxp"] {
		*pgxpFlag = st.PGXP
	}
	if !set["widescreen"] {
		*wideFlag = st.Widescreen
	}
	*texturesFlag = location.texturePath(*texturesFlag, set["textures"], st.Textures)
	if *texturesFlag != "" {
		*texturesFlag, err = filepath.Abs(*texturesFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "psgolem:", err)
			os.Exit(2)
		}
	}
	if (*wideFlag || *texturesFlag != "" || *dumpFlag != "") && *hdFlag == 0 {
		*hdFlag = 1
	}
	*perspectiveFlag, err = resolveTexturePerspective(*perspectiveFlag, set["texture-perspective"], st.TexturePerspective, *hdFlag, *pgxpFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem：", err)
		os.Exit(2)
	}
	if *pgxpFlag && *hdFlag < 2 {
		fmt.Fprintln(os.Stderr, "psgolem：-pgxp 需要 -hd 2、4 或 8")
		os.Exit(2)
	}
	if !set["filter"] && st.Filter != "" {
		*filterFlag = st.Filter
	}
	if !set["hd-renderer"] && st.HDRenderer != "" {
		*rendererFlag = st.HDRenderer
	}
	if *rendererFlag != "auto" && *rendererFlag != "cpu" && *rendererFlag != "gpu" {
		fmt.Fprintln(os.Stderr, "psgolem：hd-renderer 必須是 auto、gpu、cpu")
		os.Exit(2)
	}
	shift, ok := gpu.HDShift(*hdFlag)
	if !ok || (*filterFlag != "none" && *filterFlag != "xbr") {
		fmt.Fprintln(os.Stderr, "psgolem：hd 必須是 0、1、2、4、8；filter 必須是 none、xbr")
		os.Exit(2)
	}
	*biosPath, *cuePath = location.mediaPaths(*biosPath, *cuePath, st)
	press, err := padscript.Parse(*pressSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "psgolem:", err)
		os.Exit(2)
	}
	*postersFlag = location.texturePath(*postersFlag, set["posters"], st.Posters)
	if *postersFlag != "" {
		*postersFlag, err = filepath.Abs(*postersFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	posterSettings := ebiten.PosterSettings{Soft: true, Zoom: true}
	if st.Poster != nil {
		posterSettings = *st.Poster
	}
	if set["poster"] {
		posterSettings.ID = *posterFlag
	}
	if set["poster-soft"] {
		posterSettings.Soft = *posterSoft
	}
	if set["poster-zoom"] {
		posterSettings.Zoom = *posterZoom
	}
	if err := posterSettings.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var posterPack *ebiten.PosterPack
	if *postersFlag != "" {
		posterPack, err = ebiten.LoadPosterPack(*postersFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：海報未載入：", err)
		}
	}
	gamepadSettings := ebiten.DefaultGamepadSettings()
	if st.Gamepad != nil {
		if e := st.Gamepad.Validate(); e != nil {
			fmt.Fprintln(os.Stderr, "psgolem：手把設定：", e)
		} else {
			gamepadSettings = st.Gamepad.Clone()
		}
	}
	st = settings{Resolution: resolution, TexturePerspective: *perspectiveFlag, BIOS: *biosPath, Cue: *cuePath, HD: *hdFlag, HDRenderer: *rendererFlag, Filter: *filterFlag, PGXP: *pgxpFlag, Widescreen: *wideFlag, Gamepad: &gamepadSettings, Textures: *texturesFlag, Posters: *postersFlag, Poster: &posterSettings}
	saveGamepad := func(next ebiten.GamepadSettings) error {
		updated := st
		updated.Gamepad = &next
		if err := saveSettings(location.path, location.forSave(updated)); err != nil {
			return err
		}
		gamepadSettings = next.Clone()
		st = updated
		return nil
	}
	savePoster := func(next ebiten.PosterSettings) error {
		updated := st
		updated.Poster = &next
		if err := saveSettings(location.path, location.forSave(updated)); err != nil {
			return err
		}
		posterSettings = next
		st = updated
		return nil
	}
	var dumpErr error
	open := func(bios, cue string) (*ebiten.Session, error) {
		var pack *hd.TexturePack
		if *texturesFlag != "" {
			var loadErr error
			pack, loadErr = hd.LoadTexturePack(*texturesFlag)
			if loadErr == nil && pack.SpriteRunCount() > 0 {
				fmt.Fprintf(os.Stderr, "psgolem：HD繪圖序列=%d\n", pack.SpriteRunCount())
			}
			if loadErr != nil {
				return nil, fmt.Errorf("素材包 %s：%w", *texturesFlag, loadErr)
			}
		}
		var dump *hd.TextureDump
		if *dumpFlag != "" {
			if e := hd.CheckTextureDumpDirectory(*dumpFlag); e != nil {
				return nil, e
			}
			dump = hd.NewTextureDump()
		}
		s, err := openSession(bios, cue, *cardPath, dir)
		if err != nil {
			return nil, err
		}
		if *hdFlag != 0 {

			var raster hd.RasterDevice
			selected := "cpu"
			if *rendererFlag != "cpu" {
				device, deviceErr := hdgpu.New(shift)
				if deviceErr != nil {
					if *rendererFlag == "gpu" {
						s.Close()
						return nil, deviceErr
					}
					fmt.Fprintf(os.Stderr, "psgolem：GPU HD裝置建立失敗，使用CPU：%v\n", deviceErr)
				} else {
					raster = device
					selected = "gpu"
				}
			}
			fmt.Fprintf(os.Stderr, "psgolem：HD繪圖後端=%s\n", selected)
			shadow := hd.NewShadowWithDevice(shift, pack, dump, raster)
			shadow.SetTexturePerspective(*perspectiveFlag)
			if *wideFlag {
				shadow.SetSideMargin(48)
			}
			s.Machine.GPU.SetShadow(shadow)
			closeSession := s.Close
			s.Close = func() {
				var stats hd.TextureStats
				if *texturesFlag != "" || dump != nil {
					stats = shadow.TextureStats()
				}

				if *showStats {
					rs := shadow.RasterStatistics()
					fmt.Fprintf(os.Stderr, "psgolem：HD後端=%s 三角形=%d 精靈=%d GPU像素=%d CPU回退=%d 回讀=%d\n", selected, rs.Triangles, rs.Sprites, rs.Pixels, rs.Fallbacks, rs.Readbacks)
				}
				s.Machine.DisablePGXP()
				s.Machine.GPU.SetShadow(nil)
				shadow.Close()
				if dump != nil {
					dumpErr = errors.Join(dumpErr, dump.WriteDirectory(*dumpFlag))
				}
				closeSession()
				if *texturesFlag != "" || dump != nil {
					fmt.Fprintf(os.Stderr, "psgolem：材質上傳=%d 查詢=%d 命中=%d 替換取樣=%d\n", stats.Uploads, stats.Lookups, stats.Hits, stats.ReplacedSamples)
				}
			}
		}
		if *pgxpFlag {
			if err := s.Machine.EnablePGXP(); err != nil {
				s.Close()
				return nil, err
			}
		}
		if *wideFlag {
			w := &sotn.Widescreen{}
			s.AfterField = w.Check
			closeSession := s.Close
			s.Close = func() { w.Disable(s.Machine); closeSession() }
		}
		st = settings{Resolution: resolution, TexturePerspective: *perspectiveFlag, BIOS: bios, Cue: cue, HD: *hdFlag, HDRenderer: *rendererFlag, Filter: *filterFlag, PGXP: *pgxpFlag, Widescreen: *wideFlag, Gamepad: &gamepadSettings, Textures: *texturesFlag, Posters: *postersFlag, Poster: &posterSettings}
		if err := saveSettings(location.path, location.forSave(st)); err != nil {
			fmt.Fprintln(os.Stderr, "psgolem：設定檔：", err)
		}
		return s, nil
	}
	err = ebiten.Run(ebiten.Config{
		Open: open, BIOSPath: *biosPath, CuePath: *cuePath, Scale: *scale, WindowWidth: width, WindowHeight: height,
		Title: "ps-golem " + version, Frames: *frames, Screenshot: *shot, Mute: *mute,
		Press: press, UntilField: *untilField, Stats: *showStats,
		Filter: *filterFlag, Widescreen: *wideFlag,
		Gamepad: &gamepadSettings, SaveGamepad: saveGamepad,
		Posters: posterPack, Poster: posterSettings, SavePoster: savePoster,
	})
	err = errors.Join(err, dumpErr)
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
