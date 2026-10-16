// Package sotn 是《惡魔城X：月下的夜想曲》中文版（SLPM_860.23）專屬的自動遊玩與觀測工具
// （docs/spec/015-sotn-autoplay.md）。換一片光碟就不成立的東西都放這裡。
package sotn

import (
	"fmt"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/sio"
	"github.com/wicanr2/ps-golem/machine"
)

// fieldCycleLimit 是跑一個 field 的週期上限（約 2 個 NTSC field），顯示關閉時不會卡住。
const fieldCycleLimit = 1_200_000

// Boot 開機並放入光碟與記憶卡（card 為 nil 用空白卡）。
func Boot(biosPath, cuePath string, card []byte) (*machine.Machine, error) {
	bios, err := os.ReadFile(biosPath)
	if err != nil {
		return nil, err
	}
	m, err := machine.New(bios)
	if err != nil {
		return nil, err
	}
	d, err := disc.Open(cuePath)
	if err != nil {
		return nil, err
	}
	m.InsertDisc(d)
	if card != nil && !m.Card1.Load(card) {
		return nil, fmt.Errorf("記憶卡不是 128 KB")
	}
	return m, nil
}

// Step 按住 buttons 跑到下一次 VBlank，保證 Field 剛好前進 1，與 psexe 的 -press 語意相同：
// 按鍵記錄的第 i 筆就是第 Start+i 個 field 的手把狀態。顯示關閉時 VBlank 仍會來，
// 上限只防止機器卡死（約 1 秒）。
func Step(m *machine.Machine, buttons uint16) {
	m.Pad1.Buttons = buttons
	f := m.Field() + 1
	for i := 0; m.Field() < f; i++ {
		if i == 30 {
			panic(fmt.Sprintf("field %d 跑了 30 次上限仍沒有 VBlank", f))
		}
		m.RunUntilField(f, fieldCycleLimit)
	}
}

// Hold 按住 buttons 跑 n 個 field。
func Hold(m *machine.Machine, buttons uint16, n int) {
	for i := 0; i < n; i++ {
		Step(m, buttons)
	}
}

// SavePNG 存下目前顯示的畫面。
func SavePNG(m *machine.Machine, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, m.GPU.Screen())
}

// ButtonNames 與 psexe 的 -press 相同。
var ButtonNames = map[string]uint16{
	"SELECT": sio.BtnSelect, "START": sio.BtnStart,
	"UP": sio.BtnUp, "RIGHT": sio.BtnRight, "DOWN": sio.BtnDown, "LEFT": sio.BtnLeft,
	"L1": sio.BtnL1, "R1": sio.BtnR1, "L2": sio.BtnL2, "R2": sio.BtnR2,
	"TRIANGLE": sio.BtnTriangle, "CIRCLE": sio.BtnCircle, "CROSS": sio.BtnCross, "SQUARE": sio.BtnSquare,
}

// ParseButtons 解析 "RIGHT+CROSS"；空字串是不按。
func ParseButtons(s string) (uint16, error) {
	var b uint16
	if s == "" || s == "-" {
		return 0, nil
	}
	for _, n := range strings.Split(s, "+") {
		v, ok := ButtonNames[strings.ToUpper(n)]
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
	for n, v := range ButtonNames {
		if b&v != 0 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "+")
}

// Log 記錄每個 field 按住的鍵（從 Start 開始），可以轉成 psexe 的 -press 腳本。
type Log struct {
	Start   uint64
	Buttons []uint16
}

// Press 是 psexe -press 的一項。
type Press struct {
	At, Dur uint64
	Buttons uint16
}

// ParseScript 解析 psexe 的 "FIELD:BTN[+BTN][:DUR],..."（DUR 預設 5）。
func ParseScript(s string) ([]Press, error) {
	var out []Press
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	for _, item := range strings.Split(s, ",") {
		f := strings.Split(strings.TrimSpace(item), ":")
		if len(f) < 2 {
			return nil, fmt.Errorf("按鍵格式不對：%q", item)
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
	return out, nil
}

// ButtonsAt 回傳腳本在第 f 個 field 按住的鍵。
func ButtonsAt(ps []Press, f uint64) uint16 {
	var b uint16
	for _, p := range ps {
		if f >= p.At && f < p.At+p.Dur {
			b |= p.Buttons
		}
	}
	return b
}

// RunScript 依腳本跑到第 until 個 field。
func RunScript(m *machine.Machine, ps []Press, until uint64) {
	for m.Field() < until {
		Step(m, ButtonsAt(ps, m.Field()))
	}
}

// Script 把記錄轉成 -press 腳本：連續相同的非零按鍵合成一項。
func (l *Log) Script() string {
	var items []string
	for i := 0; i < len(l.Buttons); {
		b := l.Buttons[i]
		j := i + 1
		for j < len(l.Buttons) && l.Buttons[j] == b {
			j++
		}
		if b != 0 {
			items = append(items, fmt.Sprintf("%d:%s:%d", l.Start+uint64(i), FormatButtons(b), j-i))
		}
		i = j
	}
	return strings.Join(items, ",")
}
