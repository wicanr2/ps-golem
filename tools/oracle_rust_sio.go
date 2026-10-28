//go:build ignore

// Native SIO/card migration oracle, 026 §8. Synthetic packets only.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/sio"
	"os"
	"reflect"
)

type event struct{ kind, a, v uint32 }

func main() {
	if len(os.Args) != 2 {
		panic("output tape path required")
	}
	events := []event{}
	add := func(k, a, v uint32) { events = append(events, event{k, a, v}) }
	setup := func(port uint32) { add(0, 10, 0); add(0, 8, 0); add(0, 14, 0x88); add(0, 10, 0x1003|port<<13) }
	send := func(value byte) {
		add(1, 0, 0)
		add(0, 0, uint32(value))
		for i := 0; i < 8; i++ {
			add(2, 0x88, 0)
		}
		add(1, 4, 0)
		add(1, 0, 0)
		for _, n := range []uint32{32, 32, 32, 160, 32} {
			add(2, n, 0)
			add(1, 4, 0)
		}
	}
	packet := func(bytes []byte) {
		for _, b := range bytes {
			send(b)
		}
	}
	for port := uint32(0); port < 2; port++ {
		for _, buttons := range []uint32{0, 0x1234, 0xffff} {
			setup(port)
			add(3, port, buttons)
			packet([]byte{1, 0x42, 0, 0, 0})
			add(0, 10, 0x1013|port<<13)
		}
	}
	setup(0)
	packet([]byte{1, 0x43, 0, 0, 0})
	write := func(port, addr uint32, bad bool) {
		setup(port)
		packet([]byte{0x81, 'W', 0, 0, byte(addr >> 8), byte(addr)})
		x := byte(addr>>8) ^ byte(addr)
		for i := 0; i < 128; i++ {
			b := byte(i*7 + 3)
			send(b)
			x ^= b
		}
		if bad {
			x ^= 1
		}
		packet([]byte{x, 0, 0, 0})
	}
	read := func(port, addr uint32) {
		setup(port)
		packet([]byte{0x81, 'R', 0, 0, byte(addr >> 8), byte(addr), 0, 0, 0, 0})
		n := 130
		if addr >= 1024 {
			n = 8
		}
		for i := 0; i < n; i++ {
			send(0)
		}
	}
	write(0, 0x123, true)
	write(0, 0x123, false)
	add(4, 0, 0)
	write(0, 0x123, false)
	read(0, 0x123)
	write(0, 1024, false)
	read(0, 65535)
	write(1, 1023, false)
	read(1, 1023)
	add(0, 10, 64)
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
	s := sio.New()
	pads := []*sio.Gamepad{{}, {}}
	cards := []*sio.Memcard{sio.NewMemcard(), sio.NewMemcard()}
	for i := 0; i < 2; i++ {
		s.Pads[i] = pads[i]
		s.Cards[i] = cards[i]
	}
	out.WriteString("PSGSIO01")
	put(uint32(len(events)))
	names := []string{"mode", "control", "baud", "txBuf", "txPending", "txActive", "txBit", "rxBuf", "rxPending", "rxActive", "rxAvail", "rxBit", "divider", "now", "irq"}
	for _, e := range events {
		put(e.kind)
		put(e.a)
		put(e.v)
		var result uint32
		switch e.kind {
		case 0:
			s.Write(e.a, e.v)
		case 1:
			result = s.Read(e.a)
		case 2:
			s.Tick(int64(e.a))
		case 3:
			pads[e.a].Buttons = uint16(e.v)
		case 4:
			cards[e.a].Dirty = e.v != 0
		}
		put(result)
		put(uint64(s.NextEvent()))
		v := reflect.ValueOf(s).Elem()
		field := func(f reflect.Value) {
			var x uint64
			switch f.Kind() {
			case reflect.Bool:
				if f.Bool() {
					x = 1
				}
			case reflect.Int64:
				x = uint64(f.Int())
			default:
				x = f.Uint()
			}
			put(x)
		}
		for _, name := range names {
			field(v.FieldByName(name))
		}
		for _, name := range []string{"ackDelay", "ackUntil"} {
			a := v.FieldByName(name)
			for i := 0; i < 4; i++ {
				field(a.Index(i))
			}
		}
		for _, c := range cards {
			flag := uint64(0)
			if c.Dirty {
				flag = 1
			}
			put(flag)
			put(c.Writes)
		}
	}
	for _, c := range cards {
		put(c.Data)
	}
	if err := out.Flush(); err != nil {
		panic(err)
	}
	fmt.Printf("SIO_GO_ORACLE: %d register/bit/ACK events and 2 complete cards\n", len(events))
}
