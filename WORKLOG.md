# WORKLOG

規格正文只寫現況；被推翻的推論與原因記在這裡。

## 勘誤

| 日期 | 原推論 | 推翻證據 | 教訓 |
|---|---|---|---|
| 2026-10-15 | `gpu/rectangles` 的 2288 個不符像素是參考圖出錯：平均模式是原色、相減模式全黑，與 psx-spx 及 `gpu/transparency` 矛盾，推論參考圖來自有誤的模擬器 | 加入 `008-cycle-model` 後同一測試 0 不符。程式每個 frame 在同一處重畫半透明矩形、不清背景，混色逐 frame 累積：平均收斂到原色、相減收斂到黑。先前 VSync 一直逾時，跑的 frame 數不同 | 參考圖與實作不符時，先看程式的主迴圈是否逐 frame 累積狀態，以及時序是否已經正確，再懷疑參考圖 |
| 2026-10-15 | （程式錯誤，非推論）DICR 寫入後才判斷 bit 31 的上升緣 | 先覆蓋 DICR 會把舊的 bit 31 清掉，未確認的 channel 2 旗標使每次寫入都被當成新的上升緣，重複觸發 DMA 中斷；光碟開機時核心卡在中斷分派迴圈 | 邊緣觸發的判斷要在修改狀態之前取舊值 |
| 2026-10-15 | （程式錯誤，非推論）I/O 的 8／16 位元寫入以「讀回目前值再合併」實作 | 《月下》CD 中斷處理以 `sb` 寫 DICR bit 16–23，合併時把 bit 24–30 的旗標寫回 1 而清掉，MDEC 輸出 DMA 的完成中斷遺失，動畫出現 `MDEC_in_sync timeout`。psx-spx 的 I/O Write Datasize 表標 DICR 為 (w32)；修正後 `io-access-bitwidth` 多 10 格相符 | 台帳裡「實機寫入整個暫存器」那類已知差異，遊戲可能依賴；已知差異不能因為沒有遊戲出事就擱著 |
| 2026-10-15 | `006` §7.1 寫「108 格中 55 格相符、53 格不符」 | 台帳實際列 37 格（71 格相符），數字在之前修好 JOY 等格子時沒有跟著改 | 改台帳的同一個 commit 要同步改規格裡的數字 |

## 2026-10-17：HD 第 1、2 階段

- 工作樹 `/home/anr2/cht/ps-golem-hd`，分支 `hd`，起點 `687b33b`，作者 email `wicanr2@gmail.com`。
  開工讀取專案規格與 Codex 的 READY 閘門、文件職責路由；打包前另讀交付路由。
  依本次明確指定保留 `dist/v0.2.0-hd/` 作為本機驗收包位置，不發布。
- 在既有 Docker 內 clone 參考來源到 `workplace/ref/`。
  核實指定 Beetle commit 確有 `upscale_shift`；xBR-lv2 檔頭為 MIT，保留完整授權。
  紀錄在 `workplace/hd/references.log`。沒有建立或刪除映像，沒有主機建置或安裝套件。
- `4783e93` 實作 `render/hd/`、GP0／GP1／場奇偶的批次非同步轉送、Clone 隔離，
  接入兩個 CLI 與設定檔，加入 Kage xBR-lv2。第 3、4 階段未實作。
- N=1 的 17 個 ps1-tests GPU 執行檔、遊戲開機至序章 12000 個 field 的完整 VRAM 比對通過。
  M4 原生及 HD2 重播均通過，摘要、記憶卡與原生畫面相同。
  一般測試、資料競爭檢查與 CPU／GTE 實機語料通過。
  目前驗收台帳與證據入口只放 `docs/spec/016-hd-output.md` §7。
- 九張原生／HD2／HD4 截圖與 xvfb 的 none／xBR 截圖均存於 `workplace/hd/`，不入版控。
  入口 HD2 縮回有 147 個差異，GP0 幾何核對發現其中 64 個超出三角形邊界一個原生像素。
  新增不含原版素材的仿射貼圖反例，證實指定上游的取樣中心會造成內部差異。
  因此保留 §6 原判準並列為未通過，不宣稱 CONFORMED，也沒有自行更改取樣。
  已按 grilling 技能提出是否修訂判準的共同決策問題；未獲確認前保留現況。
- 以改動前、目前原生、HD2 交錯各三次，另做固定雙 P 核心的補充比較。
  正常排程速度降幅 14.85%，固定核心為 3.31%。總 CPU 成本另列，不混作速度。
  主機負載與逐輪數據在兩份 `performance*.json`，沒有只保留通過的單次數字。
- `tools/build_dist.sh v0.2.0-hd` 三平台通過。解包清單、SHA-256 與素材排除檢查通過。
  Linux 無缺庫、Windows 在 Wine 下啟動；macOS 僅完成結構驗收。
- 首次遊戲長程測試收到 SIGTERM，沒有產品斷言結果，原因未知；加入 field 進度後以相同工具鏈乾淨重跑通過。
  封包驗收腳本的 Python API 版本與 Wine prefix 擁有權問題均在容器內修正後重跑通過。
  臨時 Go 分析程式加上 `go:build ignore`，避免全專案測試收進多個 `main`；修正後全量重跑通過。
- Docker 工作均用 `--rm`，結束時核對本工作樹的容器與輸出擁有權。未 push、merge 或改寫歷史。
- 2026-10-17 驗收判準修訂：016 §6 原寫「`-hd 2` 縮回後差異只在多邊形邊緣」。上游 `upscale_shift` 的 UV 初始偏移隨倍率改變
  （§7.3 反例），貼圖多邊形內部本來就會取到不同 texel；入口的差異全在揮劍特效上，目視無錯位。
  判準改為允許貼圖多邊形內部的子 texel 重取樣。教訓：放大渲染的像素判準要先推導上游取樣規則再訂。

## 2026-10-17 psgolem 效能量測

- 前一輪在主機負載 42 下量到序章遊玩段每個 field 15–25 ms，曾據此判定需要加速。
  重新量測發現主機其他工作把時脈壓到最高時脈的約 30%，且 xvfb 的 llvmpipe 與模擬共用 `--cpus` 配額，這些數字不能代表模擬核心本身。
- 改為無頭量測並固定在 P 核心（`tools/go.sh` 新增 `PSGOLEM_CPUSET`）：遊玩段 9–14 ms／field（1.77 GHz）。結果寫在 012 §3。
- 遊玩段 profile：CPU 直譯約 72%，GPU 點陣化約 18%，未達值得把點陣化移到另一個 goroutine 的程度；暫不改模擬核心。
- 教訓：效能結論要附量測時的核心類型與時脈；混合架構筆電上，負載下的牆鐘數字先排除排程與降頻再下結論。

## 2026-10-17 PGXP 規格與寬螢幕可行性

- PGXP 授權核實：Beetle `pgxp/` 部分檔案無檔頭，追到原始出處 iCatButler/pcsxr（commit `62467b8`，COPYING 為 GPLv3），與本專案相容。002 原寫的路徑 `mednafen/psx/pgxp/` 不存在，實際在 repo 根目錄 `pgxp/`。
- 016 §5 第 3 階段改為 READY：只做 memory＋GTE 追蹤，算術追蹤、透視校正、頂點快取不做。
- 寬螢幕：Beetle 的做法改 GTE 投影，只對 3D 有效。第三方 PC 重編譯版的 16:9 改了遊戲繪圖邏輯，部分背景只有 4:3
  （Generation Amiga 2026-07-11〈Castlevania: Symphony of the Night PS1 recomp adds native 16:9 widescreen〉）。
  寬螢幕一定改變遊戲狀態，不能當 GPU 後端選項；維持 DRAFT，要做需另立 `apps/sotn` 遊戲修改規格。

## 2026-10-17 寬螢幕定案與逆向

- 使用者選定遊戲修改模式；AGENTS.md（兩份）列為決定性例外。
- 遊玩畫面 256×240，兩個 framebuffer 在 VRAM (0,0)、(256,0)，x=512 起是貼圖頁，VRAM 放不下加寬畫面；改用影子 GPU 的側邊畫布。
- 017 草稿原寫「畫面外的敵人、物件遊戲本來就不送給 GPU」，GPU trace probe 推翻：實體照常送出、由繪圖區裁切，只有背景圖塊受 17 格限制。
- 以 RAM 反組譯找到本光碟的 RenderTilemap（0x800ECC10），probe 套用改動後延伸區圖塊照預期增加。017 改為 READY。
- 教訓：「遊戲不送畫面外的東西」先用 GPU 命令流量測，不用對 2D 遊戲的一般印象。

## 2026-10-17 PGXP 第 3 階段

- 工作樹 `ps-golem-hd`、分支 `pgxp`，起點 `b68b4c7`。僅實作 016 §5，未實作 017。
- 路由命中 spec 閘門與文件職責，載入 Codex 的 `retro-remake-spec-gated-workflow.md`、`project-document-responsibilities.md`。
  目前狀態表沿用 016 §7，新結果放 §7.5；量測與素材截圖放忽略版控的 `workplace/pgxp/`。
- 指定 Beetle 參照工作樹乾淨，HEAD 為 `e57860d82faa953dc0fc10f201fd583739bc2f7d`。
  原始碼雜湊在 `workplace/pgxp/references.sha256`。只沿用既有 Docker 映像，未新增映像。
- 追蹤器保存主記憶體、scratchpad、GP0 埠與暫存器的 XY 附件。CPU 掛點限定於載入、儲存、COP2 搬移與 ADDU 的 `rt == 0` 分支。
  GTE 於原生 RTPS／RTPT 完成後觀測；CPU 通用分派、取指令與 Machine.Step 均未加掛點。
- 無素材搬移鏈、改字回退、DMA2 兩種模式、部分字搬移、載入延遲及跨批次測試通過。
  原生與啟用 PGXP 的 1150 筆 GTE 實機語料相符。全專案既有測試與影子資料競爭檢查通過。
- 環境修正：容器的 `bash -lc` 重設 PATH，導致基準建置找不到 Go；改用同一映像的 `bash -c` 乾淨重跑。
  新增投影測試原以精確除法預測原生 UNR 結果，改為與未掛觀測器的原生 GTE 比較；未改原生計算。
- M4、逐 field 調查、截圖、效能與三平台建置仍在驗證，尚未提交或發布。

### 後續驗證與勘誤

- 實作提交為 `10dd85b`，作者 email 為 `wicanr2@gmail.com`，含指定 Co-Authored-By 註記；未 push、merge 或改寫歷史。
- N=1 遊戲重播 12000 個 field 完整 VRAM 相同。M4 原生、HD2、HD2-PGXP 均通過，摘要均為 `6c35c2701228722f`，完整記憶卡與原生畫面相同。
  完整收據 `workplace/pgxp/m4.log`。長測試編譯於提交前；之後的正式程式差異僅有卸下影子時解除掛點，另修正測試輸出標籤與跨批次測試邊界。
- 全程 CSV 共 45784 列，RTPS 2214454 次、RTPT 389736 次，有效頂點 2202698／7132733。
  收據 `workplace/pgxp/m4-fields.csv`、彙總 `workplace/pgxp/survey.json`。
- 全活躍區間選出 30 組連續抽樣，每個變體 89 張。原生摘要全相同，68 組 PNG 完全相同，其他差異在多邊形與特效。
  原始圖、選樣、像素比較與縮圖對照均留於 `workplace/pgxp/`，未加入 Git。
- 勘誤：檢視 field 12823 時曾把顯示／判讀異常誤當成 PGXP 全黑，因而停止當時自己的截圖容器。
  三份獨立 PNG 的 SHA-256 都是 `82d85d581593b2f8d2f13f7bbe22c780a76573f6494c471c04a61d08a97a8a1a`，內容完全相同。
  診斷未發現大於兩個原生像素的精度座標差異，暫時掛點已移除；正式程式沒有因此修改。
  `workplace/pgxp/viewer-correction.json` 保存勘誤，完整 PGXP 重跑收據在 `workplace/pgxp/captures-pgxp.log`。
  `workplace/hd/diff.go` 是舊輪次的固定輸入腳本，不接受本輪命令列路徑；其輸出沒有用作本輪證據。
- 已讀 `psx_pgxp_2d_tol`；本次不移植選用回退，沿用上游預設停用。決定與實際前後對照記於 016 §7.5。
- `tools/build_dist.sh v0.3.0-pgxp` 通過三平台建置，包內清單與雜湊檢查通過。Linux 與 Wine Windows 的 PGXP field 800 摘要與 PNG 相同，設定持久化與重新開啟通過。
  Wine 首次因 `HOME=/tmp` 擁有權拒絕初始化；同一映像內建立本 UID 的暫存家目錄後乾淨重跑通過，沒有改產品程式或另建映像。
- 效能在功能測試、封包冒煙、截圖與像素比較全部結束後單獨量測，尚待三輪最小值與雜訊閘門結果。

- 效能三輪共 15 次完成，`workplace/pgxp/performance.json` 的兩個 PGXP 關閉閘門與 25% 開啟閘門均通過。
  PGXP 速度降幅 17.23%；原生與 HD2 關閉 A/B 的實測噪幅達 199.59% 與 150.90%，只作本次測段的門檻判定。
  原始結果沒有剔除、挑選或重跑調參，所有執行摘要均為 `030643de52be3c27`。
- 熱路徑函式逐字檢查在 `workplace/pgxp/hot-path-check.log`，掛點差異在 `workplace/pgxp/hooks.diff`。
  原始碼證據快照使用 `.go.txt` 副檔名，避免 Go 的 `./...` 掃描將快照當成套件。
- 最終完整回歸與 Docker／檔案衛生檢查進行中。

- 最終完整回歸通過，包含指定 BIOS、ps1-tests、1150 筆 GTE 與新增 PGXP 測試；收據 `workplace/pgxp/regression-final.log`。
  本次 016 §6 的 PGXP 各列已完成，狀態與限制回填 016 §7.5，索引同步更新。macOS 仍限結構驗收。
- 收尾檢查沒有本輪遺留容器，沒有 root-owned 檔案或異常 `.md` 目錄；封包與證據均為 UID/GID `1000:1000`。
  收據 `workplace/pgxp/containers-final.log`、`root-files-final.log`、`ownership-final.log`。
  只使用既有映像與一次性 `--rm` 容器，沒有新增或刪除映像，也沒有操作其他專案容器。
- 本機封包保留於 `dist/v0.3.0-pgxp/`，未發布；原版素材、截圖與量測輸出均未加入 Git。

## 2026-10-17：017 寬螢幕遊戲修改模式

- 開工確認工作樹乾淨，main 已含 PGXP 實作 `10dd85b` 與驗收 `d889b78`，從 main 建立 `widescreen`。
- 路由命中 READY 規格實作，載入 `retro-remake-spec-gated-workflow.md` 與文件職責。
  依使用者本次指定，沿用 `dist/v0.4.0-ws/` 作本機驗收包，不發布、不更動授權。
- 目前狀態表設於 `017-sotn-widescreen.md` §7。既有 `workplace/` 下建立指定的 `ws/` 保存本輪證據，
  素材透過 `/orig` 唯讀掛載。沒有新增研究根目錄。
- `apps/sotn/widescreen.go` 實作完整函式雜湊、七字修改、重載重套與解除。
  CPU 僅在 BREAK 冷路徑處理選用陷阱，保留原位址與 code 比對、SH 與延遲載入語意。
  `render/hd/widescreen.go` 實作獨立側邊畫布，貼圖與 CLUT 仍只讀影子 VRAM。
- 玩家 CLI 與設定檔新增預設關閉的 `widescreen`，沒有 HD 時自動選倍率 1。
  `psexe` 修改模式不印摘要，前端每個 field 結束才檢查簽章，失配回報錯誤。
- 無素材單元測試與熱路徑核對通過，證據為 `workplace/ws/unit.log`、`hot-path-check.log`。
  首輪全專案編譯發現設定 literal 重複欄位，修正後乾淨重跑；未改任何既有測試預期。
- M4、N=1、完整語料回歸、指定場景重播與三平台建置進行中。
  效能依使用者決定未量測。所有結果回填 017 §7 後才判定完成。

- 實作提交為 `3c86695`，未 push、未 merge、未改寫歷史。
- 完整語料回歸通過，包含指定 BIOS、CPU／GTE 與全部 N=1 GPU 語料；
  開機至序章 12000 個 field 的 VRAM 亦逐位元相同。
  M4 原生重播最後紀錄為 field 20000，尚未產生最終摘要與記憶卡比較收據。
- 畫面停止條件已觸發：目視 field 15000 的 HD2 對照，背景王座、樓梯與柱子相對人物約向右偏移
  48 個原生像素，左右延伸區仍大幅留黑。此差異不屬 017 §3.5 已知限制，根因尚未確認。
  收據為 `workplace/ws/stairs-hd2-{padded,wide}.png` 與兩份 `captures-{padded,wide}.log`。
  依使用者指令停止，沒有繼續修補；入口場景與完整 M4 未完成，017 保持 READY 並標明驗收失敗。
- `tools/build_dist.sh v0.4.0-ws` 三平台建置與 macOS 結構檢查通過，三包內容與 SHA-256 通過。
  Linux 實際封包啟動、xBR、設定重讀、明確關閉與摘要抑制通過。
  Wine 映像沒有 Python，改用 shell；其後 Wine 拒絕 `/tmp` 設定目錄的擁有權，
  改用容器使用者持有的暫存 HOME 後仍未通過。這是環境驗證失敗，不列為產品缺陷或 Windows 通過。
- 截圖重播曾因繪圖區清除順序的實作修正而取消一次，最後畫面取自最終程式的兩個子測試。
  原始輸入、工具、封包清單與本機驗證腳本均由 017 §7 索引。
- 停止後只檢查與停止掛載本工作樹的本輪一次性容器，沒有操作其他工作樹或專案容器。
  沒有 root-owned 檔案、異常 `.md` 目錄或本輪殘留容器；擁有權均為 `1000:1000`。
  本機包因畫面未通過而未完成交付驗收，僅保留於 `dist/v0.4.0-ws/`，沒有發布。

## 2026-10-17：寬螢幕背景右偏 48 的真因與規格勘誤

- 一次性探針（`workplace/ws/trapprobe/`，不入版控）確認陷阱在遊玩中每 field 觸發、無落空，存入 `0xFFD0`；
  逐指令追蹤 `0x1F800010` 在 RenderTilemap 內沒有被改寫。不開影子 GPU 的原生畫面同樣右偏，問題在遊戲端。
- 真因：每列只在列首算一次圖塊表指標，`tx < 0` 的格子被跳過時指標不前進。王座廳 scrollX 為 0，
  startTx 變成 −3，整列讀早 3 格，畫面右偏 48。
- 勘誤：017 §3.2 原本寫「startTx 為負時 `tx < 0` 會跳過，與房間左緣外不畫一致」，忽略了指標，已改為現況並新增
  `0x800ECE80`、`0x800ED260` 兩個陷阱（列首指標用 `max(t7, 0)`）。
- 驗證：探針加上兩個陷阱後，field 15000 原生畫面與未修改版相同（`workplace/ws/trapprobe/fix15000.png`）。

## 2026-10-17：依勘誤補齊列首指標陷阱與重新驗收

- 從乾淨的 `widescreen`、`b466844` 接手。已讀 AGENTS、規格索引、001、017 與上一節；
  路由載入 retro remake 技能、規格閘門、驗收、文件職責與打包規則。
  本輪依使用者明確指定沿用本機驗收版號 `v0.4.0-ws` 與 `tools/build_dist.sh`，不發布。
- `apps/sotn/widescreen.go` 新增 `0x800ECE80`、`0x800ED260` 的 BREAK，code 為 `0x5754`、`0x5755`。
  模擬 `v0 = max(int32(t7), 0) + t4`，保留 32 位元加法。簽章正規化、重載與解除共處理九字。
  沒有改 CPU、Machine.Step、影子 GPU、正式輸入或 M4 預期值，也沒有重新調查已確認的真因。
- 無素材測試新增兩處列首陷阱的負值、零、正值、溢位、位址／code 配對與未掛處理器、延遲槽例外。
  簽章測試補上兩個 code 的未知改字拒絕與原字／改字混用。收據為 `workplace/ws/unit-fixed.log`。
  資料競爭檢查通過，收據為 `workplace/ws/race-fixed.log`。
- 熱路徑與 main `d889b78` 比對通過：step、fastFetch、Machine.Step 相同；execute 僅有上一輪既存的
  BREAK 冷路徑呼叫。收據為 `workplace/ws/hot-path-fixed.json`、`core-main-fixed.diff`。
- 舊封包與建置目錄保留為 `workplace/ws/prior-dist-v0.4.0-ws/`、`prior-stage-v0.4.0-ws/`。
  舊樓梯失敗畫面、截圖雜湊與封包收據另以 `workplace/ws/prior-` 檔名前綴保留；
  上一輪相同檔名的截圖與封包證據請回查這些副本。本輪收據優先使用 `*-fixed.log`。
- `tools/build_dist.sh v0.4.0-ws` 重新建置三平台通過，macOS 結構四道檢查通過。
  包內容、SHA-256、架構、版號、操作文件與素材排除通過，收據為 `build-dist-fixed.log`、
  `package-validation-fixed.log`、`packages.json`，路徑皆位於 `workplace/ws/`。
- Wine 以既有 `psgolem-wine-verify:noble-r1` 重跑既有啟動腳本，仍在程式啟動前拒絕 `/tmp` 擁有權：
  `wine: '/tmp' is not owned by you, refusing to create a configuration directory there`。
  收據為 `workplace/ws/windows-psexe.log`、`package-smoke-windows-fixed.log`；不列為 Windows 啟動通過。
  沒有為此修改產品、建立映像或變更使用者指定的驗收範圍。
- 工具環境修正：輸入雜湊命令第一次未帶唯讀掛載，找不到 `/orig` 與 `/ps1tests`；
  補上既有掛載後用同一映像與命令乾淨重跑成功，收據為 `workplace/ws/inputs-fixed.sha256`。
  映像沒有 rg，容器內窄範圍搜尋改用 grep；主機未執行分析程式。
- N=1 開機至序章 12000 個 field 的完整 VRAM 逐位元相同。完整 M4、兩場景截圖、
  全專案回歸與 Linux 封包啟動仍在執行，完成後回填本節與 017 §7。效能依使用者決定不量測。

- 全專案回歸通過。命令為指定唯讀素材、`PSGOLEM_PS1TESTS=/ps1tests`、
  `PSGOLEM_GO_CMD=xvfb-run tools/go.sh -a go test ./... -count=1 -timeout=40m -v`，
  收據為 `workplace/ws/regression-fixed.log`。包含 CPU 語料、1150 筆 GTE 與全部 17 個 N=1 GPU 語料。
- Linux 本輪封包啟動通過：自動 HD1、摘要抑制、HD2-xBR 1280×720、設定重讀相同與關閉後 960×720。
  收據為 `workplace/ws/package-smoke-linux.json`、`package-smoke-linux-fixed.log`。
- M4 以 `PSGOLEM_LONG=1`、指定 BIOS／cue，執行
  `tools/go.sh test ./apps/sotn -run '^(TestM4FirstSave|TestShadowBootPrologue)$' -count=1 -timeout=170m -v`。
  原生正常輸入完整完成 field 45784，摘要 `6c35c2701228722f`，記憶卡目錄檢查通過；
  記憶卡 SHA-256 `5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36`。
  HD2 子測試已啟動，但因後述停止條件取消；HD2-PGXP 未跑，整個 TestM4FirstSave 不稱為通過。
- 本輪截圖使用 `PSGOLEM_WS_EVIDENCE=/src/workplace/ws`，執行
  `tools/go.sh test ./apps/sotn -run '^TestWidescreenCaptures$' -count=1 -timeout=110m -v`。
  已取得 field 15000 的 HD2 補黑、寬螢幕圖，以及 field 34500 的補黑圖。
  寬螢幕版 field 1205 通過九字簽章並套用；最後紀錄為 field 25000，未取得入口寬螢幕圖。
- 畫面勘誤：首次目視正確確認人物與背景不再右偏，卻誤把右緣判成延伸。
  Go 標準庫像素核對隨即推翻後者：樓梯兩張 PNG 完全相同，兩側非黑像素都是 0，
  SHA-256 同為 `e0d6d0b8927d73bb2f5a79ef279d0a9f87cc332f06bc68c1e75a52a034933cb3`。
  已向使用者更正，沒有把目視誤判回填為通過。
  收據為 `workplace/ws/visual-stairs-fixed.json`、`screenshots-fixed.sha256`；
  像素核對原始碼為 `workplace/ws/imagecheck.go.txt`。容器沒有 Pillow，故只用 Go 標準庫，沒有裝套件。
- 新停止原因：017 §4 要求繪圖區與顯示起點左上角完全相同才合成側邊。
  既有正常輸入 GP0 收據在 field 15000、34500 的繪圖區為 (0,20)–(255,226)，
  field 35000 顯示起點為 (0,0)。雙緩衝另一側同樣從 y=20 開始。
  `wideScreen` 嚴格比對 y 起點，與 §5 的正常遊玩延伸驗收衝突。
  合成契約的限制已由程式碼確認；它造成本輪全黑延伸的解釋標為強證據，尚未傾印本輪影子內部狀態。
  索引為 `workplace/ws/draw-area-existing-fixed.json`，原始來源為 `workplace/hd/entrance-gp0.json`。
  沒有重新調查列首指標真因，也沒有修改影子 GPU 或猜補新的合成規則。
- 依使用者「規格矛盾或無法照做」停止條件，停止兩個已確認掛載本工作樹的本輪 `--rm` 重播容器。
  M4 與截圖命令因此以 143 結束，屬主動取消，不列為關閉模式回歸失敗。
  017 §7 已改為本輪現況，待補證據與審查 §4 勘誤後才可繼續。
- 收尾沒有本工作樹殘留容器、root-owned 檔案或異常 `.md` 目錄；輸出與修改檔均為 `1000:1000`。
  收據為 `workplace/ws/containers-fixed.log`、`root-files-fixed.log`、`ownership-fixed.log`。
  只用既有映像，沒有新增／刪除映像、prune、操作其他專案容器或寫入其他工作樹。
  封包仍為本機驗收產物，沒有發布；效能未量測。

## 2026-10-17：017 §4 輸出條件勘誤

- 《月下》遊玩繪圖區為 (0,20)–(255,226) 與 (256,20)–(511,226)（`E3005000`、`E40388FF`），顯示起點為 (0,0) 或 (256,0)。
  原 §4 要求「顯示起點與繪圖區左上角相同」才合成側邊，遊玩時永遠不成立，輸出一直補黑。
- 改為：繪圖區 x1 與寬度對上顯示、y1 落在顯示範圍內即可合成，側邊畫布依 `y1 − 顯示起點 y` 垂直對齊；多塊符合時取最近一次畫進的。

## 2026-10-17：依 §4 勘誤重做側邊合成與驗收

- 從乾淨的 `widescreen`、`b3349cf` 接手。已讀 AGENTS、規格索引、001、017 與最新紀錄；
  路由載入 retro remake 技能、規格閘門、驗收、文件職責、版號與交付規則。
  依本輪明確指示沿用本機驗收版號 `v0.4.0-ws` 及既有打包腳本，不發布。
- 合成改為 x1 與寬度匹配、y1 落在顯示範圍內，側邊依相對 y 偏移對齊，超出側邊高度的列補黑。
  以圖元實際寫入像素的順序選取最近一塊，中央像素亦計入；零尺寸、被裁掉的圖元與狀態切換不更新順序。
  E3 清除契約、三處遊戲陷阱、CPU 與 Machine.Step 均未改動。
- 單元測試與 race 檢查通過。新增倍率 1、2、4、8 的 y 偏移、非零顯示 y、水平雙緩衝、邊界補黑及多塊選區。
  收據為 `workplace/ws/unit-compose.log`、`race-compose.log`；核心差異收據 `core-compose.diff` 為空。
- 截圖測試在目標前兩個 field 透過既有 GP0 Trace 只讀解碼邊界與 E3／E4，另記顯示起點。
  不把資料字誤認為命令，也不更動機器狀態。收據預定為 `draw-area-padded.log`、`draw-area-wide.log`。
- 舊封包與 stage 移到 `workplace/ws/pre-compose-dist-v0.4.0-ws/`、`pre-compose-stage-v0.4.0-ws/`；
  舊截圖與封包索引另存 `pre-compose-` 副本。完整 M4、N=1 轉送、截圖、全專案回歸與三平台重建執行中。
- 工具環境紀錄：首次列映像遭 sandbox 拒絕，改用既有主機 Docker 存取權限確認；
  首次讀取用到不存在的無 tag 映像，Docker 嘗試拉取但遭拒，沒有新增映像。
  此後均沿用 `psgolem-go-ebiten:1.24-r1` 等既有映像。`tools/go.sh` 同樣改用主機 Docker 權限執行。
- 實作提交為 `b1bfe2c`。首次 Git 索引寫入被 sandbox 視為唯讀，改用已授權的主機權限提交；
  作者 `wicanr2@gmail.com`，繁體中文訊息與 Codex 共同作者，沒有 push、merge 或改寫歷史。
- 回歸首遍掛到 `workplace/ps1tests` 解壓目錄，因缺 `tests.zip` 跳過語料。
  改掛既有 `testdata/ps1-tests` 後，同映像、同測試命令乾淨重跑全專案通過，
  含 CPU、1150 筆 GTE 與 17 個 N=1 GPU 語料。收據為 `regression-corpus-compose.log`。
  外層工具一度回報 143，但後續完整日誌各套件均通過，Docker 的 die 事件明列 exitCode=0；
  以 `regression-container-events-compose.jsonl` 為實際容器結束證據，不列為產品失敗。
- N=1 開機至序章 12000 個 field 的完整 VRAM 逐位元相同，收據為 `m4-compose.log`。
- 三平台重建、macOS 結構四道檢查、三包檔案／SHA-256／架構／版號／文件與素材排除通過，
  收據為 `build-dist-compose.log`、`package-validation-compose.log`、`packages.json`。
  Wine 重跑仍在啟動前拒絕 `/tmp` 擁有權，未改產品；收據為 `windows-psexe.log`、`package-smoke-windows-compose.log`。
- 解包腳本首次使用既有 Python 不支援的 `extractall(filter=...)`，Linux 啟動檢查因此找不到執行檔。
  先前封包安全路徑檢查已通過；改用相容解包方式，等解包完成後用同一映像乾淨重跑啟動檢查。
- 樓梯 field 15000 的 HD2 截圖已目視：王座、樓梯、德古拉與 Richter 位置一致，右側牆面與地板延伸，
  x=256 接縫未見錯位。Go 像素核對中央差異為 0，右側非黑像素 33764。
  左側沒有房間圖塊，實際為平坦暗紫底色，並非全黑，不宣稱全黑。
  截圖前兩個 field 只見水平雙緩衝 E3 切換，沒有同畫面切回遊玩區。
  收據為 `visual-stairs-compose.json`、`draw-area-wide.log`；工具為 `imagecheck-compose.go`。
- Linux 本輪封包啟動檢查通過：自動 HD1、不印摘要、HD2／xBR 1280×720、設定重讀畫面相同，
  明確關閉後 960×720。收據為 `package-smoke-linux-compose.log`、`package-smoke-linux.json`。
- 兩場景的補黑與寬螢幕截圖重播均完成，測試通過僅代表成功擷取，不代表畫面驗收通過。
  入口 field 34500 目視中央巨狼、樹形實體、燭台、窗框與布幕位置一致，但延伸區全黑。
  Go 核對兩圖完全相同，中央差異 0、左右非黑像素均為 0，SHA-256 均為
  `740966054f38bb8751b0120952a44d4e0fc344310f73d3ccfcfb279b2f0a367d`。
  收據為 `captures-wide-compose.log`、`captures-padded-compose.log`、`visual-entrance-compose.json`。
- 新停止原因已由命令邊界收據證實：field 34498 的 E3 從遊玩區 `(0,20)` 切至 `(512,448)` 再回來，
  另三次在 `(0,20)` 與 `(0,0)` 間往返。field 34499 的遊玩區為 `(256,20)`，往返情形相同。
  全範圍區 E4 為 `E4FFFFFF`；這些命令用途未知，不猜成 HUD。
  影子接收相同 GP0 命令，既有 E3 契約會在切回後首個圖元清空側邊。
  此規則造成入口側邊消失標為強證據，未另外傾印清除前後的影子內部畫布。
  索引為 `draw-area-wide.log`、`draw-area-padded.log`，不是沿用舊收據或把資料字當命令。
- 依使用者指定的 E3 停止條件停工，沒有修改清除規則。
  唯一仍執行的本輪 M4 容器經 inspect 確認掛載本工作樹、原版唯讀且 AutoRemove=true 後停止，
  外層以 143 結束。原生最後紀錄 field 30000，未到摘要比較點；HD2、HD2-PGXP 尚未執行。
  不宣稱完整 M4 通過，也沒有測得摘要改變。這是主動取消，不是關閉模式測試失敗。
- §7 與規格索引已更新為現況；效能未量測。封包仍是指定的本機驗收包，沒有發布。
  前端動態相依無缺件；psexe 為預期的靜態執行檔，`ldd` 的非動態訊息另存收據。
  工具、輸入、來源與截圖雜湊為 `tools-compose.log`、`inputs-compose.sha256`、`source-compose.sha256`、`screenshots-compose.sha256`。
- 結論前重新核對知識路由，仍命中本輪實作、驗收與文件職責，沒有新增決策或擴大範圍。
  收尾沒有本輪工具映像的殘留容器、root-owned 檔案或異常 `.md` 目錄，抽查修改與產物均為 `1000:1000`。
  收據為 `containers-compose.log`、`root-files-compose.log`、`ownership-compose.log`。
  本輪沒有建立／刪除映像、prune、刪除其他容器、寫入其他工作樹、push 或發布。

## 2026-10-17：017 §4 清除規則勘誤

- 城之入口 field 34498–34499 同一畫面內 E3 依序切到 (0,20)、(512,448)、(0,0)–(1023,1023) 再切回遊玩區，
  原規則「E3 起點改變就清側邊」在同一畫面內把遊玩區側邊清掉（`workplace/ws/draw-area-wide.log`）。
- 改為依顯示世代清除：GP1 05h 改變顯示起點時世代加 1，繪圖區在新世代第一次畫進時才清。

## 2026-10-17：依顯示世代清除側邊與完整驗收

本輪由 HEAD `45cc955` 接手。唯一現況表為 `docs/spec/017-sotn-widescreen.md` §7。
下表保留啟動長測前的檢查點，後續結果依本節時間序記錄：

| 項目 | 狀態 | 本輪證據 |
|---|---|---|
| 顯示世代清除、同世代 E3 往返、填色 | 已實作，倍率 1、2、4、8 單元測試通過 | `workplace/ws/unit-generation.log` |
| 關閉模式完整 M4、N=1 轉送、全專案回歸 | 待重跑 | 本節後續記錄 |
| 王座廳與入口 HD2 補黑／寬螢幕截圖 | 待重拍與目視 | 本節後續記錄 |
| 三平台本機驗收包 | 待乾淨重建 | `tools/build_dist.sh v0.4.0-ws` |
| 效能 | 未量測，依使用者決定不判定 10% 門檻 | 017 §5 |

- 已讀 AGENTS、000、001、017 與最後一節紀錄；路由命中畫面對拍、文件職責與跨平台打包，
  載入 retro remake 技能、規格閘門與驗收 reference、文件職責、交付與版號入口。
  本輪依明確授權沿用 `v0.4.0-ws` 與 `dist/` 本機驗收契約，不發布。
- 世代只在 GP1 05h 的有效 x 或 y 起點改變時增加；每塊側邊記錄清除世代，
  實際像素寫入才清除。側邊先清除再檢查遮罩與混色，填色只影響覆蓋全寬的對應列。
  E3／E4 狀態切換、零尺寸、完全被裁掉與透明圖元不消耗清除世代。
- 單元測試準備時兩次失敗屬測試輸入與舊契約斷言：x=300 仍在右側延伸裁切範圍內、
  原透明貼圖來源曾被測試圖元寫入，另有舊測試仍要求 E3 與空圖元清除。
  改用 x=400、未寫入的 15 位元貼圖來源，並按新世代契約更新舊斷言後通過。
  兩輪失敗後已重查路由與 §4，沒有放寬產品契約。
- 實作提交 `687f592`，作者 `wicanr2@gmail.com`，繁體中文訊息與 Codex 共同作者。
  目前只修改影子 GPU，`core-generation.diff` 為空；`hot-path-generation.json` 確認
  CPU 熱路徑與 Machine.Step 和 main 相同，execute 僅排除既有 BREAK 冷路徑差異。
- N=1 轉送已通過 12000 個 field 的完整 VRAM 逐位元比較；完整 M4 持續執行。
  側邊與簽章／陷阱資料競爭檢查通過，收據為 `workplace/ws/race-generation.log`。
- 三平台乾淨重建與 macOS 四道結構檢查通過，建置命令 exitCode=0，
  收據為 `workplace/ws/build-dist-generation.log`。
  首次封包檢查啟動過早，讀到尚未寫完的 ZIP，回報 BadZipFile。
  該次是驗證程序的先後順序錯誤，收據另存 `package-validation-generation-premature.log`；
  等建置命令成功結束後，以同一映像與腳本乾淨重跑，不列為產品缺陷。
- 全專案回歸通過，外層命令 exitCode=0。包含 CPU 語料、1150 筆 GTE 與全部 17 個 N=1 GPU 語料；
  收據為 `workplace/ws/regression-generation.log`。缺原版或長測條件的截圖／M4 測試由專用命令另跑。
- 三包檔案清單、SHA-256、版號、平台架構、操作文件與素材排除檢查乾淨重跑通過，
  收據為 `package-validation-generation.log`、`packages.json`。
  本輪重建後的 Linux、Windows 封包解到乾淨目錄，舊包、Linux 設定與記憶卡另存 `pre-generation-*`。
- Windows 啟動檢查仍在程式啟動前被 Wine 拒絕，訊息為 `/tmp` 不屬於目前使用者，
  無法建立設定目錄。收據為 `windows-psexe.log`、`package-smoke-windows-generation.log`。
  按使用者指示只記錄原因，沒有為此修改產品程式。
- Linux 驗證環境沒有 Python，原啟動腳本在啟動產品前失敗，收據另存
  `package-smoke-linux-generation-python-missing.log`。改用同一映像已有的 bash、od、cmp、grep、ldd，
  驗證相同的版號、尺寸、設定重讀、明確關閉與摘要抑制，不新建映像、不修改產品程式。
- Linux 本輪封包啟動通過：動態相依無缺庫、自動 HD1 342×240、摘要抑制、HD2／xBR 1280×720、
  設定重讀畫面逐位元相同、明確關閉後 960×720。收據為 `package-smoke-linux-generation.log`、
  `package-smoke-linux.json`、`linux-ldd-generation.log`；實際命令在 `commands-generation.txt`。
- 原生 M4 已由正常按鍵路徑完成 field 45784、26150980006 個週期，摘要 `6c35c2701228722f`，
  記憶卡目錄檢查通過；記憶卡 SHA-256 `5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36`，
  原生畫面 SHA-256 `02b9bc2db161fa5675d867aeec057ccebd5cb9f6912db0fe46ed159d78c8ddd7`。
  收據為 `m4-generation.log`；HD2 已開始，尚不宣稱完整 TestM4FirstSave 通過。
- 王座廳 field 15000 本輪 HD2 兩圖已目視與像素核對：中央 512×480 差異 0，
  德古拉與 Richter 位置一致；右側牆、柱基與地板接續，x=256 未見錯位，右側非黑像素 33764。
  左側非黑像素為 0，符合 scrollX=0、房間外沒有圖塊的 §3.5 限制。
  相較上一輪左側暗紫底色，本輪已清成黑色；右側保留已接受的合成。
  收據為 `stairs-hd2-padded.png`、`stairs-hd2-wide.png`、`visual-stairs-generation.json`，
  像素工具來源為 `imagecheck-generation.go.txt`，在容器 /tmp 複製成 Go 檔執行。
- M4 外層工具回報 143，但 Docker inspect 確認原容器仍為 running、OOMKilled=false，
  也沒有 die 事件，HD2 日誌已繼續到 field 10000。屬工具連線中斷，不是產品測試失敗。
  保留原容器與測試，改以 docker wait 追蹤真實退出碼，沒有取消或重跑 M4。
  當時收據為 `container-state-after-tool143-generation.log`、`m4-container-events-generation.jsonl`。
- 兩場景的補黑與寬螢幕正常按鍵重播均完成，截圖容器真實退出碼為 0，已自動刪除。
  完整日誌另以 docker logs 接續保存，再設為 `captures-generation.log`；原工具日誌另存 `captures-tool-generation.log`。
  收據為 `captures-container-exit-generation.log`、`captures-container-events-generation.jsonl`。
- 城之入口 field 34500 本輪 HD2 兩圖已目視與像素核對：中央 512×480 差異 0，
  巨狼與中央樹形實體位置、大小一致；x=0、x=256 的牆面、窗框與地板連續，
  左側樹形實體的身體與根部跨接縫正常，未見延伸區垃圾。
  左右非黑像素為 35604、31205，入口側邊已保留；與上一輪全黑結果的對照由 `pre-generation-*` 保存。
  收據為 `entrance-hd2-padded.png`、`entrance-hd2-wide.png`、`visual-entrance-generation.json`。
  四張 PNG 雜湊為 `screenshots-generation.sha256`，GPU 命令邊界為本輪 `draw-area-wide.log`、`draw-area-padded.log`。
- HD2 M4 已完成 field 45784，完整摘要與原生相同，包含 `6c35c2701228722f`；
  記憶卡與原生畫面逐位元相同，兩份 SHA-256 也與本輪原生結果一致。
  HD2-PGXP 已開始，尚不宣稱整個 TestM4FirstSave 通過。原容器完整日誌為 `m4-container-generation.log`。
- 完整 M4 與 N=1 專用命令已完成，原容器真實退出碼為 0，已自動刪除。
  原生、HD2、HD2-PGXP 三條正常按鍵路徑均完成 field 45784，完整摘要相同，尾碼均為 `6c35c2701228722f`，
  三份記憶卡與原生畫面逐位元相同，SHA-256 均與前述原生結果一致。
  整個 TestM4FirstSave 與兩個 HD 子測試均通過。收據為 `m4-generation.log`、`m4-container-generation.log`、
  `m4-container-exit-generation.log`、`m4-container-events-generation.jsonl`；原工具日誌另存 `m4-tool-generation.log`。
- 結論前重核本輪知識路由，仍命中畫面對拍、驗收與文件職責，沒有新增決策或擴大範圍。
  017 §7 與規格索引改為目前驗收結果；失敗、工具連線中斷與環境原因只留本節。
  效能未量測，未判定 10% 門檻。Wine 啟動受環境限制，macOS 未做真機驗收。
  本機驗收包仍在 `dist/v0.4.0-ws/`，沒有發布。
- 總驗收索引為 `verification-generation.json`，產生工具為 `write_receipt_generation.py`，重跑命令為 `commands-generation.txt`。
  收尾核對入口為 `root-files-generation.log`、`ownership-generation.log`、`containers-generation.log`。
  本輪只使用既有映像與 --rm 容器，沒有新增／刪除映像、prune、操作其他容器、寫入其他工作樹、push、merge 或改寫歷史。

## 2026-10-18：本機完整版 v0.5.0

- 起始工作樹乾淨，main 為 `a16dfc8`，依指定建立 `local-pack`。本輪唯一目前狀態表為 005 §8.4。
- 已讀 AGENTS、000、001、005 §8、012 §9、017 §3.4。路由命中跨平台打包與文件職責，
  載入 retro remake 技能及驗收、輸出、版號與 ZIP 編碼規則。依本輪明確授權使用 `v0.5.0`、`dist/` 及 GPL-3.0-or-later。
- 可攜設定依 AppImage 外部目錄、macOS .app 上層或執行檔目錄選擇；自動尋找只掃指定三處，逐項套用優先順序。
  設定寫回相對路徑，記憶卡保留使用者設定目錄。新增測試不使用原版素材。
- 簽章曾套用後失配改為只警告一次，保留 RAM 與處理器。列首陷阱改用 CPU 冷路徑 `SetBreakGPR`，
  先取消同目標延遲載入；來源仍讀舊值，其他目標的載入仍提交。兩項修正單獨驗收。
- `workplace/local-pack/unit.log` 通過可攜設定、三處尋找、歧義、優先順序、重定位、簽章失配及延遲載入測試。
- AppImage 工具依指定下載到 `workplace/appimage/`，只有下載容器以 bridge 開網路。
  固定版本、授權、下載時間與雜湊見 005 §8.2、`tools/appimage.sha256`、`workplace/local-pack/appimage-tools.log`。
  既有 `psgolem-go-ebiten:1.24-r1` 可執行工具，沒有新建映像。
- 新增 `tools/build_local.sh`、`tools/local_pack.py`；入口與職責登錄於 005 §8.2。
  沿用未修改的 build_dist、build_macos、verify_macos，基礎包留 `dist/v0.5.0/`，本機包留 `dist/v0.5.0-local/`。
  stage 與證據使用既有 workplace。原版不掛入建置容器。文件更新見 docs/play.md。
- 完整回歸、指定 M4 與封包驗收執行中。效能不量測，Wine 啟動依使用者決定延後。
- 全專案回歸通過，容器實際退出碼 0。包含 CPU、1150 筆 GTE 與 17 個 N=1 GPU 語料。
  完整日誌 `regression.log`、摘要 `regression-summary.log`、退出碼 `regression-container-exit.log`，均在本輪證據目錄。
- 首次三平台基礎包與 macOS 四道結構檢查通過，但 AppImage 封裝因 Go 映像缺 `file` 停止。
  既有 osxcross 映像已含 `file`，且工具啟動成功；封裝改用該映像並加前置檢查，沒有建立映像。
  第一次輸出封存於 `workplace/local-pack/attempt-without-file/`，日誌為 `build-missing-file.log`。
  修正後已由乾淨版本與 stage 目錄重跑完整腳本。這是封裝環境問題，既有測試沒有失敗。
- 兩項小修提交 `c5e250c`，可攜設定提交 `0f59d4f`；均使用指定作者與 Codex 共同作者，未推送。
- 第二次 AppImage 已成功封裝，後續守門因 appimagetool 在來源 AppDir 自動建立 `.DirIcon` 連結而停止。
  重新核對打包路由與 005 §8.2，圖示是允許的包內中介資料；只接受 `.DirIcon` 指向同 AppDir 的自製 PNG，其他連結仍拒絕。
  實際 stage 的修正檢查通過，收據 `packaging-guard.log`；第二次產物與原日誌保存於
  `workplace/local-pack/attempt-diricon/`、`build-diricon.log`。再次從乾淨版本與 stage 目錄重跑完整腳本。
- 原生 M4 完成 field 45784、26150980006 個週期，摘要仍為 `6c35c2701228722f`，記憶卡檢查通過。
  記憶卡 SHA-256 `5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36`，
  原生畫面 SHA-256 `02b9bc2db161fa5675d867aeec057ccebd5cb9f6912db0fe46ed159d78c8ddd7`。
  HD2 已開始，尚不宣稱整個 TestM4FirstSave 通過。收據 `workplace/local-pack/m4.log`。
- 第三次乾淨建置完整通過，`tools/build_local.sh v0.5.0` 外層退出碼 0。
  三平台基礎包、macOS 四道結構、AppImage 與三個 local ZIP 均完成，輸出 `dist/v0.5.0-local/`。
  ZIP 的 CRC、頂層目錄、預設設定、空素材目錄、UTF-8、禁止副檔名與 512 KiB 排除通過。
  建置日誌 `build.log`、逐項清單 `packages.json`，四包雜湊與擁有權另見 `package-checksums.log`、`package-ownership.log`。
  Linux 正由完成的 ZIP 解包做 AppImage 啟動驗收，原版三份檔案均唯讀掛載。
