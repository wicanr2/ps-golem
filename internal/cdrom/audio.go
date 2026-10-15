package cdrom

import "github.com/wicanr2/ps-golem/internal/disc"

// CD 音訊（docs/spec/013-spu-and-cd-audio.md §2）：XA ADPCM 解碼、CD-DA、重取樣到 44.1 kHz、音量矩陣。
// 移植自 beetle-psx-libretro mednafen/psx/cdc.c（commit e57860d82faa953dc0fc10f201fd583739bc2f7d，
// GPL-2.0-or-later）的 PS_CDC_XA_ProcessSector、PS_CDC_EnbufferizeCDDASector、PS_CDC_GetCDAudio、
// PS_CDC_ApplyVolume、CDC_GetCDAudioSample。

// audioState 是 CD 音訊的狀態，CDROM 內嵌一份。
type audioState struct {
	samples          [2][0x1000]int16 // [0] 左、[1] 右
	size, freq, read uint32

	xaPrev [2][2]int16

	resampBuf   [2][64]int16
	resampPos   uint32
	resampPhase uint32

	pendingVol [2][2]uint8 // [來源][輸出]
	vol        [2][2]uint8
}

// AudioSource 是 SPU 取 CD 音訊的來源；CDROM 實作它。
type AudioSource interface{ CDAudioSample() (l, r int32) }

// SPUClock 是 CD-ROM 推進時順帶推進的 SPU（Beetle SPU_UpdateFromCDC）：
// 推進 clocks 個週期，回傳到下一個取樣點的週期數。
type SPUClock interface {
	Advance(clocks int64, cd AudioSource) int64
}

func (a *audioState) clear() {
	a.samples = [2][0x1000]int16{}
	a.size, a.freq, a.read = 0, 0, 0
	a.xaPrev = [2][2]int16{}
	a.resampBuf = [2][64]int16{}
	a.resampPhase, a.resampPos = 0, 0
}

func (a *audioState) resetVolume() {
	a.pendingVol = [2][2]uint8{{0x80, 0}, {0, 0x80}}
	a.vol = a.pendingVol
}

var cdADPCMImpulse = [7][25]int16{
	{0, -5, 17, -35, 70, -23, -68, 347, -839, 2062, -4681, 15367, 21472, -5882, 2810, -1352, 635, -235, 26, 43, -35, 16, -8, 2, 0},
	{0, -2, 10, -34, 65, -84, 52, 9, -266, 1024, -2680, 9036, 26516, -6016, 3021, -1571, 848, -365, 107, 10, -16, 17, -8, 3, -1},
	{-2, 0, 3, -19, 60, -75, 162, -227, 306, -67, -615, 3229, 29883, -4532, 2488, -1471, 882, -424, 166, -27, 5, 6, -8, 3, -1},
	{-1, 3, -2, -5, 31, -74, 179, -402, 689, -926, 1272, -1446, 31033, -1446, 1272, -926, 689, -402, 179, -74, 31, -5, -2, 3, -1},
	{-1, 3, -8, 6, 5, -27, 166, -424, 882, -1471, 2488, -4532, 29883, 3229, -615, -67, 306, -227, 162, -75, 60, -19, 3, 0, -2},
	{-1, 3, -8, 17, -16, 10, 107, -365, 848, -1571, 3021, -6016, 26516, 9036, -2680, 1024, -266, 9, 52, -84, 65, -34, 10, -2, 0},
	{0, 2, -8, 16, -35, 43, 26, -235, 635, -1352, 2810, -5882, 21472, 15367, -4681, 2062, -839, 347, -68, -23, 70, -35, 17, -5, 0},
}

// xaWeights 只有前 5 組有值，其餘為 0（Beetle 的靜態陣列宣告 16 組、只初始化 5 組）。
var xaWeights = [16][2]int32{{0, 0}, {60, 0}, {115, -52}, {98, -55}, {122, -60}}

func sat16(v int32) int32 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return v
}

func (c *CDROM) readAudioBuffer() (int32, int32) {
	a := &c.audio
	l, r := int32(a.samples[0][a.read&0xFFF]), int32(a.samples[1][a.read&0xFFF])
	a.read++
	return l, r
}

func (c *CDROM) applyVolume(l, r int32) (int32, int32) {
	if c.muted {
		return 0, 0
	}
	v := &c.audio.vol
	lo := (l*int32(v[0][0]))>>7 + (r*int32(v[1][0]))>>7
	ro := (l*int32(v[0][1]))>>7 + (r*int32(v[1][1]))>>7
	return sat16(lo), sat16(ro)
}

// CDAudioSample 給 SPU 取一組 44.1 kHz 的 CD 音訊（Beetle CDC_GetCDAudioSample）。
func (c *CDROM) CDAudioSample() (int32, int32) {
	a := &c.audio
	if a.read >= a.size || a.freq == 0 {
		return 0, 0
	}
	var out [2]int32
	if a.freq == 7 || a.freq == 14 {
		out[0], out[1] = c.readAudioBuffer()
		if a.freq == 14 {
			out[0], out[1] = c.readAudioBuffer()
		}
	} else {
		for i := 0; i < 2; i++ {
			imp := &cdADPCMImpulse[a.resampPhase]
			base := (a.resampPos + 32 - 25) & 0x1F
			var acc int32
			for s := uint32(0); s < 25; s++ {
				acc += int32(imp[s]) * int32(a.resampBuf[i][base+s])
			}
			out[i] = sat16(acc >> 15)
		}
		a.resampPhase += a.freq
		if a.resampPhase >= 7 {
			a.resampPhase -= 7
			l, r := c.readAudioBuffer()
			raw := [2]int32{l, r}
			for i := 0; i < 2; i++ {
				a.resampBuf[i][a.resampPos] = int16(raw[i])
				a.resampBuf[i][a.resampPos+32] = int16(raw[i])
			}
			a.resampPos = (a.resampPos + 1) & 0x1F
		}
	}
	return c.applyVolume(out[0], out[1])
}

// decodeXAADPCM 解一個 28 取樣的單元；out[0]、out[1] 是前兩個取樣（Beetle DecodeXAADPCM）。
func decodeXAADPCM(in *[28]uint8, out *[30]int16, shift, weight uint8) {
	w := xaWeights[weight]
	for i := 0; i < 28; i++ {
		s := int32(int16(uint16(in[i]) << 8))
		s >>= shift
		s += (int32(out[i+1])*w[0])>>6 + (int32(out[i])*w[1])>>6
		out[i+2] = int16(sat16(s))
	}
}

// xaProcessSector 把一個 XA 音訊扇區解碼進音訊緩衝（Beetle PS_CDC_XA_ProcessSector）。
func (c *CDROM) xaProcessSector(buf *[disc.SectorSize]byte) {
	a := &c.audio
	coding := buf[12+4+3]
	shiftIdx := uint32(1)
	if coding&0x10 != 0 { // 8 位元
		shiftIdx = 0
	}
	stereo := coding&0x01 != 0
	a.read = 0
	a.size = 18 * (4 << shiftIdx) * 28
	if stereo {
		a.size >>= 1
	}
	a.freq = 6
	if coding&0x04 != 0 { // 18.9 kHz
		a.freq = 3
	}
	for group := uint32(0); group < 18; group++ {
		sg := buf[12+4+8+group*128:]
		params, samples := sg[:16], sg[16:128]
		for unit := uint32(0); unit < 4<<shiftIdx; unit++ {
			param := params[(unit&3)|((unit&4)<<1)]
			paramCopy := params[4|(unit&3)|((unit&4)<<1)]
			var in [28]uint8
			for i := uint32(0); i < 28; i++ {
				t := samples[i*4+(unit>>shiftIdx)]
				if shiftIdx != 0 {
					if unit&1 == 0 {
						t <<= 4
					}
					t &= 0xF0
				}
				in[i] = t
			}
			ocn := 0
			if unit&1 != 0 && stereo {
				ocn = 1
			}
			var out [30]int16
			out[0], out[1] = a.xaPrev[ocn][0], a.xaPrev[ocn][1]
			decodeXAADPCM(&in, &out, param&0x0F, param>>4)
			a.xaPrev[ocn][0], a.xaPrev[ocn][1] = out[28], out[29]
			if param != paramCopy {
				out = [30]int16{}
			}
			if stereo {
				base := group*(2<<shiftIdx)*28 + (unit>>1)*28
				for s := uint32(0); s < 28; s++ {
					a.samples[ocn][base+s] = out[2+s]
				}
			} else {
				base := group*(4<<shiftIdx)*28 + unit*28
				for s := uint32(0); s < 28; s++ {
					a.samples[0][base+s] = out[2+s]
					a.samples[1][base+s] = out[2+s]
				}
			}
		}
	}
}

// enbufferizeCDDA 把一個 CD-DA 扇區放進音訊緩衝（Beetle PS_CDC_EnbufferizeCDDASector）。
func (c *CDROM) enbufferizeCDDA(buf *[disc.SectorSize]byte) {
	a := &c.audio
	a.freq = 7
	if c.mode&modeSpeed != 0 {
		a.freq = 14
	}
	a.size = 588
	if c.subQ[0]&0x40 != 0 {
		for i := 0; i < 588; i++ {
			a.samples[0][i], a.samples[1][i] = 0, 0
		}
	} else {
		for i := 0; i < 588; i++ {
			a.samples[0][i] = int16(uint16(buf[i*4]) | uint16(buf[i*4+1])<<8)
			a.samples[1][i] = int16(uint16(buf[i*4+2]) | uint16(buf[i*4+3])<<8)
		}
	}
	a.read = 0
}
