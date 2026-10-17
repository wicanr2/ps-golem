package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableDirectory(t *testing.T) {
	for _, tc := range []struct{ platform, exe, app, want string }{
		{"linux", "/tmp/.mount/usr/bin/psgolem", "/games/ps.AppImage", "/games"},
		{"linux", "/games/psgolem", "", "/games"},
		{"darwin", "/games/ps-golem.app/Contents/MacOS/psgolem", "", "/games"},
		{"darwin", "/games/psgolem", "", "/games"},
		{"windows", "/games/psgolem.exe", "/ignored/a", "/games"},
	} {
		if got := portableDirectory(tc.platform, tc.exe, tc.app); got != tc.want {
			t.Fatalf("%+v: %q", tc, got)
		}
	}
}

func mediaFile(t *testing.T, base, name string, size int64) string {
	t.Helper()
	path := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverMedia(t *testing.T) {
	for _, sub := range []string{"", "bios", "disc"} {
		t.Run(sub, func(t *testing.T) {
			base := t.TempDir()
			bios := mediaFile(t, base, filepath.Join(sub, "rom"), 512<<10)
			cue := mediaFile(t, base, filepath.Join(sub, "game.CUE"), 10)
			mediaFile(t, base, "disc/nested/ignored.cue", 10)
			mediaFile(t, base, "bios/wrong.bin", 512<<10-1)
			b, c := discoverMedia(base)
			if b != bios || c != cue {
				t.Fatalf("%q %q", b, c)
			}
			mediaFile(t, base, "second-rom", 512<<10)
			b, c = discoverMedia(base)
			if b != "" || c != cue {
				t.Fatal("BIOS 歧義影響 cue 或仍被採用")
			}
			mediaFile(t, base, "other.cue", 10)
			b, c = discoverMedia(base)
			if b != "" || c != "" {
				t.Fatal("多個候選仍被採用")
			}
		})
	}
	if b, c := discoverMedia(t.TempDir()); b != "" || c != "" {
		t.Fatal("空目錄有候選")
	}
}

func TestPortableSettings(t *testing.T) {
	base, user := t.TempDir(), t.TempDir()
	if l := locateSettings(base, user); l.portable || l.path != filepath.Join(user, "settings.json") {
		t.Fatal(l)
	}
	st := settings{HD: 2, PGXP: true, Filter: "xbr", Widescreen: true}
	if err := saveSettings(filepath.Join(base, "settings.json"), st); err != nil {
		t.Fatal(err)
	}
	l := locateSettings(base, user)
	if !l.portable || l.path != filepath.Join(base, "settings.json") {
		t.Fatal(l)
	}
	bios := mediaFile(t, base, "bios/rom", 512<<10)
	cue := mediaFile(t, base, "disc/game.cue", 10)
	b, c := l.mediaPaths("", "", st)
	if b != bios || c != cue {
		t.Fatal(b, c)
	}
	st.BIOS, st.Cue = "bios/saved", "disc/saved.cue"
	b, c = l.mediaPaths("", "", st)
	if b != filepath.Join(base, st.BIOS) || c != filepath.Join(base, st.Cue) {
		t.Fatal(b, c)
	}
	b, c = l.mediaPaths("cli-bios", "cli-cue", st)
	if b != "cli-bios" || c != "cli-cue" {
		t.Fatal("命令列未優先")
	}
	b, c = l.mediaPaths("cli-bios", "", st)
	if b != "cli-bios" || c != filepath.Join(base, st.Cue) {
		t.Fatal("逐項優先順序錯誤")
	}
	st.BIOS, st.Cue = bios, cue
	saved := l.forSave(st)
	if filepath.IsAbs(saved.BIOS) || saved.BIOS != filepath.Join("bios", "rom") || saved.Cue != filepath.Join("disc", "game.cue") {
		t.Fatal(saved)
	}
	if err := saveSettings(l.path, saved); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(l.path); got != saved {
		t.Fatal(got)
	}
	// 搬到另一目錄，設定仍以新的可攜目錄解析。
	other := t.TempDir()
	moved := settingsLocation{filepath.Join(other, "settings.json"), other, true}
	b, c = moved.mediaPaths("", "", saved)
	if b != filepath.Join(other, saved.BIOS) || c != filepath.Join(other, saved.Cue) {
		t.Fatal(b, c)
	}
	legacy := settingsLocation{base: base}
	b, c = legacy.mediaPaths("", "", saved)
	if b != saved.BIOS || c != saved.Cue || legacy.forSave(st) != st {
		t.Fatal("一般設定路徑被改寫")
	}
	st.BIOS, st.Cue = bios, cue
	b, c = l.mediaPaths("", "", st)
	if b != bios || c != cue {
		t.Fatal("絕對路徑被改寫")
	}
}
