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

subX 由原 scrollX 的低 4 位取得（`0x800ECD60`、`0x800ED190`，在改動之前執行），減 48 不影響低 4 位。
scrollX 小於 48 時 startTx 為負，原程式的 `tx < 0` 會跳過，與房間左緣外不畫一致。

### 3.3 陷阱

`0x800ECCF0` 前後沒有可用的空位，改成 `BREAK` 指令（code 欄位取專用值）。CPU 執行 `BREAK` 時（冷路徑），
若有掛上的處理器且位址與 code 相符，就模擬原指令 `sh v1,0(s0)`、寫入 `v1 − 48`、PC 前進一條，不進例外；
否則照原本進 Bp 例外。`0x800ECCF0` 不在延遲槽。不得在 `step`、`fastFetch`、`execute` 分派或 `Machine.Step` 加判斷。

### 3.4 套用與解除

- 每個 field 結束時檢查：§3.1 的 2332 位元組（把已套用的 7 個字當成原字計算）SHA-256 相符才套用；不符就不套用、已套用的不動。
  DRA 重新載入會蓋掉改動，下一次檢查重新套用。
- 關閉模式時把 7 個字寫回原字並卸下陷阱處理器。

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
| 簽章 | 單元測試（不用原版素材）：雜湊不符時不套用；套用後再解除，7 個字回到原字 |
| 陷阱 | 單元測試：手寫 PS-EXE 片段在指定位址執行 `BREAK`，處理器模擬 `sh` 並減 48；沒有處理器時進 Bp 例外，與現行行為相同 |
| 畫面 | 序章王座廳樓梯（約 field 15000）、城之入口（約 field 34500）各存一組 `-hd 2` 截圖：原畫面補黑版、寬螢幕版；目視背景圖塊在 x=0 與 x=256 接縫處連續、實體在延伸區正常出現 |
| 效能 | `psexe -hd 2 -widescreen` 與 `-hd 2` 同段 CPU 時間，降幅在 10% 以內（`012` §3 量測方法） |
| 交叉編譯 | `tools/build_dist.sh` 三平台通過 |

## 6. 證據

- probe 程式與輸出在 `workplace/perf/gp0probe/`、`workplace/ws/`（不入版控）：以 GPU trace 統計圖元相對繪圖區的水平範圍；
  `-patch` 以 RAM 寫入套用 §3.2 的 6 個字並以逐指令檢查模擬陷阱。
- 城之入口 field 34400–34700：左延伸 48 像素內的 16×16 圖塊 4640 → 14368、右延伸 4359 → 17340；
  超出 16–64 像素的圖元每 16 像素一格由約 600 增為約 5000，左右各三格；每 field 圖塊約 385 → 480。
- DRA 反組譯：`workplace/ws/dra35000.s`（由 field 35000 的 RAM 以 `mips-linux-gnu-objdump` 產生）。
- `Xeeynamo/sotn-decomp`（AGPL-3.0，`002` §3）commit `8a42c0036e0609a1d0fda3cd8d10251bf86a82c0`：只查名稱、結構、常數，不搬程式碼。

## 7. 驗收結果

目前狀態表（2026-10-17）：**畫面驗收未通過，已依使用者停止條件停工**。
field 15000 的寬螢幕畫面中，背景王座、樓梯與柱子相對人物約向右偏移 48 個原生像素，
左右延伸區仍大幅留黑。x=0、x=256 處未取得所要求的背景延伸連續性。
人物位置與背景的相對關係已不同於補黑版；不能以房間邊界、HUD 或 overlay 限制解釋此整體偏移。
這是目視確認的失敗；根因尚未確認，不把它歸咎於規格或特定位址。
沒有繼續猜補修改，尚在執行的 M4 與截圖重播已停止。
效能依本次使用者決定為「未量測」，不宣稱符合 §5 的 10% 門檻。

| 項目 | 目前結果 | 證據 |
|---|---|---|
| 簽章與解除 | 無素材單元測試通過，包含不符時不寫入、正規化、DRA 重載與解除 | `apps/sotn/widescreen_test.go`、`workplace/ws/unit.log` |
| BREAK | 手寫 PS-EXE 通過；核對位址、code、延遲槽、延遲載入與未掛處理器的 Bp | `apps/sotn/widescreen_test.go`、`workplace/ws/unit.log` |
| 側邊畫布 | 倍率 1、2、4、8 的接縫測試通過；雙緩衝、填色、傳輸隔離、貼圖、混色與 mask 通過 | `render/hd/widescreen_test.go`、`workplace/ws/unit.log` |
| CPU 熱路徑 | 與 main 比對通過，只有 BREAK 處理改動 | `workplace/ws/hot-path-check.log` |
| 既有測試與 N=1 GPU 語料 | 完整回歸通過，唯讀掛載指定 BIOS 與 ps1-tests，包含 CPU／GTE 與 17 個 GPU 執行檔 | `workplace/ws/regression.log` |
| N=1 遊戲轉送 | 開機至序章 12000 個 field 的完整 VRAM 逐位元相同 | `workplace/ws/m4.log` |
| M4 摘要、記憶卡 | 未完成。原生重播最後進度為 field 20000，因畫面停止條件取消；沒有取得本輪 `6c35c2701228722f` 收據 | `workplace/ws/m4.log` |
| 序章王座廳樓梯 | 未通過。field 15000、HD2，補黑與寬螢幕各一張，均為 684×480；背景相對人物偏移 | `workplace/ws/stairs-hd2-padded.png`、`workplace/ws/stairs-hd2-wide.png`、`workplace/ws/screenshots.sha256` |
| 城之入口 | 未完成。到 field 15000 後因前項失敗停止，未到 field 34500 | `workplace/ws/captures-wide.log`、`workplace/ws/captures-padded.log` |
| 遊玩簽章 | field 1205 首次相符並套用，至最後紀錄未見失配；僅限本輪已跑區段 | `workplace/ws/captures-wide.log` |
| 前端與資料競爭 | 16:9 比例單元測試及新增核心測試的 race 檢查通過 | `workplace/ws/frontend-unit.log`、`workplace/ws/race.log` |
| 效能 | 未量測 | 使用者指定先確認可執行，後續再優化 |
| 三平台建置 | 通過，本機版本 `v0.4.0-ws`；macOS 雙架構、簽章與動態庫結構通過，未做真機驗收 | `workplace/ws/build-dist.log` |
| 三包內容 | 檔案清單、SHA-256、平台架構、版號、文件與素材排除檢查通過 | `workplace/ws/packages.json`、`workplace/ws/package-validation.log` |
| Linux 封包啟動 | 通過，寬螢幕自動 N=1、不印摘要、HD2-xBR 視窗 1280×720、設定重讀畫面相同，明確關閉後為 960×720 | `workplace/ws/package-smoke-linux.json`、`workplace/ws/linux-ldd.log` |
| Windows 封包啟動 | 未完成。Wine 環境拒絕暫存設定目錄的擁有權，尚未取得程式啟動收據 | `workplace/ws/windows-psexe.log`、`workplace/ws/package-smoke-windows-shell.log` |

實作入口為 `apps/sotn/widescreen.go`，通用 BREAK 介面為 `internal/cpu/break.go`，
影子側邊畫布為 `render/hd/widescreen.go`。前端只在 field 結束後呼叫遊戲檢查，
觀測 API 與 `Machine.Step` 沒有接入修改模式。玩家操作見 `docs/play.md`「《月下》寬螢幕」。

新增驗收程式與實作檔均由本節收錄；工作歷程沿用 `WORKLOG.md`。

### 重生入口與證據邊界

實作提交為 `3c86695`。原版輸入與腳本的 SHA-256 在 `workplace/ws/inputs.sha256`。
建置使用既有 `psgolem-go-ebiten:1.24-r1`、Go 1.24.13 與既有 osxcross、Wine 映像，沒有新建或刪除映像。
本機三包位於 `dist/v0.4.0-ws/`，因畫面未通過，不能稱為驗收完成的交付包，未發布。

截圖入口為 `apps/sotn/widescreen_capture_test.go`，需要原版唯讀掛載與下列環境：

```sh
PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_BIOS=/orig/bios/SCPH5500.BIN \
SOTN_CUE=/orig/disc/sotn.cue PSGOLEM_WS_EVIDENCE=/src/workplace/ws \
PSGOLEM_TIMEOUT=95m tools/go.sh test ./apps/sotn \
  -run TestWidescreenCaptures -v -count=1 -timeout 90m
```

本輪為了使用最終繪圖區清除順序，取消早先的 `captures.log`，
改以 `/wide` 與 `/padded` 子測試分別重跑，正式目視證據來自兩份分開的 log。
`workplace/ws/verify_packages.py` 檢查封包；`package_smoke.py` 為 Linux 啟動檢查，
`package_smoke_windows.sh` 為尚未完成的 Wine 檢查。這些本機工具與截圖均未加入版控。
Docker 收尾沒有本輪殘留容器，沒有 root-owned 檔案或異常 `.md` 目錄；
證據在 `workplace/ws/containers-final.log`、`root-files-final.log` 與 `ownership-final.log`。
