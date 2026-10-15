package gte

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// FuzzCase 是 ps1-tests gte-fuzz 紀錄裡的一筆：依序寫入 In、執行 Cmd（HasCmd 為假時不執行），
// 再依序讀回，期望得到 Out。格式與產生程序見 docs/spec/003-gte.md §3.1。
type FuzzCase struct {
	Number int
	Line   int // "Test N" 所在行號，失敗時方便回頭查紀錄
	Name   string
	In     [64]uint32
	Out    [64]uint32
	HasCmd bool
	Cmd    uint32
}

var (
	reTest = regexp.MustCompile(`^Test (\d+)$`)
	reReg  = regexp.MustCompile(`^([<>]) r\[(\d+)\] = 0x([0-9a-f]{8})$`)
	reCmd  = regexp.MustCompile(`^GTE 0x([0-9a-f]{2}) (\S+) \(sf=(\d), lm=(\d), tx=(\d), vx=(\d), mx=(\d)\)$`)
)

// CommandWord 依 gte-fuzz 的 gte.h／gte.s 組出命令字。
func CommandWord(op, sf, lm, tx, vx, mx uint32) uint32 {
	c := op&0x3F | lm<<10 | tx<<13 | vx<<15 | mx<<17 | sf<<19
	return c&0x1FFFFFF | 0x4A000000
}

// ParseFuzzLog 解析整份紀錄。每筆都必須有 64 個寫入值與 64 個讀回值，否則回錯。
func ParseFuzzLog(r io.Reader) ([]FuzzCase, error) {
	var cases []FuzzCase
	var cur *FuzzCase
	var nIn, nOut int
	finish := func() error {
		if cur == nil {
			return nil
		}
		if nIn != 64 || nOut != 64 {
			return fmt.Errorf("Test %d（第 %d 行）：寫入 %d 個、讀回 %d 個，應各 64 個", cur.Number, cur.Line, nIn, nOut)
		}
		if cur.Name == "" {
			cur.Name = "NOP"
		}
		cases = append(cases, *cur)
		cur = nil
		return nil
	}

	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimRight(sc.Text(), "\r")
		if m := reTest.FindStringSubmatch(s); m != nil {
			if err := finish(); err != nil {
				return nil, err
			}
			n, _ := strconv.Atoi(m[1])
			cur = &FuzzCase{Number: n, Line: line}
			nIn, nOut = 0, 0
			continue
		}
		if cur == nil {
			continue
		}
		if m := reReg.FindStringSubmatch(s); m != nil {
			i, _ := strconv.Atoi(m[2])
			v, _ := strconv.ParseUint(m[3], 16, 32)
			if i > 63 {
				return nil, fmt.Errorf("第 %d 行：暫存器編號 %d 超出範圍", line, i)
			}
			if m[1] == ">" {
				cur.In[i] = uint32(v)
				nIn++
			} else {
				cur.Out[i] = uint32(v)
				nOut++
			}
			continue
		}
		if m := reCmd.FindStringSubmatch(s); m != nil {
			var f [6]uint32
			op, _ := strconv.ParseUint(m[1], 16, 8)
			for k := 0; k < 5; k++ {
				x, _ := strconv.ParseUint(m[3+k], 10, 8)
				f[k] = uint32(x)
			}
			cur.HasCmd = true
			cur.Name = m[2]
			cur.Cmd = CommandWord(uint32(op), f[0], f[1], f[2], f[3], f[4])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return cases, nil
}

// Run 對一筆紀錄重跑一次，回傳讀回的 64 個值。
func (c *FuzzCase) Run() [64]uint32 {
	var g GTE
	for i, v := range c.In {
		g.Write(i, v)
	}
	if c.HasCmd {
		g.Execute(c.Cmd)
	}
	var out [64]uint32
	for i := range out {
		out[i] = g.Read(i)
	}
	return out
}
