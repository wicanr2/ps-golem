package ebiten

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestControllerUIFontCoversStaticText(t *testing.T) {
	var meta struct{ Glyphs map[string]uiGlyph }
	if err := json.Unmarshal(uiFontJSON, &meta); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"controller.go", "controller_ui.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range s {
				if r >= 128 {
					if _, ok := meta.Glyphs[string(r)]; !ok {
						t.Errorf("%s: missing UI glyph %q U+%04X", name, r, r)
					}
				}
			}
			return true
		})
	}
}
