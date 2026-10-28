//go:build ignore

// Fixed own-engine CPU migration oracle, 026 §6. No original ROM/BIOS input.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/cpu"
	"os"
)

type bus struct {
	ram           []byte
	biu           uint32
	writes, reads uint64
}

const basis uint64 = 14695981039346656037

func hash(h uint64, v uint32) uint64 {
	for i := 0; i < 4; i++ {
		h = (h ^ uint64(byte(v>>uint(i*8)))) * 1099511628211
	}
	return h
}
func (b *bus) read(a uint32, n int) uint32 {
	v := uint32(0)
	for i := 0; i < n; i++ {
		v |= uint32(b.ram[(a+uint32(i))&65535]) << uint(i*8)
	}
	b.reads = hash(hash(hash(b.reads, a), uint32(n)), v)
	return v
}
func (b *bus) Read8(a uint32) uint8   { return uint8(b.read(a, 1)) }
func (b *bus) Read16(a uint32) uint16 { return uint16(b.read(a, 2)) }
func (b *bus) Read32(a uint32) uint32 { return b.read(a, 4) }
func (b *bus) write(a, v uint32, n int) {
	b.writes = hash(hash(hash(b.writes, a), uint32(n)), v)
	for i := 0; i < n; i++ {
		b.ram[(a+uint32(i))&65535] = byte(v >> uint(i*8))
	}
}
func (b *bus) Write8(a, v uint32)  { b.write(a, v, 1) }
func (b *bus) Write16(a, v uint32) { b.write(a, v, 2) }
func (b *bus) Write32(a, v uint32) { b.write(a, v, 4) }
func (b *bus) IsolatedWrite(a, v uint32, n int) {
	b.writes = hash(hash(hash(b.writes, a), uint32(n)|0x80000000), v)
}
func (b *bus) InstructionBusError(a uint32) bool {
	p := a & 0x1fffffff
	return p >= 0x1f800000 && p < 0x1fc00000
}
func (b *bus) ReadCost(a uint32, n int) int {
	if a&0x1ff80000 == 0x1fc00000 {
		return n * 6
	}
	return 4
}
func (b *bus) BIU() uint32 { return b.biu }
func memhash(ram []byte) uint64 {
	h := basis
	for _, v := range ram {
		h = (h ^ uint64(v)) * 1099511628211
	}
	return h
}
func main() {
	if len(os.Args) != 2 {
		panic("output tape path required")
	}
	f, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	put := func(v any) {
		if err := binary.Write(out, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	words := []uint32{}
	for fun := uint32(0); fun < 64; fun++ {
		words = append(words, 11<<21|12<<16|13<<11|7<<6|fun)
	}
	for op := uint32(0); op < 64; op++ {
		words = append(words, op<<26|11<<21|12<<16|0x2001)
	}
	for cop := uint32(0x10); cop <= 0x13; cop++ {
		for sub := uint32(0); sub < 32; sub++ {
			words = append(words, cop<<26|sub<<21|12<<16|9<<11|0x12)
		}
	}
	// Explicit arithmetic/alignment/CP0 register, LWL/LWR merge and IRQ boundaries.
	for rd := uint32(0); rd < 32; rd++ {
		words = append(words, 0x10<<26|12<<16|rd<<11, 0x10<<26|4<<21|12<<16|rd<<11)
	}
	for _, op := range []uint32{0x21, 0x22, 0x23, 0x25, 0x26, 0x29, 0x2a, 0x2b, 0x2e, 0x32, 0x3a} {
		for align := uint32(0); align < 4; align++ {
			words = append(words, op<<26|1<<21|10<<16|align)
		}
	}
	_, _ = out.WriteString("PSGCPU01")
	put(uint32(len(words)))
	rng := uint32(0xc0ffee)
	next := func() uint32 { rng = rng*1664525 + 1013904223; return rng }
	for index, word := range words {
		b := &bus{ram: make([]byte, 65536), biu: 0x800, reads: basis, writes: basis}
		if index%3 == 0 {
			b.biu = 0
		}
		if index%7 == 0 {
			b.biu |= 4
		}
		for i := range b.ram {
			b.ram[i] = byte(next() >> 24)
		}
		var regs [32]uint32
		for i := range regs {
			regs[i] = next()
		}
		regs[0] = 0
		regs[1] = 0x80008000
		regs[4] = 0x4040ff01
		if index%4 == 0 {
			regs[4] = 0x4040ff01 | 1<<16
		}
		if index%5 == 0 {
			regs[4] &= ^(uint32(1) << 30)
		}
		regs[11] = []uint32{0, 0x80000000, 0xffffffff, 0x80008000}[index%4]
		regs[12] = []uint32{0xffffffff, 0, 1, 0x7fffffff}[index%4]
		program := []uint32{0x10<<26 | 4<<21 | 4<<16 | 12<<11, 6<<21 | 0x11, 7<<21 | 0x13, 0x12<<26 | 4<<21 | 8<<16 | 9<<11, 0x12<<26 | 6<<21 | 9<<16, 0x23<<26 | 10<<16 | 0x2000, word, 0, 0x26<<26 | 1<<21 | 10<<16 | 3, 0, 0x12<<26 | 0x10<<21 | 0x80001, 0, 0x12<<26 | 12<<16 | 9<<11, 0, 0x12, 0}
		for i, v := range program {
			binary.LittleEndian.PutUint32(b.ram[0x1000+i*4:], v)
		}
		c := cpu.New(b)
		c.SetPC(0x80001000)
		for i, v := range regs {
			c.SetGPR(i, v)
		}
		put(uint32(index))
		put(b.biu)
		put(uint32(0x80001000))
		put(regs)
		put(b.ram)
		put(uint32(16))
		for step := 0; step < 16; step++ {
			irq := index%11 == 0 && step >= 6 && step < 10
			marker := uint32(0)
			if irq {
				marker = 1
			}
			put(marker)
			c.SetIRQ(irq)
			c.Step()
			s := c.SnapshotCallState()
			put(s.GPR)
			put([]uint32{s.HI, s.LO, s.PC, s.NextPC, s.BDBT, s.LDWhich, s.LDValue})
			put(s.COP0)
			branched := uint32(0)
			if s.Branched {
				branched = 1
			}
			put(branched)
			put([]uint64{s.Now, s.Cost, s.MultiplyReady, s.GTEReady})
			for _, line := range s.Cache {
				put(line.Tag)
				put(uint32(line.Valid))
			}
			for r := 0; r < 64; r++ {
				put(c.GTE.Read(r))
			}
			put(memhash(b.ram))
			put(b.writes)
			put(b.reads)
		}
	}
	if err := out.Flush(); err != nil {
		panic(err)
	}
	fmt.Printf("CPU_ORACLE: %d cases x 16 steps; full CPU/GTE/cache/time/RAM/access records\n", len(words))
}
