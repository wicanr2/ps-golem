# 012 Ebiten 視窗前端

狀態：`READY`

## 1. 為什麼

M4 的驗收是玩家走到第一個存檔點存檔（`001` §4）。序章要打德古拉，之後操作阿魯卡德走到存檔室，
用按鍵腳本盲打不實際，需要人能即時看畫面、操作。前端只消費核心（`001` §2）：
讀畫面、寫手把按鍵、存記憶卡，不改機器狀態的推進方式。

## 2. 執行檔

`cmd/psgolem`：

```
psgolem [-bios <BIOS>] [-cue <cue>] [-card <記憶卡檔>] [-scale N]
```

`-bios`、`-cue` 可以省略，改用設定檔或拖放（§9）。

- `-card` 省略時用 `<使用者設定目錄>/ps-golem/<光碟開機檔名>.mcd`
  （Go `os.UserConfigDir`；本案例檔名 `SLPM_860.23.mcd`）。存檔屬玩家資料，不寫進素材目錄或 repo。
- `-scale` 是起始視窗倍率，預設 3。

## 3. 時間

- Ebiten 的 TPS 設為 60。每次 `Update` 讓機器跑到下一次 VBlank（`RunUntilField(field+1)`，
  上限 2 個 field 的週期數，防止顯示關閉時卡住）。
- 機器的時間只由 CPU 週期決定（`001` §3.1）；前端不跳 field、不補 field。
  模擬比實機慢時，遊戲整體變慢；不改變任何決定性狀態。
- NTSC 實機 field 率約 59.94，與 60 的差在前端可忽略。

## 4. 畫面

- 每次 `Draw` 取 `GPU.Screen()`（`007` §8）寫進 Ebiten 影像，依 4:3 顯示比例縮放到視窗中央，
  多出的邊填黑。縮放用最近鄰；HD 濾鏡另由 HD 規格處理。
- 畫面尺寸改變（遊戲切換解析度、24 位元動畫）時重建影像。

## 5. 輸入

每次 `Update` 先讀輸入，再跑機器，按鍵寫進 `machine.Pad1.Buttons`。

| 手把 | 鍵盤 | Ebiten 標準手把 |
|---|---|---|
| 十字鍵 | 方向鍵 | 左側十字鍵 |
| ✕ | `Z` | RightBottom |
| ○ | `X` | RightRight |
| □ | `A` | RightLeft |
| △ | `S` | RightTop |
| L1／R1 | `Q`／`W` | FrontTopLeft／FrontTopRight |
| L2／R2 | `1`／`3` | FrontBottomLeft／FrontBottomRight |
| START | `Enter` | CenterRight |
| SELECT | `Backspace` | CenterLeft |

鍵盤與所有已連接的標準手把取聯集。

## 6. 記憶卡

- 啟動時檔案存在就載入（`010` §4），否則用空白卡。
- 卡片 `Dirty` 且距最後一次寫入（frame 寫入完成）超過 60 個 field，寫回檔案並清掉 `Dirty`；
  關閉視窗時再檢查一次。寫檔先寫暫存檔再改名，避免寫到一半中斷。

## 7. 驗收

| 項目 | 判準 |
|---|---|
| 按鍵對應、顯示比例 | 單元測試（純函式，不開視窗） |
| 視窗啟動 | 容器內 `xvfb-run` 跑 `psgolem -frames N -screenshot <png>`：跑 N 次 `Update` 後存下 Ebiten 實際畫出的畫面並結束；與 `psexe -screen` 在同一 field 的畫面一致 |
| 實際遊玩 | 玩家在主機上執行，從開機玩到存檔點存檔（M4 驗收，由玩家確認） |

`-frames`、`-screenshot` 只給驗收與除錯用。`-mute` 不開音訊裝置（容器內沒有音效卡時用）。
`-press <腳本>` 以按鍵腳本（`010`，與 `psexe -press` 同格式，解析在 `internal/padscript`）取代鍵盤與手把，
`-fields N` 跑到第 N 次 VBlank 就存記憶卡、截圖並結束；兩者用來讓發行包的視窗執行檔重播整段遊玩。

## 8. 聲音

`013` §5：每次 `Update` 跑完一個 field 後取走 SPU 的取樣，送進 44.1 kHz 播放器（Ebiten `audio`，緩衝 60 毫秒）。
排隊超過約 0.25 秒就丟掉最舊的；不夠時補靜音。

## 9. 啟動與設定檔

Windows、macOS 的玩家通常用滑鼠雙擊開啟，沒有命令列參數，所以 BIOS 與光碟的位置要能記住。

- 設定檔：`<使用者設定目錄>/ps-golem/settings.json`，內容 `{"bios": "<路徑>", "cue": "<路徑>"}`。
- 決定路徑的順序：命令列參數優先，沒給的項目取設定檔。
- 音訊裝置開不起來時印出警告、改為靜音照常執行（雙擊開啟看不到錯誤訊息）。
- 兩個路徑都有，且 BIOS 與光碟都開得起來，就開機；成功開機後把這組路徑寫回設定檔。
- 缺任何一項，或開啟失敗，視窗顯示設定畫面：列出目前的 BIOS、cue 與錯誤訊息，
  提示把檔案拖進視窗。畫面文字只用 ASCII（Ebiten 內建的除錯字型沒有中文字）。
- 拖進的檔案依序判斷：副檔名 `.cue`（不分大小寫）當光碟；大小正好 512 KiB 當 BIOS；其他忽略。
  兩項齊了就照上一條開機。
- 拖放的實際路徑取自 Ebiten `DroppedFiles` 開出的 `*os.File` 的 `Name()`（Ebiten v2.9.9
  `internal/file/file_notjs.go` 以 `os.Open` 開實際路徑）；拿不到實際路徑的檔案忽略。

## 10. 驗收結果（2026-10-15）

- 按鍵對應、顯示比例、記憶卡等待安靜後存檔：`frontend/ebiten` 單元測試通過（`xvfb-run`）。
- 視窗啟動：容器內 `xvfb-run psgolem -frames 2000`（960×720 視窗）與 `psexe -fields 2000 -screen`
  比對，依 4:3 置中規則對回 320×240 後 0 像素不同（KONAMI 標誌畫面）。
- 實際遊玩：待玩家確認。
- 設定畫面（2026-10-16）：`dropKind`、`asciiPath` 單元測試通過；容器內不帶參數開機顯示設定畫面，
  路徑錯誤時列出錯誤；以參數開機後寫入 `settings.json`，再不帶參數開機，第 2000 次 `Update` 與 `psexe` 0 像素不同。
  拖放在 xvfb 下無法模擬，由玩家在主機上確認。
