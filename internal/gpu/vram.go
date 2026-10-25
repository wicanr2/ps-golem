package gpu

// VRAMSnapshot returns independent native RGB555/STP words in 1024x512 order.
// The caller owns the copy; retaining or modifying it cannot alias GPU storage.
// Native rasterization is synchronous. See 025-hd-primary-native-demand §9.
func (g *GPU) VRAMSnapshot() []uint16 {
	return append([]uint16(nil), g.vram[:]...)
}
