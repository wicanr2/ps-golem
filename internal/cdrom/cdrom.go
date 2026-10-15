// Package cdrom 實作 PlayStation 的 CD-ROM 控制器。規格：docs/spec/009-cdrom.md §2。
//
// 參照 beetle-psx-libretro mednafen/psx/cdc.c
// (commit e57860d82faa953dc0fc10f201fd583739bc2f7d)，GPL-2.0-or-later。
// 亂數延遲改成區間中點的常數（決定性）；XA／CD-DA 的聲音輸出留給 SPU（docs/spec/010）。
package cdrom

import "github.com/wicanr2/ps-golem/internal/disc"

const cpuHz = 33868800

// 中斷類型。
const (
	irqNone      = 0
	irqDataReady = 1
	irqComplete  = 2
	irqAck       = 3
	irqDataEnd   = 4
	irqError     = 5
)

const (
	errBadArgVal  = 0x10
	errBadNumArgs = 0x20
	errBadCommand = 0x40
	errNotReady   = 0x80
)

const (
	modeSpeed     = 0x80
	modeStrSnd    = 0x40
	modeSize      = 0x20
	modeSize2     = 0x10
	modeSF        = 0x08
	modeReport    = 0x04
	modeAutoPause = 0x02
	modeCDDA      = 0x01
)

// 磁碟機狀態。
const (
	dsStandby = -2
	dsPaused  = -1
	dsStopped = 0
	dsSeeking = iota
	dsSeekingLogical
	dsSeekingLogical2
	dsPlaying
	dsReading
	dsResetting
)

const pipeCount = 2

// CDROM 是控制器的完整狀態。
type CDROM struct {
	disc         *disc.Disc
	isPSXDisc    bool
	discID       [4]byte
	driveStatus  int
	discChanged  bool
	startupDelay int64

	regSelector uint8

	argsBuf    [16]uint8
	argsWP     uint8
	argsRP     uint8
	argsLatch  uint8
	argsRecv   [32]uint8
	argsRecvIn int

	results   [16]uint8
	resultsIn uint8
	resultsWP uint8
	resultsRP uint8

	dmaBuf   []uint8
	dmaPos   int
	sb       [2340]uint8
	sbIn     int
	pipe     [pipeCount][disc.SectorSize]uint8
	pipeSubQ [pipeCount][12]uint8
	pipePos  int
	pipeIn   int

	subQ        [12]uint8
	headerBuf   [12]uint8
	headerValid bool

	irqBuffer    uint8
	irqMask      uint8
	readyCounter int64

	filterFile, filterChan uint8

	pendingCmd     uint8
	pendingPhase   int
	pendingCounter int64

	mode            uint8
	statusAfterSeek int
	forward         bool
	backward        bool
	muted           bool
	playTrackMatch  int
	psrCounter      int64
	curSector       int32
	sectorsRead     uint32

	aipIRQ   int
	aipRes   [16]uint8
	aipCount int

	seekTarget      int32
	seekRetry       uint32
	seekFinished    int
	holdLogicalPos  bool
	reportStartup   int64
	commandLoc      int32
	commandLocDirty bool
	reportLastF     uint8

	audio      audioState
	spuCounter int64 // 到 SPU 下一個取樣點的週期數（Beetle SPUCounter）
}

// New 建立上電狀態的控制器。
func New() *CDROM {
	c := &CDROM{spuCounter: 768} // Beetle PS_CDC_Power：SPU_UpdateFromCDC(0) 回傳 768
	c.softReset()
	return c
}

// Insert 放入光碟（nil 表示沒有光碟）。
func (c *CDROM) Insert(d *disc.Disc) {
	c.disc = d
	c.isPSXDisc = false
	c.discID = [4]byte{}
	if d == nil {
		c.driveStatus = dsStopped
		return
	}
	c.headerValid = false
	c.startupDelay = cpuHz
	c.discChanged = true
	if r := d.Region(); r != "" {
		copy(c.discID[:], r)
		c.isPSXDisc = true
	}
}

func (c *CDROM) softReset() {
	c.audio.clear()
	c.audio.resetVolume()
	c.regSelector = 0
	c.argsBuf = [16]uint8{}
	c.argsWP, c.argsRP = 0, 0
	c.results = [16]uint8{}
	c.resultsWP, c.resultsRP, c.resultsIn = 0, 0, 0
	c.readyCounter = 0
	c.irqBuffer = 0
	c.irqMask = 0
	c.dmaBuf, c.dmaPos = nil, 0
	c.sbIn = 0
	c.pipePos, c.pipeIn = 0, 0
	c.sectorsRead = 0
	c.subQ = [12]uint8{}
	c.headerBuf = [12]uint8{}
	c.filterFile, c.filterChan = 0, 0
	c.pendingCmd, c.pendingPhase, c.pendingCounter = 0, 0, 0
	c.mode = 0x20
	c.headerValid = false
	c.driveStatus = dsStopped
	c.clearAIP()
	c.statusAfterSeek = dsStopped
	c.seekRetry = 0
	c.forward, c.backward, c.muted = false, false, false
	c.playTrackMatch = 0
	c.psrCounter = 0
	c.curSector = 0
	c.reportLastF = 0xFF
	c.reportStartup = 0
	c.seekFinished = -1
	c.seekTarget = 0
	c.commandLoc = 0
	c.commandLocDirty = true
	c.discChanged = true
}

// IRQ 回報中斷線狀態（接 I_STAT bit 2）。
func (c *CDROM) IRQ() bool { return c.irqBuffer&(c.irqMask&0x1F) != 0 }

func (c *CDROM) clearAIP() { c.aipCount, c.aipIRQ = 0, 0 }

func (c *CDROM) writeIRQ(v uint8) {
	c.readyCounter = 2000
	c.irqBuffer = c.irqBuffer&0x10 | v
}

func (c *CDROM) beginResults() {
	c.resultsIn, c.resultsWP, c.resultsRP = 0, 0, 0
	c.results = [16]uint8{}
}

func (c *CDROM) writeResult(v uint8) {
	c.results[c.resultsWP] = v
	c.resultsWP = (c.resultsWP + 1) & 0xF
	c.resultsIn = (c.resultsIn + 1) & 0x1F
}

func (c *CDROM) readResult() uint8 {
	r := c.results[c.resultsRP]
	c.resultsRP = (c.resultsRP + 1) & 0xF
	c.resultsIn = (c.resultsIn - 1) & 0x1F
	return r
}

func (c *CDROM) makeStatus(cmdErr bool) uint8 {
	var r uint8
	switch c.driveStatus {
	case dsPlaying:
		r |= 0x80
	case dsReading:
		if c.headerValid {
			r |= 0x20
			break
		}
		r |= 0x40
	case dsSeeking, dsSeekingLogical, dsSeekingLogical2:
		r |= 0x40
	}
	if c.disc == nil || c.discChanged {
		r |= 0x10
	}
	if c.driveStatus != dsStopped {
		r |= 0x02
	}
	if cmdErr {
		r |= 0x01
	}
	c.discChanged = false
	return r
}

func (c *CDROM) checkAIP() {
	if c.aipIRQ != 0 && c.readyCounter <= 0 {
		c.beginResults()
		for i := 0; i < c.aipCount; i++ {
			c.writeResult(c.aipRes[i])
		}
		c.writeIRQ(uint8(c.aipIRQ))
		c.clearAIP()
	}
}

func (c *CDROM) setAIP(irq int, r ...uint8) {
	c.clearAIP()
	c.aipCount = copy(c.aipRes[:], r)
	c.aipIRQ = irq
	c.checkAIP()
}

// ---- 暫存器 ----

// Read 讀 1F801800+a。
func (c *CDROM) Read(a uint32) uint8 {
	switch a & 3 {
	case 0:
		r := c.regSelector & 3
		if c.argsWP == c.argsRP {
			r |= 0x08
		}
		if (c.argsWP-c.argsRP)&0x10 == 0 {
			r |= 0x10
		}
		if c.resultsIn != 0 {
			r |= 0x20
		}
		if c.dmaPos < len(c.dmaBuf) {
			r |= 0x40
		}
		if c.pendingCounter > 0 && c.pendingPhase <= 1 {
			r |= 0x80
		}
		return r
	case 1:
		return c.readResult()
	case 2:
		if c.dmaPos < len(c.dmaBuf) {
			v := c.dmaBuf[c.dmaPos]
			c.dmaPos++
			return v
		}
		return 0
	default:
		if c.regSelector&1 != 0 {
			return 0xE0 | c.irqBuffer
		}
		return 0xFF
	}
}

// Write 寫 1F801800+a。
func (c *CDROM) Write(a uint32, v uint8) {
	a &= 3
	if a == 0 {
		c.regSelector = v & 3
		return
	}
	switch uint32(c.regSelector&3)*3 + (a - 1) {
	case 0x00:
		// Beetle 為 10500 + rand(0..3000) + 1815；取中點。
		c.pendingCounter = 10500 + 1500 + 1815
		c.pendingCmd = v
		c.pendingPhase = -1
		c.argsRecvIn = 0
	case 0x01:
		c.argsBuf[c.argsWP&0xF] = v
		c.argsWP = (c.argsWP + 1) & 0x1F
	case 0x02:
		switch {
		case v&0x80 != 0:
			if c.dmaPos >= len(c.dmaBuf) {
				if c.sbIn == 0 {
					// 沒有新扇區：整個 2340 位元組（含舊資料）再送一次，Beetle 補 0 到 FIFO 滿。
					c.dmaBuf = append(c.dmaBuf[:0], c.sb[:]...)
					for len(c.dmaBuf) < 4096 {
						c.dmaBuf = append(c.dmaBuf, 0)
					}
				} else {
					c.dmaBuf = append(c.dmaBuf[:0], c.sb[:c.sbIn]...)
					c.sbIn = 0
				}
				c.dmaPos = 0
			}
		case v&0x40 != 0:
			for i := 0; i < 4 && c.dmaPos < len(c.dmaBuf); i++ {
				c.dmaPos++
			}
		default:
			c.dmaBuf, c.dmaPos = c.dmaBuf[:0], 0
		}
		if v&0x20 != 0 {
			c.irqBuffer |= 0x10
		}
	case 0x04:
		c.irqMask = v
	case 0x07:
		c.audio.pendingVol[0][0] = v
	case 0x08:
		c.audio.pendingVol[0][1] = v
	case 0x09:
		c.audio.pendingVol[1][1] = v
	case 0x0A:
		c.audio.pendingVol[1][0] = v
	case 0x0B:
		if v&0x20 != 0 {
			c.audio.vol = c.audio.pendingVol
		}
	case 0x05:
		c.irqBuffer &^= v
		if v&0x80 != 0 {
			c.softReset()
		}
		if v&0x40 != 0 {
			c.argsWP, c.argsRP = 0, 0
		}
	}
}

// DMARead 給 DMA channel 3：讀 4 個位元組。
func (c *CDROM) DMARead() uint32 {
	var v uint32
	for i := 0; i < 4; i++ {
		if c.dmaPos < len(c.dmaBuf) {
			v |= uint32(c.dmaBuf[c.dmaPos]) << (8 * i)
			c.dmaPos++
		}
	}
	return v
}

// ---- 時間推進（Beetle PS_CDC_Update）----

// Tick 推進 clocks 個 CPU 週期。spu 不為 nil 時，時間切到 SPU 的取樣點，
// 每段處理完 CD 事件後推進 SPU（Beetle PS_CDC_Update 的 SPUCounter，013 §3）。
func (c *CDROM) Tick(clocks int64, spu SPUClock) {
	for clocks > 0 {
		chunk := clocks
		if spu != nil && c.spuCounter > 0 && chunk > c.spuCounter {
			chunk = c.spuCounter
		}
		if c.psrCounter > 0 && chunk > c.psrCounter {
			chunk = c.psrCounter
		}
		if c.pendingCounter > 0 && chunk > c.pendingCounter {
			chunk = c.pendingCounter
		}
		if c.startupDelay > 0 {
			if chunk > c.startupDelay {
				chunk = c.startupDelay
			}
			c.startupDelay -= chunk
			if c.startupDelay <= 0 {
				c.seekTarget = c.curSector
				c.holdLogicalPos = false
				c.driveStatus = dsPaused
			}
		}
		if c.irqBuffer&0xF == 0 {
			if c.readyCounter > 0 && chunk > c.readyCounter {
				chunk = c.readyCounter
			}
			if c.readyCounter > 0 {
				c.readyCounter -= chunk
			}
		}
		c.checkAIP()

		if c.psrCounter > 0 {
			c.psrCounter -= chunk
			if c.reportStartup > 0 {
				c.reportStartup -= chunk
			}
			if c.psrCounter <= 0 {
				switch c.driveStatus {
				case dsResetting:
					c.setAIP(irqComplete, c.makeStatus(false))
					c.muted = false
					c.audio.clear()
					c.sbIn = 0
					c.pipePos, c.pipeIn = 0, 0
					c.sectorsRead = 0
					c.mode = 0x20
					c.curSector = 0
					c.commandLoc = 0
					c.seekTarget = 0
					c.holdLogicalPos = false
					c.seekFinished = 1
					c.driveStatus = dsPaused
					c.clearAIP()
				case dsSeeking:
					c.curSector = c.seekTarget
					c.holdLogicalPos = false
					c.driveStatus = c.statusAfterSeek
					c.seekFinished = 1
					c.reportStartup = 24000000
					if c.driveStatus == dsPaused || c.driveStatus == dsStandby {
						c.curSector -= 9
						if c.curSector < -150 {
							c.curSector = -150
						}
					}
					c.psrCounter = c.sectorTime(false)
				case dsSeekingLogical:
					c.curSector = c.seekTarget
					c.holdLogicalPos = true
					c.driveStatus = dsSeekingLogical2
				}
				switch c.driveStatus {
				case dsPaused, dsStandby, dsReading, dsPlaying, dsSeekingLogical2:
					c.handlePlayRead()
				}
			}
		}

		if c.pendingCounter > 0 {
			c.pendingCounter -= chunk
			if c.pendingCounter <= 0 && c.readyCounter > 0 {
				c.pendingCounter = c.readyCounter
			} else if c.pendingCounter <= 0 {
				c.runPending()
			}
		}
		if spu != nil {
			c.spuCounter = spu.Advance(chunk, c)
		}
		clocks -= chunk
	}
}

// SPUCounter 回傳到 SPU 下一個取樣點的週期數（機器在 SPU 中斷開啟時據此同步，013 §3）。
func (c *CDROM) SPUCounter() int64 { return c.spuCounter }

func (c *CDROM) runPending() {
	var next int64
	if c.pendingPhase >= 2 {
		c.beginResults()
		next = c.commandPart2(c.pendingCmd)
	} else {
		switch c.pendingPhase {
		case -1:
			if c.argsRP != c.argsWP {
				c.argsLatch = c.argsBuf[c.argsRP&0xF]
				c.argsRP = (c.argsRP + 1) & 0x1F
				c.pendingPhase++
				next = 1815
			} else {
				c.pendingPhase += 2
				next = 8500
			}
		case 0:
			if c.argsRecvIn < 32 {
				c.argsRecv[c.argsRecvIn] = c.argsLatch
				c.argsRecvIn++
			}
			if c.argsRP != c.argsWP {
				c.argsLatch = c.argsBuf[c.argsRP&0xF]
				c.argsRP = (c.argsRP + 1) & 0x1F
				next = 1815
			} else {
				c.pendingPhase++
				next = 8500
			}
		default:
			c.beginResults()
			min, max, ok := cmdArgs(c.pendingCmd)
			switch {
			case !ok:
				c.writeResult(c.makeStatus(true))
				c.writeResult(errBadCommand)
				c.writeIRQ(irqError)
			case c.argsRecvIn < min || c.argsRecvIn > max:
				c.writeResult(c.makeStatus(true))
				c.writeResult(errBadNumArgs)
				c.writeIRQ(irqError)
			default:
				next = c.command(c.pendingCmd, c.argsRecv[:c.argsRecvIn])
				c.pendingPhase = 2
			}
			c.argsRecvIn = 0
		}
	}
	if next == 0 {
		c.pendingCounter = 0
	} else {
		c.pendingCounter += next
	}
}

func (c *CDROM) sectorTime(allowDouble bool) int64 {
	if c.mode&modeSpeed != 0 {
		return cpuHz / (75 * 2)
	}
	return cpuHz / 75
}

func bcdToU8(v uint8) int { return int(v>>4)*10 + int(v&0xF) }
func u8ToBCD(v int) uint8 { return uint8(v/10<<4 | v%10) }

func (c *CDROM) leadOut() int32 {
	if c.disc == nil {
		return 0
	}
	return c.disc.LeadOut()
}

// handlePlayRead 照 Beetle PS_CDC_HandlePlayRead：處理管線裡的上一個扇區、讀進下一個。
func (c *CDROM) handlePlayRead() {
	if c.disc == nil {
		return
	}
	target := &c.pipe[c.pipePos]
	wasFull := c.pipeIn >= pipeCount

	if c.curSector >= c.leadOut()+300 && c.curSector >= 75*60*75-150 {
		c.seekFinished = -1
		c.driveStatus = dsStopped
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		return
	}

	if wasFull {
		buf := target
		q := c.pipeSubQ[c.pipePos]
		c.pipeIn--
		if q[0]&0x40 != 0 { // 資料扇區
			if c.driveStatus == dsSeekingLogical2 || c.driveStatus == dsReading ||
				(c.holdLogicalPos && (c.driveStatus == dsPaused || c.driveStatus == dsStandby)) {
				copy(c.headerBuf[:], buf[12:24])
				c.headerValid = true
				if c.driveStatus == dsSeekingLogical2 {
					lba := int32(bcdToU8(c.headerBuf[0])*60*75+bcdToU8(c.headerBuf[1])*75+bcdToU8(c.headerBuf[2])) - 150
					if lba == c.seekTarget {
						c.driveStatus = c.statusAfterSeek
						c.seekFinished = 1
						c.reportStartup = 24000000
					} else if c.seekRetry == 0 {
						c.holdLogicalPos = false
						c.driveStatus = dsStandby
						c.seekFinished = -1
					} else {
						c.seekRetry--
					}
				}
			}
			if c.driveStatus == dsReading {
				if c.mode&modeStrSnd != 0 && buf[12+3] == 0x2 && buf[12+6]&0x64 == 0x64 {
					if c.xaTest(buf) && c.audio.read >= c.audio.size {
						c.xaProcessSector(buf)
					}
				} else {
					offs, size := 12, 2048
					if c.mode&modeSize != 0 {
						offs, size = 0, 2340
					}
					if c.mode&modeSize2 != 0 {
						offs, size = 12, 2328
					}
					copy(c.sb[:], buf[12+offs:12+offs+size])
					c.sbIn = size
					c.setAIP(irqDataReady, c.makeStatus(false))
				}
			}
		} else { // CD-DA 扇區
			if c.driveStatus == dsSeekingLogical2 {
				if c.mode&modeCDDA != 0 {
					c.driveStatus = c.statusAfterSeek
					c.seekFinished = 1
					c.reportStartup = 24000000
				} else if c.seekRetry == 0 {
					c.holdLogicalPos = false
					c.driveStatus = dsStandby
					c.seekFinished = -1
				} else {
					c.seekRetry--
				}
			}
			if (c.driveStatus == dsReading && c.mode&modeCDDA != 0) || c.driveStatus == dsPlaying {
				if c.audio.read >= c.audio.size {
					c.enbufferizeCDDA(buf)
				}
			}
		}
	}

	c.disc.ReadSector(c.curSector, target)
	c.subQ = c.disc.SubQ(c.curSector)
	c.pipeSubQ[c.pipePos] = c.subQ

	if c.subQ[1] == 0xAA && (c.driveStatus == dsPlaying || (c.driveStatus == dsReading && c.subQ[0]&0x40 == 0 && c.mode&modeCDDA != 0)) {
		c.headerValid = false
		c.seekTarget = c.curSector
		c.driveStatus = dsPaused
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		c.setAIP(irqDataEnd, c.makeStatus(false))
		return
	}

	if c.driveStatus == dsPlaying {
		if c.mode&modeAutoPause != 0 {
			if c.playTrackMatch == -1 {
				c.playTrackMatch = int(c.subQ[1])
			}
			if c.playTrackMatch != -1 && int(c.subQ[1]) != c.playTrackMatch {
				c.setAIP(irqDataEnd, c.makeStatus(false))
				c.seekTarget = c.curSector
				c.driveStatus = dsPaused
				c.pipePos, c.pipeIn = 0, 0
				c.sectorsRead = 0
				c.psrCounter = 0
				return
			}
		}
		if c.mode&modeReport != 0 && c.reportStartup <= 0 && ((c.subQ[9]>>4) != c.reportLastF || c.forward || c.backward) {
			var tr [8]uint8
			tr[0] = c.makeStatus(false)
			tr[1] = c.subQ[1]
			tr[2] = c.subQ[2]
			if c.subQ[9]&0x10 != 0 {
				tr[3], tr[4], tr[5] = c.subQ[3], c.subQ[4]|0x80, c.subQ[5]
			} else {
				tr[3], tr[4], tr[5] = c.subQ[7], c.subQ[8], c.subQ[9]
			}
			// 峰值（tr[6..7]）需要音訊取樣，SPU 之前回 0。
			c.setAIP(irqDataReady, tr[:]...)
		}
		c.reportLastF = c.subQ[9] >> 4
	}

	c.pipePos = (c.pipePos + 1) % pipeCount
	c.pipeIn++
	c.psrCounter += c.sectorTime(true)

	if c.driveStatus == dsPlaying {
		switch {
		case c.forward:
			c.curSector += 12
		case c.backward:
			c.curSector -= 12
			if c.curSector < 0 {
				c.curSector = 0
			}
		default:
			c.curSector++
		}
	} else {
		c.curSector++
	}
	if c.driveStatus == dsPaused || c.driveStatus == dsStandby {
		add := int32(0)
		if c.holdLogicalPos {
			add = 2
		}
		if c.curSector >= c.seekTarget+add {
			c.curSector -= 9
			if c.curSector < -150 {
				c.curSector = -150
			}
		}
	} else {
		c.sectorsRead++
	}
}

func (c *CDROM) xaTest(buf *[disc.SectorSize]byte) bool {
	file, chan_, submode := buf[12+4], buf[12+5], buf[12+6]
	if c.mode&modeStrSnd == 0 || submode&0x04 == 0 {
		return false
	}
	if c.mode&modeSF != 0 && (file != c.filterFile || chan_ != c.filterChan) {
		return false
	}
	return true
}

// Clone 深拷貝控制器（光碟映像本身共用）。
func (c *CDROM) Clone() *CDROM {
	n := *c
	n.dmaBuf = append([]uint8(nil), c.dmaBuf...)
	return &n
}

// NextEvent 回傳最多再過多少週期，控制器狀態就可能改變（不含程式存取暫存器）；-1 表示沒有排定的事件。
// 機器在這之前不必呼叫 Tick（008 §2.5）。
func (c *CDROM) NextEvent() int64 {
	next := int64(-1)
	take := func(v int64) {
		if v > 0 && (next < 0 || v < next) {
			next = v
		}
	}
	take(c.psrCounter)
	take(c.pendingCounter)
	take(c.startupDelay)
	if c.irqBuffer&0xF == 0 {
		take(c.readyCounter)
	}
	if c.aipIRQ != 0 && c.readyCounter <= 0 {
		return 1
	}
	return next
}
