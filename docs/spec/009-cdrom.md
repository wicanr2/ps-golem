# 009 CD-ROM

狀態：`CONFORMED`（2026-10-15，見 §6）

M4 的第一段（`001-scope-and-mvp` §4）。BIOS 在 SCE 開場畫面後送 CD 命令 `19h,20h`（查控制器版本）
就等中斷，沒有 CD-ROM 就停在開場畫面（2026-10-15 以 `PSGOLEM_TRACE_CD` 追蹤確認）。

## 1. 光碟映像（`internal/disc`）

- 讀 cue／bin。只支援單一 bin、`MODE2/2352` 資料軌與 `AUDIO` 音軌（本案例的格式，`001` §6）。
- cue 內的檔名找不到時（本案例：Big5 編碼、大寫 `.BIN`），退回 cue 同目錄唯一的 `.bin`，並回報這次退回。
- LBA 與檔案扇區：cue 的 INDEX 是相對 bin 開頭的 MSF，LBA = 該 MSF 換算的扇區數；
  第一軌 INDEX 01 為 LBA 0；lead-out = bin 的總扇區數。LBA < 0（第一軌之前的 2 秒）回傳全 0 的扇區。
- Sub-channel Q 由 TOC 合成（bin 不含 sub-channel）：控制位元（資料軌 `41h`、音軌 `01h`）、
  軌號、index、軌內相對 MSF、絕對 MSF（LBA + 150），全為 BCD。
- 區域字串（`GetID` 回報的 SCEx）：讀 ISO9660 根目錄的 `SYSTEM.CNF`，依 `BOOT` 檔名前綴判斷：
  第 3 字元 `P` → `SCEI`、`U` → `SCEA`、`E` → `SCEE`（照 Beetle `libretro.c` 的 CalcDiscSCEx_BySYSTEMCNF 的做法）。
  本案例 `SLPM_860.23` → `SCEI`。

## 2. 控制器（`internal/cdrom`）

移植 `beetle-psx-libretro` 的 `mednafen/psx/cdc.c`（commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，
GPL-2.0-or-later）：

- 暫存器（psx-spx `cdromdrive.md`）：4 個 bank、參數 FIFO 16 位元組、結果 FIFO 16 位元組、
  中斷旗標（INT1–5）與遮罩、資料 FIFO（`HCHPCTL` bit 7 載入目前扇區）、DMA channel 3。
- 命令處理的階段與時間（參數逐一接收 1815 週期、命令開始 8500 週期、`CDCReadyReceiveCounter` 2000）。
- 命令 `00h`–`1Eh` 的行為與 Beetle 相同，含 Setloc、ReadN／ReadS、Pause、Stop、Init（`0Ah`）、
  Setmode、Setfilter、GetlocL／P、GetTN、GetTD、SeekL／P、Test（`19h` 的子函式）、GetID、ReadTOC、Play。
- 讀取扇區：每 33868800／75（倍速 ÷2）週期一個扇區；兩格扇區管線；
  Setmode bit 5／4 決定送 2048、2328 或 2340 位元組；Setmode bit 6（XA 串流）時，符合篩選的 XA 音訊扇區
  不送給程式（之後交給 SPU，見 `010`）。
- 與 Beetle 不同之處：Beetle 在命令延遲、尋軌時間加亂數（`PSX_GetRandU32`）。本專案要決定性，
  亂數一律取區間中點（命令延遲 +1500、尋軌 +12500、Reset +1625000），寫成常數。

不在本規格：XA-ADPCM 解碼、CD-DA 送 SPU（`010`）、EDC／ECC 檢查（映像檔視為無錯）。

## 3. 時間

控制器的計數以 CPU 週期推進（`001` §3.1）。機器每執行完一條指令把週期數交給控制器。

## 4. 驗收

| 項目 | 判準 |
|---|---|
| BIOS 開機，掛《月下》光碟 | 走過 SCE 開場，畫出 PlayStation 標誌畫面，並開始執行光碟上的 `SLPM_860.23`（PC 進入該檔的載入範圍） |
| ps1-tests `cdrom/getloc` | 有 `psx.log` 時與之比對；沒有就只記錄輸出（待查 tests.zip 內容） |
| `disc` 單元測試 | cue 解析（含檔名退回）、TOC、LBA↔MSF、合成 Sub-Q，用合成的小 bin 測，不需要原版素材 |

## 5. 介面

```go
package disc
func Open(cuePath string) (*Disc, error)
func (d *Disc) ReadSector(lba int32, out *[2352]byte)
func (d *Disc) SubQ(lba int32) [12]byte
func (d *Disc) Region() string // "SCEI"／"SCEA"／"SCEE"；不是 PS1 光碟回 ""

package machine
func (m *Machine) InsertDisc(d *disc.Disc)
```

`cmd/psexe` 加 `-cue <path>`：不側載 EXE 時，以光碟開機。

## 6. 驗收結果（2026-10-15）

- 掛《月下》光碟開機：走過 SCE 開場、PlayStation 標誌畫面（「Licensed by Sony Computer Entertainment Inc.」「SCEI」），
  BIOS 載入並執行 `SLPM_860.23`，加上 `010` 的手把後到達遊戲標題畫面。
- cue 檔名退回：本案例的 Big5 檔名找不到，自動改用 `sotn.bin` 並回報。
- `internal/disc` 單元測試（合成映像，不需原版素材）通過。
- ps1-tests `cdrom/*`：tests.zip 只有互動程式、沒有期望輸出，未列入。

## 7. 唯讀階層檔案工具

狀態：CONFORMED，限本節唯讀階層工具。此切片只擴充原有ISO9660工具，不改CD控制器的暫存器、FIFO、時序或ReadN／ReadS。原有根檔名讀取保持，增加以斜線或反斜線分隔的相對路徑，以及ReadFileLimit(name string, maxBytes uint32)。ReadFile仍忽略目錄項的版本尾碼並採不分大小寫比對，找不到回io.EOF；路徑空段、點段及非目錄中間項拒絕。

證據沿既有本片SYSTEM.CNF根讀取與相鄰素材tools/extract_assets.py的實際ISO目錄抽取；後者已核對大小端extent／size、記錄長度、2048邊界、目錄旗標及原檔SHA。只讀本片MODE2/2352的2048資料視圖，並非通用ISO所有擴充。資料原始位址基準是bin的LBA及扇區24位移，不與CPU虛擬位址混用。

每個目錄上限16MiB，最多16個路徑元件；extent不能超出映像，目錄項不能跨2048邊界，大小端欄位必須相同，拒絕multi-extent。ReadFileLimit在配置輸出前檢查檔案長度，讀取結果獨立持有。最後項為目錄時仍沿原工具回其資料，避免擅改舊行為；只有中間項必須是目錄。工具使用獨立ReadAt，沒有模擬器週期或光碟機狀態效果。

驗證包含原根讀取、巢狀路徑與大小寫、缺檔／壞路徑、損壞extent／記錄及長度上限。真正月下來源另外核對ST/NO3/F_NO3.BIN與BIN/F_GAME.BIN的SHA；缺原片則skip，不以合成映像冒充原檔。合成小映像僅驗工具邊界。此切片不重開原CD硬體時序或音訊驗收。

§7結果：巢狀／大小寫／反斜線路徑、長度上限、缺檔與四種損壞記錄通過；原片BOOT、F_NO3與F_GAME完整SHA正確，區域與開機檔名保持。這是外部唯讀工具，不重開原CD硬體、原生M4或音訊驗收。實際收據由相鄰素材neighbor-modes-v1/production-v1索引。
