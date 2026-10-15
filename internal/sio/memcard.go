package sio

import "bytes"

// MemcardSize 是記憶卡的大小（128 KB，1024 個 frame × 128 位元組）。
const MemcardSize = 1 << 17

// Memcard 是記憶卡（Beetle InputDevice_Memcard）。Data 可直接讀寫存檔；Dirty 表示寫入過、需要存回檔案。
type Memcard struct {
	Data  [MemcardSize]byte
	Dirty bool

	presenceNew bool
	rwBuf       [128]byte
	writeXor    uint8
	dtr         bool
	phase       int
	bitpos      uint
	rx          uint8
	cmd         uint8
	addr        uint32
	calcXor     uint8
	tx          uint8
	txCount     int
}

// NewMemcard 建立一張格式化好的空白卡（照 Beetle InputDevice_Memcard_Format）。
func NewMemcard() *Memcard {
	c := &Memcard{presenceNew: true}
	c.Format()
	return c
}

// Load 以存檔內容建立記憶卡；長度不是 128 KB 時回 false。
func (c *Memcard) Load(b []byte) bool {
	if len(b) != MemcardSize {
		return false
	}
	copy(c.Data[:], b)
	c.Dirty = false
	return true
}

// Format 寫入空白卡的格式（header frame、15 個目錄 frame、壞區表）。
func (c *Memcard) Format() {
	c.Data = [MemcardSize]byte{}
	c.Data[0x00], c.Data[0x01], c.Data[0x7F] = 0x4D, 0x43, 0x0E
	for a := 0x80; a < 0x800; a += 0x80 {
		c.Data[a] = 0xA0
		c.Data[a+8], c.Data[a+9] = 0xFF, 0xFF
		c.Data[a+0x7F] = 0xA0
	}
	for a := 0x800; a < 0x1200; a += 0x80 {
		c.Data[a], c.Data[a+1], c.Data[a+2], c.Data[a+3] = 0xFF, 0xFF, 0xFF, 0xFF
		c.Data[a+8], c.Data[a+9] = 0xFF, 0xFF
	}
}

// SetDTR 實作 Device。
func (c *Memcard) SetDTR(on bool) {
	if !c.dtr && on {
		c.phase = 0
		c.bitpos = 0
		c.txCount = 0
	}
	c.dtr = on
}

// Clock 實作 Device。
func (c *Memcard) Clock(txd bool) (bool, int64) {
	if !c.dtr {
		return true, 0
	}
	ret := true
	if c.txCount > 0 {
		ret = (c.tx>>c.bitpos)&1 != 0
	}
	c.rx &^= 1 << c.bitpos
	if txd {
		c.rx |= 1 << c.bitpos
	}
	c.bitpos = (c.bitpos + 1) & 7
	if c.bitpos != 0 {
		return ret, 0
	}
	if c.txCount > 0 {
		c.txCount--
	}
	send := func(v uint8) { c.tx, c.txCount = v, 1 }
	switch p := c.phase; {
	case p >= 1024 && p <= 1151:
		v := uint8(0)
		if c.addr != 0xFFFF {
			v = c.Data[c.addr<<7+uint32(p-1024)]
		}
		c.calcXor ^= v
		send(v)
		c.phase++
	case p >= 2048 && p <= 2175:
		c.calcXor ^= c.rx
		c.rwBuf[p-2048] = c.rx
		send(c.rx)
		c.phase++
	default:
		switch p {
		case 0:
			if c.rx != 0x81 {
				c.phase = -1
			} else {
				if c.presenceNew {
					send(0x08)
				} else {
					send(0x00)
				}
				c.phase++
			}
		case 1:
			c.cmd = c.rx
			if c.cmd == 'R' || c.cmd == 'W' {
				c.phase++
				send(0x5A)
			} else {
				c.phase = -1
				c.tx, c.txCount = 0, 0
			}
		case 2:
			send(0x5D)
			c.phase++
		case 3:
			send(0x00)
			if c.cmd == 'R' {
				c.phase = 1000
			} else {
				c.phase = 2000
			}
		case 1000:
			c.addr = uint32(c.rx) << 8
			send(c.rx)
			c.phase++
		case 1001:
			c.addr |= uint32(c.rx)
			send('\\')
			c.phase++
		case 1002:
			if c.addr >= MemcardSize>>7 {
				c.addr = 0xFFFF
			}
			c.calcXor = 0
			send(']')
			c.phase++
		case 1003:
			send(uint8(c.addr >> 8))
			c.calcXor ^= c.tx
			c.phase++
		case 1004:
			send(uint8(c.addr))
			c.calcXor ^= c.tx
			if c.addr == 0xFFFF {
				c.phase = -1
			} else {
				c.phase = 1024
			}
		case 1024 + 128:
			send(c.calcXor)
			c.phase++
		case 1024 + 129:
			send('G')
			c.phase = -1
		case 2000:
			c.calcXor = c.rx
			c.addr = uint32(c.rx) << 8
			send(c.rx)
			c.phase++
		case 2001:
			c.calcXor ^= c.rx
			c.addr |= uint32(c.rx)
			send(c.rx)
			c.phase = 2048
		case 2048 + 128:
			c.writeXor = c.rx
			send('\\')
			c.phase++
		case 2048 + 129:
			send(']')
			c.phase++
		case 2048 + 130:
			switch {
			case c.calcXor != c.writeXor:
				send('N')
			case c.addr >= MemcardSize>>7:
				send(0xFF)
			default:
				send('G')
				c.presenceNew = false
				dst := c.Data[c.addr<<7 : c.addr<<7+128]
				if !bytes.Equal(dst, c.rwBuf[:]) {
					copy(dst, c.rwBuf[:])
					c.Dirty = true
				}
			}
			c.phase = -1
		}
	}
	if c.txCount > 0 {
		return ret, 0x100
	}
	return ret, 0
}
