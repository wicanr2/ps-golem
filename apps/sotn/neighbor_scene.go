// 017 §12. Original addresses and packet layout are retained for audit.
package sotn

import (
	"github.com/wicanr2/ps-golem/machine"
	"sort"
)

type sceneLayer struct {
	address, data, definition uint32
	rect                      [4]int
	order, flags, params      uint32
}
type sceneRoom struct {
	rect    [4]int
	tileset byte
	fg, bg  *sceneLayer
}
type sceneNode struct {
	order, page uint32
	words       [3]uint32
	palette     uint32
	common      bool
	room        [4]int
}

func sceneModeSuccessor(m *machine.Machine, ot, ptr uint32) bool {
	ot &= 0x1fffff
	for hops := 0; hops < 512; hops++ {
		if ptr >= (2<<20)-12 || ptr&3 != 0 {
			return false
		}
		tag := m.Read32(ptr)
		if ptr < ot || ptr >= ot+512*4 {
			return tag>>24 == 2 && m.Read32(ptr+4)>>24 == 0xe1 && m.Read32(ptr+8)>>24 == 0xe2
		}
		next := tag & 0xffffff
		if tag>>24 != 0 || next <= ptr {
			return false
		}
		ptr = next
	}
	return false
}

func sceneStagePointer(p, n uint32) bool {
	return p >= 0x80180000 && p <= 0x80200000 && n <= 0x80200000-p
}
func sceneReadLayer(m *machine.Machine, p uint32) *sceneLayer {
	if !sceneStagePointer(p, 16) {
		return nil
	}
	data, def, packed := m.Read32(p), m.Read32(p+4), m.Read32(p+8)
	r := [4]int{int(packed & 63), int(packed >> 6 & 63), int(packed >> 12 & 63), int(packed >> 18 & 63)}
	if r[2] < r[0] || r[3] < r[1] {
		return nil
	}
	cells := uint32((r[2] - r[0] + 1) * (r[3] - r[1] + 1) * 256)
	if cells > 65536 || !sceneStagePointer(data, cells*2) || !sceneStagePointer(def, 16) {
		return nil
	}
	return &sceneLayer{address: p, data: data, definition: def, rect: r, order: uint32(m.Read16(p + 12)), flags: uint32(m.Read16(p + 14)), params: packed >> 24}
}

func (w *Widescreen) appendNeighborScene(m *machine.Machine) {
	if !w.sources.ready {
		return
	}

	// Fixed metadata fingerprint, not a copied room/graphics table.
	if !sceneHeader(m) {
		return
	}
	if m.Read16(0x8006c39c) != 0 || m.Read16(0x8006c3a0) != 0 {
		return
	}
	flags := m.Read32(0x800730a0)
	if flags != 3 && flags != 0x203 {
		return
	}
	// Only the observed ordinary background or no background is supported.
	if m.Read32(0x800730f4) != 0 && (m.Read32(0x800730f4) != 3 || m.Read32(0x80073104) != 1) {
		return
	}
	for i := uint32(1); i < 16; i++ {
		if m.Read32(0x800730f4+i*0x30) != 0 {
			return
		}
	}
	roomTable, layers := m.Read32(0x80180010), m.Read32(0x80180020)
	if !sceneStagePointer(roomTable, 8) || !sceneStagePointer(layers, 8) {
		return
	}
	activeRect := [4]int{int(m.Read32(0x800730b0)), int(m.Read32(0x800730b4)), int(m.Read32(0x800730b8)), int(m.Read32(0x800730bc))}
	var active *sceneRoom
	rooms := []sceneRoom{}
	terminated := false
	for n := uint32(0); n < 512; n++ {
		p := roomTable + n*8
		if !sceneStagePointer(p, 8) {
			return
		}
		l := m.Read8(p)
		if l == 0x40 {
			terminated = true
			break
		}
		r := [4]int{int(l), int(m.Read8(p + 1)), int(m.Read8(p + 2)), int(m.Read8(p + 3))}
		if r[0] > r[2] || r[1] > r[3] || r[2] >= 64 || r[3] >= 64 {
			return
		}
		index := uint32(m.Read8(p + 4))
		if !sceneStagePointer(layers+index*8, 8) {
			return
		}
		room := sceneRoom{rect: r, tileset: m.Read8(p + 5), fg: sceneReadLayer(m, m.Read32(layers+index*8)), bg: sceneReadLayer(m, m.Read32(layers+index*8+4))}
		if room.tileset == 255 || room.fg == nil {
			continue
		}
		rooms = append(rooms, room)
		if room.rect == activeRect && room.fg.data == m.Read32(0x80073084) && room.fg.definition == m.Read32(0x80073088) {
			x := room
			active = &x
		}
	}
	if !terminated {
		return
	}
	if active == nil || active.tileset != 0 || active.fg.rect != activeRect {
		return
	}
	sx, sy := int(int16(m.Read16(0x8007308e))), int(int16(m.Read16(0x80073092)))
	if sx < 0 || sy < 0 {
		return
	}

	nodes := []sceneNode{}
	for iy := 0; iy < 16; iy++ {
		for ix := 0; ix < 23; ix++ {
			tx, ty := (sx>>4)-3+ix, (sy>>4)+iy
			if tx >= 0 && tx < (activeRect[2]-activeRect[0]+1)*16 && ty >= 0 && ty < (activeRect[3]-activeRect[1]+1)*16 {
				continue
			}
			dx, dy := tx*16-sx, ty*16-sy
			if !(dx+16 <= 0 || dx >= 256) || dx >= 304 || dx+16 <= -48 || dy >= 240 || dy+16 <= 0 {
				continue
			}
			wx, wy := activeRect[0]*256+tx*16, activeRect[1]*256+ty*16
			var found *sceneRoom
			for i := range rooms {
				r := &rooms[i]
				if r.rect == activeRect || r.tileset != active.tileset {
					continue
				}
				if wx >= r.rect[0]*256 && wx < (r.rect[2]+1)*256 && wy >= r.rect[1]*256 && wy < (r.rect[3]+1)*256 {
					if found != nil {
						found = nil
						break
					}
					found = r
				}
			}
			if found == nil {
				continue
			}
			for li, l := range []*sceneLayer{found.bg, found.fg} {
				if l == nil || l.rect != found.rect {
					continue
				}
				if li == 0 {
					if l.params != 1 || l.flags != 3 || l.order != 32 {
						continue
					}
				} else {
					if (l.params != 1 && l.params != 32) || (l.flags != 3 && l.flags != 0x203) {
						continue
					}
				}
				order := l.order
				if li == 1 && l.params == 32 {
					order = 96
				}
				if order != 32 && order != 96 {
					continue
				}
				col, row := (wx-l.rect[0]*256)/16, (wy-l.rect[1]*256)/16
				cols := (l.rect[2] - l.rect[0] + 1) * 16
				id := uint32(m.Read16(l.data + uint32(row*cols+col)*2))
				if id == 0 {
					continue
				}
				var values [3]uint32
				valid := true
				for j := uint32(0); j < 3; j++ {
					p := m.Read32(l.definition + j*4)
					if !sceneStagePointer(p, id+1) {
						valid = false
						break
					}
					values[j] = uint32(m.Read8(p + id))
				}
				if !valid {
					continue
				}
				page, gfx, pal := values[0], values[1], values[2]
				if l.flags&0x200 != 0 {
					if page != 0 && page != 23 {
						continue
					}
					pal += 256
				} else if page > 7 {
					continue
				}
				clut := uint32(m.Read16(0x8003c104 + pal*2))
				nodes = append(nodes, sceneNode{order: order, page: page, room: found.rect, palette: values[2], common: l.flags&0x200 != 0, words: [3]uint32{0x7d000000, uint32(uint16(dx)) | uint32(uint16(dy))<<16, (gfx&15)*16 | (gfx&240)<<8 | clut<<16}})
			}
		}
	}
	if len(nodes) == 0 {
		return
	}
	if !w.sources.matches(m, nodes) {
		return
	}
	used := m.Read32(0x80097940)
	// Reserve later ordinary sprite work. Never enlarge the original buffers.
	if used > 576 || uint32(len(nodes)) > 576-used {

		return
	}
	buffer := m.Read32(0x8006c37c)
	if buffer < 0x80000000 || uint64(buffer)+0x117f4+640*16 > 0x80200000 {
		return
	}
	ot := buffer + 0x474
	heads := map[uint32]uint32{}
	missing := map[uint32]uint32{}
	pages := map[uint32]uint32{}
	for _, n := range nodes {
		slot := n.order + n.page
		if _, ok := heads[slot]; ok {
			continue
		}
		ptr := m.Read32(ot+slot*4) & 0xffffff
		if ptr >= 2<<20 || m.Read32(ptr)>>24 != 2 || m.Read32(ptr+4) != 0xe1000000+n.page+8 || m.Read32(ptr+8) != 0xe2000000 {
			if !sceneModeSuccessor(m, ot, ptr) {

				return
			}
			missing[slot] = ptr
			pages[slot] = n.page
			heads[slot] = 0
			continue
		}
		heads[slot] = ptr
	}
	modeUsed := m.Read32(0x8009792c)
	// Original ECCD8 addresses mode slots at buffer+C74+12*used; ED458
	// checks used<1024. Leave 24 slots for later original drawing work.
	if modeUsed > 1000 || uint32(len(missing)) > 1000-modeUsed {

		return
	}
	slots := make([]uint32, 0, len(missing))
	for slot := range missing {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	for _, slot := range slots {
		addr := buffer + 0xc74 + modeUsed*12
		m.Write32(addr, 0x02000000|missing[slot])
		m.Write32(addr+4, 0xe1000000+pages[slot]+8)
		m.Write32(addr+8, 0xe2000000)
		m.Write32(ot+slot*4, addr&0xffffff)
		heads[slot] = addr & 0xffffff
		modeUsed++
	}
	for _, n := range nodes {
		head := heads[n.order+n.page]
		tag := m.Read32(head)
		addr := buffer + 0x117f4 + used*16
		m.Write32(addr, 0x03000000|tag&0xffffff)
		for j, w := range n.words {
			m.Write32(addr+4+uint32(j)*4, w)
		}
		m.Write32(head, tag&0xff000000|addr&0xffffff)

		used++
	}
	m.Write32(0x80097940, used)
	if len(missing) > 0 {
		m.Write32(0x8009792c, modeUsed)
	}

	w.sceneFrames++
}
