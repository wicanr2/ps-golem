package gpu

import (
	"slices"
	"testing"
)

func TestShadowForwarding(t *testing.T) {
	g := New()
	s := NewShadow(0)
	g.SetShadow(s)
	defer func() { g.SetShadow(nil); s.Close() }()
	// 上傳、讀回、折線、環繞複製、半透明、mask、裁切、貼圖與隔行。
	groups := [][]uint32{
		{0xe3000000, 0xe407ffff, 0xe1000600},
		{0xa0000000, 0x01ff03ff, 0x00020003, 0x8001001f, 0x03e07c00, 0x12345678},
		{0xc0000000, 0x01ff03ff, 0x00020003},
		{0x020000ff, 0x01ff03f0, 0x00020020},
		{0x80000000, 0x01ff03ff, 0x000003fe, 0x00020003},
		{0x6000ff00, 0x00100010, 0x00080008, 0x620000ff, 0x00120012, 0x00080008},
		{0x480000ff, 0x00100010, 0x00200020, 0x00100030, 0x50005000},
		{0x300000ff, 0x00100010, 0x0000ff00, 0x00200030, 0x00ff0000, 0x00300010},
		{0xe6000001, 0x680000ff, 0x00100010, 0xe6000002, 0x6800ff00, 0x00100010},
		{0xe6000000, 0xe1000100, 0x65000000, 0x00300030, 0x00000000, 0x00080008},
	}
	for i, words := range groups {
		for _, w := range words {
			g.WriteGP0(w)
		}
		if i == 2 {
			for n := 0; n < 3; n++ {
				g.ReadData()
			}
		}
		if !slices.Equal(g.vram[:], s.VRAMCopy()) {
			t.Fatalf("第 %d 組轉送不同", i)
		}
	}
	g.WriteGP1(0x08000024)
	for n := 0; n < 600; n++ {
		g.TickVideo(videoPerLine)
		g.WriteGP0(0x6800ff00)
		g.WriteGP0(uint32(n%500)<<16 | 64)
	}
	if !slices.Equal(g.vram[:], s.VRAMCopy()) {
		t.Fatal("隔行轉送不同")
	}
	clone := g.Clone()
	if clone.shadow != nil {
		t.Fatal("Clone 帶了影子")
	}
	clone.WriteGP0(0x680000ff)
	clone.WriteGP0(64)
	if !slices.Equal(g.vram[:], s.VRAMCopy()) {
		t.Fatal("Clone 的輸入洩漏至影子")
	}
	for n := 0; n < 10000; n++ {
		g.WriteGP0(0x680000ff)
		g.WriteGP0(uint32(n%500)<<16 | 32)
	}
	if !slices.Equal(g.vram[:], s.VRAMCopy()) {
		t.Fatal("跨批次轉送不同")
	}
}

func TestShadowScaledTriangle(t *testing.T) {
	// 斜邊內部應有獨立子像素，不能只把原生完成圖放大。
	for _, shift := range []uint{0, 1, 2, 3} {
		s := NewShadow(shift)
		for _, w := range []uint32{0xe3000000, 0xe407ffff, 0x200000ff, 0x000a000a, 0x000a0014, 0x0014000a} {
			s.GP0(w)
		}
		v := s.VRAMCopy()
		n := 1 << shift
		if len(v) != 1024*512*n*n {
			t.Fatal("VRAM 尺寸不符")
		}
		if v[10*n*1024*n+10*n] != 31 {
			t.Fatal("三角形內部未著色")
		}
		if n > 1 {
			a := v[(15*n)*1024*n+14*n]
			b := v[(15*n+n-1)*1024*n+14*n+n-1]
			if a == b {
				t.Fatal("斜邊沒有子像素差異")
			}
		}
		s.Close()
		s.Close()
	}
}
