package sotn

import (
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
	RunScript(m, ps, 45784)
	if got, want := m.Digest(), "6c35c2701228722f"; !strings.HasSuffix(got, want) {
		t.Errorf("摘要 %s，應以 %s 結尾", got, want)
	}
	// 記憶卡第 1 個目錄項：使用中的第一個區塊，檔名 BISLPM-86023DRAX00。
	dir := m.Card1.Data[0x80:0x100]
	if dir[0] != 0x51 || string(dir[0x0A:0x1C]) != "BISLPM-86023DRAX00" {
		t.Errorf("記憶卡目錄項不對：% x", dir[:0x20])
	}
}
