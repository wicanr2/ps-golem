package machine

import (
	"path"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/ps-golem/internal/gpu"
)

// TestShadowPS1GPU 包含 tests.zip 內全部 gpu/*.exe，每個 field 檢查全部 16 位元。
// 缺 BIOS 或語料時沿用既有 skip 契約，不用代用品。
func TestShadowPS1GPU(t *testing.T) {
	bios, files := loadBIOS(t), loadTestsZip(t)
	var names []string
	for name := range files {
		if strings.HasPrefix(name, "gpu/") && strings.HasSuffix(name, ".exe") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("語料沒有 GPU 程式")
	}
	for _, name := range names {
		t.Run(path.Dir(name), func(t *testing.T) {
			m, err := New(bios)
			if err != nil {
				t.Fatal(err)
			}
			s := gpu.NewShadow(0)
			m.GPU.SetShadow(s)
			defer func() { m.GPU.SetShadow(nil); s.Close() }()
			// shell 前每個 field 也檢查；到 shell 的最後半個 field 不略過。
			for m.CPU.PC() != ShellEntry && m.Cycles() < 400_000_000 {
				f := m.Field()
				for m.Field() == f && m.CPU.PC() != ShellEntry && m.Cycles() < 400_000_000 {
					m.Step()
				}
				m.sync()
				checkShadow(t, m, s)
			}
			if m.CPU.PC() != ShellEntry {
				t.Fatal("未到 shell")
			}
			if err := m.SideloadEXE(files[name]); err != nil {
				t.Fatal(err)
			}
			end := m.Cycles() + m3Cycles
			fields := 0
			for m.Cycles() < end {
				m.RunUntilField(m.Field()+1, end-m.Cycles())
				checkShadow(t, m, s)
				fields++
			}
			t.Logf("檢查 %d 個 field，全部 VRAM 位元相同", fields)
		})
	}
}

func checkShadow(t *testing.T, m *Machine, s *gpu.Shadow) {
	t.Helper()
	v := s.VRAMCopy()
	if slices.Equal(v, m.VRAM()) {
		return
	}
	for i, w := range m.VRAM() {
		if v[i] != w {
			t.Fatalf("field=%d VRAM[%d,%d] 原生=%04x 影子=%04x", m.Field(), i%1024, i/1024, w, v[i])
		}
	}
}
