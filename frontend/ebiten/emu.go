package ebiten

import (
	"fmt"
	"image"
	"os"
	"sync"
	"sync/atomic"
	"time"

	eb "github.com/hajimehoshi/ebiten/v2"

	"github.com/wicanr2/ps-golem/internal/padscript"
	"github.com/wicanr2/ps-golem/machine"
)

// cpuClock 是 CPU 時脈（Hz），把模擬的週期數換成實際時間（012 §3）。
const cpuClock = 33_868_800

// maxLag 是模擬落後實際時間的上限；超過就把時間基準移到現在，不追趕（012 §3）。
const maxLag = 100 * time.Millisecond

// emulator 在自己的 goroutine 推進機器（012 §3）。視窗執行緒只交換按鍵與取走最新畫面，
// 不直接碰機器；機器狀態只由這個 goroutine 改。
type emulator struct {
	m      *machine.Machine
	press  []padscript.Press
	until  uint64
	stream *pcmStream
	card   *cardSaver
	stats  bool

	buttons atomic.Uint32 // 視窗執行緒寫入的按鍵
	stop    chan struct{}
	done    chan struct{}
	err     error // done 關閉後才可讀

	mu      sync.Mutex
	front   *image.RGBA // 最新完成的畫面
	back    *image.RGBA
	frameNo uint64
}

func newEmulator(m *machine.Machine, cfg Config, stream *pcmStream, card *cardSaver) *emulator {
	return &emulator{
		m: m, press: cfg.Press, until: cfg.UntilField, stream: stream, card: card, stats: cfg.Stats,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
}

// frame 回傳最新畫面與編號；編號沒變時呼叫端不必重傳。
func (e *emulator) frame() (*image.RGBA, uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.front, e.frameNo
}

// halt 要求停止並等 goroutine 結束。
func (e *emulator) halt() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
	<-e.done
}

func (e *emulator) run() {
	defer close(e.done)
	var pcm []int16
	var st frameStats
	base, baseCycles := time.Now(), e.m.Cycles()
	for {
		select {
		case <-e.stop:
			return
		default:
		}
		if e.until > 0 && e.m.Field() >= e.until {
			return
		}
		if e.press != nil {
			e.m.Pad1.Buttons = padscript.At(e.press, e.m.Field())
		} else {
			e.m.Pad1.Buttons = uint16(e.buttons.Load())
		}
		t0 := time.Now()
		e.m.RunUntilField(e.m.Field()+1, fieldCycleLimit)
		pcm = e.m.TakeAudio(pcm[:0])
		if e.stream != nil {
			e.stream.push(pcm)
		}
		if _, err := e.card.tick(e.m.Card1, e.m.Field()); err != nil {
			e.err = fmt.Errorf("記憶卡存檔：%w", err)
			return
		}
		e.publish()
		if e.stats {
			st.record(time.Since(t0), e.m.Field())
		}

		// 依模擬的週期數對齊實際時間；落後太多就重設基準。
		target := base.Add(time.Duration(float64(e.m.Cycles()-baseCycles) * float64(time.Second) / cpuClock))
		if now := time.Now(); now.Before(target) {
			select {
			case <-e.stop:
				return
			case <-time.After(target.Sub(now)):
			}
		} else if now.Sub(target) > maxLag {
			base, baseCycles = now, e.m.Cycles()
			st.rebases++
		}
	}
}

// publish 把目前畫面畫進後備緩衝再與前景交換。
func (e *emulator) publish() {
	e.back = e.m.GPU.OutputScreenInto(e.back)
	e.mu.Lock()
	e.front, e.back = e.back, e.front
	e.frameNo++
	e.mu.Unlock()
}

// frameStats 累計 -stats 的量測值，每 600 個 field 印一行。
type frameStats struct {
	n          int
	total, max time.Duration
	rebases    int
}

func (s *frameStats) record(d time.Duration, field uint64) {
	s.n++
	s.total += d
	s.max = max(s.max, d)
	if s.n < 600 {
		return
	}
	fmt.Fprintf(os.Stderr, "stats field=%d fps=%.1f emu_avg=%.2fms emu_max=%.2fms lag_resets=%d\n",
		field, eb.ActualFPS(), float64(s.total.Microseconds())/float64(s.n)/1000,
		float64(s.max.Microseconds())/1000, s.rebases)
	*s = frameStats{}
}
