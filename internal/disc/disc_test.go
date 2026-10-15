package disc

import (
	"os"
	"path/filepath"
	"testing"
)

// 用合成的小映像驗 cue 解析、檔名退回、TOC 與合成 Sub-Q（009 §4），不需要原版素材。
func TestOpenWithFallbackAndSubQ(t *testing.T) {
	dir := t.TempDir()
	// 300 個扇區：第 1 軌資料 0..199，第 2 軌音軌 INDEX 00 在 200、INDEX 01 在 250。
	bin := make([]byte, 300*SectorSize)
	if err := os.WriteFile(filepath.Join(dir, "image.bin"), bin, 0o644); err != nil {
		t.Fatal(err)
	}
	cue := "FILE \"\xb4c\xc5].BIN\" BINARY\r\n" + // Big5 檔名、大寫副檔名，故意對不上
		"  TRACK 01 MODE2/2352\r\n    INDEX 01 00:00:00\r\n" +
		"  TRACK 02 AUDIO\r\n    INDEX 00 00:02:50\r\n    INDEX 01 00:03:25\r\n"
	cuePath := filepath.Join(dir, "image.cue")
	if err := os.WriteFile(cuePath, []byte(cue), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(cuePath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Fallback == "" {
		t.Error("應該回報檔名退回")
	}
	if d.LeadOut() != 300 {
		t.Errorf("lead-out %d，應為 300", d.LeadOut())
	}
	if len(d.Tracks) != 2 || d.Tracks[1].Index0 != 200 || d.Tracks[1].Index1 != 250 || !d.Tracks[1].Audio {
		t.Fatalf("TOC 不對：%+v", d.Tracks)
	}

	cases := []struct {
		lba                    int32
		ctrl, track, index     byte
		rm, rs, rf, am, as, af byte
	}{
		{0, 0x41, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00},   // 第 1 軌開頭，絕對時間 +2 秒
		{199, 0x41, 0x01, 0x01, 0x00, 0x02, 0x49, 0x00, 0x04, 0x49}, // 199 = 2 秒 49 格
		{230, 0x01, 0x02, 0x00, 0x00, 0x00, 0x20, 0x00, 0x05, 0x05}, // 音軌前置區：相對時間倒數
		{260, 0x01, 0x02, 0x01, 0x00, 0x00, 0x10, 0x00, 0x05, 0x35},
	}
	for _, c := range cases {
		q := d.SubQ(c.lba)
		want := [10]byte{c.ctrl, c.track, c.index, c.rm, c.rs, c.rf, 0, c.am, c.as, c.af}
		if [10]byte(q[:10]) != want {
			t.Errorf("LBA %d：Sub-Q %x，期望 %x", c.lba, q[:10], want)
		}
	}
	if q := d.SubQ(300); q[1] != 0xAA {
		t.Errorf("lead-out 的軌號 %02x，應為 AA", q[1])
	}
}
