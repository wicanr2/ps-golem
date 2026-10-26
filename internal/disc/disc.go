// Package disc 讀 cue／bin 光碟映像。規格：docs/spec/009-cdrom.md §1。
package disc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SectorSize 是 raw 扇區大小。
const SectorSize = 2352

// Track 是一軌的資訊。LBA 以光碟的邏輯扇區計（第一軌 INDEX 01 為 0）。
type Track struct {
	Number int
	Audio  bool
	Index0 int32 // INDEX 00 的 LBA；沒有時等於 Index1
	Index1 int32
}

// Disc 是一張開啟的光碟。
type Disc struct {
	f       *os.File
	sectors int32 // bin 的扇區數 = lead-out LBA
	Tracks  []Track
	region  string
	boot    string
	// Fallback 記錄 cue 內檔名找不到、改用同目錄 .bin 的情形（給呼叫端回報）。
	Fallback string
}

// Open 開啟 cue 檔。
func Open(cuePath string) (*Disc, error) {
	cue, err := os.ReadFile(cuePath)
	if err != nil {
		return nil, err
	}
	name, tracks, err := parseCue(cue)
	if err != nil {
		return nil, err
	}
	d := &Disc{Tracks: tracks}
	dir := filepath.Dir(cuePath)
	binPath := filepath.Join(dir, name)
	if _, err := os.Stat(binPath); err != nil {
		alt, err2 := soleBin(dir)
		if err2 != nil {
			return nil, fmt.Errorf("cue 指定的 %q 不存在，且無法退回：%v", name, err2)
		}
		d.Fallback = fmt.Sprintf("cue 指定的 %q 不存在，改用 %s", name, filepath.Base(alt))
		binPath = alt
	}
	f, err := os.Open(binPath)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	d.f = f
	d.sectors = int32(st.Size() / SectorSize)
	d.boot = d.detectBoot()
	d.region = regionOf(d.boot)
	return d, nil
}

// Close 關閉映像檔。
func (d *Disc) Close() error { return d.f.Close() }

// LeadOut 回傳 lead-out 的 LBA。
func (d *Disc) LeadOut() int32 { return d.sectors }

// Region 回傳 SCEI／SCEA／SCEE；判斷不出來回 ""。
func (d *Disc) Region() string { return d.region }

func soleBin(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found []string
	for _, e := range ents {
		if strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("目錄裡有 %d 個 .bin", len(found))
	}
	return found[0], nil
}

// parseCue 解析單一 FILE 的 cue。
func parseCue(b []byte) (string, []Track, error) {
	var name string
	var tracks []Track
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(strings.TrimSpace(sc.Text()))
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "FILE":
			line := sc.Text()
			i := strings.Index(line, "\"")
			j := strings.LastIndex(line, "\"")
			if i < 0 || j <= i {
				return "", nil, fmt.Errorf("FILE 行格式不對：%q", line)
			}
			if name != "" {
				return "", nil, fmt.Errorf("只支援單一 FILE")
			}
			name = line[i+1 : j]
		case "TRACK":
			if len(f) < 3 {
				return "", nil, fmt.Errorf("TRACK 行格式不對")
			}
			n, _ := strconv.Atoi(f[1])
			mode := strings.ToUpper(f[2])
			if mode != "AUDIO" && mode != "MODE2/2352" && mode != "MODE1/2352" {
				return "", nil, fmt.Errorf("不支援的軌道格式 %s", mode)
			}
			tracks = append(tracks, Track{Number: n, Audio: mode == "AUDIO", Index0: -1, Index1: -1})
		case "INDEX":
			if len(f) < 3 || len(tracks) == 0 {
				return "", nil, fmt.Errorf("INDEX 行格式不對")
			}
			lba, err := parseMSF(f[2])
			if err != nil {
				return "", nil, err
			}
			t := &tracks[len(tracks)-1]
			switch f[1] {
			case "00":
				t.Index0 = lba
			case "01":
				t.Index1 = lba
			}
		}
	}
	if name == "" || len(tracks) == 0 {
		return "", nil, fmt.Errorf("cue 沒有 FILE 或 TRACK")
	}
	for i := range tracks {
		if tracks[i].Index1 < 0 {
			return "", nil, fmt.Errorf("第 %d 軌沒有 INDEX 01", tracks[i].Number)
		}
		if tracks[i].Index0 < 0 {
			tracks[i].Index0 = tracks[i].Index1
		}
	}
	return name, tracks, nil
}

func parseMSF(s string) (int32, error) {
	p := strings.Split(s, ":")
	if len(p) != 3 {
		return 0, fmt.Errorf("MSF 格式不對：%q", s)
	}
	var v [3]int
	for i := range p {
		n, err := strconv.Atoi(p[i])
		if err != nil {
			return 0, err
		}
		v[i] = n
	}
	return int32((v[0]*60+v[1])*75 + v[2]), nil
}

// ReadSector 讀一個 raw 扇區。超出映像範圍回傳全 0。
func (d *Disc) ReadSector(lba int32, out *[SectorSize]byte) {
	*out = [SectorSize]byte{}
	if lba < 0 || lba >= d.sectors {
		return
	}
	_, _ = d.f.ReadAt(out[:], int64(lba)*SectorSize)
}

// TrackAt 回傳 lba 所在的軌（lba 落在 INDEX 00 之後的那一軌）。lead-out 回傳 nil。
func (d *Disc) TrackAt(lba int32) *Track {
	if lba >= d.sectors {
		return nil
	}
	var t *Track
	for i := range d.Tracks {
		if lba >= d.Tracks[i].Index0 || i == 0 {
			t = &d.Tracks[i]
		}
	}
	return t
}

func bcd(v int) byte { return byte(v/10<<4 | v%10) }

func toMSF(sectors int32) (byte, byte, byte) {
	if sectors < 0 {
		sectors = 0
	}
	m := sectors / (60 * 75)
	s := sectors / 75 % 60
	f := sectors % 75
	return bcd(int(m)), bcd(int(s)), bcd(int(f))
}

// SubQ 合成 lba 位置的 sub-channel Q（009 §1）。lead-out 的軌號為 AAh。
func (d *Disc) SubQ(lba int32) [12]byte {
	var q [12]byte
	t := d.TrackAt(lba)
	if t == nil {
		q[0] = 0x41
		q[1] = 0xAA
		q[2] = 0x01
		rm, rs, rf := toMSF(lba - d.sectors)
		q[3], q[4], q[5] = rm, rs, rf
	} else {
		q[0] = 0x41
		if t.Audio {
			q[0] = 0x01
		}
		q[1] = bcd(t.Number)
		rel := lba - t.Index1
		if lba < t.Index1 {
			q[2] = 0x00
			rel = t.Index1 - lba
		} else {
			q[2] = 0x01
		}
		q[3], q[4], q[5] = toMSF(rel)
	}
	q[7], q[8], q[9] = toMSF(lba + 150)
	crc := subqCRC(q[:10])
	q[10], q[11] = byte(crc>>8), byte(crc)
	return q
}

// subqCRC 是 sub-channel Q 的 CRC-16（多項式 0x1021，結果取反）。
func subqCRC(b []byte) uint16 {
	crc := uint16(0)
	for _, v := range b {
		crc ^= uint16(v) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return ^crc
}

// userData 讀 Mode 2 Form 1 扇區的 2048 位元組資料。
func (d *Disc) userData(lba int32) []byte {
	var s [SectorSize]byte
	d.ReadSector(lba, &s)
	return append([]byte(nil), s[24:24+2048]...)
}

// detectBoot 讀 SYSTEM.CNF 的 BOOT 檔名，去掉路徑與 ";1" 版本號（009 §1）。
func (d *Disc) detectBoot() string {
	cnf, err := d.ReadFile("SYSTEM.CNF")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(cnf), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(strings.ToUpper(k)) != "BOOT" {
			continue
		}
		v = strings.TrimSpace(v)
		if i := strings.LastIndexAny(v, ":\\/"); i >= 0 {
			v = v[i+1:]
		}
		v, _, _ = strings.Cut(v, ";")
		return v
	}
	return ""
}

// regionOf 依開機檔名第 3 字元判斷區域（009 §1）。
func regionOf(boot string) string {
	if len(boot) < 3 {
		return ""
	}
	switch boot[2] {
	case 'P', 'p':
		return "SCEI"
	case 'U', 'u':
		return "SCEA"
	case 'E', 'e':
		return "SCEE"
	}
	return ""
}

// BootFile 回傳 SYSTEM.CNF 指定的開機檔名（例如 SLPM_860.23）；不是 PS1 光碟回 ""。
func (d *Disc) BootFile() string { return d.boot }

// ReadFile reads the existing 2048-byte ISO view, including relative subpaths.
// 009 §7. The original root-name behavior and io.EOF result are retained.
func (d *Disc) ReadFile(name string) ([]byte, error) {
	return d.ReadFileLimit(name, ^uint32(0))
}

// ReadFileLimit is a host-side read and does not advance the emulated drive.
func (d *Disc) ReadFileLimit(name string, maxBytes uint32) ([]byte, error) {
	parts := strings.Split(strings.ReplaceAll(name, "\\", "/"), "/")
	if len(parts) > 16 {
		return nil, fmt.Errorf("ISO path too deep")
	}
	for i, p := range parts {
		p, _, _ = strings.Cut(p, ";")
		if p == "" || p == "." || p == ".." {
			return nil, fmt.Errorf("invalid ISO path")
		}
		parts[i] = p
	}
	pvd := d.userData(16)
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" || pvd[6] != 1 {
		return nil, fmt.Errorf("不是 ISO9660")
	}
	lba, size, err := d.isoExtent(pvd[156:190])
	if err != nil {
		return nil, err
	}
	for level, part := range parts {
		if size > 16<<20 {
			return nil, fmt.Errorf("ISO directory too large")
		}
		found := false
		var nextLBA, nextSize uint32
		for off := uint32(0); off < size && !found; off += 2048 {
			sec := d.userData(int32(lba + off/2048))
			end := min(uint32(2048), size-off)
			for p := uint32(0); p < end; {
				n := uint32(sec[p])
				if n == 0 {
					break
				}
				if n < 34 || n > end-p {
					return nil, fmt.Errorf("invalid ISO directory record")
				}
				rec := sec[p : p+n]
				nl := uint32(rec[32])
				if 33+nl > n {
					return nil, fmt.Errorf("invalid ISO filename length")
				}
				fn, _, _ := strings.Cut(string(rec[33:33+nl]), ";")
				if strings.EqualFold(fn, part) {
					if rec[25]&0x80 != 0 {
						return nil, fmt.Errorf("multi-extent ISO file unsupported")
					}
					if level < len(parts)-1 && rec[25]&2 == 0 {
						return nil, io.EOF
					}
					nextLBA, nextSize, err = d.isoExtent(rec)
					if err != nil {
						return nil, err
					}
					found = true
					break
				}
				p += n
			}
		}
		if !found {
			return nil, io.EOF
		}
		lba, size = nextLBA, nextSize
	}
	if size > maxBytes || uint64(size) > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("ISO file exceeds read limit")
	}
	out := make([]byte, int(size))
	for off := uint32(0); off < size; {
		sec := d.userData(int32(lba + off/2048))
		n := min(uint32(2048), size-off)
		copy(out[off:off+n], sec[:n])
		off += n
	}
	return out, nil
}

func (d *Disc) isoExtent(rec []byte) (uint32, uint32, error) {
	if len(rec) < 34 {
		return 0, 0, fmt.Errorf("invalid ISO extent record")
	}
	lba, size := binary.LittleEndian.Uint32(rec[2:]), binary.LittleEndian.Uint32(rec[10:])
	if lba != binary.BigEndian.Uint32(rec[6:]) || size != binary.BigEndian.Uint32(rec[14:]) {
		return 0, 0, fmt.Errorf("inconsistent ISO extent")
	}
	if uint64(lba)+(uint64(size)+2047)/2048 > uint64(d.sectors) {
		return 0, 0, fmt.Errorf("ISO extent outside image")
	}
	return lba, size, nil
}
