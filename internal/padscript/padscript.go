// Package padscript 解析按鍵腳本 "FIELD:BTN[+BTN][:DUR],..."（docs/spec/010-pad-and-memcard.md）：
// 從第 FIELD 次 VBlank 起按住 BTN，持續 DUR 個 field（預設 5）。psexe、psgolem 與 apps 共用。
package padscript

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/ps-golem/internal/sio"
)

// Names 是按鍵名稱。
var Names = map[string]uint16{
	"SELECT": sio.BtnSelect, "START": sio.BtnStart,
	"UP": sio.BtnUp, "RIGHT": sio.BtnRight, "DOWN": sio.BtnDown, "LEFT": sio.BtnLeft,
	"L1": sio.BtnL1, "R1": sio.BtnR1, "L2": sio.BtnL2, "R2": sio.BtnR2,
	"TRIANGLE": sio.BtnTriangle, "CIRCLE": sio.BtnCircle, "CROSS": sio.BtnCross, "SQUARE": sio.BtnSquare,
}

// Press 是一項按鍵：從第 At 個 field 起按住 Buttons，持續 Dur 個 field。
type Press struct {
	At, Dur uint64
	Buttons uint16
}

// ParseButtons 解析 "RIGHT+CROSS"；空字串或 "-" 是不按。
func ParseButtons(s string) (uint16, error) {
	var b uint16
	if s == "" || s == "-" {
		return 0, nil
	}
	for _, n := range strings.Split(s, "+") {
		v, ok := Names[strings.ToUpper(n)]
		if !ok {
			return 0, fmt.Errorf("不認識的按鍵 %q", n)
		}
		b |= v
	}
	return b, nil
}

// FormatButtons 是 ParseButtons 的反向，名稱依字母排序。
func FormatButtons(b uint16) string {
	var names []string
	for n, v := range Names {
		if b&v != 0 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "+")
}

// Parse 解析整份腳本，依 At 排序。
func Parse(s string) ([]Press, error) {
	var out []Press
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	for _, item := range strings.Split(strings.TrimSpace(s), ",") {
		f := strings.Split(strings.TrimSpace(item), ":")
		if len(f) < 2 {
			return nil, fmt.Errorf("按鍵格式不對：%q（應為 FIELD:BTN[:DUR]）", item)
		}
		at, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			return nil, err
		}
		p := Press{At: at, Dur: 5}
		if p.Buttons, err = ParseButtons(f[1]); err != nil {
			return nil, err
		}
		if len(f) > 2 {
			if p.Dur, err = strconv.ParseUint(f[2], 10, 64); err != nil {
				return nil, err
			}
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out, nil
}

// At 回傳第 f 個 field 應按住的鍵。
func At(ps []Press, f uint64) uint16 {
	var b uint16
	for _, p := range ps {
		if p.At > f {
			break
		}
		if f < p.At+p.Dur {
			b |= p.Buttons
		}
	}
	return b
}
