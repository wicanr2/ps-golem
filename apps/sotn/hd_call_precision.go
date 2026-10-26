// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import (
	"fmt"
	"github.com/wicanr2/ps-golem/machine"
)

func hdCallPrecisionTimes(cpu machine.CallCPUState, clock hdCallClock, costs []uint32) map[uint32][2]uint64 {
	prefix := clock.CostPrefix
	if len(prefix) != len(costs)+1 || prefix[0] != cpu.Now {
		panic("typed clock prefix bounds")
	}
	wanted := []uint32{0x800ed0ec, 0x800ed104, 0x800ed4a4, 0x800ed4a8, 0x80013b44, 0x800ed2e8, 0x800ed51c, 0x800ed518, 0x800ed514, 0x800ed510, 0x800ed50c, 0x800ed508, 0x800ed504, 0x800ed500, 0x800ed20c, 0x800ed4fc, 0x800ed4f8}
	times := map[uint32][2]uint64{}
	i := 0
	for _, span := range clock.Spans {
		for _, pc := range wanted {
			if pc >= span.First && pc <= span.Last {
				j := i + int((pc-span.First)/4)
				times[pc] = [2]uint64{prefix[j], prefix[j+1]}
			}
		}
		i += int((span.Last-span.First)/4) + 1
	}
	if i != len(costs) || prefix[i] != clock.Now {
		panic("typed precision cost prefix differs")
	}
	return times
}

func buildHDCallPrecision(ram, scratch []byte, cpu machine.CallCPUState, before machine.CallPrecisionImage, effects hdCallEffects, clock hdCallClock, biu uint32, costs []uint32) machine.CallPrecisionImage {
	b := hdCallRAM(ram)
	g := cpu.GPR
	now := cpu.Now
	if b.u32(0x80097940) != 0 || b.u32(0x8009792c) != 0 || b.u32(0x800730ac) != 0 {
		panic("precision ordinary complete-buffer scope")
	}
	p := effects.Plan
	if p.Modes != 48 || len(p.Tiles) == 0 {
		panic("precision two-layer mode scope")
	}
	for i, v := range before.Regs {
		if !neutralHDCallPrecision(v) || v.Flags != 0 || (i != 1 && v.Raw != 0) {
			panic(fmt.Sprintf("precision non-neutral entry register %d", i))
		}
	}
	for i, l := range before.Loads {
		if !neutralHDCallPrecision(l.Value) || l.Value.Flags != 0 || (i != 1 && i != 13 && l.Value.Raw != 0) || (l.Pending && l.Ready >= now) {
			panic(fmt.Sprintf("precision unsupported entry load %d", i))
		}
	}
	for i, v := range before.Scratch[:18] {
		if !neutralHDCallPrecision(v) || v.Flags&^uint8(2) != 0 {
			panic(fmt.Sprintf("precision active scratch provenance %d", i))
		}
	}
	meta := func(a uint32) machine.CallPrecisionValue { return before.RAM[(a&0x1fffffff)/4] }
	requireZero := func(a uint32) {
		if meta(a) != (machine.CallPrecisionValue{}) {
			panic(fmt.Sprintf("precision non-neutral source %08x", a))
		}
	}
	for _, a := range []uint32{0x8006c37c, 0x80097940, 0x8009792c} {
		requireZero(a)
	}
	for layer := 0; layer < 17; layer++ {
		a := uint32(0x80073084)
		if layer > 0 {
			a = 0x800730d8 + uint32(layer-1)*48
		}
		if b.u16(a+28)&1 == 0 {
			requireZero(a + 40)
			requireZero(a + 28)
			continue
		}
		for _, off := range []uint32{0, 4} {
			requireZero(a + off)
		}
		// Active countdown can retain an invalid low half and a valid zero
		// upper half. Tracker.checked preserves that metadata on a zero LW.
		for _, off := range []uint32{24, 28, 32, 36, 40, 44} {
			v := meta(a + off)
			if !neutralHDCallPrecision(v) || v.Flags&^uint8(2) != 0 || (v.Flags&2 != 0 && v.Raw>>16 != b.u32(a+off)>>16) {
				panic("precision source scalar provenance")
			}
		}
		defs := b.u32(a + 4)
		for off := uint32(0); off < 12; off += 4 {
			requireZero(defs + off)
		}
		buffer := b.u32(0x8006c37c)
		for page := uint32(0); page < 24; page++ {
			requireZero(buffer + 0x474 + (uint32(b.u16(a+24))+page)*4)
		}
	}
	out := before
	out.RAM = map[uint32]machine.CallPrecisionValue{}
	for k, v := range before.RAM {
		out.RAM[k] = v
	}
	put := func(a uint32, v machine.CallPrecisionValue) {
		k := (a & 0x1fffffff) / 4
		if v == (machine.CallPrecisionValue{}) {
			delete(out.RAM, k)
		} else {
			out.RAM[k] = v
		}
	}
	buffer := b.u32(0x8006c37c)
	for i, t := range p.Tiles {
		a := buffer + 0x117f4 + uint32(i)*16
		for off := uint32(0); off < 16; off += 4 {
			put(a+off, machine.CallPrecisionValue{})
		}
		put(a+8, machine.CallPrecisionValue{Raw: uint32(uint16(t.X)) | uint32(uint16(t.Y))<<16})
	}
	for i := 0; i < p.Modes; i++ {
		for off := uint32(0); off < 12; off += 4 {
			put(buffer+0xc74+uint32(i)*12+off, machine.CallPrecisionValue{})
		}
	}
	for _, order := range []uint32{uint32(b.u16(0x8007309c)), uint32(b.u16(0x800730f0))} {
		for page := uint32(0); page < 24; page++ {
			put(buffer+0x474+(order+page)*4, machine.CallPrecisionValue{})
		}
	}
	put(0x80097940, machine.CallPrecisionValue{})
	put(0x8009792c, machine.CallPrecisionValue{})
	sp := g[29]
	for off := uint32(0x20); off < 0x48; off += 4 {
		put(sp-0x48+off, machine.CallPrecisionValue{})
	}
	for _, off := range []uint32{0x30, 0x58, 0x54, 0x50, 0x78, 0x74, 0x70, 0x6c} {
		put(sp-off, machine.CallPrecisionValue{})
	}
	put(sp-0x38, machine.CallPrecisionValue{Raw: uint32(b.u16(0x800730f4))})
	for _, off := range []uint32{0x70, 0x6c} {
		put(sp-off, machine.CallPrecisionValue{Raw: 255, Flags: 2})
	}
	// Full sprite copies, camera/layer fields, data cursors and texture-window
	// scratch effects have simple typed provenance under the neutral guard.
	for i := 0; i < 18; i++ {
		out.Scratch[i] = machine.CallPrecisionValue{}
	}
	last := p.Tiles[len(p.Tiles)-1]
	out.Scratch[2].Raw = uint32(uint16(last.X)) | uint32(uint16(last.Y))<<16
	out.Scratch[6].Raw = uint32(b.u16(0x800730f0))
	out.Scratch[7].Raw = b.u32(0x800730f8) * 16 << 16
	out.Scratch[8] = before.Scratch[8]
	out.Scratch[8].Raw = out.Scratch[8].Raw&0xffff0000 | b.u32(0x800730fc)*16
	out.Scratch[8].Flags &= 2
	out.Scratch[9].Raw = uint32(b.u16(0x800730f0))
	out.Scratch[17].Raw = 255 | 255<<16
	for _, r := range []int{8, 11, 13, 15} {
		if out.Loads[r].Pending {
			out.Regs[r] = out.Loads[r].Value
			out.Loads[r].Pending = false
		}
	}
	out.Regs[3] = machine.CallPrecisionValue{Flags: 2}
	out.Regs[4] = machine.CallPrecisionValue{Raw: uint32(b.u16(0x800730f4))}
	out.Regs[6] = machine.CallPrecisionValue{Raw: 255, Flags: 2}
	for _, r := range []int{7, 9} {
		out.Regs[r] = machine.CallPrecisionValue{}
		out.Loads[r] = machine.CallPrecisionLoad{}
	}
	times := hdCallPrecisionTimes(cpu, clock, costs)
	lastLoads := map[int]uint32{2: 0x800ed0ec, 3: 0x800ed104, 4: 0x800ed4a4, 5: 0x800ed4a8, 6: 0x80013b44, 10: 0x800ed2e8, 16: 0x800ed51c, 17: 0x800ed518, 18: 0x800ed514, 19: 0x800ed510, 20: 0x800ed50c, 21: 0x800ed508, 22: 0x800ed504, 23: 0x800ed500, 25: 0x800ed20c, 30: 0x800ed4fc, 31: 0x800ed4f8}
	for r, pc := range lastLoads {
		t, ok := times[pc]
		if !ok {
			panic("precision final load clock site missing")
		}
		v := machine.CallPrecisionValue{}
		if r == 3 {
			v.Flags = 2
		}
		if r == 4 {
			v = machine.CallPrecisionValue{Raw: uint32(b.u16(0x800730f0)), Flags: 2}
		}
		if r == 6 {
			v = machine.CallPrecisionValue{Raw: 255, Flags: 2}
		}
		out.Loads[r] = machine.CallPrecisionLoad{Value: v, Ready: t[1], Pending: r != 3 && r != 6}
	}
	_ = scratch
	_ = effects
	return out
}

func neutralHDCallPrecision(v machine.CallPrecisionValue) bool {
	v.Raw, v.Flags = 0, 0
	return v == (machine.CallPrecisionValue{})
}
