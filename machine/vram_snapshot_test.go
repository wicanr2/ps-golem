package machine

import "testing"

// No BIOS execution is needed for this observation-ownership contract.
func TestVRAMSnapshotOwnership(t *testing.T) {
	m, err := New(make([]byte, 512*1024))
	if err != nil {
		t.Fatal(err)
	}
	upload := func(words ...uint32) {
		for _, word := range words {
			m.GPU.WriteGP0(word)
		}
	}
	upload(0xa0000000, 0, 0x00010004, 0x00008000, 0xffff001f)
	before := m.Digest()
	first := m.VRAM()
	want := []uint16{0x8000, 0, 0x001f, 0xffff}
	if len(first) != 1024*512 {
		t.Fatalf("snapshot length %d", len(first))
	}
	for i, word := range want {
		if first[i] != word {
			t.Fatalf("word%d=%04x want%04x", i, first[i], word)
		}
	}
	first[0] = 0x7fff
	if m.Digest() != before {
		t.Fatal("changing an observation mutated the machine")
	}
	second := m.VRAM()
	for i, word := range want {
		if second[i] != word {
			t.Fatalf("snapshot alias at%d", i)
		}
	}
	upload(0xa0000000, 0, 0x00010004, 0x83e00001, 0x7c007fff)
	if first[0] != 0x7fff || first[1] != 0 || first[2] != 0x001f || first[3] != 0xffff {
		t.Fatal("a retained snapshot followed later commands")
	}
	third := m.VRAM()
	for i, word := range []uint16{1, 0x83e0, 0x7fff, 0x7c00} {
		if third[i] != word {
			t.Fatalf("new snapshot missing command at%d", i)
		}
	}
	if second[0] != 0x8000 {
		t.Fatal("independent snapshots share storage")
	}
	if m.Digest() == before {
		t.Fatal("new GPU words missing from digest")
	}
}
