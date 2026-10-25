package ebiten

import (
	"sync/atomic"
	"testing"
)

func TestFinishSessionKeepsServiceAvailable(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	g := &Game{sess: &Session{Close: func() { calls.Add(1); close(started); <-release }}}
	if g.finishSession() {
		t.Fatal("close completed before GPU service was released")
	}
	<-started
	if g.sess.Close != nil || g.finishSession() {
		t.Fatal("closing session started twice or blocked service")
	}
	close(release)
	<-g.sessionClosing
	if !g.finishSession() || calls.Load() != 1 {
		t.Fatal("close was not exactly once")
	}
}
