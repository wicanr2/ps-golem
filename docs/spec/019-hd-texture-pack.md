# 019 HD 素材包格式與載入

狀態：`CONFORMED`，限素材包載入與驗證層。GPU 替換、傾印與 CLI 另依 CONFORMED `018-hd-texture-replacement.md` 接入，不屬本載入器層驗收。

## 1. 決定與範圍

2026-10-18 使用者授權 Codex 比較工具相容性後定案。採用 UTF-8 JSON manifest、
保留索引的 PNG，以及全彩 RGBA PNG 加獨立黑白 STP 遮罩。
索引為主、全彩補充、只留本機等範圍沿用 ps-golem #1 與 castlevinia_x #1，不重新選擇美術方向。

已證實：Go 1.24.13 `image/png` 與 Pillow 11.3.0 可原值往返保存上述 PNG。
特殊 alpha=128 在一般預覽會減半顏色，與 PS1 依命令選用的混色規則不同。
兩種方案經 bicubic 縮放都會產生需量化的中間值，因此正式載入器只接受明確的離散值。
工具、輸入與結果在 `workplace/hdtex/{format-compatibility.json,png-tests.log,png-test.go.txt}`。
測試圖全部自製，不含原版素材；位址空間不適用。

PNG 索引可能有相同的預覽 RGB，仍須保留不同索引值。Go PNG 反例已確認可原值往返。
本規格是新增輸入格式契約，不宣稱原版檔案格式或原版畫面 parity。

## 2. manifest v1

素材包目錄內唯一入口為 `manifest.json`：

```json
{
  "version": 1,
  "entries": [
    {
      "upload_sha256": "<64 位小寫十六進位>",
      "width_words": 1,
      "height": 2,
      "depth": 4,
      "scale": 4,
      "kind": "indexed",
      "image": "images/example-indexed.png"
    },
    {
      "upload_sha256": "<64 位小寫十六進位>",
      "width_words": 1,
      "height": 2,
      "depth": 4,
      "scale": 4,
      "kind": "color",
      "clut_sha256": "<64 位小寫十六進位>",
      "image": "images/example-color.png",
      "stp": "images/example-stp.png"
    }
  ]
}
```

| 欄位 | 契約 |
|---|---|
| version | 整數 1 |
| entries | 陣列，可空，不可 null 或省略 |
| upload_sha256 | `SHA-256(LE32 width_words || LE32 height || exact LE16 incoming words)`；不含目的座標或奇數末字填充 |
| width_words／height | 原生 VRAM 字尺寸，分別 1–1024／1–512，不是貼素寬度 |
| depth | 4、8、15，分別每字 4、2、1 個貼素；索引類型只接受 4、8 |
| scale | 1、2、4、8，表示素材倍率，不要求與 renderer 倍率相同 |
| kind | indexed 或 color |
| image | 包內 PNG 相對路徑 |
| clut_sha256 | color 且 depth 為 4／8 時必填，其他情況禁止；雜湊對象為有效 CLUT 的 16／256 個 LE16 字 |
| stp | color 必填，indexed 禁止；同尺寸黑白 PNG |

欄位名稱區分大小寫。未知欄位、重複 JSON 屬性、尾隨 JSON、無效 UTF-8 與錯誤型別一律拒絕。
禁止欄位即使是空字串也拒絕，不把明確存在當成省略。
語意重複鍵為 `(upload_sha256, depth, kind, clut_sha256)`，即使倍率或檔名不同仍拒絕。
同一 upload_sha256 的原始尺寸必須一致。JSON 屬性順序不影響行為。

## 3. PNG 與顏色

尺寸必須是 `(width_words × 每字貼素數 × scale, height × scale)`。

### 3.1 indexed

PNG 必須解碼成 `image.Paletted`，逐貼素保留 ColorIndex，不經 RGB 反查或合併同色索引。
4bpp 每個索引必須 ≤15；8bpp ≤255。PNG 的預覽調色盤與 alpha 不參與執行時顏色。
執行時使用遊戲實際有效的 CLUT，透明與 bit15 由 CLUT 原字決定，不把索引 0 固定當透明。

### 3.2 color

PNG 解碼成未預乘的 NRGBA 值；alpha 只接受 0 或 255。
STP 圖每個貼素必須是不透明的純黑或純白，允許灰階或 RGB PNG，不接受彩色與中間灰階。

```text
alpha = 0: output = 0
alpha = 255: output = (R >> 3) | ((G >> 3) << 5) | ((B >> 3) << 10) | (STP << 15)
```

STP 黑為 0，白為 1。RGB 轉換沿用現有影子 GPU 5 位元通道。
半透明混色仍由原繪圖命令的 ABE／ABR 決定，載入器不執行一般 alpha blending。
量化後 RGB=0、STP=0、alpha=255 無法表達不透明黑，拒絕該貼素，指出座標；不自動改色或改 STP。
alpha=0 的貼素仍檢查 STP 圖格式，但 RGB 不受黑色限制。

依據：`render/hd/raster.go` 的 getTexel、blend、plotScaled，及
[psx-spx GPU](https://psx-spx.consoledev.net/graphicsprocessingunitgpu/) 的 Texture Color Black Limitations、Semi-transparency。

## 4. 邊界與失敗行為

- 先整包驗證再交回不可變 TexturePack，任何錯誤都不交回部分結果。
- manifest 上限 1 MiB、4096 個條目；單張 PNG 上限 16 Mi 貼素、64 MiB 壓縮檔；
  整包常駐解碼資料上限 256 MiB。這些是初版載入器資源界線，不是磁碟檔案格式的編碼上限。
- PNG 先 DecodeConfig 檢查尺寸與資源界線，再完整解碼，防止先分配越界影像。
- 路徑使用 `/`，不接受絕對路徑、反斜線、空路徑、`.`、`..` 或非正規路徑。
  解開符號連結後仍須位於包內，只讀一般檔案。不改寫輸入。
- 錯誤指出 manifest 條目或 PNG 檔名及原因。缺對應素材的查詢回傳未命中，屬正常回退。
- 查詢先找符合 upload／depth／CLUT 的 color，否則找同 upload／depth 的 indexed。
  15bpp 只找無 CLUT 的 color。

## 5. 程式與驗收

程式放 `render/hd/texture_pack.go`，公開入口 `LoadTexturePack(directory)`。
回傳資料只能由 loader 建立，內部圖素不對外暴露可寫切片。
本層不依賴 machine；GPU、CLI 與 settings 的接入契約另在 018。

必要測試：

- 真實 PNG／JSON 的索引值、重複預覽色、4／8bpp、色彩與 STP、透明、倍率與尺寸。
- 同圖不同 CLUT 的優先順序、無對應回退、15bpp 無 CLUT、空包。
- 錯誤版本、未知或重複屬性、尾隨資料、無效 UTF-8、型別、雜湊、重複鍵、深度與尺寸。
- RGB 索引圖、越界索引、非離散 alpha／STP、不透明黑、壞 PNG、DecodeConfig 超限。
- 路徑逃逸、包外連結、來源檔案未改寫，以及整包失敗無部分結果。

測試只用自製素材。Go race 與現有 HD 測試通過後，標示這一層完成；
不因此關閉 HD Issue、不聲明素材替換或正式美術完成。

## 6. 決策與下一層

採用獨立 STP 遮罩，排除特殊 alpha 編碼。依據為使用者授權與 §1 的實測相容性。
未定的 AI／手繪製作方式不阻塞本格式載入器。
GPU 上傳、來源、快取、傾印與真實遊戲替換已依 `018` 通過 READY 閘門，整合驗收另依該規格。
入口登錄於 `000-index.md`；本輪歷程寫入唯一 `WORKLOG.md`。

## 7. 驗收結果（2026-10-18）

`render/hd/texture_pack.go` 與 `texture_pack_test.go` 已完成 §2–5 的格式及失敗行為。
`pack-tests.log`、`pack-regression.log`、`pack-race.log` 均通過；全專案回歸另見
`gamepad-regression.log`。測試圖均為自製，不含原版素材。

工具為 Go 1.24.13 與 Pillow 11.3.0，位址空間不適用。固定來源、輸出與日誌雜湊、
可重跑入口見 `workplace/hdtex/{implementation-verification.json,implementation-evidence.sha256,implementation-commands.txt}`。
這些結果只證明本載入器契約，不代表遊戲已顯示 HD 替換圖或美術管線完成。
