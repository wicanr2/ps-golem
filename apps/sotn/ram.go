package sotn

import (
	"encoding/binary"

	"github.com/wicanr2/ps-golem/machine"
)

// RAM 位址（docs/spec/015-sotn-autoplay.md §4）。實體表與狀態結構的位置與 sotn-decomp 記載的美版相同，
// 本版以 RAM 差分核對過；欄位語意的推論等級寫在規格台帳。
const (
	EntityBase = 0x0733D8 // 實體表，每個 0xBC 位元組；0 號是玩家
	EntitySize = 0xBC
	StatusHP   = 0x097BA0 // 玩家 HP（s32）
	StatusMax  = 0x097BA4 // 玩家 HP 上限
)

// Entity 是一個實體槽的觀測值。
type Entity struct {
	X, Y int16 // 畫面座標（16.16 定點的整數部分）
	HP   int16 // +0x3E
}

func s16(m *machine.Machine, a uint32) int16 {
	return int16(binary.LittleEndian.Uint16(m.ReadRAM(a, 2)))
}

// EntityAt 讀第 slot 個實體槽。
func EntityAt(m *machine.Machine, slot int) Entity {
	a := uint32(EntityBase + slot*EntitySize)
	return Entity{X: s16(m, a+2), Y: s16(m, a+6), HP: s16(m, a+0x3E)}
}

// PlayerHP 讀玩家 HP。
func PlayerHP(m *machine.Machine) int { return int(int32(binary.LittleEndian.Uint32(m.ReadRAM(StatusHP, 4)))) }
