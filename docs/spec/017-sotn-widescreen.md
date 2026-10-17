# 017 《月下》寬螢幕 16:9

狀態：`READY`（實作驗收未通過，見 §7）

## 1. 為什麼要改遊戲

`016` §1 要求 HD 不改變決定性狀態。《月下》遊玩畫面寬 256，背景圖塊每列只送 17 格（鏡頭左緣起），
畫面外的背景遊戲不送給 GPU，影子 GPU 再怎麼放大也畫不出來。第三方 PC 重編譯版的 16:9 同樣改了遊戲的繪圖邏輯，
部分背景只做了 4:3 的範圍，部分全螢幕效果仍在原框內
（Generation Amiga 2026-07-11〈Castlevania: Symphony of the Night PS1 recomp adds native 16:9 widescreen〉）。

實體（敵人、物件、特效）不用改：遊戲在鏡頭左 64 到右 320 像素的範圍生成實體（`sotn-decomp` `src/st/create_entity.h`），
並把畫面外的部分照常送給 GPU 由繪圖區裁掉。城之入口 field 34400–34700 每 field 約 42 個圖元部分超出繪圖區，
最遠左超 232、右超 101 像素（§6 probe）。

## 2. 與決定性的關係

採遊戲修改模式（2026-10-17 使用者決定，`AGENTS.md` 已列為例外）：

- `psgolem -widescreen`（設定檔 `widescreen` 欄位）開啟時套用 §3 的修改；預設關閉。
- 開啟期間不保證與原版重播一致：不產生 `psexe` 摘要、不用於 M4 這類驗收、不用於觀測 API。記憶卡照常讀寫。
- 關閉時，M4 重播摘要 `6c35c2701228722f` 與記憶卡不變，不碰 RAM、不掛任何陷阱。

## 3. 遊戲修改（`apps/sotn`）

### 3.1 目標函式

`RenderTilemap`（名稱取自 `sotn-decomp` `src/dra/4CE2C.c`），本光碟位於 DRA 常駐區 `0x800ECC10`–`0x800ED52B`（2332 位元組），
SHA-256 `40dfd40f6a9922dd5ed1b69f1fd32ada3f7a8139b793984f49d373d3f31b797c`。
序章（field 12000）與城之入口（field 35000）的 RAM 傾印相同。對應依據（confirmed）：

- 讀 `g_cameraOffsetX`（`0x8006C39C`）、`g_GpuUsage`（`0x8009792C`）、`g_Tilemap`（`0x80073084`）、`g_BgLayers`（`0x800730D8`），
  位址與 decomp 美版符號相同（`015` §4 已證實資料區與美版一致）。
- 前景、背景兩層各有 `slti …,17`（`N_HORIZ_TILES`）、`slti …,16`（`N_VERTI_TILES`）、`sltiu …,640`（`MAXSPRT16`）、`slti …,24`（繪圖模式迴圈）。

### 3.2 改動

左右各多畫 3 格（48 像素）：起點往左 3 格、每列 17 格改 23 格、繪製起點 x 減 48。

| 位址 | 原字 | 新字 | 說明 |
|---|---|---|---|
| `0x800ECD64` | `00031C00` `sll v1,v1,16` | `2463FFD0` `addiu v1,v1,-48` | 前景 startTx = (scrollX − 48) >> 4 |
| `0x800ECD70` | `00037D03` `sra t7,v1,20` | `00037903` `sra t7,v1,4` | 同上 |
| `0x800ED010` | `29220011` `slti v0,t1,17` | `29220017` `slti v0,t1,23` | 前景每列格數 |
| `0x800ED194` | `00021400` `sll v0,v0,16` | `2442FFD0` `addiu v0,v0,-48` | 背景層 startTx |
| `0x800ED198` | `00027D03` `sra t7,v0,20` | `00027903` `sra t7,v0,4` | 同上 |
| `0x800ED428` | `29220011` `slti v0,t1,17` | `29220017` `slti v0,t1,23` | 背景層每列格數 |
| `0x800ECCF0` | `A6030000` `sh v1,0(s0)` | 陷阱（§3.3） | 存 r->x 時改存 `v1 − 48`；前景與背景共用 r->x |
| `0x800ECE80` | `01EC1021` `addu v0,t7,t4` | 陷阱（§3.3） | 前景每列圖塊表起點改用 `max(t7,0) + t4` |
| `0x800ED260` | `01EC1021` `addu v0,t7,t4` | 陷阱（§3.3） | 背景層每列圖塊表起點，同上 |

subX 由原 scrollX 的低 4 位取得（`0x800ECD60`、`0x800ED190`，在改動之前執行），減 48 不影響低 4 位。
scrollX 小於 48 時 startTx 為負。原程式每列只在列首算一次圖塊表指標（`base + (t7 + t4) × 2`），
之後只有通過 `tx < 0` 檢查的格子才把指標加 2；負的格子被跳過但指標不前進，整列會讀早 `−startTx` 格。
因此列首指標改用 `max(t7, 0)`，`tx < 0` 的格子照常跳過、不畫，與房間左緣外不畫一致。
背景層的捲動包覆模式（層旗標 `0x1000`）每格以 `tx & 15` 重算指標（`0x800ED284`），不受影響。

### 3.3 陷阱

三個陷阱位址前後沒有可用的空位，各改成一條 `BREAK` 指令，code 欄位各取專用值。CPU 執行 `BREAK` 時（冷路徑），
若有掛上的處理器且位址與 code 相符，就模擬替換後的動作、PC 前進一條，不進例外；否則照原本進 Bp 例外。

| 位址 | BREAK code | 模擬 |
|---|---|---|
| `0x800ECCF0` | `0x5753` | `sh` 寫入 `v1 − 48` 到 `s0` |
| `0x800ECE80` | `0x5754` | `v0 = max(int32(t7), 0) + t4`，32 位元相加 |
| `0x800ED260` | `0x5755` | 同上 |

三處都不在延遲槽，前一條指令都不是載入 `v0`。每 field 約 1 次加 2 × 16 列次。
不得在 `step`、`fastFetch`、`execute` 分派或 `Machine.Step` 加判斷。

### 3.4 套用與解除

- 每個 field 結束時檢查：§3.1 的 2332 位元組（把已套用的 9 個字當成原字計算）SHA-256 相符才套用；不符就不套用、已套用的不動。
  DRA 重新載入會蓋掉改動，下一次檢查重新套用。
- 關閉模式時把 9 個字寫回原字並卸下陷阱處理器。

### 3.5 已知限制

- 精靈緩衝上限 640（`MAXSPRT16`）：圖塊超過上限時遊戲會略過後面的圖塊。城之入口實測每 field 約 480。
- HUD、選單、全螢幕效果仍在中央 256 寬範圍。
- 房間左右邊界外沒有圖塊，延伸區顯示黑色。
- 只改 DRA 的通用圖塊繪製；關卡 overlay 自己畫的特殊背景不在範圍內。

## 4. 畫面端（影子 GPU）

原生 GPU 與 VRAM 不變。寬螢幕需要影子 GPU（`-hd` 1 以上；`-widescreen` 沒有 `-hd` 時自動用 N=1）。

| 項目 | 規定 |
|---|---|
| 側邊畫布 | 每個繪圖區（E3／E4 定義的矩形）左右各配一塊 M=48 原生像素寬、與繪圖區同高的畫布（N 倍），不在 VRAM 裡 |
| 光柵化 | 寬螢幕開啟時，影子以 `[x1 − M, x2 + M]` 作為水平裁切範圍；落在 `[x1, x2]` 的像素寫 VRAM，左右超出的寫進該繪圖區的側邊畫布。貼圖、CLUT 照常從影子 VRAM 讀 |
| 清除 | 繪圖區切換到另一塊（E3 起點改變）時，新繪圖區的側邊畫布清成黑色；GP0 02h 填色覆蓋整個繪圖區寬度時，同列的側邊畫布一起填 |
| 其他 | VRAM 複製、CPU↔VRAM 傳輸不碰側邊畫布 |
| 輸出 | 顯示起點與某個繪圖區的左上角相同時，輸出 `左側邊 + 顯示範圍 + 右側邊` 再裁成中央寬 342 原生像素（256 寬、像素比 1.25 時的 16:9）；其他情況（24 位元動畫、顯示範圍對不上）輸出原畫面左右補黑到同寬 |
| 前端 | 視窗比例 16:9；`-filter xbr` 照常疊加 |
| CLI | `psgolem -widescreen`、設定檔 `widescreen`；`psexe -widescreen` 只供截圖，開啟時不印摘要 |

## 5. 驗收

| 項目 | 判準 |
|---|---|
| 關閉時 | `TestM4FirstSave` 摘要 `6c35c2701228722f`；`016` 的轉送自我檢查與全部既有測試照常通過 |
| 簽章 | 單元測試（不用原版素材）：雜湊不符時不套用；套用後再解除，9 個字回到原字 |
| 陷阱 | 單元測試：手寫 PS-EXE 片段在指定位址執行 `BREAK`，處理器模擬 `sh` 並減 48，以及兩處列首指標計算；沒有處理器時進 Bp 例外，與現行行為相同 |
| 畫面 | 序章王座廳樓梯（約 field 15000）、城之入口（約 field 34500）各存一組 `-hd 2` 截圖：原畫面補黑版、寬螢幕版；目視背景圖塊在 x=0 與 x=256 接縫處連續、實體在延伸區正常出現 |
| 效能 | 本輪依使用者決定不量測，不判定原訂的 10% 降幅門檻（`012` §3 量測方法） |
| 交叉編譯 | `tools/build_dist.sh` 三平台通過 |

## 6. 證據

- probe 程式與輸出在 `workplace/perf/gp0probe/`、`workplace/ws/`（不入版控）：以 GPU trace 統計圖元相對繪圖區的水平範圍；
  `-patch` 以 RAM 寫入套用 §3.2 的 6 個字並以逐指令檢查模擬陷阱。
- 城之入口 field 34400–34700：左延伸 48 像素內的 16×16 圖塊 4640 → 14368、右延伸 4359 → 17340；
  超出 16–64 像素的圖元每 16 像素一格由約 600 增為約 5000，左右各三格；每 field 圖塊約 385 → 480。
- DRA 反組譯：`workplace/ws/dra35000.s`（由 field 35000 的 RAM 以 `mips-linux-gnu-objdump` 產生）。
- `Xeeynamo/sotn-decomp`（AGPL-3.0，`002` §3）commit `8a42c0036e0609a1d0fda3cd8d10251bf86a82c0`：只查名稱、結構、常數，不搬程式碼。

## 7. 驗收結果

目前狀態表（2026-10-17）：**列首指標修正已完成，畫面延伸驗收未通過；依規格衝突停止**。
效能未量測。實作入口為 `apps/sotn/widescreen.go`。

| 項目 | 目前結果 | 證據 |
|---|---|---|
| 簽章與解除 | 通過；九字正規化、重載、解除、未知改字拒絕與原字／改字混用 | `apps/sotn/widescreen_test.go`、`workplace/ws/unit-fixed.log` |
| BREAK | 三處陷阱通過；列首陷阱涵蓋負值、零、正值、溢位、位址／code 配對與 Bp 例外 | `workplace/ws/unit-fixed.log` |
| 側邊畫布 | 倍率 1、2、4、8 接縫、雙緩衝、填色、傳輸隔離、貼圖、混色與 mask 通過 | `render/hd/widescreen_test.go`、`workplace/ws/regression-fixed.log` |
| CPU 熱路徑 | 與 main 核對通過；step、fastFetch、Machine.Step 相同，execute 僅有既存的 BREAK 冷路徑呼叫 | `workplace/ws/hot-path-fixed.json`、`workplace/ws/core-main-fixed.diff` |
| 既有測試與 CPU／GTE／GPU 語料 | 全專案回歸通過，包含指定 BIOS、1150 筆 GTE 實機紀錄與 17 個 N=1 GPU 語料 | `workplace/ws/regression-fixed.log` |
| N=1 遊戲轉送 | 開機至序章 12000 個 field 的完整 VRAM 逐位元相同 | `workplace/ws/m4-fixed.log` |
| M4 原生摘要、記憶卡 | PSGOLEM_LONG=1，正常輸入完成 field 45784；摘要 `6c35c2701228722f`，記憶卡目錄檢查通過，SHA-256 為 `5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36` | `workplace/ws/m4-fixed.log` |
| M4 HD2、HD2-PGXP 比較 | 未完成；HD2 重播因畫面驗收停止，HD2-PGXP 未執行，整個 TestM4FirstSave 沒有通過收據 | `workplace/ws/m4-fixed.log` |
| 王座廳樓梯 field 15000 | 未通過延伸驗收；兩張 HD2 圖均為 684×480，中央與整張 PNG 完全相同，兩側非黑像素均為 0 | `workplace/ws/stairs-hd2-padded.png`、`workplace/ws/stairs-hd2-wide.png`、`workplace/ws/visual-stairs-fixed.json` |
| 城之入口 field 34500 | 補黑版已取得；寬螢幕重播最後紀錄為 field 25000，因停止條件取消，尚無本輪寬螢幕圖 | `workplace/ws/entrance-hd2-padded.png`、`workplace/ws/captures-fixed.log` |
| 遊玩簽章 | field 1205 首次相符並套用九字；截至最後紀錄未見失配，僅限已跑區段 | `workplace/ws/captures-fixed.log` |
| 前端與資料競爭 | 全專案前端測試及陷阱／側邊畫布的 race 檢查通過 | `workplace/ws/regression-fixed.log`、`workplace/ws/race-fixed.log` |
| 效能 | 未量測，不宣稱符合 10% 門檻 | 本輪使用者決定 |
| 三平台建置 | 通過；本機驗收版 `v0.4.0-ws`，macOS 雙架構、簽章與動態庫結構通過，未做真機驗收 | `workplace/ws/build-dist-fixed.log` |
| 三包內容 | 檔案清單、SHA-256、架構、版號、文件與素材排除通過 | `workplace/ws/packages.json`、`workplace/ws/package-validation-fixed.log` |
| Linux 封包啟動 | 通過；寬螢幕自動 HD1、不印摘要、HD2-xBR 視窗 1280×720、設定重讀畫面相同，明確關閉後為 960×720 | `workplace/ws/package-smoke-linux.json`、`workplace/ws/package-smoke-linux-fixed.log`、`workplace/ws/linux-ldd-fixed.log` |
| Windows 封包啟動 | Wine 環境在啟動前拒絕 `/tmp` 擁有權，未取得啟動收據 | `workplace/ws/windows-psexe.log`、`workplace/ws/package-smoke-windows-fixed.log` |

### 畫面與停止原因

- 已證實：樓梯圖中，王座、樓梯、柱子、德古拉與 Richter 的位置和補黑版完全相同，右偏 48 已消失。
  原生 x=0、x=256 對應 PNG x=86、x=598。左延伸區黑色符合 scrollX=0 的房間外限制；
  右延伸區也全黑，沒有取得背景延伸與延伸區實體的驗收證據。兩張圖 SHA-256 同為
  `e0d6d0b8927d73bb2f5a79ef279d0a9f87cc332f06bc68c1e75a52a034933cb3`。
- 已證實：`render/hd/widescreen.go` 的 wideScreen 依 §4，要求側邊畫布的繪圖區左上角與顯示起點完全相同，
  且寬度相同，才把側邊接到輸出；其他情況只補黑。
- 強證據：此匹配條件擋住正常遊玩畫面的側邊輸出。既有正常輸入 GP0 收據在 field 15000、34500
  有 `E3005000`、`E40388FF`，繪圖區為原生 VRAM 座標 (0,20)–(255,226)；
  雙緩衝另一側為 (256,20)–(511,226)。該收據在 field 35000 的顯示起點為 (0,0)。
  y=20 的遊玩繪圖區不能符合 §4 的左上角相同條件，與 §5 要求正常遊玩背景延伸衝突。
  來源為 `workplace/hd/entrance-gp0.json`，摘要索引為 `workplace/ws/draw-area-existing-fixed.json`。
  這不是本輪兩個截圖點的影子內部狀態傾印，不把側邊畫布實際內容或兩點的顯示起點另稱為已證實。
- 目前阻塞在 §4 的合成契約。未修改影子 GPU，未以房間邊界限制掩蓋未取得的延伸證據。
  需要補齊顯示區與繪圖區的對應證據，勘誤並審查規格後，才能繼續正式實作。

通用 BREAK 介面為 `internal/cpu/break.go`，影子側邊畫布為 `render/hd/widescreen.go`。
前端只在 field 結束後呼叫遊戲檢查；觀測 API 與 Machine.Step 沒有接入修改模式。
玩家操作與限制見 `docs/play.md`。封包位於 `dist/v0.4.0-ws/`，僅供本機驗收，沒有發布。

本輪工具、輸入與截圖雜湊見 `workplace/ws/tools-fixed.log`、`inputs-fixed.sha256`、`screenshots-fixed.sha256`。
封包驗證與啟動腳本為 `workplace/ws/verify_packages.py`、`package_smoke.py`、`package_smoke_windows.sh`；
像素核對原始碼為 `workplace/ws/imagecheck.go.txt`，只用 Go 標準庫。
所有原版素材、截圖、記憶卡、封包與驗證輸出均未加入 Git。
Docker 與檔案衛生收據為 `workplace/ws/containers-fixed.log`、`root-files-fixed.log`、`ownership-fixed.log`。
