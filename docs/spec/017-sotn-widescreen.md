# 017 《月下》寬螢幕 16:9

狀態：`READY`（合成與顯示世代清除已實作，指定驗收完成；平台限制見 §7）

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
- 遊戲暫放在 4:3 框外的圖元會在延伸區露出。序章王座廳 field 15000 有三個 GP0 `0x3D` 貼圖四邊形畫在 x=264–296、y=22–118（骷髏加兩道洋紅直條，像滑入式的計量條），原版被繪圖區裁掉。

## 4. 畫面端（影子 GPU）

原生 GPU 與 VRAM 不變。寬螢幕需要影子 GPU（`-hd` 1 以上；`-widescreen` 沒有 `-hd` 時自動用 N=1）。

| 項目 | 規定 |
|---|---|
| 側邊畫布 | 每個繪圖區（E3／E4 定義的矩形）左右各配一塊 M=48 原生像素寬、與繪圖區同高的畫布（N 倍），不在 VRAM 裡 |
| 光柵化 | 寬螢幕開啟時，影子以 `[x1 − M, x2 + M]` 作為水平裁切範圍；落在 `[x1, x2]` 的像素寫 VRAM，左右超出的寫進該繪圖區的側邊畫布。貼圖、CLUT 照常從影子 VRAM 讀 |
| 清除 | 每塊側邊畫布記錄上次清除時的顯示世代；GP1 05h 改變顯示起點時世代加 1。某塊繪圖區在世代改變後第一次有圖元畫進時清成黑色，同一世代內 E3 切走再切回不清。GP0 02h 填色覆蓋整個繪圖區寬度時，同列的側邊畫布一起填 |
| 其他 | VRAM 複製、CPU↔VRAM 傳輸不碰側邊畫布 |
| 輸出 | 選一塊繪圖區：x1 等於顯示起點 x、寬度等於顯示寬度、y1 落在顯示範圍 `[顯示起點 y, 顯示起點 y + 顯示高度)` 內；有多塊符合時取最近一次有圖元畫進的那塊。輸出 `左側邊 + 顯示範圍 + 右側邊` 再裁成中央寬 342 原生像素（256 寬、像素比 1.25 時的 16:9），側邊畫布的第 0 列對齊顯示的第 `y1 − 顯示起點 y` 列，超出繪圖區高度的列為黑色。沒有符合的繪圖區（24 位元動畫、顯示範圍對不上）時輸出原畫面左右補黑到同寬 |
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
- DRA 反組譯：主工作樹 `~/cht/ps-golem/workplace/ws/dra35000.s`（由 field 35000 的 RAM 以 `mips-linux-gnu-objdump` 產生），§3.2 的原字與 §3.3 的延遲槽、前一條指令皆以此核對。
- `Xeeynamo/sotn-decomp`（AGPL-3.0，`002` §3）commit `8a42c0036e0609a1d0fda3cd8d10251bf86a82c0`：只查名稱、結構、常數，不搬程式碼。

## 7. 驗收結果

目前狀態表（2026-10-17）：§4 的合成與顯示世代清除已實作，§5 指定驗收完成。
效能未量測。實作入口為 `apps/sotn/widescreen.go`、`render/hd/widescreen.go`。

| 項目 | 目前結果 | 證據 |
|---|---|---|
| 簽章、解除與 BREAK | 通過，九字與三處陷阱保留 | `workplace/ws/unit-generation.log` |
| 側邊合成 | y1 偏移、水平雙緩衝、邊界補黑、多塊選最近實際繪製區通過；涵蓋倍率 1、2、4、8 | `render/hd/widescreen_test.go`、`workplace/ws/unit-generation.log` |
| 顯示世代清除與填色 | 通過；同世代 E3 往返保留，新世代各區首次實際畫進才清；填色與遮罩依 §4 | `workplace/ws/unit-generation.log` |
| 核心與遊戲改字 | 相對 45cc955 無修改；CPU 熱路徑與 Machine.Step 和 main 相同，execute 僅有既有 BREAK 冷路徑差異 | `workplace/ws/core-generation.diff`、`hot-path-generation.json` |
| 完整 M4 | PSGOLEM_LONG=1，原生、HD2、HD2-PGXP 均完成 field 45784，完整摘要、記憶卡與原生畫面相同，摘要為 6c35c2701228722f；容器實際退出碼 0 | `workplace/ws/m4-generation.log`、`m4-container-exit-generation.log`、`m4-container-events-generation.jsonl` |
| N=1 遊戲轉送 | 12000 個 field 的完整 VRAM 逐位元相同 | `workplace/ws/m4-generation.log` |
| 全專案回歸 | 通過，含 CPU、1150 筆 GTE 與全部 17 個 N=1 GPU 語料 | `workplace/ws/regression-generation.log` |
| 王座廳樓梯 field 15000 | HD2 兩圖中央像素零差異；左側房間外全黑，右側背景接續，人物位置一致 | `workplace/ws/stairs-hd2-padded.png`、`stairs-hd2-wide.png`、`visual-stairs-generation.json` |
| 城之入口 field 34500 | HD2 兩圖中央像素零差異；左右背景接續、左側樹形實體延伸正常，中央實體位置一致 | `workplace/ws/entrance-hd2-padded.png`、`entrance-hd2-wide.png`、`visual-entrance-generation.json` |
| 效能 | 未量測，不判定 10% 門檻 | 使用者決定 |
| 資料競爭 | 側邊合成、清除與簽章／陷阱測試通過 | `workplace/ws/race-generation.log` |
| 三平台建置與封包 | v0.4.0-ws 通過，含 macOS 四道結構、封包內容、雜湊、架構、版號與素材排除檢查；未發布 | `workplace/ws/build-dist-generation.log`、`package-validation-generation.log`、`packages.json` |
| Linux 啟動 | 通過；無缺庫，自動 HD1 342×240、摘要抑制、HD2／xBR 1280×720、設定重讀畫面相同，明確關閉後 960×720 | `workplace/ws/package-smoke-linux-generation.log`、`package-smoke-linux.json` |
| Windows 啟動 | 未取得啟動收據；Wine 因 /tmp 擁有權拒絕建立設定目錄 | `workplace/ws/windows-psexe.log`、`package-smoke-windows-generation.log` |

王座廳 field 15000：原生 x=0、x=256 對應 PNG x=86、x=598，側邊第 0 列對齊 PNG y=40。
中央 512×480 像素零差異，德古拉位於王座階上，Richter 位於樓梯右下，位置一致。
scrollX 為 0，x=0 左側是房間外，延伸區全黑，屬 §3.5 已知限制。
x=256 右側的牆面、柱基與地板接續，未見錯位；本次視野內背景延伸到右緣。
右延伸區非黑像素為 33764，左側為 0。本點未證明玩法實體在延伸區的顯示。

城之入口 field 34500：中央 512×480 像素零差異，巨狼與中央樹形實體的位置、大小一致。
x=0、x=256 的牆面、窗框與地板接續，未見圖塊錯位或延伸區垃圾；左側樹形實體的身體與根部跨接縫正常。
左右延伸區非黑像素分別為 35604、31205。兩場景均由空白記憶卡與同一按鍵腳本分別重播，
寬螢幕版在 field 1205 簽章相符並套用；畫面比較不宣稱修改模式與原版機器狀態相同。

重跑命令為 `workplace/ws/commands-generation.txt`；工具、輸入與實作雜湊見
`tools-generation.log`、`images-generation.log`、`inputs-generation.sha256`、`source-generation.sha256`，皆位於 `workplace/ws/`。
四張截圖雜湊為 `workplace/ws/screenshots-generation.sha256`，完整擷取日誌為 `captures-generation.log`；
容器實際退出碼 0 與自動刪除事件見 `captures-container-exit-generation.log`、`captures-container-events-generation.jsonl`。
總驗收索引為 `workplace/ws/verification-generation.json`，含實作與文件提交、輸入、工具、封包大小與雜湊。
檔案與 Docker 收尾核對見 `root-files-generation.log`、`ownership-generation.log`、`containers-generation.log`，皆位於 `workplace/ws/`。

玩家操作與限制見 `docs/play.md`。封包輸出為指定的 `dist/v0.4.0-ws/`，僅供本機驗收。
原版素材、截圖、記憶卡、封包與驗證輸出均不加入 Git。

## 8. 工具列保存視野偏好

狀態：READY。使用者於2026-10-25要求真正擴大可見場景，並保留原版模式與既有海報。現有主程式已從settings.widescreen啟動本規格的簽章修改，前端工具列與原子保存沿020／024。本節只補玩家可操作入口，不新增即時改字或新資料格式。

- F1／滑鼠頂緣工具列新增「視野 原版／擴大」。選擇只保存下一次啟動的widescreen偏好，當場畫面、RAM、AfterField及HD影子不變；明示「下次啟動套用」。不將未套用的設定假裝成正在顯示的模式。
- 原設定的HD倍率、解析度、材質、PGXP、透視、海報與手把保持。選擇後再保存手把或海報、或從設定畫面開啟遊戲時，不能把待套用視野覆寫回本次啟動值。明確CLI仍沿現有優先順序。
- 保存失敗顯示錯誤並保留原偏好。下一次啟動沿本規格既有流程；HD為0而啟用寬視野時仍沿既有自動HD1。關閉寬視野後恢復海報呈現及已保存的A／B／C選擇。
- 一次驗證偏好保存／回讀、其他設定保持、保存失敗及前端本次模式保持。此變更不改GPU、遊戲程式或017的時序與寬視野限制；現有正常原版與寬視野證據分別承接，不重跑美術。

本節實作已通過兩項設定／前端測試，實際Linux開發視窗完成保存、獨立重開及復原；另補尚未取得的可見工具列與正常關窗。最初兩次驗證工具因JSON空值省略及固定更新等待停止，來源／舊圖保留，只承接完成動作，未重做偏好操作。總三個視窗不是三輪美術，沒有掛原版素材或正常遊戲GUI；Windows／macOS未重編，本機v.0.7.9保持，尚未裝入三平台完整版。工具與分段收據見相鄰素材專案workplace/render-control-v1/toolbar-wide-v1/。
