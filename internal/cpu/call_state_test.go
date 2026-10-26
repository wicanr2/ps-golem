package cpu

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"reflect"
	"testing"
)

func callFixture() (*CPU, CallState) {
	c := New(nil)
	c.gpr[31] = 0x80001000
	c.now = 100
	c.cost = 1
	s := c.SnapshotCallState()
	s.PC = c.gpr[31]
	s.NextPC = s.PC + 4
	s.Now += 17
	s.Cost = 3
	s.GPR[4] = 91
	s.HI = 7
	s.LO = 8
	s.Cache[5] = CallCacheLine{Tag: 19, Valid: 15}
	return c, s
}

func TestCallReturnPreservesAttachmentsAndDetachedState(t *testing.T) {
	c, s := callFixture()
	tracker := pgxp.New()
	c.SetPrecisionTracker(tracker)
	c.GTE.SetProjectionObserver(tracker)
	gt := c.GTE
	biu := uint32(91)
	c.fbiu = &biu
	c.fram = []byte{1, 2, 3}
	ram := &c.fram[0]
	if e := c.ApplyCallReturn(s, 17); e != nil {
		t.Fatal(e)
	}
	if c.SnapshotCallState() != s {
		t.Fatal("typed return not applied")
	}
	if c.precision != tracker || c.fbiu != &biu || &c.fram[0] != ram || !reflect.DeepEqual(c.GTE, gt) {
		t.Fatal("live ownership or GTE changed")
	}
	s.GPR[4] = 0
	s.Cache[5].Valid = 0
	if c.gpr[4] != 91 || c.icache[5].valid != 15 {
		t.Fatal("return retains input alias")
	}
	out := c.SnapshotCallState()
	out.GPR[4] = 0
	out.Cache[5].Tag = 0
	if c.gpr[4] != 91 || c.icache[5].tag != 19 {
		t.Fatal("snapshot aliases live CPU")
	}
}

func TestCallReturnRejectsBeforeMutation(t *testing.T) {
	for _, name := range []string{"r0", "return", "nextPC", "time", "COP0", "GTE-time", "branch", "load", "cache", "cost", "zero-elapsed", "overflow", "entry-delay"} {
		t.Run(name, func(t *testing.T) {
			c, s := callFixture()
			elapsed := uint64(17)
			switch name {
			case "r0":
				s.GPR[0] = 1
			case "return":
				s.PC++
			case "nextPC":
				s.NextPC++
			case "time":
				s.Now++
			case "COP0":
				s.COP0[12] ^= 1
			case "GTE-time":
				s.GTEReady++
			case "branch":
				s.Branched = true
			case "load":
				s.LDWhich = 1
			case "cache":
				s.Cache[255].Valid = 16
			case "cost":
				s.Cost = 33
			case "zero-elapsed":
				elapsed = 0
				s.Now = c.now
			case "overflow":
				c.now = ^uint64(0) - 1
				elapsed = 2
				s.Now = 0
			case "entry-delay":
				c.bdbt = 2
			}
			before := *c
			if e := c.ApplyCallReturn(s, elapsed); e == nil {
				t.Fatal("invalid return accepted")
			}
			if !reflect.DeepEqual(before, *c) {
				t.Fatal("rejection mutated CPU")
			}
		})
	}
}
