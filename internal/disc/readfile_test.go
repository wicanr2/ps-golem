package disc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileHierarchyAndBounds(t *testing.T) {
	// Public ISO utility fixture only; never an original-game substitute.
	record := func(name string, lba, size uint32, flags byte) []byte {
		n := 33 + len(name)
		if n&1 != 0 {
			n++
		}
		b := make([]byte, n)
		b[0] = byte(n)
		binary.LittleEndian.PutUint32(b[2:], lba)
		binary.BigEndian.PutUint32(b[6:], lba)
		binary.LittleEndian.PutUint32(b[10:], size)
		binary.BigEndian.PutUint32(b[14:], size)
		b[25] = flags
		b[32] = byte(len(name))
		copy(b[33:], name)
		return b
	}
	makeImage := func() []byte {
		b := make([]byte, 64*SectorSize)
		put := func(lba int, data []byte) { b[lba*SectorSize+15] = 2; copy(b[lba*SectorSize+24:], data) }
		pvd := make([]byte, 2048)
		pvd[0] = 1
		copy(pvd[1:], "CD001")
		pvd[6] = 1
		copy(pvd[156:], record("\x00", 20, 2048, 2))
		put(16, pvd)
		root := append(record("ST", 21, 2048, 2), record("ROOT.BIN;1", 25, 4, 0)...)
		put(20, root)
		put(21, record("NO3", 22, 2048, 2))
		put(22, record("F_NO3.BIN;1", 24, 11, 0))
		put(24, []byte("tile source"))
		put(25, []byte("root"))
		return b
	}
	open := func(b []byte) *Disc {
		p := filepath.Join(t.TempDir(), "utility.bin")
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		f, e := os.Open(p)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { f.Close() })
		return &Disc{f: f, sectors: 64}
	}
	d := open(makeImage())
	for _, name := range []string{"ST/NO3/F_NO3.BIN", "st/no3/f_no3.bin", `ST\NO3\F_NO3.BIN;1`} {
		b, e := d.ReadFileLimit(name, 11)
		if e != nil || string(b) != "tile source" {
			t.Fatalf("%s: %q %v", name, b, e)
		}
	}
	b, e := d.ReadFile("ROOT.BIN")
	if e != nil || string(b) != "root" {
		t.Fatal("root behavior changed", e)
	}
	b[0] = 'x'
	again, e := d.ReadFile("ROOT.BIN")
	if e != nil || string(again) != "root" {
		t.Fatal("output aliases disc")
	}
	if _, e = d.ReadFileLimit("ST/NO3/F_NO3.BIN", 10); e == nil {
		t.Fatal("limit not enforced")
	}
	for _, name := range []string{"", "/ROOT.BIN", "ST//NO3", "../ROOT.BIN", "ST/./NO3", "ST/../ROOT.BIN"} {
		if _, e = d.ReadFile(name); e == nil {
			t.Fatal("invalid path accepted", name)
		}
	}
	for _, name := range []string{"missing.bin", "ROOT.BIN/child"} {
		if _, e = d.ReadFile(name); e != io.EOF {
			t.Fatal("missing/non-directory must return EOF", name, e)
		}
	}
	for _, kind := range []string{"short-record", "outside", "endian", "multi-extent"} {
		t.Run(kind, func(t *testing.T) {
			b := makeImage()
			at := 22*SectorSize + 24
			switch kind {
			case "short-record":
				b[at] = 2
			case "outside":
				binary.LittleEndian.PutUint32(b[at+2:], 99)
				binary.BigEndian.PutUint32(b[at+6:], 99)
			case "endian":
				binary.BigEndian.PutUint32(b[at+6:], 23)
			case "multi-extent":
				b[at+25] |= 0x80
			}
			if _, e := open(b).ReadFile("ST/NO3/F_NO3.BIN"); e == nil {
				t.Fatal("invalid record accepted")
			}
		})
	}
}

func TestReadOriginalSceneSources(t *testing.T) {
	root := os.Getenv("SOTN_SCENE_ORIG")
	if root == "" {
		root = "/orig"
	}
	cue := filepath.Join(root, "disc", "sotn.cue")
	if _, e := os.Stat(cue); os.IsNotExist(e) {
		t.Skip("original disc not available")
	}
	d, e := Open(cue)
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	if d.Region() != "SCEI" || d.BootFile() != "SLPM_860.23" {
		t.Fatal("original root boot selection changed")
	}
	for name, want := range map[string]string{
		"SLPM_860.23":      "a68adcd6fc8a426bebeee28383d80550037d6199cf3a41a8d4fa743ae3d61166",
		"ST/NO3/F_NO3.BIN": "a0bb78a1260eee5e3bda4bae6cfbbd0a61ddd5604905f9fd6ef833fd03319c66",
		"BIN/F_GAME.BIN":   "6f8c832dd1986aca1ed04247d3bd7977aae4ddf5cb3ac569097f6c0b1123ac74",
	} {
		b, e := d.ReadFileLimit(name, 2<<20)
		if e != nil {
			t.Fatal(name, e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != want {
			t.Fatal("original file identity mismatch", name)
		}
		b2, e := d.ReadFile(name)
		if e != nil || !bytes.Equal(b, b2) {
			t.Fatal("bounded/unbounded file differs")
		}
	}
}
