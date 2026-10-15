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

狀態定義：

- `DRAFT`：還有會改變行為的未知，只允許研究與可丟棄的 probe。
- `READY`：輸入、狀態轉移、邊界、輸出與驗收測試都有證據支持。
- `CONFORMED`：實作通過內部測試與所有適用的 oracle。
- `SUPERSEDED`：被新證據推翻，保留舊證據與訂正理由。
