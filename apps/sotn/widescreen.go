package sotn

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/wicanr2/ps-golem/internal/cpu"
	"github.com/wicanr2/ps-golem/machine"
)

// 017 §3，CPU 虛擬位址；名稱只供導覽，位址與原字保留作定位。
const tilemapStart = 0x800ecc10
const tilemapSize = 2332
const widescreenBreakCode = 0x5753
const widescreenForegroundCode = 0x5754
const widescreenBackgroundCode = 0x5755
const widescreenSceneCode = 0x5756

var tilemapHash = [32]byte{0x40, 0xdf, 0xd4, 0x0f, 0x6a, 0x99, 0x22, 0xdd, 0x5e, 0xd1, 0xb6, 0x9f, 0x1f, 0xd3, 0x2a, 0xda, 0x3f, 0x7a, 0x81, 0x39, 0xb7, 0x93, 0x98, 0x4f, 0x49, 0xd3, 0x73, 0xd3, 0xf3, 0x1b, 0x79, 0x7c}

var widescreenWords = [...]struct{ addr, original, patched uint32 }{
	{0x800ecd64, 0x00031c00, 0x2463ffd0},
	{0x800ecd70, 0x00037d03, 0x00037903},
	{0x800ed010, 0x29220011, 0x29220017},
	{0x800ed194, 0x00021400, 0x2442ffd0},
	{0x800ed198, 0x00027d03, 0x00027903},
	{0x800ed428, 0x29220011, 0x29220017},
	{0x800eccf0, 0xa6030000, widescreenBreakCode<<6 | 0x0d},
	{0x800ece80, 0x01ec1021, widescreenForegroundCode<<6 | 0x0d},
	{0x800ed260, 0x01ec1021, widescreenBackgroundCode<<6 | 0x0d},
	{0x800ed520, 0x27bd0048, widescreenSceneCode<<6 | 0x0d},
}

// Widescreen 只由遊玩端持有，不接入觀測 API 或 Machine.Step。
// 零值可用；每個 field 結束後呼叫 Check，離開模式前呼叫 Disable。
type Widescreen struct {
	applied      bool
	warned       bool
	Applications uint64
	sceneReady   bool
	sceneFrames  uint64
	sourceCue    string
	sources      sceneSources
}

// NeighborFrames is read by the player after its emulation goroutine stops.
func (w *Widescreen) NeighborFrames() uint64 { return w.sceneFrames }

// Check 把已知改字還原後驗簽。非原字或已知改字也算不符，不掩蓋未知改動。
// DRA 尚未載入時等待；曾套用後失配只警告一次，RAM 與陷阱保持不動。
func (w *Widescreen) Check(m *machine.Machine) error {
	return w.check(m, tilemapHash)
}

func (w *Widescreen) check(m *machine.Machine, expected [32]byte) error {
	b := m.ReadRAM(tilemapStart, tilemapSize)
	valid := true
	for _, p := range widescreenWords {
		o := p.addr - tilemapStart
		v := binary.LittleEndian.Uint32(b[o:])
		if v != p.original && v != p.patched {
			valid = false
		}
		if v == p.patched {
			binary.LittleEndian.PutUint32(b[o:], p.original)
		}
	}
	actual := sha256.Sum256(b)
	if !valid || actual != expected {
		w.sceneReady = false
		if w.applied && !w.warned {
			fmt.Fprintf(os.Stderr, "寬螢幕：field %d RenderTilemap 簽章不符：%x；保留現有改字\n", m.Field(), actual)
			w.warned = true
		}
		return nil
	}
	if sceneHeader(m) {
		w.sources.load(w.sourceCue)
	}
	changed := false
	for _, p := range widescreenWords {
		if m.Read32(p.addr) != p.patched {
			m.Write32(p.addr, p.patched)
			changed = true
		}
	}
	if changed {
		w.Applications++
	}
	m.CPU.SetBreakHandler(func(c *cpu.CPU, pc, code uint32) bool {
		if pc == 0x800eccf0 && code == widescreenBreakCode {
			w.sceneReady = m.Read32(0x800730ac) == 0 && m.Read16(0x800730a0)&1 != 0 && (m.Read32(0x800730f4) == 0 || m.Read32(0x80073100) == 0)
		}
		if pc == 0x800ed520 && code == widescreenSceneCode {
			if w.sceneReady {
				w.appendNeighborScene(m)
			}
			w.sceneReady = false
			c.SetBreakGPR(29, c.GPR(29)+0x48)
			return true
		}
		return widescreenTrap(c, pc, code)
	})
	w.applied = true
	return nil
}

func widescreenTrap(c *cpu.CPU, pc, code uint32) bool {
	switch {
	case pc == 0x800eccf0 && code == widescreenBreakCode:
		c.StoreHalf(c.GPR(16), c.GPR(3)-48)
	case pc == 0x800ece80 && code == widescreenForegroundCode,
		pc == 0x800ed260 && code == widescreenBackgroundCode:
		// 017 §3.2：負 tx 被跳過時表指標不前進，列首須從第零格起算。
		tx := int32(c.GPR(15))
		if tx < 0 {
			tx = 0
		}
		c.SetBreakGPR(2, uint32(tx)+c.GPR(12))
	default:
		return false
	}
	return true
}

func (w *Widescreen) Disable(m *machine.Machine) {
	if w.applied {
		for _, p := range widescreenWords {
			m.Write32(p.addr, p.original)
		}
	}
	m.CPU.SetBreakHandler(nil)
	w.applied = false
	w.sceneReady = false
	w.sources = sceneSources{}
	w.warned = false
}
