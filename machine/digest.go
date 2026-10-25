package machine

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Digest 回傳機器可觀測狀態的摘要：週期、field、PC、I_STAT、計時器，以及 RAM、scratchpad、VRAM、
// CPU 暫存器、TTY 的雜湊。用來比對兩次執行是否逐位元相同（008 §2.5）。
func (m *Machine) Digest() string {
	m.sync()
	h := sha256.New()
	h.Write(m.ram[:])
	h.Write(m.scratchpad[:])
	for _, v := range m.GPU.VRAMSnapshot() {
		h.Write([]byte{byte(v), byte(v >> 8)})
	}
	var b [4]byte
	for i := 0; i < 32; i++ {
		binary.LittleEndian.PutUint32(b[:], m.CPU.GPR(i))
		h.Write(b[:])
	}
	for _, r := range []int{12, 13, 14} {
		binary.LittleEndian.PutUint32(b[:], m.CPU.COP0(r))
		h.Write(b[:])
	}
	h.Write(m.tty)
	return fmt.Sprintf("cyc=%d field=%d pc=%08x istat=%03x timers=%v %x",
		m.cycles, m.field, m.CPU.PC(), m.io.iStat, m.io.timers.t, h.Sum(nil)[:8])
}
