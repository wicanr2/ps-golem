//go:build ignore

// Go normal-disc reference for the Rust MDEC probe, 026 section11.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/machine"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 5 {
		panic("BIOS CUE output-prefix final-field required")
	}
	bios, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	m, e := machine.New(bios)
	if e != nil {
		panic(e)
	}
	m.ExactTiming = true
	d, e := disc.Open(os.Args[2])
	if e != nil {
		panic(e)
	}
	defer d.Close()
	m.InsertDisc(d)
	prefix := os.Args[3]
	end, e := strconv.ParseUint(os.Args[4], 10, 64)
	if e != nil || end == 0 || end > 5000 {
		panic("field bound")
	}
	out, e := os.Create(prefix + ".tsv")
	if e != nil {
		panic(e)
	}
	defer out.Close()
	samples := []int16{}
	field := uint64(0)
	first, last := uint64(0), uint64(0)
	ended := false
	unique := map[uint64]bool{}
	for m.Field() < end && m.Cycles() < 4000000000 {
		m.Step()
		if m.Field() == field {
			continue
		}
		field = m.Field()
		samples = m.TakeAudio(samples)
		status := m.GPU.Status()
		movie := status&(1<<21) != 0
		hash := uint64(0)
		nonblack := 0
		if movie {
			if first == 0 {
				first = field
			}
			last = field
			f := m.GPU.Screen()
			hash = 14695981039346656037
			for _, b := range f.Pix {
				hash ^= uint64(b)
				hash *= 1099511628211
			}
			for i := 0; i < len(f.Pix); i += 4 {
				if f.Pix[i] != 0 || f.Pix[i+1] != 0 || f.Pix[i+2] != 0 {
					nonblack++
				}
			}
			unique[hash] = true
		} else if first != 0 {
			ended = true
		}
		fmt.Fprintf(out, "%d\t%d\t%08x\t%016x\t%d\n", field, m.Cycles(), status, hash, nonblack)
	}
	if m.Field() != end || first == 0 || !ended || len(unique) < 11 {
		panic("normal movie progression incomplete")
	}
	var b bytes.Buffer
	put := func(v any) {
		if e := binary.Write(&b, binary.LittleEndian, v); e != nil {
			panic(e)
		}
	}
	b.WriteString("RIFF")
	put(uint32(len(samples)*2 + 36))
	b.WriteString("WAVEfmt ")
	put(uint32(16))
	put(uint16(1))
	put(uint16(2))
	put(uint32(44100))
	put(uint32(176400))
	put(uint16(4))
	put(uint16(16))
	b.WriteString("data")
	put(uint32(len(samples) * 2))
	put(samples)
	write := func(s string, v []byte) {
		if e := os.WriteFile(prefix+s, v, 0644); e != nil {
			panic(e)
		}
	}
	write(".wav", b.Bytes())
	write(".tty", []byte(m.TTY()))
	write(".ram", m.PollingRAM())
	b.Reset()
	put(m.VRAM())
	write(".vram", b.Bytes())
	nonzero, peak := 0, 0
	for _, v := range samples {
		n := int(v)
		if n != 0 {
			nonzero++
		}
		if n < 0 {
			n = -n
		}
		if n > peak {
			peak = n
		}
	}
	j, e := json.Marshal(map[string]any{"scope": "ExactTiming Go normal reset BIOS disc boot", "cycles": m.Cycles(), "field": m.Field(), "pc": m.CPU.PC(), "first_24bit_field": first, "last_24bit_field": last, "returned_from_24bit": ended, "distinct_field_images": len(unique), "audio_frames": len(samples) / 2, "nonzero_samples": nonzero, "peak": peak, "speaker_verified": false})
	if e != nil {
		panic(e)
	}
	write(".json", j)
	fmt.Printf("GO_MDEC_NORMAL: %s\n", j)
}
