package padscript

import (
	"testing"

	"github.com/wicanr2/ps-golem/internal/sio"
)

func TestParseAndAt(t *testing.T) {
	ps, err := Parse("10:RIGHT+CROSS:3,5:START")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[uint64]uint16{4: 0, 5: sio.BtnStart, 9: sio.BtnStart, 10: sio.BtnRight | sio.BtnCross, 12: sio.BtnRight | sio.BtnCross, 13: 0}
	for f, want := range cases {
		if got := At(ps, f); got != want {
			t.Errorf("At(%d) = %04x，應為 %04x", f, got, want)
		}
	}
	if s := FormatButtons(sio.BtnRight | sio.BtnCross); s != "CROSS+RIGHT" {
		t.Errorf("FormatButtons = %q", s)
	}
	if _, err := Parse("1:JUMP"); err == nil {
		t.Error("不認識的按鍵應該報錯")
	}
}
