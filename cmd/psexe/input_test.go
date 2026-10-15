package main

import (
	"testing"

	"github.com/wicanr2/ps-golem/internal/sio"
)

func TestParsePresses(t *testing.T) {
	ps, err := parsePresses("100:start, 10:Cross+UP:2")
	if err != nil {
		t.Fatal(err)
	}
	if buttonsAt(ps, 9) != 0 || buttonsAt(ps, 10) != sio.BtnCross|sio.BtnUp || buttonsAt(ps, 12) != 0 {
		t.Error("CROSS+UP 的時間範圍不對")
	}
	if buttonsAt(ps, 104) != sio.BtnStart || buttonsAt(ps, 105) != 0 {
		t.Error("START 預設應按 5 個 field")
	}
	if _, err := parsePresses("5:JUMP"); err == nil {
		t.Error("不認識的按鍵應回錯誤")
	}
}
