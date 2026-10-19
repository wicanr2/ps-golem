package hd

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 素材包格式與資源界線：019-hd-texture-pack。這一層不接觸模擬器狀態。
const (
	textureManifestLimit = 1 << 20
	textureEntryLimit    = 4096
	texturePixelLimit    = 16 << 20
	textureFileLimit     = 64 << 20
	textureMemoryLimit   = 256 << 20
)

type textureManifest struct {
	Version int           `json:"version"`
	Entries []textureSpec `json:"entries"`
}

type textureSpec struct {
	Upload string `json:"upload_sha256"`
	Width  int    `json:"width_words"`
	Height int    `json:"height"`
	Depth  int    `json:"depth"`
	Scale  int    `json:"scale"`
	Kind   string `json:"kind"`
	CLUT   string `json:"clut_sha256,omitempty"`
	Image  string `json:"image"`
	STP    string `json:"stp,omitempty"`
}

type textureKey struct {
	upload, clut [32]byte
	depth        int
	color        bool
}

type textureImage struct {
	width, height, scale int
	indices              []byte
	colors               []uint16
}

// TexturePack 是已完整驗證的不可變素材包。圖素切片不對外暴露。
// 格式：019-hd-texture-pack。GPU 接入依 018-hd-texture-replacement。
type TexturePack struct {
	entries map[textureKey]*textureImage
}

// Len 回傳已載入的素材條目數。
func (p *TexturePack) Len() int {
	if p == nil {
		return 0
	}
	return len(p.entries)
}

// LoadTexturePack 從本機目錄的 manifest.json 整包驗證並載入。
// 失敗一律回傳 nil pack，不改寫輸入或交回部分結果。
func LoadTexturePack(directory string) (*TexturePack, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("素材包目錄：%w", err)
	}
	manifestPath, err := textureFile(root, "manifest.json")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest.json：%w", err)
	}
	b, err := io.ReadAll(io.LimitReader(f, textureManifestLimit+1))
	f.Close()
	if err != nil {
		return nil, err
	}
	if len(b) > textureManifestLimit {
		return nil, fmt.Errorf("manifest.json 超過 %d bytes", textureManifestLimit)
	}
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("manifest.json 不是有效 UTF-8")
	}
	if err = textureUniqueJSON(b); err != nil {
		return nil, fmt.Errorf("manifest.json：%w", err)
	}
	var m textureManifest
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest.json：%w", err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("manifest.json：不支援 version=%d", m.Version)
	}
	if m.Entries == nil {
		return nil, fmt.Errorf("manifest.json：entries 必須是陣列")
	}
	fields, err := textureSchemaFields(b)
	if err != nil {
		return nil, fmt.Errorf("manifest.json：%w", err)
	}
	if len(m.Entries) > textureEntryLimit {
		return nil, fmt.Errorf("manifest.json：entries 超過 %d", textureEntryLimit)
	}
	p := &TexturePack{entries: make(map[textureKey]*textureImage, len(m.Entries))}
	dimensions := map[[32]byte][2]int{}
	var memory uint64
	for i, s := range m.Entries {
		_, hasCLUT := fields[i]["clut_sha256"]
		_, hasSTP := fields[i]["stp"]
		if (s.Kind == "indexed" && (hasCLUT || hasSTP)) || (s.Depth == 15 && hasCLUT) {
			return nil, fmt.Errorf("entries[%d]：此類型禁止 CLUT 或 STP 欄位，空字串也不接受", i)
		}
		key, w, h, err := s.validate()
		if err != nil {
			return nil, fmt.Errorf("entries[%d]：%w", i, err)
		}
		if _, ok := p.entries[key]; ok {
			return nil, fmt.Errorf("entries[%d]：重複的素材鍵", i)
		}
		dim := [2]int{s.Width, s.Height}
		if old, ok := dimensions[key.upload]; ok && old != dim {
			return nil, fmt.Errorf("entries[%d]：相同上傳鍵的原始尺寸不同", i)
		}
		dimensions[key.upload] = dim
		cost := uint64(w) * uint64(h)
		if key.color {
			cost *= 2
		}
		if memory+cost > textureMemoryLimit {
			return nil, fmt.Errorf("entries[%d]：解碼資料超過 %d bytes", i, textureMemoryLimit)
		}
		img, err := texturePNG(root, s.Image, w, h)
		if err != nil {
			return nil, fmt.Errorf("entries[%d] image：%w", i, err)
		}
		e := &textureImage{width: w, height: h, scale: s.Scale}
		if !key.color {
			pal, ok := img.(*image.Paletted)
			if !ok {
				return nil, fmt.Errorf("entries[%d] %s：需要保留索引的 PNG", i, s.Image)
			}
			e.indices = make([]byte, w*h)
			limit := 1 << s.Depth
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					index := pal.ColorIndexAt(x, y)
					if int(index) >= limit {
						return nil, fmt.Errorf("entries[%d] %s (%d,%d)：索引 %d 超過 %dbpp", i, s.Image, x, y, index, s.Depth)
					}
					e.indices[y*w+x] = index
				}
			}
		} else {
			stp, err := texturePNG(root, s.STP, w, h)
			if err != nil {
				return nil, fmt.Errorf("entries[%d] stp：%w", i, err)
			}
			e.colors = make([]uint16, w*h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					mask := color.NRGBAModel.Convert(stp.At(x, y)).(color.NRGBA)
					if mask.A != 255 || mask.R != mask.G || mask.R != mask.B || (mask.R != 0 && mask.R != 255) {
						return nil, fmt.Errorf("entries[%d] %s (%d,%d)：STP 必須是不透明黑白值", i, s.STP, x, y)
					}
					if c.A != 0 && c.A != 255 {
						return nil, fmt.Errorf("entries[%d] %s (%d,%d)：alpha 必須是 0 或 255", i, s.Image, x, y)
					}
					if c.A == 0 {
						continue
					}
					v := uint16(c.R>>3) | uint16(c.G>>3)<<5 | uint16(c.B>>3)<<10
					if mask.R == 255 {
						v |= 0x8000
					}
					if v == 0 {
						return nil, fmt.Errorf("entries[%d] %s (%d,%d)：無法表達 RGB5=0、STP=0 的不透明黑", i, s.Image, x, y)
					}
					e.colors[y*w+x] = v
				}
			}
		}
		p.entries[key] = e
		memory += cost
	}
	return p, nil
}

func (s textureSpec) validate() (key textureKey, w, h int, err error) {
	key.upload, err = textureDigest(s.Upload)
	if err != nil {
		return key, 0, 0, fmt.Errorf("upload_sha256：%w", err)
	}
	if s.Width < 1 || s.Width > 1024 || s.Height < 1 || s.Height > 512 {
		return key, 0, 0, fmt.Errorf("原生字尺寸超出 1024×512")
	}
	per := 0
	switch s.Depth {
	case 4:
		per = 4
	case 8:
		per = 2
	case 15:
		per = 1
	default:
		return key, 0, 0, fmt.Errorf("depth 必須是 4、8 或 15")
	}
	switch s.Scale {
	case 1, 2, 4, 8:
	default:
		return key, 0, 0, fmt.Errorf("scale 必須是 1、2、4 或 8")
	}
	key.depth = s.Depth
	switch s.Kind {
	case "indexed":
		if s.Depth == 15 || s.CLUT != "" || s.STP != "" {
			return key, 0, 0, fmt.Errorf("indexed 只接受 4／8bpp，禁止 CLUT 與 STP 欄位")
		}
	case "color":
		key.color = true
		if s.STP == "" {
			return key, 0, 0, fmt.Errorf("color 需要 stp")
		}
		if s.Depth == 15 {
			if s.CLUT != "" {
				return key, 0, 0, fmt.Errorf("15bpp 禁止 clut_sha256")
			}
		} else {
			key.clut, err = textureDigest(s.CLUT)
			if err != nil {
				return key, 0, 0, fmt.Errorf("clut_sha256：%w", err)
			}
		}
	default:
		return key, 0, 0, fmt.Errorf("kind 必須是 indexed 或 color")
	}
	w, h = s.Width*per*s.Scale, s.Height*s.Scale
	if uint64(w)*uint64(h) > texturePixelLimit {
		return key, 0, 0, fmt.Errorf("PNG 貼素數超過 %d", texturePixelLimit)
	}
	return key, w, h, nil
}

// encoding/json 接受大小寫不同的欄位別名。schema 只接受宣告的原名，
// 並保留可選欄位是否出現，不能用空字串繞過禁止欄位。
func textureSchemaFields(b []byte) ([]map[string]json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	for name := range root {
		if name != "version" && name != "entries" {
			return nil, fmt.Errorf("未知欄位 %q", name)
		}
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(root["entries"], &entries); err != nil {
		return nil, err
	}
	for i, fields := range entries {
		for name := range fields {
			switch name {
			case "upload_sha256", "width_words", "height", "depth", "scale", "kind", "clut_sha256", "image", "stp":
			default:
				return nil, fmt.Errorf("entries[%d]：未知欄位 %q", i, name)
			}
		}
	}
	return entries, nil
}

func textureDigest(s string) (out [32]byte, err error) {
	if len(s) != 64 || strings.ToLower(s) != s {
		return out, fmt.Errorf("需要 64 位小寫十六進位")
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, fmt.Errorf("需要 64 位小寫十六進位")
	}
	copy(out[:], b)
	return out, nil
}

func textureFile(root, name string) (string, error) {
	if name == "" || name == "." || strings.Contains(name, "\\") || strings.Contains(name, ":") || path.IsAbs(name) || path.Clean(name) != name {
		return "", fmt.Errorf("不合法的包內相對路徑 %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("路徑不能包含 ..：%q", name)
		}
	}
	p, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", fmt.Errorf("%s：%w", name, err)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s 指向素材包外", name)
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s 不是一般檔案", name)
	}
	return p, nil
}

func texturePNG(root, name string, w, h int) (image.Image, error) {
	p, err := textureFile(root, name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > textureFileLimit {
		return nil, fmt.Errorf("%s 超過 %d bytes", name, textureFileLimit)
	}
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", name, err)
	}
	if cfg.Width != w || cfg.Height != h {
		return nil, fmt.Errorf("%s 尺寸 %dx%d，需要 %dx%d", name, cfg.Width, cfg.Height, w, h)
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	im, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", name, err)
	}
	return im, nil
}

// textureUniqueJSON 先拒絕重複屬性，避免 encoding/json 的後值覆蓋前值。
func textureUniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	var value func() error
	value = func() error {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if tok == nil {
			return fmt.Errorf("不接受 JSON null")
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					t, err := d.Token()
					if err != nil {
						return err
					}
					k, ok := t.(string)
					if !ok {
						return fmt.Errorf("物件屬性必須是字串")
					}
					if seen[k] {
						return fmt.Errorf("重複 JSON 屬性 %q", k)
					}
					seen[k] = true
					if err = value(); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err = value(); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("非預期的 JSON 結尾")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("尾隨 JSON 資料")
	}
	return nil
}

func (p *TexturePack) lookup(upload [32]byte, depth int, clut [32]byte) *textureImage {
	if p == nil {
		return nil
	}
	if e := p.entries[textureKey{upload: upload, depth: depth, clut: clut, color: true}]; e != nil {
		return e
	}
	return p.entries[textureKey{upload: upload, depth: depth}]
}

// textureUploadDigest 不包含目的位址與末字 padding，寬度單位為 VRAM 字。
func textureUploadDigest(width, height int, words []uint16) [32]byte {
	h := sha256.New()
	var b [8]byte
	binary.LittleEndian.PutUint32(b[:4], uint32(width))
	binary.LittleEndian.PutUint32(b[4:], uint32(height))
	h.Write(b[:])
	for _, w := range words {
		binary.LittleEndian.PutUint16(b[:2], w)
		h.Write(b[:2])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func textureCLUTDigest(words []uint16) [32]byte {
	h := sha256.New()
	var b [2]byte
	for _, w := range words {
		binary.LittleEndian.PutUint16(b[:], w)
		h.Write(b[:])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
