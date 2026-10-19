package main

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/ps-golem/frontend/ebiten"
)

func TestGamepadSettingsRoundTripPreservesOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	pad := ebiten.DefaultGamepadSettings()
	pad.Device = "keyboard"
	pad.DeadZone = 0.35
	st := settings{BIOS: "bios/original.bin", Cue: "disc/original.cue", HD: 4, PGXP: true, Filter: "xbr", Widescreen: true, Gamepad: &pad}
	if err := saveSettings(path, st); err != nil {
		t.Fatal(err)
	}
	got := loadSettings(path)
	if got.BIOS != st.BIOS || got.Cue != st.Cue || got.HD != st.HD || got.PGXP != st.PGXP || got.Filter != st.Filter || got.Widescreen != st.Widescreen {
		t.Fatal("saving gamepad lost other settings")
	}
	if got.Gamepad == nil || got.Gamepad.Device != "keyboard" || got.Gamepad.DeadZone != 0.35 || got.Gamepad.Validate() != nil {
		t.Fatal("gamepad did not roundtrip")
	}
}
