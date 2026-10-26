// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package pgxp

import (
	"fmt"
	"math"
)

type CallValue struct {
	X, Y, Z               uint32
	DepthX, DepthY        uint64
	DepthZX, DepthZY, Raw uint32
	Flags                 uint8
}
type CallLoad struct {
	Value   CallValue
	Ready   uint64
	Pending bool
}
type CallImage struct {
	RAM                     map[uint32]CallValue
	Scratch                 [256]CallValue
	Port                    CallValue
	Regs                    [32]CallValue
	Loads                   [32]CallLoad
	GTE                     [64]CallValue
	RTPS, RTPT, DepthSerial uint64
}

func callValue(v Value) CallValue {
	return CallValue{math.Float32bits(v.X), math.Float32bits(v.Y), math.Float32bits(v.Z), v.depthX, v.depthY, math.Float32bits(v.depthZX), math.Float32bits(v.depthZY), v.Raw, v.Flags}
}
func restoreCallValue(v CallValue) Value {
	return Value{X: math.Float32frombits(v.X), Y: math.Float32frombits(v.Y), Z: math.Float32frombits(v.Z), depthX: v.DepthX, depthY: v.DepthY, depthZX: math.Float32frombits(v.DepthZX), depthZY: math.Float32frombits(v.DepthZY), Raw: v.Raw, Flags: v.Flags}
}
func (t *Tracker) CallImageAt(indices []uint32) CallImage {
	r := CallImage{RAM: map[uint32]CallValue{}, Port: callValue(t.port), RTPS: t.RTPS, RTPT: t.RTPT, DepthSerial: t.depthSerial}
	for _, k := range indices {
		if k >= uint32(len(t.ram)) {
			panic("precision subset index")
		}
		v := callValue(t.ram[k])
		if v != (CallValue{}) {
			r.RAM[k] = v
		}
	}
	for i, v := range t.scratch {
		r.Scratch[i] = callValue(v)
	}
	for i, v := range t.regs {
		r.Regs[i] = callValue(v)
	}
	for i, v := range t.loads {
		r.Loads[i] = CallLoad{callValue(v.value), v.ready, v.pending}
	}
	for i, v := range t.gte {
		r.GTE[i] = callValue(v)
	}
	return r
}

// Validate the complete typed update before changing the owned clone. RAM
// outside explicit clear/set indices is retained exactly, including provenance.
func (t *Tracker) ApplyCallUpdate(v CallImage, clear []uint32) error {
	valid := func(v CallValue) bool { return v.Flags&^uint8(3) == 0 }
	if v.RAM == nil {
		return fmt.Errorf("precision subset missing")
	}
	for _, k := range clear {
		if k >= uint32(len(t.ram)) {
			return fmt.Errorf("precision clear bounds")
		}
	}
	for k, w := range v.RAM {
		if k >= uint32(len(t.ram)) || !valid(w) {
			return fmt.Errorf("precision delta bounds")
		}
	}
	for _, w := range v.Scratch {
		if !valid(w) {
			return fmt.Errorf("precision delta scratch")
		}
	}
	for _, w := range v.Regs {
		if !valid(w) {
			return fmt.Errorf("precision delta register")
		}
	}
	for _, w := range v.Loads {
		if !valid(w.Value) {
			return fmt.Errorf("precision delta load")
		}
	}
	for _, w := range v.GTE {
		if !valid(w) {
			return fmt.Errorf("precision delta GTE")
		}
	}
	if !valid(v.Port) {
		return fmt.Errorf("precision delta port")
	}
	for _, k := range clear {
		t.ram[k] = Value{}
	}
	for k, w := range v.RAM {
		t.ram[k] = restoreCallValue(w)
	}
	for i, w := range v.Scratch {
		t.scratch[i] = restoreCallValue(w)
	}
	for i, w := range v.Regs {
		t.regs[i] = restoreCallValue(w)
	}
	for i, w := range v.Loads {
		t.loads[i] = delayed{restoreCallValue(w.Value), w.Ready, w.Pending}
	}
	for i, w := range v.GTE {
		t.gte[i] = restoreCallValue(w)
	}
	t.port = restoreCallValue(v.Port)
	t.RTPS, t.RTPT, t.depthSerial = v.RTPS, v.RTPT, v.DepthSerial
	return nil
}

func (t *Tracker) ValidateCallUpdate(v CallImage, clear []uint32) error {
	valid := func(v CallValue) bool { return v.Flags&^uint8(3) == 0 }
	if v.RAM == nil {
		return fmt.Errorf("precision subset missing")
	}
	for _, k := range clear {
		if k >= uint32(len(t.ram)) {
			return fmt.Errorf("precision clear bounds")
		}
	}
	for k, w := range v.RAM {
		if k >= uint32(len(t.ram)) || !valid(w) {
			return fmt.Errorf("precision delta bounds")
		}
	}
	for _, w := range v.Scratch {
		if !valid(w) {
			return fmt.Errorf("precision delta scratch")
		}
	}
	for _, w := range v.Regs {
		if !valid(w) {
			return fmt.Errorf("precision delta register")
		}
	}
	for _, w := range v.Loads {
		if !valid(w.Value) {
			return fmt.Errorf("precision delta load")
		}
	}
	for _, w := range v.GTE {
		if !valid(w) {
			return fmt.Errorf("precision delta GTE")
		}
	}
	if !valid(v.Port) {
		return fmt.Errorf("precision delta port")
	}
	return nil
}
