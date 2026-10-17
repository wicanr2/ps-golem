package sotn

import (
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/ps-golem/internal/gpu"
)

// TestWidescreenCaptures 由空白記憶卡走正常按鍵路徑，分別重播補黑與寬螢幕。
// 這是修改模式的圖像證據，不產生或比較機器摘要。
func TestWidescreenCaptures(t *testing.T) {
	out, bios, cue := os.Getenv("PSGOLEM_WS_EVIDENCE"), os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE")
	if out == "" || bios == "" || cue == "" {
		t.Skip("需要 PSGOLEM_WS_EVIDENCE、PSGOLEM_BIOS、SOTN_CUE")
	}
	b, err := os.ReadFile("testdata/m4-first-save.press")
	if err != nil {
		t.Fatal(err)
	}
	ps, err := ParseScript(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"padded", "wide"} {
		t.Run(mode, func(t *testing.T) {
			m, err := Boot(bios, cue, nil)
			if err != nil {
				t.Fatal(err)
			}
			s := gpu.NewShadow(1)
			if mode == "wide" {
				s.SetSideMargin(48)
			}
			m.GPU.SetShadow(s)
			defer func() { m.GPU.SetShadow(nil); s.Close() }()
			w := &Widescreen{}
			for m.Field() < 34500 {
				Step(m, ButtonsAt(ps, m.Field()))
				if mode == "wide" {
					prior := w.Applications
					if err := w.Check(m); err != nil {
						t.Fatal(err)
					}
					if w.Applications != prior {
						t.Logf("field=%d 簽章相符，套用次數=%d", m.Field(), w.Applications)
					}
					if m.Field() == 12000 && !w.applied {
						t.Fatal("遊玩時 DRA 簽章未曾相符")
					}
				}
				if m.Field()%5000 == 0 {
					t.Logf("field=%d", m.Field())
				}
				name := ""
				switch m.Field() {
				case 15000:
					name = "stairs"
				case 34500:
					name = "entrance"
				}
				if name == "" {
					continue
				}
				img := s.Screen()
				if mode == "padded" {
					padded := image.NewRGBA(image.Rect(0, 0, 684, img.Bounds().Dy()))
					for i := 3; i < len(padded.Pix); i += 4 {
						padded.Pix[i] = 255
					}
					draw.Draw(padded, image.Rect(86, 0, 598, img.Bounds().Dy()), img, image.Point{}, draw.Src)
					img = padded
				}
				writeHDPNG(t, filepath.Join(out, name+"-hd2-"+mode+".png"), img)
				t.Logf("%s field=%d 畫面=%v", name, m.Field(), img.Bounds())
			}
		})
	}
}
