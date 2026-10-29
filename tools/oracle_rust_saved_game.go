//go:build ignore

// Normal BIOS saved-card reload oracle, 026 section13. GPL-3.0-or-later.
// Uses explicit caller-owned data; no game addresses or original assets.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 7 {
		panic("BIOS CUE CARD SCRIPT OUTPUT-PREFIX FIELD required")
	}
	read := func(p string) []byte {
		b, e := os.ReadFile(p)
		if e != nil {
			panic(e)
		}
		return b
	}
	target, e := strconv.ParseUint(os.Args[6], 10, 64)
	if e != nil || target == 0 || target > 20000 {
		panic("field bound")
	}
	script, e := padscript.Parse(string(read(os.Args[4])))
	if e != nil {
		panic(e)
	}
	m, e := machine.New(read(os.Args[1]))
	if e != nil {
		panic(e)
	}
	m.ExactTiming = true
	if !m.Card1.Load(read(os.Args[3])) {
		panic("invalid original card length")
	}
	d, e := disc.Open(os.Args[2])
	if e != nil {
		panic(e)
	}
	defer d.Close()
	m.InsertDisc(d)
	prefix := os.Args[5]
	samples := []int16{}
	field := m.Field()
	m.Pad1.Buttons = padscript.At(script, field)
	for m.Field() < target && m.Cycles() < target*1000000 {
		m.Step()
		if m.Field() != field {
			samples = m.TakeAudio(samples)
			field = m.Field()
			m.Pad1.Buttons = padscript.At(script, field)
			if field%600 == 0 {
				fmt.Printf("GO_RELOAD_PROGRESS: field%d cycles%d\n", field, m.Cycles())
			}
		}
	}
	if m.Field() != target {
		panic("reload did not reach field within bound")
	}
	write := func(ext string, b []byte) {
		if e := os.WriteFile(prefix+ext, b, 0644); e != nil {
			panic(e)
		}
	}
	write(".tty", []byte(m.TTY()))
	write(".ram", m.PollingRAM())
	write(".mcd", m.Card1.Data[:])
	var out bytes.Buffer
	put := func(v any) {
		if e := binary.Write(&out, binary.LittleEndian, v); e != nil {
			panic(e)
		}
	}
	put(m.VRAM())
	write(".vram", out.Bytes())
	img := m.GPU.Screen()
	write(fmt.Sprintf("-%dx%d.rgba", img.Bounds().Dx(), img.Bounds().Dy()), img.Pix)
	out.Reset()
	out.WriteString("RIFF")
	put(uint32(len(samples)*2 + 36))
	out.WriteString("WAVEfmt ")
	put(uint32(16))
	put(uint16(1))
	put(uint16(2))
	put(uint32(44100))
	put(uint32(176400))
	put(uint16(4))
	put(uint16(16))
	out.WriteString("data")
	put(uint32(len(samples) * 2))
	put(samples)
	write(".wav", out.Bytes())
	meta, e := json.Marshal(map[string]any{"scope": "normal BIOS saved-card reload, no sideload or state injection", "field": m.Field(), "cycles": m.Cycles(), "pc": m.CPU.PC(), "card_writes": m.Card1.Writes, "card_dirty": m.Card1.Dirty, "stereo_frames": len(samples) / 2})
	if e != nil {
		panic(e)
	}
	write(".json", meta)
	fmt.Printf("GO_RELOAD: %s\n", meta)
}
