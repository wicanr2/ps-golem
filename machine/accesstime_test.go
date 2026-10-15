package machine

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// accessTimeKnownGaps 是已知超出容差的格子（008 §5）。變成相符也算失敗，請更新這裡與規格。
//
// SPUCNT/32：位址未對齊，實際是 lwl／lwr 兩次 16 位元 SPU 讀取；實機比兩次單獨讀取多約 3 週期，原因未知。
var accessTimeKnownGaps = map[string]bool{
	"SPUCNT/32": true,
}

// ps1-tests cpu/access-time：每格與實機表值差距在 1 週期以內（docs/spec/008-cycle-model.md §2.5）。
func TestAccessTime(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	m := bootSnapshot(t, bios).Clone()
	mark := len(m.tty)
	if err := m.SideloadEXE(files["cpu/access-time/access-time.exe"]); err != nil {
		t.Fatal(err)
	}
	m.Run(400_000_000)
	got := parseAccessTime(string(m.tty[mark:]))
	want := parseAccessTime(string(files["cpu/access-time/psx.log"]))
	if len(want) == 0 {
		t.Fatal("期望值解析不到任何格子")
	}
	bad := 0
	for _, k := range sortedKeys(want) {
		g, ok := got[k]
		if !ok {
			t.Errorf("%s：實得沒有這一格", k)
			bad++
			continue
		}
		d := math.Abs(g - want[k])
		mark := ""
		switch {
		case d > 1.0 && accessTimeKnownGaps[k]:
			mark = "  ← 已知差異"
		case d > 1.0:
			mark = "  ← 超過 1 週期"
			bad++
		case accessTimeKnownGaps[k]:
			mark = "  ← 已知差異但已相符，請更新"
			bad++
		}
		t.Logf("%-18s 期望 %6.2f 實得 %6.2f%s", k, want[k], g, mark)
	}
	if bad > 0 {
		t.Errorf("%d 格超過容差", bad)
	}
}

var reAccessRow = regexp.MustCompile(`^(\S+)\s+\(0x[0-9a-f]+\)\s+(.*)$`)

// parseAccessTime 解析表格。程式用 printf("%2d.%2-d", n/100, n%100)，所以 "5.3" 是 5.03。
func parseAccessTime(s string) map[string]float64 {
	out := map[string]float64{}
	for _, line := range normalizeTTY(s) {
		m := reAccessRow.FindStringSubmatch(line)
		if m == nil || m[1] == "SEGMENT" {
			continue
		}
		for i, cell := range strings.Fields(m[2]) {
			parts := strings.SplitN(cell, ".", 2)
			if len(parts) != 2 {
				continue
			}
			ip, err1 := strconv.Atoi(parts[0])
			fp, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				continue
			}
			out[m[1]+"/"+[]string{"8", "16", "32"}[i]] = float64(ip) + float64(fp)/100
		}
	}
	return out
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}
