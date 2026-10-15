package ebiten

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"

	"github.com/wicanr2/ps-golem/internal/sio"
)

func TestButtons(t *testing.T) {
	keys := map[eb.Key]bool{eb.KeyX: true, eb.KeyArrowUp: true}
	pads := []eb.GamepadID{0}
	padDown := map[eb.StandardGamepadButton]bool{eb.StandardGamepadButtonCenterRight: true}
	got := buttons(func(k eb.Key) bool { return keys[k] }, pads,
		func(_ eb.GamepadID, b eb.StandardGamepadButton) bool { return padDown[b] })
	if want := uint16(sio.BtnCircle | sio.BtnUp | sio.BtnStart); got != want {
		t.Errorf("按鍵 %04x，應為 %04x", got, want)
	}
}

func TestFit(t *testing.T) {
	cases := []struct {
		w, h int
		want image.Rectangle
	}{
		{960, 720, image.Rect(0, 0, 960, 720)},
		{1280, 720, image.Rect(160, 0, 1120, 720)}, // 寬螢幕視窗左右留黑
		{800, 800, image.Rect(0, 100, 800, 700)},
	}
	for _, c := range cases {
		if got := fit(c.w, c.h); got != c.want {
			t.Errorf("fit(%d,%d) = %v，應為 %v", c.w, c.h, got, c.want)
		}
	}
}

func TestCardSaverWaitsForQuiet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "c.mcd")
	s := cardSaver{path: path}
	c := sio.NewMemcard()
	c.Data[0x2000] = 1
	c.Dirty, c.Writes = true, 3
	if wrote, _ := s.tick(c, 100); wrote {
		t.Fatal("剛寫入就存檔")
	}
	if wrote, _ := s.tick(c, 100+cardQuietFields-1); wrote {
		t.Fatal("還沒安靜夠久就存檔")
	}
	wrote, err := s.tick(c, 100+cardQuietFields)
	if err != nil || !wrote {
		t.Fatalf("該存檔了：wrote=%v err=%v", wrote, err)
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) != sio.MemcardSize || b[0x2000] != 1 || c.Dirty {
		t.Fatalf("存檔內容不對：len=%d err=%v dirty=%v", len(b), err, c.Dirty)
	}
}
