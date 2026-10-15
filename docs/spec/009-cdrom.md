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
