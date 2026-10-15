# 011 MDEC（動畫解碼器）

狀態：`READY`

## 1. 為什麼

《月下》開新遊戲後把顯示切到 24 位元模式，畫面全黑（`010` §6）。PS1 的全動態影像由 MDEC 解碼：
程式把壓縮的巨集區塊經 DMA channel 0 送進 MDEC，再經 channel 1 取回像素，搬到 VRAM 顯示。
006 的 MDEC 只是讀回固定狀態值的空殼。

## 2. 解碼器（`internal/mdec`）

移植 `beetle-psx-libretro` 的 `mednafen/psx/mdec.c`（commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，
GPL-2.0-or-later），只取純 C 路徑（不移植 SSE2／NEON 版，結果相同）：

- 暫存器：`1F801820` 寫入命令／參數、讀取輸出資料；`1F801824` 寫入控制（bit 31 重置、bit 30 允許 DMA 輸入、
  bit 29 允許 DMA 輸出），讀取狀態（bit 31 輸出 FIFO 空、30 輸入 FIFO 滿、29 命令執行中、28 可 DMA 輸入、
  27 可 DMA 輸出、26–23 命令的輸出格式位元、15–0 剩餘參數字數）。
- 輸入、輸出 FIFO 各 32 個 32 位元字。
- 命令（bit 31–29）：1 解碼巨集區塊（bit 28–27 輸出格式 4／8／24／15 位元、bit 26 有號、bit 25 設 bit 15、
  bit 15–0 參數字數）、2 載入量化表（bit 0 決定 1 或 2 張表）、3 載入 IDCT 表；其他命令只吃參數字數。
- 解碼：RLE 係數、量化、zig-zag、兩段 1D IDCT、YCbCr→RGB 的數值完全照 Beetle。
- DMA 輸入條件：輸入 FIFO 空、控制 bit 30、命令執行中且還有參數要收。
  DMA 輸出條件：輸出 FIFO 至少 32 字、控制 bit 29。輸出時依格式重排寫入位址（Beetle `MDEC_DMARead` 的 offset）。

## 3. DMA channel 0、1

- 同步模式 1（區塊）：每個區塊開始前檢查 §2 的條件，成立才搬一整個區塊；不成立就讓通道保持啟動，
  等 MDEC 狀態改變（CPU 寫 MDEC 暫存器、讀輸出資料、另一個通道搬完一個區塊）時再試。
  全部區塊搬完才清 CHCR bit 24 並設 DICR 旗標，與其他通道相同。
- 同步模式 0：整段視為一個區塊，需 CHCR bit 28 觸發。
- 每次搬完區塊後更新 MADR、BCR 的區塊數（同 Beetle）。

## 4. 時間

照 Beetle 的近似：每完成一個 8×8 區塊花 512 週期，讀進命令字花 1 週期；解碼器最多預存 128 週期
（Beetle `EventCycles` 預設值，`libretro.c`）。機器每執行一條指令把週期交給 MDEC，
channel 0、1 啟動中時再推進 DMA。DMA 搬一個區塊本身不花時間。

解碼時間會影響程式的中斷時序：DMA channel 1 完成的時間點不同，與 CD-ROM 中斷的先後也不同。

## 5. 驗收

ps1-tests（tests.zip，`build-158`）的 `mdec/*`，與 `007` 相同的台帳做法（`machine/mdec_gaps.txt`，不符數變好或變差都算失敗）：

| 測試 | 判準 |
|---|---|
| `mdec/4bit`、`mdec/8bit` | VRAM 與 `vram.png` 逐像素比對；TTY 與 `psx.log` 比對 |
| `mdec/frame` 的 15／24 位元，CPU 與 DMA 各一 | VRAM 與 `vram-15bit.png`／`vram-24bit.png` 比對 |
| `mdec/movie` 的 15／24 位元 | VRAM 與對應 png 比對（播放到固定週期數，結果依賴時間時記入台帳並寫明原因） |
| `mdec/step-by-step-log` | TTY 與 `psx.log` 比對 |
| 《月下》開新遊戲 | 24 位元顯示期間 VRAM 出現解碼後的影像（目視） |

## 6. 介面

```go
package mdec
func New() *MDEC
func (d *MDEC) Read(addr uint32) uint32      // addr 的 bit 2 選暫存器
func (d *MDEC) Write(addr uint32, v uint32)
func (d *MDEC) DMACanWrite() bool
func (d *MDEC) DMACanRead() bool
func (d *MDEC) DMAWrite(v uint32)
func (d *MDEC) DMARead() (v uint32, offs uint32)
```

`MDEC` 不含指標，`Machine.Clone` 直接複製值。

## 7. 驗收結果（2026-10-15，SCPH5500）

台帳 `machine/mdec_gaps.txt`，`machine/mdec_test.go` 每次比對。

| 測試 | 不符 | 原因 |
|---|---|---|
| `mdec/frame-15bit-dma`、`frame-24bit-dma` | 15133、37157 像素 | 捨入差。15 位元版全部是各色 ±1 LSB；24 位元版逐位元組比對，345600 個位元組中 ±1 有 47688 個、±2 到 ±4 有 7710 個，另有 114 個差約 127（推測為飽和邊界，假說）。數值照 Beetle；psx-spx 也寫明實機 IDCT 與 YCbCr→RGB 的確切捨入未知 |
| `mdec/frame-15bit` | 18362 像素 | 上列捨入差，加上最右一欄 16 像素全黑：程式讀完最後一個字後再等「輸出非空」而停住，Beetle 相同。參考圖與 DMA 版共用一張，實機在此是否停住未知 |
| `mdec/frame-24bit` | 102051 像素 | 測試程式 CPU 讀取路徑不重排 24 位元區塊（程式自己印出 not implemented），結果本來就不會等於參考圖 |
| `mdec/movie-15bit`、`movie-24bit` | 20732、34735 像素 | 動畫播放到固定週期數時停下，畫面停在哪一格依賴時間 |
| `mdec/4bit`、`8bit` | TTY 19 行、VRAM 50／54 像素 | 測試漂移：`psx.log` 在 ps1-tests commit `29eca5d` 錄下（區塊大小可變）；tests.zip 的 `mdec.cpp` 固定區塊大小 0x20，8 個字的輸出變成 0 個區塊的 DMA，程式讀不到自己的輸出 |
| `mdec/step-by-step-log` | TTY 1644 行 | 每寫一個參數字就印狀態。實機在餵入手工資料時「目前區塊」長時間停在 4、輸出 FIFO 保持空；Beetle 與本實作在第 64 個係數就結束區塊。原因未知 |

《月下》：KONAMI 標誌動畫（24 位元）以 `psexe -shots` 截圖目視正確；開場動畫連續 143 格每格 20 個切片完整讀出，
沒有 `MDEC_in_sync timeout`。
