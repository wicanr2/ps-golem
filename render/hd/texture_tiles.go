package hd

// 019 §8：只在原線性資料將超界時編製不可變共用圖塊。
import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const textureTileShift = 5
const textureTileSide = 1 << textureTileShift
const textureTilePixels = textureTileSide * textureTileSide

type textureTileBank struct {
	indices []*[textureTilePixels]byte
	colors  []*[textureTilePixels]uint16
}
type textureTileImage struct {
	bank    *textureTileBank
	ids     []uint32
	columns int
	indexed bool
}
type textureTileKey struct {
	digest  [32]byte
	indexed bool
}
type textureTileBuilder struct {
	bank        *textureTileBank
	seen        map[textureTileKey][]uint32
	hash        func([]byte) [32]byte
	used, limit uint64
}

func newTextureTileBuilder(limit uint64) *textureTileBuilder {
	return &textureTileBuilder{bank: &textureTileBank{}, seen: make(map[textureTileKey][]uint32), hash: sha256.Sum256, limit: limit}
}
func (b *textureTileBuilder) reserve(n uint64) error {
	if n > b.limit || b.used > b.limit-n {
		return fmt.Errorf("共用解碼圖素及索引超過 %d bytes", b.limit)
	}
	b.used += n
	return nil
}
func (b *textureTileBuilder) indexID(block *[textureTilePixels]byte) (uint32, error) {
	key := textureTileKey{b.hash(block[:]), true}
	for _, id := range b.seen[key] {
		if *b.bank.indices[id] == *block {
			return id, nil
		}
	}
	if err := b.reserve(textureTilePixels); err != nil {
		return 0, err
	}
	id := uint32(len(b.bank.indices))
	stored := new([textureTilePixels]byte)
	*stored = *block
	b.bank.indices = append(b.bank.indices, stored)
	b.seen[key] = append(b.seen[key], id)
	return id, nil
}
func (b *textureTileBuilder) colorID(block *[textureTilePixels]uint16) (uint32, error) {
	var raw [textureTilePixels * 2]byte
	for i, v := range block {
		binary.LittleEndian.PutUint16(raw[i*2:], v)
	}
	key := textureTileKey{b.hash(raw[:]), false}
	for _, id := range b.seen[key] {
		if *b.bank.colors[id] == *block {
			return id, nil
		}
	}
	if err := b.reserve(textureTilePixels * 2); err != nil {
		return 0, err
	}
	id := uint32(len(b.bank.colors))
	stored := new([textureTilePixels]uint16)
	*stored = *block
	b.bank.colors = append(b.bank.colors, stored)
	b.seen[key] = append(b.seen[key], id)
	return id, nil
}
func (b *textureTileBuilder) add(im *textureImage) error {
	if im.tiles != nil || im.width < 1 || im.height < 1 {
		return fmt.Errorf("圖塊編製需要有效線性圖像")
	}
	indexed := im.indices != nil
	if (indexed && len(im.indices) != im.width*im.height) || (!indexed && len(im.colors) != im.width*im.height) {
		return fmt.Errorf("線性圖素尺寸不符")
	}
	cols, rows := (im.width+textureTileSide-1)>>textureTileShift, (im.height+textureTileSide-1)>>textureTileShift
	if err := b.reserve(uint64(cols) * uint64(rows) * 4); err != nil {
		return err
	}
	out := &textureTileImage{bank: b.bank, ids: make([]uint32, cols*rows), columns: cols, indexed: indexed}
	for ty := 0; ty < rows; ty++ {
		for tx := 0; tx < cols; tx++ {
			var id uint32
			var err error
			if indexed {
				var block [textureTilePixels]byte
				for y := 0; y < textureTileSide && ty*textureTileSide+y < im.height; y++ {
					for x := 0; x < textureTileSide && tx*textureTileSide+x < im.width; x++ {
						block[y*textureTileSide+x] = im.indices[(ty*textureTileSide+y)*im.width+tx*textureTileSide+x]
					}
				}
				id, err = b.indexID(&block)
			} else {
				var block [textureTilePixels]uint16
				for y := 0; y < textureTileSide && ty*textureTileSide+y < im.height; y++ {
					for x := 0; x < textureTileSide && tx*textureTileSide+x < im.width; x++ {
						block[y*textureTileSide+x] = im.colors[(ty*textureTileSide+y)*im.width+tx*textureTileSide+x]
					}
				}
				id, err = b.colorID(&block)
			}
			if err != nil {
				return err
			}
			out.ids[ty*cols+tx] = id
		}
	}
	im.tiles = out
	im.indices = nil
	im.colors = nil
	return nil
}
func (t *textureTileImage) pixel(x, y int) (uint16, bool) {
	id := t.ids[(y>>textureTileShift)*t.columns+(x>>textureTileShift)]
	at := (y&(textureTileSide-1))*textureTileSide + (x & (textureTileSide - 1))
	if t.indexed {
		return uint16(t.bank.indices[id][at]), true
	}
	return t.bank.colors[id][at], false
}
