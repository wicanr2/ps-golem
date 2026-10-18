package cpu

// BreakHandler 只在非延遲槽的 BREAK 呼叫。回傳 true 表示已模擬替換指令，
// 由 CPU 完成 load delay 並正常前進；false 保留 Bp 例外。
// 處理器讀到的是本指令的來源暫存器，尚未提交上一條 load。
// 寫入暫存器須用 SetBreakGPR，避免被上一條 load 覆蓋。
type BreakHandler func(c *CPU, pc, code uint32) bool

func (c *CPU) SetBreakHandler(h BreakHandler) { c.breakHandler = h }

// SetBreakGPR 供 BREAK 處理器寫回結果；同目標的延遲載入作廢，與 ADDU 一致。
func (c *CPU) SetBreakGPR(i int, value uint32) {
	c.cancelLoadTo(uint32(i))
	c.SetGPR(i, value)
}

func (c *CPU) handleBreak(instr uint32) {
	if c.bdbt == 0 && c.breakHandler != nil && c.breakHandler(c, c.pc, (instr>>6)&0xfffff) {
		c.doLDS()
		return
	}
	c.doLDS()
	c.raise(ExcBP, instr)
}

// StoreHalf 模擬 SH 的對齊、快取隔離與 PGXP 失效語意。只供 BREAK 處理器使用。
func (c *CPU) StoreHalf(addr, value uint32) {
	if addr&1 != 0 {
		c.addressError(ExcADES, addr, 0x29<<26)
		return
	}
	if c.precision != nil && c.cop0[CP0SR]&srIsC == 0 {
		c.precision.Store(0, addr, value, 0x29, c.now)
	}
	c.write(addr, value, 2)
}
