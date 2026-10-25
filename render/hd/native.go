package hd

// NativePlane 封裝無美術的一倍原生相容角色；不接受呈現精度或素材設定（025 §11）。
// 由機器執行緒寫入、觀測及關閉；device 由此消費端持有，nil 使用 CPU。
type NativePlane struct {
	shadow  *Shadow
	pending int
}

// NativeSnapshot 保留原始 RGB555/STP 與像素快取，所有陣列／切片均為獨立副本。
type NativeSnapshot struct {
	VRAM                   []uint16
	CLUT                   [256]uint16
	CLUTAddress, CLUTCount uint32
	TextureTags            [256]uint32
	TextureWords           [256][4]uint16
}

const nativePendingLimit = 4096

func NewNativePlane(device RasterDevice) *NativePlane {
	s := NewShadowWithDevice(0, nil, nil, device)
	s.read(func(g *Renderer) { g.nativeCompatibility = true })
	return &NativePlane{shadow: s}
}

// bound 讓無頭及未要求畫面的流程也不會累積無界工作。
func (p *NativePlane) bound() {
	p.pending++
	if p.pending >= nativePendingLimit {
		p.shadow.read(func(g *Renderer) { g.rasterFence() })
		p.pending = 0
	}
}
func (p *NativePlane) GP0(v uint32)    { p.shadow.GP0(v); p.bound() }
func (p *NativePlane) GP1(v uint32)    { p.shadow.GP1(v); p.bound() }
func (p *NativePlane) Field(v bool)    { p.shadow.Field(v); p.bound() }
func (p *NativePlane) Fence(fn func()) { p.shadow.Fence(fn) }

func (p *NativePlane) Snapshot() (out NativeSnapshot) {
	p.shadow.read(func(g *Renderer) {
		g.syncRaster()
		out.VRAM = append([]uint16(nil), g.VRAM...)
		out.CLUT, out.CLUTAddress, out.CLUTCount = g.clutCache, g.clutCacheVB, g.clutCacheN
		for i, line := range g.texCache {
			out.TextureTags[i] = line.tag
			out.TextureWords[i] = line.data
		}
		g.rasterFence()
	})
	p.pending = 0
	return
}
func (p *NativePlane) ReadData() (word uint32) {
	p.shadow.read(func(g *Renderer) { word = g.ReadData(); g.rasterFence() })
	p.pending = 0
	return
}
func (p *NativePlane) RasterStatistics() RasterStats { return p.shadow.RasterStatistics() }

// Close 可重複呼叫；須先從 GPU 卸下，並維持 GPU 服務直到正常收尾完成。
func (p *NativePlane) Close() { p.shadow.Close() }
