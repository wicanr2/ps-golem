package machine

import "testing"

func TestReadRAMCopyPreservesWrapAndOwnership(t *testing.T) {
	m := &Machine{}
	for i := range m.ram {
		m.ram[i] = byte((i >> 16) ^ i)
	}
	for _, v := range []struct {
		start uint32
		n     int
	}{{0, 0}, {5, 17}, {ramSize - 3, 11}, {^uint32(0), ramSize*2 + 7}} {
		out := m.ReadRAM(v.start, v.n)
		for i, b := range out {
			if b != m.ram[(v.start+uint32(i))&(ramSize-1)] {
				t.Fatalf("wrap mismatch at %d", i)
			}
		}
		if len(out) > 0 {
			k := v.start & (ramSize - 1)
			old := m.ram[k]
			out[0] ^= 255
			if m.ram[k] != old {
				t.Fatal("snapshot aliases RAM")
			}
		}
	}
}
