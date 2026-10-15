package ebiten

import (
	"os"
	"path/filepath"

	"github.com/wicanr2/ps-golem/internal/sio"
)

// cardQuietFields 是記憶卡最後一次寫入後要等多少 field 才存檔（012 §6）。
const cardQuietFields = 60

// cardSaver 在記憶卡寫入告一段落後把內容存回檔案。
type cardSaver struct {
	path       string
	lastWrites uint64
	lastField  uint64
}

// tick 每個 field 呼叫一次；回傳是否寫了檔。
func (s *cardSaver) tick(c *sio.Memcard, field uint64) (bool, error) {
	if c.Writes != s.lastWrites {
		s.lastWrites, s.lastField = c.Writes, field
		return false, nil
	}
	if !c.Dirty || field-s.lastField < cardQuietFields {
		return false, nil
	}
	return true, s.flush(c)
}

// flush 有 Dirty 就寫回：先寫暫存檔再改名。
func (s *cardSaver) flush(c *sio.Memcard) error {
	if !c.Dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, c.Data[:], 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	c.Dirty = false
	return nil
}
