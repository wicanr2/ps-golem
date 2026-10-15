# 001 範圍與 MVP

狀態：`DRAFT`

## 1. 要解決的問題

《惡魔城X：月下的夜想曲》中文版只有 PlayStation 1 光碟映像。目標是讓它在 Linux、Windows、
macOS 上執行，並提供 HD 輸出，同時讓 agent 能用程式查詢執行中的狀態。

現成模擬器是為「人坐在前面玩」設計的，觀測只能靠截圖與外部除錯器。ps-golem 沿用 dosgolem
的定位：執行器本身就是 Go 套件，`go test` 裡可以停在指定條件、讀記憶體、重播輸入。

## 2. 兩種使用方式

| 使用方式 | 需要什麼 |
|---|---|
| 玩家 | 視窗、鍵盤與手把、聲音、記憶卡存檔、HD 畫面 |
| agent 觀測 | 無頭、決定性、RunUntil、讀寫 RAM／VRAM、輸入重播、即時存檔 |

兩者共用同一個核心。前端只是核心的消費者。

## 3. 分層

```
internal/cpu/     R3000A（MIPS I）、COP0、例外。不認識週邊
internal/gte/     COP2 幾何轉換引擎
internal/bus/     記憶體映射、DMA、中斷控制器、計時器
internal/gpu/     GPU 命令、VRAM、軟體光柵化（原生解析度，決定性的基準）
internal/spu/     SPU、ADPCM、混音
internal/cdrom/   CD-ROM 控制器、cue/bin 讀取、XA 音訊
internal/mdec/    MDEC（FMV 解碼）
internal/sio/     手把、記憶卡
machine/          把上面組成一台機器，處理排程與時序
oracle/           對外 Go API：Load／RunUntil／ReadRAM／Input／Save／Restore
render/hd/        HD 後端（Codex 負責），讀 GPU 命令流，不回寫機器狀態
frontend/ebiten/  視窗、輸入、聲音
apps/sotn/        《月下》專屬：位址、狀態、寬螢幕攔截點
cmd/              命令列工具
docs/spec/        規格
tools/            docker 包裝、語料抓取
```

判準沿用 dosgolem `docs/spec/006-layering.md`：換一片光碟之後還成立的程式碼往下放，
不成立的留在 `apps/`。

### 3.1 時間基準

唯一的主時鐘是 CPU 週期（33.8688 MHz）。GPU dot clock、HBlank、VBlank、SPU 取樣、
CD-ROM 扇區時序都用分數累加器從 CPU 週期推出，不另設獨立時鐘。
理由：dosgolem 有過兩個時間基準並存、拿錯一個整條時間軸被縮放卻沒有任何環節報錯的教訓
（dosgolem `CLAUDE.md`、`docs/spec/190-opl-vgm-dump` §3）。單一時鐘也是
「開關 HD 前後 RAM 與 CPU 狀態相同」能成立的前提。

`render/hd/` 與 `internal/gpu/` 分開的理由：軟體光柵化是決定性的基準，HD 只影響呈現。
開關 HD 前後，同一份輸入重播出來的 RAM 與 CPU 狀態必須相同。

## 4. 里程碑

每一項都要先有 `READY` 規格才實作。順序以「能玩」優先，觀測 API 排在可玩之後。

| # | 里程碑 | 驗收 |
|---|---|---|
| M1 ✅ | GTE | `003-gte` 的 1150 筆實機紀錄全部相符 |
| M2 ✅ | 最小機器：CPU、匯流排、BIOS 載入、PS-EXE 側載、TTY 攔截 | ps1-tests `cpu/*` 的 TTY 與 `psx.log` 相同（`004-cpu-r3000a`） |
| M3 | BIOS 開機到 SCE 開場畫面，GPU 2D | ps1-tests `gpu/*` 的 VRAM 與附帶的 `vram.png` 相同；開機畫面與參考模擬器比對 |
| M4 | 從 cue/bin 開機到《月下》可玩：DMA、CD-ROM、SPU、XA、CDDA、手把、記憶卡 | 正常玩家路徑走到第一個存檔點並存檔成功 |
| M5 | 跨平台交付 | `005-cross-platform-delivery` |
| M6 | HD 後端（Codex） | HD 規格（待寫） |
| M7 | oracle API：RunUntil、RAM 讀寫、輸入重播、即時存檔 | 附 `go test` 範例 |

M3 之後的關鍵路徑是 匯流排＋DMA＋GPU 2D → CD-ROM → SPU／XA。《月下》以 2D 為主，
大量從光碟串流，MDEC 與 GTE 3D 的重要性排在後面。

ps1-tests 附的 `vram.png` 是否為實機擷取，README 沒有寫明（未知）。採用為 oracle 前先查各測試的原始碼或提交紀錄。

## 5. 不做

- 不追求其他 PS1 遊戲的相容性。接第二片時再擴充。
- 不做網路連線、作弊碼、錄影上傳。

## 6. 已核實的輸入

2026-10-15 在 docker 內讀檔核實（confirmed）。

| 輸入 | 事實 | 依據 |
|---|---|---|
| `SCPH-100.BIN` | System ROM Version 4.3 03/11/00 J | 檔內 offset `0x7FF32` 字串 |
| `SCPH5500.BIN` | System ROM Version 3.0 09/09/96 J | 檔內 offset `0x7FF32` 字串 |
| 光碟開機檔 | `BOOT = cdrom:SLPM_860.23;1`，`TCB = 4`、`EVENT = 16`、`STACK = 801fff00` | 光碟內 `SYSTEM.CNF` |
| 光碟軌道 | Track 1 `MODE2/2352` 資料軌；Track 2 `AUDIO`，INDEX 00 `51:30:08`、INDEX 01 `51:32:08` | `.cue` |
| cue 的檔名 | Big5 編碼，副檔名寫成大寫 `.BIN`；壓縮檔內實際檔名是 UTF-8、小寫 `.bin` | `.cue` 與 zip 目錄 |

對規格的影響：

- 序號 `SLPM-86023` 是日版，BIOS 兩份都是日規，地區一致。
- cue 讀取器要容忍檔名編碼與大小寫不符：cue 內的檔名找不到時，退回同目錄唯一的 `.bin`，
  並在 log 註明。
- 需要 CDDA 音軌播放。

## 7. 未知

- 中文化改了哪些檔案（與日版 SLPM-86023 原盤比對，需要原盤的檔案雜湊表）。
- 開機畫面的交叉 oracle：先查 PCSX-Redux 命令列能否無 GUI 執行、載入 BIOS、讀 VRAM，查了再定。
