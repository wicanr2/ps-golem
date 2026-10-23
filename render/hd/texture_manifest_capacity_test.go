package hd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Self-authored tiny PNGs exercise the real loader beyond both old bounds.
func capacitySpecs(n int) []textureSpec {
	specs := make([]textureSpec, n)
	for i := range specs {
		specs[i] = packTestSpec()
		specs[i].Upload = fmt.Sprintf("%064x", i+1)
	}
	return specs
}

func TestTexturePackManifestCapacity(t *testing.T) {
	t.Run("all entries and original bytes", func(t *testing.T) {
		root := t.TempDir()
		specs := capacitySpecs(8192)
		packTestIndex(t, root, 4, []byte{0, 1, 2, 15})
		packTestManifest(t, root, specs...)
		manifest := filepath.Join(root, "manifest.json")
		png := filepath.Join(root, "index.png")
		before, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if len(before) <= 1<<20 {
			t.Fatal("fixture did not cross the old byte limit")
		}
		pngBefore, err := os.ReadFile(png)
		if err != nil {
			t.Fatal(err)
		}
		p, err := LoadTexturePack(root)
		if err != nil || p.Len() != len(specs) {
			t.Fatalf("full capacity: pack=%v error=%v", p, err)
		}
		for i, s := range specs {
			key, err := textureDigest(s.Upload)
			if err != nil {
				t.Fatal(err)
			}
			e := p.lookup(key, 4, [32]byte{})
			if e == nil || !bytes.Equal(e.indices, []byte{0, 1, 2, 15}) {
				t.Fatalf("entry %d lost its source or indices", i)
			}
		}
		for path, original := range map[string][]byte{manifest: before, png: pngBefore} {
			after, err := os.ReadFile(path)
			if err != nil || sha256.Sum256(after) != sha256.Sum256(original) {
				t.Fatalf("modified input %s: %v", path, err)
			}
		}
	})
	t.Run("entry bound before PNG", func(t *testing.T) {
		root := t.TempDir()
		packTestManifest(t, root, capacitySpecs(8193)...)
		p, err := LoadTexturePack(root)
		if p != nil || err == nil || !strings.Contains(err.Error(), "entries 超過 8192") {
			t.Fatalf("expected count rejection before missing PNG: %v", err)
		}
	})
	for _, bad := range []string{"PNG", "duplicate key"} {
		t.Run("late "+bad, func(t *testing.T) {
			root := t.TempDir()
			specs := capacitySpecs(8192)
			packTestIndex(t, root, 4, []byte{0, 1, 2, 15})
			if bad == "PNG" {
				specs[len(specs)-1].Image = "broken.png"
				if err := os.WriteFile(filepath.Join(root, "broken.png"), []byte("bad PNG"), 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				specs[len(specs)-1].Upload = specs[0].Upload
			}
			packTestManifest(t, root, specs...)
			p, err := LoadTexturePack(root)
			if p != nil || err == nil || !strings.Contains(err.Error(), "entries[8191]") {
				t.Fatalf("accepted late invalid content or returned wrong error: %v", err)
			}
			if bad == "duplicate key" && !strings.Contains(err.Error(), "重複的素材鍵") {
				t.Fatalf("did not reject the actual duplicate: %v", err)
			}
		})
	}
}

func TestTexturePackManifestByteBoundary(t *testing.T) {
	root := t.TempDir()
	packTestIndex(t, root, 4, []byte{0, 1, 2, 15})
	packTestManifest(t, root, packTestSpec())
	path := filepath.Join(root, "manifest.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, bytes.Repeat([]byte(" "), (4<<20)-len(b))...)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadTexturePack(root)
	if err != nil || p.Len() != 1 {
		t.Fatalf("exact 4MiB rejected: %v", err)
	}
	if err := os.WriteFile(path, append(b, ' '), 0644); err != nil {
		t.Fatal(err)
	}
	p, err = LoadTexturePack(root)
	if p != nil || err == nil || !strings.Contains(err.Error(), "超過 4194304 bytes") {
		t.Fatalf("expected byte bound rejection: %v", err)
	}
}
