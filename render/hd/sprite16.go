// Contract: docs/spec/025-hd-primary-native-demand.md §12.17.
package hd

// Sprite16 owns the value data of a raw, opaque 16x16 PS1 sprite.
type Sprite16 struct {
	X, Y int32
	U, V uint8
	CLUT uint16
	Page uint32
}

func (s *Shadow) Sprite16(j Sprite16) {
	s.flush()
	s.enqueue(&batch{read: func(g *Renderer) {
		if g.Trace != nil || g.bufLen != 0 || g.inCmd != inNone || g.texMode != 0 || g.spriteFlip != 0 ||
			g.texPageX != (j.Page&15)*64 || g.texPageY != (j.Page&16)*16 {
			g.WriteGP0(0x7d000000)
			g.WriteGP0(uint32(uint16(j.X)) | uint32(uint16(j.Y))<<16)
			g.WriteGP0(uint32(j.U) | uint32(j.V)<<8 | uint32(j.CLUT)<<16)
			return
		}
		g.prepareSides()
		g.updateCLUT(uint32(j.CLUT), 0)
		x, y := signExtend11(uint32(j.X)), signExtend11(uint32(j.Y))
		x = signExtend11(uint32(x + g.offsX))
		y = signExtend11(uint32(y + g.offsY))
		g.drawSprite(x, y, 16, 16, j.U, j.V, 0, true, false, blendOpaque)
	}})
}
