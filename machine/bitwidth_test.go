package machine

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// ps1-tests cpu/io-access-bitwidth 是週邊與匯流排的逐格 oracle（006 §7）。
// 每一格（讀取寬度／區段／寫入寬度）都與實機紀錄比對；已知不符的格子列在台帳
// bitwidth_gaps.txt，附負責的規格。台帳外的新不符、或台帳內已經修好的格子，都算失敗。
//
// 重寫台帳：PSGOLEM_WRITE_LEDGER=1（改完要看 git diff 確認每一行的變化）。
func TestPS1TestsIOBitwidth(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	const name = "io-access-bitwidth"
	exe := files["cpu/"+name+"/"+name+".exe"]
	want := files["cpu/"+name+"/psx.log"]
	if exe == nil || want == nil {
		t.Fatal("tests.zip 裡缺 cpu/io-access-bitwidth")
	}
	got := runEXE(t, bios, exe, 100_000_000)

	w := parseBitwidthTable(string(want))
	g := parseBitwidthTable(got)
	if len(w) == 0 {
		t.Fatal("期望值解析不到任何格子")
	}

	var mismatch []string
	for k, wv := range w {
		if bitwidthTimingCells[k] {
			continue
		}
		if g[k] != wv {
			mismatch = append(mismatch, k+"\t期望 "+wv+"\t實得 "+g[k])
		}
	}
	sort.Strings(mismatch)

	ledgerPath := bitwidthLedgerPath()
	if os.Getenv("PSGOLEM_WRITE_LEDGER") == "1" {
		writeLedger(t, ledgerPath, mismatch)
		return
	}
	known := readLedger(t, ledgerPath)

	seen := map[string]bool{}
	for _, line := range mismatch {
		key := strings.SplitN(line, "\t", 2)[0]
		seen[key] = true
		if _, ok := known[key]; !ok {
			t.Errorf("台帳外的新不符：%s", line)
		}
	}
	for key := range known {
		if !seen[key] {
			t.Errorf("台帳內的格子已經相符，請從台帳移除：%s", key)
		}
	}
	t.Logf("共 %d 格，相符 %d 格，台帳列 %d 格", len(w), len(w)-len(mismatch), len(known))
}

// bitwidthTimingCells 是結果取決於讀取當下週期的格子，週期模型規格完成前不比對。
// GPUSTAT bit 31 在 240 線模式每條掃描線翻轉（psx-spx GPU 章），32 位元讀取會讀到它；
// 本專案目前每條指令 1 週期（006 §3），讀取的掃描線相位與實機不同，
// 兩個 BIOS 開機後的相位也不同（SCPH5500 相符、SCPH-100 不符）。
var bitwidthTimingCells = map[string]bool{
	"r32/GPUSTAT/w8":  true,
	"r32/GPUSTAT/w16": true,
	"r32/GPUSTAT/w32": true,
}

var (
	reBWHeader = regexp.MustCompile(`^Reading as (\d+)bit`)
	reBWRow    = regexp.MustCompile(`^(\S+)\s+\(\s*(0x[0-9a-f]+)\)\s+(.*)$`)
)

// parseBitwidthTable 把輸出拆成 "r32/RAM/w8" → "0x78" 的格子。
// 欄位以兩個以上空白分隔；--CRASH-- 當成一個值。
func parseBitwidthTable(s string) map[string]string {
	out := map[string]string{}
	read := ""
	for _, line := range normalizeTTY(s) {
		if m := reBWHeader.FindStringSubmatch(line); m != nil {
			read = "r" + m[1]
			continue
		}
		m := reBWRow.FindStringSubmatch(line)
		if m == nil || read == "" || m[1] == "SEGMENT" {
			continue
		}
		cells := regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(m[3]), -1)
		widths := []string{"w8", "w16", "w32"}
		for i, c := range cells {
			if i < len(widths) && c != "" {
				out[read+"/"+m[1]+"/"+widths[i]] = c
			}
		}
	}
	return out
}

func bitwidthLedgerPath() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "bitwidth_gaps.txt")
}

// 台帳格式：每行「格子<TAB>期望<TAB>實得」；# 開頭為註解。負責的規格見 006 §7.1。
func readLedger(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("讀不到台帳 %s：%v", path, err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		parts := strings.SplitN(l, "\t", 2)
		out[parts[0]] = l
	}
	return out
}

func writeLedger(t *testing.T, path string, mismatch []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# ps1-tests cpu/io-access-bitwidth 與實機不符的格子（machine/bitwidth_test.go）。\n")
	b.WriteString("# 格子<TAB>期望<TAB>實得。修好就刪掉那一行；新增不符要先查原因再補。\n")
	for _, l := range mismatch {
		b.WriteString(l + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("台帳寫入 %d 行：%s", len(mismatch), path)
}
