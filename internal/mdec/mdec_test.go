package mdec

import "testing"

func TestResetStatus(t *testing.T) {
	d := New()
	d.Write(4, 0x80000000)
	if s := d.Read(4); s != 0x80040000 {
		t.Errorf("重置後狀態 %08x，應為 80040000（psx-spx）", s)
	}
}

// 單一黑白區塊：DC = 0、其餘為 0，8bpp 無號輸出應全為 0x80。
func TestMonoBlockDC0(t *testing.T) {
	d := &tickingMDEC{New()}
	d.Write(4, 0x80000000)
	d.Write(4, 0x60000000)
	// 量化表（亮度、色度各 64 位元組，全 1）
	d.Write(0, 0x40000001)
	for i := 0; i < 32; i++ {
		d.Write(0, 0x01010101)
	}
	// IDCT 表：只有第一列為 0x5A82（DC 基底），其他 0
	d.Write(0, 0x60000000)
	for i := 0; i < 32; i++ {
		v := uint32(0)
		if i < 4 {
			v = 0x5A825A82
		}
		d.Write(0, v)
	}
	if d.Read(4)&(1<<29) != 0 {
		t.Fatal("表載完後不應忙碌")
	}
	// 解碼命令：8bpp、1 個參數字（DC=0、接著 FE00 結束）
	d.Write(0, 0x28000001)
	d.Write(0, 0xFE000000)
	if d.Read(4)&(1<<31) == 0 {
		t.Fatal("解碼要花時間，還沒前進週期就不該有輸出")
	}
	for i := 0; i < 8; i++ {
		d.Tick(clockCap)
	}
	got := 0
	for d.Read(4)&(1<<31) == 0 {
		w := d.Read(0)
		for i := 0; i < 4; i++ {
			if b := uint8(w >> (8 * i)); b != 0x80 {
				t.Fatalf("第 %d 位元組 %02x，應為 80", got, b)
			}
			got++
		}
	}
	if got != 64 {
		t.Errorf("輸出 %d 位元組，應為 64", got)
	}
}

// tickingMDEC 每次寫入後前進幾個週期，像 CPU 一條條指令執行那樣。
type tickingMDEC struct{ *MDEC }

func (d *tickingMDEC) Write(addr, v uint32) {
	d.MDEC.Write(addr, v)
	d.Tick(4)
}
