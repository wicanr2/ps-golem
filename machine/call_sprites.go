// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/render/hd"
)

type Sprite16Binding struct {
	Address uint32
	Words   [3]uint32
	Sprite  hd.Sprite16
}
type sprite16Bindings struct {
	packets                                map[uint32]Sprite16Binding
	Submitted, Suppressed, Stale, Rejected uint64
}
type PreparedSprite16Bindings struct {
	owner    *Machine
	bindings *sprite16Bindings
}

// Preparation cannot mutate existing bindings. The caller installs only after
// its successful call commit. Limit is supplied by the game data contract.
func (m *Machine) PrepareSprite16Bindings(jobs []Sprite16Binding, limit int) (*PreparedSprite16Bindings, error) {
	if limit <= 0 || limit > ramSize/16 || len(jobs) > limit {
		return nil, fmt.Errorf("sprite binding budget")
	}
	for _, j := range jobs {
		if j.Address < 4 || j.Address&3 != 0 || j.Address > ramSize-12 || j.Words[0] != 0x7d000000 || j.Sprite.Page > 31 ||
			j.Words[1] != uint32(uint16(j.Sprite.X))|uint32(uint16(j.Sprite.Y))<<16 ||
			j.Words[2] != uint32(j.Sprite.U)|uint32(j.Sprite.V)<<8|uint32(j.Sprite.CLUT)<<16 {
			return nil, fmt.Errorf("sprite binding data bounds")
		}
	}
	b := &sprite16Bindings{packets: map[uint32]Sprite16Binding{}}
	if old := m.callSprites; old != nil {
		b.Submitted, b.Suppressed, b.Stale, b.Rejected = old.Submitted, old.Suppressed, old.Stale, old.Rejected
		if len(old.packets)+len(jobs) <= limit {
			for a, j := range old.packets {
				b.packets[a] = j
			}
		}
	}
	for _, j := range jobs {
		b.packets[j.Address] = j
	}
	return &PreparedSprite16Bindings{m, b}, nil
}
func (p *PreparedSprite16Bindings) Install() { p.owner.callSprites = p.bindings }
func (m *Machine) ClearSprite16Bindings()    { m.callSprites = nil }
func (m *Machine) Sprite16Statistics() (submitted, suppressed, stale, rejected uint64) {
	if b := m.callSprites; b != nil {
		return b.Submitted, b.Suppressed, b.Stale, b.Rejected
	}
	return
}

// Only dmaGPU linked-list packets call this method. All three source words are
// checked before either queue is touched; that DMA loop has no intervening CPU
// or device advance. CPU GP0 writes and block DMA retain the ordinary decoder.
func (m *Machine) submitSprite16Packet(header, count uint32) bool {
	b := m.callSprites
	if b == nil || count != 3 {
		return false
	}
	a := (header + 4) & 0x1ffffc
	j, ok := b.packets[a]
	if !ok {
		return false
	}
	delete(b.packets, a)
	if a+12 > ramSize {
		b.Stale++
		return false
	}
	for i, w := range j.Words {
		if binary.LittleEndian.Uint32(m.ram[a+uint32(i)*4:]) != w {
			b.Stale++
			return false
		}
	}
	if !m.GPU.CallSpriteReady(j.Sprite) {
		b.Rejected++
		return false
	}
	for ch := 0; ch < 7; ch++ {
		if ch != 2 && m.io.dma.active(ch) {
			b.Rejected++
			return false
		}
	}
	m.GPU.QueueCallSprite(j.Sprite)
	for i, w := range j.Words {
		if m.precision != nil {
			m.precision.ReadMemory(a+uint32(i)*4, w)
		}
		m.GPU.CallNativeWord(w)
	}
	b.Submitted++
	b.Suppressed += 3
	return true
}
