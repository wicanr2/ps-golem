//go:build ignore

// Fixed own-engine GPU video oracle, 026 §7; no BIOS/disc data.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"os"
	"reflect"
)

func main() {
	if len(os.Args) != 2 {
		panic("output path required")
	}
	f, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	put := func(v any) {
		if err := binary.Write(out, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	g := gpu.New()
	g.WriteGP0(0xa0000000)
	g.WriteGP0(0)
	g.WriteGP0(512<<16 | 1024)
	seed := uint32(0xabc123)
	for i := 0; i < 262144; i++ {
		seed = seed*1664525 + 1013904223
		g.WriteGP0(seed)
	}
	out.WriteString("PSGVID01")
	put(uint32(1024))
	seed = 0xc0ffee
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed }
	for i := 0; i < 1024; i++ {
		kind, arg := uint32(1), next()%60000
		switch i % 16 {
		case 0:
			kind, arg = 0, 0
		case 1:
			kind, arg = 0, 0x08000000|[]uint32{0, 8, 0x24, 0x2c, 0x64, 0x6c, 0x10, 0x30}[(i/16)%8]
		case 2:
			kind, arg = 0, 0x07000000|16|((240+next()%64)<<10)
		case 3:
			kind, arg = 0, 0x05000000|(next()&0x7ffff)
		case 4:
			kind, arg = 2, 0x1f000000
		case 5:
			kind, arg = 0, 0x02000000
		case 6:
			kind, arg = 0, 0x06000000|0x200|((0xa00+next()%256)<<12)
		case 7:
			kind, arg = 0, 0x04000000|next()%4
		}
		put(kind)
		put(arg)
		vb, lines := 0, 0
		if kind == 0 {
			g.WriteGP1(arg)
		} else if kind == 1 {
			vb, lines = g.TickVideo(arg)
		} else {
			g.WriteGP0(arg)
		}
		put([]uint32{uint32(vb), uint32(lines), g.Status(), g.DotDivider(), g.VideoToNextLine()})
		v := reflect.ValueOf(g).Elem()
		for _, name := range []string{"lineClock", "scanline", "linesPerField", "field", "inVBlank", "curYOffset", "curLineReadout", "fieldRAMReadout"} {
			field := v.FieldByName(name)
			if field.Kind() == reflect.Bool {
				n := uint32(0)
				if field.Bool() {
					n = 1
				}
				put(n)
			} else {
				put(uint32(field.Uint()))
			}
		}
		if i%16 == 15 {
			screen := g.Screen()
			put(uint32(screen.Bounds().Dx()))
			put(uint32(screen.Bounds().Dy()))
			h := uint64(14695981039346656037)
			for _, b := range screen.Pix {
				h = (h ^ uint64(b)) * 1099511628211
			}
			put(h)
		}
	}
	if err := out.Flush(); err != nil {
		panic(err)
	}
	fmt.Println("VIDEO_GO_ORACLE: 1024 events, 64 complete scanout hashes")
}
