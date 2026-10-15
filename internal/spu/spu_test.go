package spu

import "testing"

// 暫存器實體位址。
const (
	base       = 0x1F801C00
	regMainL   = base + 0x180 // 主音量 L
	regMainR   = base + 0x182
	regKeyOn   = base + 0x188
	regKeyOff  = base + 0x18C
	regIRQAddr = base + 0x1A4
	regXferAdr = base + 0x1A6
	regSPUCNT  = base + 0x1AA
	regSPUSTAT = base + 0x1AE
)

func voiceReg(v int, off uint32) uint32 { return base + uint32(v)*0x10 + off }

// 10 組取樣：Power 後 clock_divider = 768。768 - 7680 = -6912，
// 每加 768 算一組，加到 > 0 共加 10 次，結束時 clock_divider = 768。
func TestAdvanceSampleCount(t *testing.T) {
	s := New()
	next := s.Advance(768*10, nil)
	if next < 1 || next > 768 {
		t.Fatalf("回傳 %d，應在 1..768", next)
	}
	if next != 768 {
		t.Fatalf("回傳 %d，依 Beetle 的推導應為 768", next)
	}
	got := s.TakeSamples(nil)
	if len(got) != 20 {
		t.Fatalf("取樣數 %d，應為 20", len(got))
	}
	// 上電後沒有 voice 發聲、主音量 0，輸出全為 0。
	for i, v := range got {
		if v != 0 {
			t.Fatalf("取樣 %d = %d，應為 0", i, v)
		}
	}
	if len(s.TakeSamples(nil)) != 0 {
		t.Fatal("TakeSamples 之後緩衝應為空")
	}

	// 不足一組時不產生取樣，回傳剩餘週期。
	if n := s.Advance(100, nil); n != 668 {
		t.Fatalf("回傳 %d，應為 668", n)
	}
	if len(s.TakeSamples(nil)) != 0 {
		t.Fatal("不足 768 週期不應產生取樣")
	}
}

// 緩衝最多 4096 組，滿了丟新的（Beetle IntermediateBufferPos < 4096 才寫）。
func TestBufferLimit(t *testing.T) {
	s := New()
	s.Advance(768*5000, nil)
	if got := len(s.TakeSamples(nil)); got != 4096*2 {
		t.Fatalf("取樣數 %d，應為 %d", got, 4096*2)
	}
}

// DMA 寫入再讀回。SPU_WriteDMA／SPU_ReadDMA 都從 RWAddr 起逐半字存取，
// 1F801DA6 寫 V 時 RWAddr = (V << 2) & 0x3FFFF。傳輸模式（SPUCNT bits 4–5）
// 在 Beetle 的 spu.c 不影響存取，由 DMA 端決定方向；這裡照實機流程設定。
func TestDMARoundTrip(t *testing.T) {
	s := New()
	data := []uint32{0x12345678, 0x9ABCDEF0, 0x0000FFFF, 0xFFFF0000, 0xCAFEBABE}

	s.Write(regXferAdr, 0x0100)
	s.Write(regSPUCNT, 0x8000|0x20) // DMA 寫
	for _, v := range data {
		s.DMAWrite(v)
	}

	// 從 RAM 直接看：低半字在前。
	if s.ram[0x400] != 0x5678 || s.ram[0x401] != 0x1234 {
		t.Fatalf("RAM[0x400..] = %04x %04x", s.ram[0x400], s.ram[0x401])
	}

	s.Write(regXferAdr, 0x0100)
	s.Write(regSPUCNT, 0x8000|0x30) // DMA 讀
	for i, want := range data {
		if got := s.DMARead(); got != want {
			t.Fatalf("第 %d 字讀到 %08x，應為 %08x", i, got, want)
		}
	}

	// 手動 FIFO（1F801DA8）讀也走同一個 RWAddr。
	s.Write(regXferAdr, 0x0100)
	if got := s.Read(base + 0x1A8); got != 0x5678 {
		t.Fatalf("FIFO 讀到 %04x，應為 5678", got)
	}
}

// ADPCM 區塊放在 byte 位址 0x1000（半字索引 0x800，start address 暫存器值 0x1000/8 = 0x200）。
//
// 區塊第 0 半字：shift = 0、filter = 0、flags = 0x07（loop start＋repeat＋end），
// 播完回到自己開頭一直循環，所以 voice 不會因區塊結束被強制歸零。
// 其餘 7 個半字是 0x7777：每個 nibble 7 → (int16)(0x7000) >> 0 = 28672，
// filter 0 的權重都是 0，DecodeBuffer 全是 28672。
const blockByteAddr = 0x1000

func setupVoice0(s *SPU, cnt uint16) {
	idx := blockByteAddr / 2
	s.ram[idx] = 0x0700
	for i := 1; i < 8; i++ {
		s.ram[idx+i] = 0x7777
	}

	s.Write(voiceReg(0, 0x00), 0x3FFF) // 音量 L：bit 15 = 0，Current = 0x3FFF << 1
	s.Write(voiceReg(0, 0x02), 0x3FFF) // 音量 R
	s.Write(voiceReg(0, 0x04), 0x1000) // pitch 1.0
	s.Write(voiceReg(0, 0x06), blockByteAddr/8)
	// ADSR 低半字 0x00FF：Attack 線性、Ar = 0；Dr = 0xF；Sl = 0xF。
	//   Ar = 0：CalcVCDelta 得 increment = 7 << (0x2F >> 2) = 14336、divinco = 32768，
	//   每個取樣 +14336：0 → 14336 → 28672 → 43008 溢位（bit 15 由 0 變 1）→ 0x7FFF。
	//   到 0x7FFF 後進 Decay：Dr = 0xF 每 16 組 -8，Sl = 0xF 使 SustainLevel = 0x8000，第一步（0x7FF7）就進 Sustain。
	// ADSR 高半字 0x1FC0：Sr = 0x7F 遞增（divinco = 32768 >> 20 = 0，speed 不小於 0x7F 不補 1，
	//   位準不再變），Rr = 0 線性：increment = ~7 << 11 = -16384，每個取樣 -16384。
	s.Write(voiceReg(0, 0x08), 0x00FF)
	s.Write(voiceReg(0, 0x0A), 0x1FC0)

	s.Write(regMainL, 0x3FFF)
	s.Write(regMainR, 0x3FFF)
	s.Write(regSPUCNT, cnt)
}

func TestVoiceKeyOnOff(t *testing.T) {
	s := New()
	setupVoice0(s, 0xC000) // bit 15 啟用、bit 14 解除靜音

	s.Write(regKeyOn, 0x0001)

	// 第 1 組取樣：voice 迴圈尾端處理 key on，DecodePlayDelay = 4。
	// 第 2–5 組：延遲遞減，不跑包絡。第 6 組起每組跑一次包絡（在算完輸出之後），
	// 第 6、7、8 組跑完後位準為 14336、28672、0x7FFF，所以第 6 組（索引 5）輸出仍為 0，
	// 第 7 組（索引 6）起不為 0。
	// 主音量的 Current 在第 1 組結束時才設成 0x7FFE，不影響上述結果。
	var out []int16
	sawMax := false
	for i := 0; i < 40; i++ {
		s.Advance(768, nil)
		if s.Read(voiceReg(0, 0x0C)) == 0x7FFF {
			sawMax = true
		}
	}
	out = s.TakeSamples(out)
	if !sawMax {
		t.Fatal("包絡沒有到 0x7FFF")
	}
	nonzero := 0
	for _, v := range out {
		if v != 0 {
			nonzero++
		}
	}
	if nonzero == 0 {
		t.Fatal("key on 後取樣全為 0")
	}
	if out[2*5] != 0 || out[2*5+1] != 0 {
		t.Fatalf("第 6 組取樣（索引 5）應為 0（包絡此時仍為 0），得 %d %d", out[10], out[11])
	}
	if out[2*6] == 0 || out[2*6+1] == 0 {
		t.Fatalf("第 7 組取樣（索引 6）應不為 0，得 %d %d", out[12], out[13])
	}
	if out[2*6] < 0 {
		t.Fatalf("正值資料應得正值輸出，得 %d", out[12])
	}

	// key off：DecodePlayDelay = 0 < 3，進入 release。Rr = 0 線性每組 -16384，
	// 位準至多 0x7FFF，2 步內變負（bit 15）而被重設為 0。
	s.Write(regKeyOff, 0x0001)
	for i := 0; i < 20; i++ {
		s.Advance(768, nil)
	}
	if env := s.Read(voiceReg(0, 0x0C)); env != 0 {
		t.Fatalf("key off 後包絡 %04x，應為 0", env)
	}
	out = s.TakeSamples(out[:0])
	for i := len(out) - 10; i < len(out); i++ {
		if out[i] != 0 {
			t.Fatalf("key off 後尾端取樣 %d = %d，應為 0", i, out[i])
		}
	}
}

// IRQ 位址 = 區塊開頭（1F801DA4 寫 0x200 → IRQAddr = 0x800）。SPU_RunDecoder 在
// CurAddr & 7 == 0 讀區塊標頭之前比對 IRQAddr == CurAddr，相等就 IRQAsserted = true。
// 其他會碰 SPU RAM 的路徑都碰不到 0x800：未 key on 的 voice 從位址 0 走到 4 就停
// （pitch 0、DecodeAvail 到 12 不再解碼），CD 寫 0x000–0x3FF，voice 1／3 寫 0x400–0x7FF。
func TestIRQOnVoiceRead(t *testing.T) {
	s := New()
	s.Write(regIRQAddr, blockByteAddr/8)
	setupVoice0(s, 0xC040) // bit 6：IRQ 啟用

	for i := 0; i < 5; i++ {
		s.Advance(768, nil)
	}
	if s.IRQ() {
		t.Fatal("key on 前不應觸發 IRQ")
	}

	// key on 在第 1 組取樣尾端生效，第 2 組的 runDecoder 讀到 0x800。
	s.Write(regKeyOn, 0x0001)
	s.Advance(768, nil)
	if s.IRQ() {
		t.Fatal("key on 那一組取樣還沒讀區塊，不應觸發")
	}
	s.Advance(768, nil)
	if !s.IRQ() {
		t.Fatal("voice 讀到 IRQ 位址後應觸發 IRQ")
	}

	// SPUSTAT bit 6 在下一組取樣開頭才更新。
	s.Advance(768, nil)
	if s.Read(regSPUSTAT)&0x40 == 0 {
		t.Fatal("SPUSTAT bit 6 應為 1")
	}

	// 清 SPUCNT bit 6 解除中斷。
	s.Write(regSPUCNT, 0xC000)
	if s.IRQ() {
		t.Fatal("清 SPUCNT bit 6 後 IRQ 應解除")
	}
}

// 結構可直接值複製（快照），複製後各自獨立。
func TestValueCopy(t *testing.T) {
	s := New()
	setupVoice0(s, 0xC000)
	s.Write(regKeyOn, 1)
	s.Advance(768*20, nil)

	c := *s
	a1 := s.Advance(768*30, nil)
	a2 := c.Advance(768*30, nil)
	if a1 != a2 {
		t.Fatalf("回傳不同：%d %d", a1, a2)
	}
	x := s.TakeSamples(nil)
	y := c.TakeSamples(nil)
	if len(x) != len(y) {
		t.Fatalf("長度不同：%d %d", len(x), len(y))
	}
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("取樣 %d 不同：%d %d", i, x[i], y[i])
		}
	}
}
