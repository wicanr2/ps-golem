package hd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
)

const textureDumpUploadLimit = 65536
const textureDumpOutputLimit = 512 << 20

type dumpUpload struct {
	Upload string `json:"upload_sha256"`
	Width  uint32 `json:"width_words"`
	Height uint32 `json:"height"`
	X      uint32 `json:"x"`
	Y      uint32 `json:"y"`
	First  uint64 `json:"first_field"`
	Last   uint64 `json:"last_field"`
	Count  uint64 `json:"count"`
}
type dumpUse struct {
	up                 *textureUpload
	depth              int
	clut               [32]byte
	palette            []uint16
	first, last, count uint64
}
type dumpUseJSON struct {
	Upload string   `json:"upload_sha256"`
	Depth  int      `json:"depth"`
	CLUT   string   `json:"clut_sha256,omitempty"`
	Words  []uint16 `json:"clut_words,omitempty"`
	Image  string   `json:"image"`
	First  uint64   `json:"first_field"`
	Last   uint64   `json:"last_field"`
	Count  uint64   `json:"sample_queries"`
}

// TextureDump 只由一個 shadow 消費端收集；Close 同步後才可 WriteDirectory。
// 資源、去重與本機散布界線：018-hd-texture-replacement §5。
type TextureDump struct {
	uploads   map[[32]byte]*dumpUpload
	used      map[textureKey]*dumpUse
	retained  map[[32]byte]*textureUpload
	rawBytes  uint64
	completed uint64
	err       error
}

func NewTextureDump() *TextureDump {
	return &TextureDump{uploads: map[[32]byte]*dumpUpload{}, used: map[textureKey]*dumpUse{}, retained: map[[32]byte]*textureUpload{}}
}

func (d *TextureDump) holdRaw(u *textureUpload) {
	if d.err != nil {
		u.words = nil
		return
	}
	cost := uint64(len(u.words)) * 2
	if d.rawBytes+cost > textureMemoryLimit {
		d.err = fmt.Errorf("傾印原始資料超過 %d bytes", textureMemoryLimit)
		u.words = nil
		return
	}
	d.rawBytes += cost
}
func (d *TextureDump) releaseRaw(u *textureUpload) {
	d.rawBytes -= uint64(len(u.words)) * 2
	u.words = nil
}

func (d *TextureDump) upload(u *textureUpload, x, y uint32, field uint64) {
	if d.err != nil {
		return
	}
	d.completed++
	m := d.uploads[u.key]
	if m == nil {
		if len(d.uploads) >= textureDumpUploadLimit {
			d.err = fmt.Errorf("傾印上傳鍵超過 %d", textureDumpUploadLimit)
			return
		}
		m = &dumpUpload{Upload: fmt.Sprintf("%x", u.key), Width: u.w, Height: u.h, First: field}
		d.uploads[u.key] = m
	}
	m.X, m.Y, m.Last = x, y, field
	m.Count++
}

func (d *TextureDump) use(u *textureUpload, depth int, clut [32]byte, palette []uint16, field uint64) {
	if d.err != nil {
		return
	}
	k := textureKey{upload: u.key, depth: depth, clut: clut}
	e := d.used[k]
	if e == nil {
		if len(d.used) >= textureEntryLimit {
			d.err = fmt.Errorf("傾印使用組合超過 %d", textureEntryLimit)
			return
		}
		if len(u.words) != int(u.w*u.h) {
			d.err = fmt.Errorf("傾印來源資料已失效")
			return
		}
		canonical := d.retained[u.key]
		if canonical == nil {
			canonical = u
			canonical.refs++
			d.retained[u.key] = canonical
		}
		e = &dumpUse{up: canonical, depth: depth, clut: clut, first: field}
		if depth != 15 {
			n := 16
			if depth == 8 {
				n = 256
			}
			e.palette = append([]uint16(nil), palette[:n]...)
		}
		d.used[k] = e
	}
	e.last = field
	e.count++
}

func dumpColor(v uint16) color.NRGBA {
	alpha := uint8(255)
	if v == 0 {
		alpha = 0
	}
	convert := func(c uint16) uint8 { return uint8((c << 3) | (c >> 2)) }
	return color.NRGBA{convert(v & 31), convert((v >> 5) & 31), convert((v >> 10) & 31), alpha}
}

// WriteDirectory 拒絕既有目錄，manifest 最後寫入。失敗目錄留 INCOMPLETE，不能當成素材包。
func (d *TextureDump) WriteDirectory(directory string) (err error) {
	if d == nil {
		return fmt.Errorf("沒有傾印快照")
	}
	if d.err != nil {
		return d.err
	}
	if err = os.Mkdir(directory, 0755); err != nil {
		return fmt.Errorf("建立傾印目錄：%w", err)
	}
	marker := filepath.Join(directory, "INCOMPLETE")
	if err = os.WriteFile(marker, []byte("傾印尚未完成，不可當成素材包。\n"), 0644); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.WriteFile(marker, []byte(err.Error()+"\n"), 0644)
		}
	}()
	if err = os.Mkdir(filepath.Join(directory, "images"), 0755); err != nil {
		return err
	}
	var total uint64
	write := func(name string, b []byte) error {
		if total+uint64(len(b)) > textureDumpOutputLimit {
			return fmt.Errorf("傾印輸出超過 %d bytes", textureDumpOutputLimit)
		}
		if err := os.WriteFile(filepath.Join(directory, filepath.FromSlash(name)), b, 0644); err != nil {
			return err
		}
		total += uint64(len(b))
		return nil
	}
	writePNG := func(name string, im image.Image) error {
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			return err
		}
		return write(name, b.Bytes())
	}
	keys := make([]textureKey, 0, len(d.used))
	for k := range d.used {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if c := bytes.Compare(keys[i].upload[:], keys[j].upload[:]); c != 0 {
			return c < 0
		}
		if keys[i].depth != keys[j].depth {
			return keys[i].depth < keys[j].depth
		}
		return bytes.Compare(keys[i].clut[:], keys[j].clut[:]) < 0
	})
	manifest := textureManifest{Version: 1, Entries: []textureSpec{}}
	metadata := struct {
		Version   int           `json:"version"`
		Space     string        `json:"coordinate_space"`
		Completed uint64        `json:"completed_uploads"`
		Uploads   []*dumpUpload `json:"uploads"`
		Uses      []dumpUseJSON `json:"uses"`
	}{Version: 1, Space: "native GPU VRAM 16-bit words, 1024x512; uploads are not semantic assets", Completed: d.completed, Uploads: []*dumpUpload{}, Uses: []dumpUseJSON{}}
	for _, u := range d.uploads {
		metadata.Uploads = append(metadata.Uploads, u)
	}
	sort.Slice(metadata.Uploads, func(i, j int) bool { return metadata.Uploads[i].Upload < metadata.Uploads[j].Upload })
	seen := map[textureKey]bool{}
	for _, k := range keys {
		e := d.used[k]
		u := e.up
		base := fmt.Sprintf("%x-%d", u.key, e.depth)
		if e.depth != 15 {
			base += fmt.Sprintf("-%x", e.clut)
		}
		name := "images/" + base + ".png"
		entry := textureSpec{Upload: fmt.Sprintf("%x", u.key), Width: int(u.w), Height: int(u.h), Depth: e.depth, Scale: 1, Image: name}
		use := dumpUseJSON{Upload: entry.Upload, Depth: e.depth, Image: name, First: e.first, Last: e.last, Count: e.count}
		if e.depth != 15 {
			entry.Kind = "indexed"
			use.CLUT = fmt.Sprintf("%x", e.clut)
			use.Words = e.palette
			per := 4
			if e.depth == 8 {
				per = 2
			}
			pal := make(color.Palette, len(e.palette))
			for i, c := range e.palette {
				pal[i] = dumpColor(c)
			}
			im := image.NewPaletted(image.Rect(0, 0, int(u.w)*per, int(u.h)), pal)
			for y := 0; y < int(u.h); y++ {
				for x := 0; x < int(u.w)*per; x++ {
					word := u.words[y*int(u.w)+x/per]
					im.SetColorIndex(x, y, uint8((word>>((x%per)*e.depth))&uint16((1<<e.depth)-1)))
				}
			}
			if err = writePNG(name, im); err != nil {
				return err
			}
		} else {
			entry.Kind = "color"
			entry.STP = "images/" + base + "-stp.png"
			im := image.NewNRGBA(image.Rect(0, 0, int(u.w), int(u.h)))
			mask := image.NewGray(im.Bounds())
			for y := 0; y < int(u.h); y++ {
				for x := 0; x < int(u.w); x++ {
					v := u.words[y*int(u.w)+x]
					im.SetNRGBA(x, y, dumpColor(v))
					if v&0x8000 != 0 {
						mask.SetGray(x, y, color.Gray{255})
					}
				}
			}
			if err = writePNG(name, im); err != nil {
				return err
			}
			if err = writePNG(entry.STP, mask); err != nil {
				return err
			}
		}
		metadata.Uses = append(metadata.Uses, use)
		semantic := textureKey{upload: k.upload, depth: k.depth}
		if !seen[semantic] {
			seen[semantic] = true
			manifest.Entries = append(manifest.Entries, entry)
		}
	}
	b, e := json.MarshalIndent(metadata, "", "  ")
	if e != nil {
		return e
	}
	if err = write("dump.json", append(b, '\n')); err != nil {
		return err
	}
	b, e = json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > textureManifestLimit {
		return fmt.Errorf("傾印 manifest 超過 %d bytes", textureManifestLimit)
	}
	if err = write("manifest.json", append(b, '\n')); err != nil {
		return err
	}
	if err = os.Remove(marker); err != nil {
		os.Remove(filepath.Join(directory, "manifest.json"))
		return err
	}
	return nil
}

// CheckTextureDumpDirectory 在開機前排除既有目錄，WriteDirectory 仍再次檢查。
func CheckTextureDumpDirectory(directory string) error {
	if directory == "" {
		return fmt.Errorf("傾印目錄不可空白")
	}
	if _, err := os.Lstat(directory); err == nil {
		return fmt.Errorf("傾印目錄已存在：%s", directory)
	} else if !os.IsNotExist(err) {
		return err
	}
	p, err := os.Stat(filepath.Dir(directory))
	if err != nil {
		return err
	}
	if !p.IsDir() {
		return fmt.Errorf("傾印上層不是目錄")
	}
	return nil
}
