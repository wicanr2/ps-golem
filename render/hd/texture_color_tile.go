package hd

// 019 §8.2: exact immutable words; only a resident-budget fallback uses this.
// Mode byte, palette/raw words and packed indices all count toward the budget.
type textureColorTile struct {
	words []uint16
	codes []byte
	bits  uint8
}

func encodeTextureColorTile(block []uint16) textureColorTile {
	palette := make([]uint16, 0, 32)
	ids := make([]byte, len(block))
	for at, w := range block {
		id := 0
		for id < len(palette) && palette[id] != w {
			id++
		}
		if id == len(palette) {
			palette = append(palette, w)
		}
		ids[at] = byte(id)
	}
	bits := uint8(8)
	switch {
	case len(palette) == 1:
		bits = 0
	case len(palette) <= 2:
		bits = 1
	case len(palette) <= 4:
		bits = 2
	case len(palette) <= 16:
		bits = 4
	}
	size := len(block) * int(bits) / 8
	if len(palette)*2+size >= len(block)*2 {
		return textureColorTile{words: append([]uint16(nil), block...), bits: 16}
	}
	codes := make([]byte, size)
	if bits > 0 {
		for at, id := range ids {
			codes[at*int(bits)/8] |= id << uint(at*int(bits)%8)
		}
	}
	return textureColorTile{words: palette, codes: codes, bits: bits}
}
func (t *textureColorTile) pixel(at int) uint16 {
	if t.bits == 16 {
		return t.words[at]
	}
	if t.bits == 0 {
		return t.words[0]
	}
	bit := at * int(t.bits)
	id := t.codes[bit/8] >> uint(bit%8) & byte((1<<t.bits)-1)
	return t.words[id]
}
func (t *textureColorTile) equal(block []uint16) bool {
	for at, w := range block {
		if t.pixel(at) != w {
			return false
		}
	}
	return true
}
func (t *textureColorTile) residentBytes() uint64 {
	return uint64(len(t.words)*2 + len(t.codes) + 1)
}
func compactTexturePaletteEntries(entries map[textureKey]*textureImage, limit uint64) (*textureTileBuilder, error) {
	b := newTextureTileBuilderWithShift(limit, textureSmallTileShift)
	b.bank.paletted = true
	for _, im := range entries {
		if err := b.add(im); err != nil {
			return nil, err
		}
	}
	return b, nil
}
