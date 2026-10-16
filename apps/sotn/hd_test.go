package sotn

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wicanr2/ps-golem/internal/gpu"
)

// TestHDCaptures 只在明確指定證據目錄時產生含原版素材的本機截圖。
// 開機與按鍵都走正常玩家路徑，兩個倍率分別從空白狀態重播。
func TestHDCaptures(t *testing.T) {
	out := os.Getenv("PSGOLEM_HD_EVIDENCE")
	bios, cue := os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE")
	if out == "" || bios == "" || cue == "" {
		t.Skip("需要 PSGOLEM_HD_EVIDENCE、PSGOLEM_BIOS、SOTN_CUE")
	}
	b, err := os.ReadFile("testdata/m4-first-save.press")
	if err != nil {
		t.Fatal(err)
	}
	ps, err := ParseScript(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	shots := map[uint64]string{4320: "title", 12000: "prologue", 35000: "entrance"}
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m, err := Boot(bios, cue, nil)
			if err != nil {
				t.Fatal(err)
			}
			shift, _ := gpu.HDShift(n)
			s := gpu.NewShadow(shift)
			m.GPU.SetShadow(s)
			defer func() { m.GPU.SetShadow(nil); s.Close() }()
			for m.Field() < 35000 {
				Step(m, ButtonsAt(ps, m.Field()))
				name, ok := shots[m.Field()]
				if !ok {
					continue
				}
				native, scaled := m.GPU.Screen(), s.Screen()
				if n == 2 {
					writeHDPNG(t, filepath.Join(out, name+"-native.png"), native)
				}
				writeHDPNG(t, filepath.Join(out, fmt.Sprintf("%s-hd%d.png", name, n)), scaled)
				diff, inside := 0, 0
				for y := 0; y < native.Bounds().Dy(); y++ {
					for x := 0; x < native.Bounds().Dx(); x++ {
						c := native.RGBAAt(x, y)
						if c == scaled.RGBAAt(x*n, y*n) {
							continue
						}
						diff++
						flat := true
						for dy := -1; dy <= 1; dy++ {
							for dx := -1; dx <= 1; dx++ {
								if native.RGBAAt(x+dx, y+dy) != c {
									flat = false
								}
							}
						}
						if flat {
							inside++
						}
					}
				}
				t.Logf("%s field=%d native=%v HD%d=%v 左上取樣不同=%d 其中原生3x3平坦區=%d；%s", name, m.Field(), native.Bounds().Size(), n, scaled.Bounds().Size(), diff, inside, m.Digest())
			}
		})
	}
}

func writeHDPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestShadowBootPrologue 以正常按鍵重播開機至序章，逐 field 比對完整 VRAM。
func TestShadowBootPrologue(t *testing.T) {
	bios, cue := os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE")
	if os.Getenv("PSGOLEM_LONG") == "" || bios == "" || cue == "" {
		t.Skip("需要 PSGOLEM_LONG=1、PSGOLEM_BIOS、SOTN_CUE")
	}
	for _, p := range []string{bios, cue} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Skipf("缺素材 %s", p)
		}
	}
	b, err := os.ReadFile("testdata/m4-first-save.press")
	if err != nil {
		t.Fatal(err)
	}
	ps, err := ParseScript(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Boot(bios, cue, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := gpu.NewShadow(0)
	m.GPU.SetShadow(s)
	defer func() { m.GPU.SetShadow(nil); s.Close() }()
	for m.Field() < 12000 {
		Step(m, ButtonsAt(ps, m.Field()))
		if m.Field()%2000 == 0 {
			t.Logf("N=1 已檢查 field %d", m.Field())
		}
		v := s.VRAMCopy()
		if !slices.Equal(v, m.VRAM()) {
			for i, w := range m.VRAM() {
				if v[i] != w {
					t.Fatalf("field=%d VRAM[%d,%d] 原生=%04x 影子=%04x", m.Field(), i%1024, i/1024, w, v[i])
				}
			}
		}
	}
	t.Log("12000 個 field 的完整 VRAM 逐位元相同")
}
