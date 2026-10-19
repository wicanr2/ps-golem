package hd

// 來源、快取及保守複製契約：018-hd-texture-replacement §3–4。
// 這些資料只屬於影子 GPU，不參與原生 VRAM、讀回或機器摘要。
type textureUpload struct {
	refs  int
	key   [32]byte
	w, h  uint32
	words []uint16 // 只有傾印需要保留原始內容
}

type textureOrigin struct {
	upload *textureUpload
	offset uint32
	word   uint16
}

type TextureStats struct {
	Uploads, Lookups, Hits, ReplacedSamples uint64
}

type textureTracking struct {
	origins    []textureOrigin
	pack       *TexturePack
	dump       *TextureDump
	words      []uint16
	accepted   []bool
	clutKeys   [2][32]byte
	clutValid  [2]bool
	field      uint64
	stats      TextureStats
	lastUpload *textureUpload
	lastDepth  int
	lastCLUT   [32]byte
	lastImage  *textureImage
}

func (g *Renderer) configureTextures(pack *TexturePack, dump *TextureDump) {
	if pack.Len() == 0 && dump == nil {
		return
	}
	g.textures = &textureTracking{origins: make([]textureOrigin, VRAMWidth*VRAMHeight), pack: pack, dump: dump}
}

func (g *Renderer) startTextureUpload() {
	if t := g.textures; t != nil {
		t.words = make([]uint16, 0, int(g.fbW*g.fbH))
		t.accepted = make([]bool, 0, int(g.fbW*g.fbH))
	}
}

func (g *Renderer) abortTextureUpload() {
	if t := g.textures; t != nil {
		t.words = nil
		t.accepted = nil
	}
}

func (g *Renderer) observeTextureWord(word uint16, accepted bool) {
	if t := g.textures; t != nil {
		t.words = append(t.words, word)
		t.accepted = append(t.accepted, accepted)
	}
}

func (g *Renderer) finishTextureUpload() {
	t := g.textures
	if t == nil {
		return
	}
	u := &textureUpload{key: textureUploadDigest(int(g.fbW), int(g.fbH), t.words), w: g.fbW, h: g.fbH}
	u.refs = 1
	if t.dump != nil {
		u.words = t.words
		t.dump.holdRaw(u)
	}
	defer t.release(u)
	for i, accepted := range t.accepted {
		if !accepted {
			continue
		}
		x := (g.fbX + uint32(i)%g.fbW) & 1023
		y := (g.fbY + uint32(i)/g.fbW) & 511
		t.setOrigin(y*1024+x, textureOrigin{upload: u, offset: uint32(i), word: t.words[i]})
	}
	t.stats.Uploads++
	if t.dump != nil {
		t.dump.upload(u, g.fbX&1023, g.fbY&511, t.field)
	}
	t.words = nil
	t.accepted = nil
}

// copyTextureOrigins 先按原生 128 字分塊推進來源工作格，再由原 fbCopy 寫影子像素。
// 均一格的所有子列等價；部分遮罩／非均一格不推測來源。
// 反例及固定種子逐子像素 oracle：workplace/hdtex/texture-origin-probe.go。
func (g *Renderer) copyTextureOrigins(sx, sy, dx, dy, w, h uint32) {
	t := g.textures
	if t == nil {
		return
	}
	type cell struct {
		value   uint16
		origin  textureOrigin
		uniform bool
	}
	cells := make(map[uint32]cell)
	held := map[*textureUpload]bool{}
	defer func() {
		for u := range held {
			t.release(u)
		}
	}()
	n := uint32(1) << g.shift
	stride := uint32(1024) << g.shift
	get := func(x, y uint32) cell {
		x &= 1023
		y &= 511
		idx := y*1024 + x
		if c, ok := cells[idx]; ok {
			return c
		}
		c := cell{value: g.texel(x, y), origin: t.origins[idx], uniform: true}
		for yy := uint32(0); yy < n; yy++ {
			for xx := uint32(0); xx < n; xx++ {
				if g.VRAM[(y*n+yy)*stride+x*n+xx] != c.value {
					c.uniform = false
					c.origin = textureOrigin{}
				}
			}
		}
		if u := c.origin.upload; u != nil && !held[u] {
			u.refs++
			held[u] = true
		}
		return c
	}
	var tmp [128]cell
	for y := uint32(0); y < h; y++ {
		for x := uint32(0); x < w; x += 128 {
			count := min(uint32(128), w-x)
			for c := uint32(0); c < count; c++ {
				tmp[c] = get(sx+x+c, sy+y)
			}
			for c := uint32(0); c < count; c++ {
				idx := ((dy+y)&511)*1024 + ((dx + x + c) & 1023)
				dest, src := get(dx+x+c, dy+y), tmp[c]
				if !dest.uniform && g.maskEvalAND != 0 {
					dest.origin = textureOrigin{}
				} else if !dest.uniform || dest.value&g.maskEvalAND == 0 {
					dest = src
					dest.value |= g.maskSetOR
					if !src.uniform || dest.value != src.value {
						dest.origin = textureOrigin{}
					}
				}
				cells[idx] = dest
				t.setOrigin(idx, dest.origin)
			}
		}
	}
}

func (g *Renderer) textureCLUTKey(tm uint32) [32]byte {
	if tm >= 2 {
		return [32]byte{}
	}
	t := g.textures
	if !t.clutValid[tm] {
		n := 16
		if tm == 1 {
			n = 256
		}
		t.clutKeys[tm] = textureCLUTDigest(g.clutCache[:n])
		t.clutValid[tm] = true
	}
	return t.clutKeys[tm]
}

// 呼叫前須由 getTexel 載入相同位置，來源與字都取自同一條快取線。
// resolveTexture 只記住不可變 pack 的最後一筆結果；含 miss。
// 018-hd-texture-replacement.md §4.1：只快取不可變查詢結果。
func (t *textureTracking) resolveTexture(u *textureUpload, depth int, clut [32]byte) *textureImage {
	if t.lastUpload == u && t.lastDepth == depth && t.lastCLUT == clut {
		return t.lastImage
	}
	e := t.pack.lookup(u.key, depth, clut)
	t.lastUpload, t.lastDepth, t.lastCLUT, t.lastImage = u, depth, clut, e
	return e
}

func (g *Renderer) lookupTexture(tm, u, v uint32) (*textureImage, int, int) {
	t := g.textures
	if t == nil {
		return nil, 0, 0
	}
	if tm > 2 {
		tm = 2
	}
	uExt := (u & g.twxAnd) + g.twxAdd
	fx := (uExt >> (2 - tm)) & 1023
	fy := (v & g.twyAnd) + g.twyAdd
	gro := fy*1024 + fx
	slot := (gro>>2)&7 | (gro>>7)&0xf8
	if tm == 0 {
		slot = (gro>>2)&3 | (gro>>8)&0xfc
	}
	c := &g.texCache[slot]
	o := c.origin[gro&3]
	t.stats.Lookups++
	if o.upload == nil || o.word != c.data[gro&3] {
		return nil, 0, 0
	}
	depth := 4
	per := uint32(4)
	if tm == 1 {
		depth = 8
		per = 2
	}
	if tm == 2 {
		depth = 15
		per = 1
	}
	clut := g.textureCLUTKey(tm)
	if t.dump != nil {
		t.dump.use(o.upload, depth, clut, g.clutCache[:], t.field)
	}
	e := t.resolveTexture(o.upload, depth, clut)
	if e == nil {
		return nil, 0, 0
	}
	t.stats.Hits++
	x := int((o.offset%o.upload.w)*per + (uExt & (per - 1)))
	y := int(o.offset / o.upload.w)
	return e, x, y
}

func (g *Renderer) texturePixel(e *textureImage, x, y int, fu, fv uint32, bits uint) uint16 {
	x = x*e.scale + int(uint64(fu)*uint64(e.scale)>>bits)
	y = y*e.scale + int(uint64(fv)*uint64(e.scale)>>bits)
	g.textures.stats.ReplacedSamples++
	if e.indices != nil {
		return g.clutCache[e.indices[y*e.width+x]]
	}
	return e.colors[y*e.width+x]
}

func (g *Renderer) getTexelHD(tm, u, v, fu, fv uint32, bits uint) uint16 {
	native := g.getTexel(tm, u, v)
	e, x, y := g.lookupTexture(tm, u, v)
	if e == nil {
		return native
	}
	return g.texturePixel(e, x, y, fu, fv, bits)
}

// 命中素材時才走子像素 plot；未命中保留舊 plot 的整格混色與 mask 語意。
func (g *Renderer) drawHDTexturePixel(x, y int32, u, v uint32, uInc, vInc int32, r, gg, b uint32, modulate bool, mode int) bool {
	// 007 §9：精靈中性色不改變替換素材的任何 RGB5 或 STP 位元。
	modulate = modulate && (r != 128 || gg != 128 || b != 128)
	e, tx, ty := g.lookupTexture(g.texMode, u, v)
	if e == nil {
		return false
	}
	n := int32(1) << g.shift
	for yy := int32(0); yy < n; yy++ {
		for xx := int32(0); xx < n; xx++ {
			fx, fy := xx, yy
			if uInc < 0 {
				fx = n - 1 - xx
			}
			if vInc < 0 {
				fy = n - 1 - yy
			}
			color := g.texturePixel(e, tx, ty, uint32(fx), uint32(fy), g.shift)
			if color == 0 {
				continue
			}
			if modulate {
				color = g.modTexel(2, 3, color, r, gg, b)
			}
			g.plotScaled((x<<g.shift)+xx, (y<<g.shift)+yy, color, mode, true)
		}
	}
	return true
}

// VRAM、快取與傾印各自持有來源；最後一個引用解除時釋放未使用的原始字。
func (t *textureTracking) release(u *textureUpload) {
	if u == nil {
		return
	}
	u.refs--
	if u.refs == 0 && len(u.words) > 0 {
		t.dump.releaseRaw(u)
	}
}
func (t *textureTracking) setOrigin(index uint32, origin textureOrigin) {
	if origin.upload != nil {
		origin.upload.refs++
	}
	old := t.origins[index].upload
	t.origins[index] = origin
	t.release(old)
}
func (t *textureTracking) setCachedOrigin(dest *textureOrigin, origin textureOrigin) {
	if origin.upload != nil {
		origin.upload.refs++
	}
	old := dest.upload
	*dest = origin
	t.release(old)
}
