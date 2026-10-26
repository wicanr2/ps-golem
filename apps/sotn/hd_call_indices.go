// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import "github.com/wicanr2/ps-golem/machine"

func hdCallPrecisionIndices(ram []byte, p hdCallPlan, write bool) []uint32 {
	b := hdCallRAM(ram)
	seen := map[uint32]bool{}
	add := func(a uint32) {
		if a&0x1fffffff >= 2<<20 {
			panic("precision context outside RAM")
		}
		seen[(a&0x1fffffff)/4] = true
	}
	buffer := b.u32(0x8006c37c)
	for _, a := range []uint32{0x8006c37c, 0x80097940, 0x8009792c} {
		add(a)
	}
	for layer := 0; layer < 17; layer++ {
		a := uint32(0x80073084)
		if layer > 0 {
			a = 0x800730d8 + uint32(layer-1)*48
		}
		if !write {
			for _, off := range []uint32{0, 4, 24, 28, 32, 36, 40, 44} {
				add(a + off)
			}
		}
		if b.u16(a+28)&1 != 0 {
			if !write {
				for off := uint32(0); off < 12; off += 4 {
					add(b.u32(a+4) + off)
				}
			}
			for page := uint32(0); page < 24; page++ {
				add(buffer + 0x474 + (uint32(b.u16(a+24))+page)*4)
			}
		}
	}
	if write {
		for i := range p.Tiles {
			for off := uint32(0); off < 16; off += 4 {
				add(buffer + 0x117f4 + uint32(i)*16 + off)
			}
		}
		for i := 0; i < p.Modes; i++ {
			for off := uint32(0); off < 12; off += 4 {
				add(buffer + 0xc74 + uint32(i)*12 + off)
			}
		}
		// Actual stack pointer is provided by the CPU producer, not hardcoded.
	}
	r := make([]uint32, 0, len(seen))
	for k := range seen {
		r = append(r, k)
	}
	return r
}
func hdCallPrecisionClear(ram []byte, p hdCallPlan, cpu machine.CallCPUState) []uint32 {
	r := hdCallPrecisionIndices(ram, p, true)
	sp := cpu.GPR[29]
	for off := uint32(0x20); off < 0x48; off += 4 {
		r = append(r, ((sp-0x48+off)&0x1fffffff)/4)
	}
	for _, off := range []uint32{0x30, 0x38, 0x58, 0x54, 0x50, 0x78, 0x74, 0x70, 0x6c} {
		r = append(r, ((sp-off)&0x1fffffff)/4)
	}
	return r
}
