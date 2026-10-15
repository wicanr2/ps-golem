package sio

import "testing"

// xfer 模擬主機傳一個位元組：逐位元交給裝置，回傳裝置送回的位元組與最後的 /ACK 延遲。
func xfer(d Device, b uint8) (uint8, int64) {
	var r uint8
	var ack int64
	for i := uint(0); i < 8; i++ {
		bit, delay := d.Clock(b>>i&1 != 0)
		if bit {
			r |= 1 << i
		}
		ack = delay
	}
	return r, ack
}

func TestMemcardWriteThenRead(t *testing.T) {
	c := NewMemcard()
	var frame [128]byte
	for i := range frame {
		frame[i] = uint8(i*7 + 3)
	}
	const addr = 0x0123

	// 寫入：81 57 00 00 MSB LSB data[128] xor 00 00 00
	c.SetDTR(true)
	xfer(c, 0x81)
	if r, _ := xfer(c, 'W'); r != 0x08 {
		t.Fatalf("新卡的旗標 %02x，應為 08", r)
	}
	xfer(c, 0)
	xfer(c, 0)
	xfer(c, addr>>8)
	xfer(c, addr&0xFF)
	x := uint8(addr>>8) ^ uint8(addr&0xFF)
	for _, v := range frame {
		xfer(c, v)
		x ^= v
	}
	xfer(c, x)
	r1, _ := xfer(c, 0)
	r2, _ := xfer(c, 0)
	end, _ := xfer(c, 0)
	if r1 != '\\' || r2 != ']' || end != 'G' {
		t.Fatalf("寫入結尾 %q %q %q，應為 \\ ] G", r1, r2, end)
	}
	c.SetDTR(false)
	if !c.Dirty {
		t.Error("寫入後應標記 Dirty")
	}

	// 讀取：81 52 00 00 MSB LSB 00×4 → 128 位元組、xor、'G'
	c.SetDTR(true)
	xfer(c, 0x81)
	if r, _ := xfer(c, 'R'); r != 0x00 {
		t.Fatalf("寫過之後旗標 %02x，應為 00", r)
	}
	if r, _ := xfer(c, 0); r != 0x5A {
		t.Fatalf("ID1 %02x，應為 5A", r)
	}
	if r, _ := xfer(c, 0); r != 0x5D {
		t.Fatalf("ID2 %02x，應為 5D", r)
	}
	xfer(c, addr>>8)
	xfer(c, addr&0xFF)
	xfer(c, 0) // '\'
	xfer(c, 0) // ']'
	hi, _ := xfer(c, 0)
	lo, _ := xfer(c, 0)
	if uint16(hi)<<8|uint16(lo) != addr {
		t.Fatalf("回傳位址 %02x%02x，應為 %04x", hi, lo, addr)
	}
	sum := hi ^ lo
	for i := range frame {
		v, ack := xfer(c, 0)
		if v != frame[i] {
			t.Fatalf("第 %d 位元組 %02x，應為 %02x", i, v, frame[i])
		}
		if ack == 0 {
			t.Fatalf("第 %d 位元組後沒有 /ACK", i)
		}
		sum ^= v
	}
	if v, _ := xfer(c, 0); v != sum {
		t.Errorf("校驗 %02x，應為 %02x", v, sum)
	}
	if v, _ := xfer(c, 0); v != 'G' {
		t.Errorf("結尾 %q，應為 G", v)
	}
}

func TestMemcardFormatted(t *testing.T) {
	c := NewMemcard()
	if c.Data[0] != 'M' || c.Data[1] != 'C' {
		t.Error("header frame 應以 MC 開頭")
	}
	// 每個 frame 的最後一位元組是前 127 位元組的 XOR（header 與目錄 frame）。
	for f := 0; f < 16; f++ {
		var x uint8
		for i := 0; i < 127; i++ {
			x ^= c.Data[f*128+i]
		}
		if x != c.Data[f*128+127] {
			t.Errorf("frame %d 校驗 %02x，存的是 %02x", f, x, c.Data[f*128+127])
		}
	}
}
