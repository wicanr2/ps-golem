// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package machine

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"github.com/wicanr2/ps-golem/internal/sio"
)

type CallPrecisionValue = pgxp.CallValue
type CallPrecisionLoad = pgxp.CallLoad
type CallPrecisionImage = pgxp.CallImage
type CallPrecisionDelta struct {
	Image CallPrecisionImage
	Clear []uint32
}

func (m *Machine) CallPrecisionEnabled() bool { return m.precision != nil }
func (m *Machine) CallPrecisionAt(indices []uint32) CallPrecisionImage {
	return m.precision.CallImageAt(indices)
}
func (m *Machine) callPrecisionOwnerReady() bool {
	observer := m.CPU.PrecisionObserver()
	if m.precision == nil {
		return observer == nil
	}
	p, ok := observer.(*pgxp.Tracker)
	return ok && p == m.precision
}
func callNoSecondDevice(d sio.Device) bool {
	switch d.(type) {
	case nil, sio.None:
		return true
	}
	return false
}
