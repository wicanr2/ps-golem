package gpu

import "github.com/wicanr2/ps-golem/render/hd"

// SetNativePlane 只在第一筆輸入前接上空白平面。nil 同步後卸下，恢復原生光柵化。
// 平面由呼叫者持有，卸下後才可 Close；不得覆蓋另一個已掛平面（025 §11）。
func (g *GPU) SetNativePlane(p *hd.NativePlane) {
	if p != nil && g.nativePlane != nil {
		panic("原生相容平面已接上")
	}
	if p == nil {
		g.syncNativePixels()
	}
	g.nativePlane = p
}

func (g *GPU) syncNativePixels() {
	if g.nativePlane == nil {
		return
	}
	p := g.nativePlane.Snapshot()
	copy(g.vram[:], p.VRAM)
	g.clutCache, g.clutCacheVB, g.clutCacheN = p.CLUT, p.CLUTAddress, p.CLUTCount
	for i := range g.texCache {
		g.texCache[i].tag = p.TextureTags[i]
		g.texCache[i].data = p.TextureWords[i]
	}
}

// NativeRasterWork 回傳由相容平面承接的精靈、線、多邊形、填色、複製、上傳字數。
// 診斷計數不參與機器摘要，Clone 會清掉它們。
func (g *GPU) NativeRasterWork() [6]uint64 { return g.nativeSkipped }
