package machine

import (
	"encoding/binary"
	"github.com/wicanr2/ps-golem/render/hd"
	"reflect"
	"testing"
)

func entryMachine(t *testing.T, precision bool) (*Machine, *hd.Shadow) {
	t.Helper()
	m, e := New(make([]byte, biosSize))
	if e != nil {
		t.Fatal(e)
	}
	s := hd.NewShadow(1)
	m.GPU.SetShadow(s)
	if precision {
		if e = m.EnablePGXP(); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() { m.GPU.SetShadow(nil); s.Close() })
	return m, s
}
func entryAllPrecision(m *Machine) CallPrecisionImage {
	indices := make([]uint32, ramSize/4)
	for i := range indices {
		indices[i] = uint32(i)
	}
	return m.CallPrecisionAt(indices)
}
func entryCPU(m *Machine, total uint64) CallCPUState {
	s := m.CallCPUState()
	s.PC = m.CPU.GPR(31)
	s.NextPC = s.PC + 4
	s.Now += total
	s.Cost = 1
	return s
}

func TestCallRegistrationKeepsNativeStepAndClone(t *testing.T) {
	m, _ := entryMachine(t, false)
	calls := 0
	first, e := m.RegisterCall(m.CPU.PC(), func(uint64) bool { calls++; return false })
	if e != nil {
		t.Fatal(e)
	}
	if m.Clone().callRegistration != nil {
		t.Fatal("clone retained handler")
	}
	m.Step()
	if calls != 0 {
		t.Fatal("native Step invoked call handler")
	}
	if !first.Close() {
		t.Fatal("owner failed to close")
	}
	second, e := m.RegisterCall(m.CPU.PC(), func(uint64) bool { calls++; return false })
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	if first.Close() || m.callRegistration != second {
		t.Fatal("old token removed new registration")
	}
	control := m.Clone()
	m.RunUntilField(1, 32)
	control.RunUntilField(1, 32)
	if calls != 1 || !reflect.DeepEqual(m.Clone(), control.Clone()) {
		t.Fatal("rejected callback changed native execution")
	}
}

func TestCallCommitPreservesOwners(t *testing.T) {
	for _, precision := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "PGXP"}[precision], func(t *testing.T) {
			m, s := entryMachine(t, precision)
			c, g, cd, si, md, sp, pad, card, p := m.CPU, m.GPU, m.CD, m.SIO, m.MDEC, m.SPU, m.Pad1, m.Card1, m.precision
			var delta *CallPrecisionDelta
			if precision {
				image := m.CallPrecisionAt(nil)
				image.RAM[5000] = CallPrecisionValue{X: 0x80000000, DepthX: 17, Flags: 1, Raw: 91}
				if e := m.precision.ApplyCallUpdate(image, nil); e != nil {
					t.Fatal(e)
				}
				image = m.CallPrecisionAt(nil)
				image.RAM[7] = CallPrecisionValue{Raw: 123}
				delta = &CallPrecisionDelta{Image: image, Clear: []uint32{7}}
			}
			ram := m.ReadRAM(0, ramSize)
			ram[100] = 33
			scratch := make([]byte, scratchpadSize)
			scratch[9] = 44
			state := entryCPU(m, 32)
			if e := m.CommitCall([]uint32{32}, ram, scratch, state, 32, delta); e != nil {
				t.Fatal(e)
			}
			if m.CPU != c || m.GPU != g || m.CD != cd || m.SIO != si || m.MDEC != md || m.SPU != sp || m.Pad1 != pad || m.Card1 != card || m.precision != p {
				t.Fatal("owner replaced")
			}
			if m.CallCPUState() != state || m.ram[100] != 33 || m.scratchpad[9] != 44 {
				t.Fatal("return data missing")
			}
			if precision {
				image := m.CallPrecisionAt([]uint32{5000, 7})
				if image.RAM[5000].DepthX != 17 || image.RAM[5000].X != 0x80000000 || image.RAM[7].Raw != 123 {
					t.Fatal("precision delta lost unrelated bits")
				}
			}
			s.VRAMCopy()
			if !m.GPU.HasHDShadow() {
				t.Fatal("presentation lost")
			}
		})
	}
}

func TestCallCommitRejectsAtomically(t *testing.T) {
	for _, kind := range []string{"CPU", "precision", "late-IRQ", "late-field", "observer"} {
		t.Run(kind, func(t *testing.T) {
			m, s := entryMachine(t, true)
			costs := []uint32{32}
			if kind == "late-field" {
				costs = make([]uint32, 25000)
				for i := range costs {
					costs[i] = 32
				}
			}
			if kind == "late-IRQ" {
				m.GPU.WriteGP0(0x1f000000)
				m.io.iMask = 1 << irqGPU
				m.deadline = 1
			}
			if kind == "observer" {
				IRQTrace = func(bool, uint) { t.Fatal("external observation during trial") }
				defer func() { IRQTrace = nil }()
			}
			total := uint64(len(costs)) * 32
			state := entryCPU(m, total)
			delta := &CallPrecisionDelta{Image: m.CallPrecisionAt(nil)}
			if kind == "CPU" {
				state.PC++
			}
			if kind == "precision" {
				delta.Image.RAM[0] = CallPrecisionValue{Flags: 4}
			}
			before, pixels, meta, p := m.Clone(), s.VRAMCopy(), entryAllPrecision(m), m.precision
			if e := m.CommitCall(costs, m.ReadRAM(0, ramSize), make([]byte, scratchpadSize), state, total, delta); e == nil {
				t.Fatal("invalid commit accepted")
			}
			if !reflect.DeepEqual(before, m.Clone()) || !reflect.DeepEqual(pixels, s.VRAMCopy()) || !reflect.DeepEqual(meta, entryAllPrecision(m)) || p != m.precision {
				t.Fatal("rejection changed complete state or ownership")
			}
		})
	}
}

func sprite16Setup(m *Machine) {
	words := []uint32{0xe1000008, 0xe3000000, 0xe4000000 | 31 | 31<<10, 0xa0000000, 512, 4 | 16<<16}
	for i := 0; i < 32; i++ {
		words = append(words, 0x11111111)
	}
	words = append(words, 0xa0000000, 100<<16, 16|1<<16, 0x001f0000, 0, 0, 0, 0, 0, 0, 0, 0x01000000, 0x60002060, 0, 32|32<<16)
	for _, v := range words {
		m.GPU.WriteGP0(v)
	}
}

func TestSprite16WholePacketAndFallback(t *testing.T) {
	for _, kind := range []string{"typed", "stale", "page", "short-packet"} {
		t.Run(kind, func(t *testing.T) {
			a, sa := entryMachine(t, false)
			b, sb := entryMachine(t, false)
			sprite16Setup(a)
			sprite16Setup(b)
			j := Sprite16Binding{Address: 0x1004, Words: [3]uint32{0x7d000000, 4 | 5<<16, (100 << 6) << 16}, Sprite: hd.Sprite16{X: 4, Y: 5, CLUT: 100 << 6, Page: 8}}
			plan, e := b.PrepareSprite16Bindings([]Sprite16Binding{j}, 1280)
			if e != nil {
				t.Fatal(e)
			}
			plan.Install()
			count := uint32(3)
			if kind == "short-packet" {
				count = 2
			}
			for _, m := range []*Machine{a, b} {
				binary.LittleEndian.PutUint32(m.ram[0x1000:], count<<24|0x800000)
				for i, w := range j.Words {
					binary.LittleEndian.PutUint32(m.ram[0x1004+i*4:], w)
				}
				if kind == "stale" {
					binary.LittleEndian.PutUint32(m.ram[0x1008:], 7|5<<16)
				}
				if kind == "page" {
					m.GPU.WriteGP0(0xe1000009)
				}
				m.io.dma.chcr[2] = 1<<24 | 2<<9 | 1
				m.io.dma.madr[2] = 0x1000
				m.dmaGPU()
			}
			if !reflect.DeepEqual(a.Clone(), b.Clone()) || !reflect.DeepEqual(sa.VRAMCopy(), sb.VRAMCopy()) {
				t.Fatal("whole-packet path differs")
			}
			submitted, words, _, _ := b.Sprite16Statistics()
			if kind == "typed" {
				if submitted != 1 || words != 3 {
					t.Fatal("no typed work")
				}
			} else if submitted != 0 || words != 0 {
				t.Fatal("fallback emitted typed work")
			}
		})
	}
}

func TestSprite16InvalidBatchIsAtomic(t *testing.T) {
	m, _ := entryMachine(t, false)
	good := Sprite16Binding{Address: 0x1004, Words: [3]uint32{0x7d000000, 0, 0}, Sprite: hd.Sprite16{Page: 8}}
	plan, e := m.PrepareSprite16Bindings([]Sprite16Binding{good}, 1280)
	if e != nil {
		t.Fatal(e)
	}
	plan.Install()
	before := m.callSprites
	bad := good
	bad.Address = 0xfffffff8
	if _, e = m.PrepareSprite16Bindings([]Sprite16Binding{good, bad}, 1280); e == nil || m.callSprites != before {
		t.Fatal("invalid batch changed old bindings")
	}
}
