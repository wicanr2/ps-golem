package hdgpu_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/wicanr2/ps-golem/internal/pgxp"
	"github.com/wicanr2/ps-golem/render/hd"
	"github.com/wicanr2/ps-golem/render/hdgpu"
)

func xy(x, y uint32) uint32 { return x | y<<16 }
func uv(x, y uint32) uint32 { return x | y<<8 }
func send(s *hd.Shadow, w ...uint32) {
	for _, v := range w {
		s.GP0(v)
	}
}
func upload(s *hd.Shadow, x, y, w, h uint32, seed uint32) {
	send(s, 0xa0000000, xy(x, y), xy(w, h))
	for i := uint32(0); i < w*h; i += 2 {
		a := uint16((i*seed + seed*3) & 0xffff)
		b := uint16(((i+1)*seed + seed*3) & 0xffff)
		send(s, uint32(a)|uint32(b)<<16)
	}
}

func replayScene(s *hd.Shadow, mode int, wide bool, tm uint32) {
	s.GP1(0x03000000)
	s.GP1(0x08000000)
	s.GP1(0x07000000 | 0x10 | 0x100<<10)
	if wide {
		s.SetSideMargin(48)
	}
	clip := uint32(0)
	if wide {
		clip = 32
	}
	send(s, 0xe3000000|clip, 0xe4000000|319|239<<10)
	page := uint32(8|16) | (tm << 7)
	if mode >= 0 {
		page |= uint32(mode) << 5
	}
	send(s, 0xe1000000|page|1<<9)
	upload(s, 512, 256, 64, 64, 733)
	upload(s, 0, 480, 256, 1, 1193)
	clut := uint32(480 << 6)
	// 真正帶有HD色彩子像素的底圖，之後精靈半透明須以原生左上角取底色。
	send(s, 0x30d31c7e, xy(20, 18), 0x0084e912, xy(220, 33), 0x00bd21f8, xy(43, 198))
	for i := uint32(0); i < 4; i++ {
		x, y := uint32(30+i*23), uint32(32+i*17)
		cmd := uint32(0x2c)
		if mode >= 0 {
			cmd |= 2
		}
		send(s, cmd<<24|0x70a1b3, xy(x, y), clut<<16, xy(x+43, y+2), uint32(31)|(page<<16), xy(x+2, y+39), 31<<8, xy(x+45, y+41), 31|31<<8)
		cmd = 0x64
		if mode >= 0 {
			cmd |= 2
		}
		send(s, cmd<<24|0x909170, xy(x+8, y+9), uint32(7)|uint32(9)<<8|clut<<16, xy(17, 13))
		send(s, 0x62000000|0xd02058, xy(x+4, y+7), xy(13, 17))
	}
	// PGXP附件的精確座標只影響HD，仍應由同一個邊界演算法產生覆蓋。
	send(s, 0x30808080)
	w := xy(175, 150)
	s.GP0Precision(w, pgxp.Value{X: 175.375, Y: 150.625, Raw: w, Flags: pgxp.ValidXY})
	send(s, 0x00e09020, xy(241, 157), 0x0040a0d0, xy(180, 211))
	// mask、VRAM複製與側邊畫布保守CPU回退，再回到GPU。
	send(s, 0xe6000001, 0x60112233, xy(70, 55), xy(13, 17), 0xe6000002, 0x60224488, xy(72, 58), xy(27, 19), 0xe6000000)
	send(s, 0x80000000, xy(40, 35), xy(250, 140), xy(37, 29))
	send(s, 0xe1000000|page|1<<12|1<<13, 0x66808080, xy(105, 132), clut<<16|7|9<<8, xy(21, 15))
	if wide {
		send(s, 0x60808080, xy(24, 73), xy(22, 18))
	}
	// 回讀先前GPU寫入區域作貼圖；自我貼圖保留CPU快取及順序。
	send(s, 0xe1000100, 0x2dffffff, xy(70, 70), uv(20, 20), xy(108, 72), uv(42, 20)|0x100<<16, xy(71, 104), uv(20, 42), xy(109, 106), uv(42, 42))
	// 再次進入GPU，驗證CPU髒區上傳與讀寫回合。
	send(s, 0xe1000000|page, 0x2dffffff, xy(170, 30), clut<<16, xy(202, 30), 31|page<<16, xy(170, 62), 31<<8, xy(202, 62), 31|31<<8)
	// GPU新寫區的來源／目的重疊複製，必須先回讀並保留原CPU推進順序。
	send(s, 0x80000000, xy(171, 31), xy(174, 34), xy(23, 23))
	s.Field(true)
}

func TestGPUCommandReplay(t *testing.T) {
	for _, shift := range []uint{0, 1, 2} {
		for mode := -1; mode < 4; mode++ {
			for _, wide := range []bool{false, true} {
				t.Run(fmt.Sprintf("N%d/mode%d/wide%v", 1<<shift, mode, wide), func(t *testing.T) {
					d, err := hdgpu.New(shift)
					if err != nil {
						t.Fatal(err)
					}
					cpu, gpu := hd.NewShadow(shift), hd.NewShadowWithDevice(shift, nil, nil, d)
					defer cpu.Close()
					defer gpu.Close()
					tm := uint32(mode+1) % 3
					replayScene(cpu, mode, wide, tm)
					replayScene(gpu, mode, wide, tm)
					a, b := cpu.VRAMCopy(), gpu.VRAMCopy()
					if !slices.Equal(a, b) {
						for i, v := range a {
							if v != b[i] {
								t.Fatalf("VRAM(%d,%d): CPU=%04x GPU=%04x", i%(1024<<shift), i/(1024<<shift), v, b[i])
							}
						}
					}
					ca, cb := cpu.Screen(), gpu.Screen()
					if ca.Rect != cb.Rect || !slices.Equal(ca.Pix, cb.Pix) {
						t.Fatal("display pixels differ")
					}
					stats := gpu.RasterStatistics()
					if stats.Triangles == 0 || stats.Sprites == 0 || stats.Fallbacks == 0 {
						t.Fatalf("missing hardware/fallback coverage: %+v", stats)
					}
				})
			}
		}
	}
}
