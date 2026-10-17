package sotn

import (
	"bytes"
	"crypto/sha256"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/machine"
	"os"
	"strings"
	"testing"
)

// TestM4FirstSave 重播 testdata/m4-first-save.press：從空白記憶卡開機，打完序章，
// 操作阿魯卡德走到入口區的存檔室存檔（docs/spec/015-sotn-autoplay.md §7）。
// 約 260 億週期，要 PSGOLEM_LONG=1 與原版素材（PSGOLEM_BIOS、SOTN_CUE，容器內路徑）才跑。
func TestM4FirstSave(t *testing.T) {
	bios, cue := os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE")
	if os.Getenv("PSGOLEM_LONG") == "" || bios == "" || cue == "" {
		t.Skip("需要 PSGOLEM_LONG=1、PSGOLEM_BIOS、SOTN_CUE")
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
	runM4Progress(t, m, ps)
	if got, want := m.Digest(), "6c35c2701228722f"; !strings.HasSuffix(got, want) {
		t.Errorf("摘要 %s，應以 %s 結尾", got, want)
	}
	// 記憶卡第 1 個目錄項：使用中的第一個區塊，檔名 BISLPM-86023DRAX00。
	dir := m.Card1.Data[0x80:0x100]
	if dir[0] != 0x51 || string(dir[0x0A:0x1C]) != "BISLPM-86023DRAX00" {
		t.Errorf("記憶卡目錄項不對：% x", dir[:0x20])
	}
	t.Logf("原生：%s，記憶卡 SHA256=%x，畫面 SHA256=%x", m.Digest(), sha256.Sum256(m.Card1.Data[:]), sha256.Sum256(m.GPU.Screen().Pix))
	for _, mode := range []string{"HD2", "HD2-PGXP"} {
		t.Run(mode, func(t *testing.T) {
			h, err := Boot(bios, cue, nil)
			if err != nil {
				t.Fatal(err)
			}
			s := gpu.NewShadow(1)
			h.GPU.SetShadow(s)
			defer func() { h.DisablePGXP(); h.GPU.SetShadow(nil); s.Close() }()
			var observe func()
			if mode == "HD2-PGXP" {
				if err := h.EnablePGXP(); err != nil {
					t.Fatal(err)
				}
				observe = recordPGXPFields(t, h, s)
			}
			runM4Progress(t, h, ps, observe)
			if got := h.Digest(); got != m.Digest() {
				t.Errorf("%s 摘要不同：%s", mode, got)
			}
			if !bytes.Equal(h.Card1.Data[:], m.Card1.Data[:]) {
				t.Errorf("%s 記憶卡不同", mode)
			}
			if !bytes.Equal(h.GPU.Screen().Pix, m.GPU.Screen().Pix) {
				t.Errorf("%s 原生畫面不同", mode)
			}
			t.Logf("%s：%s，記憶卡 SHA256=%x，畫面 SHA256=%x", mode, h.Digest(), sha256.Sum256(h.Card1.Data[:]), sha256.Sum256(h.GPU.Screen().Pix))
		})
	}
}

func runM4Progress(t *testing.T, m *machine.Machine, ps []Press, observers ...func()) {
	t.Helper()
	for m.Field() < 45784 {
		Step(m, ButtonsAt(ps, m.Field()))
		for _, observe := range observers {
			if observe != nil {
				observe()
			}
		}
		if m.Field()%5000 == 0 {
			t.Logf("已重播 field %d", m.Field())
		}
	}
}
