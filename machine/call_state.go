// Contract: docs/spec/025-hd-primary-native-demand.md §12.16.
package machine

import "github.com/wicanr2/ps-golem/internal/cpu"

type CallCPUState = cpu.CallState
type CallCacheLine = cpu.CallCacheLine

// CallCPUState returns detached timing/architectural values, excluding GTE data
// and machine ownership. It does not synchronize devices or execute the CPU.
func (m *Machine) CallCPUState() CallCPUState { return m.CPU.SnapshotCallState() }
