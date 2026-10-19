package main

import (
	"path/filepath"
	"testing"
)

func TestHDRendererSettingsRoundTrip(t *testing.T) {
	for _, mode := range []string{"", "auto", "gpu", "cpu"} {
		p := filepath.Join(t.TempDir(), "settings.json")
		st := settings{HD: 2, HDRenderer: mode, PGXP: true, Filter: "xbr", Widescreen: true}
		if err := saveSettings(p, st); err != nil {
			t.Fatal(err)
		}
		got := loadSettings(p)
		if got.HDRenderer != mode || got.HD != 2 || !got.PGXP || !got.Widescreen || got.Filter != "xbr" {
			t.Fatalf("settings not preserved: %+v", got)
		}
	}
}
