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
	startOnce  sync.Once
	m          *machine.Machine
	press      []padscript.Press
	until      uint64
	stream     *pcmStream
	card       *cardSaver
	stats      bool
	afterField func(*machine.Machine) error

	wantFrame atomic.Bool   // 呈現端合併要求，只取最新完成的畫面
	paused    atomic.Bool   // 設定面板只要求在 field 邊界暫停
	parked    atomic.Bool   // 在 field 邊界進入暫停的確認狀態
	buttons   atomic.Uint32 // 視窗執行緒寫入的按鍵
	stop      chan struct{}
	done      chan struct{}
	err       error // done 關閉後才可讀

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

// frameNumber 只查詢發布計數，並要求下一張畫面；像素須經 useFrame 讀取。
func (e *emulator) frameNumber() uint64 {
	e.wantFrame.Store(true)
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.frameNo
}

// useFrame 在上傳期間保留前景所有權，防止慢視窗讀到已重用的緩衝。
func (e *emulator) useFrame(fn func(*image.RGBA, uint64)) {
	e.wantFrame.Store(true)
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e.front, e.frameNo)
}

// start 僅在視窗就緒後起跑一次。
func (e *emulator) start() { e.startOnce.Do(func() { go e.run() }) }

// halt 要求停止並等 goroutine 結束。
// requestStop 讓前端在圖形主迴圈仍運作時等GPU工作完成，避免關窗後才回讀。
func (e *emulator) requestStop() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
	e.start()
}
func (e *emulator) halt() {
	e.requestStop()
	<-e.done
}

func (e *emulator) run() {
	defer close(e.done)
	defer e.parked.Store(false)
	var pending chan struct{}
	var requestedField uint64
	waitFrame := func() {
		if pending != nil {
			<-pending
			pending = nil
		}
	}
	defer waitFrame()

	var pcm []int16
	var st frameStats
	base, baseCycles := time.Now(), e.m.Cycles()
	e.wantFrame.Store(true)
	for {
		select {
		case <-e.stop:
			return
		default:
		}
		if e.paused.Load() {
			if !e.parked.Load() {
				waitFrame()
				if e.m.Field() > 0 {
					e.publish()
				}
			}
			e.parked.Store(true)
			select {
			case <-e.stop:
				return
			case <-time.After(time.Second / 60):
				continue
			}
		}
		e.parked.Store(false)
		if e.until > 0 && e.m.Field() >= e.until {
			return
		}
		if e.press != nil {
			e.m.Pad1.Buttons = padscript.At(e.press, e.m.Field())
		} else {
			e.m.Pad1.Buttons = uint16(e.buttons.Load())
		}
		e.m.GPU.TextureField(e.m.Field())
		t0 := time.Now()
		if e.stats && st.n == 0 {
			st.start = t0
			st.startCycles = e.m.Cycles()
			if e.stream != nil {
				st.audioStart, st.dropStart = e.stream.counters()
				st.producedStart, st.requestedStart, st.nonzeroStart = e.stream.traffic()
			}
		}
		if reached := e.m.RunUntilField(e.m.Field()+1, fieldCycleLimit); reached && e.afterField != nil {
			if err := e.afterField(e.m); err != nil {
				e.err = err
				return
			}
		}
		coreEnd := t0
		if e.stats {
			coreEnd = time.Now()
		}
		if pending != nil {
			select {
			case <-pending:
				pending = nil
			default:
			}
			if pending != nil && e.m.Field()-requestedField >= 1 {
				waitFrame()
			}
		}
		if e.stats {
			st.core += coreEnd.Sub(t0)
			st.hdWait += time.Since(coreEnd)
		}
		pcm = e.m.TakeAudio(pcm[:0])
		if e.stream != nil {
			e.stream.push(pcm)
		}
		if _, err := e.card.tick(e.m.Card1, e.m.Field()); err != nil {
			e.err = fmt.Errorf("記憶卡存檔：%w", err)
			return
		}
		var video time.Duration

		if e.until > 0 && e.m.Field() >= e.until {
			videoStart := time.Now()
			waitFrame()
			e.publish()
			video = time.Since(videoStart)
		} else if pending == nil && e.wantFrame.Swap(false) {
			videoStart := time.Now()
			done := make(chan struct{})
			if e.m.GPU.RequestOutputScreen(e.back, func(img *image.RGBA) {
				e.mu.Lock()
				e.front, e.back = img, e.front
				e.frameNo++
				e.mu.Unlock()
				close(done)
			}) {
				pending = done
				requestedField = e.m.Field()
			} else {
				e.publish()
			}
			video = time.Since(videoStart)
		}

		if pending == nil && !(e.until > 0 && e.m.Field() >= e.until) {
			done := make(chan struct{})
			if e.m.GPU.RequestFence(func() { close(done) }) {
				pending = done
				requestedField = e.m.Field()
			}
		}

		if e.stats {
			if st.n == 599 && e.stream != nil {
				a, d := e.stream.counters()
				st.audioStarved = a - st.audioStart
				st.audioDropped = d - st.dropStart
				p, r, z := e.stream.traffic()
				st.audioProduced = p - st.producedStart
				st.audioRequested = r - st.requestedStart
				st.audioNonzeroDropped = z - st.nonzeroStart
			}
			st.record(time.Since(t0), video, e.m.Field(), e.m.Cycles())
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
	producedStart, requestedStart, nonzeroStart, audioProduced, audioRequested, audioNonzeroDropped uint64
	n                                                                                               int
	core, hdWait                                                                                    time.Duration
	total, max, video                                                                               time.Duration
	start                                                                                           time.Time
	startCycles                                                                                     uint64
	published                                                                                       int
	audioStart, dropStart, audioStarved, audioDropped                                               uint64
	rebases                                                                                         int
}

func (s *frameStats) record(d, video time.Duration, field, cycles uint64) {
	s.n++
	s.total += d
	s.video += video
	if video > 0 {
		s.published++
	}
	s.max = max(s.max, d)
	if s.n < 600 {
		return
	}
	elapsed := time.Since(s.start).Seconds()
	fmt.Fprintf(os.Stderr, "stats field=%d fps=%.1f field_hz=%.2f speed=%.3f emu_avg=%.2fms core_avg=%.2fms hd_wait_avg=%.2fms video_avg=%.2fms published=%d audio_starved=%d audio_dropped=%d audio_nonzero_dropped=%d audio_produced=%d audio_requested=%d emu_max=%.2fms lag_resets=%d\n",
		field, eb.ActualFPS(), float64(s.n)/elapsed, float64(cycles-s.startCycles)/cpuClock/elapsed,
		float64(s.total.Microseconds())/float64(s.n)/1000, float64(s.core.Microseconds())/float64(s.n)/1000, float64(s.hdWait.Microseconds())/float64(s.n)/1000, float64(s.video.Microseconds())/float64(max(1, s.published))/1000, s.published, s.audioStarved, s.audioDropped, s.audioNonzeroDropped, s.audioProduced, s.audioRequested,
		float64(s.max.Microseconds())/1000, s.rebases)
	*s = frameStats{}
}
