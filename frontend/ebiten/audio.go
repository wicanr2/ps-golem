package ebiten

import (
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// sampleRate 是 SPU 的輸出取樣率（docs/spec/013-spu-and-cd-audio.md §5）。
const sampleRate = 44100

// maxQueued 是最多排隊的取樣組數（約 0.25 秒）；超過時丟掉最舊的，避免延遲越積越長。
const maxQueued = sampleRate / 4

// pcmStream 是給 Ebiten 播放器讀的立體聲 16 位元小端串流。
// Update 寫入、播放器在自己的 goroutine 讀取，所以要上鎖。
type pcmStream struct {
	produced, requested, nonzeroDropped uint64
	mu                                  sync.Mutex
	buf                                 []int16 // L、R 交錯
	starved, dropped                    uint64  // 立體聲取樣組數，只供 -stats 觀測
}

// push 加入取樣（L、R 交錯）。
func (s *pcmStream) push(samples []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.produced += uint64(len(samples) / 2)
	s.buf = append(s.buf, samples...)
	if over := len(s.buf) - maxQueued*2; over > 0 {
		s.dropped += uint64(over / 2)
		for i := 0; i+1 < over; i += 2 {
			if s.buf[i] != 0 || s.buf[i+1] != 0 {
				s.nonzeroDropped++
			}
		}
		s.buf = append(s.buf[:0], s.buf[over:]...)
	}
}

// Read 實作 io.Reader：有多少給多少，不夠的部分補靜音，讓播放器不會停下來。
func (s *pcmStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(p) / 4 * 4
	frames := n / 2 // int16 個數
	s.requested += uint64(frames / 2)
	take := min(frames, len(s.buf))
	s.starved += uint64((frames - take) / 2)
	for i := 0; i < take; i++ {
		v := uint16(s.buf[i])
		p[2*i], p[2*i+1] = byte(v), byte(v>>8)
	}
	for i := 2 * take; i < n; i++ {
		p[i] = 0
	}
	s.buf = append(s.buf[:0], s.buf[take:]...)
	return n, nil
}

func (s *pcmStream) counters() (uint64, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starved, s.dropped
}

// newAudio 建立播放器並開始播放。
func newAudio(s *pcmStream) (*audio.Player, error) {
	ctx := audio.NewContext(sampleRate)
	p, err := ctx.NewPlayer(s)
	if err != nil {
		return nil, err
	}
	p.SetBufferSize(60 * time.Millisecond)
	p.Play()
	return p, nil
}

// playbackReady 只在起跑前查詢後端就緒；靜音模式不等待音訊。
func playbackReady() bool { c := audio.CurrentContext(); return c != nil && c.IsReady() }

func (s *pcmStream) traffic() (uint64, uint64, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.produced, s.requested, s.nonzeroDropped
}
