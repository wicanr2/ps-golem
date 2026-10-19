// 022-hd-texture-perspective：獨立有理插值，僅改影子貼圖取樣。
package hd

import "math"

type TexturePerspectiveStats struct{ TexturedTriangles, ValidDepthTriangles, UnequalDepthTriangles, CorrectedTriangles uint64 }
type perspectivePlane struct{ base, dx, dy float64 }
type perspectiveTriangle struct{ u, v, q perspectivePlane }

func makePerspectivePlane(vs [3]triVertex, a [3]float64) perspectivePlane {
	x0, y0 := float64(vs[0].x), float64(vs[0].y)
	x1, y1 := float64(vs[1].x)-x0, float64(vs[1].y)-y0
	x2, y2 := float64(vs[2].x)-x0, float64(vs[2].y)-y0
	den := x1*y2 - x2*y1
	dx := ((a[1]-a[0])*y2 - (a[2]-a[0])*y1) / den
	dy := (x1*(a[2]-a[0]) - x2*(a[1]-a[0])) / den
	return perspectivePlane{a[0] - dx*x0 - dy*y0, dx, dy}
}
func (g *Renderer) setPerspectiveTriangle(vs [3]triVertex, f polyFlags) {
	g.perspectiveTriangle = nil
	if !f.textured {
		return
	}
	g.perspectiveStats.TexturedTriangles++
	for _, v := range vs {
		if !(v.z > 0) || math.IsInf(v.z, 0) {
			return
		}
	}
	g.perspectiveStats.ValidDepthTriangles++
	if vs[0].z == vs[1].z && vs[1].z == vs[2].z {
		return
	}
	g.perspectiveStats.UnequalDepthTriangles++
	if !g.texturePerspective {
		return
	}
	var u, v, q [3]float64
	for i, p := range vs {
		q[i] = 1 / p.z
		u[i] = float64(p.u) * q[i]
		v[i] = float64(p.v) * q[i]
	}
	g.perspectiveTriangle = &perspectiveTriangle{makePerspectivePlane(vs, u), makePerspectivePlane(vs, v), makePerspectivePlane(vs, q)}
	g.perspectiveStats.CorrectedTriangles++
}
func (g *Renderer) perspectiveUV(x, y int32, u, v uint32) (uint32, uint32) {
	t := g.perspectiveTriangle
	if t == nil {
		return u, v
	}
	fx, fy := float64(x)+0.5, float64(y)+0.5
	at := func(p perspectivePlane) float64 { return p.base + p.dx*fx + p.dy*fy }
	q := at(t.q)
	if !(q > 0) {
		return u, v
	}
	tu, tv := at(t.u)/q, at(t.v)/q
	if math.IsNaN(tu) || math.IsNaN(tv) || math.Abs(tu) > 65536 || math.Abs(tv) > 65536 {
		return u, v
	}
	return uint32(int64(math.Floor(tu * 16777216))), uint32(int64(math.Floor(tv * 16777216)))
}
func (s *Shadow) SetTexturePerspective(on bool) {
	s.read(func(g *Renderer) { g.texturePerspective = on })
}
func (s *Shadow) TexturePerspectiveStatistics() (v TexturePerspectiveStats) {
	s.read(func(g *Renderer) { v = g.perspectiveStats })
	return
}
