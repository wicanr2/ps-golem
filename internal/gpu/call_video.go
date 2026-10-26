// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package gpu

import "github.com/wicanr2/ps-golem/render/hd"

type CallVideoState struct {
	lineClock, scanline, linesPerField uint32
	field, fieldRAMReadout, inVBlank   bool
	curYOffset, curLineReadout         uint32
	shadow                             *Shadow
	native                             *hd.NativePlane
}

func (g *GPU) BeginCallVideo() CallVideoState {
	v := CallVideoState{g.lineClock, g.scanline, g.linesPerField, g.field,
		g.fieldRAMReadout, g.inVBlank, g.curYOffset, g.curLineReadout, g.shadow, g.nativePlane}
	// No speculative Field call may reach either presentation queue.
	g.shadow, g.nativePlane = nil, nil
	return v
}

func (g *GPU) CallVideoCompatible(v CallVideoState) bool {
	return g.fieldRAMReadout == v.fieldRAMReadout && g.shadow == nil && g.nativePlane == nil
}

func (g *GPU) EndCallVideo(v CallVideoState, commit bool) {
	if !commit {
		g.lineClock, g.scanline, g.linesPerField = v.lineClock, v.scanline, v.linesPerField
		g.field, g.fieldRAMReadout, g.inVBlank = v.field, v.fieldRAMReadout, v.inVBlank
		g.curYOffset, g.curLineReadout = v.curYOffset, v.curLineReadout
	}
	g.shadow, g.nativePlane = v.shadow, v.native
}

func (g *GPU) CallContextReady() bool { return g.Trace == nil }
func (g *GPU) HasHDShadow() bool      { return g.shadow != nil }
