package pgxp

import "testing"

func TestMemoryTransport(t *testing.T) {
	p := New()
	raw := uint32(12 | 34<<16)
	p.Projected(12.75, 34.5, raw, 0, true)
	p.StoreCOP2(14, 0x80001000, raw)
	if v := p.ReadMemory(0xa0601000, raw); !v.Valid(raw) || v.X != 12.75 {
		t.Fatal(v)
	}
	p.Load(1, 0x1000, raw, 0x23, 10, 15)
	p.Store(1, 0x2000, raw, 0x2b, 15)
	if p.ReadMemory(0x2000, raw).Valid(raw) {
		t.Fatal("load delay 提早啟用")
	}
	p.Move(2, 1, raw, 16)
	p.Store(2, 0x1f800000, raw, 0x2b, 17)
	if !p.ReadMemory(0x9f800000, raw).Valid(raw) {
		t.Fatal("scratchpad 搬移失敗")
	}
	p.LoadCOP2(15, 0x1000, raw)
	p.StoreCOP2(14, 0x1f801810, raw)
	if !p.ReadMemory(0xbf801810, raw).Valid(raw) {
		t.Fatal("GP0 埠搬移失敗")
	}
	if p.ReadMemory(0x1000, raw+1).Valid(raw + 1) {
		t.Fatal("改字未失效")
	}
}

func TestPartialAccess(t *testing.T) {
	for _, op := range []uint32{0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26} {
		p := New()
		raw := uint32(12 | 34<<16)
		p.Projected(12.75, 34.5, raw, 0, true)
		p.StoreCOP2(14, 0x1000, raw)
		value := raw
		if op == 0x21 || op == 0x25 {
			value = 12
		}
		if op == 0x20 || op == 0x24 {
			value = 12
		}
		p.Load(1, 0x1000, value, op, 1, 2)
		p.Store(1, 0x2000, value, 0x2b, 3)
		got := p.ReadMemory(0x2000, value)
		if op == 0x20 || op == 0x24 {
			if got.Valid(value) {
				t.Fatal("位元組載入保留座標")
			}
		} else if !got.Valid(value) {
			t.Fatalf("op=%x %v", op, got)
		}
	}
	p := New()
	raw := uint32(12 | 34<<16)
	p.Projected(12.75, 34.5, raw, 0, true)
	p.StoreCOP2(14, 0x1000, raw)
	p.Load(1, 0x1002, 34, 0x25, 1, 2)
	p.Store(1, 0x2000, 34, 0x29, 3)
	p.Load(2, 0x1000, 12, 0x25, 4, 5)
	p.Store(2, 0x2002, 12, 0x29, 6)
	if v := p.ReadMemory(0x2000, 34|12<<16); !v.Valid(34|12<<16) || v.X != 34.5 || v.Y != 12.75 {
		t.Fatal(v)
	}
	p.Store(2, 0x2000, 12, 0x28, 7)
	if p.ReadMemory(0x2000, 34|12<<16).Flags != 0 {
		t.Fatal("SB 沒有使整字失效")
	}
}

func TestCOP2AndLoadOverwrite(t *testing.T) {
	p := New()
	raw := uint32(12 | 34<<16)
	p.Projected(12.75, 34.5, raw, 0, true)
	p.ReadCOP2(1, 14, raw, 1, 2)
	p.Move(2, 1, raw, 3)
	p.WriteCOP2(2, 40, raw, raw, 4)
	p.ReadCOP2(3, 40, raw, 5, 6)
	p.Store(3, 0x1000, raw, 0x2b, 7)
	if !p.ReadMemory(0x1000, raw).Valid(raw) {
		t.Fatal("控制暫存器搬移失敗")
	}
	p.Load(3, 0x3000, raw, 0x23, 8, 9)
	p.Load(3, 0x1000, raw, 0x23, 9, 10)
	p.Store(3, 0x2000, raw, 0x2b, 11)
	if !p.ReadMemory(0x2000, raw).Valid(raw) {
		t.Fatal("連續載入覆蓋失敗")
	}
}
