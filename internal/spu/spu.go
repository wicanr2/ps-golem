// Package spu 是 PS1 的聲音處理器（docs/spec/013-spu-and-cd-audio.md）。
//
// 移植自 beetle-psx-libretro mednafen/psx/spu.c、spu.h
// （commit e57860d82faa953dc0fc10f201fd583739bc2f7d，GPL-2.0-or-later），只取純 C 路徑
// （混響重取樣不取 SSE2／NEON 分支）。Beetle 的設定項取預設值：spu_samples = 1；
// psx_spu_silent_voice_opt 這個捷徑不做，每個 voice 每個取樣都照完整路徑跑。
// Beetle 呼叫 IRQ_Assert(IRQ_SPU, ...) 的地方只更新 irqAsserted，由 IRQ() 讀出；
// CDC_GetCDAudioSample 改成 Advance 傳入的 CDAudio。
package spu

// ADSR 階段（Beetle spu.h）。
const (
	adsrAttack  = 0
	adsrDecay   = 1
	adsrSustain = 2
	adsrRelease = 3
)

// 每組取樣的 CPU 週期數：33868800 ÷ 44100（Beetle spu_samples*768，spu_samples = 1）。
const clocksPerSample = 768

// 取樣緩衝的容量（Beetle IntermediateBuffer[4096][2]）。
const bufferFrames = 4096

// CDAudio 提供 CD 音訊取樣（Beetle CDC_GetCDAudioSample）。值已夾在 -32768..32767。
type CDAudio interface{ CDAudioSample() (l, r int32) }

// adsr 對應 Beetle SPU_ADSR。
type adsr struct {
	envLevel uint16 // Beetle 在多處轉成 int16 用；存 uint16 以免有號溢位
	divider  uint32
	phase    uint32

	attackExp  bool
	sustainExp bool
	sustainDec bool
	releaseExp bool

	attackRate   int32 // Ar
	decayRate    int32 // Dr * 4
	sustainRate  int32 // Sr
	releaseRate  int32 // Rr * 4
	sustainLevel int32 // (Sl + 1) << 11
}

// sweep 對應 Beetle SPU_Sweep：音量與音量掃描。
type sweep struct {
	control uint16
	current uint16
	divider uint32
}

// voice 對應 Beetle SPU_Voice。
type voice struct {
	decodeBuffer [0x20]int16
	decodeM2     int16
	decodeM1     int16

	decodePlayDelay uint32
	decodeWritePos  uint32
	decodeReadPos   uint32
	decodeAvail     uint32

	ignoreSampLA bool

	decodeShift  uint8
	decodeWeight uint8
	decodeFlags  uint8

	sweep [2]sweep

	pitch    uint16
	curPhase uint32

	startAddr   uint32
	curAddr     uint32
	adsrControl uint32
	loopAddr    uint32

	preLRSample int32 // 套過包絡、還沒套左右音量。範圍 -32768..32767

	adsr adsr
}

// 暫存器區塊 regs 的索引（Beetle SPU_RegBlock 這個 union 的具名欄位）。
// regs[i] 對應位址 0x1F801C00 + i*2。
const (
	regSPUStatus = 0xD7 // global.s.SPUStatus（1F801DAE）
	regD6        = 0xD6 // Regs[0xD6]（1F801DAC）

	rvFBSrcA   = 0xE0
	rvFBSrcB   = 0xE1
	rvIIRAlpha = 0xE2
	rvACCCoefA = 0xE3
	rvACCCoefB = 0xE4
	rvACCCoefC = 0xE5
	rvACCCoefD = 0xE6
	rvIIRCoef  = 0xE7
	rvFBAlpha  = 0xE8
	rvFBX      = 0xE9
	rvIIRDestA = 0xEA // [2]
	rvACCSrcA  = 0xEC // [2]
	rvACCSrcB  = 0xEE // [2]
	rvIIRSrcA  = 0xF0 // [2]
	rvIIRDestB = 0xF2 // [2]
	rvACCSrcC  = 0xF4 // [2]
	rvACCSrcD  = 0xF6 // [2]
	rvIIRSrcB  = 0xF8 // [2]
	rvMixDestA = 0xFA // [2]
	rvMixDestB = 0xFC // [2]
	rvInCoef   = 0xFE // [2]，int16
)

// SPU 不含指標、slice、map、函式值或 interface，可直接複製（Machine.Clone）。
type SPU struct {
	voices [24]voice

	noiseDivider uint32
	noiseCounter uint32
	lfsr         uint16

	fmMode     uint32
	noiseMode  uint32
	reverbMode uint32

	reverbWA uint32

	globalSweep [2]sweep // 不影響混響音量

	reverbVol [2]int32
	cdVol     [2]int32
	externVol [2]int32

	irqAddr uint32
	rwAddr  uint32

	spuControl uint16

	voiceOn  uint32
	voiceOff uint32
	blockEnd uint32

	cwa uint32

	regs    [0x100]uint16
	auxRegs [0x10]uint16

	rdsb      [2][128]int16
	rusb      [2][64]int16
	rvbResPos int32

	reverbCur uint32

	irqAsserted bool

	clockDivider int32

	ram [524288 / 2]uint16

	buf    [bufferFrames][2]int16 // Beetle IntermediateBuffer
	bufPos uint32                 // Beetle IntermediateBufferPos
}

// New 回傳上電狀態的 SPU（Beetle SPU_Init 之後 SPU_Power）。
func New() *SPU {
	s := &SPU{} // SPU_Init：全部清零
	s.power()
	return s
}

// power 對應 Beetle SPU_Power。取樣緩衝不動（Beetle 也只在 SPU_Init 清）。
func (s *SPU) power() {
	buf, bufPos := s.buf, s.bufPos
	*s = SPU{}
	s.buf, s.bufPos = buf, bufPos
	s.clockDivider = clocksPerSample
	s.reverbCur = s.reverbWA
}

// IRQ 回傳 SPU 中斷線的狀態（Beetle IRQAsserted）。
func (s *SPU) IRQ() bool { return s.irqAsserted }

// IRQEnabled 回傳 SPUCNT bit 6（中斷開啟）；機器據此決定同步期限（013 §3）。
func (s *SPU) IRQEnabled() bool { return s.spuControl&0x40 != 0 }

// TakeSamples 把累積的立體聲取樣（L、R 交錯）接到 dst 後面並清空緩衝。
func (s *SPU) TakeSamples(dst []int16) []int16 {
	for i := uint32(0); i < s.bufPos; i++ {
		dst = append(dst, s.buf[i][0], s.buf[i][1])
	}
	s.bufPos = 0
	return dst
}

// calcVCDelta 對應 Beetle CalcVCDelta：算出包絡或音量掃描這一步的增量與除頻器增量。
func calcVCDelta(zs uint8, speed uint8, logMode, decMode, invIncrement bool, current int16) (increment, divinco int32) {
	increment = 7 - int32(speed&0x3)

	if invIncrement {
		increment = ^increment
	}

	divinco = 32768

	if speed < 0x2C {
		increment = int32(uint32(increment) << ((0x2F - uint32(speed)) >> 2))
	}

	if speed >= 0x30 {
		divinco >>= (uint32(speed) - 0x2C) >> 2
	}

	if logMode {
		if decMode { // 對數遞減
			increment = (int32(current) * increment) >> 15
		} else { // 對數遞增
			if (int32(current) & 0x7FFF) >= 0x6000 {
				if speed < 0x28 {
					increment >>= 2
				} else if speed >= 0x2C {
					divinco >>= 2
				} else { // 0x28 ... 0x2B
					increment >>= 1
					divinco >>= 1
				}
			}
		}
	}

	if divinco == 0 && speed < zs {
		divinco = 1
	}
	return
}

func b2u16(b bool) uint16 {
	if b {
		return 0xFFFF
	}
	return 0
}

// clock 對應 Beetle SPU_Sweep_Clock。
func (sw *sweep) clock() {
	logMode := sw.control&0x4000 != 0
	decMode := sw.control&0x2000 != 0
	invMode := sw.control&0x1000 != 0
	invIncrement := (decMode != invMode) || (decMode && logMode)
	vcCvXor := b2u16(invMode && !(decMode && logMode))
	testInvert := b2u16(invMode)

	increment, divinco := calcVCDelta(0x7F, uint8(sw.control&0x7F), logMode, decMode, invIncrement, int16(sw.current^vcCvXor))

	var signWant uint16 = 0x8000
	if invMode {
		signWant = 0x0000
	}
	if (decMode && !(invMode && logMode)) && ((sw.current&0x8000) == signWant || sw.current == 0) {
		// Beetle 註：不確定這個條件該停住除頻器，還是把增量歸零。
		sw.current = 0
	} else {
		sw.divider += uint32(divinco)

		if sw.divider&0x8000 != 0 {
			sw.divider = 0

			if decMode || (sw.current^testInvert) != 0x7FFF {
				prev := sw.current
				sw.current = uint16(int32(sw.current) + increment)

				if !decMode && (sw.current^prev)&0x8000 != 0 && (sw.current^testInvert)&0x8000 != 0 {
					sw.current = 0x7FFF ^ testInvert
				}
			}
		}
	}
}

// run 是取樣迴圈裡的「Run sweep」：bit 15 開時跑掃描，否則音量 = (Control & 0x7FFF) << 1。
func (sw *sweep) run() {
	if sw.control&0x8000 != 0 {
		sw.clock()
	} else {
		sw.current = (sw.control & 0x7FFF) << 1
	}
}

// adpcmWeights 是 ADPCM 濾波係數（s-1、s-2）。5 到 0xF 在實機上看起來都是 0。
var adpcmWeights = [16][2]int32{
	{0, 0},
	{60, 0},
	{115, -52},
	{98, -55},
	{122, -60},
}

func (s *SPU) assertIRQ() {
	s.irqAsserted = true
}

// runDecoder 對應 Beetle SPU_RunDecoder。注意不要在下一個區塊開始解碼之前就觸發它的 SPU IRQ。
func (s *SPU) runDecoder(v *voice, vn int) {
	if v.decodeAvail >= 11 {
		if s.spuControl&0x40 != 0 {
			testAddr := (v.curAddr - 1) & 0x3FFFF
			if s.irqAddr == testAddr || s.irqAddr == (testAddr&0x3FFF8) {
				s.assertIRQ()
			}
		}
		return
	}

	if v.curAddr&0x7 == 0 {
		// 處理上一個區塊延後的旗標。Beetle 註：設 BlockEnd 與強制包絡歸零的時機略晚。
		if v.decodeFlags&0x1 != 0 {
			v.curAddr = v.loopAddr &^ 0x7

			s.blockEnd |= 1 << uint(vn)

			if v.decodeFlags&0x2 == 0 { // 不循環就把包絡強制歸零
				if s.noiseMode&(1<<uint(vn)) == 0 {
					v.adsr.phase = adsrRelease
					v.adsr.envLevel = 0
				}
			}
		}
	}

	if s.spuControl&0x40 != 0 {
		testAddr := v.curAddr & 0x3FFFF
		if s.irqAddr == testAddr || s.irqAddr == (testAddr&0x3FFF8) {
			s.assertIRQ()
		}
	}

	if v.curAddr&0x7 == 0 {
		cv := s.ram[v.curAddr]
		v.decodeShift = uint8(cv & 0xF)
		v.decodeWeight = uint8((cv >> 4) & 0xF)
		v.decodeFlags = uint8((cv >> 8) & 0xFF)

		if v.decodeFlags&0x4 != 0 {
			if !v.ignoreSampLA {
				v.loopAddr = v.curAddr
			}
		}
		v.curAddr = (v.curAddr + 1) & 0x3FFFF
	}

	// 不能寫成 else：DecodeAvail < 11 時每次都要解 4 個取樣，否則高音高播放會壞掉。
	weightM1 := adpcmWeights[v.decodeWeight][0]
	weightM2 := adpcmWeights[v.decodeWeight][1]

	cv := s.ram[v.curAddr]
	shift := uint32(v.decodeShift)

	// shift > 12 是保留值。Beetle 夾成 8 並把每個 nibble 遮到只剩符號位元（CV &= 0x8888），
	// 與 psx-spx 記載的「同 shift=9」不同；Beetle 註明刻意保留，照 Beetle。
	if shift > 12 {
		shift = 8
		cv &= 0x8888
	}

	coded := uint32(cv) << 12
	wp := v.decodeWritePos

	for i := uint32(0); i < 4; i++ {
		sample := int32(int16(uint16(coded&0xF000))) >> shift

		sample += (int32(v.decodeM2) * weightM2) >> 6
		sample += (int32(v.decodeM1) * weightM1) >> 6

		if sample < -32768 {
			sample = -32768
		} else if sample > 32767 {
			sample = 32767
		}

		v.decodeBuffer[wp+i] = int16(sample)
		v.decodeM2 = v.decodeM1
		v.decodeM1 = int16(sample)
		coded >>= 4
	}
	v.decodeWritePos = (v.decodeWritePos + 4) & 0x1F
	v.decodeAvail += 4
	v.curAddr = (v.curAddr + 1) & 0x3FFFF
}

// cacheEnvelope 對應 Beetle SPU_CacheEnvelope：把 ADSR 暫存器拆成各欄位。
func (v *voice) cacheEnvelope() {
	raw := v.adsrControl
	a := &v.adsr
	sl := int32(raw & 0x0F)
	dr := int32((raw >> 4) & 0x0F)
	ar := int32((raw >> 8) & 0x7F)

	rr := int32((raw >> 16) & 0x1F)
	sr := int32((raw >> 22) & 0x7F)

	a.attackExp = raw&(1<<15) != 0
	a.releaseExp = raw&(1<<21) != 0
	a.sustainExp = raw&(1<<31) != 0
	a.sustainDec = raw&(1<<30) != 0

	a.attackRate = ar
	a.decayRate = dr << 2
	a.sustainRate = sr
	a.releaseRate = rr << 2

	a.sustainLevel = (sl + 1) << 11
}

// resetEnvelope 對應 Beetle SPU_ResetEnvelope。
func (v *voice) resetEnvelope() {
	v.adsr.envLevel = 0
	v.adsr.divider = 0
	v.adsr.phase = adsrAttack
}

// releaseEnvelope 對應 Beetle SPU_ReleaseEnvelope。
func (v *voice) releaseEnvelope() {
	v.adsr.divider = 0
	v.adsr.phase = adsrRelease
}

// runEnvelope 對應 Beetle SPU_RunEnvelope。
func (v *voice) runEnvelope() {
	a := &v.adsr
	var increment, divinco int32
	var uoflowReset int16

	if a.phase == adsrAttack && a.envLevel == 0x7FFF {
		a.phase++
	}

	switch a.phase {
	case adsrAttack:
		increment, divinco = calcVCDelta(0x7F, uint8(a.attackRate), a.attackExp, false, false, int16(a.envLevel))
		uoflowReset = 0x7FFF
	case adsrDecay:
		increment, divinco = calcVCDelta(0x1F<<2, uint8(a.decayRate), true, true, true, int16(a.envLevel))
		uoflowReset = 0
	case adsrSustain:
		increment, divinco = calcVCDelta(0x7F, uint8(a.sustainRate), a.sustainExp, a.sustainDec, a.sustainDec, int16(a.envLevel))
		if a.sustainDec {
			uoflowReset = 0
		} else {
			uoflowReset = 0x7FFF
		}
	case adsrRelease:
		increment, divinco = calcVCDelta(0x1F<<2, uint8(a.releaseRate), a.releaseExp, true, true, int16(a.envLevel))
		uoflowReset = 0
	default:
		// Beetle 是 assert(0)；NDEBUG 下增量都是 0。
	}

	a.divider += uint32(divinco)
	if a.divider&0x8000 != 0 {
		prev := a.envLevel

		a.divider = 0
		a.envLevel = uint16(int32(a.envLevel) + increment)

		if a.phase == adsrAttack {
			// 上一次最高位元是 0、這次變 1，就是溢位。
			if ((prev^a.envLevel)&a.envLevel)&0x8000 != 0 {
				a.envLevel = uint16(uoflowReset)
			}
		} else {
			if a.envLevel&0x8000 != 0 {
				a.envLevel = uint16(uoflowReset)
			}
		}
		if a.phase == adsrDecay && int32(a.envLevel) < a.sustainLevel {
			a.phase++
		}
	}
}

// checkIRQAddr 對應 Beetle SPU_CheckIRQAddr。
func (s *SPU) checkIRQAddr(addr uint32) {
	if s.spuControl&0x40 != 0 {
		if s.irqAddr != addr {
			return
		}
		s.assertIRQ()
	}
}

func (s *SPU) writeRAM(addr uint32, v uint16) {
	s.checkIRQAddr(addr)
	s.ram[addr] = v
}

func (s *SPU) readRAM(addr uint32) uint16 {
	s.checkIRQAddr(addr)
	return s.ram[addr]
}

func reverbSat(samp int32) int16 {
	if samp > 32767 {
		samp = 32767
	} else if samp < -32768 {
		samp = -32768
	}
	return int16(samp)
}

// reverbNeg 對應 Beetle REVERB_NEG。
func reverbNeg(samp int16) int32 {
	if samp == -32768 {
		return 0x7FFF
	}
	return -int32(samp)
}

// reverbOffset 對應 Beetle SPU_Get_Reverb_Offset：超出 SPU RAM 就繞回混響工作區開頭。
func (s *SPU) reverbOffset(inOffset uint32) uint32 {
	offset := s.reverbCur + (inOffset & 0x3FFFF)
	offset += s.reverbWA & (0 - ((offset >> 18) & 1))
	offset &= 0x3FFFF
	return offset
}

// rdRvb 對應 Beetle SPU_RD_RVB。rawOffs 在 C 是 uint16_t 參數，呼叫端的差值會先截成 16 位元。
func (s *SPU) rdRvb(rawOffs uint16, extraOffs int32) int16 {
	return int16(s.readRAM(s.reverbOffset(uint32(int32(rawOffs)<<2 + extraOffs))))
}

// wrRvb 對應 Beetle SPU_WR_RVB：SPUCNT bit 7 開才寫。
func (s *SPU) wrRvb(rawOffs uint16, sample int16) {
	if s.spuControl&0x80 != 0 {
		s.writeRAM(s.reverbOffset(uint32(rawOffs)<<2), uint16(sample))
	}
}

// resampTable 是混響重取樣 FIR（去掉零與中間的 16384）。
var resampTable = [20]int32{
	-1, 2, -10, 35, -103, 266, -616, 1332, -2960, 10246, 10246, -2960, 1332, -616, 266, -103, 35, -10, 2, -1,
}

func clamp16(v int32) int32 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return v
}

// reverb4422 對應 Beetle Reverb4422 的純 C 版：44100 → 22050 降取樣。src 從 off 開始讀 39 筆。
func reverb4422(src *[128]int16, off int32) int32 {
	var out int32
	for i := int32(0); i < 20; i++ {
		out += resampTable[i] * int32(src[off+i*2])
	}
	out += 0x4000 * int32(src[off+19]) // 中間不為零的係數
	out >>= 15
	return clamp16(out)
}

// reverb2244 對應 Beetle Reverb2244 的純 C 版：22050 → 44100 升取樣。src 從 off 開始讀 20 筆。
func reverb2244(src *[64]int16, off int32) int32 {
	var out int32
	for i := int32(0); i < 20; i++ {
		out += resampTable[i] * int32(src[off+i])
	}
	out >>= 14
	return clamp16(out)
}

// iiasm 對應 Beetle IIASM。
func iiasm(alpha, insamp int16) int32 {
	if alpha == -32768 {
		if insamp == -32768 {
			return 0
		}
		return int32(insamp) * -65536
	}
	return int32(insamp) * (32768 - int32(alpha))
}

func (s *SPU) rs(i int) int16 { return int16(s.regs[i]) }

// runReverbChannel 對應 Beetle SPU_RunReverbChannel：對一個聲道（lr）跑梳狀濾波，
// 結果寫進該聲道升取樣環形緩衝的 RvbResPos 位置。SPU RAM 的讀寫順序照 Beetle。
func (s *SPU) runReverbChannel(lr int, downsampled int32) {
	r := &s.regs
	iirCoef := int32(s.rs(rvIIRCoef))
	inCoef := int32(s.rs(rvInCoef + lr))
	iirAlpha := s.rs(rvIIRAlpha)

	iirInputA := reverbSat((((int32(s.rdRvb(r[rvIIRSrcA+(lr^0)], 0)) * iirCoef) >> 14) + ((downsampled * inCoef) >> 14)) >> 1)
	iirInputB := reverbSat((((int32(s.rdRvb(r[rvIIRSrcB+(lr^1)], 0)) * iirCoef) >> 14) + ((downsampled * inCoef) >> 14)) >> 1)
	iirA := reverbSat((((int32(iirInputA) * int32(iirAlpha)) >> 14) + (iiasm(iirAlpha, s.rdRvb(r[rvIIRDestA+lr], -1)) >> 14)) >> 1)
	iirB := reverbSat((((int32(iirInputB) * int32(iirAlpha)) >> 14) + (iiasm(iirAlpha, s.rdRvb(r[rvIIRDestB+lr], -1)) >> 14)) >> 1)

	s.wrRvb(r[rvIIRDestA+lr], iirA)
	s.wrRvb(r[rvIIRDestB+lr], iirB)

	acc := ((int32(s.rdRvb(r[rvACCSrcA+lr], 0)) * int32(s.rs(rvACCCoefA))) >> 14) +
		((int32(s.rdRvb(r[rvACCSrcB+lr], 0)) * int32(s.rs(rvACCCoefB))) >> 14) +
		((int32(s.rdRvb(r[rvACCSrcC+lr], 0)) * int32(s.rs(rvACCCoefC))) >> 14) +
		((int32(s.rdRvb(r[rvACCSrcD+lr], 0)) * int32(s.rs(rvACCCoefD))) >> 14)

	// C 的 uint16 相減會升成 int，傳給 uint16_t 參數時再截回 16 位元。
	fbA := int32(s.rdRvb(r[rvMixDestA+lr]-r[rvFBSrcA], 0))
	fbB := int32(s.rdRvb(r[rvMixDestB+lr]-r[rvFBSrcB], 0))
	fbAlpha := s.rs(rvFBAlpha)
	fbX := s.rs(rvFBX)
	mda := reverbSat((acc + ((fbA * reverbNeg(fbAlpha)) >> 14)) >> 1)
	mdb := reverbSat(fbA + ((((int32(mda) * int32(fbAlpha)) >> 14) + ((fbB * reverbNeg(fbX)) >> 14)) >> 1))
	ivb := reverbSat(fbB + ((int32(mdb) * int32(fbX)) >> 15))

	s.wrRvb(r[rvMixDestA+lr], mda)
	s.wrRvb(r[rvMixDestB+lr], mdb)

	p := s.rvbResPos >> 1
	s.rusb[lr][p|0x20] = ivb // 輸出取樣
	s.rusb[lr][p] = ivb
}

// runReverb 對應 Beetle SPU_RunReverb。左聲道在奇數週期算、右聲道在下一個偶數週期算，
// 兩者共用 ReverbCur，右聲道算完才前進。
func (s *SPU) runReverb(in [2]int32) (out [2]int32) {
	for lr := 0; lr < 2; lr++ {
		s.rdsb[lr][s.rvbResPos|0x00] = int16(in[lr])
		s.rdsb[lr][s.rvbResPos|0x40] = int16(in[lr]) // 讓 MAC 迴圈不用遮罩或檢查邊界
	}

	if s.rvbResPos&1 != 0 {
		downsampled := reverb4422(&s.rdsb[0], (s.rvbResPos-38)&0x3F)
		s.runReverbChannel(0, downsampled)
	} else {
		downsampled := reverb4422(&s.rdsb[1], (s.rvbResPos-38)&0x3F)
		s.runReverbChannel(1, downsampled)

		s.reverbCur = (s.reverbCur + 1) & 0x3FFFF
		if s.reverbCur == 0 {
			s.reverbCur = s.reverbWA
		}
	}

	// 每個週期把兩聲道升回 44100 Hz：自己那一拍跑內插 FIR，另一拍取中間係數。
	src := ((s.rvbResPos >> 1) - 19) & 0x1F
	if s.rvbResPos&1 != 0 {
		out[0] = reverb2244(&s.rusb[0], src)
		out[1] = int32(s.rusb[1][src+9])
	} else {
		out[0] = int32(s.rusb[0][src+9])
		out[1] = reverb2244(&s.rusb[1], src)
	}

	s.rvbResPos = (s.rvbResPos + 1) & 0x3F
	return
}

// runNoise 對應 Beetle SPU_RunNoise。
func (s *SPU) runNoise() {
	rf := uint32(s.spuControl>>8) & 0x3F
	dividerInc := uint32(2) << (rf >> 2)
	counterInc := 4 + (rf & 0x3)

	if rf >= 0x3C {
		dividerInc = 0x8000
		counterInc = 8
	}

	s.noiseDivider += dividerInc

	if s.noiseDivider&0x8000 != 0 {
		s.noiseDivider = 0

		s.noiseCounter += counterInc

		if s.noiseCounter&0x8 != 0 {
			s.noiseCounter &= 0x7
			l := s.lfsr
			s.lfsr = (l << 1) | (((l >> 15) ^ (l >> 12) ^ (l >> 11) ^ (l >> 10) ^ 1) & 1)
		}
	}
}

// Advance 推進 clocks 個 CPU 週期，回傳到下一個取樣點的週期數（Beetle SPU_UpdateFromCDC）。
// cd 可以是 nil，此時 CD 音訊取樣視為 0。
func (s *SPU) Advance(clocks int64, cd CDAudio) int64 {
	sampleClocks := 0

	div := int64(s.clockDivider) - clocks
	for div <= 0 {
		div += clocksPerSample
		sampleClocks++
	}
	s.clockDivider = int32(div)

	for ; sampleClocks > 0; sampleClocks-- {
		s.runSample(cd)
	}

	return int64(s.clockDivider)
}

// runSample 產生一組立體聲取樣（Beetle SPU_UpdateFromCDC 的 while(sample_clocks > 0) 本體）。
func (s *SPU) runSample(cd CDAudio) {
	var accum, accumFV [2]int32 // [0] 左、[1] 右；accumFV 是混響輸入

	phaseModCache := s.fmMode &^ 1

	// 1F801DAE（SPUSTAT）：低 6 位元延後複製 SPUCNT，bit 6 是中斷狀態。
	st := s.spuControl & 0x3F
	if s.irqAsserted {
		st |= 0x40
	}
	if s.regs[regD6] == 0x4 { // Beetle TODO：0x2C 暫存器待查
		if s.cwa&0x100 != 0 {
			st |= 0x800
		}
	}
	s.regs[regSPUStatus] = st

	for vn := 0; vn < 24; vn++ {
		v := &s.voices[vn]
		bit := uint32(1) << uint(vn)

		v.preLRSample = 0

		if v.decodePlayDelay != 0 {
			v.ignoreSampLA = false
		}

		// 需要的話解新取樣。
		s.runDecoder(v, vn)

		var pvs int32
		if s.noiseMode&bit != 0 {
			pvs = int32(int16(s.lfsr))
		} else {
			si := v.decodeReadPos
			pi := (v.curPhase & 0xFFF) >> 4
			f := &firTable[pi]

			pvs = (int32(v.decodeBuffer[(si+0)&0x1F])*int32(f[0]) +
				int32(v.decodeBuffer[(si+1)&0x1F])*int32(f[1]) +
				int32(v.decodeBuffer[(si+2)&0x1F])*int32(f[2]) +
				int32(v.decodeBuffer[(si+3)&0x1F])*int32(f[3])) >> 15
		}

		pvs = (pvs * int32(int16(v.adsr.envLevel))) >> 15
		v.preLRSample = pvs

		if vn == 1 || vn == 3 {
			index := uint32(vn >> 1)
			s.writeRAM(0x400|(index*0x200)|s.cwa, uint16(pvs))
		}

		l := (pvs * int32(int16(v.sweep[0].current))) >> 15
		r := (pvs * int32(int16(v.sweep[1].current))) >> 15

		accum[0] += l
		accum[1] += r

		if s.reverbMode&bit != 0 {
			accumFV[0] += l
			accumFV[1] += r
		}

		v.sweep[0].run()
		v.sweep[1].run()

		if v.decodePlayDelay == 0 {
			v.runEnvelope()

			var phaseInc uint32
			if phaseModCache&bit != 0 {
				// 舊公式 (Pitch * (prev->PreLRSample + 0x8000)) >> 15 處理不了 Pitch >= 0x8000。
				// C 的 unsigned 指定：負值會變成很大的數，再被下面夾成 0x3FFF。
				phaseInc = uint32(int32(v.pitch) + ((int32(int16(v.pitch)) * s.voices[vn-1].preLRSample) >> 15))
			} else {
				phaseInc = uint32(v.pitch)
			}

			if phaseInc > 0x3FFF {
				phaseInc = 0x3FFF
			}

			tmp := v.curPhase + phaseInc
			used := tmp >> 12

			v.curPhase = tmp & 0xFFF
			v.decodeAvail -= used
			v.decodeReadPos = (v.decodeReadPos + used) & 0x1F
		} else {
			v.decodePlayDelay--
		}

		if s.voiceOff&bit != 0 {
			if v.adsr.phase != adsrRelease {
				// Beetle 註：延遲期間的 key off 只在 DecodePlayDelay < 3 時生效（保守作法）。
				if v.decodePlayDelay < 3 {
					v.releaseEnvelope()
				}
			}
		}

		if s.voiceOn&bit != 0 {
			v.resetEnvelope()

			v.decodeFlags = 0
			v.decodeWritePos = 0
			v.decodeReadPos = 0
			v.decodeAvail = 0
			v.decodePlayDelay = 4

			s.blockEnd &^= bit

			// 濾波器的前值歸零。
			v.decodeM2 = 0
			v.decodeM1 = 0

			v.curPhase = 0
			v.curAddr = v.startAddr &^ 0x7
			v.ignoreSampLA = false
		}

		if s.spuControl&0x8000 == 0 {
			v.adsr.phase = adsrRelease
			v.adsr.envLevel = 0
		}
	}

	s.voiceOff = 0
	s.voiceOn = 0

	// 靜音控制（SPUCNT bit 14）看起來不影響 CD 音訊。
	if s.spuControl&0x4000 == 0 {
		accum = [2]int32{}
		accumFV = [2]int32{}
	}

	// CD 音訊。
	{
		var cdaRaw [2]int32
		if cd != nil {
			cdaRaw[0], cdaRaw[1] = cd.CDAudioSample()
		}

		s.writeRAM(s.cwa|0x000, uint16(cdaRaw[0]))
		s.writeRAM(s.cwa|0x200, uint16(cdaRaw[1]))

		var cdav [2]int32
		for i := 0; i < 2; i++ {
			cdav[i] = (cdaRaw[i] * s.cdVol[i]) >> 15
		}

		if s.spuControl&0x0001 != 0 {
			accum[0] += cdav[0]
			accum[1] += cdav[1]

			if s.spuControl&0x0004 != 0 { // Beetle TODO：這個位元是否依賴 bit 0 待測
				accumFV[0] += cdav[0]
				accumFV[1] += cdav[1]
			}
		}
	}

	s.cwa = (s.cwa + 1) & 0x1FF

	s.runNoise()

	// 混響輸入先夾成 16 位元。
	accumFV[0] = clamp16(accumFV[0])
	accumFV[1] = clamp16(accumFV[1])

	reverb := s.runReverb(accumFV)

	for lr := 0; lr < 2; lr++ {
		accum[lr] += (reverb[lr] * s.reverbVol[lr]) >> 15
		accum[lr] = clamp16(accum[lr])
	}

	if s.bufPos < bufferFrames { // 滿了就丟新的
		for lr := 0; lr < 2; lr++ {
			out := clamp16((accum[lr] * int32(int16(s.globalSweep[lr].current))) >> 15)
			// 衰減到 75%，留給重取樣的餘量。
			s.buf[s.bufPos][lr] = int16((out*3 + 2) >> 2)
		}
		s.bufPos++
	}

	// 主音量掃描。
	s.globalSweep[0].run()
	s.globalSweep[1].run()
}

// DMAWrite 寫一個 32 位元字到 SPU RAM 的傳輸位址（Beetle SPU_WriteDMA）。
func (s *SPU) DMAWrite(v uint32) {
	s.writeRAM(s.rwAddr, uint16(v))
	s.rwAddr = (s.rwAddr + 1) & 0x3FFFF

	s.writeRAM(s.rwAddr, uint16(v>>16))
	s.rwAddr = (s.rwAddr + 1) & 0x3FFFF

	s.checkIRQAddr(s.rwAddr)
}

// DMARead 從 SPU RAM 的傳輸位址讀一個 32 位元字（Beetle SPU_ReadDMA）。
func (s *SPU) DMARead() uint32 {
	ret := uint32(s.readRAM(s.rwAddr))
	s.rwAddr = (s.rwAddr + 1) & 0x3FFFF

	ret |= uint32(s.readRAM(s.rwAddr)) << 16
	s.rwAddr = (s.rwAddr + 1) & 0x3FFFF

	s.checkIRQAddr(s.rwAddr)

	return ret
}

// Write 寫 16 位元暫存器（Beetle SPU_Write）。addr 是實體位址 1F801C00–1F801FFF。
func (s *SPU) Write(addr uint32, v uint16) {
	a := addr & 0x3FF

	if a >= 0x200 {
		if a < 0x260 {
			vo := &s.voices[(a-0x200)>>2]
			vo.sweep[(a&2)>>1].current = v
		} else if a < 0x280 {
			s.auxRegs[(a&0x1F)>>1] = v
		}
		return
	}

	if a < 0x180 {
		vo := &s.voices[a>>4]

		switch a & 0xF {
		case 0x00, 0x02:
			vo.sweep[(a&2)>>1].control = v
		case 0x04:
			vo.pitch = v
		case 0x06:
			vo.startAddr = (uint32(v) << 2) & 0x3FFFF
		case 0x08:
			vo.adsrControl &= 0xFFFF0000
			vo.adsrControl |= uint32(v)
			vo.cacheEnvelope()
		case 0x0A:
			vo.adsrControl &= 0x0000FFFF
			vo.adsrControl |= uint32(v) << 16
			vo.cacheEnvelope()
		case 0x0C:
			vo.adsr.envLevel = v
		case 0x0E:
			vo.loopAddr = (uint32(v) << 2) & 0x3FFFF
			vo.ignoreSampLA = true
		}
	} else {
		v32 := uint32(v)
		switch a & 0x7F {
		case 0x00, 0x02:
			s.globalSweep[(a&2)>>1].control = v

		case 0x04:
			s.reverbVol[0] = int32(int16(v))
		case 0x06:
			s.reverbVol[1] = int32(int16(v))

		case 0x08: // key on
			s.voiceOn = s.voiceOn&0xFFFF0000 | v32
		case 0x0A:
			s.voiceOn = s.voiceOn&0x0000FFFF | (v32&0xFF)<<16

		case 0x0C: // key off
			s.voiceOff = s.voiceOff&0xFFFF0000 | v32
		case 0x0E:
			s.voiceOff = s.voiceOff&0x0000FFFF | (v32&0xFF)<<16

		case 0x10:
			s.fmMode = s.fmMode&0xFFFF0000 | v32
		case 0x12:
			s.fmMode = s.fmMode&0x0000FFFF | (v32&0xFF)<<16

		case 0x14:
			s.noiseMode = s.noiseMode&0xFFFF0000 | v32
		case 0x16:
			s.noiseMode = s.noiseMode&0x0000FFFF | (v32&0xFF)<<16

		case 0x18:
			s.reverbMode = s.reverbMode&0xFFFF0000 | v32
		case 0x1A:
			s.reverbMode = s.reverbMode&0x0000FFFF | (v32&0xFF)<<16

		case 0x1C:
			s.blockEnd = s.blockEnd&0xFFFF0000 | v32
		case 0x1E:
			s.blockEnd = s.blockEnd&0x0000FFFF | v32<<16

		case 0x22:
			s.reverbWA = (v32 << 2) & 0x3FFFF
			s.reverbCur = s.reverbWA

		case 0x24:
			s.irqAddr = (v32 << 2) & 0x3FFFF
			s.checkIRQAddr(s.rwAddr)

		case 0x26:
			s.rwAddr = (v32 << 2) & 0x3FFFF
			s.checkIRQAddr(s.rwAddr)

		case 0x28:
			s.writeRAM(s.rwAddr, v)
			s.rwAddr = (s.rwAddr + 1) & 0x3FFFF
			s.checkIRQAddr(s.rwAddr)

		case 0x2A:
			s.spuControl = v
			if v&0x40 == 0 {
				s.irqAsserted = false
			}
			s.checkIRQAddr(s.rwAddr)

		case 0x2C:

		case 0x30:
			s.cdVol[0] = int32(int16(v))
		case 0x32:
			s.cdVol[1] = int32(int16(v))

		case 0x34:
			s.externVol[0] = int32(int16(v))
		case 0x36:
			s.externVol[1] = int32(int16(v))

		case 0x38, 0x3A:
			s.globalSweep[(a&2)>>1].current = v
		}
	}

	s.regs[(a&0x1FF)>>1] = v
}

// Read 讀 16 位元暫存器（Beetle SPU_Read）。addr 是實體位址 1F801C00–1F801FFF。
func (s *SPU) Read(addr uint32) uint16 {
	a := addr & 0x3FF

	if a >= 0x200 {
		if a < 0x260 {
			vo := &s.voices[(a-0x200)>>2]
			return vo.sweep[(a&2)>>1].current
		} else if a < 0x280 {
			return s.auxRegs[(a&0x1F)>>1]
		}
		return 0xFFFF
	}

	if a < 0x180 {
		vo := &s.voices[a>>4]

		switch a & 0xF {
		case 0x0C:
			return vo.adsr.envLevel
		case 0x0E:
			return uint16(vo.loopAddr >> 2)
		}
	} else {
		switch a & 0x7F {
		case 0x1C:
			return uint16(s.blockEnd)
		case 0x1E:
			return uint16(s.blockEnd >> 16)
		case 0x26:
		case 0x28:
			ret := s.readRAM(s.rwAddr)
			s.rwAddr = (s.rwAddr + 1) & 0x3FFFF
			s.checkIRQAddr(s.rwAddr)
			return ret
		case 0x2A:
			return s.spuControl
		case 0x3C: // Beetle FIXME：用途不明
			return 0
		case 0x38, 0x3A:
			return s.globalSweep[(a&2)>>1].current
		}
	}

	return s.regs[(a&0x1FF)>>1]
}
