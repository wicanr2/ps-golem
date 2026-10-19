package hd

import "testing"

func TestTexturePerspectiveExplicitOffRetainsAffine(t *testing.T) {
	g := newRenderer(1)
	v := [3]triVertex{{x: 20, y: 20, z: 200, u: 0, v: 0}, {x: 200, y: 20, z: 300, u: 127, v: 0}, {x: 20, y: 200, z: 400, u: 0, v: 127}}
	g.texturePerspective = true
	g.setPerspectiveTriangle(v, polyFlags{textured: true})
	if g.perspectiveTriangle == nil {
		t.Fatal("test did not enter perspective")
	}
	g.texturePerspective = false
	g.setPerspectiveTriangle(v, polyFlags{textured: true})
	u, w := g.perspectiveUV(60, 70, 123, 456)
	if u != 123 || w != 456 || g.perspectiveTriangle != nil {
		t.Fatal("off mode changed affine sample")
	}
}
