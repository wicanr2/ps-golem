// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package machine

import "fmt"

// CallRegistration is owned by the single machine execution thread. Handlers
// must leave the machine unchanged on rejection and advance within budget on
// success. Step and RunUntilPC always retain native instruction semantics.
type CallRegistration struct {
	owner   *Machine
	pc      uint32
	handler func(uint64) bool
}

func (m *Machine) RegisterCall(pc uint32, handler func(uint64) bool) (*CallRegistration, error) {
	if pc&3 != 0 || handler == nil || m.callRegistration != nil {
		return nil, fmt.Errorf("call registration context")
	}
	r := &CallRegistration{m, pc, handler}
	m.callRegistration = r
	return r, nil
}

// Close returns whether this token still owned the registration.
func (r *CallRegistration) Close() bool {
	if r == nil || r.owner == nil || r.owner.callRegistration != r {
		return false
	}
	r.owner.callRegistration = nil
	return true
}

func (m *Machine) runUntilFieldCalls(f, maxCycles uint64) bool {
	end := m.cycles + maxCycles
	defer m.sync()
	for m.field < f {
		if m.cycles >= end {
			return false
		}
		before := m.CPU.PC()
		handled := false
		if r := m.callRegistration; r != nil && before == r.pc {
			cycles := m.cycles
			handled = r.handler(end - cycles)
			if handled && (m.cycles <= cycles || m.cycles > end) {
				panic("call handler accepted outside progress budget")
			}
		}
		if !handled {
			m.Step()
		}
		if m.field < f && m.CPU.PC() < before {
			m.tryRAMCountdown(end)
		}
	}
	return true
}
