// Package contract: docs/spec/025-hd-primary-native-demand.md §12.16.
package cpu

import "fmt"

// CallCacheLine carries timing metadata, never instruction bytes.
type CallCacheLine struct {
	Tag   uint32 `json:"tag"`
	Valid uint8  `json:"valid"`
}

// CallState is a detached value for a proved high-level call replacement.
// GTE data, memory, observers and bus ownership stay with the live CPU.
type CallState struct {
	GPR           [35]uint32         `json:"gpr"`
	HI            uint32             `json:"hi"`
	LO            uint32             `json:"lo"`
	PC            uint32             `json:"pc"`
	NextPC        uint32             `json:"nextPC"`
	BDBT          uint32             `json:"bdbt"`
	LDWhich       uint32             `json:"ldWhich"`
	LDValue       uint32             `json:"ldValue"`
	COP0          [32]uint32         `json:"cop0"`
	Branched      bool               `json:"branched"`
	Now           uint64             `json:"now"`
	Cost          uint64             `json:"cost"`
	MultiplyReady uint64             `json:"muldivDone"`
	GTEReady      uint64             `json:"gteDone"`
	Cache         [256]CallCacheLine `json:"icache"`
}

func (c *CPU) SnapshotCallState() CallState {
	s := CallState{GPR: c.gpr, HI: c.hi, LO: c.lo, PC: c.pc, NextPC: c.nextPC,
		BDBT: c.bdbt, LDWhich: c.ldWhich, LDValue: c.ldValue, COP0: c.cop0,
		Branched: c.branched, Now: c.now, Cost: c.cost, MultiplyReady: c.muldivDone, GTEReady: c.gteDone}
	for i, v := range c.icache {
		s.Cache[i] = CallCacheLine{v.tag, v.valid}
	}
	return s
}

func (c *CPU) CallReturnReady() bool {
	return c.breakHandler == nil && c.bdbt == 0 && c.ldWhich == ldNone && c.nextPC == c.pc+4
}

// ApplyCallReturn validates every value before mutation. The owning Machine
// must separately validate memory, device events, IRQ and presentation effects.
func (c *CPU) ApplyCallReturn(s CallState, elapsed uint64) error {
	if !c.CallReturnReady() || elapsed == 0 || elapsed > ^uint64(0)-c.now ||
		s.GPR[0] != 0 || s.PC != c.gpr[31] || s.NextPC != s.PC+4 ||
		s.Now != c.now+elapsed || s.COP0 != c.cop0 || s.GTEReady != c.gteDone ||
		s.BDBT != 0 || s.Branched || s.LDWhich != ldNone || s.Cost == 0 || s.Cost > 32 {
		return fmt.Errorf("unsupported typed CPU call return")
	}
	for _, v := range s.Cache {
		if v.Valid&^uint8(15) != 0 {
			return fmt.Errorf("CPU call cache validity bounds")
		}
	}
	c.gpr = s.GPR
	c.hi, c.lo = s.HI, s.LO
	c.pc, c.nextPC = s.PC, s.NextPC
	c.bdbt = s.BDBT
	c.ldWhich, c.ldValue = s.LDWhich, s.LDValue
	c.cop0 = s.COP0
	c.branched = s.Branched
	c.now, c.cost = s.Now, s.Cost
	c.muldivDone, c.gteDone = s.MultiplyReady, s.GTEReady
	for i, v := range s.Cache {
		c.icache[i] = icLine{tag: v.Tag, valid: v.Valid}
	}
	return nil
}
