package hd

import (
	"slices"
	"testing"
)

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
