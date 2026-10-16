// sotnroute 重播 psexe -press 腳本，記下每次換房間（015 §4 的 0x0730B0／B4）的 field，
// 並存一張進房間後 30 field 的截圖。
//
//	sotnroute -bios B -cue C -script S -until F -out DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/ps-golem/apps/sotn"
)

func main() {
	bios := flag.String("bios", "", "BIOS")
	cue := flag.String("cue", "", "cue")
	script := flag.String("script", "", "腳本")
	from := flag.Uint64("from", 0, "從這個 field 起記錄")
	until := flag.Uint64("until", 0, "跑到這個 field")
	out := flag.String("out", "", "截圖目錄")
	flag.Parse()
	ps, err := sotn.ParseScript(*script)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m, err := sotn.Boot(*bios, *cue, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.MkdirAll(*out, 0o755)
	lastL, lastT := -999, -999
	shotAt := uint64(0)
	for m.Field() < *until {
		sotn.Step(m, sotn.ButtonsAt(ps, m.Field()))
		if m.Field() < *from {
			continue
		}
		l, t := sotn.Room(m)
		if l != lastL || t != lastT {
			fmt.Printf("field=%d room=%d,%d hp=%d\n", m.Field(), l, t, sotn.PlayerHP(m))
			lastL, lastT = l, t
			shotAt = m.Field() + 30
		}
		if m.Field() == shotAt {
			_ = sotn.SavePNG(m, filepath.Join(*out, fmt.Sprintf("f%06d_%02d_%02d.png", m.Field(), l, t)))
		}
	}
}
