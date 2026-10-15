// Package cpu 實作 PlayStation 的 R3000A（MIPS I）與 COP0，GTE 掛在 COP2。
//
// 規格：docs/spec/004-cpu-r3000a.md、docs/spec/006-minimal-machine.md §4。
//
// 參照 beetle-psx-libretro mednafen/psx/cpu.c
// (commit e57860d82faa953dc0fc10f201fd583739bc2f7d)，GPL-2.0-or-later。
// 指令語意、load delay、branch delay、例外與中斷檢查的位置照它；
// 週期模型與 i-cache 不搬（006 §3、§4）。
package cpu

import "github.com/wicanr2/ps-golem/internal/gte"

// Bus 是 CPU 看到的記憶體。位址是 CPU 的虛擬位址，區段轉換由 Bus 負責。
type Bus interface {
	Read8(addr uint32) uint8
	Read16(addr uint32) uint16
	Read32(addr uint32) uint32
	Write8(addr uint32, v uint8)
	Write16(addr uint32, v uint16)
	Write32(addr uint32, v uint32)
	// IsolatedWrite 是 SR.IsC 成立時的資料寫入（BIOS 用來清快取），size 為 1、2、4。
	IsolatedWrite(addr uint32, v uint32, size int)
	// InstructionBusError 回報從 addr 取指令是否會發生匯流排錯誤。
	InstructionBusError(addr uint32) bool
}

// COP0 暫存器編號。
const (
	CP0BPC   = 3
	CP0BDA   = 5
	CP0TAR   = 6
	CP0DCIC  = 7
	CP0BADA  = 8
	CP0BDAM  = 9
	CP0BPCM  = 11
	CP0SR    = 12
	CP0CAUSE = 13
	CP0EPC   = 14
	CP0PRID  = 15
)

// 例外碼（CAUSE bit 2–6）。
const (
	ExcINT     = 0
	ExcADEL    = 4
	ExcADES    = 5
	ExcIBE     = 6
	ExcSYSCALL = 8
	ExcBP      = 9
	ExcRI      = 10
	ExcCOPU    = 11
	ExcOV      = 12
)

const (
	srIEc = 1 << 0
	srIsC = 1 << 16
	srBEV = 1 << 22
	srCU2 = 1 << 30
)

// ldNone 是 load delay 的「沒有待寫入」槽位，寫到 gpr 的備用格。
const ldNone = 34

// CPU 是一顆 R3000A 的完整狀態。
type CPU struct {
	gpr    [35]uint32 // 0..31 一般暫存器，32 以後是 load delay 的備用格
	hi, lo uint32
	pc     uint32
	nextPC uint32
	// bdbt：2 表示目前指令位於分支延遲槽，3 表示延遲槽且分支成立（寫進 CAUSE bit 31、30）。
	bdbt uint32

	ldWhich uint32
	ldValue uint32

	cop0 [32]uint32
	GTE  gte.GTE

	bus Bus

	// branched 為真時，本指令已自行設定 pc／nextPC（分支），結束時不再前進。
	branched bool
}

// New 建立一顆上電狀態的 CPU。
func New(bus Bus) *CPU {
	c := &CPU{bus: bus}
	c.Reset()
	return c
}

// Reset 回到上電狀態：PC=BFC00000，SR 的 BEV、TS 成立。
func (c *CPU) Reset() {
	*c = CPU{bus: c.bus}
	c.pc = 0xBFC00000
	c.nextPC = c.pc + 4
	c.ldWhich = ldNone
	c.cop0[CP0SR] = 1<<22 | 1<<21
	c.cop0[CP0PRID] = 2
}

// Rebind 換掉 CPU 看到的匯流排（複製整台機器時用）。
func (c *CPU) Rebind(bus Bus) { c.bus = bus }

// PC 回傳下一條要執行的指令位址。
func (c *CPU) PC() uint32 { return c.pc }

// GPR 讀第 i 個一般暫存器。
func (c *CPU) GPR(i int) uint32 {
	if i == 0 {
		return 0
	}
	return c.gpr[i]
}

// SetGPR 寫第 i 個一般暫存器（側載 PS-EXE 用）。
func (c *CPU) SetGPR(i int, v uint32) {
	if i != 0 {
		c.gpr[i] = v
	}
}

// SetPC 直接跳到 pc，取消分支延遲與 load delay。
func (c *CPU) SetPC(pc uint32) {
	c.pc = pc
	c.nextPC = pc + 4
	c.bdbt = 0
	c.ldWhich = ldNone
}

// COP0 讀 COP0 暫存器（觀測用）。
func (c *CPU) COP0(i int) uint32 { return c.cop0[i] }

// SetIRQ 設定硬體中斷線（CAUSE bit 10），由中斷控制器驅動。
func (c *CPU) SetIRQ(asserted bool) {
	if asserted {
		c.cop0[CP0CAUSE] |= 1 << 10
	} else {
		c.cop0[CP0CAUSE] &^= 1 << 10
	}
}

func (c *CPU) interruptPending() bool {
	return c.cop0[CP0SR]&c.cop0[CP0CAUSE]&0xFF00 != 0 && c.cop0[CP0SR]&srIEc != 0
}

// doLDS 把上一條 load 的結果寫進暫存器。
func (c *CPU) doLDS() {
	c.gpr[c.ldWhich] = c.ldValue
	c.ldWhich = ldNone
}

// startLoad 排一個延遲寫入。同一個暫存器已有待寫入時，舊的那筆作廢。
func (c *CPU) startLoad(rt, v uint32) {
	c.ldWhich = rt
	c.ldValue = v
}

func (c *CPU) cancelLoadTo(rt uint32) {
	if c.ldWhich == rt {
		c.ldWhich = 0
	}
}

// exception 進入例外，回傳處理常式位址。
func (c *CPU) exception(code, pc, np, instr uint32) uint32 {
	handler := uint32(0x80000080)
	if c.cop0[CP0SR]&srBEV != 0 {
		handler = 0xBFC00180
	}
	c.cop0[CP0EPC] = pc
	if c.bdbt&2 != 0 {
		c.cop0[CP0EPC] -= 4
		c.cop0[CP0TAR] = np
	}
	sr := c.cop0[CP0SR]
	c.cop0[CP0SR] = sr&^0x3F | (sr<<2)&0x3F
	cause := c.cop0[CP0CAUSE] & 0x0000FF00
	cause |= code << 2
	cause |= c.bdbt << 30
	cause |= (instr << 2) & (3 << 28)
	c.cop0[CP0CAUSE] = cause
	c.bdbt = 0
	return handler
}

// raise 在目前指令觸發例外：下一條執行處理常式。
func (c *CPU) raise(code, instr uint32) {
	c.nextPC = c.exception(code, c.pc, c.nextPC, instr)
}

// branch 執行分支或跳躍。link 不為 0 時寫回返回位址（延遲槽之後那條）。
func (c *CPU) branch(cond bool, offset, mask uint32, link uint32, doLink bool) {
	c.pc = c.nextPC
	c.nextPC += 4
	c.bdbt = 2
	if doLink {
		c.gpr[link] = c.nextPC
	}
	if cond {
		c.nextPC = ((c.nextPC - 4) & mask) + offset
		c.bdbt = 3
	}
	c.branched = true
}

// Step 執行一條指令。
func (c *CPU) Step() {
	c.gpr[0] = 0
	c.branched = false
	pc := c.pc

	if pc&3 != 0 {
		c.cop0[CP0BADA] = pc
		c.raise(ExcADEL, 0)
		c.advance()
		return
	}

	if c.bus.InstructionBusError(pc) {
		c.raise(ExcIBE, 0)
		c.advance()
		return
	}
	instr := c.bus.Read32(pc)

	// 中斷在指令邊界檢查；PC 指著 COP2 指令時不收（照參照實作，避開 GTE pipeline 細節）。
	if c.interruptPending() && instr>>26 != 0x12 {
		c.doLDS()
		c.raise(ExcINT, instr)
		c.advance()
		return
	}

	c.execute(instr)
	c.advance()
}

func (c *CPU) advance() {
	if c.branched {
		return
	}
	c.pc = c.nextPC
	c.nextPC += 4
	c.bdbt = 0
}

func (c *CPU) execute(instr uint32) {
	op := instr >> 26
	rs := (instr >> 21) & 0x1F
	rt := (instr >> 16) & 0x1F
	rd := (instr >> 11) & 0x1F
	shamt := (instr >> 6) & 0x1F
	imm := uint32(int32(int16(instr)))
	immZE := instr & 0xFFFF
	g := &c.gpr

	if op == 0 {
		switch instr & 0x3F {
		case 0x00: // SLL
			r := g[rt] << shamt
			c.doLDS()
			g[rd] = r
		case 0x02: // SRL
			r := g[rt] >> shamt
			c.doLDS()
			g[rd] = r
		case 0x03: // SRA
			r := uint32(int32(g[rt]) >> shamt)
			c.doLDS()
			g[rd] = r
		case 0x04: // SLLV
			r := g[rt] << (g[rs] & 0x1F)
			c.doLDS()
			g[rd] = r
		case 0x06: // SRLV
			r := g[rt] >> (g[rs] & 0x1F)
			c.doLDS()
			g[rd] = r
		case 0x07: // SRAV
			r := uint32(int32(g[rt]) >> (g[rs] & 0x1F))
			c.doLDS()
			g[rd] = r
		case 0x08: // JR
			t := g[rs]
			c.doLDS()
			c.branch(true, t, 0, 0, false)
		case 0x09: // JALR
			t := g[rs]
			c.doLDS()
			c.branch(true, t, 0, rd, true)
		case 0x0C: // SYSCALL
			c.doLDS()
			c.raise(ExcSYSCALL, instr)
		case 0x0D: // BREAK
			c.doLDS()
			c.raise(ExcBP, instr)
		case 0x10: // MFHI
			c.doLDS()
			g[rd] = c.hi
		case 0x11: // MTHI
			c.hi = g[rs]
			c.doLDS()
		case 0x12: // MFLO
			c.doLDS()
			g[rd] = c.lo
		case 0x13: // MTLO
			c.lo = g[rs]
			c.doLDS()
		case 0x18: // MULT
			r := uint64(int64(int32(g[rs])) * int64(int32(g[rt])))
			c.doLDS()
			c.lo, c.hi = uint32(r), uint32(r>>32)
		case 0x19: // MULTU
			r := uint64(g[rs]) * uint64(g[rt])
			c.doLDS()
			c.lo, c.hi = uint32(r), uint32(r>>32)
		case 0x1A: // DIV
			n, d := g[rs], g[rt]
			switch {
			case d == 0:
				if n&0x80000000 != 0 {
					c.lo = 1
				} else {
					c.lo = 0xFFFFFFFF
				}
				c.hi = n
			case n == 0x80000000 && d == 0xFFFFFFFF:
				c.lo, c.hi = 0x80000000, 0
			default:
				c.lo = uint32(int32(n) / int32(d))
				c.hi = uint32(int32(n) % int32(d))
			}
			c.doLDS()
		case 0x1B: // DIVU
			n, d := g[rs], g[rt]
			if d == 0 {
				c.lo, c.hi = 0xFFFFFFFF, n
			} else {
				c.lo, c.hi = n/d, n%d
			}
			c.doLDS()
		case 0x20: // ADD
			r := g[rs] + g[rt]
			ov := (^(g[rs] ^ g[rt]))&(g[rs]^r)&0x80000000 != 0
			c.doLDS()
			if ov {
				c.raise(ExcOV, instr)
			} else {
				g[rd] = r
			}
		case 0x21: // ADDU
			r := g[rs] + g[rt]
			c.doLDS()
			g[rd] = r
		case 0x22: // SUB
			r := g[rs] - g[rt]
			ov := (g[rs]^g[rt])&(g[rs]^r)&0x80000000 != 0
			c.doLDS()
			if ov {
				c.raise(ExcOV, instr)
			} else {
				g[rd] = r
			}
		case 0x23: // SUBU
			r := g[rs] - g[rt]
			c.doLDS()
			g[rd] = r
		case 0x24: // AND
			r := g[rs] & g[rt]
			c.doLDS()
			g[rd] = r
		case 0x25: // OR
			r := g[rs] | g[rt]
			c.doLDS()
			g[rd] = r
		case 0x26: // XOR
			r := g[rs] ^ g[rt]
			c.doLDS()
			g[rd] = r
		case 0x27: // NOR
			r := ^(g[rs] | g[rt])
			c.doLDS()
			g[rd] = r
		case 0x2A: // SLT
			r := b2u(int32(g[rs]) < int32(g[rt]))
			c.doLDS()
			g[rd] = r
		case 0x2B: // SLTU
			r := b2u(g[rs] < g[rt])
			c.doLDS()
			g[rd] = r
		default:
			c.doLDS()
			c.raise(ExcRI, instr)
		}
		return
	}

	switch op {
	case 0x01: // BCOND：BLTZ、BGEZ、BLTZAL、BGEZAL
		tv := g[rs]
		cond := int32(tv^(rt<<31)) < 0
		link := uint32(0)
		if rt&0x1E == 0x10 {
			link = 31
		}
		c.doLDS()
		c.branch(cond, imm<<2, ^uint32(0), link, true)
	case 0x02: // J
		c.doLDS()
		c.branch(true, (instr&0x3FFFFFF)<<2, 0xF0000000, 0, false)
	case 0x03: // JAL
		c.doLDS()
		c.branch(true, (instr&0x3FFFFFF)<<2, 0xF0000000, 31, true)
	case 0x04: // BEQ
		cond := g[rs] == g[rt]
		c.doLDS()
		c.branch(cond, imm<<2, ^uint32(0), 0, false)
	case 0x05: // BNE
		cond := g[rs] != g[rt]
		c.doLDS()
		c.branch(cond, imm<<2, ^uint32(0), 0, false)
	case 0x06: // BLEZ
		cond := int32(g[rs]) <= 0
		c.doLDS()
		c.branch(cond, imm<<2, ^uint32(0), 0, false)
	case 0x07: // BGTZ
		cond := int32(g[rs]) > 0
		c.doLDS()
		c.branch(cond, imm<<2, ^uint32(0), 0, false)
	case 0x08: // ADDI
		r := g[rs] + imm
		ov := (^(g[rs] ^ imm))&(g[rs]^r)&0x80000000 != 0
		c.doLDS()
		if ov {
			c.raise(ExcOV, instr)
		} else {
			g[rt] = r
		}
	case 0x09: // ADDIU
		r := g[rs] + imm
		c.doLDS()
		g[rt] = r
	case 0x0A: // SLTI
		r := b2u(int32(g[rs]) < int32(imm))
		c.doLDS()
		g[rt] = r
	case 0x0B: // SLTIU
		r := b2u(g[rs] < imm)
		c.doLDS()
		g[rt] = r
	case 0x0C: // ANDI
		r := g[rs] & immZE
		c.doLDS()
		g[rt] = r
	case 0x0D: // ORI
		r := g[rs] | immZE
		c.doLDS()
		g[rt] = r
	case 0x0E: // XORI
		r := g[rs] ^ immZE
		c.doLDS()
		g[rt] = r
	case 0x0F: // LUI
		c.doLDS()
		g[rt] = immZE << 16
	case 0x10:
		c.cop0Op(instr)
	case 0x11, 0x13:
		c.cop13Op(instr)
	case 0x12:
		c.cop2Op(instr)
	case 0x20: // LB
		a := g[rs] + imm
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, uint32(int32(int8(c.bus.Read8(a)))))
	case 0x21: // LH
		a := g[rs] + imm
		if a&1 != 0 {
			c.doLDS()
			c.addressError(ExcADEL, a, instr)
			return
		}
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, uint32(int32(int16(c.bus.Read16(a)))))
	case 0x22: // LWL
		a := g[rs] + imm
		v := c.mergeBase(rt)
		var r uint32
		switch a & 3 {
		case 0:
			r = v&0x00FFFFFF | c.read32(a&^3)<<24
		case 1:
			r = v&0x0000FFFF | c.read32(a&^3)<<16
		case 2:
			r = v&0x000000FF | c.read32(a&^3)<<8
		case 3:
			r = c.read32(a &^ 3)
		}
		c.startLoad(rt, r)
	case 0x23: // LW
		a := g[rs] + imm
		if a&3 != 0 {
			c.doLDS()
			c.addressError(ExcADEL, a, instr)
			return
		}
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, c.read32(a))
	case 0x24: // LBU
		a := g[rs] + imm
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, uint32(c.bus.Read8(a)))
	case 0x25: // LHU
		a := g[rs] + imm
		if a&1 != 0 {
			c.doLDS()
			c.addressError(ExcADEL, a, instr)
			return
		}
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, uint32(c.bus.Read16(a)))
	case 0x26: // LWR
		a := g[rs] + imm
		v := c.mergeBase(rt)
		w := c.read32(a &^ 3)
		var r uint32
		switch a & 3 {
		case 0:
			r = w
		case 1:
			r = v&0xFF000000 | w>>8
		case 2:
			r = v&0xFFFF0000 | w>>16
		case 3:
			r = v&0xFFFFFF00 | w>>24
		}
		c.startLoad(rt, r)
	case 0x28: // SB
		a := g[rs] + imm
		c.write(a, g[rt], 1)
		c.doLDS()
	case 0x29: // SH
		a := g[rs] + imm
		if a&1 != 0 {
			c.addressError(ExcADES, a, instr)
		} else {
			c.write(a, g[rt], 2)
		}
		c.doLDS()
	case 0x2A: // SWL
		a := g[rs] + imm
		al := a &^ 3
		w := c.read32(al)
		v := g[rt]
		switch a & 3 {
		case 0:
			w = w&0xFFFFFF00 | v>>24
		case 1:
			w = w&0xFFFF0000 | v>>16
		case 2:
			w = w&0xFF000000 | v>>8
		case 3:
			w = v
		}
		c.write(al, w, 4)
		c.doLDS()
	case 0x2B: // SW
		a := g[rs] + imm
		if a&3 != 0 {
			c.addressError(ExcADES, a, instr)
		} else {
			c.write(a, g[rt], 4)
		}
		c.doLDS()
	case 0x2E: // SWR
		a := g[rs] + imm
		al := a &^ 3
		w := c.read32(al)
		v := g[rt]
		switch a & 3 {
		case 0:
			w = v
		case 1:
			w = w&0x000000FF | v<<8
		case 2:
			w = w&0x0000FFFF | v<<16
		case 3:
			w = w&0x00FFFFFF | v<<24
		}
		c.write(al, w, 4)
		c.doLDS()
	case 0x30, 0x31, 0x33: // LWC0、LWC1、LWC3
		a := g[rs] + imm
		c.doLDS()
		if !c.copUsable(op & 3) {
			c.raise(ExcCOPU, instr)
		} else if a&3 != 0 {
			c.addressError(ExcADEL, a, instr)
		} else {
			c.read32(a)
		}
	case 0x32: // LWC2
		a := g[rs] + imm
		c.doLDS()
		if !c.copUsable(2) {
			// 參照實作沒有這個檢查。依 SWC2 的實機結果（見下）比照辦理，推論等級：假說。
			c.raise(ExcCOPU, instr)
		} else if a&3 != 0 {
			c.addressError(ExcADEL, a, instr)
		} else {
			c.GTE.Write(int(rt), c.read32(a))
		}
	case 0x38, 0x39, 0x3B: // SWC0、SWC1、SWC3
		a := g[rs] + imm
		c.doLDS()
		if !c.copUsable(op & 3) {
			c.raise(ExcCOPU, instr)
		} else if a&3 != 0 {
			c.addressError(ExcADES, a, instr)
		}
	case 0x3A: // SWC2
		a := g[rs] + imm
		if !c.copUsable(2) {
			// 參照實作沒有這個檢查；ps1-tests cpu/cop 的實機紀錄 testSwc2Disabled 為 pass，
			// 代表 COP2 停用時 SWC2 會丟協處理器不可用例外（confirmed）。
			c.raise(ExcCOPU, instr)
		} else if a&3 != 0 {
			c.addressError(ExcADES, a, instr)
		} else {
			c.write(a, c.GTE.Read(int(rt)), 4)
		}
		c.doLDS()
	default:
		c.doLDS()
		c.raise(ExcRI, instr)
	}
}

// mergeBase 給 LWL／LWR：同一個暫存器有待寫入的 load 時，以那個值為底合併（不先寫回）。
func (c *CPU) mergeBase(rt uint32) uint32 {
	if c.ldWhich == rt {
		return c.ldValue
	}
	v := c.gpr[rt]
	c.doLDS()
	return v
}

func (c *CPU) addressError(code, addr, instr uint32) {
	c.cop0[CP0BADA] = addr
	c.raise(code, instr)
}

func (c *CPU) read32(a uint32) uint32 { return c.bus.Read32(a) }

func (c *CPU) write(a, v uint32, size int) {
	if c.cop0[CP0SR]&srIsC != 0 {
		c.bus.IsolatedWrite(a, v, size)
		return
	}
	switch size {
	case 1:
		c.bus.Write8(a, uint8(v))
	case 2:
		c.bus.Write16(a, uint16(v))
	default:
		c.bus.Write32(a, v)
	}
}

func (c *CPU) copUsable(n uint32) bool { return c.cop0[CP0SR]&(1<<(28+n)) != 0 }

func (c *CPU) cop0Op(instr uint32) {
	sub := (instr >> 21) & 0x1F
	rt := (instr >> 16) & 0x1F
	rd := (instr >> 11) & 0x1F
	val := c.gpr[rt]
	switch {
	case sub == 0x00: // MFC0
		switch rd {
		case 0, 1, 2, 4, 10:
			c.doLDS()
			c.raise(ExcRI, instr)
		case 3, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15:
			c.cancelLoadTo(rt)
			c.doLDS()
			c.startLoad(rt, c.cop0[rd])
		default:
			c.doLDS()
		}
	case sub == 0x04: // MTC0
		c.doLDS()
		switch rd {
		case 0, 1, 2, 4, 10:
			c.raise(ExcRI, instr)
		case CP0BPC, CP0BDA, CP0BDAM, CP0BPCM:
			c.cop0[rd] = val
		case CP0DCIC:
			c.cop0[rd] = val & 0xFF80003F
		case CP0CAUSE:
			c.cop0[rd] = c.cop0[rd]&^(3<<8) | val&(3<<8)
		case CP0SR:
			c.cop0[rd] = val &^ (3<<26 | 3<<23 | 3<<6)
		}
	case sub == 0x02 || sub == 0x06:
		c.doLDS()
		c.raise(ExcRI, instr)
	case sub == 0x08 || sub == 0x0C: // BC0F／BC0T：條件訊號恆為假
		c.doLDS()
		c.branch(instr&(1<<16) == 0, uint32(int32(int16(instr)))<<2, ^uint32(0), 0, false)
	case sub >= 0x10:
		c.doLDS()
		switch instr & 0x1F {
		case 0x10: // RFE
			sr := c.cop0[CP0SR]
			c.cop0[CP0SR] = sr&^0x0F | (sr>>2)&0x0F
		case 0x01, 0x02, 0x06, 0x08: // TLB 指令
			c.raise(ExcRI, instr)
		}
	default:
		c.doLDS()
	}
}

func (c *CPU) cop13Op(instr uint32) {
	c.doLDS()
	if !c.copUsable((instr >> 26) & 3) {
		c.raise(ExcCOPU, instr)
		return
	}
	sub := (instr >> 21) & 0x1F
	if sub == 0x08 || sub == 0x0C {
		c.branch(instr&(1<<16) == 0, uint32(int32(int16(instr)))<<2, ^uint32(0), 0, false)
	}
}

func (c *CPU) cop2Op(instr uint32) {
	sub := (instr >> 21) & 0x1F
	rt := (instr >> 16) & 0x1F
	rd := int((instr >> 11) & 0x1F)
	val := c.gpr[rt]
	if c.cop0[CP0SR]&srCU2 == 0 {
		c.doLDS()
		c.raise(ExcCOPU, instr)
		return
	}
	switch {
	case sub == 0x00: // MFC2
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, c.GTE.Read(rd))
	case sub == 0x02: // CFC2
		c.cancelLoadTo(rt)
		c.doLDS()
		c.startLoad(rt, c.GTE.Read(32+rd))
	case sub == 0x04: // MTC2
		c.doLDS()
		c.GTE.Write(rd, val)
	case sub == 0x06: // CTC2
		c.doLDS()
		c.GTE.Write(32+rd, val)
	case sub == 0x08 || sub == 0x0C:
		c.doLDS()
		c.branch(instr&(1<<16) == 0, uint32(int32(int16(instr)))<<2, ^uint32(0), 0, false)
	case sub >= 0x10:
		c.doLDS()
		c.GTE.Execute(instr)
	default:
		c.doLDS()
	}
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
