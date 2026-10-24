package main

import (
	"github.com/wicanr2/ps-golem/frontend/ebiten"
	"path/filepath"
	"testing"
)

func TestPosterPortableSettingsRoundtrip(t *testing.T) {
	base := t.TempDir()
	l := settingsLocation{base: base, path: filepath.Join(base, "settings.json"), portable: true}
	pad := ebiten.DefaultGamepadSettings()
	s := settings{Resolution: "1920x1080", HD: 4, PGXP: true, TexturePerspective: true, HDRenderer: "gpu", Textures: filepath.Join(base, "textures", "pack"), Posters: filepath.Join(base, "posters"), Poster: &ebiten.PosterSettings{ID: "b", Soft: true, Zoom: true}, Gamepad: &pad}
	if err := saveSettings(l.path, l.forSave(s)); err != nil {
		t.Fatal(err)
	}
	got := loadSettings(l.path)
	if got.Posters != "posters" || got.Poster == nil || *got.Poster != *s.Poster || got.HD != 4 || !got.PGXP || !got.TexturePerspective || got.HDRenderer != "gpu" || got.Gamepad == nil {
		t.Fatal("poster save lost fields", got)
	}
	moved := settingsLocation{base: filepath.Join(base, "moved"), portable: true}
	if moved.texturePath("", false, got.Posters) != filepath.Join(base, "moved", "posters") {
		t.Fatal("poster path not portable")
	}
	if moved.texturePath("", true, got.Posters) != "" {
		t.Fatal("explicit clear ignored")
	}
	got.Gamepad.Device = "keyboard"
	if err := saveSettings(l.path, got); err != nil {
		t.Fatal(err)
	}
	again := loadSettings(l.path)
	if *again.Poster != *s.Poster || again.Posters != "posters" {
		t.Fatal("gamepad save lost poster")
	}
	if got := loadSettings(filepath.Join(base, "missing.json")); got.Poster != nil || got.Posters != "" {
		t.Fatal("old settings enabled poster")
	}
}
