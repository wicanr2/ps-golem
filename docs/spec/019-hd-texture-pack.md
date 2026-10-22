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


## 8. 內部不可變圖塊共用（CONFORMED，限載入器與合成取樣，2026-10-22）

目的：在原manifest v1、PNG／STP與256MiB常駐資料界線內載入更多已製作素材，不縮小圖片、不改像素、API、CLUT、混色、素材鍵或未知回退。來源為本規格、`render/hd/{texture_pack,texture_tracking,sprite_runs,raster_state}.go`於`3efa0a6`；線性圖素的正式讀取集中於texturePixel，CPU與GPU皆經此入口，023自己的字格切片沿原索引路徑。

已證實：解碼後的圖素可按完全相同的32×32塊共用，逐點重建包含索引及完整RGB555／STP字。圖塊相等須逐字確認，雜湊僅用來查找；索引與色字分開比較。邊緣不足32格填零，但有效寬高、倍率與原取樣範圍保持。實際素材量測由素材專案保存，此公共契約不包含遊戲資料。

載入狀態：原本線性資料仍符合界線時沿原表示。首次即將超過時，只在尚未交回的本機pack內把已驗證圖像編成不可變共用圖塊及每圖uint32索引，釋放原線性切片；後續圖像仍完整通過原JSON、路徑、PNG尺寸、索引、alpha、STP及黑色守門，再編入相同圖塊庫。所有來源條目仍分別驗證，不因圖片路徑或雜湊相同跳過檢查。記憶體容不下即拒絕整包，不回傳部分結果。

常駐計帳包含每個唯一索引塊的1024 bytes、每個唯一色字塊的2048 bytes，以及每圖每格4-byte索引；023編製切片沿原計帳加入同一256MiB。物件、指標表、載入期PNG／編製暫存與建表雜湊不冒充圖素資料或程序峰值。編製建表只在Load內使用，交回前去除，不由renderer延遲寫入或解壓。此分支不放寬4096條目／1MiB／單圖限制，也不聲明已足夠完整遊戲。

原來可接受的線性包保持原表示與判定；新的共用分支只接原線性計帳超界的包。沒有修改輸入檔案、磁碟格式、設定、原生GPU或機器狀態。取樣只按原整數座標查回原圖素，不能改動透明、STP、CLUT或圖元順序。

驗收：合成索引／全彩及邊緣、32格邊界、來源保留、雜湊碰撞候選的逐字守門、編表記憶體界線、觸發轉換及已共用後的晚期非法PNG均測試；超過線性界線但可共用的真PNG／JSON要由正式Load成功，不能只測孤立helper。原HD回歸與race通過。素材專案以現行正式解碼資料逐字比較，再用實際CPU／GPU抽樣取樣與回退；此層完成不等於新美術、正常玩家包或完整HD交付。由既有000索引進入，歷程寫唯一WORKLOG。

§8驗收：render／hdgpu／machine回歸、完整`go test -race -timeout 25m ./render/hd`及實體GPU合成HD4比較通過。首趟race被Go預設10分鐘中止，當時仍在大型真PNG解碼，沒有race報告；只延長驗證逾時，不改產品或刪測例。三平台v.0.7.6乾淨候選已建置，macOS限結構、沒有本批正常遊戲GUI或完整包驗收。

### 8.1 較小不可變圖塊（CONFORMED，限載入與資料取樣，2026-10-23）

目的：完整素材候選在32×32共用表示仍達256MiB界線時，保留所有已製作像素與原manifest v1，減少圖塊內未相同部分造成的重複儲存。本段只調整§8同一機制的內部粒度，對外API、整數取樣及玩家設定沿原契約。

證據與定位：正式來源基準`d2df5c978a6c91398cbc27ca53f477032cfb23b3`，`render/hd/texture_tiles.go`的`textureTileShift`、`textureTileBuilder.add`與`textureTileImage.pixel`；`texture_tracking.go`的`texturePixel`為正式共用取樣入口。來源搜尋已確認圖塊維度由同一組常數推導，沒有其他正式讀寫端寫死32。已證實的完整候選布局計帳見[公開容量工作項目#14](https://github.com/wicanr2/ps-golem/issues/14#issuecomment-6347675709)：32圖塊298355964、16圖塊241254208、8圖塊172765640 bytes，均包含每圖格索引與姓名；量測為私有診斷，不是正式載入或程序／GPU峰值。私人來源與素材不進本repo。

契約：

1. 先沿原線性與32×32共用表示，只有容量不足才在同一次Load內編成8×8共用表示。32圖塊仍為1024／2048 bytes，8圖塊為64／128 bytes，兩者每圖格uint32索引均4 bytes；圖塊庫記錄自身位移量，索引與色字塊分別按該庫實際長度儲存。圖像有效寬高與倍率保持，右／下不足整塊填零，取樣只在原有效範圍。
2. 原線性計帳可接受的包沿原表示，原32共用計帳可接受的包保持32。線性首次即將超界時先編32，只有圖素／圖格資源錯誤才重編8；32追加新條目或023姓名超界時也可轉8。重編讀取已驗證的線性或共用色字，不重讀來源或重新量化；新條目仍經完整PNG驗證。8仍超界即整包拒絕。維持同一256MiB含唯一塊、每圖索引與姓名，物件／指標／載入暫存不冒充常駐計帳。
3. 全部索引或16位色字必須逐值相等才共用。雜湊只選候選，索引與色字分開；0透明、0x8000及其他半透明字完整保留。不能由近似色、CLUT、來源位置或材質名稱推定相等。
4. 所有條目依原JSON、路徑、尺寸、索引、alpha、STP、不透明黑、重複鍵與023守門驗證。只以型別辨識資源界線錯誤觸發重編，其他錯誤原路拒絕。重編僅改未交回的本機pack，途中失敗仍不回傳部分結果。4096條目／1MiB manifest及單圖界線保持。023也沿同一預算，資源不足時可在重編圖像後重新驗證序列，不能略過序列內容守門。
5. renderer讀相同整數座標必須得到原線性表示相同的索引／色字與統計，CPU／GPU取樣、混色、未知上傳／CLUT回退及圖元順序保持。此段不寫CPU／RAM、光碟或存檔。

驗收：

- 公開合成32×32真PNG，由重複8×8細節組合出不同大塊。使用同一私有載入流程的1KiB預算，須容納四張圖，逐像素包含透明與STP；另有先線性再32、32追加容量不足轉8的測例，不只測孤立helper。正式入口固定256MiB，測試預算不從檔案或玩家參數輸入。
- 既有全像素線性／共用比較、雜湊碰撞、索引／色字區分、右下非整塊、圖格／圖素界線，以及從32表示直接重編8後逐像素相同。原32單塊資源測例及原可接受32包保持。
- 既有正式Load的65個真PNG條目觸發共用、完整來源不變與晚期壞PNG拒絕；`render/hd`及`render/hdgpu`回歸。這是一輪容量修正驗證，不重驗舊美術或原版CPU套件。
- 重編後的晚期壞PNG、索引與STP內容仍拒絕整包；圖像與姓名合計超界仍拒絕，不因改粒度略過守門。所有重編及建表只在Load內，交回後沒有延遲寫入、解壓或可變快取。
- 素材專案以未改動的完整候選，接續原定45350912新色字／全部父圖素、原透明／STP、圖塊邊界及四姓名核對；再執行原175個CPU／實體GPU案例，原容量停止的案例數為0。固定實際新來源雜湊，不能把正式引擎已修改說成未修改。

停止線：本段只驗收載入表示及渲染資料等價，不宣稱正常NO0路線、全部新圖可見、速度、完整素材包或整個HD完成。正常玩家路徑、已安裝程式與三平台交付仍沿素材專案當批契約執行。

§8.1驗收：一次`render/hd`及`render/hdgpu`回歸178測例、失敗／skip0。完整未改動候選由正式256MiB入口載入，1757父條目保持32表示，1930新候選條目全部為8表示；45350912預期色字、186130240父圖素、2790144原圖塊邊界值及四姓名規則／像素相同。常駐圖素158240896、圖格14481736、姓名43008，合計172765640 bytes，未修改原界線，非程序／GPU峰值。

原定175個來源衍生CPU／實體Intel GPU案例一次完成，完整VRAM、畫面與統計相同。全部硬體精靈，173個新像素案例與兩個未知控制保持；沒有CPU光柵回退。這是原始來源／CLUT的合成圖元，不是新原GP0／CPU／RAM同場、正常新關卡路線或速度驗收。原32容量停止時資料／GPU案例0，其日誌與來源保留；沒有重跑舊美術或已完成案例。本輪未重跑完整race，先前§8結果不改標為本輪。

實際證據索引為相鄰素材專案`docs/assets-catalog.md`的NO0儲存接續及`engine-storage-acceptance-v1.json`，本公開規格只保存通用契約與驗證摘要。正常玩家包、三平台候選、全部新素材可見性與完整HD仍按素材專案驗收，不由本段CONFORMED推定。
