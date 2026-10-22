package hd

import (
	"github.com/wicanr2/ps-golem/internal/pgxp"
	"image"
	"math"
	"sync"
)

// Shadow 由一個機器執行緒寫入；Screen、VRAMCopy、Close 也由該執行緒呼叫。
// 輸入以批次送給獨立 goroutine。生產端只交換佇列，不等待光柵化。
type Shadow struct {
	tracked    bool
	scale      int
	pending    *batch
	n          int
	mu         sync.Mutex
	head, tail *batch
	wake       chan struct{}
	done       chan struct{}
	pool       sync.Pool
	closed     bool
}

// 及早送出，讓短命令串也能與 CPU 並行；語意與同步邊界不變（016 §8）。
const batchWords = 16

type batch struct {
	words [batchWords]uint64
	n     int
	next  *batch
	read  func(*Renderer)
	stop  bool
}

// NewShadow 支援 shift 0–3，即 1、2、4、8 倍。呼叫者負責 Close。
func NewShadow(shift uint) *Shadow { return NewShadowWithTextures(shift, nil, nil) }

// NewShadowWithTextures 在第一筆 GPU 輸入前配置已驗證素材包及選用傾印。
func NewShadowWithTextures(shift uint, pack *TexturePack, dump *TextureDump) *Shadow {
	return NewShadowWithDevice(shift, pack, dump, nil)
}

// NewShadowWithDevice 在開機前選擇呈現裝置；nil 保留純CPU路徑。裝置由消費端關閉。
func NewShadowWithDevice(shift uint, pack *TexturePack, dump *TextureDump, device RasterDevice) *Shadow {
	if shift > 3 {
		panic("影子 GPU 的 shift 必須介於 0 與 3")
	}
	s := &Shadow{scale: 1 << shift, wake: make(chan struct{}, 1), done: make(chan struct{})}
	s.pool.New = func() any { return new(batch) }
	s.pending = s.pool.Get().(*batch)
	g := newRenderer(shift)
	g.configureTextures(pack, dump)
	g.attachRaster(device)
	s.tracked = g.textures != nil
	go s.run(g)
	return s
}

func (s *Shadow) append(v uint64) {
	if s.closed {
		panic("影子 GPU 已關閉")
	}
	s.pending.words[s.n] = v
	s.n++
	if s.n == batchWords {
		s.flush()
	}
}

// GP0、GP1、Field 保留原生入口的順序與原始字，不解碼或改寫輸入。
func (s *Shadow) GP0(v uint32) { s.append(uint64(v)) }

func (s *Shadow) Scale() int { return s.scale }

// GP0Precision 複製精度值，避免消費端讀到之後被遊戲覆寫的記憶體影子。
func (s *Shadow) GP0Precision(v uint32, p pgxp.Value) {
	if p.Valid(v) {
		if p.Z > 0 {
			s.append(8<<32 | uint64(math.Float32bits(p.Z)))
		}
		s.append(3<<32 | uint64(math.Float32bits(p.X)))
		s.append(4<<32 | uint64(math.Float32bits(p.Y)))
		s.append(5<<32 | uint64(v))
	}
	s.GP0(v)
}

// PrecisionStats 同步後回傳累計命中與多邊形頂點數。
func (s *Shadow) PrecisionStats() (hits, vertices uint64) {
	s.read(func(g *Renderer) { hits, vertices = g.pgxpVertices, g.polygonVertices })
	return
}
func (s *Shadow) GP1(v uint32) { s.append(1<<32 | uint64(v)) }
func (s *Shadow) Field(v bool) {
	var b uint64
	if v {
		b = 1
	}
	s.append(2<<32 | b)
}

func (s *Shadow) enqueue(b *batch) {
	s.mu.Lock()
	if s.tail == nil {
		s.head = b
	} else {
		s.tail.next = b
	}
	s.tail = b
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Shadow) flush() {
	if s.n == 0 {
		return
	}
	b := s.pending
	b.n = s.n
	s.pending, s.n = s.pool.Get().(*batch), 0
	s.enqueue(b)
}

func (s *Shadow) run(g *Renderer) {
	defer close(s.done)
	if g.raster != nil {
		defer g.raster.device.Close()
	}
	for range s.wake {
		s.mu.Lock()
		list := s.head
		s.head, s.tail = nil, nil
		s.mu.Unlock()
		for b := list; b != nil; {
			for _, v := range b.words[:b.n] {
				switch v >> 32 {
				case 0:
					g.WriteGP0(uint32(v))
				case 1:
					g.WriteGP1(uint32(v))
				case 2:
					g.SetField(v&1 != 0)
				case 3:
					g.nextPrecision.X = math.Float32frombits(uint32(v))
				case 4:
					g.nextPrecision.Y = math.Float32frombits(uint32(v))
				case 5:
					g.nextPrecision.Raw, g.nextPrecision.Flags = uint32(v), pgxp.ValidXY
				case 6:
					g.flushSpriteRun()
					g.textures.field = (g.textures.field & 0xffffffff00000000) | uint64(uint32(v))
				case 8:
					g.nextPrecision.Z = math.Float32frombits(uint32(v))
				case 7:
					g.flushSpriteRun()
					g.textures.field = (g.textures.field & 0xffffffff) | (uint64(uint32(v)) << 32)
				}
			}
			if b.read != nil || b.stop {
				g.flushSpriteRun()
			}
			if b.read != nil {
				b.read(g)
			}
			if b.stop {
				return
			}
			next := b.next
			b.n, b.next, b.read = 0, nil, nil
			s.pool.Put(b)
			b = next
		}
	}
}

// read 在所有先前輸入消化完畢後才取樣，不把影子資訊交回機器。
func (s *Shadow) read(fn func(*Renderer)) {
	if s.closed {
		panic("影子 GPU 已關閉")
	}
	s.flush()
	done := make(chan struct{})
	s.enqueue(&batch{read: func(g *Renderer) { fn(g); close(done) }})
	<-done
}

func (s *Shadow) Screen() *image.RGBA { return s.ScreenInto(nil) }

func (s *Shadow) ScreenInto(dst *image.RGBA) (img *image.RGBA) {
	s.read(func(g *Renderer) { img = g.ScreenInto(dst) })
	return
}

// VRAMCopy 回傳已同步的獨立副本，供 N=1 逐 field 自我檢查。
func (s *Shadow) VRAMCopy() (v []uint16) {
	s.read(func(g *Renderer) { g.syncRaster(); v = append([]uint16(nil), g.VRAM...); g.rasterFence() })
	return
}

// Close 等待已排入的輸入處理完畢並回收 goroutine，可重複呼叫。
// 關閉前須先從原生 GPU 卸下，關閉後不可再送入輸入或取畫面。
func (s *Shadow) Close() {
	if s.closed {
		return
	}
	s.flush()
	s.closed = true
	s.pending = nil
	s.enqueue(&batch{stop: true})
	<-s.done
}

// TextureField 是觀測附件，不是 GP1 隔行奇偶，不回寫機器。
func (s *Shadow) TextureField(field uint64) {
	if !s.tracked {
		return
	}
	s.append(6<<32 | uint64(uint32(field)))
	s.append(7<<32 | uint64(uint32(field>>32)))
}
func (s *Shadow) TextureStats() (stats TextureStats) {
	s.read(func(g *Renderer) {
		if g.textures != nil {
			stats = g.textures.stats
		}
	})
	return
}

// RequestScreen 在目前命令邊界非同步取樣。fn 在影子消費端執行，須保持有界且不能存取 Machine。
// 呼叫者在 fn 完成前不得重用 dst，且須自行限制未完成要求數量（016 §8.1）。
func (s *Shadow) RequestScreen(dst *image.RGBA, fn func(*image.RGBA)) {
	if s.closed {
		panic("影子 GPU 已關閉")
	}
	s.flush()
	s.enqueue(&batch{read: func(g *Renderer) { fn(g.ScreenInto(dst)) }})
}

// Fence 在先前呈現命令處理後執行 fn；GPU後端是提交界線，像素讀取另由Read同步。
// 不轉換RGB，供呈現端限制field積壓（016 §8.1、021）。
func (s *Shadow) Fence(fn func()) {
	if s.closed {
		panic("影子 GPU 已關閉")
	}
	s.flush()
	s.enqueue(&batch{read: func(g *Renderer) { g.rasterFence(); fn() }})
}

// RasterStatistics 是呈現診斷；CPU後端回傳零值。
func (s *Shadow) RasterStatistics() (stats RasterStats) {
	s.read(func(g *Renderer) {
		if g.raster != nil {
			stats = g.raster.stats
		}
	})
	return
}
