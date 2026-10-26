package sotn

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/ps-golem/machine"
)

func sceneOriginal(t *testing.T, path string) []byte {
	t.Helper()
	root := os.Getenv("SOTN_SCENE_ORIG")
	if root == "" {
		root = "/orig"
	}
	b, e := os.ReadFile(filepath.Join(root, path))
	if os.IsNotExist(e) {
		t.Skip("original retained input absent: " + path)
	}
	if e != nil {
		t.Fatal(e)
	}
	return b
}

type sceneTestState struct {
	m             *machine.Machine
	w             *Widescreen
	steps, cycles int
}

func runSceneOriginal(t *testing.T, ram, vr, bios []byte, enabled bool) sceneTestState {
	t.Helper()
	m, e := machine.New(bios)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < len(ram); i += 4 {
		m.Write32(uint32(i), binary.LittleEndian.Uint32(ram[i:]))
	}
	for _, p := range widescreenWords {
		m.Write32(p.addr, p.original)
	}
	m.GPU.WriteGP0(0xa0000000)
	m.GPU.WriteGP0(0)
	m.GPU.WriteGP0(1024 | 512<<16)
	for i := 0; i < len(vr); i += 4 {
		m.GPU.WriteGP0(binary.LittleEndian.Uint32(vr[i:]))
	}
	m.Write32(0x80097940, 0)
	m.Write32(0x8009792c, 0)
	m.Write32(0x800730ac, 0)
	buffer := m.Read32(0x8006c37c)
	ot := buffer + 0x474
	for i := uint32(0); i < 512; i++ {
		next := uint32(0xffffff)
		if i < 511 {
			next = (ot + (i+1)*4) & 0xffffff
		}
		m.Write32(ot+i*4, next)
	}
	root := os.Getenv("SOTN_SCENE_ORIG")
	if root == "" {
		root = "/orig"
	}
	if _, e := os.Stat(filepath.Join(root, "disc/sotn.cue")); os.IsNotExist(e) {
		t.Skip("original cue absent")
	}
	w := &Widescreen{}
	w.SetSourceCue(filepath.Join(root, "disc/sotn.cue"))
	if e = w.Check(m); e != nil {
		t.Fatal(e)
	}
	if !w.sources.ready {
		t.Fatal("actual cue sources not validated")
	}
	if !enabled {
		m.Write32(0x800ed520, 0x27bd0048)
		m.CPU.SetBreakHandler(widescreenTrap)
	}
	m.CPU.SetGPR(29, 0x801fff00)
	m.CPU.SetGPR(31, 0x8000fff0)
	m.CPU.SetPC(0x800ecc10)
	state := sceneTestState{m: m, w: w}
	for state.steps < 500000 && m.CPU.PC() != 0x8000fff0 {
		state.cycles += m.CPU.Step()
		state.steps++
	}
	if m.CPU.PC() != 0x8000fff0 {
		t.Fatal("original function did not return")
	}
	return state
}

func TestSceneProductionOriginalPackets(t *testing.T) {
	bios := sceneOriginal(t, "bios/SCPH5500.BIN")
	rows := []struct {
		name, ram, vram, oracle string
		count, modes            uint32
	}{
		{"room26", "workplace/render-control-v1/wide-hd-v1/final-RAM.bin", "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/normal-scene-VRAM.bin", "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/mode-gap-v1.json", 48, 1},
		{"room15", "workplace/render-control-v1/scroll-view-v1/normal-jump-final-RAM.bin", "workplace/render-control-v1/scroll-view-v1/normal-jump-final-VRAM.bin", "workplace/render-control-v1/neighbor-scroll-v1/normal-scene-v1/scene-controlled-v1.json", 42, 0},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			ram, vr := sceneOriginal(t, row.ram), sceneOriginal(t, row.vram)
			var saved struct {
				Passed bool `json:"passed"`
				Probe  struct {
					Last []struct {
						Order, Page uint32
						Words       [3]uint32
					}
				} `json:"probe_stats"`
			}
			if e := json.Unmarshal(sceneOriginal(t, row.oracle), &saved); e != nil {
				t.Fatal(e)
			}
			if !saved.Passed {
				t.Fatal("retained original packet evidence not accepted")
			}
			old, candidate := runSceneOriginal(t, ram, vr, bios, false), runSceneOriginal(t, ram, vr, bios, true)
			before, after := old.m.Read32(0x80097940), candidate.m.Read32(0x80097940)
			mb, ma := old.m.Read32(0x8009792c), candidate.m.Read32(0x8009792c)
			if after-before != row.count || ma-mb != row.modes || candidate.w.sceneFrames != 1 {
				t.Fatalf("source validation or packets failed %d/%d modes%d/%d frames%d", before, after, mb, ma, candidate.w.sceneFrames)
			}
			if old.steps != candidate.steps || old.cycles != candidate.cycles {
				t.Fatal("original model cost changed")
			}
			for i := 0; i < 32; i++ {
				if old.m.CPU.GPR(i) != candidate.m.CPU.GPR(i) || old.m.CPU.COP0(i) != candidate.m.CPU.COP0(i) {
					t.Fatal("register ABI changed")
				}
			}
			expected := map[[4]uint32]int{}
			for _, p := range saved.Probe.Last {
				expected[[4]uint32{p.Order + p.Page, p.Words[0], p.Words[1], p.Words[2]}]++
			}
			buffer := old.m.Read32(0x8006c37c)
			ot := buffer + 0x474
			first, last := (buffer+0x117f4+before*16)&0x1fffff, (buffer+0x117f4+after*16)&0x1fffff
			actual := map[[4]uint32]int{}
			allowed := map[uint32]bool{}
			mark := func(a, n uint32) {
				for i := uint32(0); i < n; i++ {
					allowed[(a+i)&0x1fffff] = true
				}
			}
			mark(0x800ed520, 4)
			mark(0x80097940, 4)
			mark(0x8009792c, 4)
			mark(first, last-first)
			mark(buffer+0xc74+mb*12, (ma-mb)*12)
			for slot := uint32(0); slot < 512; slot++ {
				p := candidate.m.Read32(ot+slot*4) & 0xffffff
				seen := map[uint32]bool{}
				for p != 0xffffff && (p < (ot&0x1fffff) || p >= (ot&0x1fffff)+2048) {
					if p >= 2<<20 || seen[p] {
						t.Fatal("packet chain invalid")
					}
					seen[p] = true
					tag := candidate.m.Read32(p)
					if p >= first && p < last {
						if tag>>24 != 3 {
							t.Fatal("new sprite length")
						}
						key := [4]uint32{slot, candidate.m.Read32(p + 4), candidate.m.Read32(p + 8), candidate.m.Read32(p + 12)}
						actual[key]++
						x := int(int16(key[2]))
						if x+16 > 0 && x < 256 {
							t.Fatal("new tile in native center")
						}
					}
					p = tag & 0xffffff
				}
			}
			if len(actual) != len(expected) {
				t.Fatal("packet key count differs")
			}
			for key, count := range expected {
				if actual[key] != count {
					t.Fatalf("formal packet differs from retained original MIPS: %v", key)
				}
				slot := key[0]
				head := old.m.Read32(ot+slot*4) & 0xffffff
				if old.m.Read32(head)>>24 == 2 {
					mark(head, 4)
				} else {
					mark(ot+slot*4, 4)
				}
			}
			b0, b1 := old.m.ReadRAM(0, 2<<20), candidate.m.ReadRAM(0, 2<<20)
			for i, v := range b0 {
				if b1[i] != v && !allowed[uint32(i)] {
					t.Fatalf("undeclared RAM write at %08x", 0x80000000+i)
				}
			}
			candidate.w.Disable(candidate.m)
			if sha256.Sum256(candidate.m.ReadRAM(0xecc10, 2332)) != tilemapHash {
				t.Fatal("Disable did not restore original code")
			}
			t.Logf("formal cue-source/modes/packets match original: sprites %d->%d modes%d->%d steps%d cycles%d", before, after, mb, ma, old.steps, old.cycles)
		})
	}
}

func TestSceneProductionGuards(t *testing.T) {
	ram := sceneOriginal(t, "workplace/render-control-v1/wide-hd-v1/final-RAM.bin")
	vr := sceneOriginal(t, "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/normal-scene-VRAM.bin")
	old := runSceneOriginal(t, ram, vr, sceneOriginal(t, "bios/SCPH5500.BIN"), false)
	buffer := old.m.Read32(0x8006c37c)
	ot := buffer + 0x474
	var saved struct {
		Probe struct{ Last []struct{ Words [3]uint32 } } `json:"probe_stats"`
	}
	if e := json.Unmarshal(sceneOriginal(t, "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/mode-gap-v1.json"), &saved); e != nil {
		t.Fatal(e)
	}
	uv := saved.Probe.Last[0].Words[2]
	u, v, clut := uv&255, uv>>8&255, uv>>16
	mutateWord := func(m *machine.Machine, x, y uint32) {
		native := m.GPU.VRAMSnapshot()
		word := native[y*1024+x] ^ 1
		m.GPU.WriteGP0(0xa0000000)
		m.GPU.WriteGP0(x | y<<16)
		m.GPU.WriteGP0(1 | 1<<16)
		m.GPU.WriteGP0(uint32(word))
	}
	for _, kind := range []string{"missing-source", "tile-word", "palette-STP", "sprite-budget", "mode-budget", "end-only-chain", "backward-chain", "stage-fingerprint"} {
		t.Run(kind, func(t *testing.T) {
			m := old.m.Clone()
			w := &Widescreen{sources: old.w.sources}
			switch kind {
			case "missing-source":
				w.sources = sceneSources{}
			case "tile-word":
				mutateWord(m, 12*64+u/4, v)
			case "palette-STP":
				native := m.GPU.VRAMSnapshot()
				x, y := (clut&63)*16, clut>>6
				word := native[y*1024+x] ^ 0x8000
				m.GPU.WriteGP0(0xa0000000)
				m.GPU.WriteGP0(x | y<<16)
				m.GPU.WriteGP0(1 | 1<<16)
				m.GPU.WriteGP0(uint32(word))
			case "sprite-budget":
				m.Write32(0x80097940, 576)
			case "mode-budget":
				m.Write32(0x8009792c, 1000)
			case "end-only-chain":
				m.Write32(ot+36*4, 0xffffff)
			case "backward-chain":
				m.Write32(ot+37*4, (ot+36*4)&0xffffff)
			case "stage-fingerprint":
				m.Write32(0x80180000, m.Read32(0x80180000)^1)
			}
			before := m.ReadRAM(0, 2<<20)
			w.appendNeighborScene(m)
			if w.sceneFrames != 0 || !bytes.Equal(before, m.ReadRAM(0, 2<<20)) {
				t.Fatal("unsafe context changed game RAM")
			}
		})
	}
	w := old.w
	w.SetSourceCue("/missing-scene-source.cue")
	if w.sources.ready || w.sources.tried || w.sceneReady {
		t.Fatal("changing cue retained old scene state")
	}
}

// A CLUT coordinate outside native VRAM must fail closed before indexing it.
func TestSceneProductionCLUTRange(t *testing.T) {
	ram := sceneOriginal(t, "workplace/render-control-v1/wide-hd-v1/final-RAM.bin")
	vr := sceneOriginal(t, "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/normal-scene-VRAM.bin")
	state := runSceneOriginal(t, ram, vr, sceneOriginal(t, "bios/SCPH5500.BIN"), false)
	var saved struct {
		Probe struct{ Last []struct{ Words [3]uint32 } } `json:"probe_stats"`
	}
	if e := json.Unmarshal(sceneOriginal(t, "workplace/render-control-v1/neighbor-modes-v1/normal-scene-v1/mode-gap-v1.json"), &saved); e != nil {
		t.Fatal(e)
	}
	clut := uint16(saved.Probe.Last[0].Words[2] >> 16)
	changed := 0
	for i := uint32(0); i < 256; i++ {
		a := uint32(0x8003c104) + i*2
		if state.m.Read16(a) == clut {
			state.m.Write16(a, 0xffff)
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("original CLUT table entry missing")
	}
	before := state.m.ReadRAM(0, 2<<20)
	state.w.appendNeighborScene(state.m)
	if state.w.sceneFrames != 0 || !bytes.Equal(before, state.m.ReadRAM(0, 2<<20)) {
		t.Fatal("invalid CLUT changed game RAM")
	}
}
