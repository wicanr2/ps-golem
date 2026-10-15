# 010 手把與記憶卡（SIO0）

狀態：`READY`（手把 `CONFORMED`；記憶卡實作完成，待遊戲存檔驗收）

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

## 4. 記憶卡

移植 Beetle `InputDevice_Memcard`（同一 commit）：

- 128 KB，1024 個 frame × 128 位元組。新卡依 Beetle `Format` 寫入 header frame（`MC`）、15 個空目錄 frame 與壞區表。
- 命令 `81h` 之後：`52h`（讀）回 `5Ah 5Dh`、位址、`\ ]`、位址回送、128 位元組、XOR、`G`；
  `57h`（寫）收位址、128 位元組、XOR，回 `\ ]` 與結果（`G` 成功、`N` 校驗錯、`FFh` 位址越界）。
  其他命令（含 `53h`）不回應，與 Beetle 相同。
- 旗標位元組：新插入的卡回 `08h`，第一次寫入成功後回 `00h`。
- 每個要回應的位元組後 0x100 週期拉 /ACK。
- `machine.Card1` 是插在 1P 的記憶卡；`Data` 為卡片內容，`Dirty` 表示寫入過。
- `psexe -card <path>`：檔案存在就載入（長度須為 128 KB），結束時 `Dirty` 才寫回。存檔屬玩家資料，不進版控。

## 5. 按鍵腳本

`psexe -press "FIELD:BTN[+BTN][:DUR],..."`：從第 FIELD 次 VBlank 起按住按鍵 DUR 個 field（預設 5）。
按鍵名稱：`SELECT START UP RIGHT DOWN LEFT L1 R1 L2 R2 TRIANGLE CIRCLE CROSS SQUARE`。
機器每個 field 開始時依腳本設定 `Pad1.Buttons`（`machine.RunUntilField`），結果可重現。

## 6. 驗收

| 項目 | 結果（2026-10-15） |
|---|---|
| 《月下》開機到標題畫面「PRESS START BUTTON」 | 達成（`psexe -cue`，2,500,000,000 週期） |
| `cpu/io-access-bitwidth` 的 JOY 暫存器 | 11 格變成與實機相符，已從台帳移除 |
| 按鍵腳本進選單 | 達成：`-press "4500:START,4700:START,4900:START"` 到檔案選單「SELECT YOUR DESTINY」，中文說明正常 |
| 記憶卡寫入 | 達成：插空白卡進選單後，frame 63（寫入測試 frame）被寫成 header 的複本，與實機格式相符 |
| 遊戲內存檔 | 未達成：選檔、開新遊戲後進入 24 位元顯示且畫面全黑（推測為動畫，需 MDEC，假說），尚未走到存檔點 |
