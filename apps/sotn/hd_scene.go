// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package sotn

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/ps-golem/machine"
	"github.com/wicanr2/ps-golem/render/hd"
)

// HDSceneRenderer is a partial game renderer. Other routines and widescreen
// keep their existing HD paths until independently supported.
type HDSceneRenderer struct {
	m     *machine.Machine
	token *machine.CallRegistration
	wide  bool
	stats HDSceneStats
}
type HDSceneStats struct {
	Attempts, Accepted, Rejected uint64
	LastReason                   string
}

func AttachHDSceneRenderer(m *machine.Machine, wide bool) (*HDSceneRenderer, error) {
	if m == nil || m.GPU == nil || !m.GPU.HasHDShadow() {
		return nil, fmt.Errorf("Go scene entry requires HD")
	}
	r := &HDSceneRenderer{m: m, wide: wide}
	token, e := m.RegisterCall(tilemapStart, func(budget uint64) bool {
		r.stats.Attempts++
		if r.wide {
			r.stats.Rejected++
			r.stats.LastReason = "widescreen uses existing path"
			return false
		}
		ok, reason := tryHDLayerCall(m, budget)
		if ok {
			r.stats.Accepted++
		} else {
			r.stats.Rejected++
			r.stats.LastReason = reason
		}
		return ok
	})
	if e != nil {
		return nil, e
	}
	r.token = token
	return r, nil
}

// Read statistics only while the emulation thread is stopped.
func (r *HDSceneRenderer) Statistics() HDSceneStats { return r.stats }
func (r *HDSceneRenderer) Close() {
	if r.token.Close() {
		r.m.ClearSprite16Bindings()
	}
}
func hdLayerScope(m *machine.Machine) string {
	if m.CPU.PC() != tilemapStart || m.CPU.GPR(31) != 0x800e3c2c || !m.CPU.CallReturnReady() {
		return "call context"
	}
	sp := m.CPU.GPR(29)
	if sp&3 != 0 || sp < 0x80000080 || sp > 0x80200000 {
		return "stack scope"
	}
	if !sceneHeader(m) {
		return "stage scope"
	}
	if sha256.Sum256(m.ReadRAM(tilemapStart, tilemapSize)) != tilemapHash {
		return "modified routine"
	}
	if fmt.Sprintf("%x", sha256.Sum256(m.ReadRAM(0x11f18, 60))) != "1c2e33c9e0ddfcd56385916112089fab3d7d343336e2514633a70260ef7d4064" ||
		fmt.Sprintf("%x", sha256.Sum256(m.ReadRAM(0x135f0, 1464))) != "bbae27e62769d757261709c0c55190c9d7ea6098ee3efd5cbb8f83c2f1636615" {
		return "helper identity"
	}
	h := sha256.New()
	var b [4]byte
	for _, a := range []uint32{0x73084, 0x73088, 0x730a4, 0x730a8, 0x730b0, 0x730b4, 0x730d8, 0x730dc, 0x730f8, 0x730fc, 0x73104} {
		binary.LittleEndian.PutUint32(b[:], m.Read32(0x80000000|a))
		h.Write(b[:])
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != "4281dfee0f618f53bf31bfb1a0e30b2b66f57eebb58ce4db0c84b0b4483958fe" {
		return "layout scope"
	}
	buffer := m.Read32(0x8006c37c)
	if buffer != 2147732232 && buffer != 2147828476 {
		return "buffer scope"
	}
	if m.Read32(0x80097940) != 0 || m.Read32(0x8009792c) != 0 || m.Read32(0x800730ac) != 0 {
		return "ordinary empty-buffer scope"
	}
	return ""
}
func hdCallScratch(m *machine.Machine) []byte {
	b := make([]byte, 1024)
	for i := range b {
		b[i] = m.Read8(0x1f800000 + uint32(i))
	}
	return b
}

func tryHDLayerCall(m *machine.Machine, budget uint64) (accepted bool, reason string) {
	committed := false
	defer func() {
		if v := recover(); v != nil {
			if committed {
				panic(v)
			}
			accepted = false
			reason = fmt.Sprint(v)
		}
	}()
	if reason := hdLayerScope(m); reason != "" {
		return false, reason
	}
	ram, scratch := m.ReadRAM(0, 2<<20), hdCallScratch(m)
	cpu := m.CallCPUState()
	fx := buildHDCallEffects(ram, scratch, cpu)
	clock := clockForHDCall(ram, cpu, m.BIU())
	costs := clock.Costs
	fx.CPU.Now, fx.CPU.Cost, fx.CPU.MultiplyReady, fx.CPU.Cache = clock.Now, clock.Cost, clock.MultiplyReady, clock.Cache
	var precision *machine.CallPrecisionDelta
	if m.CallPrecisionEnabled() {
		image := m.CallPrecisionAt(hdCallPrecisionIndices(ram, fx.Plan, false))
		image = buildHDCallPrecision(ram, scratch, cpu, image, fx, clock, m.BIU(), costs)
		precision = &machine.CallPrecisionDelta{Image: image, Clear: hdCallPrecisionClear(ram, fx.Plan, cpu)}
	}

	buffer := hdCallRAM(ram).u32(0x8006c37c)
	jobs := make([]machine.Sprite16Binding, 0, len(fx.Plan.Tiles))
	for i, t := range fx.Plan.Tiles {
		a := (buffer + 0x117f4 + uint32(i)*16 + 4) & 0x1fffffff
		b := hdCallRAM(fx.RAM)
		jobs = append(jobs, machine.Sprite16Binding{Address: a, Words: [3]uint32{b.u32(a), b.u32(a + 4), b.u32(a + 8)}, Sprite: hd.Sprite16{X: int32(t.X), Y: int32(t.Y), U: uint8(t.U), V: uint8(t.V), CLUT: uint16(t.CLUT), Page: uint32(t.Page + 8)}})
	}
	bindings, e := m.PrepareSprite16Bindings(jobs, 2*640)
	if e != nil {
		return false, e.Error()
	}
	e = m.CommitCall(costs, fx.RAM, fx.Scratch, fx.CPU, budget, precision)
	if e != nil {
		return false, e.Error()
	}
	committed = true
	bindings.Install()
	return true, ""
}
