// Package pgxp 保存呈現用的平行精度，不寫回機器狀態。
// 規格：docs/spec/016-hd-output.md §5。
// 參照 beetle-psx-libretro pgxp/pgxp_gte.c（GPL-2.0-or-later）、
// pgxp/pgxp_cpu.c、pgxp/pgxp_mem.c、pgxp/pgxp_value.c（原出處 pcsxr，GPLv3），
// commit e57860d82faa953dc0fc10f201fd583739bc2f7d。
package pgxp

import "math"

// Value 的兩個旗標分別對應低、高半字；只有兩者有效才能當作頂點。
type Value struct {
	X, Y             float32
	Z                float32
	depthX, depthY   uint64
	depthZX, depthZY float32
	Raw              uint32
	Flags            uint8
}

const (
	ValidX  uint8 = 1
	ValidY  uint8 = 2
	ValidXY       = ValidX | ValidY
)

func (v Value) Valid(raw uint32) bool { return v.Flags == ValidXY && v.Raw == raw }

func (v *Value) refreshDepth() {
	v.Z = 0
	if v.Flags == ValidXY && v.depthX != 0 && v.depthX == v.depthY && v.depthZX == v.depthZY && v.depthZX > 0 {
		v.Z = v.depthZX
	}
}
func checked(v Value, raw uint32) Value {
	if uint16(v.Raw) != uint16(raw) {
		v.Flags &^= ValidX
		v.depthX = 0
	}
	if uint16(v.Raw>>16) != uint16(raw>>16) {
		v.Flags &^= ValidY
		v.depthY = 0
	}
	v.refreshDepth()
	return v
}

type delayed struct {
	value   Value
	ready   uint64
	pending bool
}

// Tracker 只由機器執行緒使用；送進影子 GPU 的 Value 一律按值複製。
type Tracker struct {
	ram         []Value
	scratch     [256]Value
	port        Value
	regs        [32]Value
	loads       [32]delayed
	gte         [64]Value
	RTPS, RTPT  uint64
	depthSerial uint64
}

func New() *Tracker { return &Tracker{ram: make([]Value, 2<<20/4)} }

func (t *Tracker) memory(addr uint32) *Value {
	p := addr & 0x1fffffff
	if addr >= 0xc0000000 {
		return nil
	}
	switch {
	case p < 0x800000:
		return &t.ram[(p&0x1fffff)>>2]
	case p >= 0x1f800000 && p < 0x1f800400:
		return &t.scratch[(p-0x1f800000)>>2]
	case p&^3 == 0x1f801810:
		return &t.port
	}
	return nil
}

// ReadMemory 以實際收到的字驗證來源。DMA 或其他寫入造成的不符在此回退。
func (t *Tracker) ReadMemory(addr, raw uint32) Value {
	if p := t.memory(addr); p != nil {
		*p = checked(*p, raw)
		return *p
	}
	return Value{}
}

func (t *Tracker) reg(r uint32, raw uint32, now uint64) Value {
	if r == 0 {
		return Value{}
	}
	d := &t.loads[r]
	if d.pending && now > d.ready {
		t.regs[r], d.pending = d.value, false
	}
	t.regs[r] = checked(t.regs[r], raw)
	return t.regs[r]
}

// Load 的 ready 是本指令結束週期。下一條指令仍讀舊值，之後才啟用影子。
// 不在 CPU 的通用分派或延遲槽處新增掛點。
func (t *Tracker) Load(rt, addr, raw, op uint32, now, ready uint64) {
	if rt == 0 {
		return
	}
	// 同目標連續載入會取消舊載入；已過延遲槽的載入則先保留。
	d := &t.loads[rt]
	if d.pending && now > d.ready {
		t.regs[rt] = d.value
	}
	v := Value{}
	switch op {
	case 0x23, 0x22, 0x26:
		v = t.ReadMemory(addr, raw)
	case 0x21, 0x25:
		if p := t.memory(addr); p != nil {
			v = *p
			v.Z = 0
			if addr&2 != 0 {
				v.X, v.Flags, v.Raw = v.Y, (v.Flags>>1)&1, v.Raw>>16
				v.depthX, v.depthZX = v.depthY, v.depthZY
			}
			if uint16(v.Raw) != uint16(raw) {
				v.Flags = 0
				v.depthX = 0
			}
			v.depthY, v.depthZY = 0, 0
			v.Y = 0
			if op == 0x21 && int32(raw) < 0 {
				v.Y = -1
			}
			v.Flags = v.Flags&ValidX | ValidY
			v.Raw = raw
		}
	}
	t.loads[rt] = delayed{v, ready, true}
}

func (t *Tracker) store(addr uint32, v Value, raw, op uint32) {
	p := t.memory(addr)
	if p == nil {
		return
	}
	switch op {
	case 0x28:
		*p = Value{}
	case 0x29:
		p.Z = 0
		if uint16(v.Raw) != uint16(raw) {
			v.Flags &^= ValidX
			v.depthX = 0
		}
		if addr&2 == 0 {
			p.depthX, p.depthZX = v.depthX, v.depthZX
			p.X, p.Raw, p.Flags = v.X, p.Raw&0xffff0000|raw&0xffff, p.Flags&ValidY|v.Flags&ValidX
		} else {
			p.depthY, p.depthZY = v.depthX, v.depthZX
			p.Y, p.Raw, p.Flags = v.X, p.Raw&0xffff|(raw&0xffff)<<16, p.Flags&ValidX|(v.Flags&ValidX)<<1
		}
		p.refreshDepth()
	default:
		*p = checked(v, raw)
	}
}

func (t *Tracker) Store(rt, addr, raw, op uint32, now uint64) {
	t.store(addr, t.reg(rt, raw, now), raw, op)
}

func (t *Tracker) Move(rd, rs, raw uint32, now uint64) {
	v := t.reg(rs, raw, now)
	if rd != 0 {
		t.regs[rd], t.loads[rd] = v, delayed{}
	}
}

func (t *Tracker) readGTE(reg int, raw uint32) Value {
	if reg == 15 {
		reg = 14
	}
	t.gte[reg] = checked(t.gte[reg], raw)
	return t.gte[reg]
}

func (t *Tracker) writeGTE(reg int, v Value, raw uint32) {
	v = checked(v, raw)
	v.Raw = raw
	switch reg {
	case 15:
		t.gte[12], t.gte[13], t.gte[14], t.gte[15] = t.gte[13], t.gte[14], v, v
	case 14:
		t.gte[14], t.gte[15] = v, v
	case 29, 31:
		return
	default:
		t.gte[reg] = v
	}
}

func (t *Tracker) ReadCOP2(rt uint32, reg int, raw uint32, now, ready uint64) {
	if rt == 0 {
		return
	}
	d := &t.loads[rt]
	if d.pending && now > d.ready {
		t.regs[rt] = d.value
	}
	t.loads[rt] = delayed{t.readGTE(reg, raw), ready, true}
}

func (t *Tracker) WriteCOP2(rt uint32, reg int, source, result uint32, now uint64) {
	t.writeGTE(reg, t.reg(rt, source, now), result)
}

func (t *Tracker) LoadCOP2(reg int, addr, raw uint32) {
	t.writeGTE(reg, t.ReadMemory(addr, raw), raw)
}

func (t *Tracker) StoreCOP2(reg int, addr, raw uint32) {
	t.store(addr, t.readGTE(reg, raw), raw, 0x2b)
}

// Projected 保留既有只有 XY 的觀測介面；深度由 ProjectedDepth 提供。
func (t *Tracker) Projected(x, y float32, raw uint32, vertex int, last bool) {
	if vertex == 0 {
		if last {
			t.RTPS++
		} else {
			t.RTPT++
		}
	}
	v := Value{X: x, Y: y, Raw: raw, Flags: ValidXY}
	t.gte[12], t.gte[13], t.gte[14], t.gte[15] = t.gte[13], t.gte[14], v, v
}

// ProjectedDepth 按022保存呈現用深度與唯一來源，不改機器狀態。
func (t *Tracker) ProjectedDepth(x, y, z float32, raw uint32, vertex int, last bool) {
	t.Projected(x, y, raw, vertex, last)
	t.depthSerial++
	if t.depthSerial == 0 {
		panic("PGXP depth provenance exhausted")
	}
	if z > 0 && !math.IsInf(float64(z), 0) {
		v := &t.gte[14]
		v.Z = z
		v.depthX = t.depthSerial
		v.depthY = t.depthSerial
		v.depthZX = z
		v.depthZY = z
		t.gte[15] = *v
	}
}

func (t *Tracker) RepeatRAMCountdown(rt, counterAddress, pollAddress, initialCounter, pollValue uint32, now, iterations uint64) {
	// 保留全部原始 PGXP 回呼及時間戳；不讀原生 RAM，也不省略 metadata 狀態轉移。
	for i := uint64(0); i < iterations; i++ {
		value := initialCounter - uint32(i)
		at := now + i*26
		t.Load(rt, counterAddress, value, 0x23, at, at+5)
		t.Store(rt, counterAddress, value-1, 0x2b, at+7)
		t.Load(rt, counterAddress, value-1, 0x23, at+8, at+13)
		t.Load(rt, pollAddress, pollValue, 0x23, at+17, at+22)
	}
}
