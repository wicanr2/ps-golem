// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import "github.com/wicanr2/ps-golem/machine"

type hdCallClockSpan struct {
	Phase       string
	First, Last uint32
	Start, End  uint64
	RAMReads    int
}

var hdCallRAMReadSites = []uint32{
	0x80011f24, 0x80011f28, 0x80011f3c, 0x8001360c, 0x80013630, 0x80013634, 0x80013638, 0x80013904,
	0x800ecc50, 0x800ecc98, 0x800eccb4, 0x800eccbc, 0x800ecce0, 0x800eccf8,
	0x800ecd2c, 0x800ecd4c, 0x800ecd54, 0x800ecd6c, 0x800ecd80, 0x800ecd84, 0x800ecd9c, 0x800ecda4, 0x800ecdac, 0x800ecdcc,
	0x800ece40, 0x800ece78, 0x800ecebc, 0x800ecedc, 0x800ecee0, 0x800ecef4, 0x800ecf04, 0x800ecf14,
	0x800ecfac, 0x800ecfb0, 0x800ecfdc, 0x800ecff4, 0x800ed038, 0x800ed0b0,
	0x800ed0ec, 0x800ed104, 0x800ed120, 0x800ed128, 0x800ed134, 0x800ed13c, 0x800ed148, 0x800ed150,
	0x800ed15c, 0x800ed168, 0x800ed178, 0x800ed188, 0x800ed1a0, 0x800ed20c, 0x800ed258, 0x800ed2c8,
	0x800ed2e8, 0x800ed2ec, 0x800ed300, 0x800ed310, 0x800ed32c, 0x800ed3c4, 0x800ed3c8, 0x800ed3f0, 0x800ed40c,
	0x800ed450, 0x800ed4c8, 0x800ed4f8, 0x800ed4fc, 0x800ed500, 0x800ed504, 0x800ed508,
	0x800ed50c, 0x800ed510, 0x800ed514, 0x800ed518, 0x800ed51c,
}

type hdCallClock struct {
	Now, Cost, MultiplyReady uint64
	Cache                    [256]machine.CallCacheLine
	Cached                   bool
	Steps, RAMReads          int
	FetchCycles, WaitCycles  uint64
	Spans                    []hdCallClockSpan
	Costs                    []uint32
	CostPrefix               []uint64
}

// Confirmed RAM read sites for this ordinary data contract, preserved in
// KSEG0 address space. Scratch reads have zero cost under 008 §2.1. The
// comparator independently counts the retained original bus observations.
var hdCallRAMReadIndex = func() [1 << 18]bool {
	var r [1 << 18]bool
	for _, pc := range hdCallRAMReadSites {
		p := pc & 0x1fffffff
		if p >= 1<<20 {
			panic("clock read-site bounds")
		}
		r[p/4] = true
	}
	return r
}()

// 008 §2.2 hardware-model cache transition, independent of game semantics.
// Source: ps-golem internal/cpu/cpu.go at 1810cb99a730b1ef810ed684e7df0f14323755e6,
// GPL-3.0-or-later. The earlier Beetle source/license is recorded in 008 §4.
// A span scans at most four validity bits per cache line, not ISA opcodes.
func (c *hdCallClock) fetch(first, last uint32) uint64 {
	if first > last || first&3 != 0 || last&3 != 0 {
		panic("clock footprint bounds")
	}
	if !c.Cached {
		c.Cost = 5
		return uint64((last-first)/4+1) * 4
	}
	var total uint64
	for base := first &^ 15; base <= last; base += 16 {
		lo, hi := uint32(0), uint32(3)
		if base < first {
			lo = (first - base) / 4
		}
		if base+12 > last {
			hi = (last - base) / 4
		}
		p := base & 0x1fffffff
		line := &c.Cache[(p>>4)&255]
		for word := lo; word <= hi; word++ {
			cost := uint64(0)
			if line.Tag != p>>12 || line.Valid&(1<<word) == 0 {
				fill := uint8(0xf << word & 0xf)
				if line.Tag == p>>12 {
					line.Valid |= fill
				} else {
					line.Tag = p >> 12
					line.Valid = fill
				}
				cost = uint64(7 - word)
			}
			total += cost
			c.Cost = 1 + cost
		}
	}
	return total
}
func (c *hdCallClock) run(phase string, first, last uint32) {
	start := c.Now
	reads := 0
	for pc := first; pc <= last; pc += 4 {
		p := pc & 0x1fffffff
		if p >= 1<<20 {
			panic("typed clock footprint bounds")
		}
		fetch := c.fetch(pc, pc)
		cost := uint64(1) + fetch
		if hdCallRAMReadIndex[p/4] {
			cost += 4
			reads++
		}
		c.Now += cost
		c.Cost = cost
		c.FetchCycles += fetch
		c.Steps++
		c.Costs = append(c.Costs, uint32(cost))
		c.CostPrefix = append(c.CostPrefix, c.Now)
	}
	c.RAMReads += reads
	c.Spans = append(c.Spans, hdCallClockSpan{phase, first, last, start, c.Now, reads})
}
func (c *hdCallClock) productReady() { c.MultiplyReady = c.Now + 7 }
func (c *hdCallClock) awaitProduct(phase string, pc uint32) {
	c.run(phase, pc, pc)
	if c.Now < c.MultiplyReady {
		wait := c.MultiplyReady - c.Now
		c.Now += wait
		c.Cost += wait
		c.WaitCycles += wait
		c.Costs[len(c.Costs)-1] += uint32(wait)
		c.CostPrefix[len(c.CostPrefix)-1] = c.Now
		c.Spans[len(c.Spans)-1].End = c.Now
	}
}
func (c *hdCallClock) drawMode(extendedEncoding bool) {
	c.run("mode call frame", 0x800135f0, 0x8001361c)
	c.run("mode environment", 0x80013900, 0x80013910)
	if extendedEncoding {
		c.run("mode environment extended", 0x80013914, 0x80013918)
		c.run("mode environment extended", 0x80013920, 0x80013924)
	} else {
		c.run("mode environment standard", 0x80013930, 0x80013934)
		c.run("mode environment standard", 0x8001393c, 0x80013940)
	}
	c.run("mode environment return", 0x80013948, 0x8001394c)
	c.run("texture window call", 0x80013620, 0x80013628)
	c.run("texture window", 0x80013b24, 0x80013b28)
	c.run("texture window", 0x80013b34, 0x80013ba4)
	c.run("mode call return", 0x8001362c, 0x80013644)
}

func clockForHDCall(ram []byte, before machine.CallCPUState, biu uint32) hdCallClock {
	b := hdCallRAM(ram)
	c := hdCallClock{Cached: biu&0x800 != 0}
	c.Now, c.Cache, c.MultiplyReady = before.Now, before.Cache, before.MultiplyReady
	c.Costs = make([]uint32, 0, 60000)
	c.CostPrefix = make([]uint64, 1, 60001)
	c.CostPrefix[0] = c.Now
	cp0 := before.COP0
	if cp0[12]&0x10000 != 0 || cp0[12]&cp0[13]&0xff00 != 0 && cp0[12]&1 != 0 {
		panic("clock isolated cache/IRQ context")
	}
	extendedEncoding := b.u32(0x8002c26c) != 0
	sprites, modes := b.u32(0x80097940), b.u32(0x8009792c)
	c.run("call setup", 0x800ecc10, 0x800ecd10)
	if b.u32(0x800730ac) != 0 {
		c.run("foreground countdown", 0x800ecd14, 0x800ecd24)
	} else {
		c.run("foreground data", 0x800ecd28, 0x800ecdf8)
		c.run("foreground scan setup", 0x800ece14, 0x800ece38)
		data := b.u32(0x80073084)
		cols := int(b.u32(0x800730a4)) * 16
		sx, sy := int(int16(b.u16(0x8007308e))), int(int16(b.u16(0x80073092)))
		for y := sy / 16; y < sy/16+16; y++ {
			if y < 0 || y >= 256 {
				panic("clock multiply latency scope")
			}
			c.run("foreground row", 0x800ece3c, 0x800ece6c)
			c.productReady()
			c.run("foreground row source", 0x800ece70, 0x800ece78)
			c.awaitProduct("foreground row product", 0x800ece7c)
			c.run("foreground row source", 0x800ece80, 0x800ece8c)
			for x := sx / 16; x < sx/16+17; x++ {
				c.run("foreground cell", 0x800ece90, 0x800ecec8)
				id := b.u16(data + uint32(y*cols+x)*2)
				if id != 0 {
					c.run("foreground material", 0x800ececc, 0x800ecf0c)
					c.run("foreground budget", 0x800ecf14, 0x800ecf24)
					if sprites < 640 {
						c.run("foreground sprite", 0x800ecf28, 0x800ecf40)
						c.run("foreground sprite/link", 0x800ecf50, 0x800ed004)
						sprites++
					}
				}
				c.run("foreground next cell", 0x800ed008, 0x800ed018)
			}
			c.run("foreground next row", 0x800ed01c, 0x800ed028)
		}
		c.run("foreground modes", 0x800ed02c, 0x800ed034)
		for page := 0; page < 24; page++ {
			c.run("foreground mode budget", 0x800ed038, 0x800ed048)
			if modes >= 1024 {
				break
			}
			c.run("foreground mode", 0x800ed04c, 0x800ed068)
			c.run("foreground mode call", 0x800ed070, 0x800ed088)
			c.drawMode(extendedEncoding)
			c.run("foreground mode link", 0x800ed08c, 0x800ed0a4)
			c.run("mode link", 0x80011f18, 0x80011f50)
			c.run("foreground next mode", 0x800ed0a8, 0x800ed0c8)
			modes++
		}
		c.run("foreground end", 0x800ed0cc, 0x800ed0cc)
	}
	c.run("background setup", 0x800ed0d0, 0x800ed0e8)
	for layer := 0; layer < 16; layer++ {
		base := uint32(0x800730d8 + layer*48)
		c.run("background countdown", 0x800ed0ec, 0x800ed0f8)
		if b.u32(base+40) != 0 {
			panic("background countdown clock unproved")
		}
		c.run("background flags", 0x800ed104, 0x800ed114)
		if b.u16(base+28)&1 != 0 {
			c.run("background data", 0x800ed118, 0x800ed1b8)
			c.run("background data", 0x800ed1c4, 0x800ed1d4)
			c.run("background scan setup", 0x800ed1ec, 0x800ed200)
			data := b.u32(base)
			cols := int(b.u32(base+32)) * 16
			sx, sy := int(int16(b.u16(base+10))), int(int16(b.u16(base+14)))
			for y := sy / 16; y < sy/16+16; y++ {
				if y < 0 || y >= 256 {
					panic("clock multiply latency scope")
				}
				c.run("background row", 0x800ed204, 0x800ed220)
				c.run("background row bounds/product", 0x800ed228, 0x800ed24c)
				c.productReady()
				c.run("background row source", 0x800ed250, 0x800ed258)
				c.awaitProduct("background row product", 0x800ed25c)
				c.run("background row source", 0x800ed260, 0x800ed26c)
				for x := sx / 16; x < sx/16+17; x++ {
					c.run("background cell flags", 0x800ed270, 0x800ed280)
					c.run("background cell", 0x800ed29c, 0x800ed2d4)
					id := b.u16(data + uint32(y*cols+x)*2)
					if id != 0 {
						c.run("background material", 0x800ed2d8, 0x800ed318)
						c.run("background budget", 0x800ed32c, 0x800ed33c)
						if sprites < 640 {
							c.run("background sprite", 0x800ed340, 0x800ed358)
							c.run("background sprite/link", 0x800ed368, 0x800ed41c)
							sprites++
						}
					}
					c.run("background next cell", 0x800ed420, 0x800ed430)
				}
				c.run("background next row", 0x800ed434, 0x800ed440)
			}
			c.run("background modes", 0x800ed444, 0x800ed44c)
			for page := 0; page < 24; page++ {
				c.run("background mode budget", 0x800ed450, 0x800ed460)
				if modes >= 1024 {
					break
				}
				c.run("background mode", 0x800ed464, 0x800ed480)
				c.run("background mode call", 0x800ed488, 0x800ed4a0)
				c.drawMode(extendedEncoding)
				c.run("background mode link", 0x800ed4a4, 0x800ed4bc)
				c.run("mode link", 0x80011f18, 0x80011f50)
				c.run("background next mode", 0x800ed4c0, 0x800ed4e0)
				modes++
			}
		}
		c.run("next background", 0x800ed4e4, 0x800ed4f4)
	}
	c.run("call return", 0x800ed4f8, 0x800ed528)
	return c
}
