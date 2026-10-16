package ebiten

import (
	eb "github.com/hajimehoshi/ebiten/v2"
	"testing"
)

func TestXBRShader(t *testing.T) {
	s, err := eb.NewShader(xbrSource)
	if err != nil {
		t.Fatal(err)
	}
	s.Deallocate()
}
