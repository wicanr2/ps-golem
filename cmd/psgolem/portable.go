package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 005 §8.1：AppImage 使用外部目錄，macOS 使用 .app 的上一層。
func portableDirectory(platform, executable, appImage string) string {
	if platform == "linux" && appImage != "" {
		return filepath.Dir(appImage)
	}
	dir := filepath.Dir(executable)
	if platform == "darwin" && filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" {
		app := filepath.Dir(filepath.Dir(dir))
		if strings.HasSuffix(app, ".app") {
			return filepath.Dir(app)
		}
	}
	return dir
}

type settingsLocation struct {
	path, base string
	portable   bool
}

func locateSettings(base, userDir string) settingsLocation {
	path := filepath.Join(base, "settings.json")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return settingsLocation{path, base, true}
	}
	return settingsLocation{filepath.Join(userDir, "settings.json"), base, false}
}

func (l settingsLocation) mediaPaths(bios, cue string, st settings) (string, string) {
	resolve := func(cli, saved, found string) string {
		if cli != "" {
			return cli
		}
		if saved != "" {
			if l.portable && !filepath.IsAbs(saved) {
				return filepath.Join(l.base, saved)
			}
			return saved
		}
		return found
	}
	foundBIOS, foundCue := discoverMedia(l.base)
	return resolve(bios, st.BIOS, foundBIOS), resolve(cue, st.Cue, foundCue)
}

// 三個目錄各讀一層；各類只有一個候選才採用，不以排序任選一個。
func discoverMedia(base string) (string, string) {
	var bios, cues []string
	for _, dir := range []string{base, filepath.Join(base, "bios"), filepath.Join(base, "disc")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if info.Size() == 512<<10 {
				bios = append(bios, path)
			}
			if strings.EqualFold(filepath.Ext(path), ".cue") {
				cues = append(cues, path)
			}
		}
	}
	unique := func(paths []string) string {
		if len(paths) == 1 {
			return paths[0]
		}
		return ""
	}
	return unique(bios), unique(cues)
}

func (l settingsLocation) forSave(st settings) settings {
	if !l.portable {
		return st
	}
	relative := func(path string) string {
		if path == "" {
			return ""
		}
		abs, err := filepath.Abs(path)
		if err == nil {
			if rel, err := filepath.Rel(l.base, abs); err == nil {
				return rel
			}
		}
		return path
	}
	st.BIOS, st.Cue = relative(st.BIOS), relative(st.Cue)
	st.Textures = relative(st.Textures)
	return st
}

// 018 §6：命令列可明確清除；可攜包的保存路徑相對包根解析。
func (l settingsLocation) texturePath(cli string, explicit bool, saved string) string {
	if explicit {
		return cli
	}
	if l.portable && saved != "" && !filepath.IsAbs(saved) {
		return filepath.Join(l.base, saved)
	}
	return saved
}

// 022：舊設定維持仿射，明確 false 可覆蓋已保存的透視選項。
func resolveTexturePerspective(cli, explicit, saved bool, scale int, pgxp bool) (bool, error) {
	if !explicit {
		cli = saved
	}
	if cli && (scale < 2 || !pgxp) {
		return false, fmt.Errorf("-texture-perspective 需要 -hd 2、4 或 8 及 -pgxp")
	}
	return cli, nil
}
