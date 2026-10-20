package main

import (
	"path/filepath"
	"testing"
)

func TestResolutionPreferenceAndValidation(t *testing.T) {
	cases := []struct {
		cli, saved, want  string
		explicit, invalid bool
	}{
		{"", "", "", false, false},
		{"", "1920x1080", "1920x1080", false, false},
		{"1280x720", "1920x1080", "1280x720", true, false},
		{"auto", "1920x1080", "", true, false},
		{"", "1920x1080", "", true, false},
		{" 1920X1080 ", "", "1920x1080", true, false},
		{"7680x4320", "", "7680x4320", true, false},
		{"", "1920x0", "", false, true},
		{"1920", "", "", true, true},
		{"1920x1080x2", "", "", true, true},
		{"319x1080", "", "", true, true},
		{"1920x239", "", "", true, true},
		{"7681x1080", "", "", true, true},
		{"1920x4321", "", "", true, true},
		{"999999999999999999999999x1080", "", "", true, true},
	}
	for _, tc := range cases {
		name, w, h, err := resolveResolution(tc.cli, tc.explicit, tc.saved)
		if (err != nil) != tc.invalid || name != tc.want {
			t.Fatalf("%+v: %q %dx%d %v", tc, name, w, h, err)
		}
		if tc.want == "1920x1080" && (w != 1920 || h != 1080) {
			t.Fatal(w, h)
		}
	}
}

func TestResolutionSavedWithOtherPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := settings{Resolution: "1920x1080", HD: 8, HDRenderer: "gpu", PGXP: true,
		TexturePerspective: true, Textures: "textures/custom", Filter: "none", Widescreen: true}
	if err := saveSettings(path, before); err != nil {
		t.Fatal(err)
	}
	after := loadSettings(path)
	if after.Resolution != before.Resolution || after.HD != 8 || after.HDRenderer != "gpu" ||
		!after.PGXP || !after.TexturePerspective || !after.Widescreen || after.Textures != before.Textures {
		t.Fatal(after)
	}
	name, w, h, err := resolveResolution("", false, after.Resolution)
	if err != nil || name != before.Resolution || w != 1920 || h != 1080 {
		t.Fatal(name, w, h, err)
	}
}
