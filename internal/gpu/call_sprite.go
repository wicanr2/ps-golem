// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package gpu

import "github.com/wicanr2/ps-golem/render/hd"

func (g *GPU) CallSpriteReady(j hd.Sprite16) bool {
	return g.Trace == nil && g.shadow != nil && g.bufLen == 0 && g.inCmd == inNone && g.texMode == 0 && g.spriteFlip == 0 &&
		g.texPageX == (j.Page&15)*64 && g.texPageY == (j.Page&16)*16
}
func (g *GPU) QueueCallSprite(j hd.Sprite16) { g.shadow.Sprite16(j) }
func (g *GPU) CallNativeWord(v uint32)       { g.writeGP0(v) }
