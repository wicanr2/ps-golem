package hd

import (
	"bytes"
	"strings"
	"testing"
)

// 019 §8.4: budgets are fixed by the caller, never read from material files.
func TestTexturePackImageBudgetRange(t *testing.T) {
	root := t.TempDir()
	packTestIndex(t, root, 4, []byte{0, 1, 2, 15})
	spec := packTestSpec()
	packTestManifest(t, root, spec)
	for _, tc := range []struct {
		name   string
		budget uint64
		valid  bool
	}{
		{"above former bound", (256 << 20) + 1, true},
		{"exact fixed bound", textureMemoryLimit, true},
		{"zero", 0, false},
		{"above fixed bound", textureMemoryLimit + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := loadTexturePackWithMemoryLimit(root, tc.budget)
			if !tc.valid {
				if p != nil || err == nil || !strings.Contains(err.Error(), "記憶體界線無效") {
					t.Fatalf("invalid budget returned pack=%v error=%v", p, err)
				}
				return
			}
			if err != nil || p.Len() != 1 {
				t.Fatalf("valid budget returned pack=%v error=%v", p, err)
			}
			key, err := textureDigest(spec.Upload)
			if err != nil {
				t.Fatal(err)
			}
			im := p.lookup(key, 4, [32]byte{})
			if im == nil || !bytes.Equal(im.indices, []byte{0, 1, 2, 15}) {
				t.Fatal("budget changed the original indices")
			}
		})
	}
}
