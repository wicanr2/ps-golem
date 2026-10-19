package ebiten

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/ps-golem/machine"
)

func TestSlowConsumerPreservesAllFields(t *testing.T) {
	path := os.Getenv("PSGOLEM_BIOS")
	if path == "" {
		t.Skip("需要實際 BIOS")
	}
	bios, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("缺少實際 BIOS")
	}
	if err != nil {
		t.Fatal(err)
	}
	m, err := machine.New(bios)
	if err != nil {
		t.Fatal(err)
	}
	want := m.Clone()
	e := newEmulator(m, Config{UntilField: 120}, nil, &cardSaver{path: filepath.Join(t.TempDir(), "card.mcd")})
	// 沒有視窗消費者。只應發布首次與終點，模擬仍完整執行。
	e.start()
	<-e.done
	if e.err != nil {
		t.Fatal(e.err)
	}
	for want.Field() < 120 {
		if !want.RunUntilField(want.Field()+1, fieldCycleLimit) {
			t.Fatal("oracle field limit")
		}
	}
	if m.Field() != 120 || e.frameNo != 2 {
		t.Fatalf("field=%d publications=%d", m.Field(), e.frameNo)
	}
	if got, expected := m.Digest(), want.Digest(); got != expected {
		t.Fatalf("state changed: %s != %s", got, expected)
	}
	if e.front == nil {
		t.Fatal("final frame missing")
	}
}
