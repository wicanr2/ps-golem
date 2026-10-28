//go:build ignore

// Native Rust input/normal-player oracle, 026 section12. GPL-3.0-or-later.
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func put(w io.Writer, v any) {
	if e := binary.Write(w, binary.LittleEndian, v); e != nil {
		panic(e)
	}
}
func read(p string) []byte {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	return b
}
func main() {
	if len(os.Args) == 7 && os.Args[1] == "-normal" {
		normal(os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
		return
	}
	if len(os.Args) != 2 {
		panic("output root required")
	}
	root := os.Args[1]
	cases := []string{"", "  \n", "0:START", "3:RIGHT+CROSS:0", "20:LEFT:100,0:RIGHT:40,20:UP:7", "1:-:5", "2:", "3:down+l1", "3:ſtart", "1:UP:2:ignored", "2:UP,,3:LEFT", "-1:UP", "+1:UP", "1 :UP", "1: UP", "1:UP :2", "1:UP: 2", "1:UP:+2", "1:UP:-1", "1:UP:", "1:UP+", "1:+UP", "1:UP+NO", "1:START:0", "18446744073709551615:UP:1", "18446744073709551614:RIGHT:5", "1:UP:18446744073709551615", "18446744073709551616:UP", "1:UP:18446744073709551616", "1:UP:0005", " 2:UP:4,1:DOWN:3 ", "1:up+UP:7", "0x10:UP", "0:UP:1", "0:UP:0", "0:UP:2,1:DOWN:4"}
	cases = append(cases, string(read("apps/sotn/testdata/m4-first-save.press")))
	names := []string{"SELECT", "START", "UP", "RIGHT", "DOWN", "LEFT", "L1", "L2", "R1", "R2", "TRIANGLE", "CIRCLE", "CROSS", "SQUARE"}
	for i, n := range names {
		cases = append(cases, fmt.Sprintf("%d:%s:7", i, n))
	}
	f, e := os.OpenFile(filepath.Join(root, "rust-input-oracle-v1.bin"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	text := func(s string) { put(w, uint32(len(s))); w.WriteString(s) }
	w.WriteString("PSGINP01")
	put(w, uint32(len(cases)))
	queries := 0
	valid := 0
	for _, s := range cases {
		text(s)
		ps, e := padscript.Parse(s)
		if e != nil {
			put(w, uint32(0))
			continue
		}
		put(w, uint32(1))
		valid++
		put(w, uint32(len(ps)))
		fields := map[uint64]bool{0: true, 1: true, 2: true, 5: true, 100: true, 4500: true, 4900: true, 12000: true, ^uint64(0): true}
		for _, p := range ps {
			put(w, p.At)
			put(w, p.Dur)
			put(w, p.Buttons)
			fields[p.At] = true
			fields[p.At-1] = true
			fields[p.At+p.Dur] = true
			fields[p.At+p.Dur-1] = true
		}
		fs := []uint64{}
		for field := range fields {
			fs = append(fs, field)
		}
		sort.Slice(fs, func(i, j int) bool { return fs[i] < fs[j] })
		put(w, uint32(len(fs)))
		for _, field := range fs {
			put(w, field)
			put(w, padscript.At(ps, field))
			queries++
		}
	}
	put(w, uint32(1024))
	for i := uint16(0); i < 1024; i++ {
		mask := i*193 + 0x5513
		put(w, mask)
		text(padscript.FormatButtons(mask))
	}
	if e = w.Flush(); e != nil {
		panic(e)
	}
	fmt.Printf("GO_INPUT: %d parser cases, %d accepted, %d field boundaries, 1024 masks; complete M4 included\n", len(cases), valid, queries)
}
func normal(bios, cue, prefix, scriptPath, fieldText string) {
	target, e := strconv.ParseUint(fieldText, 10, 64)
	if e != nil || target == 0 || target > 20000 {
		panic("field bound")
	}
	script, e := padscript.Parse(string(read(scriptPath)))
	if e != nil {
		panic(e)
	}
	m, e := machine.New(read(bios))
	if e != nil {
		panic(e)
	}
	m.ExactTiming = true
	d, e := disc.Open(cue)
	if e != nil {
		panic(e)
	}
	defer d.Close()
	m.InsertDisc(d)
	field := m.Field()
	m.Pad1.Buttons = padscript.At(script, field)
	samples := []int16{}
	for m.Field() < target && m.Cycles() < target*1000000 {
		m.Step()
		if m.Field() != field {
			samples = m.TakeAudio(samples)
			field = m.Field()
			m.Pad1.Buttons = padscript.At(script, field)
			if field%600 == 0 {
				fmt.Printf("GO_PLAYER_PROGRESS: field%d cycles%d\n", field, m.Cycles())
			}
		}
	}
	if m.Field() != target {
		panic("normal player field deadline")
	}
	write := func(ext string, b []byte) {
		if e := os.WriteFile(prefix+ext, b, 0644); e != nil {
			panic(e)
		}
	}
	write(".tty", []byte(m.TTY()))
	write(".ram", m.PollingRAM())
	var b bytes.Buffer
	put(&b, m.VRAM())
	write(".vram", b.Bytes())
	write(".mcd", m.Card1.Data[:])
	img := m.GPU.Screen()
	write(fmt.Sprintf("-%dx%d.rgba", img.Bounds().Dx(), img.Bounds().Dy()), img.Pix)
	b.Reset()
	b.WriteString("RIFF")
	put(&b, uint32(len(samples)*2+36))
	b.WriteString("WAVEfmt ")
	put(&b, uint32(16))
	put(&b, uint16(1))
	put(&b, uint16(2))
	put(&b, uint32(44100))
	put(&b, uint32(176400))
	put(&b, uint16(4))
	put(&b, uint16(16))
	b.WriteString("data")
	put(&b, uint32(len(samples)*2))
	put(&b, samples)
	write(".wav", b.Bytes())
	j, e := json.Marshal(map[string]any{"field": m.Field(), "cycles": m.Cycles(), "pc": m.CPU.PC(), "card_writes": m.Card1.Writes, "card_dirty": m.Card1.Dirty, "stereo_frames": len(samples) / 2, "scope": "normal exact-timing Go, full M4 script through bounded field; not first save"})
	if e != nil {
		panic(e)
	}
	write(".json", j)
	fmt.Printf("GO_PLAYER: %s\n", j)
}
