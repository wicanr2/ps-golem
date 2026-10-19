# 018 HD 貼圖傾印與替換

狀態：`CONFORMED`，限本規格、標題素材鏈與正常 M4 路線。2026-10-18 完成來源／快取審查、正式實作與同狀態驗證。
素材包格式與驗證沿用 `019-hd-texture-pack.md`；工具列沿用 `020-gamepad-toolbar.md`。

## 1. 範圍與使用者決定

依 [ps-golem #1](https://github.com/wicanr2/ps-golem/issues/1) 與
[castlevinia_x #1](https://github.com/wicanr2/castlevinia_x/issues/1)，通用機制在 `render/hd/`，
月下素材及製作工具在 castlevinia_x。索引替換為主，全彩補充。
美術製作方式仍未定，不阻塞機制及研究用替換。不得把研究圖當成最終 HD 美術。

2026-10-18 使用者確認：素材包路徑保存於設定檔，下次自動啟用；完整版雙擊啟動。
藍芽配對及手把決策維持 020。所有原版及衍生素材、傾印、畫面和卡片只留本機。
這輪不承諾全遊戲素材清單、最終美術、其他作品相容性或效能優化。

## 2. 證據與限制

基準 main `6271e767c4b34febcc9e925bc0047e203a63543b`；Go 1.24.13，
`psgolem-go-ebiten:1.24-r1`。實際 BIOS、cue、bin 及正常輸入腳本雜湊見
`workplace/hdtex/survey-inputs.sha256`。位址空間為原生 VRAM 1024×512 個 16 位元字。

| 等級 | 結論與來源 |
|---|---|
| 已證實 | 原生 A0 解析只帶尺寸及 16 位元字，不帶深度／CLUT；`internal/gpu/gp0.go` 的 fbWriteStart／fbWriteWord |
| 已證實 | 4／8／15bpp 的解碼及 CLUT 來自繪圖時有效快取；`render/hd/{gp0,raster,draw,poly}.go` |
| 已證實 | 同位址 VRAM 改寫後貼圖／CLUT 快取可保留舊值；`boundary-tests.log`、`spike-tests.log` |
| 已證實 | 複製以 128 原生字分塊；放大後逐子像素遮罩可造成部分寫入；`boundary-test.go.txt` |
| 已證實 | 原生字來源模型對照逐子像素來源 oracle，固定種子 18026、720 組、倍率 1／2／4，未錯認來源；`texture-origin-probe.go`、`texture-origin-probe.log` |
| 已證實 | 自製原型可讓精靈／多邊形使用子貼素、快取 CLUT／bit15 及解除回退；`spike-overlay.json`、`spike-tests.log` |
| 已證實 | 完整 M4 至 field 45784：122265 次完成上傳、22394 筆去重、149960068 字、去重 114775014 字；無中止／pending；`survey-m4.json` |
| 未知 | Alucard 動畫歸屬及全遊戲繪圖／CLUT 覆蓋；上傳數不是素材張數 |

硬體契約依 [psx-spx GPU](https://psx-spx.consoledev.net/graphicsprocessingunitgpu/) 的
Memory Transfer、Texture Caching、Texture Window、Semi-transparency。只採公開介面契約。
新增行為是呈現層契約，不宣稱原版逐週期或新遊戲 parity。

## 3. 上傳與來源狀態

上傳鍵為 `SHA-256(LE32 width_words || LE32 height || exact LE16 incoming words)`。
不含目的座標、命令保留位元或奇數末字高半字；尺寸遮罩及零值展開沿用現有解析器。

- 未啟用非空素材包或傾印時不配置來源平面，既有渲染路徑維持原狀。
- 每個原生 VRAM 字記錄可選上傳物件、原始字內偏移與原始字值。
- A0 期間收集傳入字及接受狀態，上限 1024×512 字。實際接受的寫入立即解除舊來源；被 mask 拒絕的字保留舊來源。
- 只有完整上傳才附新來源。實際寫入值須等於原始字；maskSet 改變資料時保守回退。
- GP1(00／01) 中止 pending，不把已寫入字認成完整上傳；既有 VRAM 與其他有效來源保留。
- 填色、線段、精靈、多邊形實際回寫時解除對應來源；部分子像素改寫也解除整個原生字來源。
- 貼圖快取每次填入四個字時，同步保存四個來源快照。取樣查快取來源，不能查目前 VRAM 的新來源。
- 不新增原生快取刷新，不把替換色彩寫回打包索引／CLUT 區。

### 3.1 複製

來源轉移按原生字、128 字分塊及原命令的橫向／縱向順序計算，與實際放大複製分開。
暫存工作格包含原字、來源與所有子像素是否相同。每塊先讀來源，再寫目的。

- 來源與目的格都均一時，按原 mask 判定轉移來源。
- 全部被拒絕時保留目的來源；全寫入且 maskSet 未改變原字時可保留來源身分。
- 子像素不均一且有 maskEval，或來源已不能確認時，解除來源；不得錯認。
- 未使用 maskEval 的整格覆寫可重新取得均一來源。
- 初始均一格的各子列執行相同 128 字操作，所得來源可等價於原生模型；部分格採保守解除。

固定種子探針比較的是來源是否誤認，不把顏色碰巧相同當成同一來源。

## 4. 取樣與回退

- 先走現有貼圖快取得到原字，再由同次快取來源定位原上傳內的字偏移。
- 4／8bpp 的字內索引位置由實際 E2 視窗及貼圖頁轉換後的 U 決定；15bpp 每字一貼素。
- CLUT 鍵使用快取的前 16／256 字，不能用當前 VRAM。重載時失效鍵，8bpp 轉 4bpp 只取前 16 字。
- 查詢優先順序依 019：指定 CLUT 的全彩、索引、原圖。15bpp 只用無 CLUT 的全彩。
- 多邊形保留原固定小數 UV 到取樣端；未命中時沿用原整數取樣。
- 精靈命中時逐輸出子像素取樣，翻轉時反轉子貼素順序。未命中時仍用原 plot 整格路徑，避免改變背景混色或 mask 行為。
- 子貼素以最近鄰選取，倍率可不同；索引查有效 CLUT，色彩及 bit15 沿用調色、抖色、ABE／ABR、mask 與側邊畫布。
- 不要求替換結果與原圖顏色一致。未指定、空包、完全未命中須與原有 shadow 輸出逐位元相同。

### 4.1 素材查詢單筆快取

狀態：READY。只縮短上述不可變素材包的唯讀查詢，原生狀態與取樣結果不變。

證據為 019 的不可變 TexturePack 及 render/hd/texture_pack.go 的 lookup：
有效 CLUT 全彩優先，未命中再查 indexed；最多兩次唯讀 map 查詢。
純函式輸入為 upload key、depth、完整有效 CLUT。upload 物件的 key 不會改寫。

- 每個 textureTracking 只保存最後一個 upload 物件、depth、32-byte CLUT 及結果指標。
  同三者重複時可重用命中或 nil；不同就照舊查詢。amd64 固定增加 56 bytes。
- cache 不持有原始 words 的邏輯引用，不增加 upload.refs；既有 releaseRaw 照常釋放。
  Go 只多保留至多一個來源描述物件；不能延長原始字資料的保留。
- 先驗證原快取來源與字值，再查 memo。不得以目前 VRAM 來源取代原 texCache 快照。
- E2、來源偏移、CLUT reload、全彩／索引／原圖優先、STP、mask 與子貼素皆不變。
- Lookups、Hits、ReplacedSamples 逐取樣照舊計數，dump.use 每次照舊呼叫。
- 新 configureTextures 建立新 tracking，不跨素材包沿用；只在 shadow 消費端存取。
  不新增無界 map、磁碟操作、原生 CPU／RAM／週期或讀回狀態。

研究驗證：seed 18018、120000 次對直接查詢的比對，涵蓋 nil／empty、重複鍵不同物件、
4／8／15bpp、有效 CLUT 改變、全彩優先、indexed 回退及未命中；傾印／統計不漏計。
完整 HD race 通過 38.814 秒。單次 indexed 微基準由 142.6～163.5 ns 降至 21.20～24.51 ns，
color 與 miss 也下降；交替 key 未見超出原範圍的退化，全部 0 alloc。
收據為 workplace/hdtex/perf/texture-memo-race.log、texture-memo-bench.log。
這是局部查詢量測。後續有效 M4／最後 HD 圖已通過，完整取樣計數與舊值相同；
收據為 perf/poll-v2-m4.log。真實 GUI 早段最低 0.260，速度未過；維持 READY。

## 5. 傾印與資源契約

傾印只在繪圖取樣實際觸及來源時保存深度與有效 CLUT。上傳本身不猜用途。
shadow 消費端收集不可變快照，繪圖熱路徑不讀寫磁碟；關閉時同步消化完命令後輸出。

- 完成上傳按鍵去重記錄字尺寸、最近位置、首次／最近 field、次數。最多 65536 個鍵。
- 使用組合按 `(upload, depth, effective CLUT)` 去重，最多 4096 個；原始資料只按 upload 保留一次，上限 256 MiB。
- 15bpp 不帶 CLUT；其他組合保存原 CLUT 16 位元字與其鍵。
- field 編號由機器執行緒在 RunUntilField 前附送，只作觀測，不回寫機器。不是 GP1 隔行奇偶訊號。
- 新輸出目錄必須不存在。輸出 PNG、019 v1 manifest 及 `dump.json`；失敗回報，不覆寫既有資料。
- `dump.json` 為版本 1 的傾印中繼資料，包含座標空間、上傳計數、使用組合、原 CLUT、首次／最近 field；不稱為語意素材清單。
- 索引預覽保留原索引。相同 upload／depth 的不同 CLUT 各留預覽，manifest 只選一個索引條目，避免重複鍵。
- 15bpp 輸出標準 RGBA＋STP，可由 019 直接載入。索引預覽調色盤不改變執行時語意。
- PNG 與 JSON 總輸出上限 512 MiB。容量或 IO 錯誤使本次傾印失敗，目錄標示未完成；manifest 最後寫入，沒有它不能當成完整素材包。

未被繪圖使用的上傳只登錄統計，不保留全套原始字。來源物件不再被 VRAM、快取或傾印引用時可回收。
不能由容量限制或一次遊玩路線推論全遊戲覆蓋。

## 6. CLI、設定與完整版

- `psgolem`、`psexe` 增加 `-textures <素材包目錄>`、`-dump-textures <新輸出目錄>`。
- 指定任一功能且 HD=0 時使用影子倍率 1，沿用既有寬螢幕的啟用規則。PGXP 仍須倍率至少 2。
- psgolem 的可選 `textures` 欄位保存路徑；命令列明確指定優先，`-textures=` 清除並保存。傾印路徑不保存。
- 可攜設定的素材路徑相對於包根解析及寫回，與 BIOS／cue 同樣可搬移；空路徑保持空。
- 素材包整包驗證後才開機；損壞不靜默忽略。缺單項條目正常回退。
- GPU、RAM、CPU、時間、讀回、記憶卡格式與原生摘要不受影響。寬螢幕例外仍依 017。
- 本機完整版沿用已授權素材組裝流程，雙擊自動找到 BIOS／cue 並載入已保存素材路徑。
  沒有定版 HD 美術時不可把研究替換預設裝進完整版。

垂直鏈：原始 A0 → 完整上傳來源 → 繪圖深度／CLUT → 本機預覽與 manifest →
研究用 HD 圖 → 素材包驗證 → 精靈／多邊形呈現 → 設定重讀 → 原生存檔不變。

## 7. 驗收

| 範圍 | 最小充分檢查 |
|---|---|
| 來源 | 奇數 padding、位置無關鍵、尺寸遮罩、零值、環繞、GP1 中止、mask 接受／拒絕、填色／繪圖解除 |
| 複製／快取 | 128 字重疊、均一／部分子像素遮罩、舊貼圖／CLUT 快取與刷新 |
| 取樣 | 4／8／15bpp、精靈／多邊形、子貼素、翻轉、E2、環繞、倍率、PGXP、CLUT 全彩／索引／原圖回退 |
| 傾印 | 真實 PNG 往返到 019，CLUT 多組合不產生重複索引鍵，容量、目錄存在與寫入失敗 |
| 不變 | 未指定／空包／未命中與原 shadow 逐位元相同；CPU、1150 筆 GTE 與 17 個 N=1 GPU 語料仍通過 |
| 正常路徑 | 真實光碟正常按鍵傾印一項使用素材，研究替換與關閉回退截圖；完整 M4 原生摘要 `6c35c2701228722f`、原生畫面／卡片不變 |
| 前端／交付 | 可攜設定往返、正常視窗啟動、三平台編譯，更新本機完整版後從封包啟動；Windows／macOS 真機限制沿用原決定 |

通過只聲明本規格與實際驗證範圍。Alucard 動畫、最終美術及其他場景未知保留，不延伸成全遊戲 HD 已完成。
效能不另開里程碑。入口為 `000-index.md`，歷程追加唯一 `WORKLOG.md`，目前狀態在 castlevinia_x `docs/hd-assets-evaluation.md` §8。

## 8. 實作與驗收結果（2026-10-18）

| 範圍 | 結果與本機入口 |
|---|---|
| 正式來源／取樣／傾印 | `texture_tracking.go`、`texture_dump.go`，接入 shadow 消費端；真 PNG、4／8／15bpp、來源解除、快取、mask／copy、STP、精靈／多邊形與可攜保存測試通過；`texture-cli-tests.log` |
| 原生探針交叉核對 | 標題 1357 鍵、每鍵次數及首次／最近 field 與原生探針完全相同；3302 次完成上傳，51 使用組合、49 可載入條目 |
| 同狀態替換 | 原生／HD2／研究替換／關閉四次正常開機均為 field 4320，摘要 `9ac6fa7f0133daec`；RAM 與原生 VRAM 逐位元相同，關閉回復原 HD2；研究圖改變 25360 個輸出像素；`texture-title-verification.json` |
| 完整 M4 | 空白卡、既有正常按鍵、HD2＋PGXP＋研究替換至 field 45784，摘要 `6c35c2701228722f`，卡片與原生畫面 SHA-256 符合既有原生收據；上傳 122265 次且確有替換命中；`texture-m4-verification.json`、`texture-m4.log` |
| 回歸與競態 | 全專案及 HD／前端／兩個 CLI race 通過；`texture-regression.log`、`texture-race.log` |
| 玩家與封包 | 新版 Linux 正式完整版零參數開機；F1／取消／關窗及驗證副本的素材／手把保存重啟通過；完整預設 HD2／PGXP／xBR／寬螢幕 GUI 到標題；`texture-zeroargs-receipt.log`、`texture-full-title-receipt.log` |

總入口為 `workplace/hdtex/texture-integration-verification.json`，來源、輸入與輸出雜湊
為 `texture-integration-evidence.sha256`，重跑入口為 `texture-integration-commands.txt`。
新版與 macOS 結構、AppImage payload 結果另見 005 §9。
研究素材不預設裝進完整版。全遊戲素材覆蓋、最終美術與實體 USB／藍芽仍未驗證，不能據此宣稱完成。
