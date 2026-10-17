package gte

// ProjectionObserver 接收原生計算完成後的呈現資訊，不持有可寫機器狀態。
type ProjectionObserver interface {
	Projected(x, y float32, raw uint32, vertex int, last bool)
}

func (g *GTE) SetProjectionObserver(p ProjectionObserver) { g.projection = p }

// observeProjection 參照 beetle-psx-libretro mednafen/psx/gte.c 的
// pgxp_precise_z、pgxp_precise_h_div_sz、TransformXY，
// commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later。
// 重新讀取 Z 累加的輸入，不改原生的旗標、暫存器或計算順序。
func (g *GTE) observeProjection(v int, last bool) {
	row := g.matrixRow(mxRotation, 2)
	z := g.translation(cvTR, 2)
	for i := 0; i < 3; i++ {
		z = signExtend44(z + int64(row[i]*g.vector(v, i)))
	}
	h := float32(uint16(g.cr[crH]))
	fz := max(h/2, float32(float64(z)/4096))
	ratio := float32(0x1ffff) / 65536
	if fz > 0 {
		ratio = h / fz
	}
	x := float32(int32(g.cr[crOFX]))/65536 + float32(g.ir(1))*ratio
	y := float32(int32(g.cr[crOFY]))/65536 + float32(g.ir(2))*ratio
	g.projection.Projected(max(-1024, min(x, 1023)), max(-1024, min(y, 1023)), g.dr[14], v, last)
}
