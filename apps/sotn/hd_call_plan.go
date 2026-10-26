// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import (
	"encoding/binary"
	"fmt"
)

type hdCallTile struct{ Layer, Order, Page, X, Y, U, V, CLUT int }
type hdCallPlan struct {
	Tiles []hdCallTile
	Modes int
	RAM   []byte
}
type hdCallRAM []byte

func (b hdCallRAM) at(a uint32, n int) int {
	p := a & 0x1fffffff
	if p >= 2<<20 || int(p)+n > len(b) {
		panic(fmt.Sprintf("Go hdCallPlan RAM bounds:%08x", a))
	}
	return int(p)
}
func (b hdCallRAM) u32(a uint32) uint32 { return binary.LittleEndian.Uint32(b[b.at(a, 4):]) }
func (b hdCallRAM) u16(a uint32) uint16 { return binary.LittleEndian.Uint16(b[b.at(a, 2):]) }
func (b hdCallRAM) u8(a uint32) uint8   { return b[b.at(a, 1)] }
func (b hdCallRAM) set(a, v uint32)     { binary.LittleEndian.PutUint32(b[b.at(a, 4):], v) }

// Data -> layers -> tiles -> typed output and compatibility packet buffers.
// The 17x16 compatibility window and 24 page buckets are original data-format
// contracts anchored by the saved IDA export; they are not GPU draw calls.
func buildHDCallPlan(input []byte) hdCallPlan {
	if len(input) != 2<<20 {
		panic("Go hdCallPlan RAM size")
	}
	b := hdCallRAM(append([]byte(nil), input...))
	result := hdCallPlan{RAM: b}
	buffer := b.u32(0x8006c37c)
	ot := buffer + 0x474
	sprites, modes := b.u32(0x80097940), b.u32(0x8009792c)
	if sprites > 640 || modes > 1024 {
		panic("unsupported starting budget")
	}
	for layer := 0; layer < 17; layer++ {
		base := uint32(0x80073084)
		if layer > 0 {
			base = 0x800730d8 + uint32(layer-1)*48
		}
		if countdown := b.u32(base + 40); countdown != 0 {
			b.set(base+40, countdown-1)
			continue
		}
		flags := b.u16(base + 28)
		if flags&1 == 0 {
			continue
		}
		if flags != 3 {
			panic("unsupported layer flags")
		}
		data, defs := b.u32(base), b.u32(base+4)
		cols, rows := int(b.u32(base+32))*16, int(b.u32(base+36))*16
		if cols <= 0 || rows <= 0 || cols*rows > 65536 {
			panic("Go hdCallPlan layer size")
		}
		sx, sy := int(int16(b.u16(base+10))), int(int16(b.u16(base+14)))
		if sx < 0 || sy < 0 || b.u16(0x8006c39c) != 0 || b.u16(0x8006c3a0) != 0 {
			panic("unsupported camera")
		}
		order := int(b.u16(base + 24))
		if order < 0 || order+24 > 512 {
			panic("Go hdCallPlan ordering bounds")
		}
		for row := sy / 16; row < sy/16+16 && row < rows; row++ {
			for col := sx / 16; col < sx/16+17 && col < cols; col++ {
				id := uint32(b.u16(data + uint32(row*cols+col)*2))
				if id == 0 || sprites >= 640 {
					continue
				}
				if id >= 4096 {
					panic("unsupported hdCallTile ID")
				}
				page := int(b.u8(b.u32(defs) + id))
				gfx := int(b.u8(b.u32(defs+4) + id))
				pal := uint32(b.u8(b.u32(defs+8) + id))
				if page > 7 {
					panic("unsupported hdCallTile page")
				}
				t := hdCallTile{layer, order, page, col*16 - sx, row*16 - sy, (gfx & 15) * 16, (gfx >> 4) * 16, int(b.u16(0x8003c104 + pal*2))}
				result.Tiles = append(result.Tiles, t)
				packet := buffer + 0x117f4 + sprites*16
				bucket := ot + uint32(order+page)*4
				b.set(packet, 0x03000000|b.u32(bucket)&0xffffff)
				b.set(packet+4, 0x7d000000)
				b.set(packet+8, uint32(uint16(t.X))|uint32(uint16(t.Y))<<16)
				b.set(packet+12, uint32(t.U)|uint32(t.V)<<8|uint32(t.CLUT)<<16)
				b.set(bucket, b.u32(bucket)&0xff000000|packet&0xffffff)
				sprites++
			}
		}
		for page := 0; page < 24 && modes < 1024; page++ {
			packet := buffer + 0xc74 + modes*12
			bucket := ot + uint32(order+page)*4
			b.set(packet, 0x02000000|b.u32(bucket)&0xffffff)
			b.set(packet+4, 0xe1000000+uint32(page)+8)
			b.set(packet+8, 0xe2000000)
			b.set(bucket, b.u32(bucket)&0xff000000|packet&0xffffff)
			modes++
			result.Modes++
		}
	}
	b.set(0x80097940, sprites)
	b.set(0x8009792c, modes)
	return result
}
