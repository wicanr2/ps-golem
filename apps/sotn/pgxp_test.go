package sotn

import (
	"bufio"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/machine"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// recordPGXPFields 每個 field 都寫一列，含沒有投影或多邊形的零值。
// 明確指定證據目錄才產檔，收據不進版控。
func recordPGXPFields(t *testing.T, m *machine.Machine, s *gpu.Shadow) func() {
	out := os.Getenv("PSGOLEM_PGXP_EVIDENCE")
	if out == "" {
		return nil
	}
	f, err := os.Create(filepath.Join(out, "m4-fields.csv"))
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "field,rtps,rtpt,valid_vertices,polygon_vertices,hit_rate")
	var prevR, prevT, prevH, prevV uint64
	t.Cleanup(func() {
		if err := w.Flush(); err != nil {
			t.Error(err)
		}
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	return func() {
		r, pt := m.PGXPTransforms()
		h, v := s.PrecisionStats()
		rate := float64(0)
		if v > prevV {
			rate = float64(h-prevH) / float64(v-prevV)
		}
		if _, err := fmt.Fprintf(w, "%d,%d,%d,%d,%d,%.9f\n", m.Field(), r-prevR, pt-prevT, h-prevH, v-prevV, rate); err != nil {
			t.Fatal(err)
		}
		prevR, prevT, prevH, prevV = r, pt, h, v
	}
}

// TestPGXPCaptures 的 field 清單須來自完整 M4 調查；只沿正常按鍵路線重播。
// 同一清單分別以 HD4 與 HD4-PGXP 從開機產生，沒有座標或狀態注入。
func TestPGXPCaptures(t *testing.T) {
	out, fields := os.Getenv("PSGOLEM_PGXP_EVIDENCE"), os.Getenv("PSGOLEM_PGXP_FIELDS")
	bios, cue := os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE")
	if out == "" || fields == "" || bios == "" || cue == "" {
		t.Skip("需要證據目錄、調查後的 field 清單與原版素材")
	}
	shots := map[uint64]bool{}
	var end uint64
	for _, s := range strings.Split(fields, ",") {
		f, err := strconv.ParseUint(s, 10, 64)
		if err != nil || f == 0 || f > 45784 {
			t.Fatalf("無效 field %q", s)
		}
		shots[f] = true
		end = max(end, f)
	}
	b, err := os.ReadFile("testdata/m4-first-save.press")
	if err != nil {
		t.Fatal(err)
	}
	ps, err := ParseScript(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		name := "hd4"
		if enabled {
			name = "hd4-pgxp"
		}
		t.Run(name, func(t *testing.T) {
			m, err := Boot(bios, cue, nil)
			if err != nil {
				t.Fatal(err)
			}
			s := gpu.NewShadow(2)
			m.GPU.SetShadow(s)
			defer func() { m.DisablePGXP(); m.GPU.SetShadow(nil); s.Close() }()
			if enabled {
				if err := m.EnablePGXP(); err != nil {
					t.Fatal(err)
				}
			}
			for m.Field() < end {
				Step(m, ButtonsAt(ps, m.Field()))
				if shots[m.Field()] {
					writeHDPNG(t, filepath.Join(out, fmt.Sprintf("field-%06d-%s.png", m.Field(), name)), s.Screen())
					t.Logf("%s field %d：%s", name, m.Field(), m.Digest())
				}
			}
		})
	}
}
