package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/ps-golem/internal/sio"
)

var buttonNames = map[string]uint16{
	"SELECT": sio.BtnSelect, "START": sio.BtnStart,
	"UP": sio.BtnUp, "RIGHT": sio.BtnRight, "DOWN": sio.BtnDown, "LEFT": sio.BtnLeft,
	"L1": sio.BtnL1, "R1": sio.BtnR1, "L2": sio.BtnL2, "R2": sio.BtnR2,
	"TRIANGLE": sio.BtnTriangle, "CIRCLE": sio.BtnCircle, "CROSS": sio.BtnCross, "SQUARE": sio.BtnSquare,
}

// press 是一次按鍵：從第 at 個 field 開始按住 buttons，持續 dur 個 field。
type press struct {
	at, dur uint64
	buttons uint16
}

// parsePresses 解析 "FIELD:BTN[+BTN][:DUR],..."，DUR 預設 5 個 field。
func parsePresses(s string) ([]press, error) {
	var out []press
	if s == "" {
		return nil, nil
	}
	for _, item := range strings.Split(s, ",") {
		f := strings.Split(strings.TrimSpace(item), ":")
		if len(f) < 2 {
			return nil, fmt.Errorf("按鍵格式不對：%q（應為 FIELD:BTN[:DUR]）", item)
		}
		at, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			return nil, err
		}
		p := press{at: at, dur: 5}
		for _, n := range strings.Split(f[1], "+") {
			b, ok := buttonNames[strings.ToUpper(n)]
			if !ok {
				return nil, fmt.Errorf("不認識的按鍵 %q", n)
			}
			p.buttons |= b
		}
		if len(f) > 2 {
			if p.dur, err = strconv.ParseUint(f[2], 10, 64); err != nil {
				return nil, err
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out, nil
}

// buttonsAt 回傳第 f 個 field 應按住的鍵。
func buttonsAt(ps []press, f uint64) uint16 {
	var b uint16
	for _, p := range ps {
		if f >= p.at && f < p.at+p.dur {
			b |= p.buttons
		}
	}
	return b
}
