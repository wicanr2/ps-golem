// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import (
	"encoding/binary"
	"github.com/wicanr2/ps-golem/machine"
)

type hdCallEffects struct {
	RAM, Scratch []byte
	CPU          machine.CallCPUState
	Plan         hdCallPlan
}

func buildHDCallEffects(ram, scratch []byte, before machine.CallCPUState) hdCallEffects {
	if len(scratch) != 1024 {
		panic("scratch size")
	}
	input := hdCallRAM(ram)
	g := before.GPR
	fg, bg := uint32(0x80073084), uint32(0x800730d8)
	if input.u32(fg+28) != 3 || input.u32(bg+28) != 3 || input.u32(bg+40) != 0 || input.u32(bg+44) != 1 {
		panic("state layer scope")
	}
	for layer := 1; layer < 16; layer++ {
		p := bg + uint32(layer)*48
		if input.u32(p+28) != 0 || input.u32(p+40) != 0 {
			panic("state extra background")
		}
	}
	initialSprites, initialModes := input.u32(0x80097940), input.u32(0x8009792c)
	if initialModes > 976 && !(initialModes == 1024 && initialSprites == 640) {
		panic("partial mode state not proved")
	}
	p := buildHDCallPlan(ram)
	b := hdCallRAM(p.RAM)
	s := append([]byte(nil), scratch...)
	sh := func(a int, v uint16) { binary.LittleEndian.PutUint16(s[a:], v) }
	sw := func(a int, v uint32) { binary.LittleEndian.PutUint32(s[a:], v) }
	// Shared sprite template, drawing offsets and the full texture window.
	// Original storage: scratch 0x1F800000..0x1F800047.
	s[3] = 3
	s[7] = 0x7c
	sh(0x10, input.u16(0x8006c39c))
	sh(0x12, input.u16(0x8006c3a0))
	sh(0x40, 0)
	sh(0x42, 0)
	sh(0x44, 255)
	sh(0x46, 255)
	cols, rows := int(input.u32(bg+32))*16, int(input.u32(bg+36))*16
	sx, sy := int(int16(input.u16(bg+10))), int(int16(input.u16(bg+14)))
	if sx < 0 || sy < 0 || sx/16+17 > cols || sy/16+16 > rows {
		panic("state full scan window")
	}
	data, defs := input.u32(bg), input.u32(bg+4)
	lastRow, lastCol := sy/16+15, sx/16+16
	// Last nonzero cell determines the remaining scan temporaries even when
	// the sprite budget is full. Selection is a grid operation, not ISA work.
	var page, gfx, palette uint32
	found := false
	for y := sy / 16; y <= lastRow; y++ {
		for x := sx / 16; x <= lastCol; x++ {
			id := uint32(input.u16(data + uint32(y*cols+x)*2))
			if id == 0 {
				continue
			}
			page = uint32(input.u8(input.u32(defs) + id))
			gfx = uint32(input.u8(input.u32(defs+4) + id))
			palette = uint32(input.u8(input.u32(defs+8) + id))
			found = true
		}
	}
	if !found {
		panic("state empty background unproved")
	}
	// Disabled trailing background records still publish their zero flags.
	sh(0x14, 0)
	sh(0x16, 0)
	sh(0x18, input.u16(bg+24))
	sh(0x1a, 0)
	sh(0x1c, 0)
	sh(0x1e, uint16(cols))
	sh(0x20, uint16(rows))
	sw(0x24, data+uint32(lastRow*cols+lastCol+1)*2)
	for n := 0; n < 3; n++ {
		sw(0x28+n*4, input.u32(defs+uint32(n)*4))
	}
	buffer := input.u32(0x8006c37c)
	ot := buffer + 0x474
	sw(0x34, ot)
	sw(0x38, buffer+0x117f4+b.u32(0x80097940)*16)
	sw(0x3c, buffer+0xc74+b.u32(0x8009792c)*12)
	if len(p.Tiles) > 0 {
		last := p.Tiles[len(p.Tiles)-1]
		s[7] = 0x7d
		sw(8, uint32(uint16(last.X))|uint32(uint16(last.Y))<<16)
		sw(12, uint32(last.U)|uint32(last.V)<<8|uint32(last.CLUT)<<16)
	}
	// Compatibility frames: own function saves the caller's callee-saved
	// registers; the draw-mode helper saves its caller and rectangle locals.
	// Layout evidence: 0x800ECC10 prologue and IDA helper export at
	// 0x800135F0/0x80013B24. The drawing itself remains the typed Go hdCallPlan.
	sp := g[29]
	frame := sp - 0x48
	for n, reg := range []int{16, 17, 18, 19, 20, 21, 22, 23, 30, 31} {
		b.set(frame+0x20+uint32(n)*4, g[reg])
	}
	b.set(frame+24, uint32(sx&15))
	if p.Modes > 0 {
		b.set(frame+16, 0x1f800040)
		b.set(sp-0x58, 0x1f800010)
		b.set(sp-0x54, 23)
		b.set(sp-0x50, 0x800ed4a4)
		for n := 0; n < 4; n++ {
			b.set(sp-0x78+uint32(n)*4, 0)
		}
	}
	// Architectural results of the completed ordinary scan. Clock, pipeline
	// deadlines and cache retirement are a separate still-DRAFT contract.
	if input.u32(fg+40) == 0 {
		g[1] = 0x1f800000
	} else {
		g[1] = 0x80070000
	}
	g[0], g[2], g[3] = 0, 0, 0
	if p.Modes > 0 {
		g[4] = ot + uint32(input.u16(bg+24)+23)*4
		g[5] = (buffer + 0xc74 + (b.u32(0x8009792c)-1)*12) & 0xffffff
		g[6], g[7] = 0xffffff, 0xff000000
	} else {
		g[4] = palette * 2
		g[5] = uint32(input.u16(0x8003c104 + palette*2))
		g[6] = (gfx & 0xf0)
		g[7] = uint32(lastCol + 1)
	}
	g[8] = gfx << 4
	g[9] = 17
	g[10] = page
	g[11] = uint32(17*16 - (sx & 15))
	g[12] = uint32(lastRow * cols)
	g[13] = uint32(16*16 - (sy & 15))
	g[14] = 0xffffff
	g[15] = uint32(sx / 16)
	g[24] = 0xff000000
	g[25] = uint32(sx & 15)
	g[34] = g[16]
	c := before
	c.GPR = g
	c.HI = uint32(0)
	c.LO = uint32(lastRow * cols)
	c.PC = g[31]
	c.NextPC = g[31] + 4
	c.LDWhich = uint32(34)
	c.LDValue = g[16]
	c.BDBT = uint32(0)
	c.Branched = false
	return hdCallEffects{b, s, c, p}
}
