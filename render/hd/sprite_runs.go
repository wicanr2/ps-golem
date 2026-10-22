package hd

// 通用完整精靈序列替換，契約：023-hd-sprite-runs。
// 序列與圖像由使用者素材包提供；不含遊戲識別、文字或機器掛鉤。
import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type spriteRunSpec struct {
	ID          string           `json:"id"`
	Upload      string           `json:"upload_sha256"`
	WidthWords  int              `json:"width_words"`
	Height      int              `json:"height"`
	TPage       []uint32         `json:"tpage"`
	Offset      []int32          `json:"offset"`
	BufferY256  bool             `json:"buffer_y256"`
	Width       int              `json:"image_width"`
	ImageHeight int              `json:"image_height"`
	Scale       int              `json:"scale"`
	Image       string           `json:"image"`
	Steps       []spriteStepSpec `json:"steps"`
}
type spriteStepSpec struct {
	Words       []uint32 `json:"words"`
	Source      []int    `json:"source"`
	ImageOffset []int    `json:"image_offset"`
}
type spriteRunStep struct {
	words  [4]uint32
	source [2]int
	image  *textureImage
}
type spriteRunRule struct {
	id            string
	upload        [32]byte
	width, height int
	tpage         [2]uint32
	offset        [2]int32
	plane         bool
	steps         []spriteRunStep
}
type SpriteRunStats struct{ Matches, Fallbacks uint64 }
type spriteRunState struct {
	rules   []*spriteRunRule
	rule    *spriteRunRule
	count   int
	plane   uint32
	pending [32][4]uint32
	stats   SpriteRunStats
}

func (p *TexturePack) SpriteRunCount() int {
	if p == nil {
		return 0
	}
	return len(p.spriteRuns)
}

func spriteKeys(raw json.RawMessage, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key := range fields {
		found := false
		for _, name := range names {
			found = found || key == name
		}
		if !found {
			return fmt.Errorf("未知欄位 %q", key)
		}
	}
	return nil
}

func loadSpriteRuns(root string, memory, memoryLimit uint64) ([]*spriteRunRule, error) {
	const name = "sprite-runs.json"
	if _, err := os.Lstat(filepath.Join(root, name)); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	path, err := textureFile(root, name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, textureManifestLimit+1))
	f.Close()
	if err != nil {
		return nil, err
	}
	if len(b) > textureManifestLimit || !utf8.Valid(b) {
		return nil, fmt.Errorf("%s大小或UTF-8無效", name)
	}
	if err = textureUniqueJSON(b); err != nil {
		return nil, err
	}
	if err = spriteKeys(b, "version", "runs"); err != nil {
		return nil, err
	}
	var raw struct {
		Version int               `json:"version"`
		Runs    []json.RawMessage `json:"runs"`
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	if raw.Version != 1 || raw.Runs == nil || len(raw.Runs) > 64 {
		return nil, fmt.Errorf("%s需要version1及最多64個runs", name)
	}
	rules := make([]*spriteRunRule, 0, len(raw.Runs))
	ids := map[string]bool{}
	for i, row := range raw.Runs {
		if err = spriteKeys(row, "id", "upload_sha256", "width_words", "height", "tpage", "offset", "buffer_y256", "image_width", "image_height", "scale", "image", "steps"); err != nil {
			return nil, err
		}
		var s spriteRunSpec
		d := json.NewDecoder(bytes.NewReader(row))
		d.DisallowUnknownFields()
		if err = d.Decode(&s); err != nil {
			return nil, err
		}
		var obj map[string]json.RawMessage
		_ = json.Unmarshal(row, &obj)
		var steps []json.RawMessage
		_ = json.Unmarshal(obj["steps"], &steps)
		for _, step := range steps {
			if err = spriteKeys(step, "words", "source", "image_offset"); err != nil {
				return nil, err
			}
		}
		key, e := textureDigest(s.Upload)
		if e != nil {
			return nil, e
		}
		if s.ID == "" || len(s.ID) > 64 || ids[s.ID] || s.WidthWords < 1 || s.WidthWords > 1024 || s.Height < 1 || s.Height > 512 || len(s.TPage) != 2 || s.TPage[0] > 960 || s.TPage[0]%64 != 0 || (s.TPage[1] != 0 && s.TPage[1] != 256) || len(s.Offset) != 2 || s.Offset[0] < -1024 || s.Offset[0] > 1023 || s.Offset[1] < -1024 || s.Offset[1] > 1023 || s.Width < 1 || s.Width > 256 || s.ImageHeight < 1 || s.ImageHeight > 256 || len(s.Steps) < 1 || len(s.Steps) > 32 {
			return nil, fmt.Errorf("runs[%d]尺寸／序列／識別無效", i)
		}
		if s.Scale != 1 && s.Scale != 2 && s.Scale != 4 && s.Scale != 8 {
			return nil, fmt.Errorf("runs[%d]倍率無效", i)
		}
		cost := uint64(len(s.Steps) * 64 * s.Scale * s.Scale)
		if memory+cost > memoryLimit {
			return nil, &textureBudgetError{"精靈序列超過整包記憶體界線"}
		}
		memory += cost
		rule := &spriteRunRule{id: s.ID, upload: key, width: s.WidthWords, height: s.Height, tpage: [2]uint32{s.TPage[0], s.TPage[1]}, offset: [2]int32{s.Offset[0], s.Offset[1]}, plane: s.BufferY256}
		for _, p := range s.Steps {
			if len(p.Words) != 4 || len(p.Source) != 2 || len(p.ImageOffset) != 2 || p.Words[0] != 0x65000000 || p.Words[3] != 0x80008 || p.Source[0] < 0 || p.Source[0]+8 > s.WidthWords*4 || p.Source[1] < 0 || p.Source[1]+8 > s.Height || p.Words[2]&255 > 248 || (p.Words[2]>>8)&255 > 248 || p.ImageOffset[0] < -2048 || p.ImageOffset[0] > 2048 || p.ImageOffset[1] < -2048 || p.ImageOffset[1] > 2048 {
				return nil, fmt.Errorf("runs[%d]只接受有效8x8原始4bpp精靈", i)
			}
			if s.BufferY256 && (p.Words[1]>>16)&0xffff >= 256 {
				return nil, fmt.Errorf("可選256列平移的基準Y須小於256")
			}
			if len(rule.steps) > 0 && p.Words[2]>>16 != rule.steps[0].words[2]>>16 {
				return nil, fmt.Errorf("同一序列CLUT位址必須相同")
			}
			rule.steps = append(rule.steps, spriteRunStep{words: [4]uint32{p.Words[0], p.Words[1], p.Words[2], p.Words[3]}, source: [2]int{p.Source[0], p.Source[1]}})
		}
		for _, prior := range rules {
			if prior.steps[0].words == rule.steps[0].words {
				return nil, fmt.Errorf("序列起始命令不得有歧義")
			}
		}
		im, e := texturePNG(root, s.Image, s.Width*s.Scale, s.ImageHeight*s.Scale)
		if e != nil {
			return nil, e
		}
		pal, ok := im.(*image.Paletted)
		if !ok {
			return nil, fmt.Errorf("序列圖像必須保留索引")
		}
		for y := 0; y < pal.Rect.Dy(); y++ {
			for x := 0; x < pal.Rect.Dx(); x++ {
				if pal.ColorIndexAt(x, y) > 15 {
					return nil, fmt.Errorf("序列圖像索引超過4bpp")
				}
			}
		}
		for n, p := range s.Steps {
			size := 8 * s.Scale
			entry := &textureImage{width: size, height: size, scale: s.Scale, indices: make([]byte, size*size)}
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					sx, sy := p.ImageOffset[0]*s.Scale+x, p.ImageOffset[1]*s.Scale+y
					if image.Pt(sx, sy).In(pal.Rect) {
						entry.indices[y*size+x] = pal.ColorIndexAt(sx, sy)
					}
				}
			}
			rule.steps[n].image = entry
		}
		ids[s.ID] = true
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *spriteRunRule) matches(cb []uint32, n int) bool {
	if len(cb) != 4 || n >= len(r.steps) {
		return false
	}
	want := r.steps[n].words
	for i, w := range want {
		v := cb[i]
		if i == 1 && r.plane {
			v &^= 1 << 24
		}
		if v != w {
			return false
		}
	}
	return true
}
func (g *Renderer) spriteRunEnvironment(r *spriteRunRule) bool {
	return g.texMode == 0 && !g.texDisable && g.spriteFlip == 0 && g.tww == 0 && g.twh == 0 && g.twx == 0 && g.twy == 0 && g.texPageX == r.tpage[0] && g.texPageY == r.tpage[1] && g.offsX == r.offset[0] && g.offsY == r.offset[1]
}

// 在私有快取副本中重建原始取樣來源，不改真實快取的引用、來源字或統計。
func (g *Renderer) spriteRunSource(r *spriteRunRule, cb [][4]uint32) bool {
	cache := g.texCache
	for n, words := range cb {
		u0, v0 := words[2]&255, (words[2]>>8)&255
		for y := uint32(0); y < 8; y++ {
			for x := uint32(0); x < 8; x++ {
				u := ((u0 + x) & g.twxAnd) + g.twxAdd
				fy := ((v0 + y) & g.twyAnd) + g.twyAdd
				fx := (u >> 2) & 1023
				gro := fy*1024 + fx
				line := gro &^ 3
				c := &cache[(gro>>2)&3|(gro>>8)&0xfc]
				if c.tag != line {
					cx := fx &^ 3
					for i := uint32(0); i < 4; i++ {
						c.data[i] = g.texel(cx+i, fy)
						c.origin[i] = g.textures.origins[(fy&511)*1024+cx+i]
					}
					c.tag = line
				}
				o := c.origin[gro&3]
				if o.upload == nil || o.word != c.data[gro&3] || o.upload.key != r.upload || int(o.upload.w) != r.width || int(o.upload.h) != r.height || int((o.offset%o.upload.w)*4+(u&3)) != r.steps[n].source[0]+int(x) || int(o.offset/o.upload.w) != r.steps[n].source[1]+int(y) {
					return false
				}
			}
		}
	}
	return true
}

func (g *Renderer) flushSpriteRun() {
	s := g.spriteRun
	if s == nil || s.count == 0 {
		return
	}
	n := s.count
	s.count = 0
	s.rule = nil
	s.stats.Fallbacks++
	for i := 0; i < n; i++ {
		g.executeRaw(s.pending[i][:])
	}
}

// 023：只有完全重送目前狀態的 E1／零視窗 E2 可穿過待完成序列。
// setTPage 的模式與頁相同時不清快取；其餘有效位元也須相同。
func (g *Renderer) spriteRunNoop(cb []uint32) bool {
	if len(cb) != 1 || !g.spriteRunEnvironment(g.spriteRun.rule) {
		return false
	}
	if cb[0] == 0xe2000000 {
		return true
	}
	want := uint32(0xe1000000) | g.texPageX/64 | g.texPageY>>4 | g.texMode<<7 | g.abr<<5 | g.spriteFlip
	if g.dtd {
		want |= 1 << 9
	}
	if g.dfe {
		want |= 1 << 10
	}
	return cb[0] == want
}

func (g *Renderer) acceptSpriteRun(cb []uint32) bool {
	s := g.spriteRun
	if s == nil {
		return false
	}
	if s.count > 0 {
		if g.spriteRunNoop(cb) {
			g.executeRaw(cb)
			return true
		}
		if !s.rule.matches(cb, s.count) || !g.spriteRunEnvironment(s.rule) || (s.rule.plane && cb[1]&(1<<24) != s.plane) {
			g.flushSpriteRun()
		} else {
			copy(s.pending[s.count][:], cb)
			s.count++
			if s.count == len(s.rule.steps) {
				g.finishSpriteRun()
			}
			return true
		}
	}
	for _, rule := range s.rules {
		if rule.matches(cb, 0) && g.spriteRunEnvironment(rule) {
			s.rule = rule
			s.plane = cb[1] & (1 << 24)
			copy(s.pending[0][:], cb)
			s.count = 1
			if len(rule.steps) == 1 {
				g.finishSpriteRun()
			}
			return true
		}
	}
	return false
}

func (g *Renderer) finishSpriteRun() {
	s := g.spriteRun
	rule, n := s.rule, s.count
	g.updateCLUT(s.pending[0][2]>>16, 0)
	if g.clutCache[0] != 0 || !g.spriteRunSource(rule, s.pending[:n]) {
		g.flushSpriteRun()
		return
	}
	s.count = 0
	s.rule = nil
	s.stats.Matches++
	for i := 0; i < n; i++ {
		g.runImage = rule.steps[i].image
		g.runUV = [2]uint32{s.pending[i][2] & 255, (s.pending[i][2] >> 8) & 255}
		g.executeRaw(s.pending[i][:])
		g.runImage = nil
	}
}

func (s *Shadow) SpriteRunStatistics() (stats SpriteRunStats) {
	s.read(func(g *Renderer) {
		if g.spriteRun != nil {
			stats = g.spriteRun.stats
		}
	})
	return
}
