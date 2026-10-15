package gte

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 驗收：ps1-tests gte-fuzz 的 1150 筆實機紀錄全部相符（docs/spec/003-gte.md §3.2）。
func TestHardwareFuzzLog(t *testing.T) {
	cases := loadFuzzLog(t)
	if len(cases) != 1150 {
		t.Fatalf("紀錄筆數 %d，應為 1150（語料被換掉了？）", len(cases))
	}

	byName := map[string][]FuzzCase{}
	var order []string
	for _, c := range cases {
		if _, ok := byName[c.Name]; !ok {
			order = append(order, c.Name)
		}
		byName[c.Name] = append(byName[c.Name], c)
	}

	for _, name := range order {
		t.Run(name, func(t *testing.T) {
			fails := 0
			for _, c := range byName[name] {
				got := c.Run()
				for i := range got {
					if got[i] == c.Out[i] {
						continue
					}
					fails++
					if fails <= 5 {
						t.Errorf("Test %d（紀錄第 %d 行）r[%d]：期望 0x%08x，實得 0x%08x",
							c.Number, c.Line, i, c.Out[i], got[i])
					}
				}
			}
			if fails > 5 {
				t.Errorf("另有 %d 個不符未列出", fails-5)
			}
		})
	}
}

func loadFuzzLog(t *testing.T) []FuzzCase {
	t.Helper()
	dir := os.Getenv("PSGOLEM_PS1TESTS")
	if dir == "" {
		_, self, _, _ := runtime.Caller(0)
		dir = filepath.Join(filepath.Dir(self), "..", "..", "testdata", "ps1-tests")
	}
	path := filepath.Join(dir, "gte-fuzz", "gte_valid_0xc0ffee_50.log")
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("缺語料 %s，先跑 tools/fetch_ps1tests.sh", path)
	}
	defer f.Close()
	cases, err := ParseFuzzLog(f)
	if err != nil {
		t.Fatal(err)
	}
	return cases
}
