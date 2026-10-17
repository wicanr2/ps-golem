package machine

import (
	"fmt"
	"github.com/wicanr2/ps-golem/internal/pgxp"
)

// EnablePGXP 必須在第一筆開機輸入前、接上至少兩倍影子 GPU 後呼叫。
func (m *Machine) EnablePGXP() error {
	if m.precision != nil {
		return nil
	}
	if !m.GPU.PrecisionSupported() {
		return fmt.Errorf("PGXP 需要 -hd 2、4 或 8")
	}
	m.precision = pgxp.New()
	m.CPU.SetPrecisionTracker(m.precision)
	m.CPU.GTE.SetProjectionObserver(m.precision)
	return nil
}

// DisablePGXP 在卸下影子前解除所有追蹤掛點。
func (m *Machine) DisablePGXP() {
	m.precision = nil
	m.CPU.SetPrecisionTracker(nil)
	m.CPU.GTE.SetProjectionObserver(nil)
}

func (m *Machine) writeGP0(v, addr uint32) {
	if m.precision != nil {
		m.GPU.WriteGP0Precision(v, m.precision.ReadMemory(addr, v))
	} else {
		m.GPU.WriteGP0(v)
	}
}

// PGXPTransforms 供逐 field 的調查使用，不參與摘要或機器決策。
func (m *Machine) PGXPTransforms() (rtps, rtpt uint64) {
	if m.precision != nil {
		return m.precision.RTPS, m.precision.RTPT
	}
	return 0, 0
}
