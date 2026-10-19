package main

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/ps-golem/frontend/ebiten"
)

func TestTexturePortableRememberClearAndGamepadSave(t *testing.T) {
	base := t.TempDir()
	location := settingsLocation{path: filepath.Join(base, "settings.json"), base: base, portable: true}
	g := ebiten.DefaultGamepadSettings()
	g.Device = "keyboard"
	st := settings{BIOS: filepath.Join(base, "bios", "own.bin"), Cue: filepath.Join(base, "disc", "own.cue"), HD: 2, Filter: "xbr", PGXP: true, Widescreen: true, Gamepad: &g, Textures: filepath.Join(base, "hd-assets", "pack")}
	if err := saveSettings(location.path, location.forSave(st)); err != nil {
		t.Fatal(err)
	}
	loaded := loadSettings(location.path)
	if loaded.Textures != filepath.Join("hd-assets", "pack") || loaded.Gamepad.Device != "keyboard" || loaded.HD != 2 || !loaded.PGXP || !loaded.Widescreen || loaded.Filter != "xbr" {
		t.Fatal("settings fields lost")
	}
	if got := location.texturePath("", false, loaded.Textures); got != st.Textures {
		t.Fatal("portable texture path did not resolve")
	}
	other := settingsLocation{base: filepath.Join(base, "moved"), portable: true}
	if got := other.texturePath("", false, loaded.Textures); got != filepath.Join(other.base, loaded.Textures) {
		t.Fatal("moved pack used old directory")
	}
	if got := location.texturePath("", true, loaded.Textures); got != "" {
		t.Fatal("explicit clear ignored")
	}
	loaded.Textures = ""
	if err := saveSettings(location.path, location.forSave(loaded)); err != nil {
		t.Fatal(err)
	}
	if loaded = loadSettings(location.path); loaded.Textures != "" {
		t.Fatal("cleared texture path not persisted")
	}
	if got := location.forSave(settings{}); got.Textures != "" || got.BIOS != "" || got.Cue != "" {
		t.Fatal("empty path became portable dot")
	}
}
