package sotn

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/machine"
)

// 017 §12. Original files remain on the user's disc, never in the program.
type sceneSources struct {
	tried, ready  bool
	tiles, common []byte
}

// SetSourceCue uses the same existing media selection as the player.
// It performs no I/O until a supported stage is actually observed.
func (w *Widescreen) SetSourceCue(cue string) {
	if w.sourceCue != cue {
		w.sourceCue = cue
		w.sources = sceneSources{}
		w.sceneReady = false
	}
}

func (s *sceneSources) validate(tiles, common []byte) {
	s.tried = true
	if fmt.Sprintf("%x", sha256.Sum256(tiles)) != "a0bb78a1260eee5e3bda4bae6cfbbd0a61ddd5604905f9fd6ef833fd03319c66" ||
		fmt.Sprintf("%x", sha256.Sum256(common)) != "6f8c832dd1986aca1ed04247d3bd7977aae4ddf5cb3ac569097f6c0b1123ac74" {
		return
	}
	s.tiles, s.common, s.ready = tiles, common, true
}

func (s *sceneSources) load(cue string) {
	if s.tried {
		return
	}
	s.tried = true
	if cue == "" {
		return
	}
	d, err := disc.Open(cue)
	if err != nil {
		return
	}
	defer d.Close()
	tiles, err := d.ReadFileLimit("ST/NO3/F_NO3.BIN", 2<<20)
	if err != nil {
		return
	}
	common, err := d.ReadFileLimit("BIN/F_GAME.BIN", 2<<20)
	if err != nil {
		return
	}
	s.validate(tiles, common)
}

func sceneHeader(m *machine.Machine) bool {
	return fmt.Sprintf("%x", sha256.Sum256(m.ReadRAM(0x180000, 0x24))) == "0a19676621ba9dfaad1a39a2ab6ead6fd0438fad85a57c91be011b4f9fe0cd82"
}

// matches compares exact 4-bit source and palette words, retaining all bits.
// One owned native snapshot prevents the HD image from becoming its own source.
func (s *sceneSources) matches(m *machine.Machine, nodes []sceneNode) bool {
	if !s.ready {
		return false
	}
	vram := m.GPU.VRAMSnapshot()
	commonChecked := false
	for _, n := range nodes {
		u, v := int(n.words[2]&255), int(n.words[2]>>8&255)
		clut := n.words[2] >> 16
		cx, cy := int(clut&63)*16, int(clut>>6)
		if cy >= 512 {
			return false
		}
		var palette []byte
		if n.common {
			at := 0x40000 + int(n.palette)*32
			if at+32 > len(s.common) {
				return false
			}
			palette = s.common[at : at+32]
		} else {
			bank := int(n.palette) % 16
			at := (bank/4)*0x8000 + (bank%2)*32 + (int(n.palette)/16)*64
			if bank%4 < 2 {
				at += 0x5c00
			} else {
				at += 0x7c00
			}
			if at+32 > len(s.tiles) {
				return false
			}
			palette = s.tiles[at : at+32]
		}
		for i := 0; i < 16; i++ {
			if vram[cy*1024+cx+i] != binary.LittleEndian.Uint16(palette[i*2:]) {
				return false
			}
		}
		t := n.page + 8
		px, py := int(t&15)*64+u/4, int(t>>4&1)*256+v
		if n.common {
			switch {
			case n.page == 0 && u == 0 && v == 0:
				for y := 0; y < 16; y++ {
					for x := 0; x < 4; x++ {
						if vram[(py+y)*1024+px+x] != 0 {
							return false
						}
					}
				}
			case n.page == 23 && u >= 128 && u+16 <= 208 && v == 112:
				if !commonChecked {
					h := sha256.New()
					var b [8]byte
					binary.LittleEndian.PutUint32(b[:4], 20)
					binary.LittleEndian.PutUint32(b[4:], 16)
					h.Write(b[:])
					for y := 368; y < 384; y++ {
						for x := 992; x < 1012; x++ {
							binary.LittleEndian.PutUint16(b[:2], vram[y*1024+x])
							h.Write(b[:2])
						}
					}
					if fmt.Sprintf("%x", h.Sum(nil)) != "cc019c6fd1db225d260aefcd8fdd415847fe83bf6c872486d656aa7036c52092" {
						return false
					}
					commonChecked = true
				}
			default:
				return false
			}
		} else {
			q := (v/128)*2 + u/128
			at := int(n.page)*0x8000 + q*0x2000 + (v%128)*64 + (u%128)/2
			if at+15*64+8 > len(s.tiles) {
				return false
			}
			for y := 0; y < 16; y++ {
				for x := 0; x < 4; x++ {
					if vram[(py+y)*1024+px+x] != binary.LittleEndian.Uint16(s.tiles[at+y*64+x*2:]) {
						return false
					}
				}
			}
		}
	}
	return true
}
