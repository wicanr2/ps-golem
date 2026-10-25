package main

import (
	"github.com/wicanr2/ps-golem/frontend/ebiten"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWidescreenPreferencePreservesPortableSettings(t *testing.T) {
	base := t.TempDir()
	location := settingsLocation{base: base, path: filepath.Join(base, "settings.json"), portable: true}
	pad := ebiten.DefaultGamepadSettings()
	before := settings{Resolution: "1920x1080", HD: 4, PGXP: true, TexturePerspective: true, HDRenderer: "gpu", Textures: filepath.Join(base, "textures", "pack"), Posters: filepath.Join(base, "posters"), Poster: &ebiten.PosterSettings{ID: "b", Soft: true, Zoom: true}, Gamepad: &pad}
	updated, err := saveWidescreenPreference(location, before, true)
	if err != nil {
		t.Fatal(err)
	}
	got := loadSettings(location.path)
	want := location.forSave(before)
	want.Widescreen = true
	if !reflect.DeepEqual(got, want) || before.Widescreen || !updated.Widescreen {
		t.Fatal("view save lost preference or mutated caller")
	}
	updated.Gamepad.Device = "keyboard"
	if err := saveSettings(location.path, location.forSave(updated)); err != nil {
		t.Fatal(err)
	}
	got = loadSettings(location.path)
	if !got.Widescreen || got.Poster.ID != "b" || got.Textures != "textures/pack" {
		t.Fatal("other settings save lost pending view")
	}
	dir := filepath.Join(base, "directory")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	location.path = dir
	failed, err := saveWidescreenPreference(location, before, true)
	if err == nil || !reflect.DeepEqual(failed, before) {
		t.Fatal("failed write changed in-memory settings")
	}
}
