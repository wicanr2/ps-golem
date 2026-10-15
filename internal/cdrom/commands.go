package cdrom

// cmdArgs 回傳命令的參數個數範圍（Beetle cdc.c 的 Commands 表）。ok 為假表示不存在的命令。
func cmdArgs(cmd uint8) (min, max int, ok bool) {
	switch cmd {
	case 0x01, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0F, 0x10, 0x11, 0x13, 0x15, 0x16, 0x1A, 0x1B, 0x1C, 0x1E:
		return 0, 0, true
	case 0x02:
		return 3, 3, true
	case 0x03:
		return 0, 1, true
	case 0x0D, 0x1D:
		return 2, 2, true
	case 0x0E, 0x12, 0x14, 0x19:
		return 1, 1, true
	}
	return 0, 0, false
}

func (c *CDROM) ack() {
	c.writeResult(c.makeStatus(false))
	c.writeIRQ(irqAck)
}

func (c *CDROM) cmdError(code uint8) {
	c.writeResult(c.makeStatus(true))
	c.writeResult(code)
	c.writeIRQ(irqError)
}

func (c *CDROM) checkDisc() bool {
	if c.disc == nil || c.startupDelay > 0 {
		c.cmdError(errNotReady)
		return false
	}
	return true
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// calcSeekTime 照 Beetle；亂數 rand(0..25000) 取中點 12500。
func (c *CDROM) calcSeekTime(initial, target int32, motorOn, paused bool) int64 {
	var ret int64
	if !motorOn {
		initial = 0
		ret += cpuHz
	}
	calc := int64(abs32(initial-target)) * cpuHz * 1000 / (72 * 60 * 75) / 1000
	if calc < 20000 {
		calc = 20000
	}
	ret += calc
	d := abs32(initial - target)
	switch {
	case d >= 2250:
		ret += cpuHz * 300 / 1000
	case paused:
		if c.mode&modeSpeed != 0 {
			ret += 1237952
		} else {
			ret += 1237952 * 2
		}
	case d >= 3 && d < 12:
		ret += c.sectorTime(false) * 4
	}
	return ret + 12500
}

// preSeek 照 Beetle PS_CDC_PreSeekHack：位置直接設到目標，讀目標的 Sub-Q。
func (c *CDROM) preSeek(target int32) {
	c.curSector = target
	c.seekRetry = 128
	c.subQ = c.disc.SubQ(target)
}

func (c *CDROM) command(cmd uint8, args []uint8) int64 {
	switch cmd {
	case 0x01: // Nop（Getstat）
		c.ack()
	case 0x02: // Setloc
		if args[0]&0x0F > 9 || args[0] > 0x99 || args[1]&0x0F > 9 || args[1] > 0x59 || args[2]&0x0F > 9 || args[2] > 0x74 {
			c.cmdError(errBadArgVal)
			return 0
		}
		c.commandLoc = int32(bcdToU8(args[2])+75*bcdToU8(args[1])+75*60*bcdToU8(args[0])) - 150
		c.commandLocDirty = true
		c.ack()
	case 0x03: // Play
		return c.cmdPlay(args)
	case 0x04, 0x05: // Forward、Backward
		if !c.checkDisc() {
			return 0
		}
		c.ack()
		c.forward = cmd == 0x04
		c.backward = cmd == 0x05
	case 0x06, 0x1B: // ReadN、ReadS
		if c.checkDisc() {
			c.readBase()
		}
	case 0x07: // Standby（MotorOn）
		if !c.checkDisc() {
			return 0
		}
		if c.driveStatus != dsStopped {
			c.cmdError(0x20)
			return 0
		}
		c.ack()
		c.audio.clear()
		c.clearAIP()
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		c.seekTarget = c.curSector
		c.holdLogicalPos = false
		c.driveStatus = dsStandby
		return cpuHz * 100 / 1000
	case 0x08: // Stop
		if !c.checkDisc() {
			return 0
		}
		c.ack()
		if c.driveStatus == dsStopped {
			return 5000
		}
		c.audio.clear()
		c.clearAIP()
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		c.driveStatus = dsStopped
		c.headerValid = false
		return 33868
	case 0x09: // Pause
		if !c.checkDisc() {
			return 0
		}
		c.ack()
		if c.driveStatus == dsPaused || c.driveStatus == dsStopped {
			return 5000
		}
		sub := c.sectorsRead
		if sub > 4 {
			sub = 4
		}
		c.curSector -= int32(sub)
		c.sectorsRead = 0
		c.pipePos, c.pipeIn = 0, 0
		c.clearAIP()
		c.seekTarget = c.curSector
		c.driveStatus = dsPaused
		c.psrCounter = c.sectorTime(false)
		mul := int64(2)
		if c.mode&modeSpeed != 0 {
			mul = 1
		}
		// 亂數 rand(0..100000) 取中點。
		return (1124584+int64(c.curSector)*42596/(75*60))*mul + 50000
	case 0x0A: // Init（Beetle 稱 Reset）
		c.ack()
		c.muted = false
		c.mode = 0x20
		c.commandLoc = 0
		if c.disc != nil && c.startupDelay <= 0 {
			if (c.driveStatus == dsSeekingLogical || c.driveStatus == dsSeeking || c.driveStatus == dsSeekingLogical2) &&
				c.statusAfterSeek == dsPaused && c.seekTarget == 0 {
				return 256
			}
			c.audio.clear()
			c.sbIn = 0
			c.pipePos, c.pipeIn = 0, 0
			c.sectorsRead = 0
			c.seekTarget = 0
			st := c.calcSeekTime(c.curSector, c.seekTarget, c.driveStatus != dsStopped, c.driveStatus == dsPaused)
			rt := int64(1625000) // 亂數 rand(0..3250000) 取中點
			if rt > st {
				c.psrCounter = rt
			} else {
				c.psrCounter = st
			}
			c.preSeek(c.seekTarget)
			c.seekFinished = 0
			if c.holdLogicalPos {
				c.driveStatus = dsSeekingLogical
			} else {
				c.driveStatus = dsSeeking
			}
			c.statusAfterSeek = dsPaused
			c.clearAIP()
			return 4100000
		}
		c.seekFinished = 1
		return 70000
	case 0x0B:
		c.muted = true
		c.ack()
	case 0x0C:
		c.muted = false
		c.ack()
	case 0x0D: // Setfilter
		c.filterFile, c.filterChan = args[0], args[1]
		c.ack()
	case 0x0E: // Setmode
		c.mode = args[0]
		c.ack()
	case 0x0F: // Getparam
		c.writeResult(c.makeStatus(false))
		c.writeResult(c.mode)
		c.writeResult(0)
		c.writeResult(c.filterFile)
		c.writeResult(c.filterChan)
		c.writeIRQ(irqAck)
	case 0x10: // GetlocL
		if !c.checkDisc() {
			return 0
		}
		if !c.headerValid {
			c.cmdError(0x80)
			return 0
		}
		for i := 0; i < 8; i++ {
			c.writeResult(c.headerBuf[i])
		}
		c.writeIRQ(irqAck)
	case 0x11: // GetlocP
		if !c.checkDisc() {
			return 0
		}
		for _, i := range []int{1, 2, 3, 4, 5, 7, 8, 9} {
			c.writeResult(c.subQ[i])
		}
		c.writeIRQ(irqAck)
	case 0x12: // SetSession（Beetle ReadT）
		c.ack()
		return 44100 * 768 / 1000
	case 0x13: // GetTN
		if !c.checkDisc() {
			return 0
		}
		c.writeResult(c.makeStatus(false))
		c.writeResult(u8ToBCD(c.disc.Tracks[0].Number))
		c.writeResult(u8ToBCD(c.disc.Tracks[len(c.disc.Tracks)-1].Number))
		c.writeIRQ(irqAck)
	case 0x14: // GetTD
		if !c.checkDisc() {
			return 0
		}
		var lba int32
		if args[0] == 0 {
			lba = c.disc.LeadOut()
		} else {
			t := bcdToU8(args[0])
			first, last := c.disc.Tracks[0].Number, c.disc.Tracks[len(c.disc.Tracks)-1].Number
			if args[0]&0xF > 9 || args[0]>>4 > 9 || t < first || t > last {
				c.cmdError(errBadArgVal)
				return 0
			}
			lba = c.disc.Tracks[t-first].Index1
		}
		a := lba + 150
		c.writeResult(c.makeStatus(false))
		c.writeResult(u8ToBCD(int(a / (60 * 75))))
		c.writeResult(u8ToBCD(int(a / 75 % 60)))
		c.writeIRQ(irqAck)
	case 0x15, 0x16: // SeekL、SeekP
		if !c.checkDisc() {
			return 0
		}
		c.ack()
		c.seekTarget = c.commandLoc
		if cmd == 0x15 {
			c.sectorsRead = 0
			c.pipePos, c.pipeIn = 0, 0
		} else {
			c.headerValid = false
		}
		c.psrCounter = c.calcSeekTime(c.curSector, c.seekTarget, c.driveStatus != dsStopped, c.driveStatus == dsPaused)
		c.preSeek(c.seekTarget)
		c.seekFinished = 0
		if cmd == 0x15 {
			c.driveStatus = dsSeekingLogical
		} else {
			c.driveStatus = dsSeeking
		}
		c.statusAfterSeek = dsStandby
		c.clearAIP()
		return c.psrCounter
	case 0x19:
		c.cmdTest(args[0])
	case 0x1A: // GetID
		if !c.checkDisc() {
			return 0
		}
		c.ack()
		return 33868
	case 0x1C: // Beetle 的 Init：不回應
	case 0x1D:
		c.ack()
	case 0x1E: // ReadTOC
		c.headerValid = false
		c.ack()
		if !c.checkDisc() {
			return 26000
		}
		t := 30000000 + c.calcSeekTime(c.curSector, 0, c.driveStatus != dsStopped, c.driveStatus == dsPaused)
		c.seekTarget = 0
		c.holdLogicalPos = false
		c.driveStatus = dsPaused
		c.clearAIP()
		return t
	}
	return 0
}

func (c *CDROM) commandPart2(cmd uint8) int64 {
	switch cmd {
	case 0x07, 0x08: // Standby、Stop
		c.psrCounter = 0
		c.writeResult(c.makeStatus(false))
		c.writeIRQ(irqComplete)
	case 0x09, 0x12, 0x1E: // Pause、ReadT、ReadTOC
		c.writeResult(c.makeStatus(false))
		c.writeIRQ(irqComplete)
	case 0x0A, 0x15, 0x16: // Init、SeekL、SeekP
		if c.driveStatus != dsSeeking && c.driveStatus != dsSeekingLogical && c.driveStatus != dsSeekingLogical2 {
			if c.seekFinished != 0 {
				c.beginResults()
				if c.seekFinished < 0 {
					c.writeResult(c.makeStatus(false) | 0x04)
					c.writeResult(0x04)
					c.writeIRQ(irqError)
				} else {
					c.writeResult(c.makeStatus(false))
					c.writeIRQ(irqComplete)
				}
			}
			return 0
		}
		if c.psrCounter > 256 {
			return c.psrCounter
		}
		return 256
	case 0x1A: // GetID
		if c.isPSXDisc {
			c.writeResult(c.makeStatus(false))
			c.writeResult(0x00)
			c.writeResult(0x20)
			c.writeResult(0x00)
			for _, b := range c.discID {
				c.writeResult(b)
			}
			c.writeIRQ(irqComplete)
		} else {
			c.writeResult(c.makeStatus(false) | 0x08)
			c.writeResult(0x90)
			c.writeResult(0x00)
			c.writeResult(0x00)
			c.writeResult(0xFF)
			c.writeResult(0)
			c.writeResult(0)
			c.writeResult(0)
			c.writeIRQ(irqError)
		}
	}
	return 0
}

func (c *CDROM) cmdPlay(args []uint8) int64 {
	if !c.checkDisc() {
		return 0
	}
	c.clearAIP()
	c.ack()
	c.forward, c.backward = false, false
	if len(args) > 0 && args[0] != 0 {
		t := bcdToU8(args[0])
		first, last := c.disc.Tracks[0].Number, c.disc.Tracks[len(c.disc.Tracks)-1].Number
		if t < first {
			t = first
		} else if t > last {
			t = last
		}
		c.audio.clear()
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		c.playTrackMatch = t
		c.seekTarget = c.disc.Tracks[t-first].Index1
		c.startSeekPlay()
	} else if c.commandLocDirty || c.driveStatus != dsPlaying {
		c.audio.clear()
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		if c.commandLocDirty {
			c.seekTarget = c.commandLoc
		} else {
			c.seekTarget = c.curSector
		}
		c.playTrackMatch = -1
		c.startSeekPlay()
	}
	c.commandLocDirty = false
	return 0
}

func (c *CDROM) startSeekPlay() {
	c.psrCounter = c.calcSeekTime(c.curSector, c.seekTarget, c.driveStatus != dsStopped, c.driveStatus == dsPaused)
	c.headerValid = false
	c.preSeek(c.seekTarget)
	c.reportLastF = 0xFF
	c.seekFinished = 0
	c.driveStatus = dsSeeking
	c.statusAfterSeek = dsPlaying
}

func (c *CDROM) readBase() {
	if !c.isPSXDisc {
		c.cmdError(errBadCommand)
		return
	}
	c.ack()
	if (c.driveStatus == dsSeekingLogical || c.driveStatus == dsSeekingLogical2) && c.seekTarget == c.commandLoc && c.statusAfterSeek == dsReading {
		c.commandLocDirty = false
		return
	}
	if c.commandLocDirty || c.driveStatus != dsReading {
		c.clearAIP()
		c.audio.clear()
		c.sbIn = 0
		c.pipePos, c.pipeIn = 0, 0
		c.sectorsRead = 0
		if c.commandLocDirty {
			c.seekTarget = c.commandLoc
		} else if c.driveStatus != dsPaused && c.driveStatus != dsStandby {
			c.seekTarget = c.curSector
		}
		c.psrCounter = c.sectorTime(false) + c.calcSeekTime(c.curSector, c.seekTarget, c.driveStatus != dsStopped, c.driveStatus == dsPaused)
		c.headerValid = false
		c.preSeek(c.seekTarget)
		c.seekFinished = 0
		c.driveStatus = dsSeekingLogical
		c.statusAfterSeek = dsReading
	}
	c.commandLocDirty = false
}

// cmdTest 是命令 19h 的子函式（Beetle PS_CDC_Command_Test）。
func (c *CDROM) cmdTest(sub uint8) {
	put := func(b ...uint8) {
		for _, v := range b {
			c.writeResult(v)
		}
		c.writeIRQ(irqAck)
	}
	switch {
	case sub <= 0x03 || (sub >= 0x10 && sub <= 0x1A):
		c.ack()
	case sub == 0x04:
		c.ack()
	case sub == 0x05:
		put(0x00, 0x00)
	case sub == 0x20:
		put(0x97, 0x01, 0x10, 0xC2)
	case sub == 0x21:
		put(0x01)
	case sub == 0x22:
		put([]uint8("for U/C")...)
	case sub == 0x23, sub == 0x24:
		put([]uint8("CXD2545Q")...)
	case sub == 0x25:
		put([]uint8("CXD1815Q")...)
	default:
		c.cmdError(0x10)
	}
}
