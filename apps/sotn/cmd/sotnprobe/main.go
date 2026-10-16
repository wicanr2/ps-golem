// sotnprobe 是找 RAM 位址用的探針（docs/spec/015-sotn-autoplay.md §3）：
// 依腳本跑到某個 field，從那裡複製出幾份機器，各自按不同的鍵跑一段，比較 RAM。
//
//	sotnprobe -bios B -cue C -card K -script S -at F -variant "RIGHT:60" -variant "-:60" [-shots dir]
//
// 輸出各份之間值不同的 16 位元位址（只比 0x00000–0x1FFFFF 主記憶體）。
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/ps-golem/apps/sotn"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, " ") }
func (m *multi) Set(s string) error { *m = append(*m, s); return nil }

func main() {
	bios := flag.String("bios", "", "BIOS")
	cue := flag.String("cue", "", "cue")
	cardPath := flag.String("card", "", "記憶卡（唯讀，不寫回）")
	script := flag.String("script", "", "psexe -press 格式的腳本")
	at := flag.Uint64("at", 0, "分岔的 field")
	shots := flag.String("shots", "", "存各份最後畫面的目錄")
	maxOut := flag.Int("max", 200, "最多列幾個位址")
	var variants multi
	flag.Var(&variants, "variant", "BTN[+BTN]:N[,BTN:N...]，可重複")
	flag.Parse()

	var card []byte
	if *cardPath != "" {
		b, err := os.ReadFile(*cardPath)
		if err != nil {
			fail(err)
		}
		card = b
	}
	ps, err := sotn.ParseScript(*script)
	if err != nil {
		fail(err)
	}
	m, err := sotn.Boot(*bios, *cue, card)
	if err != nil {
		fail(err)
	}
	sotn.RunScript(m, ps, *at)
	if *shots != "" {
		_ = os.MkdirAll(*shots, 0o755)
		_ = sotn.SavePNG(m, filepath.Join(*shots, "base.png"))
	}
	base := m.ReadRAM(0, 2<<20)
	rams := [][]byte{}
	for i, v := range variants {
		c := m.Clone()
		for _, seg := range strings.Split(v, ",") {
			f := strings.Split(seg, ":")
			b, err := sotn.ParseButtons(f[0])
			if err != nil {
				fail(err)
			}
			n, err := strconv.Atoi(f[1])
			if err != nil {
				fail(err)
			}
			sotn.Hold(c, b, n)
		}
		rams = append(rams, c.ReadRAM(0, 2<<20))
		if *shots != "" {
			_ = sotn.SavePNG(c, filepath.Join(*shots, fmt.Sprintf("v%d.png", i)))
		}
		fmt.Fprintf(os.Stderr, "variant %d %q field=%d\n", i, v, c.Field())
	}
	printed := 0
	for a := 0; a < 2<<20 && printed < *maxOut; a += 2 {
		v0 := binary.LittleEndian.Uint16(rams[0][a:])
		diff := false
		for _, r := range rams[1:] {
			if binary.LittleEndian.Uint16(r[a:]) != v0 {
				diff = true
			}
		}
		if !diff {
			continue
		}
		fmt.Printf("%06x base=%5d", a, int16(binary.LittleEndian.Uint16(base[a:])))
		for _, r := range rams {
			fmt.Printf(" %6d", int16(binary.LittleEndian.Uint16(r[a:])))
		}
		fmt.Println()
		printed++
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sotnprobe:", err)
	os.Exit(1)
}
