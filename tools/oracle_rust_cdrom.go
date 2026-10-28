//go:build ignore

// Own Rust CD migration oracle, 026 section 9. Synthetic disc only.
// Controller source is internal/cdrom at f00311b; inherited Beetle attribution there.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/cdrom"
	"github.com/wicanr2/ps-golem/internal/disc"
	"os"
	"path/filepath"
	"reflect"
)

type event struct{ k, a, v uint32 }

func fixture(dir string) string {
	b := make([]byte, 600*2352)
	for s := 0; s < 600; s++ {
		q := b[s*2352 : (s+1)*2352]
		for i := range q {
			q[i] = byte(s*19 + i*7 + i/11)
		}
		abs := s + 150
		bcd := func(v int) byte { return byte(v/10<<4 | v%10) }
		q[12] = bcd(abs / 4500)
		q[13] = bcd(abs / 75 % 60)
		q[14] = bcd(abs % 75)
		q[15] = 2
		q[16] = 3
		q[17] = 4
		q[18] = 8
		q[19] = 0
		if s >= 80 && s < 100 {
			q[18] = 0x64
			q[19] = []byte{0, 1, 4, 5, 0x10, 0x11, 0x14, 0x15}[(s-80)%8]
			for g := 0; g < 18; g++ {
				off := 24 + g*128
				for i := 0; i < 16; i++ {
					q[off+i] = byte((i%4)<<4 | 12)
				}
				if s%3 == 0 {
					q[off+4] ^= 1
				}
			}
		}
	}
	data := func(l int) []byte { return b[l*2352+24 : l*2352+24+2048] }
	rec := func(l, size uint32, name string, flags byte) []byte {
		n := 33 + len(name)
		if n%2 != 0 {
			n++
		}
		r := make([]byte, n)
		r[0] = byte(n)
		binary.LittleEndian.PutUint32(r[2:], l)
		binary.BigEndian.PutUint32(r[6:], l)
		binary.LittleEndian.PutUint32(r[10:], size)
		binary.BigEndian.PutUint32(r[14:], size)
		r[25] = flags
		r[28] = 1
		r[31] = 1
		r[32] = byte(len(name))
		copy(r[33:], name)
		return r
	}
	p := data(16)
	clear(p)
	p[0] = 1
	copy(p[1:], "CD001")
	p[6] = 1
	copy(p[156:], rec(20, 2048, "\x00", 2))
	root := data(20)
	clear(root)
	cnf := []byte("BOOT = cdrom:SLPM_000.00;1\r\n")
	off := 0
	for _, r := range [][]byte{rec(21, uint32(len(cnf)), "SYSTEM.CNF;1", 0), rec(22, 4096, "SUB", 2)} {
		copy(root[off:], r)
		off += len(r)
	}
	copy(data(21), cnf)
	clear(data(22))
	copy(data(22), rec(24, 3000, "FILE.DAT;1", 0))
	bin := filepath.Join(dir, "rust-cdrom-fixture-v1.bin")
	if e := os.WriteFile(bin, b, 0644); e != nil {
		panic(e)
	}
	cue := filepath.Join(dir, "rust-cdrom-fixture-v1.cue")
	c := []byte("FILE \"missing\xb4c.BIN\" BINARY\r\n TRACK 01 MODE2/2352\r\n INDEX 01 00:00:00\r\n TRACK 02 AUDIO\r\n INDEX 00 00:05:00\r\n INDEX 01 00:05:25\r\n")
	if e := os.WriteFile(cue, c, 0644); e != nil {
		panic(e)
	}
	return cue
}
func hashState(c *cdrom.CDROM) uint64 {
	h := uint64(14695981039346656037)
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).Name != "disc" {
					walk(v.Field(i))
				}
			}
		case reflect.Array, reflect.Slice:
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
	walk(reflect.ValueOf(c).Elem())
	return h
}
func main() {
	if len(os.Args) != 2 {
		panic("output directory required")
	}
	dir := os.Args[1]
	fixtures := filepath.Join(dir, "rust-cdrom-fixtures-v1")
	if e := os.Mkdir(fixtures, 0755); e != nil {
		panic(e)
	}
	cue := fixture(fixtures)
	d, e := disc.Open(cue)
	if e != nil {
		panic(e)
	}
	defer d.Close()
	events := []event{}
	add := func(k, a, v uint32) { events = append(events, event{k, a, v}) }
	tick := func(n uint32) { add(2, n, 0) }
	ack := func() {
		add(0, 0, 1)
		add(1, 3, 0)
		for i := 0; i < 20; i++ {
			add(1, 1, 0)
		}
		add(0, 3, 0x5f)
		add(0, 0, 0)
		tick(2001)
	}
	cmd := func(cmd byte, args ...byte) {
		add(0, 0, 0)
		for _, v := range args {
			add(0, 2, uint32(v))
		}
		add(0, 1, uint32(cmd))
		for _, n := range []uint32{1, 1814, 12000, 8500, 2000, 100000} {
			tick(n)
			add(1, 0, 0)
		}
		ack()
	}
	add(0, 0, 1)
	add(0, 2, 31)
	add(0, 0, 0)
	cmd(0)
	cmd(0x19, 0x20)
	cmd(0x0e)
	cmd(0x02, 0xff, 0, 0)
	cmd(0x1a)
	add(5, 1, 0)
	tick(33868800)
	ack()
	for _, c := range []byte{1, 7, 8, 9, 10, 11, 12, 15, 16, 17, 19, 26, 28, 30} {
		cmd(c)
		tick(35000000)
		ack()
	}
	for _, sub := range []byte{0, 4, 5, 0x10, 0x1a, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0xff} {
		cmd(25, sub)
	}
	cmd(13, 3, 4)
	cmd(18, 1)
	tick(100000)
	ack()
	cmd(20, 0)
	cmd(20, 2)
	cmd(20, 0x99)
	cmd(29, 0, 0)
	for _, mode := range []byte{0, 0x20, 0x10, 0x30, 0x80, 0x48} {
		cmd(14, mode)
		cmd(2, 0, 2, 0x75)
		cmd(2, 0, 3, 0x05)
		cmd(21)
		tick(5000000)
		ack()
		cmd(6)
		tick(5000000)
		ack()
		cmd(16)
		cmd(17)
		add(0, 0, 0)
		add(0, 3, 128)
		for i := 0; i < 1100; i++ {
			add(3, 0, 0)
		}
		add(0, 3, 0)
		add(0, 3, 128)
		for i := 0; i < 8; i++ {
			add(1, 2, 0)
		}
		add(0, 3, 64)
		cmd(9)
		tick(5000000)
		ack()
	}
	cmd(14, 0x48)
	cmd(2, 0, 3, 5)
	cmd(0x1b)
	for i := 0; i < 28; i++ {
		tick(451584)
		ack()
		for j := 0; j < 7; j++ {
			add(4, 100, 0)
		}
	}
	cmd(9)
	tick(5000000)
	ack()
	cmd(14, 7)
	cmd(3, 2)
	tick(5000000)
	ack()
	for i := 0; i < 8; i++ {
		tick(451584)
		for j := 0; j < 6; j++ {
			add(4, 100, 0)
		}
	}
	cmd(4)
	tick(5000000)
	ack()
	cmd(5)
	tick(5000000)
	ack()
	cmd(22)
	tick(5000000)
	ack()
	cmd(8)
	tick(100000)
	ack()
	// FIFO wrap, mid-command overwrite, reset, removal and reload.
	for i := 0; i < 40; i++ {
		add(0, 2, uint32(i))
	}
	cmd(1)
	add(0, 1, 0x19)
	tick(13815)
	add(0, 1, 1)
	tick(50000)
	ack()
	add(0, 0, 1)
	add(0, 3, 128)
	add(5, 0, 0)
	cmd(6)
	add(5, 1, 0)
	tick(33868800)
	ack()
	f, e := os.OpenFile(filepath.Join(dir, "rust-cdrom-oracle-v1.bin"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
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
	w.WriteString("PSGCDR01")
	put(uint32(len(events)))
	c := cdrom.New()
	for _, x := range events {
		put(x.k)
		put(x.a)
		put(x.v)
		var result uint64
		switch x.k {
		case 0:
			c.Write(x.a, byte(x.v))
		case 1:
			result = uint64(c.Read(x.a))
		case 2:
			c.Tick(int64(x.a), nil)
		case 3:
			result = uint64(c.DMARead())
		case 4:
			result = 14695981039346656037
			for i := uint32(0); i < x.a; i++ {
				l, r := c.CDAudioSample()
				for _, v := range []int32{l, r} {
					for j := 0; j < 4; j++ {
						result ^= uint64(byte(v))
						result *= 1099511628211
						v >>= 8
					}
				}
			}
		case 5:
			if x.a == 0 {
				c.Insert(nil)
			} else {
				c.Insert(d)
			}
		}
		put(result)
		put(uint64(c.NextEvent()))
		put(hashState(c))
	}
	if e := w.Flush(); e != nil {
		panic(e)
	}
	fmt.Printf("CDROM_GO_ORACLE: %d events, synthetic disc %s, region %s\n", len(events), cue, d.Region())
	// Independent full raw-sector and Q records, including pregap and lead-out.
	f2, e := os.OpenFile(filepath.Join(dir, "rust-disc-oracle-v1.bin"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	defer f2.Close()
	for _, l := range []int32{-151, -150, -1, 0, 16, 20, 21, 24, 79, 80, 100, 374, 375, 399, 400, 599, 600, 650} {
		binary.Write(f2, binary.LittleEndian, l)
		var raw [2352]byte
		d.ReadSector(l, &raw)
		f2.Write(raw[:])
		q := d.SubQ(l)
		f2.Write(q[:])
	}
	file, e := d.ReadFile("sub/FILE.dat;1")
	if e != nil {
		panic(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "rust-disc-file-v1.bin"), file, 0644); e != nil {
		panic(e)
	}
}
