package cdrom

import (
	"testing"

	"github.com/wicanr2/ps-golem/internal/disc"
)

// 4 位元單聲道 XA：第 0 組第 0 單元 shift 12、filter 0，低半位元組 7 → 取樣 (0x7000 >> 12) = 7（Beetle DecodeXAADPCM）。
func TestXADecode4BitMono(t *testing.T) {
	c := New()
	var buf [disc.SectorSize]byte
	buf[12+4+3] = 0x00 // coding：4 位元、37.8 kHz、單聲道
	sg := 12 + 4 + 8
	buf[sg+0], buf[sg+4] = 0x0C, 0x0C // 單元 0 的參數與副本
	for i := 0; i < 28; i++ {
		buf[sg+16+i*4] = 0x07 // 單元 0 用低半位元組
	}
	c.xaProcessSector(&buf)
	if c.audio.size != 18*8*28 || c.audio.freq != 6 {
		t.Fatalf("size=%d freq=%d，應為 4032、6", c.audio.size, c.audio.freq)
	}
	for i := 0; i < 28; i++ {
		if c.audio.samples[0][i] != 7 || c.audio.samples[1][i] != 7 {
			t.Fatalf("取樣 %d = %d/%d，應為 7", i, c.audio.samples[0][i], c.audio.samples[1][i])
		}
	}
}

// 參數與副本不同時整個單元輸出 0。
func TestXAParamMismatchSilences(t *testing.T) {
	c := New()
	var buf [disc.SectorSize]byte
	sg := 12 + 4 + 8
	buf[sg+0], buf[sg+4] = 0x0C, 0x0B
	for i := 0; i < 28; i++ {
		buf[sg+16+i*4] = 0x07
	}
	c.xaProcessSector(&buf)
	for i := 0; i < 28; i++ {
		if c.audio.samples[0][i] != 0 {
			t.Fatalf("取樣 %d = %d，應為 0", i, c.audio.samples[0][i])
		}
	}
}

// CD-DA 1 倍速：每個 44.1 kHz 取樣讀一筆，音量矩陣預設左→左、右→右 0x80（×1），靜音時為 0。
func TestCDDAVolumeAndMute(t *testing.T) {
	c := New()
	var buf [disc.SectorSize]byte
	buf[0], buf[1], buf[2], buf[3] = 0x34, 0x12, 0xCC, 0xED // L=0x1234、R=-0x1234
	c.enbufferizeCDDA(&buf)
	l, r := c.CDAudioSample()
	if l != 0x1234 || r != -0x1234 {
		t.Fatalf("取樣 %d/%d", l, r)
	}
	c.muted = true
	if l, r := c.CDAudioSample(); l != 0 || r != 0 {
		t.Fatalf("靜音時 %d/%d", l, r)
	}
}
