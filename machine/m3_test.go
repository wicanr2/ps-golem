package machine

import (
	"bufio"
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// M3 驗收（docs/spec/007-gpu-and-dma.md §2）：VRAM 測試逐像素比對 vram.png，
// TTY 測試比對 psx.log。每個測試的不符數記在台帳 m3_gaps.txt；
// 實得數與台帳不同（變好或變差）都算失敗。重寫台帳：PSGOLEM_WRITE_LEDGER=1。

var m3VRAMTests = []string{
	"gpu/rectangles", "gpu/lines", "gpu/triangle", "gpu/quad", "gpu/transparency",
	"gpu/texture-flip", "gpu/texture-overflow", "gpu/clut-cache", "gpu/clipping",
	"gpu/uv-interpolation", "gpu/vram-to-vram-overlap",
}

var m3TTYTests = []string{
	"gpu/gp0-e1", "gpu/mask-bit",
	"dma/otc-test", "dma/dpcr", "dma/chain-looping", "dma/chopping",
}

// vramCompareRegion 是只比對部分 VRAM 的測試與理由（007 §2.1）。
//
// gpu/transparency：vram.png 在 ps1-tests commit da6b472 錄下，那一版的 clearScreen()
// 把整個 VRAM 清成白色；2fe1570 重構後改成 clearScreen() 清成黑色，而 tests.zip 是之後建的。
// 程式只畫 320×240，範圍外的差異只反映兩版清除顏色不同。
var vramCompareRegion = map[string][2]int{
	"gpu/transparency": {320, 240},
}

// m3Cycles 是側載後執行的週期數（007 §2.1）。
const m3Cycles = 200_000_000

func TestM3(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	boot := bootSnapshot(t, bios)

	got := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	run := func(path string, fn func(m *Machine) int) {
		defer wg.Done()
		base := path[strings.LastIndex(path, "/")+1:]
		exe := files[path+"/"+base+".exe"]
		if exe == nil {
			t.Errorf("tests.zip 裡缺 %s", path)
			return
		}
		m := boot.Clone()
		mark := len(m.TTY())
		if err := m.SideloadEXE(exe); err != nil {
			t.Errorf("%s：%v", path, err)
			return
		}
		m.Run(m3Cycles)
		m.tty = m.tty[mark:]
		n := fn(m)
		mu.Lock()
		got[path] = n
		mu.Unlock()
	}
	for _, path := range m3VRAMTests {
		want := files[path+"/vram.png"]
		wg.Add(1)
		go run(path, func(m *Machine) int {
			region := [2]int{1024, 512}
			if r, ok := vramCompareRegion[path]; ok {
				region = r
			}
			n, err := vramMismatch(m.VRAM(), want, region[0], region[1])
			if err != nil {
				t.Errorf("%s：%v", path, err)
			}
			if dir := os.Getenv("PSGOLEM_DUMP_VRAM"); dir != "" {
				_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(path, "/", "_")+".png"), VRAMPNG(m.VRAM()), 0o644)
			}
			return n
		})
	}
	for _, path := range m3TTYTests {
		want := string(files[path+"/psx.log"])
		wg.Add(1)
		go run(path, func(m *Machine) int {
			n := ttyMismatch(m.TTY(), want)
			if n != 0 {
				t.Logf("%s 實得 TTY：\n%s", path, m.TTY())
			}
			return n
		})
	}
	wg.Wait()

	ledger := m3LedgerPath()
	if os.Getenv("PSGOLEM_WRITE_LEDGER") == "1" {
		writeM3Ledger(t, ledger, got)
		return
	}
	known := readM3Ledger(t, ledger)
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-28s 不符 %d（台帳 %d）", k, got[k], known[k])
		if got[k] != known[k] {
			t.Errorf("%s：不符 %d，台帳記 %d", k, got[k], known[k])
		}
	}
}

var (
	bootOnce sync.Map // BIOS 雜湊 → *Machine
)

// bootSnapshot 開機到 shell 一次，之後各測試用 Clone 起跑。
func bootSnapshot(t *testing.T, bios []byte) *Machine {
	t.Helper()
	key := fmt.Sprintf("%x", bios[0x7FF32:0x7FF52])
	if v, ok := bootOnce.Load(key); ok {
		return v.(*Machine)
	}
	m, err := New(bios)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RunUntilPC(ShellEntry, 400_000_000); err != nil {
		t.Fatal(err)
	}
	bootOnce.Store(key, m)
	return m
}

// vramMismatch 依 007 §2.1：VRAM 每像素 r5<<3、g5<<3、b5<<3，與 PNG 逐像素完全相等。
// 只比對左上 w×h。
func vramMismatch(vram []uint16, pngData []byte, w, h int) (int, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return -1, err
	}
	b := img.Bounds()
	if b.Dx() != 1024 || b.Dy() != 512 {
		return -1, fmt.Errorf("vram.png 尺寸 %dx%d", b.Dx(), b.Dy())
	}
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			p := vram[y*1024+x]
			if c.R != uint8(p&31)<<3 || c.G != uint8((p>>5)&31)<<3 || c.B != uint8((p>>10)&31)<<3 {
				n++
			}
		}
	}
	return n, nil
}

// ttyMismatch 依 006 §7 的正規化，從期望第一行開始比，回傳差異行數（增加加刪除，以最長共同子序列計）。
func ttyMismatch(got, want string) int {
	w := normalizeTTY(want)
	g := normalizeTTY(got)
	if len(w) == 0 {
		return -1
	}
	start := -1
	for i, l := range g {
		if l == w[0] {
			start = i
			break
		}
	}
	if start < 0 {
		return len(w)
	}
	g = g[start:]
	return len(w) + len(g) - 2*lcs(w, g)
}

func lcs(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			switch {
			case a[i-1] == b[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func m3LedgerPath() string {
	_, self, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(self), "m3_gaps.txt")
}

func readM3Ledger(t *testing.T, path string) map[string]int {
	t.Helper()
	out := map[string]int{}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("讀不到台帳 %s：%v", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		parts := strings.Split(l, "\t")
		n, _ := strconv.Atoi(parts[1])
		out[parts[0]] = n
	}
	return out
}

func writeM3Ledger(t *testing.T, path string, got map[string]int) {
	t.Helper()
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# M3 驗收台帳（machine/m3_test.go）：測試<TAB>不符數。VRAM 為不符像素數，TTY 為差異行數。\n")
	b.WriteString("# 每一列不為 0 的原因見 docs/spec/007-gpu-and-dma.md §6。\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s\t%d\n", k, got[k])
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("台帳寫入 %s", path)
}
