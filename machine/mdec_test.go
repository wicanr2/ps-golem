package machine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// MDEC 驗收（docs/spec/011-mdec.md §5）：ps1-tests mdec/* 的 VRAM 與 TTY，
// 不符數記在台帳 mdec_gaps.txt，變好或變差都算失敗。重寫台帳：PSGOLEM_WRITE_LEDGER=1。

type mdecCase struct {
	name, exe, png, log string
	cycles              uint64
}

var mdecCases = []mdecCase{
	{"mdec/4bit", "mdec/4bit/4bit.exe", "mdec/4bit/vram.png", "mdec/4bit/psx.log", m3Cycles},
	{"mdec/8bit", "mdec/8bit/8bit.exe", "mdec/8bit/vram.png", "mdec/8bit/psx.log", m3Cycles},
	{"mdec/frame-15bit", "mdec/frame/frame-15bit.exe", "mdec/frame/vram-15bit.png", "", m3Cycles},
	{"mdec/frame-15bit-dma", "mdec/frame/frame-15bit-dma.exe", "mdec/frame/vram-15bit.png", "", m3Cycles},
	{"mdec/frame-24bit", "mdec/frame/frame-24bit.exe", "mdec/frame/vram-24bit.png", "", m3Cycles},
	{"mdec/frame-24bit-dma", "mdec/frame/frame-24bit-dma.exe", "mdec/frame/vram-24bit.png", "", m3Cycles},
	{"mdec/movie-15bit", "mdec/movie/movie-15bit.exe", "mdec/movie/vram-15bit.png", "", m3Cycles},
	{"mdec/movie-24bit", "mdec/movie/movie-24bit.exe", "mdec/movie/vram-24bit.png", "", m3Cycles},
	{"mdec/step-by-step-log", "mdec/step-by-step-log/step-by-step-log.exe", "", "mdec/step-by-step-log/psx.log", m3Cycles},
}

func TestMDEC(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	boot := bootSnapshot(t, bios)

	got := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	set := func(k string, n int) {
		mu.Lock()
		got[k] = n
		mu.Unlock()
	}
	for _, c := range mdecCases {
		wg.Add(1)
		go func(c mdecCase) {
			defer wg.Done()
			exe := files[c.exe]
			if exe == nil {
				t.Errorf("tests.zip 裡缺 %s", c.exe)
				return
			}
			m := boot.Clone()
			mark := len(m.TTY())
			if err := m.SideloadEXE(exe); err != nil {
				t.Errorf("%s：%v", c.name, err)
				return
			}
			m.Run(c.cycles)
			m.tty = m.tty[mark:]
			if dir := os.Getenv("PSGOLEM_DUMP_VRAM"); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(c.name, "/", "_")+".png"), VRAMPNG(m.VRAM()), 0o644)
			}
			if c.png != "" {
				n, err := vramMismatch(m.VRAM(), files[c.png], 1024, 512)
				if err != nil {
					t.Errorf("%s：%v", c.name, err)
				}
				set(c.name+" vram", n)
			}
			if c.log != "" {
				n := ttyMismatch(m.TTY(), string(files[c.log]))
				if n != 0 && os.Getenv("PSGOLEM_SHOW_TTY") != "" {
					t.Logf("%s 實得 TTY：\n%s", c.name, m.TTY())
				}
				set(c.name+" tty", n)
			}
		}(c)
	}
	wg.Wait()

	ledger := filepath.Join(filepath.Dir(m3LedgerPath()), "mdec_gaps.txt")
	if os.Getenv("PSGOLEM_WRITE_LEDGER") == "1" {
		writeM3Ledger(t, ledger, got, "# MDEC 驗收台帳（machine/mdec_test.go）：測試<TAB>不符數。VRAM 為不符像素數，TTY 為差異行數。\n"+
			"# 每一列不為 0 的原因見 docs/spec/011-mdec.md §7。\n")
		return
	}
	known := readM3Ledger(t, ledger)
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-32s 不符 %d（台帳 %d）", k, got[k], known[k])
		if got[k] != known[k] {
			t.Errorf("%s：不符 %d，台帳記 %d", k, got[k], known[k])
		}
	}
}
