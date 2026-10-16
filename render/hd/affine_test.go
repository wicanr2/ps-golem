package hd_test

import (
	"testing"

	"github.com/wicanr2/ps-golem/internal/gpu"
)

// TestAffineTextureSampleCenters 保留上游 UV 取樣中心的可重現反例。
// 參照 beetle-psx-libretro mednafen/psx/gpu_polygon.c 的 DrawTriangle，
// commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later。
// UV 初始偏移是 1<<(COORD_FBS-1-upscale_shift)，N=1 為 0.5、N=2 為 0.25。
// 因此一般仿射貼圖的內部像素也可能改變，不能把「縮回只差邊緣」當成已通過。
func TestAffineTextureSampleCenters(t *testing.T) {
	g := gpu.New()
	s := gpu.NewShadow(1)
	g.SetShadow(s)
	defer func() { g.SetShadow(nil); s.Close() }()
	write := func(words ...uint32) {
		for _, w := range words {
			g.WriteGP0(w)
		}
	}
	write(0xe3000000, 0xe407ffff)
	// 15 位元貼圖頁 (0,256)，每個 texel 直接編入 U、V。
	write(0xa0000000, 0x01000000, 0x00800080)
	for v := uint32(0); v < 128; v++ {
		for u := uint32(0); u < 128; u += 2 {
			a := uint16(0x8000 | u&31 | (v&31)<<5)
			b := uint16(0x8000 | (u+1)&31 | (v&31)<<5)
			write(uint32(a) | uint32(b)<<16)
		}
	}
	// XY：(0,0)、(60,0)、(0,60)；UV：(0,0)、(100,0)、(0,100)。
	write(0x25000000, 0, 0, 60, 0x01100064, 60<<16, 100<<8)
	native := g.VRAM[10*1024+10]
	scaled := s.VRAMCopy()[20*2048+20]
	if native != 0x8231 || scaled != 0x8210 {
		t.Fatalf("取樣中心改變：原生=%04x，HD2=%04x", native, scaled)
	}
	t.Logf("距離幾何邊界至少 10 個原生像素的 (10,10)：原生 UV=(17,17) %04x，HD2 左上 UV=(16,16) %04x", native, scaled)
}
