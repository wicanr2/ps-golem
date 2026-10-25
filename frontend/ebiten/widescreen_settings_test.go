package ebiten

import (
	"errors"
	"image"
	"strings"
	"testing"
)

func TestWidescreenSavedPreferenceDoesNotSwitchActiveSession(t *testing.T) {
	saved := false
	g := &Game{cfg: Config{Widescreen: false, SaveWidescreen: func(next bool) error { saved = next; return nil }}, shown: image.Pt(1920, 1080)}
	g.changeWidescreen(true)
	if !saved || !g.nextWidescreen || g.cfg.Widescreen || g.shown != (image.Point{}) || !strings.Contains(g.widescreenNotice, "下次啟動套用") {
		t.Fatal("pending mode or active session changed", g.nextWidescreen, g.cfg.Widescreen, g.widescreenNotice)
	}
	g.changeWidescreen(false)
	if saved || g.nextWidescreen || g.cfg.Widescreen || strings.Contains(g.widescreenNotice, "下次啟動套用") {
		t.Fatal("return to active preference not reflected")
	}
	g.cfg.SaveWidescreen = func(bool) error { return errors.New("read-only settings") }
	g.changeWidescreen(true)
	if g.nextWidescreen || g.cfg.Widescreen || !strings.Contains(g.widescreenNotice, "read-only settings") {
		t.Fatal("failed save changed preference")
	}
}
