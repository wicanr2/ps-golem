// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package machine

import "fmt"

type callEventScalars struct {
	io                                                              ioState
	cycles, videoAcc, pending, deadline, mdecPending, dotAcc, field uint64
	gpuIRQ, cdIRQ, sioIRQ, spuIRQ                                   bool
}

// Rejection preserves every source value and presentation owner. Acceptance
// retains Machine, device, tracker and presentation addresses. No Machine.Clone
// or full precision clone is used. This is single-owner machine-thread code.
func (m *Machine) CommitCall(costs []uint32, ram, scratch []byte, cpuState CallCPUState,
	budget uint64, precision *CallPrecisionDelta) error {
	if len(ram) != ramSize || len(scratch) != scratchpadSize || len(costs) == 0 || m.ExactTiming ||
		(m.precision == nil) != (precision == nil) || m.CPU.COP0(13)&0x400 != 0 || IRQTrace != nil || DMATrace != nil || CDTrace != nil || !m.callPrecisionOwnerReady() || !m.CPU.CallReturnReady() ||
		!m.GPU.CallContextReady() || m.io.irqPending() || m.MDEC.WaitingClock() ||
		!callNoSecondDevice(m.SIO.Pads[1]) || !callNoSecondDevice(m.SIO.Cards[1]) ||
		m.SIO.Pads[0] != m.Pad1 || m.SIO.Cards[0] != m.Card1 {
		return fmt.Errorf("Go commit initial context rejected")
	}
	for ch := 0; ch < 7; ch++ {
		if m.io.dma.active(ch) {
			return fmt.Errorf("Go commit DMA%d active", ch)
		}
	}
	var total uint64
	for _, cost := range costs {
		if cost == 0 || cost > 32 {
			return fmt.Errorf("Go commit cost bounds")
		}
		total += uint64(cost)
	}
	if total > budget || total > ^uint64(0)-m.cycles {
		return fmt.Errorf("Go commit budget")
	}
	if precision != nil {
		if err := m.precision.ValidateCallUpdate(precision.Image, precision.Clear); err != nil {
			return err
		}
	}
	// Validate and prepare the CPU return before touching any live device.
	prepared := *m.CPU

	if err := prepared.ApplyCallReturn(cpuState, total); err != nil {
		return err
	}

	state := callEventScalars{m.io, m.cycles, m.videoAcc, m.pending, m.deadline, m.mdecPending, m.dotAcc, m.field,
		m.gpuIRQ, m.cdIRQ, m.sioIRQ, m.spuIRQ}
	cpuBefore := *m.CPU
	cd := m.CD.Clone() // its DMA buffer is the sole mutable slice in this journal
	sp, md, si, pad, card := *m.SPU, *m.MDEC, *m.SIO, *m.Pad1, *m.Card1
	video := m.GPU.BeginCallVideo()
	committed := false
	defer func() {
		m.GPU.EndCallVideo(video, committed)
		if !committed {
			*m.CPU = cpuBefore
			*m.CD = *cd
			*m.SPU = sp
			*m.MDEC = md
			*m.SIO = si
			*m.Pad1 = pad
			*m.Card1 = card
			m.io = state.io
			m.cycles, m.videoAcc, m.pending, m.deadline = state.cycles, state.videoAcc, state.pending, state.deadline
			m.mdecPending, m.dotAcc, m.field = state.mdecPending, state.dotAcc, state.field
			m.gpuIRQ, m.cdIRQ, m.sioIRQ, m.spuIRQ = state.gpuIRQ, state.cdIRQ, state.sioIRQ, state.spuIRQ
		}
	}()
	for _, cost := range costs {
		v := uint64(cost)
		m.cycles += v
		m.pending += v
		m.mdecPending += v
		if m.MDEC.WaitingClock() || m.io.dma.active(0) || m.io.dma.active(1) {
			m.syncMDEC()
			m.CPU.SetIRQ(m.io.irqPending())
		}
		if m.cycles >= m.deadline {
			m.sync()
		}
		if m.field != state.field || m.io.irqPending() {
			return fmt.Errorf("Go commit crosses field/IRQ")
		}
	}
	if !m.GPU.CallVideoCompatible(video) {
		return fmt.Errorf("Go commit presentation changed")
	}
	// No rejection remains after this prevalidated, atomic precision update.
	if precision != nil {
		if err := m.precision.ApplyCallUpdate(precision.Image, precision.Clear); err != nil {
			return err
		}
	}
	copy(m.ram[:], ram)
	copy(m.scratchpad[:], scratch)
	*m.CPU = prepared
	committed = true
	return nil
}
