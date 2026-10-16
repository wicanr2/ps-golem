package hd

import "testing"

func send(g *Renderer, words ...uint32) {
	for _, w := range words {
		g.WriteGP0(w)
	}
}

// 核對真正放大的 VRAM 操作，避免 N=1 轉送掩蓋倍率錯誤。
func TestScaledCopyAndUpload(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := newRenderer(shift)
		n := uint32(1) << shift
		send(g, 0xa0000000, 0x01ff03ff, 0x00020003, 0x8001001f, 0x03e07c00, 0x12345678)
		for dy := uint32(0); dy < n; dy++ {
			for dx := uint32(0); dx < n; dx++ {
				if g.VRAM[((511*n+dy)<<(10+shift))+1023*n+dx] != 31 {
					t.Fatal("環繞上傳未複製完整區塊")
				}
			}
		}
		// 同一原生像素中的子像素不相同，複製必須保留子像素。
		g.VRAM[0] = 0x1234
		if n > 1 {
			g.VRAM[1] = 0x4321
		}
		send(g, 0x80000000, 0, 0x00010002, 0x00010001)
		if g.VRAM[(n<<(10+shift))+2*n] != 0x1234 {
			t.Fatal("放大複製錯誤")
		}
		if n > 1 && g.VRAM[(n<<(10+shift))+2*n+1] != 0x4321 {
			t.Fatal("複製遺失子像素")
		}
		// mask 在放大後逐子像素檢查。
		g.VRAM[4*n] = 0x8000
		send(g, 0xe6000002, 0x80000000, 0, 4, 0x00010001)
		if g.VRAM[4*n] != 0x8000 {
			t.Fatal("複製忽略 mask")
		}
	}
}

func TestScreen24AndDimensions(t *testing.T) {
	for shift := uint(0); shift <= 3; shift++ {
		g := newRenderer(shift)
		g.WriteGP1(0x08000010)
		send(g, 0xa0000000, 0, 0x00010003, 0x44332211, 0x00006655)
		s := g.Screen()
		n := 1 << shift
		if s.Bounds().Dx() != 256*n || s.Bounds().Dy() != 240*n {
			t.Fatalf("畫面尺寸 %v", s.Bounds())
		}
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				c := s.RGBAAt(x, y)
				if c.R != 0x11 || c.G != 0x22 || c.B != 0x33 || c.A != 255 {
					t.Fatalf("24 位元未依位元組取色：%v", c)
				}
			}
		}
		c := s.RGBAAt(n, 0)
		if c.R != 0x44 || c.G != 0x55 || c.B != 0x66 {
			t.Fatalf("24 位元第二像素錯誤：%v", c)
		}
	}
}
