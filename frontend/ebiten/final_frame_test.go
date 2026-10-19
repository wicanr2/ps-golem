package ebiten

import (
	"testing"

	eb "github.com/hajimehoshi/ebiten/v2"
)

// Ebiten 可以連續呼叫數次 Update 才 Draw，結束時不能保存前一張畫面。
func TestFinishWaitsForActualDraw(t *testing.T) {
	g := &Game{finishing: true}
	for n := 0; n < 3; n++ {
		if err := g.Update(); err != nil {
			t.Fatalf("terminated before drawing: %v", err)
		}
	}
	screen := eb.NewImage(32, 32)
	defer screen.Deallocate()
	g.Draw(screen)
	if err := g.Update(); err != eb.Termination {
		t.Fatalf("after final draw: %v", err)
	}
}
