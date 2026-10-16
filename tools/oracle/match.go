//go:build ignore

// match：為一張 oracle 截圖在 ps-golem 的連續截圖中找最接近的一張，逐像素比（各色只比高 5 位元）。
//
//	go run tools/oracle/match.go <oracle.png> <ours 目錄> <起 field> <迄 field> [<對照圖輸出.png>]
//
// ours 目錄是 psexe -shots 的輸出（f<field>.png），檔名的 field 為 6 位數。
// mednafen 的截圖含水平邊框（顯示寬 256 時為 280），ps-golem 只輸出顯示範圍，
// 所以在 oracle 圖內水平平移找 ps-golem 畫面的位置，只比重疊的部分。
// mednafen 固定輸出 240（交錯 480）行，ps-golem 只輸出 GP1(07h) 的垂直顯示範圍，
// 所以 oracle 最多高 maxDY 行時也上下平移。
// 輸出最接近的 field、平移量、不同的像素數與比例；指定輸出檔時存「oracle 對應區｜ours｜差異」三欄對照圖。
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func load(p string) image.Image {
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	return img
}

func same(a, b color.Color) bool {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()
	return r1>>11 == r2>>11 && g1>>11 == g2>>11 && b1>>11 == b2>>11
}

// maxDY 是 oracle 比 ps-golem 高時允許的上下平移量。
const maxDY = 4

// diff 比 oracle o 自 (dx, dy) 起與 u 等大的區域。
func diff(o image.Image, dx, dy int, u image.Image) int {
	n := 0
	ub := u.Bounds()
	for y := 0; y < ub.Dy(); y++ {
		for x := 0; x < ub.Dx(); x++ {
			if !same(o.At(o.Bounds().Min.X+dx+x, o.Bounds().Min.Y+dy+y), u.At(ub.Min.X+x, ub.Min.Y+y)) {
				n++
			}
		}
	}
	return n
}

// bestShift 回傳差異最少的平移量與差異數。
func bestShift(o, u image.Image) (int, int, int) {
	bx, by, bn := 0, 0, -1
	for dy := 0; dy+u.Bounds().Dy() <= o.Bounds().Dy(); dy++ {
		for dx := 0; dx+u.Bounds().Dx() <= o.Bounds().Dx(); dx++ {
			if n := diff(o, dx, dy, u); bn < 0 || n < bn {
				bx, by, bn = dx, dy, n
			}
		}
	}
	return bx, by, bn
}

func main() {
	o := load(os.Args[1])
	from, _ := strconv.Atoi(os.Args[3])
	to, _ := strconv.Atoi(os.Args[4])
	files, _ := filepath.Glob(filepath.Join(os.Args[2], "f*.png"))
	bestN, bestF, bestP, bestD, bestY := -1, 0, "", 0, 0
	for _, f := range files {
		v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "f"), ".png"))
		if err != nil || v < from || v > to {
			continue
		}
		u := load(f)
		if dh := o.Bounds().Dy() - u.Bounds().Dy(); dh < 0 || dh > maxDY || u.Bounds().Dx() > o.Bounds().Dx() {
			continue
		}
		if d, dy, n := bestShift(o, u); bestN < 0 || n < bestN {
			bestN, bestF, bestP, bestD, bestY = n, v, f, d, dy
		}
	}
	if bestN < 0 {
		fmt.Printf("%s：範圍內沒有同尺寸（%v）的截圖\n", filepath.Base(os.Args[1]), o.Bounds().Size())
		return
	}
	u := load(bestP)
	total := u.Bounds().Dx() * u.Bounds().Dy()
	fmt.Printf("%s：最接近 field %d，平移 %d,%d，不同 %d / %d 像素（%.2f%%）\n", filepath.Base(os.Args[1]), bestF, bestD, bestY, bestN, total, 100*float64(bestN)/float64(total))
	if len(os.Args) > 5 {
		w, h := u.Bounds().Dx(), u.Bounds().Dy()
		out := image.NewRGBA(image.Rect(0, 0, w*3+8, h))
		draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(out, image.Rect(0, 0, w, h), o, o.Bounds().Min.Add(image.Pt(bestD, bestY)), draw.Src)
		draw.Draw(out, image.Rect(w+4, 0, 2*w+4, h), u, u.Bounds().Min, draw.Src)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := color.RGBA{0, 0, 0, 255}
				if !same(o.At(o.Bounds().Min.X+bestD+x, o.Bounds().Min.Y+bestY+y), u.At(u.Bounds().Min.X+x, u.Bounds().Min.Y+y)) {
					c = color.RGBA{255, 0, 255, 255}
				}
				out.Set(2*w+8+x, y, c)
			}
		}
		f, _ := os.Create(os.Args[5])
		defer f.Close()
		_ = png.Encode(f, out)
	}
}
