package sotn

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/ps-golem/render/hd"
)

// 正常按鍵從空白記憶卡重播，不做 direct-entry、RAM 或座標注入。
// 原版與衍生素材缺檔即 skip；本機衍生證據不可加入 Git。
func TestTextureM4PreservesNative(t *testing.T) {
	bios, cue, packPath, out := os.Getenv("PSGOLEM_BIOS"), os.Getenv("SOTN_CUE"), os.Getenv("PSGOLEM_TEXTURE_PACK"), os.Getenv("PSGOLEM_TEXTURE_EVIDENCE")
	if os.Getenv("PSGOLEM_LONG") == "" || bios == "" || cue == "" || packPath == "" || out == "" {
		t.Skip("需要實際原版、研究包、證據目錄與 PSGOLEM_LONG=1")
	}
	for _, p := range []string{bios, cue, filepath.Join(packPath, "manifest.json")} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Skip("缺少實際輸入", p)
		} else if err != nil {
			t.Fatal(err)
		}
	}
	pack, err := hd.LoadTexturePack(packPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/m4-first-save.press")
	if err != nil {
		t.Fatal(err)
	}
	press, err := ParseScript(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := Boot(bios, cue, nil)
	if err != nil {
		t.Fatal(err)
	}
	shadow := hd.NewShadowWithTextures(1, pack, nil)
	m.GPU.SetShadow(shadow)
	defer func() { m.DisablePGXP(); m.GPU.SetShadow(nil); shadow.Close() }()
	if err = m.EnablePGXP(); err != nil {
		t.Fatal(err)
	}
	for m.Field() < 45784 {
		m.GPU.TextureField(m.Field())
		Step(m, ButtonsAt(press, m.Field()))
		if m.Field()%6000 == 0 {
			shadow.Screen()
			t.Logf("正常重播 field=%d", m.Field())
		}
	}
	digest := m.Digest()
	cardHash := fmt.Sprintf("%x", sha256.Sum256(m.Card1.Data[:]))
	nativeHash := fmt.Sprintf("%x", sha256.Sum256(m.GPU.Screen().Pix))
	if !strings.HasSuffix(digest, "6c35c2701228722f") {
		t.Error("native digest changed", digest)
	}
	if cardHash != "5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36" {
		t.Error("native card changed", cardHash)
	}
	if nativeHash != "02b9bc2db161fa5675d867aeec057ccebd5cb9f6912db0fe46ed159d78c8ddd7" {
		t.Error("native screen changed", nativeHash)
	}
	dir := m.Card1.Data[0x80:0x100]
	if dir[0] != 0x51 || string(dir[0x0a:0x1c]) != "BISLPM-86023DRAX00" {
		t.Error("normal first save failed")
	}
	stats := shadow.TextureStats()
	if stats.Hits == 0 || stats.ReplacedSamples == 0 {
		t.Error("research pack was never used", stats)
	}
	writeHDPNG(t, filepath.Join(out, "texture-m4-native.png"), m.GPU.Screen())
	writeHDPNG(t, filepath.Join(out, "texture-m4-hd2-pgxp.png"), shadow.Screen())
	receipt := map[string]any{"field": m.Field(), "digest": digest, "card_sha256": cardHash, "native_screen_sha256": nativeHash, "stats": stats, "path": "normal m4-first-save.press from boot and blank card; HD2+PGXP+research texture"}
	b, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "texture-m4-verification.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}
