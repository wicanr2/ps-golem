package hd

// 019 §8–8.1: build immutable tiles during Load; retain accepted32 packs.
import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
)

const textureTileShift = 5
const textureTileSide = 1 << textureTileShift
const textureTilePixels = textureTileSide * textureTileSide
const textureSmallTileShift = 3

type textureTileBank struct {
	shift        uint
	indices      [][]byte
	colors       [][]uint16
	paletted     bool
	packedColors []textureColorTile
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
type textureBudgetError struct{ message string }

func (e *textureBudgetError) Error() string { return e.message }
func isTextureBudget(err error) bool {
	var budget *textureBudgetError
	return errors.As(err, &budget)
}

func newTextureTileBuilder(limit uint64) *textureTileBuilder {
	return newTextureTileBuilderWithShift(limit, textureTileShift)
}
func newTextureTileBuilderWithShift(limit uint64, shift uint) *textureTileBuilder {
	if shift != textureTileShift && shift != textureSmallTileShift {
		panic("invalid internal tile shift")
	}
	return &textureTileBuilder{bank: &textureTileBank{shift: shift}, seen: make(map[textureTileKey][]uint32), hash: sha256.Sum256, limit: limit}
}
func (b *textureTileBuilder) reserve(n uint64) error {
	if n > b.limit || b.used > b.limit-n {
		return &textureBudgetError{fmt.Sprintf("共用解碼圖素及索引超過 %d bytes", b.limit)}
	}
	b.used += n
	return nil
}
func (b *textureTileBuilder) indexID(block []byte) (uint32, error) {
	if len(block) != 1<<(2*b.bank.shift) {
		return 0, fmt.Errorf("索引圖塊尺寸不符")
	}
	key := textureTileKey{b.hash(block), true}
	for _, id := range b.seen[key] {
		if bytes.Equal(b.bank.indices[id], block) {
			return id, nil
		}
	}
	if err := b.reserve(uint64(len(block))); err != nil {
		return 0, err
	}
	id := uint32(len(b.bank.indices))
	b.bank.indices = append(b.bank.indices, append([]byte(nil), block...))
	b.seen[key] = append(b.seen[key], id)
	return id, nil
}
func (b *textureTileBuilder) colorID(block []uint16) (uint32, error) {
	if len(block) != 1<<(2*b.bank.shift) {
		return 0, fmt.Errorf("色字圖塊尺寸不符")
	}
	raw := make([]byte, len(block)*2)
	for i, v := range block {
		binary.LittleEndian.PutUint16(raw[i*2:], v)
	}
	key := textureTileKey{b.hash(raw), false}
	for _, id := range b.seen[key] {
		if (b.bank.paletted && b.bank.packedColors[id].equal(block)) || (!b.bank.paletted && slices.Equal(b.bank.colors[id], block)) {
			return id, nil
		}
	}
	if b.bank.paletted {
		tile := encodeTextureColorTile(block)
		if err := b.reserve(tile.residentBytes()); err != nil {
			return 0, err
		}
		id := uint32(len(b.bank.packedColors))
		b.bank.packedColors = append(b.bank.packedColors, tile)
		b.seen[key] = append(b.seen[key], id)
		return id, nil
	}
	if err := b.reserve(uint64(len(block)) * 2); err != nil {
		return 0, err
	}
	id := uint32(len(b.bank.colors))
	b.bank.colors = append(b.bank.colors, append([]uint16(nil), block...))
	b.seen[key] = append(b.seen[key], id)
	return id, nil
}
func (b *textureTileBuilder) add(im *textureImage) error {
	if im.width < 1 || im.height < 1 {
		return fmt.Errorf("圖塊編製需要有效圖像")
	}
	source := im.tiles
	indexed := im.indices != nil
	if source != nil {
		if source.bank == b.bank || im.indices != nil || im.colors != nil {
			return fmt.Errorf("圖塊來源表示無效")
		}
		indexed = source.indexed
	} else if (indexed && len(im.indices) != im.width*im.height) || (!indexed && len(im.colors) != im.width*im.height) {
		return fmt.Errorf("線性圖素尺寸不符")
	}
	shift := b.bank.shift
	side := 1 << shift
	pixels := side * side
	cols, rows := (im.width+side-1)>>shift, (im.height+side-1)>>shift
	if err := b.reserve(uint64(cols) * uint64(rows) * 4); err != nil {
		return err
	}
	out := &textureTileImage{bank: b.bank, ids: make([]uint32, cols*rows), columns: cols, indexed: indexed}
	for ty := 0; ty < rows; ty++ {
		for tx := 0; tx < cols; tx++ {
			var id uint32
			var err error
			if indexed {
				block := make([]byte, pixels)
				for y := 0; y < side && ty*side+y < im.height; y++ {
					for x := 0; x < side && tx*side+x < im.width; x++ {
						if source != nil {
							v, _ := source.pixel(tx*side+x, ty*side+y)
							block[y*side+x] = byte(v)
						} else {
							block[y*side+x] = im.indices[(ty*side+y)*im.width+tx*side+x]
						}
					}
				}
				id, err = b.indexID(block)
			} else {
				block := make([]uint16, pixels)
				for y := 0; y < side && ty*side+y < im.height; y++ {
					for x := 0; x < side && tx*side+x < im.width; x++ {
						if source != nil {
							block[y*side+x], _ = source.pixel(tx*side+x, ty*side+y)
						} else {
							block[y*side+x] = im.colors[(ty*side+y)*im.width+tx*side+x]
						}
					}
				}
				id, err = b.colorID(block)
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
	shift := t.bank.shift
	side := 1 << shift
	id := t.ids[(y>>shift)*t.columns+(x>>shift)]
	at := (y&(side-1))*side + (x & (side - 1))
	if t.indexed {
		return uint16(t.bank.indices[id][at]), true
	}
	if t.bank.paletted {
		return t.bank.packedColors[id].pixel(at), false
	}
	return t.bank.colors[id][at], false
}

// The pack has not escaped Load. Retiling never mutates the source bank or
// re-quantizes words; partially rebuilt packs are discarded on failure.
func compactTextureEntries(entries map[textureKey]*textureImage, limit uint64, shift uint) (*textureTileBuilder, error) {
	b := newTextureTileBuilderWithShift(limit, shift)
	for _, im := range entries {
		if err := b.add(im); err != nil {
			return nil, err
		}
	}
	return b, nil
}
func startTextureTiles(entries map[textureKey]*textureImage, limit uint64) (*textureTileBuilder, error) {
	b, err := compactTextureEntries(entries, limit, textureTileShift)
	if isTextureBudget(err) {
		b, err = compactTextureEntries(entries, limit, textureSmallTileShift)
	}
	if isTextureBudget(err) {
		return compactTexturePaletteEntries(entries, limit)
	}
	return b, err
}
