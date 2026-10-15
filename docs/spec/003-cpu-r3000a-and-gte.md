# 003 CPU（R3000A）與 GTE

狀態：`DRAFT`

## 1. 範圍

- R3000A：MIPS I 整數指令、load delay slot、branch delay slot、COP0 例外與中斷、
  指令快取的可觀察行為（隔離快取寫入被 BIOS 用來清快取）。
- GTE（COP2）：暫存器、命令、旗標暫存器、UNR 除法表。

不在此規格：記憶體映射與匯流排時序（另立 `internal/bus/` 規格）。

## 2. 驗收

判準是**全部通過**。CPU 的錯不會報錯，只會在幾百萬條指令之後畫錯一個像素。

| 語料 | 授權 | 狀態 |
|---|---|---|
| `JaCzekanski/ps1-tests` | MIT | 待核實其中哪些項目只依賴 CPU／GTE，哪些需要完整機器 |
| 其他 CPU／GTE 單步語料 | 未知 | 待搜尋；找不到就以 Beetle PSX 的逐指令執行結果當交叉 oracle |

語料用 `tools/` 下的腳本抓到 `testdata/`，不進版控。缺檔就 skip。

## 3. 參照

主要參照 `beetle-psx-libretro` 的 `mednafen/psx/cpu.c` 與 `gte.c`（GPL-2.0-or-later）。
引用時依 `002-references-and-licensing` §4 記 commit。

## 4. 進入 READY 前要補

- 語料的實際內容、格式、筆數。
- delay slot 與例外交錯的邊界案例清單。
- 時序：本專案的決定性以「指令數與匯流排週期」為時間基準，週期模型的精度要定下來。
