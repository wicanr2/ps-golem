//go:build ignore

// Rust MDEC differential oracle, 026 section11. GPL-3.0-or-later.
// Uses own Go implementation; original game data is never embedded.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/mdec"
	"github.com/wicanr2/ps-golem/machine"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

type event struct{ K, A, V uint32 }

func hashState(d *mdec.MDEC) uint64 {
	h := uint64(14695981039346656037)
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		default:
			var n uint64
			switch v.Kind() {
			case reflect.Bool:
				if v.Bool() {
					n = 1
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n = uint64(v.Int())
			default:
				n = v.Uint()
			}
			for i := 0; i < 8; i++ {
				h ^= n & 255
				h *= 1099511628211
				n >>= 8
			}
		}
	}
	walk(reflect.ValueOf(d).Elem())
	return h
}
func output(path string) (*bufio.Writer, func()) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	w := bufio.NewWriter(f)
	return w, func() {
		if e := w.Flush(); e != nil {
			panic(e)
		}
		if e := f.Close(); e != nil {
			panic(e)
		}
	}
}
func put(w io.Writer, v any) {
	if e := binary.Write(w, binary.LittleEndian, v); e != nil {
		panic(e)
	}
}
func main() {
	if len(os.Args) == 5 && os.Args[1] == "-programs" {
		programs(os.Args[2], os.Args[3], os.Args[4])
		return
	}
	if len(os.Args) != 2 {
		panic("output root required")
	}
	root := os.Args[1]
	es := []event{}
	add := func(k, a, v uint32) { es = append(es, event{k, a, v}) }
	wr := func(a, v uint32) { add(0, a, v); add(2, 4, 0) }
	tables := func(seed uint32) {
		wr(0, 0x40000001)
		for i := uint32(0); i < 32; i++ {
			v := (i*0x193071 + seed*0x1010111) ^ 0x0180ff00
			if seed == 0 {
				v = 0x01010101
			}
			wr(0, v)
		}
		wr(0, 0x60000000)
		for i := uint32(0); i < 32; i++ {
			v := (i*0x719382 + seed*0x9171111) ^ 0x5a825a82
			if seed == 0 && i >= 4 {
				v = 0
			}
			wr(0, v)
		}
	}
	for format := uint32(0); format < 4; format++ {
		for flags := uint32(0); flags < 4; flags++ {
			for seed := uint32(0); seed < 3; seed++ {
				add(5, 0, 0)
				wr(4, 0xe0000000)
				tables(seed)
				body := []uint32{0xfe00fe00}
				blocks := 12
				if format < 2 {
					blocks = 5
				}
				for b := 0; b < blocks; b++ {
					dc := uint32((b*137+int(seed)*59)&1023) | ((uint32(b)+seed)%64)<<10
					ac := uint32((b*271+17)&1023) | ((uint32(b)*11)%63)<<10
					body = append(body, dc|(ac<<16), 0xfe000000|uint32((b*99+1)&1023))
				}
				wr(0, 0x20000000|(format<<27)|(flags<<25)|uint32(len(body)))
				for i, v := range body {
					if i%3 == 0 {
						add(3, 0, v)
					} else {
						add(0, 0, v)
					}
					add(2, uint32([]int{0, 1, 127, 128, 511, 512}[i%6]), 0)
					add(1, 4, 0)
				}
				for i := 0; i < 600; i++ {
					add(2, 128, 0)
					if i%2 == 0 {
						add(4, 0, 0)
					} else {
						add(1, 0, 0)
					}
				}
				// Reset while retaining tables and stale FIFO/offset data, then reuse tables.
				wr(4, 0xe0000000)
				wr(0, 0x28000001)
				wr(0, 0xfe000000)
				for i := 0; i < 24; i++ {
					add(2, 128, 0)
					add(1, 0, 0)
				}
			}
		}
	}
	// Deliberately overflow input while output is backpressured; empty DMA reads.
	add(5, 0, 0)
	wr(4, 0x60000000)
	tables(1)
	wr(0, 0x30000050)
	for i := 0; i < 100; i++ {
		add(3, 0, 0xfe000000|uint32(i))
		if i < 4 {
			add(2, 512, 0)
		}
	}
	for i := 0; i < 512; i++ {
		add(2, 128, 0)
		add(4, 0, 0)
	}
	wr(4, 0x80000000)
	add(1, 4, 0)
	wr(0, 0xe0001234)
	add(1, 4, 0)
	add(2, 0, 0)
	w, close := output(filepath.Join(root, "rust-mdec-oracle-v1.bin"))
	w.WriteString("PSGMDC01")
	put(w, uint32(len(es)))
	d := mdec.New()
	reads := 0
	for _, x := range es {
		put(w, x)
		var value, off uint32
		switch x.K {
		case 0:
			d.Write(x.A, x.V)
		case 1:
			value = d.Read(x.A)
			reads++
		case 2:
			d.Tick(int32(x.A))
		case 3:
			d.DMAWrite(x.V)
		case 4:
			value, off = d.DMARead()
			reads++
		case 5:
			d = mdec.New()
		}
		put(w, value)
		put(w, off)
		put(w, d.Read(4))
		put(w, hashState(d))
	}
	close()
	fmt.Printf("MDEC_GO_ORACLE: %d events, %d output/status reads, full-state hashes\n", len(es), reads)
	machineEvents(root)
}
func machineEvents(root string) {
	es := []event{}
	add := func(k, a, v uint32) { es = append(es, event{k, a, v}) }
	wr := func(a, v uint32) { add(0, a, v) }
	tick := func(n uint32) { add(4, n, 0) }
	// Bus narrow writes merge, status peeks do not consume the data FIFO.
	wr(0x1f801824, 0xe0000000)
	add(2, 0x1f801825, 0x60)
	add(3, 0x1f801824, 0x6000)
	wr(0x1f801824, 0xe0000000)
	tick(4)
	wr(0x1f801820, 0x40000001)
	tick(4)
	for i := uint32(0); i < 32; i++ {
		wr(0x1000+i*4, 0x01010101)
	}
	// DMA waits for DPCR, then finishes in 32-word block, without bit28.
	wr(0x1f8010f4, 0x00830000)
	wr(0x1f801080, 0x801000)
	wr(0x1f801084, 0x10020)
	wr(0x1f801088, 0x01000201)
	wr(0x1f8010f0, 0x076543a9)
	tick(4)
	wr(0x1f801820, 0x60000000)
	tick(4)
	for i := uint32(0); i < 32; i++ {
		v := uint32(0)
		if i < 4 {
			v = 0x5a825a82
		}
		wr(0x2000+i*4, v)
	}
	wr(0x1f801080, 0x2000)
	wr(0x1f801084, 0x10020)
	wr(0x1f801088, 0x01000201)
	tick(4)
	for format := uint32(0); format < 4; format++ {
		wr(0x1f801824, 0xe0000000)
		wr(0x1f801820, 0x20000000|(format<<27)|24)
		tick(4)
		for i := uint32(0); i < 32; i++ {
			wr(0x3000+i*4, 0xfe000000|(i*37)&1023)
		}
		// More than one output block, including wrapped reverse writes.
		addr := uint32(0x4000)
		ctrl := uint32(0x01000200)
		if format == 3 {
			addr = 4
			ctrl |= 2
		}
		wr(0x1f801090, addr)
		wr(0x1f801094, 0x00040020)
		wr(0x1f801098, ctrl)
		wr(0x1f801080, 0x3000)
		wr(0x1f801084, 0x00010020)
		wr(0x1f801088, 0x01000201)
		for i := 0; i < 40; i++ {
			tick(128)
			add(1, 0x1f801824, 0)
		}
		// Data reads release output FIFO backpressure and may complete pending DMA.
		for i := 0; i < 96; i++ {
			add(1, 0x1f801820, 0)
			tick(128)
		}
	}
	// DMA0 device-to-RAM clears payload, negative stepping and BCR-zero block count.
	wr(0x1f801824, 0xe0000000)
	wr(0x1f801820, 0x40000001)
	tick(4)
	wr(0x1f801080, 4)
	wr(0x1f801084, 0x00000002)
	wr(0x1f801088, 0x01000202)
	wr(0x1f8010f0, 0)
	wr(0x1f8010f4, 0x7f830000)
	wr(0x1f801824, 0x80000000)
	m, e := machine.New(make([]byte, 0x80000))
	if e != nil {
		panic(e)
	}
	m.ExactTiming = true
	w, close := output(filepath.Join(root, "rust-mdec-machine-oracle-v1.bin"))
	w.WriteString("PSGMDM01")
	put(w, uint32(len(es)))
	for _, x := range es {
		put(w, x)
		var v uint32
		switch x.K {
		case 0:
			m.Write32(x.A, x.V)
		case 1:
			v = m.Read32(x.A)
		case 2:
			m.Write8(x.A, x.V)
		case 3:
			m.Write16(x.A, x.V)
		case 4:
			m.Run(uint64(x.A))
		}
		put(w, v)
		put(w, m.Cycles())
		put(w, hashState(m.MDEC))
		for _, a := range []uint32{0x1f801070, 0x1f801080, 0x1f801084, 0x1f801088, 0x1f801090, 0x1f801094, 0x1f801098, 0x1f8010f0, 0x1f8010f4} {
			put(w, m.Read32(a))
		}
		h := uint64(14695981039346656037)
		for _, b := range m.PollingRAM() {
			h ^= uint64(b)
			h *= 1099511628211
		}
		put(w, h)
	}
	close()
	fmt.Printf("MDEC_MACHINE_GO: %d bus/DMA/timing events, full RAM hashes\n", len(es))
}
func programs(root, biosPath, zipPath string) {
	bios, e := os.ReadFile(biosPath)
	if e != nil {
		panic(e)
	}
	boot, e := machine.New(bios)
	if e != nil {
		panic(e)
	}
	boot.ExactTiming = true
	if e = boot.RunUntilPC(machine.ShellEntry, 400000000); e != nil {
		panic(e)
	}
	z, e := zip.OpenReader(zipPath)
	if e != nil {
		panic(e)
	}
	defer z.Close()
	dir := filepath.Join(root, "rust-mdec-public-v1")
	if e = os.Mkdir(dir, 0755); e != nil {
		panic(e)
	}
	write := func(n string, b []byte) {
		if e := os.WriteFile(filepath.Join(dir, n), b, 0644); e != nil {
			panic(e)
		}
	}
	names := []string{"4bit/4bit", "8bit/8bit", "frame/frame-15bit", "frame/frame-15bit-dma", "frame/frame-24bit", "frame/frame-24bit-dma", "movie/movie-15bit", "movie/movie-24bit", "step-by-step-log/step-by-step-log"}
	for _, name := range names {
		f, e := z.Open("mdec/" + name + ".exe")
		if e != nil {
			panic(e)
		}
		exe, e := io.ReadAll(f)
		f.Close()
		if e != nil {
			panic(e)
		}
		base := filepath.Base(name)
		write(base+".exe", exe)
		m := boot.Clone()
		mark := len(m.TTY())
		if e = m.SideloadEXE(exe); e != nil {
			panic(e)
		}
		m.Run(200000000)
		write(base+".tty", []byte(m.TTY()[mark:]))
		var b bytes.Buffer
		put(&b, m.VRAM())
		write(base+".vram", b.Bytes())
		write(base+".ram", m.PollingRAM())
		meta := []uint64{m.Cycles(), m.Field(), uint64(m.CPU.PC()), hashState(m.MDEC)}
		b.Reset()
		put(&b, meta)
		write(base+".state", b.Bytes())
		j, _ := json.Marshal(map[string]any{"cycles": meta[0], "field": meta[1], "pc": meta[2], "mdec_hash": meta[3], "scope": "Go differential, hardware gaps unchanged"})
		write(base+".json", j)
		fmt.Printf("MDEC_PUBLIC_GO: %s cycles=%d field=%d PC=%08x\n", base, m.Cycles(), m.Field(), m.CPU.PC())
	}
}
