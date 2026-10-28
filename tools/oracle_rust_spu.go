//go:build ignore

// Native Rust SPU oracle, 026 section10. Synthetic data, no original game assets.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/cdrom"
	"github.com/wicanr2/ps-golem/internal/disc"
	"github.com/wicanr2/ps-golem/internal/spu"
	"github.com/wicanr2/ps-golem/machine"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

type event struct{ k, a, v uint32 }
type ramp struct{ n uint32 }

func (r *ramp) CDAudioSample() (int32, int32) {
	r.n++
	return int32(int16(uint16(r.n*1237 + 17))), int32(int16(uint16(r.n*997 + 32769)))
}

type clock struct{ s *spu.SPU }

func (c clock) Advance(n int64, a cdrom.AudioSource) int64 { return c.s.Advance(n, a) }
func hashState(s *spu.SPU) uint64 {
	h := uint64(14695981039346656037)
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				name := v.Type().Field(i).Name
				if name != "ram" && name != "buf" {
					walk(v.Field(i))
				}
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
			case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
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
	walk(reflect.ValueOf(s).Elem())
	return h
}
func main() {
	if len(os.Args) != 2 {
		panic("output root required")
	}
	root := os.Args[1]
	events := []event{}
	add := func(k, a, v uint32) { events = append(events, event{k, a, v}) }
	wr := func(a, v uint32) { add(0, a, v) }
	adv := func(n, cd uint32) { add(2, n, cd) }
	take := func() { add(5, 0, 0) }
	ram := func() { add(7, 0, 0) }
	adv(767, 0)
	adv(1, 0)
	take()
	adv(768*5000, 0)
	take()
	ram()
	add(6, 0, 0)
	// Write all registers, including auxiliary and readback-only regions.
	for a := uint32(0); a < 0x400; a += 2 {
		wr(a, (a*713+0x81ff)&65535)
		add(1, a, 0)
	}
	ram()
	add(6, 0, 0)
	wr(0x1a6, 0xffff)
	for i := uint32(0); i < 33; i++ {
		add(3, 0, 0xabc00000+i*19)
	}
	wr(0x1a6, 0xffff)
	for i := 0; i < 33; i++ {
		add(4, 0, 0)
	}
	ram()
	wr(0x1a4, 0)
	wr(0x1aa, 0xc040)
	adv(768, 0)
	wr(0x1aa, 0xc000)
	// One looped block per voice; exercise all 16 filters and reserved shifts.
	for v := uint32(0); v < 24; v++ {
		wr(0x1a6, 0x200+v*2)
		add(3, 0, 0x77770000|0x700|((v%16)<<4)|(v%16))
		for i := uint32(0); i < 3; i++ {
			add(3, 0, 0x98761234^(v*0x11111111+i*0x10203))
		}
		base := v * 16
		wr(base, 0x3fff-v*67)
		wr(base+2, 0x3fff-v*103)
		wr(base+4, 0x500+v*593)
		wr(base+6, 0x200+v*2)
		wr(base+8, 0xff)
		wr(base+10, 0x1fc0)
	}
	wr(0x180, 0x3fff)
	wr(0x182, 0x3fff)
	wr(0x1b0, 0x7fff)
	wr(0x1b2, 0x8000)
	wr(0x1aa, 0xc001)
	wr(0x188, 0xffff)
	wr(0x18a, 0xff)
	for i := 0; i < 32; i++ {
		adv(uint32(71+i*181), 1)
		take()
	}
	ram()
	for _, mode := range []uint32{0, 0x8000, 0x9000, 0xa000, 0xb000, 0xc000, 0xd000, 0xe000, 0xf000} {
		wr(0, mode|0x28)
		wr(2, mode|0x7f)
		wr(0x180, mode|0x2b)
		wr(0x182, mode|0x30)
		adv(768*32, 1)
		take()
	}
	wr(0x190, 0xfffe)
	wr(0x192, 255)
	wr(0x194, 0x5a5a)
	wr(0x196, 0xa5)
	for _, cnt := range []uint32{0xc001, 0xcf01, 0xfc01, 0xff01} {
		wr(0x1aa, cnt)
		adv(768*100, 1)
		take()
	}
	ram()
	// ADSR forms, delay key-offs and explicit loop address override.
	for _, lo := range []uint32{0x8000, 0x7f00, 0x00ff, 0x8f8f} {
		for _, hi := range []uint32{0, 0x1fc0, 0x8020, 0xffff} {
			wr(8, lo)
			wr(10, hi)
			wr(0x188, 1)
			adv(768*2, 1)
			wr(0x18c, 1)
			adv(768*48, 1)
			take()
			wr(14, 0x200)
		}
	}
	// Reverb circular addressing, saturating coefficients and channel sequencing.
	wr(0x1a2, 0xfff0)
	wr(0x184, 0x7fff)
	wr(0x186, 0x8000)
	wr(0x198, 0xffff)
	wr(0x19a, 255)
	for a := uint32(0x1c0); a < 0x200; a += 2 {
		wr(a, (a*193+0x9876)&65535)
	}
	wr(0x1aa, 0xc085)
	for i := 0; i < 40; i++ {
		adv(768*31, 1)
		take()
	}
	ram()
	wr(0x18c, 0xffff)
	wr(0x18e, 255)
	adv(768*100, 1)
	take()
	ram()
	// Deterministic register/DMA stress. No randomized host seed.
	rng := uint32(0xc0ffee)
	for i := 0; i < 256; i++ {
		rng ^= rng << 13
		rng ^= rng >> 17
		rng ^= rng << 5
		wr((rng%0x200)&^1, rng>>16)
		adv((rng%3072)+1, 1)
		if i%8 == 0 {
			take()
		}
	}
	ram()
	take()
	f, e := os.OpenFile(filepath.Join(root, "rust-spu-oracle-v1.bin"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	put := func(v any) {
		if e := binary.Write(w, binary.LittleEndian, v); e != nil {
			panic(e)
		}
	}
	w.WriteString("PSGSPU01")
	put(uint32(len(events)))
	s := spu.New()
	cd := &ramp{}
	sampleCount := 0
	checks := 0
	for _, x := range events {
		put(x.k)
		put(x.a)
		put(x.v)
		var result uint64
		var samples []int16
		switch x.k {
		case 0:
			s.Write(x.a, uint16(x.v))
		case 1:
			result = uint64(s.Read(x.a))
		case 2:
			if x.v != 0 {
				result = uint64(s.Advance(int64(x.a), cd))
			} else {
				result = uint64(s.Advance(int64(x.a), nil))
			}
		case 3:
			s.DMAWrite(x.v)
		case 4:
			result = uint64(s.DMARead())
		case 5:
			samples = s.TakeSamples(nil)
			sampleCount += len(samples) / 2
		case 6:
			s = spu.New()
		case 7:
			checks++
		}
		put(result)
		put(hashState(s))
		put(cd.n)
		put(uint32(len(samples)))
		put(samples)
		if x.k == 7 {
			ram := reflect.ValueOf(s).Elem().FieldByName("ram")
			for i := 0; i < ram.Len(); i++ {
				put(uint16(ram.Index(i).Uint()))
			}
		}
	}
	if e := w.Flush(); e != nil {
		panic(e)
	}
	fmt.Printf("SPU_GO_ORACLE: %d events, %d exact stereo frames, %d complete RAM checkpoints\n", len(events), sampleCount, checks)
	// CD/SPU chunk integration uses the existing synthetic CD fixture only.
	d, e := disc.Open(filepath.Join(root, "rust-cdrom-fixtures-v1/rust-cdrom-fixture-v1.cue"))
	if e != nil {
		panic(e)
	}
	defer d.Close()
	c := cdrom.New()
	c.Insert(d)
	s = spu.New()
	s.Write(0x180, 0x3fff)
	s.Write(0x182, 0x3fff)
	s.Write(0x1b0, 0x7fff)
	s.Write(0x1b2, 0x7fff)
	s.Write(0x1aa, 0xc001)
	f2, e := os.OpenFile(filepath.Join(root, "rust-spu-cd-oracle-v1.bin"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	defer f2.Close()
	cw := bufio.NewWriter(f2)
	write := func(v any) {
		if e := binary.Write(cw, binary.LittleEndian, v); e != nil {
			panic(e)
		}
	}
	ce := []event{}
	ce = append(ce, event{2, 33868800, 0})
	cmd := func(op byte, args ...byte) {
		ce = append(ce, event{0, 0, 1}, event{0, 3, 0x5f}, event{2, 2001, 0}, event{0, 0, 0})
		for _, v := range args {
			ce = append(ce, event{0, 2, uint32(v)})
		}
		ce = append(ce, event{0, 1, uint32(op)}, event{2, 100000, 0})
	}
	cmd(14, 0x48)
	cmd(2, 0, 3, 5)
	cmd(6)
	ce = append(ce, event{2, 5000000, 0})
	for i := 0; i < 80; i++ {
		ce = append(ce, event{2, uint32(9000 + i*97), 0})
	}
	cmd(9)
	ce = append(ce, event{2, 5000000, 0})
	cmd(14, 1)
	cmd(3, 2)
	ce = append(ce, event{2, 5000000, 0})
	for i := 0; i < 24; i++ {
		ce = append(ce, event{2, 20000, 0})
	}
	cw.WriteString("PSGSPC01")
	write(uint32(len(ce)))
	nFrames := 0
	for _, x := range ce {
		write(x.k)
		write(x.a)
		write(x.v)
		if x.k == 0 {
			c.Write(x.a, uint8(x.v))
		} else {
			c.Tick(int64(x.a), clock{s})
		}
		samples := s.TakeSamples(nil)
		nFrames += len(samples) / 2
		write(uint32(len(samples)))
		write(samples)
		write(hashState(s))
		write(uint64(c.SPUCounter()))
		write(uint64(c.NextEvent()))
		v := reflect.ValueOf(c).Elem()
		a := v.FieldByName("audio")
		for _, name := range []string{"size", "freq", "read", "resampPos", "resampPhase"} {
			write(a.FieldByName(name).Uint())
		}
	}
	if e := cw.Flush(); e != nil {
		panic(e)
	}
	fmt.Printf("SPU_CD_GO_ORACLE: %d events, %d stereo frames\n", len(ce), nFrames)
}

// Public SPU integration uses normal BIOS initialization then public EXE sideload.
// This is a developer test, not SOTN gameplay evidence.
func init() {
	if len(os.Args) == 5 && os.Args[1] == "-programs" {
		spuPrograms(os.Args[2], os.Args[3], os.Args[4])
		os.Exit(0)
	}
}
func spuPrograms(root, biosPath, zipPath string) {
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
	dir := filepath.Join(root, "rust-spu-public-v1")
	if e = os.Mkdir(dir, 0755); e != nil {
		panic(e)
	}
	read := func(name string) []byte {
		f, e := z.Open(name)
		if e != nil {
			panic(e)
		}
		defer f.Close()
		b, e := io.ReadAll(f)
		if e != nil {
			panic(e)
		}
		return b
	}
	write := func(name string, b []byte) {
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			panic(e)
		}
	}
	for _, name := range []string{"memory-transfer", "stereo", "test"} {
		exe := read("spu/" + name + "/" + name + ".exe")
		write(name+".exe", exe)
		if name == "memory-transfer" {
			write(name+".psx.log", read("spu/"+name+"/psx.log"))
		}
		m := boot.Clone()
		mark := len(m.TTY())
		m.TakeAudio(nil)
		if e = m.SideloadEXE(exe); e != nil {
			panic(e)
		}
		m.Run(100000000)
		write(name+".tty", []byte(m.TTY()[mark:]))
		samples := m.TakeAudio(nil)
		var data bytes.Buffer
		binary.Write(&data, binary.LittleEndian, samples)
		write(name+".pcm", data.Bytes())
		ram := reflect.ValueOf(m.SPU).Elem().FieldByName("ram")
		data.Reset()
		for i := 0; i < ram.Len(); i++ {
			binary.Write(&data, binary.LittleEndian, uint16(ram.Index(i).Uint()))
		}
		write(name+".spu-ram", data.Bytes())
		meta := map[string]any{"cycles": m.Cycles(), "field": m.Field(), "pc": m.CPU.PC(), "spu_hash": hashState(m.SPU), "exe_sha256": fmt.Sprintf("%x", sha256.Sum256(exe)), "stereo_frames": len(samples) / 2}
		j, e := json.MarshalIndent(meta, "", "  ")
		if e != nil {
			panic(e)
		}
		write(name+".json", j)
		fmt.Printf("SPU_PUBLIC_GO: %s cycles=%d field=%d PC=%08x, %d stereo frames\n", name, m.Cycles(), m.Field(), m.CPU.PC(), len(samples)/2)
	}
}
