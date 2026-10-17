package sotn

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"reflect"
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
			// 只讀既有 GP0 解碼狀態辨識命令邊界，避免把傳輸資料誤認為 E3。
			// 收據涵蓋截圖前兩個 field，不改原生或影子狀態。
			state := reflect.ValueOf(m.GPU).Elem()
			var trace strings.Builder
			m.GPU.Trace = func(v uint32) {
				field := m.Field()
				if field != 14998 && field != 14999 && field != 34498 && field != 34499 {
					return
				}
				if state.FieldByName("bufLen").Int() == 0 && state.FieldByName("inCmd").Int() == 0 && (v>>24 == 0xe3 || v>>24 == 0xe4) {
					fmt.Fprintf(&trace, "field=%d GP0=%08x prior=(%d,%d)-(%d,%d)\n", field, v,
						state.FieldByName("clipX0").Int(), state.FieldByName("clipY0").Int(),
						state.FieldByName("clipX1").Int(), state.FieldByName("clipY1").Int())
				}
			}
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
				fmt.Fprintf(&trace, "capture=%s field=%d display=(%d,%d) clip=(%d,%d)-(%d,%d)\n", name, m.Field(),
					state.FieldByName("displayXStart").Uint(), state.FieldByName("displayYStart").Uint(),
					state.FieldByName("clipX0").Int(), state.FieldByName("clipY0").Int(),
					state.FieldByName("clipX1").Int(), state.FieldByName("clipY1").Int())
				if err := os.WriteFile(filepath.Join(out, "draw-area-"+mode+".log"), []byte(trace.String()), 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("%s field=%d 畫面=%v", name, m.Field(), img.Bounds())
			}
		})
	}
}
