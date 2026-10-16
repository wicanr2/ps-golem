package hd

import (
	"image"
	"sync"
)

// Shadow 由一個機器執行緒寫入；Screen、VRAMCopy、Close 也由該執行緒呼叫。
// 輸入以批次送給獨立 goroutine。生產端只交換佇列，不等待光柵化。
type Shadow struct {
	pending    *batch
	n          int
	mu         sync.Mutex
	head, tail *batch
	wake       chan struct{}
	done       chan struct{}
	pool       sync.Pool
	closed     bool
}

const batchWords = 4096

type batch struct {
	words [batchWords]uint64
	n     int
	next  *batch
	read  func(*Renderer)
	stop  bool
}

// NewShadow 支援 shift 0–3，即 1、2、4、8 倍。呼叫者負責 Close。
func NewShadow(shift uint) *Shadow {
	if shift > 3 {
		panic("影子 GPU 的 shift 必須介於 0 與 3")
	}
	s := &Shadow{wake: make(chan struct{}, 1), done: make(chan struct{})}
	s.pool.New = func() any { return new(batch) }
	s.pending = s.pool.Get().(*batch)
	go s.run(newRenderer(shift))
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
				}
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

func (s *Shadow) Screen() (img *image.RGBA) {
	s.read(func(g *Renderer) { img = g.Screen() })
	return
}

// VRAMCopy 回傳已同步的獨立副本，供 N=1 逐 field 自我檢查。
func (s *Shadow) VRAMCopy() (v []uint16) {
	s.read(func(g *Renderer) { v = append([]uint16(nil), g.VRAM...) })
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
