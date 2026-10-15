package machine

import (
	"encoding/binary"
	"fmt"
)

// ShellEntry 是 BIOS 初始化完核心、進入 shell 的位址（006 §5）。
const ShellEntry = 0x80030000

// EXEHeader 是 PS-EXE 的檔頭欄位（psx-spx cdromfileformats.md）。
type EXEHeader struct {
	PC, GP     uint32
	Dest, Size uint32
	BSS, BSSSz uint32
	SPBase     uint32
	SPOff      uint32
}

// ParseEXE 解析 PS-EXE，回傳檔頭與程式本體。
func ParseEXE(b []byte) (EXEHeader, []byte, error) {
	var h EXEHeader
	if len(b) < 0x800 || string(b[:8]) != "PS-X EXE" {
		return h, nil, fmt.Errorf("不是 PS-EXE")
	}
	u := func(o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }
	h = EXEHeader{
		PC: u(0x10), GP: u(0x14), Dest: u(0x18), Size: u(0x1C),
		BSS: u(0x28), BSSSz: u(0x2C), SPBase: u(0x30), SPOff: u(0x34),
	}
	body := b[0x800:]
	if uint32(len(body)) < h.Size {
		return h, nil, fmt.Errorf("本體 %d 位元組，檔頭宣稱 %d", len(body), h.Size)
	}
	return h, body[:h.Size], nil
}

// SideloadEXE 把 PS-EXE 載入記憶體並設定暫存器。應在 PC 到達 ShellEntry 時呼叫。
func (m *Machine) SideloadEXE(b []byte) error {
	h, body, err := ParseEXE(b)
	if err != nil {
		return err
	}
	for i, v := range body {
		m.ram[(physical(h.Dest)+uint32(i))&(ramSize-1)] = v
	}
	for i := uint32(0); i < h.BSSSz; i++ {
		m.ram[(physical(h.BSS)+i)&(ramSize-1)] = 0
	}
	c := m.CPU
	c.SetGPR(28, h.GP)
	if h.SPBase != 0 {
		sp := h.SPBase + h.SPOff
		c.SetGPR(29, sp)
		c.SetGPR(30, sp)
	}
	c.SetGPR(4, 1)
	c.SetGPR(5, 0)
	c.SetPC(h.PC)
	return nil
}
