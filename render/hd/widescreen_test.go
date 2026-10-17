package hd

import (
	"image/color"
	"slices"
	"testing"
)

func TestSideCanvasDisplayOffsetAndHorizontalBuffers(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := wideRenderer(shift)
		n := 1 << shift
		// 遊玩雙緩衝沿 x 排列，繪圖區比顯示起點低 20 列。
		for buffer, rgb := range []uint32{0x0000ff, 0x00ff00} {
			x := uint32(buffer * 256)
			send(g, 0xe3000000|20<<10|x, 0xe4000000|226<<10|(x+255))
			send(g, 0x60000000|rgb, 20<<16|((x-8)&0xffff), 16|207<<16)
			send(g, 0x60000000|rgb, 20<<16|(x+248), 16|207<<16)
		}
		for buffer, want := range []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}} {
			// 非零顯示 y 同樣以相對列對齊。
			for _, displayY := range []int{0, 10} {
				g.WriteGP1(0x05000000 | uint32(displayY)<<10 | uint32(buffer*256))
				s := g.Screen()
				for _, seam := range []int{43 * n, (43 + 256) * n} {
					for x := seam - 8*n; x < seam+8*n; x++ {
						for y := (20 - displayY) * n; y < (227-displayY)*n; y++ {
							if got := s.RGBAAt(x, y); got != want {
								t.Fatalf("倍率=%d buffer=%d 顯示y=%d (%d,%d)=%v，預期%v", n, buffer, displayY, x, y, got, want)
							}
						}
					}
				}
				for _, y := range []int{(20-displayY)*n - 1, (227 - displayY) * n} {
					for _, x := range []int{42 * n, 299 * n} {
						if got := s.RGBAAt(x, y); got != (color.RGBA{A: 255}) {
							t.Fatalf("繪圖區外未補黑 (%d,%d)=%v", x, y, got)
						}
					}
				}
			}
		}
		// y1 在顯示範圍之前或恰好落在下界時都不能合成。
		g.WriteGP1(0x05000000 | 21<<10)
		if got := g.Screen().RGBAAt(42*n, 0); got != (color.RGBA{A: 255}) {
			t.Fatal("y1 在顯示範圍前仍被選入", got)
		}
		send(g, 0xe3000000|240<<10, 0xe4000000|255<<10|255, 0x600000ff, 240<<16|0xffff, 1|1<<16)
		g.WriteGP1(0x05000000)
		// 原 y=20 區仍符合；移除它後只剩 y=240 的不符合區。
		delete(g.sides, [4]int32{0, 20, 255, 226})
		if got := g.Screen().RGBAAt(42*n, 0); got != (color.RGBA{A: 255}) {
			t.Fatal("y1 在顯示下界仍被選入", got)
		}
	}
}

func TestSideCanvasMostRecentDraw(t *testing.T) {
	g := wideRenderer(1)
	// 兩塊寬度相同而 y1 不同的繪圖區都符合，最近一次實際寫入者優先。
	send(g, 0xe3000000|20<<10, 0xe4000000|226<<10|255, 0x600000ff, 20<<16|0xffff, 1|10<<16)
	send(g, 0xe3000000|24<<10, 0xe4000000|226<<10|255, 0x6000ff00, 24<<16|0xffff, 1|10<<16)
	for i := 0; i < 30; i++ {
		if got := g.Screen().RGBAAt(85, 48); got != (color.RGBA{G: 255, A: 255}) {
			t.Fatal("未選最近繪圖區", got)
		}
	}
	// 中央圖元也算畫進；回到舊區會依既有 E3 契約先清黑。
	send(g, 0xe3000000|20<<10, 0x600000ff, 20<<16, 1|1<<16)
	if got := g.Screen().RGBAAt(85, 48); got != (color.RGBA{A: 255}) {
		t.Fatal("中央圖元未更新選區，或 E3 未清黑", got)
	}
	// 切換狀態、零尺寸與完全被裁掉的圖元不能搶走最近繪圖區。
	send(g, 0xe3000000|24<<10, 0x6000ff00, 24<<16, 0, 0x6000ff00, 0, 1|1<<16)
	if got := g.Screen().RGBAAt(85, 48); got != (color.RGBA{A: 255}) {
		t.Fatal("未畫進的圖元影響選區", got)
	}
}

func wideRenderer(shift uint) *Renderer {
	g := newRenderer(shift)
	g.margin = 48
	g.sides = make(map[[4]int32]*sideCanvas)
	send(g, 0xe3000000, 0xe4000000|239<<10|255)
	return g
}

func TestSideCanvasSeams(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := wideRenderer(shift)
		// 橫跨左右接縫的矩形，兩側每個子像素必須連續。
		send(g, 0x600000ff, 0x0000fff8, 16|16<<16)
		send(g, 0x6000ff00, 248, 16|16<<16)
		n := 1 << shift
		s := g.Screen()
		if s.Bounds().Dx() != 342*n {
			t.Fatal(s.Bounds())
		}
		for y := 0; y < 16*n; y++ {
			for x := -8 * n; x < 8*n; x++ {
				if c := s.RGBAAt(43*n+x, y); c.R != 255 {
					t.Fatalf("左接縫 x=%d y=%d：%v", x, y, c)
				}
				if c := s.RGBAAt((43+256)*n+x, y); c.G != 255 {
					t.Fatalf("右接縫 x=%d y=%d：%v", x, y, c)
				}
			}
		}
		// 延伸像素不得環繞寫入貼圖區。
		if g.texel(1023, 0) != 0 || g.texel(256, 0) != 0 {
			t.Fatal("側邊污染 VRAM")
		}
		// 24 位元動畫、找不到顯示起點時只補黑。
		g.WriteGP1(0x08000010)
		if c := g.Screen().RGBAAt(42*n, 0); c.R != 0 || c.G != 0 {
			t.Fatal("動畫沒有補黑")
		}
		g.WriteGP1(0x08000000)
		g.WriteGP1(0x05000002)
		if c := g.Screen().RGBAAt(42*n, 0); c.R != 0 || c.G != 0 {
			t.Fatal("顯示起點失配沒有補黑")
		}
	}
}

func TestSideCanvasBufferAndFill(t *testing.T) {
	g := wideRenderer(1)
	send(g, 0x600000ff, 0xfff8, 8|8<<16)
	a := g.side
	before := slices.Clone(a.left)
	// CPU 上傳與 VRAM 複製均不碰側邊。
	send(g, 0xa0000000, 0, 0x00010002, 0x12345678, 0x80000000, 0, 2, 0x00010002)
	if !slices.Equal(before, a.left) {
		t.Fatal("傳輸改了側邊")
	}
	// 切到另一個繪圖區；舊緩衝仍可顯示。
	send(g, 0xe3000000|256<<10, 0xe4000000|495<<10|255, 0x6000ff00, 256<<16|0xfff8, 8|8<<16)
	if !slices.Equal(before, a.left) {
		t.Fatal("切換清掉舊緩衝")
	}
	if c := g.Screen().RGBAAt(42*2, 0); c.R != 255 {
		t.Fatal("未保留顯示中緩衝")
	}
	// 回到舊區後應清黑，並在第一個繪圖命令前完成。
	send(g, 0xe3000000, 0xe4000000|239<<10|255, 0x60000000, 0, 0)
	for _, v := range a.left {
		if v != 0 {
			t.Fatal("重新使用緩衝仍有殘影")
		}
	}
	// 整寬填色只改對應列，局部填色不改側邊。
	send(g, 0x020000ff, 5<<16, 256|2<<16)
	if *g.sidePixel(-1, 10) != 31 || *g.sidePixel(512, 10) != 31 || *g.sidePixel(-1, 8) != 0 {
		t.Fatal("填色列錯誤")
	}
	send(g, 0x0200ff00, 5<<16, 16|2<<16)
	if *g.sidePixel(-1, 10) != 31 {
		t.Fatal("局部填色改側邊")
	}
}

func TestSideCanvasTextureBlendAndPolygon(t *testing.T) {
	g := wideRenderer(1)
	// 15 位元貼圖仍從 VRAM 讀；原生 x=-1 在側邊，來源在 x=512。
	send(g, 0xa0000000, 512, 0x00010002, 0x001f001f, 0xe1000108)
	send(g, 0x65000000, 0xffff, 0, 2|1<<16)
	if *g.sidePixel(-2, 0) != 31 {
		t.Fatal("側邊貼圖未讀 VRAM")
	}
	// 半透明與 mask 取側邊本身的背景。
	send(g, 0xe1000020, 0x62000040, 0xffff, 1|1<<16)
	if *g.sidePixel(-2, 0) != 31 {
		t.Fatal("側邊混色錯誤")
	}
	send(g, 0xe6000001, 0x6000ff00, 0xffff, 1|1<<16, 0xe6000002, 0x60ff0000, 0xffff, 1|1<<16)
	if *g.sidePixel(-2, 0) != 0x83e0 {
		t.Fatal("側邊 mask 錯誤")
	}
	// 多邊形與線段都應跨過左接縫，負座標不被無號遮罩丟掉。
	send(g, 0xe6000000, 0x200000ff, 10<<16|0xfff8, 10<<16|8, 26<<16|0xfff8)
	if *g.sidePixel(-4, 24) != 31 {
		t.Fatal("側邊多邊形缺失")
	}
	send(g, 0x4000ff00, 30<<16|0xfff8, 30<<16|8)
	if *g.sidePixel(-4, 60) != 0x3e0 {
		t.Fatal("側邊線段缺失")
	}
}
