# 010 手把與記憶卡（SIO0）

狀態：`READY`（手把 `CONFORMED`；記憶卡待實作）

## 1. 為什麼

《月下》在 PlayStation 標誌之後初始化手把驅動（TTY `PS-X Control PAD Driver Ver 3.0`），
送出位元組後輪詢 `JOY_STAT` bit 1（接收 FIFO 有資料）。006 的 SIO 空殼永遠不給資料，
程式就停在 BIOS 的手把常式（2026-10-15 反組譯 `0x800045C4` 的等待迴圈確認）。

## 2. SIO0（`internal/sio`）

移植 `beetle-psx-libretro` 的 `mednafen/psx/frontio.c`（commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，
GPL-2.0-or-later）的 FrontIO：

- 暫存器（psx-spx `serialinterfacessio.md`）：DR、SR、MR、CR、BR。
- 位元時脈 = max(0x20, (BR << {0,0,4,6}[MR&3]) &^ 1) CPU 週期；收發同時進行，8 位元一組。
- /CS：CR bit 1 且依 bit 13 選 port 1 或 port 2。
- /ACK：裝置在位元組結束後要求延遲 N 週期拉 DSR，持續 32 週期；CR bit 12 開啟時觸發中斷（I_STAT bit 7）。
- 不實作 SIO1、multitap。

## 3. 數位手把

Beetle `InputDevice_Gamepad`：收到 `01h` 回 `41h`；接著收到 `42h` 回 `5Ah` 與兩個按鍵位元組（按下為 0）；
有資料要送時，每個位元組後 64 週期拉 /ACK。按鍵位元依 psx-spx 標準手把表。

`machine.Pad1` 是插在 1P 的手把，`Buttons` 即目前按鍵。

## 4. 記憶卡（待實作）

讀、寫、ID 命令（`52h`、`57h`、`53h`），128 KB 映像存在玩家指定的檔案；
參照 Beetle `InputDevice_Memcard`。完成後本節改寫成正式規格。

## 5. 驗收

| 項目 | 結果（2026-10-15） |
|---|---|
| 《月下》開機到標題畫面「PRESS START BUTTON」 | 達成（`psexe -cue`，2,500,000,000 週期） |
| `cpu/io-access-bitwidth` 的 JOY 暫存器 | 11 格變成與實機相符，已從台帳移除 |
| 記憶卡存檔 | 未做 |
