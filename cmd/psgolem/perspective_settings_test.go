package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTexturePerspectiveRememberAndExplicitOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"hd":2,"pgxp":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	st := loadSettings(path)
	if st.TexturePerspective {
		t.Fatal("old settings enabled perspective")
	}
	st.TexturePerspective = true
	if err := saveSettings(path, st); err != nil {
		t.Fatal(err)
	}
	saved := loadSettings(path)
	if !saved.TexturePerspective || !saved.PGXP || saved.HD != 2 {
		t.Fatal(saved)
	}
	for _, tc := range []struct {
		cli, explicit, saved bool
		scale                int
		pgxp, want, fail     bool
	}{
		{false, false, true, 2, true, true, false}, {false, true, true, 2, true, false, false},
		{true, true, false, 2, true, true, false}, {true, true, false, 1, true, false, true},
		{true, true, false, 2, false, false, true}, {false, false, false, 0, false, false, false},
	} {
		got, err := resolveTexturePerspective(tc.cli, tc.explicit, tc.saved, tc.scale, tc.pgxp)
		if got != tc.want || (err != nil) != tc.fail {
			t.Fatalf("%+v got=%v err=%v", tc, got, err)
		}
	}
}
