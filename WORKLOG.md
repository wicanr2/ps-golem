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
- Linux AppImage 驗收通過，無 Go 的 Ubuntu 24.04 既有映像以原版唯讀掛載啟動，未提供 BIOS/cue 參數。
  兩次 2000 Update 畫面及設定逐位元相同，設定寫回 `bios/SCPH5500.BIN`、`disc/sotn.cue`，
  HD2、PGXP、xBR 與寬螢幕選項均保留；記憶卡路徑仍是使用者設定目錄。
  實際 AppImage 內容與建置來源執行檔逐位元相同，素材排除通過。
  收據 `linux-smoke.log`、`linux-ldd.log`、`linux-container.json`、`linux-container-exit.log`、`settings-first.json`；
  原生工具命令及驗收容器退出碼均為 0。第一次畫面已目視確認是遊戲標題畫面，截圖不入版控。
- 封包與文件提交 `a6a9195`。目前只剩指定 M4 的 HD2 與 HD2-PGXP 子測試，原容器持續執行，沒有重啟。
- HD2 M4 完成 field 45784，完整摘要、記憶卡與原生畫面均與原生逐位元相同，摘要 `6c35c2701228722f`。
  HD2-PGXP 已開始，尚不宣稱整個 TestM4FirstSave 通過；日誌仍為 `workplace/local-pack/m4.log`。
- 外層 M4 工具連線回傳 143，Docker 核對原容器仍為 running、OOMKilled=false、AutoRemove=true。
  這是工具連線中斷，尚非測試退出；沒有取消或重跑。以 `docker logs --follow` 接續保存同一容器完整日誌到
  `workplace/local-pack/m4-container.log`，既有 `docker wait` 持續追蹤實際退出碼。
  當時狀態收據為 `m4-after-tool143.json`，不把 143 寫成產品測試失敗。
- 完整 TestM4FirstSave 與 HD2、HD2-PGXP 子測試均通過，原容器實際退出碼 0，已自動刪除。
  三模式都完成 field 45784、26150980006 個週期，完整摘要一致，尾碼 `6c35c2701228722f`；
  記憶卡與原生畫面逐位元相同，SHA-256 與前述原生值一致。
  正式日誌 `m4.log` 由同一容器的完整 `m4-container.log` 保存；原工具輸出另存 `m4-tool.log`。
  退出碼為 `m4-container-exit.log`，沒有取消或重跑，也沒有挑選結果。
- 收尾前重新比對知識路由，仍命中跨平台打包、驗收與文件職責；沒有新增決策或擴大範圍。
  005 §8.4 更新為最終驗收表。總索引 `workplace/local-pack/verification.json` 由同目錄 `write-receipt.py` 彙整，
  重跑命令為 `commands.txt`，完整交付清單為 `dist-files.txt`。
  root-owned 檔案、異常 .md 目錄、擁有權與容器清理收據為 `root-files.log`、`ownership-final.log`、`containers-final.log`。
- 本輪只使用既有映像與 --rm 容器；沒有新建映像、prune、rmi、操作其他容器或寫入其他工作樹。
  效能依使用者決定不量測，Windows 啟動依使用者決定延後，macOS 僅做結構驗收。
  原版素材、截圖、記憶卡與封包均不加入 Git，沒有推送、合併、改寫歷史或發布。
- 最終驗收提交為 `96f8915`。收尾核對時，共用 Git 的 main 已另行前進至 `9b4f34d`，
  內容為新增後撤回含素材組裝的 005 §8.5；本輪沒有移動 main 或合併這些提交。
  `git merge-base HEAD main` 仍為指定的 `a16dfc8`，本輪程式、測試與封包均以此為基準。
  005 §8.4 的比較基準改用固定提交，避免把日後 main 的變動套入本輪證據。

## 2026-10-18：Codex 接手《月下》HD 與素材上傳研究

- 基準為 main `6271e767c4b34febcc9e925bc0047e203a63543b`，起始工作樹乾淨。
  路由命中復古跨平台引擎、驗收與文件職責，已載入 reverse-engineer-retro-game-remake 技能、
  project-document-responsibilities 及驗收 reference；已讀 AGENTS、000、001、016、017 與兩 repo 遠端 Issue。
- 目前 HD 狀態沿用 016 §7、017 §7，不重新開啟已驗收階段。真 HD 替換仍只有
  castlevinia_x `docs/hd-assets-evaluation.md` DRAFT；下一步是研究與 READY 規格，不直接進正式實作。
- 交叉核對 `~/cht/ps-golem-hd` 的 `7cab4c1` 與目前 main，檔案內容相同。
  既有完整 M4、原容器退出碼與全專案回歸日誌仍在該工作樹 `workplace/local-pack/`，
  逐檔 SHA-256 符合 verification.json。沒有搬移或刪除此工作樹與封包。
- 正式程式 `go test ./render/hd ./internal/pgxp ./apps/sotn ./machine` 通過，均命中 Go 快取。
  收據 `workplace/hdtex/takeover-tests.log`；本輪沒有重跑完整 M4 或重新打包。
- 舊 `workplace/hdtex/uploads.go` 未追蹤一般 GP0 命令邊界，可能把參數誤認成上傳；
  奇數像素的填充半字也會進入雜湊。保留原檔，改建一次性 Go overlay 探針，
  直接觀測原生 fbWriteStart/fbWriteWord。正式 GPU 檔案唯讀，沒有 production path 變更。
- 探針自檢通過命令邊界、奇數 padding、目的座標不影響鍵、尺寸遮罩與 GP1 中止傳輸。
  來源、overlay、自檢與重跑入口在 `workplace/hdtex/{survey.go,gp0.research.txt,overlay.json,self-check.log,commands.txt}`。
- 正常按鍵從開機到標題 field 4320：3302 次完成上傳、1357 筆去重、18377024 個 16 位元字，
  去重為 7375664 字；420 個 field 有上傳，單場最多 12 次，中止與 pending 均為零。
  摘要尾碼 `9ac6fa7f0133daec`。結果 `survey-title.json` 與 `survey-title.log`。
  這是 VRAM 上傳區塊統計，包含調色盤與影像資料，不能稱為素材張數。
- 完整 M4 上傳研究曾啟動後主動中止，退出碼 143，原日誌 `survey-m4-stopped.log` 保留。
  不視為產品測試失敗；完整數量、Alucard 動畫歸屬與繪圖／CLUT 關聯仍未知。
- 工具 Go 1.24.13，沿用 `psgolem-go-ebiten:1.24-r1`；固定 main、映像 ID、原版輸入、
  腳本、探針與輸出雜湊在 `source-context.txt`、`survey-inputs.sha256`、`evidence.sha256`。
  位址空間為原生 GPU VRAM 1024×512 的 16 位元字，不把 W 當成解碼後貼素寬。
- 結論與文件寫入前重核路由，仍命中驗收與文件職責；目前狀態、量測與限制回填
  castlevinia_x `docs/hd-assets-evaluation.md` §7，沿用既有 progress 入口，沒有新增同義台帳。
- 本輪只用既有映像及有界 --rm 容器，原版唯讀，輸出 UID/GID 為 1000:1000。
  收尾擁有權與本專案容器核對見 `workplace/hdtex/ownership-final.log`、`containers-final.log`。
  沒有新增映像、rmi、prune、操作其他容器、寫入遠端 Issue、commit、push 或發布。

## 2026-10-18：HD 素材包載入器與手把設定

- 延續 main `6271e767c4b34febcc9e925bc0047e203a63543b` 的接手工作，沿用既有研究目錄。
  路由命中規格閘門、驗收及文件職責；已讀 grilling，依使用者既有範圍執行。
  已以主機 gh 核對 ps-golem #1、castlevinia_x #1／#3 全文與狀態，均開啟，未留言或修改。
- 使用者授權比較全彩格式後定案。Go／Pillow 實測支持一般 RGBA 加獨立二值 STP 遮罩；
  特殊 alpha=128 會改變一般預覽，故排除。索引 PNG 保留相同預覽色的不同索引。
  新增 018 DRAFT、019 READY 後才實作載入器；019 本輪僅載入器層達 CONFORMED。
- 完成自製子貼素、CLUT 舊值、bit15、解除回退及 mask／複製反例。
  128 字複製分塊與部分子像素遮罩已確認；overlay 僅研究使用。
  018 的正式來源狀態轉移、傾印與 CLI 契約仍待審查，沒有 GPU 替換接入。
- 追加先前完整 M4 未完成的後續結果：研究探針完成 field 45784，122265 次上傳、
  22394 筆去重、149960068 字、去重 114775014 字；31590 個 field 有上傳，單場最多 58 次。
  中止與 pending 均零，摘要 `6c35c2701228722f`，卡片檢查通過，卡片與原生畫面雜湊沿用既有 M4。
  這次測量使用基準程式加 overlay，不能當成本輪前端或 HD 替換驗收；動畫歸屬與繪圖／CLUT 關聯仍未知。
  首次主動中止的 `survey-m4-stopped.log` 與原 source-context 保留，未改寫歷史。
- `render/hd/texture_pack.go` 實作嚴格 JSON、PNG 索引／全彩／STP、不可變資料、CLUT 查詢優先、
  RGB5 黑色限制、路徑與連結、檔案與常駐資源上限。自製 PNG 測試、HD 回歸與 race 通過。
- 使用者選定系統處理藍芽配對，工具列滑鼠移到上緣或 F1 才顯示；展示的面板原型定版保存於 docs/ui/。
  020 READY 後實作裝置選擇、14 動作重設、死區、即時測試、取消／原子儲存與離線保留。
  未知配置不猜測映射，先按住的輸入與觸發器靜止值不誤綁。
  工具列不改 viewport；面板在 field 邊界暫停，關閉後等按鍵放開才恢復輸入，退出仍可收尾。
- UI 字圖集由固定 Noto Sans CJK TC Sans2.004 與 Pillow 11.3.0 生成，OFL-1.1／著作權附在 repo，
  原始 TTC 與 Debian 授權快照留本機。建置／本機封包腳本加入授權附件，沒有重建正式包。
- 全套 Go 測試、前端與設定 race、真實 BIOS 暫停／恢復、文字範圍與字圖覆蓋均通過。
  最終 Linux 執行檔以正常視窗操作確認 F1、懸停、裝置、死區、儲存、持久設定與關窗。
  Linux／Windows／macOS 編譯及 macOS 四道結構檢查通過。實體手把及 Windows／macOS 真機仍未驗證。
- 驗證過程的 Go 編譯 pid 上限問題，設定 GOMAXPROCS=2 與 -p 2 後乾淨重跑通過。
  GUI 限時腳本與漏點擊各保留原紀錄；改以實際畫面狀態確認操作後，最終腳本通過。
  以上為工具環境／腳本問題，未列為產品缺陷。沒有挑選手把或遊戲結果冒充驗收。
- 工具、來源、輸入與結果總索引 `workplace/hdtex/implementation-verification.json`，
  重跑入口 `implementation-commands.txt`，雜湊 `implementation-evidence.sha256`。
  原版唯讀，輸出 UID/GID 1000:1000，root-owned／異常 .md 目錄與容器清理另有本輪收據。
  所有本輪容器已結束並刪除，沒有新增映像、prune、rmi 或操作其他專案容器。
- 開發入口 `bin/psgolem` 已更新，castlevinia_x `local-full/v0.5.0/` 沒有重新組裝。
  目前狀態表更新 castlevinia_x `docs/hd-assets-evaluation.md` §8，progress 連回該表。
  沒有提交、推送、改寫歷史、寫入遠端 Issue、發布或新增含原版資料的版控檔。

- 收尾審閱補齊 Delete／Backspace 清除指派，讓衝突檢查下仍能交換既有按鍵；
  原始按鈕／軸值顯示在現有即時測試區。實際擷取發現兩行高度不足，依字圖行高修正，
  新增真實墨跡檢查後重跑前端、實際 BIOS、交換按鍵、race 與正常視窗操作，均通過。
- 素材包 schema 另補禁止欄位空字串與大小寫別名負例，載入器拒絕這些繞過方式；
  HD 與 HD race 重跑通過。全套回歸的 machine 測試本輪重跑為 285.092 秒，亦通過。
  最終 Linux／Windows 重新編譯，macOS 在最後 UI 修正後另重建並核對建置前後來源雜湊。

## 2026-10-18：接入 HD 材質替換與雙擊完整版

- 使用者確認保存素材路徑、下次自動啟用，並要求完整版雙擊啟動。沿用 grilling 已展示的範圍，原版衍生素材仍只留本機。
- 路由命中跨平台引擎、READY 閘門、驗收、文件職責及新版交付，讀取對應技能與入口；兩個 HD Issue 以主機 gh 重新核對，仍開啟，未修改。
- 固定種子 18026 的 720 組來源／複製探針通過。018 補齊來源狀態轉移、快取副本、子貼素、傾印容量與 CLI／保存路徑後先標 READY，才接正式程式。
- 材質來源包含完整上傳鍵、字內偏移與原字；GPU 寫回解除來源，128 字複製用均一工作格保守轉移；快取保留舊來源。傾印以引用數回收未使用原始字，避免上傳物件累積突破容量。
- 首批真 PNG／manifest 與 GPU 命令測試、CLUT／STP 混色、翻轉／E2、mask／copy、未命中不變、傾印往返與可攜路徑保存通過。正式標題傾印完成 3302 次上傳、1357 鍵、51 使用組合、49 可載入條目，摘要仍為 9ac6fa7f0133daec。
- 本機程序化診斷素材在 castlevinia_x/hd-assets/research-sample，只用於子貼素測試，不當成定版美術或裝進完整版。完整 M4 與同狀態替換／關閉對照執行中。
- 新版採 v.0.6.0-20261018，乾淨封包依路由在 dist-all/<版本>/patch；含素材依本專案 AGENTS 例外留 castlevinia_x/local-full。舊版與舊工作樹不搬、不覆寫。打包新增容器內 tools/dist_pack.py，Linux 本機 ZIP 附原生雙擊啟動檔。
- 驗證環境修正：研究 Go 探針加 build ignore，避免 ./... 收入研究 C 驅動；保留原失敗日誌後重跑。新版建置原缺 AppImage 工具，從舊工作樹唯讀複製固定快照，雜湊核對通過，沒有重建或下載映像。

- 同狀態標題四次重播通過：原生、HD2、研究替換與關閉均為摘要 9ac6fa7f0133daec，RAM／原生 VRAM 相同，關閉回復原 HD2。研究替換實際命中 16236544 次，改變 25360 個輸出像素；真原生與 HD 圖均保留本機，已目視確認。
- 正式傾印 1357 個鍵逐一對照原生探針，次數與首次／最近 field 全部相同。全專案乾淨重跑及四個套件 race 通過；原研究套件誤收的失敗日誌保留為 texture-regression-research-package.log。
- 新版七個乾淨產物、macOS 四道結構、ZIP CRC／UTF-8／設定／素材排除、AppImage 版本／缺庫／payload 均通過。三平台含素材版本組裝到 local-full/v.0.6.0-20261018，輸入與複製後素材雜湊一致。沒有覆寫舊包。
- 在無 Go 的執行映像從其他 cwd 零參數開啟封包及真正完整版，正常 F1、儲存／取消、關窗與設定重啟通過；研究包只掛驗證副本，正式完整版維持自動手把且不含診斷圖。
- 正式完整版再以預設四個畫面選項從 GUI 開機到 field 4320 標題，已目視確認。Windows／macOS 僅編譯、內容及 macOS 結構，實體手把及真機聲音仍未驗，不擴大完成聲明。
- 完整 M4 以空白卡、原有正常按鍵、HD2＋PGXP＋研究素材完成 field 45784，摘要 6c35c2701228722f，卡片及原生畫面雜湊完全符合既有原生收據；122265 次上傳且確有替換命中。018 限此範圍標 CONFORMED，最終美術與全遊戲素材覆蓋仍未完成。
- 收尾路由核對仍命中驗收／文件；另依較早規格回填入口修正 001 的 HD 規格待寫斷言與 005 的素材例外指標。沒有重開已完成的 016／017 工作。
- 總索引 workplace/hdtex/texture-integration-verification.json，來源／輸入／產物雜湊 texture-integration-evidence.sha256，重跑入口 texture-integration-commands.txt。原版唯讀，輸出與啟動檔 UID/GID 1000:1000，無 root-owned 或錯建 .md 目錄。本輪容器結束後均自動刪除，沒有新增映像、prune、rmi、遠端 Issue 寫入、commit、push、tag 或發布。


### 2026-10-18：重新開啟速度與正常玩家路線驗證（進行中）

使用者要求先確認速度與可玩性，再製作 HD 美術。主機 gh 憑證通過，讀取 ps-golem #4 全文，舊 Issue 仍標延後；未寫入遠端。唯一目前狀態表及素材 progress 已改成進行中。以下均尚未構成速度合格或新版本發行聲明。

- 沿用正常玩家路線路由、012 即時時脈與 016 規格。016 §8 先補 READY 的逐像素等價、緩衝所有權及批次效能契約，再實作。
- HD 新增 ScreenInto；RGB5 同公式查表，24bpp、環繞、寬螢幕及黑邊維持舊輸出。保留舊版函式於 screen_reuse_test.go 作獨立 oracle。倍率 1／2／4／8、色深、尺寸變化、側邊消失皆通過；相容尺寸 0 配置。微基準舊版 6.1–7.3 ms，新版 2.1–3.9 ms，受共享主機負載限制，不當作全遊戲加速比例。
- 前端 useFrame 持鎖上傳，防止後備緩衝過早重用。慢視窗可合併畫面要求，全部 field、GPU 命令、音訊與輸入仍完整執行；首次及指定終點必須發布。實際 BIOS 的慢消費者摘要、暫停／恢復與競態檢查通過。
- -stats 區分視窗 FPS、field_hz、週期速度比、畫面耗時與發布數，並新增音訊缺樣／丟棄的立體聲取樣組數。
- 正常按鍵至 field 12000 的逐場 HD2＋PGXP 輸出探針已完成，原生摘要尾碼 54daa77ae7f572ca，輸出 SHA-256 526276d437482edbe04e12be1178d3f980947b261c182de6b67d1bbcfaeb466d。path-after.json／log／pprof 是診斷，不是全遊戲或速度通過。最後部分與另一個視窗取樣重疊，不能作無干擾速度 A/B。
- 取樣中 CPU／週邊累計約 61%，HD 取樣、繪圖與轉換佔其餘主要成本。4096 字批次可能延遲短命令串到場末才執行；改成 256 字以提早並行。PGXP 邊界、HD 素材、寬螢幕與 race 通過。兩種批次及 PGO 的開機樣本原生摘要與輸出雜湊相同，但速度變動仍大。
- Go PGO 僅建立比較版本，未放入正式建置。依 https://go.dev/doc/pgo 核對使用方式；研究來源為本次正常路線 CPU profile。
- 初始 ALSA null 沒有播放節拍，Oto 繁忙讀取佔大量 CPU，歸類為驗證環境問題。改用既有 eob-audio-capture:20261009-r3 的 PulseAudio null sink，確認播放有節拍，保存 gui-paced.wav。Xvfb 軟體 OpenGL 的 field 12000 視窗重播完成但未達即時速度，不能聲稱可玩驗收通過。
- 一次靜音基準達 180 秒界限，未完成且 CPU profile 為空；保留失敗 log，不當作有效完整取樣。第一次規格寫入命令引號錯誤已立即修正，正式程式未受該命令影響。
- Mesa Zink 可辨識實體 Intel MTL GPU 且讀回測試色正確，Xvfb 缺少 DRI3，視窗呈現仍黑；不能只憑 renderer 字串宣稱硬體驗證成功。依 https://docs.mesa3d.org/envvars.html 核對後仍未解決，轉向 Weston／Xwayland 的隔離視窗。
- 沿用 wine-verify.Dockerfile 建立明確替代 noble-r1 的 noble-r2，加入 Weston、Xwayland、PulseAudio、ALSA plugin、ffmpeg 及 mesa-utils，鎖定既有 Ubuntu 基底摘要並保存套件版本清單。建置與 GPU 視窗驗證尚在進行。

本輪所有證據、研究程式與比較執行檔均在忽略的 workplace/hdtex/perf/。原版仍唯讀、不外傳。未修改既有 v.0.6.0-20261018 封包、tag 或 Release；尚未製作定版 HD 美術。容器清理與最終來源／測試收據待本輪結束補記。


#### 同日續記：實體 GPU 與視窗生命週期

- noble-r2 建置完成，OCI 映像摘要 89335cd2ce73ef33bc344940f54187eaf4ace095b8b3385e967e48b214857492；Weston 13.0.0、Xwayland 23.2.6、Mesa 25.2.8、PulseAudio 16.1。完整套件清單在 perf/toolchain-r2-packages.txt。
- 首次 GPU probe 缺少容器內 /tmp/.X11-unix，修正初始化後使用同一映像重跑成功。硬體 renderer 為 Intel MTL，direct rendering 為 Yes；測試色讀回 51,102,153,255，實際 X 視窗像素 0x336699。證據 hardware-probe2/。
- hardware-fast/ 與 hardware-batch16/ 均完成正常按鍵至 field 12000，畫面為序章王座廳。16 字批次目前僅為研究 overlay，正式常數仍為 256。慢段約從 0.4 倍改善到 0.8–0.9 倍，仍有低速區段，不能聲稱整條路線速度通過；共享主機與核心時脈變動限制 A/B 因果判斷。
- 兩次終點 PNG 不同；檢查發現 Game.Update 可以在最後一次 Draw 前結束。已加入實際 Draw 確認及 final_field／final_frame／shown_frame 診斷；不先把影像差異判成批次渲染回歸，待同狀態重跑。TestFinishWaitsForActualDraw 通過。
- 驗收模式原本每次 Draw 都複製整張畫面；改成只在最後 Draw 擷取一次，使速度量測更接近一般啟動的負載。
- 啟動時可見約 1.6 秒聲音被丟棄。模擬在 RunGame 之前開始，音訊與視窗的就緒順序不一致；改成首次 Update 起跑，startOnce 管理唯一生命週期，尚未開始就關窗也能收尾。
- 真 BIOS 暫停／恢復、慢消費者完整 field 與摘要、提早關窗及前景所有權 race 通過，startup-race-tests.log。整組前端第一次因一個舊測試直接 go e.run 而重複關閉 done；改用 e.start 後重跑，失敗 log 保留為 lifecycle-old-test-entry-failure.log。
- PGO 的硬體比較 hardware-pgo16/ 在 Weston 的額外測試視窗切換時失敗，無有效速度資料；以同一工具鏈省略重複測試視窗，hardware-pgo16b/ 完成 field 12000，最後 Draw 確認相符。仍有低速區段，PGO 未放進正式建置。
- 目前正建置含起跑與單次終點擷取修正的非 PGO／16 字比較版本。未改 CPU 指令、週期模型或正式版本；HD 美術仍依使用者順序等待速度閘門。

#### 同日續記：等價取指與正常路線

- 修正前述啟動診斷：音訊後端 ready 且至少一次 Draw 後才起跑，仍會有啟動佇列整理。新增 produced／requested／nonzeroDropped 後，短測 55748 組丟棄中只有 26 組非零，約 0.59 毫秒；不能把整段丟棄數稱為有聲內容遺失。ALSA null 與 PulseAudio 冷啟動的限制仍保留。
- 在單一 P 核、同程序交錯比較真實開機後 field 7200 至 7800，KSEG0 快取 RAM 取指捷徑 CPU 時間省約 9% 至 19%；六次終態均為 800e0aa7bce449bf。先追加 008 §6 READY，再納入正式 CPU。沒有跳過指令或修改週期，其他取指路徑不動。
- 保留修改前 cpu.step 為獨立 oracle，4096 個快取／鏡像／區段／未對齊／IRQ 案例完整 CPU 狀態及週期相同。17 個 GPU 語料逐 field 全 VRAM 位元相符；1150 筆 GTE、公開 CPU 與 access-time 均通過，既有 SPUCNT/32 差異維持原台帳。HD／前端 race 通過。
- 短批次本次候選採 16 字，較早 256 字與研究 overlay 的歷史記錄保留。候選已從空白卡以完整預設 HD2／PGXP／xBR／寬螢幕在 Intel MTL GPU 視窗跑正常按鍵，field 1800 至 7800 維持約 1 倍速且零缺樣。完整 M4 尚在執行，不先宣稱速度通過。
- 本機來源與量測在 workplace/hdtex/perf/；主要日誌 candidate-tests.log、candidate-oracles.log、candidate-race.log、hardware-candidate-m4/。原生狀態完整 M4 另由既有 TestTextureM4PreservesNative 驗證，寬螢幕模式不作原生摘要證據。

- field 23400 至 24600 出現約 0.97 倍速及短暫缺樣，與本輪 macOS 建置重疊。檢查確認 build_macos.sh 未沿用 PSGOLEM_CPUSET，雖有 2 CPU 配額仍可佔用驗收核心。試圖只調整已核實的本輪容器時它已正常結束，未更動任何容器。補上可選 CPUSET，後續驗收不與建置並行；此輪重疊區段如實保留，不包裝成乾淨速度基準。

- field 32400 的 0.78 倍速未達本輪速度目標，Docker cpu.stat 顯示 nr_throttled=0。追加城堡進場局部 profile，正常開機全路線仍完整推進。初版研究探針沿用一般 speed-probe 的每段摘要，與寬螢幕契約不符，主動停止後改成寬螢幕不呼叫 Digest；初版日誌保留，不作任何原生狀態證據。正式程式沒有這項研究輸出。

- 正式候選完成 GUI field 45784，最後發布與實際 Draw 相符；記憶卡 SHA-256 仍為 5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36。城堡後段最低約 0.568 倍，速度不合格，沒有宣稱可通關或已完成 HD 美術。
- GUI 程式與 paced.sh 正常結束，但執行中的 weston.sh 曾被研究編輯改變尾端位置，外層讀到 EOF 而回傳 2。屬驗證腳本錯誤，原輸出保留；後續使用不可修改的 weston-stable.sh 快照，不當成遊戲退出失敗。
- v.0.6.1-20261018 七個乾淨候選產物及三平台本機組裝完成，舊版未改；尚未取代目前入口。真正 Linux 完整版從其他 cwd 零參數啟動、F1／取消、正常選單辨識首存 ALUCARD 城之入口、讀取還原到存檔室、方向鍵／跳躍及正常關窗均通過。圖片與卡片留 perf/package-zeroargs/，沒有資料注入或 direct-entry。
- 自動核准審查拒絕將新的有界畫面請求標 READY，理由為尚未由使用者確認的實質架構／呈現排程變更。正式 spec 與程式未套用。只在忽略的 perf/ 建立 DRAFT overlay 原型，一個未完成畫面、最多兩場在前、原生同步及暫停／終點／退出排空，供具體比較。
- 原型初次碰到尚未執行任何場就暫停的零值測試；改成有實際場後才強制發布暫停畫面。原失敗保留，重跑 HD／前端 race 及獨立快照命令位置／關閉排空測試通過，async-prototype-tests-v2.log。這不代表原型已批准或速度通過。
- 原型第一輪速度量測與其他重型驗證重疊，P 核讀值約 1.2–1.3 GHz，主機 load 約 21.5、Docker 沒有節流。主動停止、保留日誌，待本輪其他重型工作結束後依序量測。不要以隔離不同核心就宣稱排除共享功率與排程干擾。

#### 同日續記：正式等價驗收與排程提案

- 正式 TestTextureM4PreservesNative 完成，1377.45 秒，退出碼 0；field 45784、26150980006 週期、摘要 6c35c2701228722f，卡片及原生畫面符合既有雜湊。研究素材確有命中。舊／新 HD2-PGXP 終點 PNG 逐位元相同，SHA-256 a1e60cc76521bfe6217231f47fdf3b48f3014626a7e65a44d761ffff1c16bb9a。008 §6 只將等價取指範圍標 CONFORMED，速度仍未通過。
- 候選 AppImage 無 Go／缺庫／版本、實際 payload、素材排除及固定 field 2000 重啟畫面相同皆通過。verify_local.sh 原以 Update 次數比較，改用固定 field 避免呈現頻率改變擷取狀態。真正完整版的正常讀檔／移動後記憶卡與讀前相同。v.0.6.1 仍是候選，沒有取代 v.0.6.0。
- 城堡取樣完成正常開機至 field 34200，只在 field 30000 後擷取 CPU profile；widescreen 模式沒有產生原生摘要。castle-profile-v2-top.txt 顯示機器約 45%、影子主要繪圖與轉換約其餘半數，總取樣約 1.09 個 CPU。現有畫面發布會等待影子，這是來源可見機制；實際速度收益仍未知，不將 profile 當成排程修法已有效。
- 完成本輪其他重型工作後，先獨立量測正式候選，再比較研究原型。兩輪仍未達原速；外部負載從約 20 升到 48.85，而實測拓樸只有 14 個邏輯核心。0–3 為兩個實體 P 核的 SMT，4–11 為 E 核，12–13 為較低時脈核心；0–5 並非全為 P 核。Docker nr_throttled 皆為 0。不得把這些資料包裝成乾淨的 A/B 因果證據，量測已主動停止，原日誌保留。
- 一場緩衝原型另補閒置 HD 場的完成通知，讓視窗不取畫面時也限制命令積累。原生仍同步；暫停、終點與停止排空。async-final-boundary.log、async-final-fence.log 的競態、真 BIOS 與獨立快照順序／關閉通知均通過。較早效能樣本尚未包含閒置通知，不當成最終原型速度證據。
- 原型仍只在忽略的 overlay 與研究執行檔。可審閱方案為 [async-proposal.txt](workplace/hdtex/perf/async-proposal.txt)，清楚列出一場呈現緩衝、已驗範圍、未知速度與審查拒絕原因。正式 016 沒有新增 READY 排程規格，正式包沒有原型方法。
- 最後全套 Go 回歸遇到 machine 套件的 10 分鐘總時限，唯一仍執行的 TestDeferredSyncEquivalent 已跑 3 分 52 秒，堆疊為 runnable，沒有發現狀態差異斷言。保留 final-regression.log 與實際退出碼 1；重查正常玩家路徑／驗證路由後，將有限的同步對照以相同來源、語料及資源單獨重跑，未更改產品時脈或測試期望。

- 單獨 TestDeferredSyncEquivalent 完成，337.72 秒、退出碼 0，全部 13 份語料的逐條／延後同步摘要相同。final-sync-check.log、final-sync-check-exit.json。原整組逾時日誌與退出碼 1 保留，不改寫為原命令成功。
- 正式 Go 來源 gofmt 檢查與兩個 repo 的 git diff --check 通過。原始資料唯讀；抽查來源、候選與真正完整版 UID/GID 1000:1000，root-owned 與錯建 .md 目錄均為零。本輪有名稱的容器已自動刪除，依本專案硬規則保留既有映像，未 rmi／prune 或操作其他專案容器。
- 本輪最終入口為 workplace/hdtex/perf/performance-verification.json，雜湊 performance-evidence.sha256，來源快照 performance-source-snapshot.tar.gz 與 material-performance-source-snapshot.tar.gz。原型獨立保留在同目錄，不進正式來源或發行包。
- 目前交付入口仍為 v.0.6.0-20261018；v.0.6.1-20261018 僅為已驗功能的候選。速度閘門未通過，HD 美術尚未開始。新的呈現排程需先取得使用者決定，再做正常完整路線及較低負載下的速度驗證。沒有遠端 Issue 寫入、commit、push、tag、Release 或原版素材外傳。

- 末次審閱發現既有慢消費者測試只走原生回退，不能單憑它證明新 HD 回呼。補上僅在 overlay 使用的真 BIOS／HD2／PGXP 測試：無讀者與慢讀者都完整跑至 field 120，原生摘要相同，最後 HD 圖符合同步讀回，發布數分別 2 與 117；race 通過。另確認實際 HD 路徑暫停在 field 4、恢復後主動停止在 field 7，原生摘要不變，暫停畫面為最新。收據 async-real-hd-check.log、async-hd-pause-stop.log。
- 初次總收據及來源快照保留為初始 checkpoint。補測後的正式入口仍為 performance-verification.json／performance-evidence.sha256，最終來源快照改用 performance-source-snapshot-v2.tar.gz 與 material-performance-source-snapshot-v2.tar.gz。新測試與原型仍不在 production path；完整原型 M4 與速度不標完成。

## 2026-10-18 一場有界排程獲授權

使用者已查看原型並明確授權繼續一場有界 HD 呈現排程。016 §8.1 已標 READY，準備把已測原型轉入正式程式。
這是先前架構審查拒絕之後的新授權；不再請使用者重複決定。速度閘門仍未通過，HD 美術尚未開工。
依使用者要求嘗試在 Codex 全域設定加入限定專案的授權附加政策，被自動審查拒絕，設定未變更。
授權保存於本規格與工作歷程；不變更安全審查邊界，也不影響其他專案。

### 正式一場緩衝：邊界通過，速度診斷不合格

正式 render/hd 與 frontend/ebiten 全套 race 通過，分別 56.080 秒與 63.290 秒。
包含真 BIOS、HD2／PGXP 的慢讀者、無讀者、暫停／恢復／停止及最後畫面一致。
正常 GUI 從空白卡重播，使用 Intel MTL 硬體 OpenGL、Pulse 有節拍音訊、HD2＋PGXP＋xBR＋寬螢幕。
標誌動畫與序章多段速度不合格，最低 600-field 區段 0.276 倍速；在 field 7456 正常關窗，轉入窄剖析。
此輪不是完整 M4，也不能宣稱新排程已改善整體速度。外部負載約 30–40，容器無 CPU 節流。
證據入口：workplace/hdtex/perf/async-production-verification.json。既有同步候選及完整 M4 收據不覆寫。

### 分離計時與區塊寫入

新增 core_avg 與 hd_wait_avg，把原生模擬與影子完成等待分開。補充勘誤：CPU 剖析的 futex 取樣少，
不能用來推定牆鐘等待少。正常場景曾量到原生模擬 20–36 ms，部分區段另等 HD 1–6.5 ms。
同一正式二進位在主機負載回落時，標誌段又能達到 1 倍速，故不把不同條件的數字當成因果 A/B。
分離計時重播在 field 11216 正常關窗，轉入已定位熱路徑的修正，不算完整 M4。

016 §8.2 保持均色區塊逐位元輸出，以直接寫入及逐列複製省掉重複迴圈。
完整 VRAM oracle、render/hd race 與真 BIOS frontend race 通過，分別 52.224 秒與 33.365 秒。
HD2 put 微基準：原函式 14.24–15.20 ns/op，新函式 11.90–13.78 ns/op；不外推成整體遊戲加速百分比。
證據：put-block-race.log、put-block-benchmark.log。接著只在研究建置試用 Go PGO，正式建置契約尚未改動。

再次依已開啟的官方 Configuration Reference 核對 auto_review.extra_policy，改成只掛單一設定檔後重試，
自動審查仍拒絕全域設定寫入，設定未變更。授權已加入本專案 AGENTS.md 作為後續接手入口，不覆寫安全政策。

### 一場緩衝完整正常 GUI 首存

研究 PGO 編譯完成正常光碟重播至 field 45784，actual final Draw 確認完成，
發布與顯示 frame 均為 29658。記憶卡及最後 GUI PNG 與同步基準逐位元相同。
記憶卡 SHA-256：5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36。
GUI PNG SHA-256：cee5284aa6adfad2b36dc69ad0245c21169a66a407b289770aa971c8528232b3。
此輪開啟寬螢幕，沒有產生原生摘要。這證明該編譯的一場緩衝首存與終點呈現；
不代替後續原生中性色調變變更的回歸，也不宣稱完整 CPU/RAM 的寬螢幕 parity。

76 個 600-field 統計區段最低 0.396 倍速，速度仍未過關，PGO 不納入正式建置。
實際紀錄在 async-pgo-m4/，總入口 async-production-verification.json。
同時省略精靈中性色的恆等運算，007 §9／016 §8.3 已 READY，正跑完整色值、GPU oracle 及 race。
下一個窄研究是正常重播的 guest PC 分布，先確認 CPU 忙迴圈，不先猜補加速機制。

### 2026-10-18：中性色回歸與素材拆解分支

中性色精靈的原生 GPU、HD 與真 BIOS 前端 race 通過。公開 GPU oracle 維持既有差異台帳，
不改稱零差異。完整原生／HD1 VRAM 對照通過 17 個語料、每例 351 或 352 場，294.68 秒。
最初缺 PSGOLEM_BIOS 的命令為 skip；收據 neutral-shadow-oracle.log 保留，
真正通過的是 neutral-shadow-oracle-with-bios.log。

正常玩家腳本的原生指令取樣完成至 field 20400，519.5 秒；窗口與取樣 PC 保存在
perf/guest-pc-probe.json。此探針沒有開 HD 或寬螢幕，也沒有跳過指令；目前未實作忙等待批次加速。
一場有界呈現的正常 GUI 完整 M4 仍以 async-pgo-m4 收據為準；PGO 只是研究構建，
最低量測速度 0.396，速度未通過。新中性色修改尚未以完整 GUI 重播，不借用較早構建的完成聲明。

使用者另要求原版及 HD 的怪物、材質、地圖總覽，已開素材專案 docs/assets-catalog.md，
並保留先速度後 HD 美術的閘門。工具與圖像都在素材專案；不接入通用 CD／GPU 的讀取路徑。

### 2026-10-18：原版與 HD 素材目錄驗證

素材專案新增 tools/extract_assets.py、extract_assets.sh、verify_assets.py，
文件入口 docs/assets-catalog.md 已掛入 README 及唯一目前狀態表。
原先素材專案沒有研究工作區，依既有路由建立唯一 workplace/，輸出在 workplace/assets/；
原版唯讀，全部原版及衍生物仍被 Git 忽略，沒有加入乾淨包。

原版光碟 SHA-256 de88bfeafc2b0277c5ad699ae3c0589fab77ab9ddbee25b019afecdd0a456a69。
295 個目錄項已抽出。第一輪 CD1.DA 越界來自 ISO 音軌占位項，依 cue INDEX 01 保存完整 CDDA，
不補零，保留 ISO 與 cue 兩組定位。兩次中間總覽完成後發現轉場索引、共用材質及字型缺口，
已修正；另一次研究重建主動停止，退出碼 143，沒有當成產品測試失敗。
最終解碼 119.680 秒，退出碼 0；source 快照與雜湊在素材 workplace/assets/。

像素驗證退出碼 0：147 張圖鑑的 1580544 個像素連同 STP 可還原來源；
60 組材質共 478 頁、15663104 位元組可重打包；57 組靜態前景地圖，895 個房間記錄
含 134 個轉場，無解析錯誤或未知頁碼。全戰鬥動畫及動態效果不在已驗範圍。
已檢視原版三類總覽及 HD 進度圖，修正中文字型、HD 標籤間距與小型地圖預覽比例。
最後僅重建總覽，PNG 完整解碼及文字寬度檢查通過，像素資料未變。

HD 已建立同編號、來源雜湊與 RGBA／黑白 STP 的製作索引，錯誤來源、越界路徑、
alpha=128、非黑白遮罩及尺寸錯誤拒絕。測試色塊只在臨時目錄，已移除。
HD 正式美術仍為 0，沒有把灰階原版參考或最近鄰放大當作美術完成。
素材目錄與執行器 manifest 分開；尚未逐項對應 GPU 上傳鍵，不預設裝入完整版。

暖快取 256 KiB 樣本交錯測試的中位數為扇區視圖 1.211 ms、拆檔 0.098 ms。
這只說明離線工具讀取，不代表遊戲加速；正式 CD／CPU／GPU 路徑未因此改動。
目前完整版仍 v.0.6.0-20261018，速度未通過、HD 美術未開工。

本輪收尾：兩個 repo 的 git diff --check 通過。Docker 內掃描 36895 個路徑，
root-owned 與異常 .md 目錄皆為零；輸入、工具與總覽抽查 UID/GID 1000:1000。
本輪素材、GPU 回歸與指令觀察容器均已自動移除，沒有 prune／rmi 或操作其他專案容器。
沒有 commit、push、Issue 寫入、tag、Release 或原版素材外傳。全域自動審查設定保持未變。

### 2026-10-18：使用者確認美術方向及有界 RAM 原型

使用者表示同意美術，開始以原版造型／配色作 HD 樣本；正式完整版仍保留速度閘門。
內建 image_gen 僅收到文字描述，沒有上傳原版影像。城堡石牆草稿保存於
素材專案 hd-assets/drafts/stone-wall-v1.png，提示詞、雜湊與 1254×1254 尺寸見同名 JSON。
這是材質研究圖，未標為完整素材、未裝入遊戲，尚未加入正式 HD catalog。

效能研究入口：[RAM 計數迴圈提案](workplace/hdtex/perf/poll-loop-proposal.md)，狀態 DRAFT。
只用 Go overlay；正式 CPU／機器／PGXP 檔案未改。128 組 CPU／RAM／PGXP 完整終態對照首輪通過。
拒絕測試有一項把合法 LUI 位址變更當成非法，已改測 opcode 破壞；原失敗 poll-unit.log 保留。
這是測試規格錯誤，沒有為通過測試收窄可用 RAM 位址或改變 native VM。

### RAM 原型邊界及石牆試用圖

poll-unit-v2 的 race、128 組完整狀態與拒絕條件通過；poll-machine-boundary 的完整 Machine 對照通過。
新 poll-irq-v2 補測未知 RAM／PGXP 觀測器、不同 RAM backing 與排定 VBlank 中斷，均通過。
原 poll-irq.log 為研究測試欄位 iStat 大小寫錯誤造成編譯失敗，保留原收據；沒有修改產品以迎合測試。
完整正常 M4 仍在執行，原型維持 DRAFT／overlay，沒有轉入正式 CPU。

素材工具 tools/build_hd_stone_sample.py 已掛入素材 docs/assets-catalog.md。NO3 第 0 頁第 37 塊，
來源區塊與正常 M4 field 31495 的 32×128 字上傳完全匹配，色盤 1 的靜態關聯為強證據。
索引試用圖保留全部 512 組來源色盤的透明及 STP，但共用色盤限制令細節不足，保留技術試驗。
精確 CLUT 的全彩試用包保留原色值、透明及 STP；只在該 CLUT 雜湊命中時替換。
原先整片石牆縮成單塊會改變磚塊比例，改取已選生成圖中的單塊石材表面，並保留原生邊界與深四色階。
原圖不修改、不上傳。對照圖與全部提示詞／輸入輸出雜湊留 hd-assets/drafts/。
兩包通過正式 Go LoadTexturePack，每包 1 條；實際繪圖 CLUT、接縫及遊戲內替換未驗，未裝入完整版。
Pillow 首次命令誤用 image entrypoint 造成 126，指定 Python3 entrypoint 後乾淨重跑成功。
正式 HD catalog 仍為 0，這些都是草稿，沒有 commit／push／Issue 寫入。

### RAM 原型第一次正常 M4：未命中，回到 DRAFT 勘誤

1417.53 秒完成 field 45784，native Digest、卡片及原生畫面相同；但研究 batches/cycles 為 0。
這是沒有啟用加速路徑的完整回歸，不能寫成原型有效。原始 poll-m4.log 保留。
根因為第一個 BNE 的比較端錯限 zero，實際為另一個固定 GPR；輪詢門檻又是不同 GPR。
已先修正 poll-loop-proposal.md 的 v2 DRAFT 契約，再改 overlay，沒有改正式 CPU。
新增任意終止值及模 2^32 邊界的完整狀態對照，短正常開機確認命中後才重跑 M4。
原型 v1 的 CPU、測試、overlay 副本已保存；尚未執行的 GUI 12000 設定留作 prepared 紀錄。

素材查詢的研究入口：[單筆快取 DRAFT](workplace/hdtex/perf/texture-memo-proposal.md)。
正常 M4 計到 9857445616 次取樣；有效來源時最多重做兩個唯讀 map 查詢，先做有界單筆 memo 的 oracle 與微基準。
只在 overlay，正式 GPU 尚未改動；不改來源、有效 CLUT、統計或傾印時機，不先聲明整體加速。

### RAM v2 短正常路線與素材查詢原型

272 組 CPU／RAM／PGXP 完整狀態與拒絕條件、機器事件及排定 IRQ 的 race 通過。
CPU 套件 130.640 秒，machine 41.763 秒；不是整組時間相加。
正常空白卡開機 field 1454：逐指令基準、v2 原生與 HD2／PGXP 的 Digest 完全相同。
後兩者命中 211 批、446368 週期，來源及結果在 perf/poll-v2-short-verification.json。
elapsed 僅作診斷，不把不同資源／負載下的數字當作加速百分比。有效路徑完整 M4 與 GUI 仍未驗。

單筆素材查詢快取 DRAFT 的完整 render/hd race 通過 38.814 秒，
含固定 seed 18018 直接查詢 oracle、缺項、color／indexed 優先、CLUT 切換與傾印／統計計數。
來源與原 raw cache 仍走既有邏輯，只有 pack 的唯讀結果可被記住。正式 GPU 尚未改動。
微基準另在沒有本輪重型工作的容器執行，不外推成遊戲速度閘門。

### 素材查詢單筆快取轉入正式程式

018 §4.1 先寫 READY 契約，才將與已驗證原型相同的實作及測試轉入 render/hd。
CPU v2 仍僅 overlay。快取只有一筆，含 nil，保留原來源、有效 CLUT、統計與傾印時機。
indexed 查詢 142.6～163.5 ns → 21.20～24.51 ns；color 62.26～70.16 ns → 15.49～16.91 ns；
miss 44.60～59.54 ns → 18.80～23.29 ns，均 0 alloc。這些不是整體遊戲倍速。
交替來源的 memo 38.22～50.52 ns、原 43.00～52.89 ns，範圍交疊，不宣稱精確因果百分比。
研究完整 HD race 及 120000 次直接 oracle 通過，正式來源仍需完整 M4／GUI 驗收。

完整 M4 的新研究 overlay 入口為 perf/stone-m4-overlay.json。CPU v2 加正式 018 §4.1 快取，
附帶只篩選既知 NO3 上傳鍵的 CLUT 雜湊 observer：stone-runtime-tap.txt、stone-m4-tests.txt。
只記有效 depth4／CLUT 雜湊與 resolver query field，不保存原始色盤或圖像，最多 256 組；
命中 memo 不增加 query count，不能當作像素取樣次數。這個 observer 不進正式 GPU。
M4 證據目錄 poll-v2-m4；若加速批次為 0 直接失敗，避免再次把未命中當有效驗收。
輸入仍為正常空白卡、原按鍵腳本、HD2／PGXP／既有程序化研究包；新美術未裝入。

### 有效完整 M4 通過；真實 GUI 仍低速

CPU v2＋正式素材 memo＋研究 CLUT observer，正常空白卡／原按鍵至 field 45784，
1173.55 秒，6870854 批、14276539524 週期。CPU 摘要 6c35c2701228722f，
卡片、原生像素及最後 HD PNG 與既有基準完全相同，取樣統計亦相同。
HD PNG SHA-256 a1e60cc76521bfe6217231f47fdf3b48f3014626a7e65a44d761ffff1c16bb9a。
這是功能與同狀態證據；測試 elapsed 不當作實際播放速度。

真正 Intel MTL／HD2／PGXP／xBR／WS／節拍音訊的 GUI 早段仍低速，最低 0.260，
field 5669 正常關窗轉剖析。final_frame 3543、shown_frame 3541、final_drawn=false；
這是手動提前關窗，不聲稱完成原定 12000 場或最後 Draw 驗收。
主機 load 約 36～40，CPU pressure some 約 50～60%，容器 full 約 10～15%，
核心頻率有約 1～3.4 GHz 變動，quota throttled=0。不用它與舊 PGO／不同負載作因果 A/B。
新剖析入口 perf/poll-profile-overlay.json、poll-profile-main.txt，
實際硬體 GUI 的 CPU／block profile 在 poll-profile-v2-2400/，只作診斷。

NO3 已知上傳的三組有效 CLUT 對應 NO3 0、33／74、F_GAME 0；
草稿色盤 1 未觀測，故原圖塊用途與實際樣本仍需確認，不能宣稱草稿已在遊戲顯示。
只知道整塊上傳曾被使用，不能推定其中 tile 37 被相同色盤取樣。
研究美術未安裝；正式 HD catalog 仍為 0，CPU 仍 DRAFT overlay，交付版仍 v.0.6.0-20261018。

新的輸出研究入口：[RGB555 連續區段 DRAFT](workplace/hdtex/perf/screen-span-proposal.md)。
目前 GUI profile 的 ScreenInto 約 16% CPU，逐 RGB555 迴圈約 7.42 秒／77.51 秒；
用兩段連續來源取代每像素 wrap mask，每四像素用兩次 LittleEndian 64-bit 寫入。
沒有 unsafe，原生與 RGB24／側畫布不變，先走獨立逐色 oracle、全部字值、wrap／尾端及微基準。
原型只在 screen-span-overlay.json，正式 screen.go 未改。未在 GUI 量測期間執行重型測試。

CPU 分派的研究入口：[NOP／BIOS DRAFT](workplace/hdtex/perf/cpu-dispatch-proposal.md)。
僅 v3 overlay：NOP 保留原 fetch／中斷／doLDS／GPR0／advance；KSEG1 512 KiB BIOS 保留 +24 週期。
與保留的未修改 step 作完整狀態對照，合成 ROM 只作 ISA 取指測試，不能替代真 BIOS。
v2 的有效 M4 與源檔保持不動；v3 尚未測，正式 CPU 未改。

### RGB555 區段轉換轉入正式來源

獨立逐色 oracle／全部 65536 原字／未對齊與尾端／完整寬度／wrap／RGB24／side canvas 的
race 通過 21.327 秒。640×480 buffer reuse 微基準不 wrap 1.150～1.358 ms → 0.455～0.510 ms，
wrap 0.901～1.280 ms → 0.438～0.623 ms，皆 0 alloc；不外推整體倍速。
先將 016 §8.4 寫 READY，再複製同一實作及測試進正式 screen.go／screen_reuse_test.go。
CPU v3 仍只研究 overlay；新的完整狀態對照尚在跑，不能沿用 v2 M4 宣稱新分派完成。

v3 正常 M4 的準備入口為 perf/stone-m4-overlay-v3.json，尚未執行。
新的有界石牆 observer 每個上傳只判一次目標 SHA，在目標區塊記錄 16×16 圖塊與有效 CLUT 的轉換；
最多 16384 組，不把轉換次數當像素取樣數，不輸出原始色盤或影像。來源是已知 W32/H128/depth4。
這能分辨真正 tile 37 的色盤，避免僅憑整塊上傳推斷單一圖塊。
正式 GPU 不加入遊戲專屬觀察器。M4 通過後另留本機 m4-card.mcd 供正常重開讀取與美術對照，
卡片及圖像仍只存 ignored workplace；不新增 direct-entry 或座標注入。

v3 M4 另外在 CPU field 32400、36200、45100 保存原生／HD2 圖與當場 Digest，
GPU 命令標籤為 field-1。這是正常玩家按鍵檢查點，未安裝新美術、未開 WS、沒有座標或 RAM 注入。

008 §7–8 已加入可審查 DRAFT，分開既有 CONFORMED 週期與新 RAM／分派原型。
完整 precondition、terminal 模距離、PGXP 掛點時間、未知 observer 回退與 v1 勘誤均保留。
正式 CPU／Machine／PGXP 尚未改，不把 v2 的 M4 借給仍在驗的 v3。

解除固定核心的同版診斷到 field 2400，0.762／0.938／0.929／0.836，最後 Draw 完成；
另關 WS 為 0.971／0.996／0.897／0.786，最後 Draw 完成。不同時段／負載不當作因果 A/B。
沒有因此改本機包預設 WS；005 §8.3 有既定的玩家設定例外。
最新診斷／v3 來源與雜湊入口為 perf/dispatch-v3-verification.json；完整 v3 M4 尚在跑。

v3 的原 step 對照 480 組、迴圈 272 組與 race 通過 192.618 秒。
公開 CPU、1150 筆 GTE、access-time 通過，SPUCNT/32 既有差異未改。
NOP／BIOS 分派微基準為局部成本，全部 0 alloc，來源 dispatch-v3-bench.log；
不以這些單步數字取代正常 GUI 速度驗證。正式 CPU 尚未改。


### 2026-10-18：v3 正常 M4 通過，008 §7–8 READY 轉入正式來源

獨立 v3 M4 1245.25 秒完成正常空白卡開機至 field 45784，原生摘要、卡片、原生／HD PNG 與基準相同；有效 6870854 批、14276539524 週期。收據為 workplace/hdtex/perf/poll-v3-m4.log、poll-v3-m4/。既有完整狀態、公開 CPU／GTE 及事件／IRQ 證據足夠支持等價優化，先標 READY 再轉入 CPU／Machine／PGXP，正式來源移除研究原子計數器。正式來源檢查待跑。

使用者確認 stone-wall-v1 美術方向；原版未上傳，草稿尚未在正常路線匹配。完整硬體 GUI v3 仍在跑，field 21600 約 0.394 倍速，速度閘門未過。未更動目前 v.0.6.0-20261018 完整版。


### 2026-10-18：石牆來源重選與正常讀檔窄驗證入口

新有界研究工具 workplace/hdtex/perf/stone-reload.go 只從實際 BIOS／光碟及 M4 記憶卡正常開機，按鍵讀檔、移動；HD2＋PGXP、WS 關。比較相同輸入的替換關閉／開啟之摘要、卡片、原生像素與最後 HD 像素，不能以整象限命中數宣稱某塊美術已可見。輸出預定同目錄 stone-reload-v2/，只本機保留。圖塊來源與草稿輸出契約見素材 repo docs/assets-catalog.md。


### 2026-10-18：v3 完整硬體 GUI 正常首存，速度未過

本機收據 workplace/hdtex/perf/gui-v3-verification.json。Intel MTL、HD2＋PGXP＋xBR＋WS、空白卡及 paced audio 跑到 field 45784，退出碼 0，final_drawn=true，顯示與最後發布 frame 相同。記憶卡符合既有首存雜湊。最終 PNG 對歷史 GUI 的比較與所有速度窗口見收據，最低 0.394、中位 0.9065000000000001；低速伴隨音訊供應不足，速度閘門未過。WS 模式未產生原生摘要，不能拿卡片／畫面替代 CPU／RAM 證明。測速期間無建置或測試，曾有輕量文件讀取及兩次有界單 CPU 圖像對照，非不同主機負載的因果 A/B。容器正常退出並移除。


### 2026-10-18：正式 CPU 等價範圍 CONFORMED

新入口 workplace/hdtex/perf/formal-v3-adoption-verification.json。正式 CPU 全套 5.795 秒、PGXP 0.024 秒；倒數／分派 race CPU 133.331 秒，真 BIOS 的事件／IRQ race machine 40.814 秒，全為退出碼 0。正式來源移除研究原子計數器，008 §7–8 標 CONFORMED，僅限等價行為。速度閘門保持未通過，完整 GUI 首存與最後畫面符合基準不能代替速度。第二石牆試包正式載入器通過，入口 stone-v2-loader.json；正常讀檔對照另在跑。


### 2026-10-18：正常讀檔成功，修正存檔房出口測試

stone-reload-v2 的 baseline 正常讀檔成功；LEFT 120 碰到存檔房左牆，不能拿來驗城堡石牆。已對照 m4-first-save.press 最後 LEFT 入房確認正確出口為 RIGHT，主動停止該研究容器，退出碼 143，不列產品缺陷或完整替換驗收。原探針另存 stone-reload-left.go.txt，原輸出保留。

新入口仍為 stone-reload.go，下一輸出 stone-reload-v2-right/，按鍵向右返回城堡；可選 --dump 使用正式 018 傾印記錄實際 CLUT／來源，--profile 只量替換開啟分支 field8000–9000 的 CPU 剖析。兩者均屬診斷，不作速度 A/B。原版衍生輸出只留 workplace/hdtex/perf/，不進 Git。


### 2026-10-18：石牆第二試包未命中，改用實際存檔房石柱

stone-reload-v2-right/ 完成正常讀檔至 field9000，原生摘要、卡片與原生像素全部相同，HD changed fields=[]，NO3 第二試包 0 次命中。該按鍵只在存檔房移動，未證明已離房，先前意圖返回城堡不當作完成證據。正式 dump 完成 3140 個不重複上傳、301 組使用；石柱的實際來源為 BIN/F_GAME.BIN page4 象限0，upload 8aedcdcab7c92892fb769eadc19b4d20eb24f3241bf128e81b6c664fdb3d3bf0、CLUT 4c53b213d949547b15cb5b72d1eb690c330f2e609bab09d13fdae59bd1faeaa2，與共用色盤163相同。這是整象限來源／CLUT 證據，單塊可見性仍另驗。

新實際素材樣本沿用已核准石材，預定素材側 hd-assets/drafts/stone-column-v1-*。研究 stone-reload.go 新增 --baseline 接受已完成、相同卡片與相同按鍵的正常 baseline 收據，只重跑替換分支；仍從 BIOS／光碟開機，不載入任意 RAM 或 PC。舊右路線探針保存 stone-reload-right-dump.go.txt。下一輸出為 stone-column-v1-reload/；baseline SHA 必須保存。


### 2026-10-18：石柱區塊命中但 tile51 未顯示，改追實際目的／來源座標

stone-column-v1-reload/ 正常相同輸入至 field9000，11 個原生摘要／卡片／原生畫面檢查點相同。來源區塊命中 638048 次，但 HD changed fields=[]。這不能作美術可見性驗收。連續兩次未改變畫面後，重查 reverse-engineer-retro-game-remake 的 verification-and-handoff.md，停止靠整頁預覽挑 tile。

新研究入口 stone-source-probe.go、source-probe-overlay.json，只在 HD 呈現路徑的實際目的區域記錄來源 upload／CLUT／word offset／E2 後 UV 及原生目的座標，另記 VRAM 複製矩形。欄位固定6400–6550、6799、7999、8999，來源組合最多32768、複製最多4096，超出明示 truncated；不改顏色或 native 狀態。輸出預定 source-region-reload/source-region.json，仍正常開機讀檔，沿用同一 baseline。沒有進正式 GPU。


### 2026-10-18：可見石柱真正來自執行時圖集

source-region-reload 完成11個原生／HD檢查點對照，觀測器無輸出改變。source-region.json 有2473個來源組合、truncated=false；此視窗0個複製命令，不能外推其他欄位。目的石柱主要來源為 upload05c7b9bd…、CLUT4c53b213…；執行時128×128的tile13在目的區域被讀67584次。先前F_GAME磁碟頁與此圖集座標不同；其原始archive組成方式仍未知。正式Dump對此上傳有多個CLUT預覽，第一個預覽不代表目標CLUT，沒有證據顯示正式CLUT快取錯誤。

素材側保存精確runtime索引／CLUT快照，編譯器做原生上傳鍵往返驗證。下一試包stone-column-v2只改真正來源tile13，下一正常同輸入輸出為 workplace/hdtex/perf/stone-column-v2-reload/；既有baseline不變，不猜補來源座標。


### 2026-10-18：核准石材第一個正常玩家可見樣本通過

stone-column-v2-reload/ 以正式來源與真正runtime圖集tile13跑到field9000，11個原生摘要／卡片／原生畫面檢查點相同；7個HD檢查點改變。field8000 1030像素差異，原始HD畫布512×480。素材側 hd-assets/drafts/stone-column-v2-ingame.png／json為同狀態對照；RGBA／STP離線檢查2087個RGB555子像素改變，960個邊界子像素及其他63格保持，5組runtime錯誤來源拒絕通過。正式載入器兩試包均1entry。

比較圖工具首次把高度誤限512×448，屬驗證工具假設，不是遊戲失敗；改為先讀兩側實際尺寸並核對Go像素雜湊，在同容器與同命令乾淨重跑通過。來源、原始PNG與比較圖雜湊均在圖表收據。未安裝正式完整版；v.0.6.0-20261018維持，完整HD與速度閘門仍未過。


### 2026-10-18：本批美術與效能工作收尾自檢

原版光碟、BIOS／cue與295個拆檔雜湊再核對不變；正式CPU／PGXP／GPU檔案仍符合已驗證source雜湊。find掃描無root-owned檔案或異常.md目錄，抽查1000:1000；清除本輪Python衍生快取。當批psgolem-*與sotn-*研究／驗證容器全已完成並移除，收尾audit也用--rm。新收據 workplace/hdtex/perf/art-final-hygiene-verification.json。未commit／push／tag／Release／Issue寫入；現行完整版仍v.0.6.0-20261018，完整HD與速度未完成。

### 2026-10-19：速度優先，HD 掃描線等價重構

使用者要求先調整速度，暫停擴充美術。入口為 016 §8.5 READY。
保存修改前 poly.go 於 workplace/hdtex/perf/span-step-before.go.txt，先外移固定旗標與抖色運算。
獨立完整狀態對照通過34.124秒；第一輪 wall-clock 微基準波動大，
改用交錯 CPU／wall 量測，原始記錄 span-step-cpu-probe.log，不外推整體速度。
再依現有快取讀寫關係，重用同掃描線連續相同 UV 的原生貼素，
仍逐子像素查 HD 素材、調變、混色與更新統計。新版本驗證待跑。
正常讀檔效能工具 speed-reload.go、硬體 GUI 腳本 paced-speed-reload.sh
只使用已完成 M4 的記憶卡及正常按鍵，沒有 RAM／PC 注入。研究輸出沿用原 perf 目錄。

### 2026-10-19：同 UV 原型不採用，改做原色掃描線分派

span-v2-hd-all.log 通過11.758秒，實際並行／替換與重疊 race 通過84.769秒。
先前全量單執行緒狀態 race 主動中止143，超過6分鐘，未冒稱通過，紀錄 span-v2-race-scope.json。
固定核心4／GOMAXPROCS1的8組交錯 CPU 量測顯示 unique UV 中位成本增加約7%，
重複 UV 改善不足且波動大，因此保存 v2 來源與所有數據，不採貼素重用。
兩輪收益未收斂時重查規格／驗收路由。依既有全65536字的中性色恆等式，
將原色及無抖色固定中性色多邊形分派至簡單 UV 迴圈，先寫 016 §8.6 READY，再修改正式來源。
getTexel 與逐子像素材統計均保留；v3 驗證與實際正常讀檔／GUI 尚未完成。

### 2026-10-19：正常讀檔等價完成，使用者要求比較成熟模擬器與參數

原色掃描線新版的完整 HD 測試20.879秒、真正並行路徑 race 5.816秒通過。
固定核心配對中位 CPU 比0.6387／0.6556，平面無抖色中性色降低約34～36%；gouraud波動不作加速聲明。
正常真 BIOS／原碟／原 M4 存檔跑至field9000，15組原生摘要及HD像素全部相同，
卡片及原生畫面符合既有基準。收據 span-reload-verification.json。
舊327.00秒／新152.18秒跨不同主機負載，不聲稱整體加速百分比。

使用者新要求比較現有 PS 模擬器並排查系統參數，先轉入這個診斷分支，HD美術仍暫停。
入口 parameter-probe.go、mednafen-performance.sh。前者仍正常 BIOS 開機，分別量 native、
HD2、PGXP與Go執行緒，記錄I/O、CPU壓力及真實參數；不是新正式CLI。
既有 Mednafen 1.29.0 映像沿用，軟體畫面輸出、frameskip=0、聲音仍開、44100Hz，
使用SDL dummy作技術節拍，快轉僅診斷最大吞吐，不當正常遊玩聲音驗收。
資料依官方 Mednafen／Libretro／DuckStation 文件；DuckStation只讀文件、不移植程式碼。
現有014記錄Beetle固定工具未能開機，不能借它宣稱實際遊戲性能。


### 2026-10-19：CPU 軟體模擬對照與系統參數診斷完成

收尾再核對復古遊戲驗收／spec 閘門路由。六組正常 BIOS／原碟開機至field5400，
原生、HD2、PGXP及GOMAXPROCS2／4／14的9個原生摘要、卡片與原生畫面均相同。
指令快取全開、ExactTiming全關、抽樣MDEC未等時鐘，容器沒有CPU配額節流。
原生相同案例前後選單吞吐0.918／3.898倍，主機負載與頻率有明顯變化，
不可用六個順序案例聲稱PGXP或執行緒數的因果收益。
矩陣期間I/O壓力累計約4毫秒，沒有證據以光碟預載作主要修法。
CPU政策唯讀快照允許睿頻、上限4.9GHz；powersave配合performance偏好，
不能單憑名稱斷言降速原因。沒有修改主機政策、正式遊戲時鐘或已批准畫質預設。
入口 parameter-comparison.json、parameter-diagnostic-inputs.json。

另固定0、2兩顆不同實體效能核心，GOMAXPROCS2，PGXP開／關／開：
最後600fields吞吐0.866／1.144／1.110倍，9個原生摘要、卡片及原生畫面相同，終點HD像素也相同。
核心時脈及共享主機排程仍變化，不宣稱固定核心或關PGXP的穩定加速倍數。
field4500–5400的CPU取樣中，HD drawSpan累計54.24／56.87／56.12%，
三組都以HD多邊形掃描線為主要熱點；包含取樣、調變與繪製，不和子函式百分比相加。
這是標題／讀檔選單的分布，不外推城堡或完整流程。
入口 pinned-parameter-comparison.json、pinned-*-top.txt。

沿用Mednafen1.29.0固定映像，以相同原版、正常開機及M4卡片測試。
原生CPU軟體繪圖、SDL軟體呈現、frameskip0、sound0、cd.image_memcache0：
正常節拍抽樣59.837FPS，選單快轉144.97／182.47／156.79FPS。
這支持原生場景可用CPU正常模擬，不是聲音或整款遊戲驗收，也不是精確同狀態性能A/B。
前一SDL dummy音效輸出有較低FPS，不能據此判斷模擬核心不足；
首次無聲測試的X_SetInputFocus失敗屬未映射視窗競態，改等onlyvisible後同命令乾淨重跑通過。
收據 mednafen-comparison.json、mednafen-cpu-nosound-v2/。
官方架構對照：
https://github.com/stenzek/duckstation
https://docs.libretro.com/library/beetle_psx/
https://mednafen.github.io/documentation/mednafen.html
DuckStation具有JIT、向量化／多執行緒軟體繪圖及硬體GPU選項；
ps-golem目前為直譯器、原生與HD雙CPU繪圖，硬體GPU處理最後呈現與xBR。
Beetle的PGXP有不同成本模式，不能直接將ps-golem等同其memory+CPU模式。
只用官方文件作架構參考，沒有搬入非自由程式碼。

新版實際Intel MTL硬體OpenGL GUI正常讀檔、移動及攻擊至field9000，
最後幀5498已顯示，卡片仍符合既有M4值，截圖確認實際存檔房。
HD2／PGXP／xBR／WS開啟，無素材包；速度最低0.491、中位0.980，
15個窗口有7個低於0.95，有音訊缺樣，速度閘門未通過。
video_avg只是呈現請求CPU時間，沒有涵蓋GPU硬體完整執行，不作GPU耗時證據。
WS開啟不產生原生摘要；另由span-reload-verification.json的15個WS關閉檢查點證明重構等價。
收據 span-v3-gui-verification.json、span-v3-gui-reload/。
這是9千fields讀檔路線，不是新的完整M4或通關驗收。
新來源尚未組裝進正式完整版，現行交付仍v.0.6.0-20261018；HD美術擴充仍暫停。

本批收尾：原版光碟、BIOS及cue雜湊保持；兩repo diff --check及新來源gofmt通過。
掃描沒有root-owned檔案或異常.md目錄，新檔抽查1000:1000。
PGXP開關會改變部分HD檢查點，原生9個摘要、卡片及原生畫面均不變；終點選單HD相同，
同為PGXP開的兩次固定核心重播全部HD檢查點相同。差異欄位已寫入pinned-parameter-comparison.json。
本批psgolem-*量測容器全部完成並移除；未commit／push／tag／Release。
收尾收據 workplace/hdtex/perf/speed-comparison-final-audit.json。

### 2026-10-19：使用者指定HD繪圖支援GPU

採用GPU執行HD像素繪圖，原生決定性CPU基準維持；不把共享主機負載當成唯一掉速原因。
路由命中reverse-engineer-retro-game-remake與spec閘門，GPU方向已有使用者明確授權，不重問。
新增021-hd-gpu-renderer.md DRAFT並掛入000索引。先做未接production入口的Kage數值元件，
驗證RGB555／STP／混色／遮罩／32位元插值，再依結果推進READY與命令整合。
render/hdgpu/獨立放含Ebitengine的裝置，避免無頭hd核心引入GUI初始化。
來源是既有render/hd及固定Ebitengine2.9.9公開API，沒有搬入DuckStation程式碼。

GPU數值原型通過：軟體OpenGL14.652秒；Intel MTL硬體完整字／混色／mask／插值6.41秒，倍率及恆等0.58秒。未出現像素差異。
021已審查推進READY，開始命令流及VRAM相依整合。收據gpu-kernel-evidence.json及gpu-kernel-hardware/。

GPU第一版正常開機在[600, 1200, 1800, 2400, 3000, 3600, 4200, 4800, 5400, 6000, 6600]檢查點的原生摘要與HD像素均相同，但速度差。
依Ebitengine2.9.9 ui.readPixels查明跨frame回讀會等下一frame；主動停止首輪143，未當9000fields通過。
改由Update以容量1／25ms有界服務佇列執行GPU請求，Screen／VRAM快照結尾Flush。
原始收據gpu-normal-v1-interrupted.json保留，修正後重跑同一條正常讀檔路線。

GPU服務佇列v2在Intel上通過全字數值1.45秒、恆等0.32秒、30組混合命令2.89秒、
替換／STP0.14秒。但實際GUI依然慢；在field8621以WM_DELETE正常關窗退出0，
未當成field9000／最後幀通過。stats為三角形111408、精靈551514、GPU像素2420492072、
CPU回退126080、回讀19515。沒有xdotool的R2驗證映像改用既有xgb依賴的
window-close.go／window-close工具，Docker內建置與送WM_DELETE，未改用主機GUI工具。
正常關窗GPU排空驗證成立，速度仍失敗。
CPU取樣gpu-profile-v2/：47.82秒中CPU19.19秒；服務計數gpu-service-profile/包含等待與工作時間，
不能將VBlank吞吐當獨立GPU計算速度。重查spec閘門後，先分開提交與回讀，再以
Unmanaged工作影像避開反覆來源／目的切換的自動圖集。不是調高原生時鐘。

### 2026-10-19：GPU HD命令整合與本機候選驗證完成，速度仍未過

v3分開提交／回讀並使用Unmanaged工作影像後，Intel MTL全字數值1.23秒、
倍率／恆等0.21秒、30組混合命令2.63秒、索引替換／STP0.11秒通過。
HD完整鏡像／快取／來源／傾印狀態race14.600秒；真正GPU混合命令及替換race通過。
後續加全彩RGBA＋黑白STP遮罩及透明／實色／半透明黑色實際命中，race通過；
再補GPU新寫區的重疊VRAM複製。收據gpu-command-final-race/、gpu-color-race/。
cmd與frontend乾淨測試0.025／0.067秒通過。

第一次v3功能長重播碰到420秒外層時限，退出124；到field7200的12個檢查點相同。
保存gpu-normal-v3-timeout.json，沒有當成field9000成功。提高到1200秒有界時限後乾淨重跑，
正常真BIOS／原碟／既有M4卡片／既有按鍵，至field9000退出0：15個原生摘要及HD像素相同，
卡片5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36，
原生像素342defa70e117d0d65246d2f6859fb403feef1f8954a701d9162d7e8d49e71f9，
HD像素76190a1a6e312c2626ba7d7cf53c3e16f19cc9b1a60daf36ffb8f4294fa2b659。
GPU三角形138610、精靈636791、像素2935327782、CPU回退132234、回讀19359。
入口gpu-normal-verification.json；最後圖已檢視，為存檔房正常操作結尾。
研究小視窗每場擷取、VSync關閉，期間另有建置及短驗證，只作功能／狀態證據，不作效能A/B。

速度仍慢，因此通用程式預設CPU；gpu／auto明確選用且保存，下次雙擊沿用。
未調高原生時鐘、未修改原生GPU／Machine、未用降低畫質掩蓋同步成本。
021保持READY，000索引修正狀態及表格；玩法、速度、GPU支援、美術與通關分開記錄。

新v.0.7.0-20261019三平台本機候選由乾淨輸入重建到dist-all/該版本patch/，
封包／版號／雜湊／素材排除通過；macOS只有結構驗證，不宣稱GPU真機通過。
素材側組裝到local-full/同版本，明確gpu設定，原版唯讀並逐平台驗證複製雜湊。
Linux候選從/tmp無素材與後端參數啟動，180幀正常退出且最後幀顯示；
重開保留相對素材路徑及GPU設定，field603收到WM_DELETE正常退出0，GPU工作非零且排空。
關窗final_drawn=false如實保留，這不是-frames最後幀失敗。技術靜音，不代替音訊驗收。
入口gpu-package-verification.json；總入口gpu-backend-verification.json。
目前穩定完整版仍v.0.6.0-20261018，GPU候選未取代它；未commit／push／tag／Release／Issue寫入。

本批GPU收尾：兩repo diff --check、新GPU與設定來源gofmt通過；
原版光碟／BIOS／cue雜湊保持，兩工作根無root-owned或異常.md目錄，抽查1000:1000。
所有命名GPU驗證容器已完成並刪除，無新image或共用資源清理。
最終入口gpu-backend-verification.json，衛生收據gpu-final-audit.json。

### 2026-10-19：續作HD並查證GPSXE的Go加速方式

使用者確認保留CPU／GPU後端，繼續HD；另外指定比較Koops0/GPSXE。
公開原始碼固定b1ecd8cdcd6dab0a588d2e0a6077210e5fa2ba11，MIT，只有來源／shader／授權，未下載或執行BIOS、DLL或外部程式。
GPU批次累積可作設計參考，但它Draw仍同步等Fence，COP2與貼圖未完成；不作月下性能基準。
Go編譯器證實hdgpu逐圖元參數表、float切片與請求閉包的堆積逃逸，尚未改正式程式或聲稱加速。
候選與出處入口workplace/hdtex/perf/gpsxe-comparison.json；002補入固定來源與授權。

素材側沿用核准石材與已觀測runtime圖集，首批12塊；22796個RGB555子像素變更，
未選圖塊、接縫、透明及STP保持。工具build_hd_stone_batch.py已掛入assets-catalog.md，
正常讀檔／移動／攻擊對照進行中，正式HD目錄計數尚未增加。

首批12塊石材已用目前正式來源的stone-batch-replay完成正常開機讀檔到field9000，
11個原生摘要／原生像素／卡片檢查點相同，7個HD畫面改變；field8000有9876像素差異。
原始與快取來源字相同的實際取樣已逐塊確認；離線PNG往返22796個RGB555子像素變更，
接縫、透明、STP、其他52塊不變。對照工具新增可量字寬的label，批次對照與正常玩家圖已掛入素材總覽。
新批次只跑CPU後端正常路線，不拿021既有GPU素材測試冒充本批長重播。
入口素材側save-room-stone-v1-runtime-verification.json／-pixel-verification.json／-ingame.json，
執行器側workplace/hdtex/perf/save-room-stone-v1-reload/verification.json。
仍為HD試包，未改現行完整版或GPU候選的設定；正式HD目錄0／264。

本批收尾：兩repo diff --check、Python語法、來源雜湊、總覽既有入口正對照與新入口通過。
原版光碟／BIOS／cue雜湊保持，無root-owned或異常.md目錄，抽查1000:1000。
本輪Go及Pillow一次性容器已完成，素材總覽命名容器查無殘留；未建新image、未改主機參數。
衛生及工具來源入口workplace/hdtex/perf/hd-batch-gpsxe-final-audit.json。
未commit／push／tag／Release／Issue寫入；GPU／CPU支援維持，新HD試包尚未裝入完整版。

### 2026-10-19：先登記HD與延後優化工作，ps-golem改為public

使用者明確要求公開ps-golem，並先將全部工作登記Issue，HD完成後再討論優化。
已核對兩repo現有Issue，新增素材#5材質、#6怪物／角色／動畫、#7地圖；
執行器#8雙後端素材包與交付、#9GPU同步／批次優化延後、#10Go配置／PGO優化延後，
#11公開稽核；三則現行狀態留言掛回素材#1及執行器#1／#4，沒有重開已完成任務。

主機gh auth通過，遠端只有main6271e76，無tag／Release／Actions成品／PR。
Docker唯讀掃描61個提交、398個blob，共2473027位元組，皆UTF-8文字、無NUL，
未發現原版／二進位素材路徑或常見憑證模式；既有Issue／留言沒有原版附件。
稽核收據workplace/hdtex/perf/repository-public-audit.json，靜態檢查不宣稱涵蓋所有祕密形式。
gh repo edit完成後回讀visibility=PUBLIC，url=https://github.com/wicanr2/ps-golem。
本機未提交程式、研究輸出、HD衍生圖與含素材local-full均未上傳。
素材AGENTS更新public／private界線；公開只適用ps-golem，素材repo維持private。
尚未commit／push／tag／Release；接下來只做HD，效能實作延後。

### 2026-10-19：HD15塊前景子集與本機雙擊預覽

保留兩後端，沒有新的效能實作。內建image_gen以核准AI石材作風格參考生成鐵表面，
未上傳原版圖；原圖、完整提示與來源雜湊存素材側save-room-iron-v1.png／json。
編製器加入foreground模式，13塊石材與2塊格柵，保留輪廓與接縫，降低亮部顆粒。
獨立PNG往返24329個RGB555子像素改變，其他49塊、透明及STP保持。
正式來源的正常CPU重播至field9000，11個原生摘要／畫面／卡片相同；7個HD畫面改變，
field8000有10459像素差異。真正Intel GPU以該包實際上傳／CLUT測五種模式、精靈／多邊形／
重疊複製，完整HD VRAM、畫面及替換統計一致；這不是GPU正常遊戲長重播。

素材候選的manifest字節可與舊試包相同，不能單靠manifest驗美術版本；
GPU工具補入全部PNG／STP內容雜湊並前後核對，短測重跑通過；舊收據保留before-image-hashes。
READY預覽契約掛入assets-catalog，install_hd_preview.py核對原生／像素／GPU證據後，
將包複製至三平台v.0.7.0本機候選並只保存textures相對路徑，設定先備份；執行檔不改。
Linux首次180幀正常完成，但重開時Weston segfault、應用回報X連線中斷。
保存gpu-foreground-package-attempt1.json，按同一image／候選／腳本乾淨重跑退出0，
180幀final_drawn=true；field606 WM_DELETE正常關窗，設定重開保持。技術靜音，不算音訊或速度通過。

本機證據：save-room-foreground-v1-reload/、gpu-foreground-pack/、
hd-preview-package-verification.json及素材側workplace/assets/hd-preview-install/。
正式HD目錄0／264，當前穩定v.0.6.0未改；新預覽只是15塊子集。
進度已貼素材private Issue#5。公開Issue#8的詳細本機metadata留言被auto-review拒絕，
理由為未明確授權該metadata外傳；已改為僅一般工作狀態留言，移除本機路徑／版本／數據後獲准，
未繞過拒絕或把詳細metadata換別種方式公開。公開僅記排程與進度，證據留本機。

新素材、圖塊對照與正常玩家對照掛回素材總覽；Windows／macOS未真機。
未commit／push／tag／Release；原版與衍生圖均未加入公開repo。

本批收尾：兩repo diff --check、Python語法、原版光碟／BIOS／cue雜湊、
各平台既有非textures設定與新素材複製雜湊通過；總覽既有入口正對照與新入口通過。
兩工作根無root-owned或異常.md目錄，抽查1000:1000。GPU／候選／素材命名容器均移除，
Go工作容器已結束；未建新image或清理他人資源。
最終入口workplace/hdtex/perf/hd-foreground-public-final-audit.json，公開稽核repository-public-audit.json。
已登記GitHub工作；HD仍未全部完成，CPU／GPU支援保留，優化延後。

### 2026-10-19：先完成HD，四圖集素材批次

使用者先說先玩HD，隨後明確改為先完成HD。本輪只做素材製作與驗收，未做新效能實作。
載入復古素材／驗收路由、reverse-engineer-retro-game-remake及verification-and-handoff；
沿用018／019既有格式，材料assets-catalog新增READY批次及預覽更新契約。

build_hd_room_pack.py編製save-room-atlases-v1雙圖集三CLUT，再以--decor完成v2四圖集五CLUT。
使用既有核准石材／鐵材質，另以內建imagegen文字生成木材，未提供原版圖像；
原圖與完整提示存素材hd-assets/drafts/save-room-wood-v1.png／json。
v2涵蓋前景、背景、棺框與燈座，246個不同來源圖塊改變；五色盤合計392104個RGB555子像素，
輪廓、原色調、接縫、透明與STP保持。未觀測圖塊只稱離線編製，不算全房或全遊戲完成。
背景細節較v1降至45%，減少顆粒。編製工具及逐輸出雜湊均保存在-pack.json。

第一個decor批次在6e3c3fe2…來源被零變更檢查攔下。原始16384個索引全零，
雖有取樣紀錄，不能當成HD建築。部分輸出與來源保存在材料workplace/assets/hd-room-author-attempt1/，
分類為來源選擇錯誤，非引擎缺陷。排除該來源後以相同image／命令乾淨重跑通過。
沒有提高顆粒或猜補像素來讓空白來源通過。

CPU正式載入器正常開機／讀檔／移動／攻擊至field9000，沿用同卡、同按鍵、同基準收據：
11個原生摘要／畫面／卡片相同，7個HD畫面改變，field8000有32932像素差異。
v1與v2這個晚期場景的HD畫面相同，不能把它當成棺框可見性。另以僅棺框的精確條目，
兩次獨立正常開機至field6700；6500／6600／6700三個原生檢查點相同，
HD只在6500／6600改變，6500有7142像素差異；6700棺框消失後恢復相同。
compare_hd_sample.py新增同一收據內成對獨立開機基準，仍逐PNG核對摘要及原生狀態。

真正Intel MTL GPU使用實際索引、上傳鍵與CLUT，各測五種模式，共25組；
精靈、多邊形、漸層及重疊複製的完整HD VRAM、畫面與替換統計相同。
GPU工具與輸入腳本為gpu-room-check.go／gpu-room-decor-run.sh，
逐來源收據在gpu-room-atlases-v2-pack/。這是獨立素材命令，不是GPU正常遊戲長重播或速度。

install_hd_preview.py --room-atlases核對以上證據後更新本機v.0.7.0三平台候選。
保留15塊舊包，僅改textures相對路徑；原設定與組裝收據存材料workplace/assets/hd-room-preview-install/，
舊hd_preview移入assembly.json的hd_preview_history。執行檔、其他設定與使用者卡片不改，
現行v.0.6.0不改。Linux真正GPU無素材參數啟動、重開與WM_DELETE正常退出0，
frames180最後畫面已呈現；重開field605關閉，設定保持。技術靜音，未做人工音訊／速度聲明。
Windows／macOS只代表組裝完成，待真機。

整合入口workplace/hdtex/perf/hd-room-atlases-verification.json與hd-room-finalize.py，
材料側save-room-atlases-v2-runtime-verification.json、-ingame.png及-coffin-ingame.png。
正式HD目錄仍0／264；其他關卡、角色／怪物／動畫、地圖與完整交付尚未完成。
本批詳細進度只寫私人素材Issue#5，公開repo未貼本機metadata或圖片，父項目維持開啟。
未commit／push／tag／Release；原版、衍生圖與local-full均未上傳。

收尾：兩repo diff --check、Python語法、素材與複製雜湊、非textures設定、舊預覽及執行檔保持通過。
原版光碟／BIOS／cue雜湊保持；工作根find無root-owned或異常.md目錄，抽查1000:1000。
本輪命名容器已自動移除，未建新image、未清理其他專案資源；CPU／GPU兩後端保留，優化延後。

本批最後連結稽核發現既有草稿HTML以容器/out與/hd的相對位置寫入href，主機開啟會失效。已改用專案固定workplace/assets → ../../hd-assets/drafts關係，重建總覽；在實際主機目錄關係中逐PNG解析，既有石柱正對照及新增入口均通過。首輪失敗是素材總覽工具缺陷，未影響已安裝HD或原版資料。

### 2026-10-19：精簡HD素材、正式3D透視校正與提交準備

使用者要求素材圖片維持小型，並核准HD透視校正、保留原版模式及缺深度回退。
本輪載入復古素材／驗收、spec閘門、文件職責及grilling決策規則；既有選擇不重問。

素材工具compact_hd_pack.py保留正式解碼RGB555／STP，改用PNG色表、1bit遮罩及共用遮罩。
四圖集五色盤由910081降至321032 bytes，減少64.7%；ZIP313833 bytes、9檔。
正式Go載入器逐一核對1310720個字完全相同。三平台候選已裝精簡包，僅改textures，
其他設定、執行檔、原版資料與舊試包保持。Linux真Intel啟動及新顯示工作階段重開／關窗通過。
同一Weston重開曾segfault，保留環境失敗；不稱同合成器重開通過。
初次檔案稽核以read_bytes雜湊大光碟碰到容器記憶體上限137，改串流雜湊後同工具鏈重跑通過。
入口compact-hd-delivery-verification.json；素材私人Issue#8已完成並關閉。這是儲存精簡，不是FPS收益。

透視研究先做自製斜面與解析反投影，CPU／Intel GPU五素材各五模式通過。
第一版正常開機到13000fields，XY有效但有效深度全為0，未當完成；原型與收據保留。
第二版補上兩半座標的投影來源，拒絕混合來源，移除不符合既有浮點XY模型的遠深度硬上限。
兩項同時改動，沒有聲稱已量到各自對實際遊戲的獨立貢獻。
第二版正常序章取得深度及鐘樓可見差異後，022由DRAFT審查至READY，再接正式程式。
正式GTE另拒絕近裁切與XY飽和深度，不改既有XY或任何原生暫存器。

新增-texture-perspective、保存texture_perspective，明確false關閉；需要HD2以上與PGXP。
完整字與同來源半字保留深度，原值失效、位元組覆寫及異源重組回退。
CPU原色／調變、HD子貼素及GPU命令準備共用有理UV，原生覆蓋、顏色、STP、混色保持。

正式版本正常BIOS開機至11724fields，標準M4按鍵，無PC／RAM／座標注入：
20個原生摘要、原生像素與卡片檢查點相同；鐘樓11722／11723／11724連續畫面
分別改變18042／38907／17881個HD像素。其他未保存成對PNG的點只比雜湊，不把缺少像素統計當0。
正式25組Intel CPU／GPU的完整VRAM、畫面、素材統計與仿射差異通過。
真GPU前端在獨立暫存設定中啟用、無透視參數重開沿用及明確關閉均通過，使用者卡片未掛入。
設定彙整首輪直接比較絕對／相對字串失敗；依既有portable契約比較解析後路徑，重跑通過。

回歸首輪go test ./...被未入Git研究目錄的gl-info.c干擾，保存原失敗。
同一工具鏈隔離workplace後全專案通過；正式透視修改後再重跑全套及真正並行路徑race通過。
帶深度觀測的GTE實機1150筆各64暫存器通過；自製深度／裁切、解析幾何與關閉模式通過。
來源與收據入口perspective-production-verification.json；022驗收限已測Linux功能與正常序章抽樣。
尚未更新local-full候選執行檔，未重建三平台透視候選；不聲稱完整美術、速度、通關或非Linux真機。
新增深度來源附件會增加PGXP記憶體與搬運成本，效能仍依使用者要求延後討論。

使用者追加明確授權commit／push目前程式與原版／已驗證HD截圖。
公開ps-golem只放通用程式、規格、自製手把示意及OFL字型；四張未修改的正常遊戲截圖
合計227613 bytes，只放素材私人repo docs/screenshots，README連結來源JSON與範圍。
素材AGENTS記錄此明確窄例外，仍排除光碟、BIOS、卡片、完整素材包與公開遊戲圖片。
提交與遠端確認收據將記錄於precommit-publication-verification.json，不建立tag或Release。

本批提交程式e8d67e0及私人素材563db47，均已push並以git ls-remote核對遠端main相同。
公開Issue#12及私人素材#5已更新；私人#8精簡工作已關閉，整體HD仍未結案。
最終兩工作根find無root-owned檔案或異常.md目錄；本輪sotn-hd-命名容器含已停止者均無殘留。
沿用既有image，未建新image、未清理共用Docker資源，未建立tag或Release。
公開引擎只含自製手把示意與OFL字型圖片；四張遊戲圖僅進私人repo。
來源、原版資料、既有候選執行檔與其他設定的保持證據見本節收據。

### 2026-10-19：透視候選、CPU雙擊預覽與圖集總覽

沿用已批准的HD優先順序與005／021／022，不做新效能實作。
005新增READY候選契約，本機ZIP預設開啟透視並保留HD2、PGXP、xBR與寬螢幕；
通用程式透視預設不變，CPU／GPU均保留。
v.0.7.1-20261019三平台乾淨重建，既有封包、UTF-8、版號及媒體排除檢查通過。
AppImage先在缺Python的osxcross容器完成解出，後續檢查因工具缺件退出127；
改在既有Go容器核對解出內容通過。此為驗證環境問題，沒有因此改產品。

素材專案新候選安裝器核對既有精簡包、來源程式及正式像素證據，只複製必要執行素材。
繼承手把與其他可攜設定；新組裝器明確選擇的後端優先，既定預設CPU。
安裝工具版本、原設定及組裝收據先保存，後續工具修訂不冒充舊安裝來源。
新版封包CPU完成正常序章，原生摘要、記憶卡與HD像素符合來源基準。
Linux實際CPU視窗亦完成正常讀檔、移動及HD素材命中，最後畫面已呈現；
另一顯示工作階段無後端／媒體參數重開，正常關窗並保持設定。
GUI使用既有寬螢幕遊戲修改，不能宣稱該GUI測試原生摘要不變。

新版GPU啟動及獨立顯示工作階段重開通過；正常序章因低速，在目標前正常關窗。
未完成終點、沒有終點截圖，明確保留為未通過，不能用先前素材命令測試替代。
新候選雙擊預覽選用既有CPU設定，GPU仍可選用；其他畫質與手把設定保持。
完整速度、人工音訊、實體手把、通關與其他平台真機維持未驗證。
候選整合收據與獨立設定／日誌在workplace/local-pack/v.0.7.1-20261019/。

素材側另補已驗圖集的總覽及原字對應研究，重複候選與未知來源如實保留。
正常按鍵的離房觀測仍停在原房，未當作新關卡素材覆蓋；美術完成度沒有增加。
首次跨掛載rename失敗後改用複製、雜湊核對再移除；總覽CLI及字型標籤修正記在私人素材證據。
這些研究只供下一批素材選擇，沒有修改正式遊戲路徑或推測補洞。
本批不建立tag或Release，含素材候選與研究圖維持本機，公開引擎不加入遊戲截圖。

本批收尾：Python語法、總覽來源／連結、候選檔案雜湊與兩repo差異檢查通過。
兩工作根無root-owned檔案或異常.md目錄，本輪命名容器含已停止者均無殘留。
沿用既有映像，沒有清理其他專案資源；公開Issue已修正過時狀態，未關閉整體HD項目。
提交前暫存檢查只含預期文字檔，提交／推送依本對話既有授權進行。

本批引擎d272ca2與私人素材9f7c17e均已push，git ls-remote回讀main相同。
公開#8／#12已更新既有工作清單，私人素材#5已補本輪完整限制；所有整體HD項目維持開啟。
後續以正常玩家按鍵觀測新場景，只讀原卡並另存研究輸出，未修改正式程式。

後續正常離房觀測：左右平走各自完整執行仍未出房，沒有因此改碰撞或正式路徑。
重查玩家路線驗收入口，使用既有跳躍按鍵後已正常進入鄰接場景；
分支前的原生摘要／像素、卡片與HD畫面符合基準，原版輸入雜湊保持。
新場景來源比較與本機總覽已保存，角色、介面與建築未混為獨立素材數；
新美術尚未製作，下一批先選實際可見的建材，怪物動畫另追既有Issue。
證據入口workplace/hdtex/perf/castle-jump-right-survey-v3/route-provenance.json，
詳細語意及畫面只在私人素材專案記錄，沒有原版截圖加入公開repo。

正常離房批次收尾：原版與研究卡片雜湊、分支前狀態對照及兩repo diff檢查通過。
兩工作根無root-owned或異常.md目錄，sotn-hd命名容器含已停止者均無殘留。
私人材質#5與怪物#6已登記證據入口；整體HD仍未完成。
最後提交與遠端核對收據為workplace/hdtex/perf/v071-publication-verification.json。

### 2026-10-19：階梯區HD建材與雙擊候選更新

沿用使用者核准的石材風格、019格式及HD優先順序，正式Go與效能路徑未改。
素材側READY契約新增正常離房路線的建築批次；排除角色、怪物及介面，
收錄所選圖集的全部實際色盤，保留原色、暗部、透明、STP與圖塊邊界。
編製器增加批次模式，無損精簡後再合併既有預覽；舊素材與其他候選保持。
正式Go載入器以研究overlay逐字確認新包精簡前後及合併後兩個來源的解碼資料相同。

兩次獨立正常開機／讀檔／移動／跳躍離房，原生摘要、像素與卡片符合既有正常路線；
房內既有HD保持，房外新建材可見。原版建材的對照左側另確認與未替換HD2相同。
真正Intel GPU在單一顯示工作階段完成本批全部來源的五種混色、精靈、多邊形、漸層及重疊複製，
完整HD VRAM、畫面及替換統計同CPU。這是素材命令比對，不冒充GPU正常長路線或速度通過。
來源及收據入口workplace/hdtex/perf/gpu-stairway-verification.json與stairway-atlases-v1-replay/。

素材安裝器只改既有v.0.7.1三平台候選的textures相對路徑；先備份設定與assembly，
保留原執行檔、舊包、後端、透視、手把及其他畫質設定，產生本機必要素材ZIP。
Linux實際候選未帶素材／後端參數，正常讀檔、跳躍離房且最後畫面已呈現；
新的顯示工作階段重開並正常關窗，保存設定保持。GUI沿用既有寬螢幕遊戲修改，
不產生原生摘要相同聲明；人工音訊、完整速度與其他平台真機仍未完成。
整合收據workplace/hdtex/perf/stairway-delivery-verification.json。

本機總覽加入新批次及七圖集代表色盤，舊入口正對照與新入口均通過。
同狀態比較工具支援指定成對模式，仍核對原生狀態、卡片與原始PNG像素雜湊。
第一張放大區落在介面旁背景，保留原圖，另以差異最多的共同視窗編製建材細節對照；
沒有改原始遊戲截圖。私人素材repo依既有明確授權新增兩張未修改正常遊戲圖，
原四張保持，公開引擎沒有加入任何遊戲畫面、原版或衍生素材包。
逐批原字對應與完整動畫仍有未知，正式HD目錄未因局部試包增為完成。
下一批從已觀測怪物圖集追正常動作與色盤，整體HD維持未完成，效能工作延後。

本批收尾：兩repo diff檢查、工具語法、實際PNG與全部素材／備份雜湊通過；
原版光碟／BIOS／cue與研究卡片保持，三平台只改textures，其他設定與執行檔相同。
兩工作根無root-owned或異常.md目錄；本批命名容器含已停止者均已移除。
沿用既有映像，沒有清理其他專案Docker資源，不建立tag或Release。
提交／推送及Issue回讀的本機核對入口為workplace/hdtex/perf/stairway-publication-verification.json。

## 2026-10-19 月下局部怪物素材與正常交戰驗證

遊戲專屬骨質與盾面美術沿用019的精確CLUT全彩PNG與STP格式，於獨立本機素材區編製。
正式Go載入器證實精簡與合併資料逐字保持；實際Intel GPU的混色、翻轉與材質視窗對照通過。
兩次獨立正常BIOS開機、讀檔、離房與交戰的原生摘要、原生像素與卡片符合基準，原有HD保持。
新版局部素材已同步到既有三平台本機候選，執行檔與其他設定不變；Linux實際GUI交戰與獨立重開通過。
這不代表完整怪物、全遊戲HD、交叉oracle、音訊、速度或Windows／macOS真機驗收。
本批沒有修改正式Go模擬或呈現程式。原版及衍生素材仍不進公開repo；私人畫面沿用使用者明確授權。
研究工具的輸出路徑碰撞、擷取逾時與移植常數錯誤已保留日誌並修正，沒有改成產品缺陷。
隔離Go工具明訂GOMAXPROCS配合容器CPU配額；沒有因果效能比較，效能實作依使用者順序延後。
本機驗證入口為workplace/hdtex/perf/skeleton-delivery-verification.json；素材現況仍以私人專案的
docs/hd-assets-evaluation.md §8為準。整體HD追蹤#8及GPU序章限制#12均保持開啟。

本輪GUI速度最低0.263、中位0.463，速度未過。環境採樣的主機負載25.23，CPU等待偏高，
容器節流計數為0；不是同場A/B，不據此指定原因或宣稱收益。
兩工作根無root-owned或異常.md目錄；本批命名容器含已停止者均已移除。
沿用既有映像，未清理其他專案Docker資源，未建立tag或Release。
提交／推送及Issue回讀的本機入口為workplace/hdtex/perf/skeleton-publication-verification.json。
