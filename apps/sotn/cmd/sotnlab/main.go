// sotnlab 是常駐的實驗程式（docs/spec/015-sotn-autoplay.md §3）：開機跑完腳本後，從標準輸入讀指令，
// 操作同一台機器，結果寫到標準輸出。用來找 RAM 位址與試操作，不必每次重新開機。
//
// 指令（每行一個）：
//
//	hold BTN N          按住 BTN（"-" 為不按）跑 N 個 field，記進按鍵記錄
//	save NAME / load NAME   把目前機器複製到槽／從槽複製回來（含按鍵記錄）
//	shot PATH           存目前畫面
//	peek ADDR N         印出 ADDR 起 N 個 16 位元值（有號十進位）
//	snap NAME           記下目前 RAM
//	diff A B [MAX]      列出 RAM 快照 A、B 不同的 16 位元位址
//	filter EXPR ...     見 filterCmd
//	script              印出從開機以來的 -press 腳本
//	field               印出目前 field
//	dump PATH ADDR N    把 RAM 的 ADDR 起 N 位元組寫到檔案
//	writecard PATH      把記憶卡寫到檔案
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/ps-golem/apps/sotn"
	"github.com/wicanr2/ps-golem/machine"
)

type state struct {
	m   *machine.Machine
	log sotn.Log
}

func (s *state) clone() *state {
	return &state{m: s.m.Clone(), log: sotn.Log{Start: s.log.Start, Buttons: append([]uint16(nil), s.log.Buttons...)}}
}

func main() {
	bios := flag.String("bios", "", "BIOS")
	cue := flag.String("cue", "", "cue")
	cardPath := flag.String("card", "", "記憶卡（唯讀，不寫回）")
	script := flag.String("script", "", "psexe -press 格式的腳本")
	at := flag.Uint64("at", 0, "腳本跑到這個 field 後開始讀指令")
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
	cur := &state{m: m}
	for m.Field() < *at {
		b := sotn.ButtonsAt(ps, m.Field())
		cur.log.Buttons = append(cur.log.Buttons, b)
		sotn.Step(m, b)
	}
	slots := map[string]*state{}
	snaps := map[string][]byte{}
	fmt.Printf("ready field=%d\n", m.Field())
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		out, err := run(cur, f, slots, snaps)
		if err != nil {
			fmt.Printf("error %s: %v\n", f[0], err)
		} else if out != nil {
			cur = out
		}
		fmt.Printf("ok %s field=%d\n", f[0], cur.m.Field())
	}
}

func run(cur *state, f []string, slots map[string]*state, snaps map[string][]byte) (*state, error) {
	arg := func(i int) string {
		if i < len(f) {
			return f[i]
		}
		return ""
	}
	num := func(i int) (int, error) { v, err := strconv.ParseInt(arg(i), 0, 64); return int(v), err }
	switch f[0] {
	case "hold":
		b, err := sotn.ParseButtons(arg(1))
		if err != nil {
			return nil, err
		}
		n, err := num(2)
		if err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			cur.log.Buttons = append(cur.log.Buttons, b)
			sotn.Step(cur.m, b)
		}
	case "save":
		slots[arg(1)] = cur.clone()
	case "load":
		s, ok := slots[arg(1)]
		if !ok {
			return nil, fmt.Errorf("沒有槽 %s", arg(1))
		}
		return s.clone(), nil
	case "shot":
		return nil, sotn.SavePNG(cur.m, arg(1))
	case "peek":
		a, err := num(1)
		if err != nil {
			return nil, err
		}
		n, _ := num(2)
		if n == 0 {
			n = 1
		}
		b := cur.m.ReadRAM(uint32(a)&0x1FFFFF, 2*n)
		var vals []string
		for i := 0; i < n; i++ {
			vals = append(vals, strconv.Itoa(int(int16(binary.LittleEndian.Uint16(b[2*i:])))))
		}
		fmt.Printf("peek %06x %s\n", a&0x1FFFFF, strings.Join(vals, " "))
	case "snap":
		snaps[arg(1)] = cur.m.ReadRAM(0, 2<<20)
	case "diff":
		a, b := snaps[arg(1)], snaps[arg(2)]
		if a == nil || b == nil {
			return nil, fmt.Errorf("沒有快照")
		}
		max, _ := num(3)
		if max == 0 {
			max = 100
		}
		n := 0
		for i := 0; i < len(a) && n < max; i += 2 {
			va, vb := int16(binary.LittleEndian.Uint16(a[i:])), int16(binary.LittleEndian.Uint16(b[i:]))
			if va != vb {
				fmt.Printf("d %06x %d %d\n", i, va, vb)
				n++
			}
		}
	case "filter":
		return nil, filterCmd(f[1:], snaps)
	case "script":
		fmt.Printf("script %s\n", cur.log.Script())
	case "field":
	case "dump":
		a, err := num(2)
		if err != nil {
			return nil, err
		}
		n, err := num(3)
		if err != nil {
			return nil, err
		}
		return nil, os.WriteFile(arg(1), cur.m.ReadRAM(uint32(a)&0x1FFFFF, n), 0o644)
	case "writecard":
		return nil, os.WriteFile(arg(1), cur.m.Card1.Data[:], 0o644)
	default:
		return nil, fmt.Errorf("不認識的指令")
	}
	return nil, nil
}

// filterCmd：filter S1 OP S2 [S3 OP S4 ...] — 列出同時滿足所有關係的 16 位元位址（有號比較）。
// OP 為 < > = !=。例：filter a = b b < c c < d
func filterCmd(f []string, snaps map[string][]byte) error {
	if len(f)%3 != 0 || len(f) == 0 {
		return fmt.Errorf("參數要是 S OP S 的組合")
	}
	type rel struct {
		a, b []byte
		op   string
	}
	var rels []rel
	for i := 0; i < len(f); i += 3 {
		a, b := snaps[f[i]], snaps[f[i+2]]
		if a == nil || b == nil {
			return fmt.Errorf("沒有快照 %s 或 %s", f[i], f[i+2])
		}
		rels = append(rels, rel{a, b, f[i+1]})
	}
	n := 0
	for i := 0; i < 2<<20 && n < 200; i += 2 {
		ok := true
		for _, r := range rels {
			va, vb := int16(binary.LittleEndian.Uint16(r.a[i:])), int16(binary.LittleEndian.Uint16(r.b[i:]))
			switch r.op {
			case "<":
				ok = va < vb
			case ">":
				ok = va > vb
			case "=":
				ok = va == vb
			case "!=":
				ok = va != vb
			default:
				return fmt.Errorf("不認識的關係 %s", r.op)
			}
			if !ok {
				break
			}
		}
		if ok {
			var vals []string
			for _, r := range rels {
				vals = append(vals, strconv.Itoa(int(int16(binary.LittleEndian.Uint16(r.a[i:])))))
			}
			fmt.Printf("f %06x %s\n", i, strings.Join(vals, " "))
			n++
		}
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sotnlab:", err)
	os.Exit(1)
}
