package cpu

// PrecisionTracker 是選用的呈現觀測器，只在載入、儲存、COP2 搬移與 ADDU 搬移呼叫。
// 參照 beetle-psx-libretro mednafen/psx/cpu.c、pgxp/pgxp_cpu.c、pgxp/pgxp_gte.c，
// commit e57860d82faa953dc0fc10f201fd583739bc2f7d。
// cpu.c、pgxp_gte.c 為 GPL-2.0-or-later；pgxp_cpu.c 原出處 pcsxr 為 GPLv3。
type PrecisionTracker interface {
	Load(rt, addr, raw, op uint32, now, ready uint64)
	Store(rt, addr, raw, op uint32, now uint64)
	Move(rd, rs, raw uint32, now uint64)
	ReadCOP2(rt uint32, reg int, raw uint32, now, ready uint64)
	WriteCOP2(rt uint32, reg int, source, result uint32, now uint64)
	LoadCOP2(reg int, addr, raw uint32)
	StoreCOP2(reg int, addr, raw uint32)
}

func (c *CPU) SetPrecisionTracker(p PrecisionTracker) { c.precision = p }
