package machine

import "testing"

// VBlank 中斷要走完整條鏈：I_STAT bit 0 → I_MASK → CAUSE bit 10 → SR.IEc → 例外 → BIOS。
// 量法：側載測試程式後，數 CPU 以中斷碼（ExcCode 0）進入 80000080 的次數，應與經過的 field 數相近。
func TestVBlankReachesProgram(t *testing.T) {
	bios := loadBIOS(t)
	files := loadTestsZip(t)
	m, err := New(bios)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RunUntilPC(ShellEntry, 400_000_000); err != nil {
		t.Fatal(err)
	}
	if err := m.SideloadEXE(files["cpu/cop/cop.exe"]); err != nil {
		t.Fatal(err)
	}
	f0 := m.Field()
	irqs := 0
	prev := m.CPU.PC()
	for i := 0; i < 50_000_000; i++ {
		m.Step()
		pc := m.CPU.PC()
		if physical(pc) == 0x80 && physical(prev) != 0x80 && (m.CPU.COP0(13)>>2)&0x1F == 0 {
			irqs++
		}
		prev = pc
	}
	fields := m.Field() - f0
	t.Logf("經過 %d 個 field，中斷進入 %d 次", fields, irqs)
	if fields == 0 || irqs == 0 {
		t.Fatal("VBlank 沒有送到程式")
	}
	if uint64(irqs) < fields/2 {
		t.Errorf("中斷次數 %d 遠少於 field 數 %d", irqs, fields)
	}
}
