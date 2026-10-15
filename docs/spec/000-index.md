# 規格索引

SDD：量測／反組譯 → 規格 → 才寫程式。只有標 `READY` 的規格可以動手。
引用規格連號碼帶檔名，例如 `002-references-and-licensing`。

| 規格 | 狀態 | 內容 |
|---|---|---|
| `001-scope-and-mvp.md` | DRAFT | 目標、分層、里程碑、已核實的 BIOS 與光碟事實 |
| `002-references-and-licensing.md` | READY | GPL-3.0-or-later、可參照與只能讀的來源 |
| `003-gte.md` | CONFORMED | GTE 暫存器與命令，1150 筆實機紀錄 |
| `004-cpu-r3000a.md` | CONFORMED | CPU；與參照實作的差異依實機紀錄修正 |
| `005-cross-platform-delivery.md` | DRAFT | 三平台在 Linux docker 內建置 |
| `006-minimal-machine.md` | CONFORMED | 匯流排、BIOS 開機、PS-EXE 側載、TTY；週邊台帳 |
| `007-gpu-and-dma.md` | READY（實作中） | GPU 原生光柵化、DMA2；M3 台帳 |
| `008-cycle-model.md` | CONFORMED | 取指令、讀取、乘除法、GTE 的週期成本 |
| `009-cdrom.md` | CONFORMED | cue／bin、CD-ROM 控制器、DMA3 |
| `010-pad-and-memcard.md` | READY（手把完成） | SIO0、數位手把、記憶卡、按鍵腳本 |
| `011-mdec.md` | READY（實作完成，台帳見 §7） | MDEC 動畫解碼、DMA0／1 |
| `012-frontend-ebiten.md` | READY（實作完成，待玩家遊玩驗收） | 視窗、鍵盤與手把、記憶卡自動存檔 |

狀態定義：

- `DRAFT`：還有會改變行為的未知，只允許研究與可丟棄的 probe。
- `READY`：輸入、狀態轉移、邊界、輸出與驗收測試都有證據支持。
- `CONFORMED`：實作通過內部測試與所有適用的 oracle。
- `SUPERSEDED`：被新證據推翻，保留舊證據與訂正理由。
