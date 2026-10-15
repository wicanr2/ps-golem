package machine

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 驗收：ps1-tests cpu/cop、cpu/code-in-io、gte/test-all 的 TTY 與實機 psx.log 正規化後相同（docs/spec/006-minimal-machine.md §7）。
// 需要 PSGOLEM_BIOS 指到 BIOS 檔（容器內路徑），缺就 skip。
func TestPS1TestsCPU(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	for _, path := range []string{"cpu/cop", "cpu/code-in-io", "gte/test-all"} {
		t.Run(path, func(t *testing.T) {
			base := path[strings.LastIndex(path, "/")+1:]
			exe := files[path+"/"+base+".exe"]
			want := files[path+"/psx.log"]
			if exe == nil || want == nil {
				t.Fatalf("tests.zip 裡缺 %s", path)
			}
			got := runEXE(t, bios, exe, 100_000_000)
			compareTTY(t, dropLogDrift(path, got), string(want))
		})
	}
}

// logDrift 是「程式比實機紀錄新」造成的多餘行，只允許完全相同的字串（006 §7 第 5 條）。
//
// gte/test-all：psx.log 在 commit 8544f93 錄下；"Total tests" 這行到 3c210f4 才加進 tests.c，
// 而 tests.zip（release build-158）是之後建的。
var logDrift = map[string][]string{
	"gte/test-all": {"Total tests: 1150"},
}

func dropLogDrift(path, got string) string {
	for _, l := range logDrift[path] {
		got = strings.Replace(got, l+"\r\n", "", 1)
		got = strings.Replace(got, l+"\n", "", 1)
	}
	return got
}

func runEXE(t *testing.T, bios, exe []byte, cycles uint64) string {
	t.Helper()
	m, err := New(bios)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RunUntilPC(ShellEntry, 400_000_000); err != nil {
		t.Fatal(err)
	}
	mark := len(m.TTY())
	if err := m.SideloadEXE(exe); err != nil {
		t.Fatal(err)
	}
	m.Run(cycles)
	return m.TTY()[mark:]
}

// normalizeTTY 套用 006 §7 的正規化：CRLF→LF、去掉行首 "% "、去掉空行。
func normalizeTTY(s string) []string {
	s = strings.ReplaceAll(s, "\r", "")
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimPrefix(l, "% ")
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// compareTTY：實得輸出從期望值第一行出現處開始比（006 §7 第 4 條），之後逐行相同、行數相同。
func compareTTY(t *testing.T, got, want string) {
	t.Helper()
	w := normalizeTTY(want)
	g := normalizeTTY(got)
	if len(w) == 0 {
		t.Fatal("期望值是空的")
	}
	start := -1
	for i, l := range g {
		if l == w[0] {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("實得輸出裡找不到期望的第一行 %q。實得：\n%s", w[0], strings.Join(g, "\n"))
	}
	g = g[start:]
	n := max(len(g), len(w))
	bad := 0
	for i := 0; i < n; i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			bad++
			t.Errorf("第 %d 行\n  期望 %q\n  實得 %q", i+1, wl, gl)
		}
	}
	if bad == 0 {
		t.Logf("%d 行全部相同", len(w))
	}
}

func loadBIOS(t *testing.T) []byte {
	t.Helper()
	p := os.Getenv("PSGOLEM_BIOS")
	if p == "" {
		t.Skip("未設 PSGOLEM_BIOS")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("讀不到 BIOS：%v", err)
	}
	return b
}

func loadTestsZip(t *testing.T) map[string][]byte {
	t.Helper()
	dir := os.Getenv("PSGOLEM_PS1TESTS")
	if dir == "" {
		_, self, _, _ := runtime.Caller(0)
		dir = filepath.Join(filepath.Dir(self), "..", "testdata", "ps1-tests")
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "tests.zip"))
	if err != nil {
		t.Skipf("缺 tests.zip，先跑 tools/fetch_ps1tests.sh：%v", err)
	}
	defer zr.Close()
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b
	}
	return out
}
