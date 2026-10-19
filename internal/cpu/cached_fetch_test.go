package cpu

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"math/rand"
	"reflect"
	"testing"
)

// 純 CPU／RAM 語料，不提供或代替 BIOS；原版整合另用真實 BIOS 與公開語料。
type fetchProbeBus struct {
	ram []byte
	biu uint32
}

func (b *fetchProbeBus) FetchMemory() ([]byte, []byte, *uint32) { return b.ram, nil, &b.biu }
func (b *fetchProbeBus) BIU() uint32                            { return b.biu }
func (b *fetchProbeBus) Read8(a uint32) uint8                   { return b.ram[a&4095] }
func (b *fetchProbeBus) Read16(a uint32) uint16                 { return binary.LittleEndian.Uint16(b.ram[a&4094:]) }
func (b *fetchProbeBus) Read32(a uint32) uint32                 { return binary.LittleEndian.Uint32(b.ram[a&4092:]) }
func (b *fetchProbeBus) ReadCost(uint32, int) int               { return 4 }
func (b *fetchProbeBus) InstructionBusError(uint32) bool        { return false }
func (b *fetchProbeBus) Write8(uint32, uint32)                  {}
func (b *fetchProbeBus) Write16(uint32, uint32)                 {}
func (b *fetchProbeBus) Write32(uint32, uint32)                 {}
func (b *fetchProbeBus) IsolatedWrite(uint32, uint32, int)      {}

// referenceStep 保留 6271e767 的 cpu.step 全文，逐步比對完整 CPU 狀態及週期。
// 原 cpu.go SHA-256：e9487123bceaacbc09b98d86ec5510a43fb348a4d00fbe4d3904ae84c059af01。
func (c *CPU) referenceStep() {
	c.gpr[0] = 0
	c.branched = false
	pc := c.pc

	if pc&3 != 0 {
		c.cop0[CP0BADA] = pc
		c.raise(ExcADEL, 0)
		c.advance()
		return
	}

	instr, ok := c.fastFetch(pc)
	if !ok {
		if c.bus.InstructionBusError(pc) {
			c.raise(ExcIBE, 0)
			c.advance()
			return
		}
		c.cost += c.fetchCost(pc)
		instr = c.bus.Read32(pc)
	}

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

func TestCachedRAMFetchMatchesOriginalStep(t *testing.T) {
	b := &fetchProbeBus{ram: make([]byte, 4096)}
	seed := uint32(18018)
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed }
	bases := []uint32{0, 0x80000000, 0x80200000, 0x80600000, 0x807ff000, 0xa0000000, 0x80800000, 0xbfc00000}
	for n := 0; n < 4096; n++ {
		b.biu = []uint32{0, 0x800, 0x880}[n%3]
		pc := bases[n%len(bases)] + next()&0xffc
		if n%17 == 0 {
			pc += 2
		}
		binary.LittleEndian.PutUint32(b.ram[pc&4092:], 0x34020000|next()&0xffff)
		c := New(b)
		c.SetPC(pc)
		c.now = uint64(next())
		if n%23 == 0 {
			c.fram = nil
		}
		if n%5 == 0 {
			c.bdbt = 2 + uint32(n%2)
		}
		c.ldWhich, c.ldValue = uint32(n%31+1), next()
		if n%7 == 0 {
			c.cop0[CP0SR] |= 0x401
			c.cop0[CP0CAUSE] = 0x400
		}
		for i := range c.icache {
			c.icache[i] = icLine{tag: next() & 0x1ffff, valid: uint8(next())}
		}
		line := &c.icache[(pc>>4)&255]
		if n%4 != 0 {
			line.tag = (pc & 0x1fffffff) >> 12
			line.valid = uint8(n % 16)
		}
		want := *c
		cost := c.Step()
		want.cost = 1
		want.referenceStep()
		want.now += want.cost
		if cost != int(want.cost) || !reflect.DeepEqual(*c, want) {
			t.Fatalf("case=%d pc=%08x biu=%x cost=%d/%d", n, pc, b.biu, cost, want.cost)
		}
	}
}

type pollingProbeBus struct {
	ram []byte
	biu uint32
}

func (b *pollingProbeBus) FetchMemory() ([]byte, []byte, *uint32) { return b.ram, nil, &b.biu }
func (b *pollingProbeBus) PollingRAM() []byte                     { return b.ram }
func (b *pollingProbeBus) BIU() uint32                            { return b.biu }
func (b *pollingProbeBus) Read8(a uint32) uint8                   { return b.ram[a&uint32(len(b.ram)-1)] }
func (b *pollingProbeBus) Read16(a uint32) uint16 {
	return binary.LittleEndian.Uint16(b.ram[a&uint32(len(b.ram)-2):])
}
func (b *pollingProbeBus) Read32(a uint32) uint32 {
	return binary.LittleEndian.Uint32(b.ram[a&uint32(len(b.ram)-4):])
}
func (b *pollingProbeBus) ReadCost(uint32, int) int        { return 4 }
func (b *pollingProbeBus) InstructionBusError(uint32) bool { return false }
func (b *pollingProbeBus) Write8(a, v uint32)              { b.ram[a&uint32(len(b.ram)-1)] = byte(v) }
func (b *pollingProbeBus) Write16(a, v uint32) {
	binary.LittleEndian.PutUint16(b.ram[a&uint32(len(b.ram)-2):], uint16(v))
}
func (b *pollingProbeBus) Write32(a, v uint32) {
	binary.LittleEndian.PutUint32(b.ram[a&uint32(len(b.ram)-4):], v)
}
func (b *pollingProbeBus) IsolatedWrite(a, v uint32, n int) {}
func pollingI(op, rs, rt uint32, imm int16) uint32 {
	return op<<26 | rs<<21 | rt<<16 | uint32(uint16(imm))
}
func pollingFixture(counter uint32, precision bool) (*CPU, *pollingProbeBus, *pgxp.Tracker) {
	b := &pollingProbeBus{ram: make([]byte, 8192), biu: 0x800}
	start := uint32(0x80000200)
	target := start + 64
	first := pollingI(0x23, 29, 5, 20)
	words := []uint32{first, 0, pollingI(9, 5, 5, -1), pollingI(0x2b, 29, 5, 20), first, 0, pollingI(5, 5, 0, 9), 0}
	tail := []uint32{pollingI(15, 0, 5, int16(0x8000-0x10000)), pollingI(0x23, 5, 5, 0x1800), 0, 5<<21 | 6<<16 | 5<<11 | 0x2a, pollingI(5, 5, 0, -21), 0}
	for i, w := range words {
		b.Write32(start+uint32(i)*4, w)
	}
	for i, w := range tail {
		b.Write32(target+uint32(i)*4, w)
	}
	b.Write32(0x1000, counter)
	b.Write32(0x1800, 7)
	c := New(b)
	c.SetPC(start)
	c.gpr[29] = 0x80000fec
	c.gpr[6] = 9
	c.ldValue = 0x98765432
	c.gpr[34] = 123
	c.now = 12345
	for i := range words {
		c.icacheCost(start + uint32(i)*4)
	}
	for i := range tail {
		c.icacheCost(target + uint32(i)*4)
	}
	var p *pgxp.Tracker
	if precision {
		p = pgxp.New()
		p.Projected(1.25, 2.75, counter, 0, true)
		p.StoreCOP2(14, 0x80001000, counter)
		p.Projected(-3.5, 8.125, 7, 0, true)
		p.StoreCOP2(14, 0x80001800, 7)
		p.Load(5, 0x80001800, 7, 0x23, c.now-20, c.now-15)
		c.SetPrecisionTracker(p)
	}
	return c, b, p
}
func pollingComparable(c *CPU) CPU {
	r := *c
	r.bus = nil
	r.fram = nil
	r.fbios = nil
	r.fbiu = nil
	r.precision = nil
	return r
}
func TestRAMCountdownFullState(t *testing.T) {
	counters := []uint32{0, 1, 2, 3, 0xffff, 0x10000, 0x10001, 0xffffffff}
	budgets := []uint64{0, 25, 26, 27, 52, 260, 26624, 30000}
	for _, counter := range counters {
		for _, budget := range budgets {
			for _, precision := range []bool{false, true} {
				c, b, p := pollingFixture(counter, precision)
				ref, rb, rp := pollingFixture(counter, precision)
				old := *c
				cycles := c.TryRAMCountdown(budget)
				if cycles == 0 {
					if !reflect.DeepEqual(old, *c) {
						t.Fatal("rejection mutated CPU")
					}
					if budget >= 26 && counter != 1 {
						t.Fatalf("unexpected rejection counter=%x budget=%d", counter, budget)
					}
					continue
				}
				if cycles%26 != 0 || cycles > budget {
					t.Fatal("budget")
				}
				var actual uint64
				for actual < cycles {
					actual += uint64(ref.dispatchReferenceStep())
				}
				if actual != cycles || !reflect.DeepEqual(pollingComparable(c), pollingComparable(ref)) || !reflect.DeepEqual(b.ram, rb.ram) || !reflect.DeepEqual(p, rp) {
					t.Fatalf("counter=%x budget=%d pgxp=%v cycles=%d/%d got=%+v want=%+v metadata=%v", counter, budget, precision, cycles, actual, pollingComparable(c), pollingComparable(ref), reflect.DeepEqual(p, rp))
				}
			}
		}
	}
}
func TestRAMCountdownRejectsChangedProgram(t *testing.T) {
	offsets := []uint32{0, 4, 8, 12, 16, 20, 24, 28, 64, 68, 72, 76, 80, 84}
	for _, off := range offsets {
		c, b, _ := pollingFixture(100, false)
		change := uint32(1)
		if off == 64 {
			change = 1 << 26
		}
		b.Write32(c.pc+off, b.Read32(c.pc+off)^change)
		before := *c
		ram := append([]byte(nil), b.ram...)
		if c.TryRAMCountdown(260) != 0 || !reflect.DeepEqual(before, *c) || !reflect.DeepEqual(ram, b.ram) {
			t.Fatalf("changed offset %d accepted/mutated", off)
		}
	}
	cases := []func(*CPU, *pollingProbeBus){
		func(c *CPU, b *pollingProbeBus) { c.icache = [256]icLine{} },
		func(c *CPU, b *pollingProbeBus) { c.cop0[CP0SR] |= srIsC },
		func(c *CPU, b *pollingProbeBus) { c.cop0[CP0SR] |= 0x401; c.cop0[CP0CAUSE] |= 0x400 },
		func(c *CPU, b *pollingProbeBus) { c.ldWhich = 8 },
		func(c *CPU, b *pollingProbeBus) { c.bdbt = 3 },
		func(c *CPU, b *pollingProbeBus) { b.biu = 0 },
		func(c *CPU, b *pollingProbeBus) { c.gpr[29]++ },
		func(c *CPU, b *pollingProbeBus) { c.gpr[29] = 0x1f801800 - 20 },
		func(c *CPU, b *pollingProbeBus) { c.gpr[29] = c.pc - 20 },
		func(c *CPU, b *pollingProbeBus) { c.gpr[29] = 0x80001800 - 20 },
		func(c *CPU, b *pollingProbeBus) { b.Write32(0x1800, 9) },
	}
	for i, change := range cases {
		c, b, _ := pollingFixture(100, false)
		change(c, b)
		old := *c
		ram := append([]byte(nil), b.ram...)
		if c.TryRAMCountdown(260) != 0 || !reflect.DeepEqual(old, *c) || !reflect.DeepEqual(ram, b.ram) {
			t.Fatalf("guard %d failed", i)
		}
	}
}

// 未承諾無觀測 RAM 或 PGXP 時，一律逐條解譯。
type opaqueCountdownBus struct{ Bus }
type opaqueCountdownPrecision struct{ PrecisionTracker }

func TestRAMCountdownRejectsObservers(t *testing.T) {
	for _, precision := range []bool{false, true} {
		c, b, _ := pollingFixture(100, precision)
		if precision {
			c.precision = opaqueCountdownPrecision{c.precision}
		} else {
			c.bus = opaqueCountdownBus{c.bus}
		}
		before := *c
		ram := append([]byte(nil), b.ram...)
		if c.TryRAMCountdown(260) != 0 || !reflect.DeepEqual(before, *c) || !reflect.DeepEqual(ram, b.ram) {
			t.Fatalf("observer accepted or mutated: precision=%v", precision)
		}
	}
	c, b, _ := pollingFixture(100, false)
	c.fram = append([]byte(nil), c.fram...)
	before := *c
	ram := append([]byte(nil), b.ram...)
	if c.TryRAMCountdown(260) != 0 || !reflect.DeepEqual(before, *c) || !reflect.DeepEqual(ram, b.ram) {
		t.Fatal("different backing RAM accepted")
	}
}

func TestRAMCountdownGeneralTerminal(t *testing.T) {
	for _, terminal := range []uint32{0, 1, 0x10000, 0xffffffff} {
		for _, counter := range []uint32{terminal, terminal + 1, terminal + 2, terminal - 1, 0, 0xffffffff} {
			for _, budget := range []uint64{26, 52, 26624} {
				for _, precision := range []bool{false, true} {
					c, b, p := pollingFixture(counter, precision)
					ref, rb, rp := pollingFixture(counter, precision)
					for _, entry := range []struct {
						c *CPU
						b *pollingProbeBus
					}{{c, b}, {ref, rb}} {
						entry.b.Write32(entry.c.pc+24, pollingI(5, 5, 3, 9))
						entry.c.gpr[3] = terminal
					}
					cycles := c.TryRAMCountdown(budget)
					if cycles == 0 && counter-terminal != 1 {
						t.Fatalf("unexpected rejection terminal=%x counter=%x", terminal, counter)
					}
					var actual uint64
					for actual < cycles {
						actual += uint64(ref.dispatchReferenceStep())
					}
					if actual != cycles || !reflect.DeepEqual(pollingComparable(c), pollingComparable(ref)) || !reflect.DeepEqual(b.ram, rb.ram) || !reflect.DeepEqual(p, rp) {
						t.Fatalf("terminal=%x counter=%x budget=%d pgxp=%v", terminal, counter, budget, precision)
					}
				}
			}
		}
	}
	c, b, _ := pollingFixture(100, false)
	b.Write32(c.pc+24, pollingI(5, 5, 5, 9))
	old := *c
	if c.TryRAMCountdown(260) != 0 || !reflect.DeepEqual(old, *c) {
		t.Fatal("BNE temp,temp accepted")
	}
}

func (c *CPU) stepDispatchReference() {
	c.gpr[0] = 0
	c.branched = false
	pc := c.pc

	if pc&3 != 0 {
		c.cop0[CP0BADA] = pc
		c.raise(ExcADEL, 0)
		c.advance()
		return
	}

	// 008-cycle-model.md §6：只縮短既有 KSEG0 RAM 取指路徑，保留週期與順序。
	var instr uint32
	var ok bool
	// 只縮短已快取 KSEG0 RAM 的既有取字與時間查詢，不存新的指令快取。
	if c.fram != nil && pc&0xff800000 == 0x80000000 && *c.fbiu&0x800 != 0 {
		p := pc & 0x1fffffff
		line := &c.icache[(p>>4)&0xff]
		if line.tag != p>>12 || line.valid&(1<<((p>>2)&3)) == 0 {
			c.cost += c.icacheCost(pc)
		}
		instr = binary.LittleEndian.Uint32(c.fram[pc&uint32(len(c.fram)-1):])
		ok = true
	} else {
		instr, ok = c.fastFetch(pc)
	}
	if !ok {
		if c.bus.InstructionBusError(pc) {
			c.raise(ExcIBE, 0)
			c.advance()
			return
		}
		c.cost += c.fetchCost(pc)
		instr = c.bus.Read32(pc)
	}

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

func (c *CPU) dispatchReferenceStep() int {
	c.cost = 1
	c.stepDispatchReference()
	c.now += c.cost
	return int(c.cost)
}

// 合成 ISA-ROM 窗口只測 CPU 取指；不開機、不代替實際 BIOS。
type dispatchROMBus struct {
	*pollingProbeBus
	rom []byte
}

func (b *dispatchROMBus) FetchMemory() ([]byte, []byte, *uint32) { return b.ram, b.rom, &b.biu }
func TestFastDispatchMatchesOldStep(t *testing.T) {
	rng := rand.New(rand.NewSource(18029))
	for _, pc := range []uint32{0x80000200, 0xa0000200, 0xbfc00000, 0xbfc7fffc, 0x1fc00000, 0x9fc00000} {
		for _, ins := range []uint32{0, 9<<16 | 8<<11 | 3<<6, 0x24080001, 0x8d280000, 0xffffffff} {
			for state := 0; state < 16; state++ {
				bus := &dispatchROMBus{pollingProbeBus: &pollingProbeBus{ram: make([]byte, 8192), biu: 0x800}, rom: make([]byte, 0x80000)}
				if state&1 != 0 {
					bus.biu = 0
				}
				if pc&0x1fffffff >= 0x1fc00000 {
					binary.LittleEndian.PutUint32(bus.rom[pc&0x7ffff:], ins)
				} else {
					bus.Write32(pc, ins)
				}
				c := New(bus)
				c.SetPC(pc)
				for i := range c.gpr {
					c.gpr[i] = rng.Uint32()
				}
				for i := range c.cop0 {
					c.cop0[i] = rng.Uint32()
				}
				c.gpr[9] = 0x80001000
				c.ldWhich = []uint32{0, 1, 8, ldNone}[state%4]
				c.ldValue = rng.Uint32()
				c.bdbt = uint32(state % 4)
				if c.bdbt != 0 {
					c.nextPC = 0x80001000
				}
				c.now = uint64(rng.Uint32())
				if state&2 != 0 {
					c.icacheCost(pc)
				}
				ref := *c
				got, want := c.Step(), ref.dispatchReferenceStep()
				if got != want || !reflect.DeepEqual(*c, ref) {
					t.Fatalf("pc=%x instruction-class=%x state=%d cycles=%d/%d", pc, ins>>26, state, got, want)
				}
			}
		}
	}
}
func BenchmarkCPUFastDispatch(b *testing.B) {
	for _, rom := range []bool{false, true} {
		for _, nop := range []bool{false, true} {
			for _, fast := range []bool{false, true} {
				b.Run(fmt.Sprintf("rom=%v/nop=%v/fast=%v", rom, nop, fast), func(b *testing.B) {
					bus := &dispatchROMBus{pollingProbeBus: &pollingProbeBus{ram: make([]byte, 8192), biu: 0x800}, rom: make([]byte, 0x80000)}
					pc := uint32(0x80000200)
					ins := uint32(0x24080001)
					if nop {
						ins = 0
					}
					if rom {
						pc = 0xbfc00000
						binary.LittleEndian.PutUint32(bus.rom, ins)
					} else {
						bus.Write32(pc, ins)
					}
					c := New(bus)
					c.SetPC(pc)
					c.icacheCost(pc)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						c.SetPC(pc)
						if fast {
							c.Step()
						} else {
							c.dispatchReferenceStep()
						}
					}
				})
			}
		}
	}
}
