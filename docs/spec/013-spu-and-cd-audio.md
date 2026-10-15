# 013 SPU 與 CD 音訊

狀態：`READY`

## 1. 為什麼

M4 包含聲音（`001` §4）。目前 SPU 只有讀回寫入值的暫存器，遊戲沒有聲音；
《月下》的背景音樂以 CD-XA 串流播放，效果音由 SPU 的 ADPCM voice 播放。

## 2. 來源

移植 `beetle-psx-libretro`（commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，GPL-2.0-or-later）：

- `mednafen/psx/spu.c`：24 個 voice、ADPCM 解碼、ADSR、音量掃描、雜訊、FM、混響、SPU RAM、IRQ、資料傳輸。
- `mednafen/psx/cdc.c` 的 CD 音訊部分：XA 篩選與 ADPCM 解碼（`XA_Test`、`XA_ProcessSector`）、
  CD-DA 放進音訊緩衝（`EnbufferizeCDDASector`）、重取樣到 44.1 kHz 與 CD 音量矩陣（`GetCDAudioSample`、`ApplyVolume`）。

只取純 C 路徑（不移植 SSE2／NEON 版）。Beetle 的設定項取預設值：`spu_samples` = 1。

## 3. 時間

- SPU 每 768 個 CPU 週期產生一組立體聲取樣（33868800 ÷ 44100）。
- 照 Beetle 的結構：CD-ROM 推進時把時間切到 SPU 的下一個取樣點，先推進 SPU，再處理 CD 事件
  （Beetle `PS_CDC_Update` 的 `SPUCounter`）。SPU 每個取樣向 CD-ROM 取一筆 CD 音訊。
  取樣點只由週期數決定，與機器怎麼批次推進無關。
- SPU 中斷開啟（SPUCNT bit 6）時，機器的同步期限不晚於下一個取樣點（`008` §2.5），中斷時間與逐條同步相同。

## 4. 介面

```go
package spu
func New() *SPU
func (s *SPU) Read(addr uint32) uint16           // 1F801C00–1F801FFF
func (s *SPU) Write(addr uint32, v uint16)
func (s *SPU) DMAWrite(v uint32)
func (s *SPU) DMARead() uint32
func (s *SPU) Advance(clocks int64, cd CDAudio) int64 // 回傳到下一個取樣點的週期數
func (s *SPU) IRQ() bool
func (s *SPU) TakeSamples(dst []int16) []int16     // 取走累積的立體聲取樣（L、R 交錯）

type CDAudio interface{ CDAudioSample() (l, r int32) } // Beetle CDC_GetCDAudioSample
```

`SPU` 不含指標與函式值，`Machine.Clone` 直接複製；CD 音訊來源在呼叫時傳入。
累積的取樣緩衝最多 4096 組（Beetle `IntermediateBuffer`），滿了丟棄新的。

匯流排：SPU 暫存器 16 位元；8 位元寫入照 psx-spx 表標為 `(i16)`。DMA channel 4 依 SPUCNT 的傳輸模式讀寫 SPU RAM。

## 5. 前端輸出

Ebiten 的 `audio` 以 44100 Hz、16 位元立體聲播放。每次 `Update` 跑完一個 field 後取走取樣寫入播放器。
模擬比實機慢時播放器會斷音；不為此改變機器時間。

## 6. 驗收

| 項目 | 判準 |
|---|---|
| ps1-tests `spu/*` | 有 `psx.log` 的與之比對；台帳記錄不符與原因 |
| 延後同步等價 | `machine/sync_test.go` 加入 `spu/*`；《月下》開機到序章的摘要與逐條同步相同 |
| 《月下》 | 標題畫面、序章有音樂與效果音：把取樣存成 WAV，人工聆聽確認 |

## 7. 驗收結果（2026-10-15，SCPH5500）

- `internal/spu` 單元測試（取樣數、緩衝上限、DMA 往返、voice 發聲與 key off、IRQ、值複製）與
  `internal/cdrom` 的 XA 解碼、CD-DA 音量／靜音測試通過，不需要原版素材。
- ps1-tests：`spu/memory-transfer` 與 `psx.log` 差 8 行，全是 DMA 時序（`007` §6）；`dma/dpcr` 由 17 行變 0；
  `io-access-bitwidth` 的 SPUCNT 8 位元寫入 2 格變成相符。
- 延後同步等價：`spu/test`、`spu/memory-transfer`、`spu/stereo` 加入 `machine/sync_test.go` 後兩種方式相同；
  《月下》開機、動畫、按鍵進入序章 7B 週期，兩種方式摘要與記憶卡相同。
- 《月下》：`psexe -wav` 錄下開機到序章 206 秒。開機音效、KONAMI 標誌、序章動畫（XA）、序章遊戲中都有聲音，
  標題畫面一段靜音；沒有削峰。是否與實機聽起來一致待人工聆聽。
