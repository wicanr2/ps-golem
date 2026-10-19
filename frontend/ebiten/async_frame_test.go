package ebiten

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/wicanr2/ps-golem/internal/gpu"
	"github.com/wicanr2/ps-golem/machine"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAsyncHDIdleAndSlowReaderPreserveNative(t *testing.T) {
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
	for _, slow := range []bool{false, true} {
		t.Run(fmt.Sprintf("slow=%t", slow), func(t *testing.T) {
			m, err := machine.New(bios)
			if err != nil {
				t.Fatal(err)
			}
			want := m.Clone()
			s := gpu.NewShadow(1)
			m.GPU.SetShadow(s)
			defer func() { m.DisablePGXP(); m.GPU.SetShadow(nil); s.Close() }()
			if err = m.EnablePGXP(); err != nil {
				t.Fatal(err)
			}
			e := newEmulator(m, Config{UntilField: 120}, nil, &cardSaver{path: filepath.Join(t.TempDir(), "card.mcd")})
			e.start()
			var readerDone chan struct{}
			if slow {
				readerDone = make(chan struct{})
				go func() {
					defer close(readerDone)
					for {
						select {
						case <-e.done:
							return
						default:
						}
						e.useFrame(func(img *image.RGBA, _ uint64) {
							if img == nil {
								return
							}
							before := sha256.Sum256(img.Pix)
							time.Sleep(20 * time.Millisecond)
							if before != sha256.Sum256(img.Pix) {
								t.Error("HD front changed during slow upload")
							}
						})
						time.Sleep(5 * time.Millisecond)
					}
				}()
			}
			<-e.done
			if readerDone != nil {
				<-readerDone
			}
			if e.err != nil {
				t.Fatal(e.err)
			}
			for want.Field() < 120 {
				if !want.RunUntilField(want.Field()+1, fieldCycleLimit) {
					t.Fatal("oracle field limit")
				}
			}
			if m.Field() != 120 || e.frameNo < 2 || (!slow && e.frameNo != 2) {
				t.Fatalf("field=%d publications=%d", m.Field(), e.frameNo)
			}
			if m.Digest() != want.Digest() {
				t.Fatalf("native changed: %s != %s", m.Digest(), want.Digest())
			}
			if e.front == nil || !bytes.Equal(e.front.Pix, s.Screen().Pix) {
				t.Fatal("final HD snapshot changed")
			}
			t.Logf("field=%d publications=%d native=%s", m.Field(), e.frameNo, m.Digest())
		})
	}
}

func TestAsyncHDPauseStopPreserveNative(t *testing.T) {
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
	s := gpu.NewShadow(1)
	m.GPU.SetShadow(s)
	defer func() { m.DisablePGXP(); m.GPU.SetShadow(nil); s.Close() }()
	if err = m.EnablePGXP(); err != nil {
		t.Fatal(err)
	}
	e := newEmulator(m, Config{}, nil, &cardSaver{path: filepath.Join(t.TempDir(), "card.mcd")})
	defer e.halt()
	e.start()
	wait := func(ready func() bool) {
		deadline := time.Now().Add(20 * time.Second)
		for !ready() {
			if time.Now().After(deadline) {
				t.Fatal("HD lifecycle wait timeout")
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(func() bool { return e.frameNumber() >= 3 })
	e.paused.Store(true)
	wait(func() bool { return e.parked.Load() })
	parked := m.Field()
	var pausedPixels []byte
	e.useFrame(func(img *image.RGBA, _ uint64) {
		if img != nil {
			pausedPixels = append([]byte(nil), img.Pix...)
		}
	})
	if !bytes.Equal(pausedPixels, s.Screen().Pix) {
		t.Fatal("parked HD frame is stale")
	}
	time.Sleep(25 * time.Millisecond)
	if m.Field() != parked {
		t.Fatal("machine advanced while parked")
	}
	before := e.frameNumber()
	e.paused.Store(false)
	wait(func() bool { return e.frameNumber() > before })
	e.halt()
	if e.err != nil {
		t.Fatal(e.err)
	}
	for want.Field() < m.Field() {
		if !want.RunUntilField(want.Field()+1, fieldCycleLimit) {
			t.Fatal("oracle field limit")
		}
	}
	if m.Digest() != want.Digest() {
		t.Fatalf("native changed after pause/stop: %s != %s", m.Digest(), want.Digest())
	}
	t.Logf("parked=%d final=%d native=%s", parked, m.Field(), m.Digest())
}
