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

## 2026-10-19 月下序章場景表面擴充

遊戲專屬序章場景表面依既有RGBA／STP格式擴充，原有素材與兩種後端保留。
正常新遊戲來源先符合既有原生及HD基準，再以本機命令錄製重現畫面供美術試畫。
錄製僅為Go overlay，正式Go程式與本機候選執行檔不變；原始命令及衍生素材不進公開repo。
新素材由正式Go載入器確認精簡與合併逐字相同，並以獨立正常開機確認原生摘要、像素及卡片保持。
實際Intel GPU的混色、翻轉與材質視窗比對通過；此證據不替代完整GPU前端序章或速度驗收。
本機三平台候選只更新素材路徑，Linux實際新遊戲序章、最後畫面及獨立重開通過。
研究時的全零來源平凡匹配與HUD矩形誤含場景已保留紀錄並修正，不列產品缺陷。
原圖與新素材的比較保持相同場景；未分類區及未觀測範圍不稱完整HD。
完整HD、速度、人工音訊及其他平台真機仍未驗收，效能實作依使用者順序延後。
本機總入口為workplace/hdtex/perf/journey-delivery-verification.json；唯一現況表仍在私人素材專案
docs/hd-assets-evaluation.md §8，場景、怪物與地圖分別追蹤其#5／#6／#7，公開整合維持#8。

本批收尾：兩repo差異檢查通過，來源工具、候選資料及私人截圖的雜湊核對完成。
兩工作根無root-owned檔案或異常.md目錄，本輪命名容器含已停止者均無殘留。
沿用既有映像，未清理其他專案Docker資源，未建立tag或Release。
提交／推送及Issue回讀的本機入口為workplace/hdtex/perf/journey-publication-verification.json。


## 2026-10-19 月下角色衣料與完整首存來源

角色衣料及靴面沿用既有保形、保色相的局部表面方向；原創布料與皮革由內建imagegen文字生成，未提供原版圖。
正式Go載入器確認精簡及與舊素材合併逐字相同；獨立正常新遊戲的原生摘要、畫面與卡片符合既有來源。
實際Intel GPU的本批混色、翻轉及材質視窗比對通過；離線命令試畫與正式開機結果相同。
既有場景保持，原生臉部及未分類部位保持。這不是完整角色、完整HD、交叉oracle或速度驗收。

三平台本機候選只改素材路徑，保留舊包、執行檔及其他設定。Linux實際候選正常進入角色可見場景且最後畫面已呈現。
首次整包唯讀導致設定寫回警告，分類為驗證掛載問題；改用可寫設定與唯讀原版資料獨立重開，正常保存與關窗。
實際寬螢幕的框外計量圖元對應017 §3.5已知限制，未稱完整寬螢幕修復；其他平台真機及人工音訊仍待驗。
本輪速度仍未通過，沒有同場效能A/B或新優化實作。整合入口workplace/hdtex/perf/richter-delivery-verification.json。

正常新遊戲來源觀測另完成至首存，前段符合已封存基準，終點原生摘要、畫面與卡片亦符合既有M4。
研究overlay僅錄製影子命令，正式Go來源與候選執行檔保持；取樣記錄不外推為全部中途狀態的交叉oracle。
素材側整理完整路線的實際來源與色盤，排除直接色影像並保留未知分類及原檔字相同候選，供下一批場景與角色製作。
來源入口workplace/hdtex/perf/m4-material-capture-v1/verification.json及m4-material-census-v1.json。
最後來源核對入口為同目錄m4-material-observation-verification.json。
原版、衍生素材與原始命令維持本機；未修改遊戲圖只依使用者既有明確授權放私人素材repo。
唯一現況仍為私人素材專案docs/hd-assets-evaluation.md §8；角色工作#6與公開整合#8均保持開啟。


本批收尾：來源觀測、素材與候選交付核對、Python語法及兩repo差異檢查通過。
兩工作根無root-owned檔案或異常.md目錄，本輪命名容器含已停止者均無殘留。
沿用既有映像，未清理其他專案Docker資源，未建立tag或Release。
提交／推送、遠端與Issue回讀的本機入口為workplace/hdtex/perf/richter-publication-verification.json。

## 2026-10-19 月下場景表面與重用來源的色盤補齊

遊戲專屬場景表面沿用已核准的石材、金屬及布料輸入，沒有新生成或上傳原版圖。
正式Go模擬、呈現程式與既有候選執行檔未改；效能實作依使用者順序延後。
初版局部目的區沒有變化，另以有界來源觀測先重現原始畫面與統計，
確認重用圖集在新場景缺少精確色盤條目，原字有效且正確回退。
依READY補充契約增加缺少的條目，沒有調強度或改GPU來掩蓋來源問題。

正式載入器確認新素材精簡及合併後逐字保持；最終素材另以獨立正常開機驗證原生摘要、像素與卡片，
既有HD畫面保持，新表面在原預定取樣區可見，介面取樣區保持。
實際Intel GPU證據分父包素材命令及最終包新增條目；正式Go另證明父包資料在最終包保持。
沒有把繼承測試稱為全部直接測試最終包，也不把素材命令比對當成完整GPU前端或速度驗收。

本機三平台候選只改素材路徑，舊包、執行檔及其他設定保持。
Linux實際候選正常新遊戲到新場景、最後畫面及獨立顯示工作階段重開通過，無素材或後端參數。
設定可寫，原版資料與素材唯讀；兩次正常關窗均保持保存設定。
GUI沿用已批准的寬螢幕修改，不作原生摘要相同聲明，框外圖元仍屬017已知限制。
速度最低0.232、中位0.652，仍未通過；沒有同場因果效能比較。
人工音訊、其他平台真機、完整角色、完整場景與全遊戲HD仍未完成。

總覽及同狀態比較工具接入本批資料，既有兩種總覽輸出逐位元保持；
比較工具另核對來源前綴完整性，拒絕按鍵不同、原生像素不同及未到終點的收據。
研究準備時的來源檔名、依賴啟動順序、容器工作目錄及工具說明字串同步問題已修正並保留證據，
沒有列為產品缺陷。來源與HD包仍只在本機，未修改遊戲圖依既有明確授權只進私人素材repo。
公開引擎只追加本工作紀錄，唯一目前狀態為私人素材專案docs/hd-assets-evaluation.md §8。
整合收據為workplace/hdtex/perf/throne-delivery-verification.json，工具核對為throne-tools-verification.json。
提交、遠端、Issue回讀與容器清理的本機入口為throne-publication-verification.json。
整體HD項目保持開啟，下一批繼續正常玩家路線已觀測的角色素材。

本批收尾：交付、來源工具、原始截圖與兩repo差異檢查通過，原版資料及三平台既有執行檔保持。
兩工作根無root-owned檔案或異常.md目錄，本批命名容器含已停止者均無殘留。
沿用既有映像，未清理其他專案Docker資源，未建立tag或Release。
私人材質與角色Issue已更新，公開整合Issue已補一般進度，整體HD維持開啟。

## 2026-10-19 角色、頭像、武器與怪物的真正重繪標準

使用者補充要求真正提高像素與造型細節，角色、頭像、武器及怪物均適用；換色、補色或表面起伏不算完成。
先前角色衣料與怪物表面只保留為局部預覽，不再把它們當作完整角色或怪物HD的完成標準。
素材側已用內建imagegen的純文字模式製作主角及怪物外觀樣張，原版圖未作生成輸入。
樣張包含面部、服裝、全身與武器細節，均為DRAFT製作原型，未對齊動畫或裝入本機候選。
怪物的官方身份與確切造型仍未知，概念圖不作原版語意證據；需先核對實際來源再套用。
原始提示詞、圖片與雜湊只存本機素材製作區，公開repo不加入圖像或素材。
私人角色／怪物Issue已更新並新增武器專項，整體HD仍未完成。
目前候選保留已驗場景與局部表面，本輪未修改正式程式、遊戲規則、格式或候選設定。
下一步先核對已觀測角色肖像及站姿的來源與幾何，再做真正重繪的姿勢對齊及遊戲內小尺寸驗證。
驗證入口為workplace/hdtex/perf/character-redraw-prototypes-v1-verification.json，
後續提交與遠端核對入口為character-redraw-publication-verification.json。

樣張收尾：圖片、提示詞雜湊、工具語法及本機目錄連結核對通過；已驗候選素材未改。
兩工作根無root-owned檔案或異常.md目錄，本輪容器無殘留；沿用既有映像，未清理共用資源。

## 2026-10-19 第一張真正重繪頭像接入與人物輪廓實驗

依使用者補充，頭像直接採新畫稿的五官、髮束與服裝色彩，未使用原版亮度或紋理乘數。
內建imagegen只編修先前自製樣張，沒有提供原版圖；提示、母稿與父圖雜湊留在本機製作區。
本批僅一張對話頭像完成指定垂直鏈，完整人物、Richter、武器與怪物仍未完成。

正式Go載入器證實精簡與合併逐字相同；獨立正常空白卡BIOS開機至首存的232個原生摘要、
像素與卡片點符合來源，前136個既有HD點保持。父包的232張圖另從封存命令重播取得，
不稱第二次CPU開機。新頭像僅在三個抽樣對話畫面出現，其他說話者畫面保持；
文字及頭像外像素保持。實際Intel GPU的本批25組8bpp混色與變換，完整VRAM、畫面及統計相同。

三平台本機候選只更換素材路徑，既有執行檔、舊包與其他設定保持。
Linux實際候選無素材或後端參數，正常新遊戲至40000，最後畫面已呈現且新頭像可見。
獨立Weston工作階段重開至602並正常關窗，保存設定保持；原版與素材唯讀、設定可寫。
GUI沿用已批准的寬螢幕修改，不作原生摘要相同聲明。速度最低0.132、中位0.2675，仍未通過；
與較早負載不同，沒有因果A/B。人工音訊、Windows／macOS真機與完整GPU前端長路線仍待驗。

首輪正常開機及父包重播因驗證掛載含可寫研究輸入而主動停止，部分結果保留且未採納；
v2以引擎及輸入唯讀、專用輸出可寫完成。像素核對初次誤設所有畫布均512×480，
後改為逐對核對實際尺寸；對話區才使用固定框。總覽首次漏掛中文字型，依既有腳本補掛後乾淨重跑。
這些是環境及驗證工具問題，沒有因此改產品程式。

站姿DRAFT實驗找到舊低解析透明遮罩會裁掉新畫稿手腳。現有CPU／GPU均支援替換圖自己的透明輪廓，
本輪沒有修改正式引擎。單一常用色盤的新輪廓通過25組實際GPU／CPU完整VRAM、畫面及統計比較，
其中亦確認能畫在原版透明處；仍須驗遊戲實際取樣範圍、姿勢及其他色盤，沒有安裝進候選。
勘誤：該來源21組色盤只在索引6及未使用的7不同，不應稱為全圖調光或效果；索引6用途未知。
其他對話頭像來源及Richter左向母稿已整理，尚未編製或安裝，不以母稿計完成數。

本批公開repo只追加歷程。原版、衍生美術、命令與HD包留本機；未修改正常遊戲圖依既有授權只進私人素材repo。
唯一現況表仍為私人素材專案docs/hd-assets-evaluation.md §8。
指定驗證入口為workplace/hdtex/perf/alucard-portrait-validation-v2/delivery-verification.json，
工具為同目錄verify-delivery.py，後續提交與遠端核對為publication-verification.json。
私人角色Issue #6及公開整合#8保持開啟，效能實作依使用者順序延後。

本批收尾：交付、來源工具、母稿提示、私人原始截圖及文件連結核對通過。
兩工作根無root-owned檔案或異常.md目錄，本批命名容器含已停止者均無殘留。
沿用既有映像，未清理其他專案資源，未建立tag或Release；私人角色與材質項目、公開整合項目已更新。
後續繼續Richter頭像與人物逐姿勢重繪，完整HD保持未完成。

## 2026-10-20 Richter頭像近景編製與退場取樣修正

素材側按已定案的真正重繪標準製作近景頭像，直接採新畫稿的五官、髮束與服裝RGB。
v2半身母稿在原有佔位有明顯透明缺口，內建imagegen僅編修自製圖，改成近景v3；未提供原版圖。
母稿、完整提示與來源雜湊只留本機，執行素材維持小尺寸。
READY編製器保持原生透明及STP分類，正式Go確認精簡與合併全部字相同；
新增素材的25組實際Intel GPU混色、翻轉及材質視窗，完整VRAM、畫面及取樣統計與CPU相同。

首輪正常開機已到新頭像對話，文字、HUD及頭像外像素保持。
在16000的舊HD保持檢查中止，原因是驗證把最後材質查詢時間當成畫面消失時間。
封存原始畫面顯示該點仍有縮小中的同一角色頭像；這是驗證範圍假設錯誤，正式產品及素材未改。
初次工具、日誌與部分擷取保留，未當成完整路線通過。v2先封存修正後的退場範圍，
再乾淨重跑完整M4，232個原生點及終點保持要求不變，並驗區間外舊HD與其他說話者保持。
新候選尚未安裝，已驗本機預覽與舊素材路徑保持；沒有把孤立GPU或部分畫面稱為整體完成。

全身站姿另沿用既有有界研究overlay觀測末場的來源字偏移、UV與原始目的座標，
必須先重現全部232張原始HD及統計才採納。正式模擬、呈現與效能程式均未改。
本輪兩個有界工作仍在執行：正常M4 v2與站姿來源觀測；應查詢同一程序，不因觀察逾時重啟。
其他已結束的本輪命名容器由--rm清除；沿用既有映像，未清理共用資源。
原版、命令、母稿與HD包只存本機，未新增公開圖像，私人Issue #6已更新且仍開啟。
唯一現況表跨日更新為素材專案docs/hd-assets-evaluation.md §8，同時修正既有引用。
研究入口為workplace/hdtex/perf/richter-portrait-validation-v1/及richter-portrait-validation-v2/；
站姿入口為alucard-portrait-validation-v2/standing-region-research-sources.json及standing-region/。
完整HD、人物動畫、武器、其他怪物、速度與通關仍未完成。

## 2026-10-20 兩張真正重繪頭像整合與站姿材質原型

Richter近景頭像完整正常空白卡開機至45784通過，232個原生摘要、像素及卡片點相同，226個區間外舊HD點保持。
與既有阿魯卡多頭像包的正常開機畫面比較，只有14500、15500及16000的Richter頭像改變，差異分別為10976、10976與298。
文字、HUD及頭像外畫面保持，其他說話者及既有頭像保持。兩張頭像於同一最終合併包共50組直接實際Intel GPU／CPU通過，正式Go精簡與合併字保持。

早期16000退場範圍錯誤已由原始畫面訂正；40分鐘預算在35597停止，保留173原生點及167舊HD點的部分收據，不算完整驗收。
相同執行檔、輸入與檢查門檻在90分鐘有界預算完成，未修改產品或降低驗收門檻。
整合收據初版漏附已核對的素材檔案清單，補齊並保留來源及收據快照；完整像素核對結果不變。

本機三平台候選只改textures，執行檔、舊包、原版資料及其餘設定保持。
Linux實際候選無素材／後端參數，正常新遊戲至14500呈現新頭像，測試卡符合來源。
獨立Weston重開至603並正常關窗，設定保持；原版與素材唯讀、候選設定可寫。
GUI沿用核准寬螢幕，不宣稱該模式原生摘要相同。速度最低0.198、中位0.431仍未過，無跨負載因果A/B；人工音訊、其他平台真機及GPU前端完整路線待驗。
新增總覽與正常遊戲對照；四種既有總覽圖像及語意資料保持。未修改的正常遊戲圖只進私人素材repo，公開引擎不加圖片。

全身站姿v4只編修自製畫稿及骨架，沒有原版圖作生成輸入。人物RGB採新畫稿，21色盤僅在自繪內襯遮罩保留已觀測變化，材質歸屬仍為外觀強推論。
精簡前後451584個字相同；21色盤的525組CPU／實際Intel GPU完整VRAM、畫面及統計相同，各有新輪廓與實際替換證據。
原型未安裝，姿勢、UV、正常人物路線與完整動畫仍待驗，不把孤立GPU測試當成全身完成。

正常攻擊的一組圖集逐位元組對應到既有兩份武器archive中的重複內容，並以索引原字獨立重建上傳鍵。
這證實內容相同，實際CD讀取哪一份仍未知；正式武器ID、握持點及武器本體／特效分界待驗，不以算術區塊編號猜武器身份。
先前單來源UV研究在1200秒上限只到23600，保留但不採納末場證據。
後續工具已合併28人物姿勢與一組武器16場窗口，需先重現232張原始HD及統計才採納；工具與二進位均已釘選。

正式Go核心與效能路徑未改。完整HD仍未完成；唯一現況表為私人素材專案docs/hd-assets-evaluation.md §8。
本機交付驗證入口為workplace/hdtex/perf/richter-portrait-validation-v2/delivery-verification.json；工具為同目錄verify-delivery.py。
站姿與來源觀測入口為workplace/hdtex/perf/alucard-body-observation-v1/，GPU核對為standing-v4-gpu-verification.json。
原版、抽檔、命令及HD素材只留本機；效能實作依使用者順序延後，不建立tag或Release。

本批交付核對、Python語法、舊總覽相容性、原始截圖及差異檢查通過；兩工作根無root-owned或異常.md目錄。
已完成的GUI及GPU命名容器含已停止者均無殘留，沿用既有映像，未清理共用資源。
頭像GUI完成後已啟動psgolem-alucard-weapon-source-regions，3600秒、2CPU上限，輸入唯讀且僅獨立輸出可寫。
該29來源工作仍在執行，沒有完整原始圖像及統計結果前不採納UV；不可因觀察逾時另起重複工作。
輸出為alucard-body-observation-v1/replay/run/，進度與啟動釘選在其父目錄。
私人材質、角色與武器Issue已更新，公開整合項目補一般進度且保持開啟。
提交／推送與遠端回讀入口為workplace/hdtex/perf/richter-portrait-validation-v2/publication-verification.json。

## 2026-10-20 全身原型的正常對照、披風修正與來源清單補齊

原始命令完整重播通過232張畫面與取樣統計，29目標來源的68396列來源／快取字均符合原始上傳。
兩張全身原型所需的整張來源矩形都被實際取樣，新輪廓沒有超出觀測範圍。
這是原始呈現命令重播，不稱第二次CPU開機；複製命令未另觀測，不虛構完整複製鏈。
命令原字整理另保持整份雜湊、記錄數及232點完整上傳數，僅用於原字、順序與整數圖元，沒有採用略過光柵後的畫面或VRAM作證據。

全身移動母稿由內建imagegen僅參考自製站姿與骨架，v1前腳趾碰邊，v2補齊完整腳型。
編製成21色盤、176×176的DRAFT原型，正式Go精簡前後650496字相同。
與既有站姿及已安裝素材合併的201條目由正式Go核對，全部40153728字保持，原圖不轉碼，只替新檔名加前綴避免衝突。
兩個來源的42新增條目在同一研究合併包直接驗1050組CPU／實際Intel GPU，完整VRAM、畫面、統計及新輪廓相同。

正常玩家路線另以相同首存卡，各獨立真BIOS開機、選單讀檔及一般方向移動；52點原生摘要、像素及卡片相同。
6個畫面有新人物：站姿取樣3204個差異像素，移動3279；預定下半繪製區之外及上方220列保持。
真實來源查詢與圖像差異對應兩個新姿勢。兩份正常對照已生成，明示研究原型未安裝，不以單幀當作完整動畫或全部21色盤的玩家驗收。
視覺審查發現站姿披風內襯遮住腿間，與預定姿勢不合；保留舊包及證據，沒有因測試通過就裝入預覽。
內建imagegen另以自製圖及自繪骨架生成v5，將披風收回後側並恢復腿間透明。v5未編製或重驗，下一步重定披風材質遮罩並補連續動作。

第二版原字觀測補上既定人物上傳位置，取得111組來源候選、13664次上傳，全部有實際4bpp使用紀錄，84組具有常用色盤。
每張索引圖都重新構成尺寸與上傳雜湊，並提供本機原版人物總覽及逐筆上傳順序。
先前28組是便利選樣，不是完整動作清單；111也只涵蓋本次玩家路線及該上傳位置，正式動畫ID與其他玩家路線仍未知。
固定96／48場週期假說與原字序列不符，重查驗收方法後不再調參；保留原始場號與順序，不推演動畫時鐘或修改播放規則。

武器16場的取樣區與原始圖元逐場對應，四個來源矩形完整覆蓋；四組CLUT原字也在兩份archive各有九處完全匹配。
這是內容相同證據，實際CD讀取副本與正式武器ID仍未知；不以灰白線條就認定完整物理武器或添加握柄。
現行雙擊候選仍為已驗的兩張頭像包，三平台執行檔、設定、原版資料及舊包未改。
正式模擬與呈現程式未改，研究只新增非破壞性的傾印讀取accessor，於影子消費端同步讀取。
效能實作繼續延後，重複上傳只是觀測事實，沒有宣稱成本瓶頸或加速效果。

驗證器曾把沒有角色時的nil傾印列表當成必為陣列，及把母稿alpha上限254誤判為必須255；修正符合既有空資料及128二值門檻契約，原圖與像素判準不改，前版快照保留。
本機主要入口為workplace/hdtex/perf/alucard-body-observation-v1/body-normal-pixel-verification.json、body-normal-visual-review.json及body-upload-inventory-v1.json。
來源工具及驗證已掛入私人素材索引，唯一現況表仍為私人素材專案docs/hd-assets-evaluation.md §8。
公開repo不加入圖片或原始資料；新畫稿、完整提示及父圖雜湊留本機，沒有原版圖作生成輸入。

本批收尾：研究核對、原版／HD總覽連結、母稿及提示雜湊、已驗三平台素材與執行檔保持、Python語法及差異檢查通過。
私人角色與武器Issue已更新，公開效能Issue只登記後續量測且維持延後；沒有新增效能實作、tag或Release。
兩工作根無root-owned或異常.md目錄，所有本批命名容器含已停止者均無殘留，沿用既有映像，未清理共用資源。
本機檢查入口為alucard-body-observation-v1/research-checkpoint-verification.json；提交／推送及遠端回讀為同目錄publication-verification.json。
完整HD與人物動畫仍未完成，下一步先編製披風修正版並補齊實際動作來源，不重跑已完成的原始232點觀測。

## 2026-10-20 披風修正版正常驗證與新繪跑步支撐姿勢

站姿v5以新畫稿自己的透明輪廓編為112×192、21色盤，腿間透明，後側披風材質遮罩632貼素。
精簡為362303 bytes，正式Go451584個RGB555／STP字保持。
與既有159條目及移動v2合為40來源、201條目的研究包，6856890 bytes；正式Go40153728字保持。
站姿直接在最終研究包驗525組CPU／Intel GPU，完整VRAM、畫面、統計及新透明輪廓相同。
移動v2資料保持，沿用前版直接GPU證據與本版Go保持核對，不宣稱本版重驗1050組。

正常路線作一次新候選BIOS開機、選單讀檔及一般方向移動，對照封存的原版52點。
原版PNG逐張驗證原像素並保持原檔，不稱本批兩次新開機；正式核心差異只有文件。
52點原生摘要、像素與卡片相同，六點有新人物；站姿6800差異3177，移動8285仍3279且PNG保持上一版。
預定繪製區外及上方220列保持；視覺審查確認後側披風與腿間透明，僅採納孤立姿勢修正。
正常及GPU入口為alucard-body-observation-v1/body-v2-normal-pixel-verification.json、standing-v5-final-gpu-verification.json。

內建imagegen另僅參考自製移動母稿，新繪跑步支撐姿勢，沒有原版圖作生成輸入。
完整提示、父圖雜湊及PNG保存於私人素材目錄alucard-run-support-redraw-v1。
此張編為160×184、21條目的獨立DRAFT原型，774657精簡至544993 bytes，ZIP541401 bytes。
正式Go確認618240字相同；實際UV、GPU及正常玩家路線仍待驗，沒有合併安裝或改動畫時序。
先前28來源只是便利選樣，新支撐來源未含於既有取樣區觀測，不拿111來源清單代替UV證據。

私人畫廊增加修正版正常對照及新母稿，目前62個草稿入口；正式HD目錄仍0／264。
三平台雙擊預覽的38來源／159條目、執行檔及既有17張私人原始截圖均保持，所有全身素材仍DRAFT。
正式模擬、GPU及效能程式未改，優化依使用者順序延後，沒有tag或Release。
本輪核對入口為alucard-body-observation-v1/standing-v5-checkpoint-verification.json，工具audit-standing-v5.py。
唯一現況表為私人素材專案docs/hd-assets-evaluation.md §8。

本批核對、Python語法、62個草稿入口與差異檢查通過；兩工作根無root-owned檔案或異常.md目錄。
所有本批命名容器含已停止者均無殘留，沿用既有映像。私人角色Issue已更新並回讀，保持開啟。
提交與遠端回讀入口為alucard-body-observation-v1/standing-v5-publication-verification.json。

## 2026-10-20 八張新跑姿、正常移動畫面與111來源取樣區

內建imagegen只參考自製支撐姿勢，新畫收腳、換步、抬膝與前段抬腿、擺腿、落腿、騰空、伸步共八張母稿。
完整提示與父圖雜湊保留私人素材目錄alucard-run-eight-v1-prompt-set.json；沒有原版圖作生成輸入。
八張各編21色盤，正式Go精簡前後4838400字相同。連同既有跨步與支撐，已有十個相鄰跑姿小圖原型。
原版／新畫稿總覽保留來源尺寸與首次上傳順序，不稱完整動畫循環或正式動畫ID。

五個跑姿與站姿合併研究包44來源／285條目8839390 bytes，ZIP估算8574852 bytes、318檔。
正式Go42423744個RGB555／STP字保持，舊159條目與兩個先前姿勢保持。
四個新增移動來源的84色盤直接在此最終包驗2100組CPU／Intel GPU，完整VRAM、畫面、統計及新輪廓相同。
站姿及舊跨步沿用前版直接GPU與本版Go保持證據，不宣稱本版全部285條目直接GPU。
GPU研究工具依各來源尺寸處理上傳，補末半字填充；正式GPU及效能程式未改。

一次新候選正常BIOS開機、首存選單讀檔與方向移動至9000，封存原版52點的原生摘要、像素及卡片相同。
10點有HD人物差異，預定下方繪製區外及上方220列保持。原版PNG逐張核對原像素，不稱本批兩次新開機。
另取得8200至8320的121張實際候選畫面，五移動來源都有查詢；額外畫面沒有逐場封存原版，不稱121點一致。
8297有HD差異但當場沒有重新查詢目標來源，保留查詢場與顯示場的區別，不猜固定呈現延遲或動畫週期。
正常總覽與121張來源畫面的慢速審查GIF只留本機，GIF820840 bytes、4840ms，不當作原圖或速度證據。
研究驗證器原先誤要求所有開機畫面為512×480，及每張差異圖都要當場查詢，已保留前版並修正分類。
實際開機含728×480，兩側仍逐張同尺寸／像素核對；人物差異畫面維持512×480與原定區域，原生比較不變。

前段另外五個獨立DRAFT小圖已完成105條目Go精簡核對，尚未合併到285包，GPU及正常玩家路線待驗。
衣料遮罩與動作稱呼仍是外觀強推論；原版播放順序、時序及正式模擬核心沒有改動。

111人物來源的單趟有界取樣區觀測已完成，原始232張畫面與統計符合。
203967列原始來源字／快取字一致，失效列0；站姿與十個跑姿的11個新輪廓均未超出有效取樣範圍。
這是原始呈現命令重播，不是另一次CPU開機；複製命令未另觀測，111只涵蓋本次玩家路線與上傳位置。
主要入口為alucard-body-observation-v1/all-body-region-verification.json、run-five-normal-pixel-verification.json及run-five-v1-gpu-verification.json。
本批綜合核對為同目錄run-five-checkpoint-verification.json，工具audit-run-five.py；提交與遠端回讀為run-five-publication-verification.json。

私人畫廊增加八張母稿、結構與正常總覽、慢速審查，目前74個圖像／動畫草稿入口及完整提示入口。
三平台已驗38來源／159條目的素材、執行檔與素材設定保持，17張私人原始遊戲圖保持；全身新圖仍DRAFT未安裝。
正式HD檔案級目錄仍0／264，完整人物、武器、怪物及月下HD未完成。唯一現況表仍為私人素材docs/hd-assets-evaluation.md §8。

本批綜合核對、Python語法、74個圖像／動畫入口與差異檢查通過；兩工作根無root-owned或異常.md目錄。
原字觀測、GPU、正常讀檔與其他本批容器含已停止者均無殘留，沿用既有映像。
私人角色Issue已更新並回讀，保持開啟；不加入圖片、原始資料、tag或Release。


### 2026-10-20：十六個移動來源草稿與未使用色索引修正

- 依使用者真正重繪要求另畫六張阿魯卡多母稿，只參考自製圖。累計16個相鄰移動來源，仍為DRAFT，未安裝到雙擊候選。完整提示留素材區`alucard-run-compact-six-v1-prompt-set.json`；原版、圖稿與包不進公開repo。
- 研究包v1為55來源／515條目；另修正五張原圖未使用索引7卻被草稿布料遮罩指派的問題。CPU／GPU等價不代表原版材質歸屬正確。v2只使用實際索引6，材質名稱保留外觀推論，原v1資料與收據保持。
- v2為14067030 bytes，ZIP估算13753554 bytes、559檔。正式Go核對48468352字，100個非標準色盤條目共48081個RGB字只在布料區改正，標準配色、其他來源及全部透明／STP保持。六張新圖精簡125條目2857984字、五份修正版105條目2619456字另逐字保持。
- 實際Intel GPU直接核對v1新增11來源230色盤5750組，v2修正五來源105色盤另2625組；完整VRAM、畫面及取樣統計與CPU一致。其餘v2沿用直接舊測與Go保持證據，不稱8375組全部直接測v2。
- 一次v1真BIOS與正常選單讀檔／移動至9000，對照封存285條目包，共133個不同原生摘要、像素與卡片點通過。其中52點另有封存原版證據鏈；121張連續候選圖不稱逐張直接原版對拍。相對父包19個既有HD點與56張連續圖有新人物，限定區域外保持。
- 正常路線135028列取樣涵蓋16移動來源完整矩形，原字／快取字一致，新輪廓未超出範圍。v2實際查詢的17個已重繪人物條目與v1完全相同，因此沿用該次正常畫面，未另跑v2開機。查詢場不當作顯示姿勢ID，不猜動畫週期。
- 實際路線使用59個人物來源／色盤組合，17個已畫、42個仍回退原版。下一批依`normal-body-art-coverage-v1.json`補齊，完整全身、Richter、怪物與武器仍未完成。私人Issue #6已更新，武器仍由#9追蹤。
- 素材現況表與深層目錄已更新，本機畫廊83個圖像／動畫入口。16來源結構、正常對照及121來源場慢速審查GIF均只留本機，不進執行包。正式HD檔案級目錄仍0／264。
- `workplace/hdtex/perf/alucard-body-observation-v1/audit-run-sixteen.py`核對通過，總收據`run-sixteen-checkpoint-verification.json` SHA-256：adce88c2ab6e4e17e13c9c99172a6e33f018af4a287a19fc2a3704f55e5344ae。三平台既有38來源／159條目、執行檔、路徑設定及17張私人原始截圖均保持；正式模擬、GPU與效能程式未改，未重跑無關整套測試。速度、音訊、全遊戲與跨平台真機驗收仍未完成。
- 本批容器皆以非root隔離執行並在完成後移除。提交與遠端回讀收據為同研究目錄`run-sixteen-publication-verification.json`；只提交文字文件及素材畫廊入口工具。


### 2026-10-20：六個早段移動來源與白閃草稿

- 沿用使用者核准造型，內建圖像生成只參考自製母稿，再畫六張阿魯卡多早段移動來源。共22個移動來源草稿；全身仍未安裝，完整HD與動畫尚未完成。原版、母稿與HD包留本機，公開repo只記文字。
- 六張編為112×188、112×188、112×184、112×176、144×172及176×176，各22組實際色盤。精簡359164／409329／425429／382845／529726／587547 bytes，減少約26.2%至30.4%。正式Go核對新增132條目3039872字。
- 最新研究包61來源／647條目16760842 bytes，ZIP估算16418854 bytes、697檔。正式Go核對51508224字，新132條目與父515條目逐字保持，正式程式未改。
- 新白閃色盤索引0為0、其餘為0xffff，包含標準色盤透明的索引10。研究驗證器原先錯誤要求所有色盤透明範圍相同，已依原字修正；新稿白閃使用自繪輪廓，不稱原版遮罩逐像素相同。另核對57554個新稿白閃非透明貼素均為白色與STP1。首張未用索引7，只讓索引6控制自繪布料；材質名稱仍為外觀推論。
- 在同一647條目包直接核對新增六來源132色盤3300組CPU／實際Intel MTL GPU，完整VRAM、畫面及取樣統計一致。五種混色×五種變換均有獨立結果。舊來源沿用此前直接GPU與本輪Go保持證據，不稱全部647條目直接GPU、正常GPU玩家路線或速度驗收。
- 六來源既有原始取樣已覆蓋完整矩形，未重跑原始CPU或命令重播。既有正常路線59個查詢鍵與新manifest交叉核對，23個有新稿、36個回退。這是鍵覆蓋，沒有新的647條目正常開機／遊戲內對照證據。前批正常路線仍以run-sixteen-normal-pixel-verification.json為準。
- 下一批十個早段來源已定位，分開記錄傾印可用CLUT與完整觀測數；清單次序15仍需補完整UV。入口normal-body-art-coverage-v2.json、entry-six-v1-next-ten-targets.json。完整角色、Richter、怪物與武器仍待完成，私人Issue #6已更新。
- 本機新增母稿、結構與白閃檢視，畫廊91個圖像／動畫入口，正式HD檔案級目錄仍0／264。完整提示在素材區alucard-move-entry-six-v1-prompt-set.json；圖稿與比較不進執行包。現況表已移除過期60草稿及站姿v5尚未編製的舊斷言，歷史證據保留深層目錄。
- 首次Weston啟動因容器缺/tmp/.X11-unix停止，原日誌保留；補目錄後同映像、同GPU、同門檻乾淨重跑通過。非產品缺陷，沒有建立重複映像。容器非root並已完成移除。
- 彙整工具workplace/hdtex/perf/alucard-body-observation-v1/audit-entry-six-v1.py通過，entry-six-v1-checkpoint-verification.json SHA-256：28a96eb729a4b07dfc2bffebd60445e8c6724dfc6ccf23b56c1ac9e3c35edd58。三平台現有38來源／159條目、執行檔、設定與17張私人原始截圖保持，正式模擬／GPU／效能程式保持，未重跑無關全套測試。提交與遠端回讀入口entry-six-v1-publication-verification.json。


### 2026-10-20：補齊32個移動來源草稿與新正常路線

- 再畫來源清單6至15的十張阿魯卡多母稿，只用自製抬腿母稿作內建圖像生成參考。前32個移動來源都有新稿，整張人物RGB來自新畫稿，仍為DRAFT未安裝，不稱完整角色或正式動畫ID。完整提示與來源雜湊在本機素材區alucard-move-sweep-ten-v1-prompt-set.json。
- 十張編製邊長176至208像素，實際色盤數22／20／22／21／20／19／15／16／16／13，共184條目6895680個RGB555／STP字。精簡後各376012至752698 bytes，減少26.0%至32.9%；七張未使用原索引7的來源只指派索引6，前三張保留已觀測白閃，材質名稱仍為外觀推論。
- 最新研究包71來源／831條目22503118 bytes，ZIP估算22121554 bytes、891檔。正式Go逐字核對58403904字，父647條目及新增184條目保持。研究測試透過overlay加入，正式模擬、GPU與效能程式沒有改動。
- 同一831條目包直接核對新增十來源184色盤4600組CPU／實際Intel MTL GPU，完整VRAM、畫面與取樣統計一致。五種混色與五種變換均有結果；舊內容沿用先前直接GPU與本輪Go保持證據，不稱831條目全部直接GPU、正常GPU前端或速度驗收。
- 新831條目包一次真BIOS開機、正常首存選單讀檔及一般方向移動至9000，與封存515條目包的133個不同原生摘要、像素及卡片點相同。其中52點另有封存原版證據鏈；121張連續圖不稱逐張直接原版對拍。19個共同HD點與57張連續圖有新人物，預定下方區域外及上方220列保持。封存PNG逐張核對雜湊與像素，未重跑原版開機。
- 同趟480596列實際取樣涵蓋32來源完整矩形，原始字與快取字相同，新輪廓未越界。來源清單15先前在原始最後觀測場的部分覆蓋缺口已補驗，歷史證據保持。查詢場不當作顯示姿勢ID或固定動畫週期。正常觀測同樣只用Go overlay，沒有程式入口、座標或記憶體注入。
- 新正常路線仍查詢59個人物來源／色盤組合，33個有新稿、26個回退。剩餘26來源已與本次查詢核對；18組常用色盤配對不在舊M4傾印，新正常路線已觀測，原始索引及已知色盤字各以SHA-256核實。入口normal-body-art-coverage-v3.json與sweep-ten-v1-next-twenty-six-targets.json，供下一批美術，不重開已完成來源。
- 本機新增32來源總覽、12個原版人物／新稿同狀態取樣及121張實際圖組成的慢速GIF，105個畫廊圖像／動畫入口。GIF1042516 bytes、減色後94張、4840ms，只供審查，不代表原版時序或速度。大母稿與檢視圖不進執行包；正式HD檔案級目錄仍0／264。私人Issue #6與唯一現況表已更新。
- 彙整工具workplace/hdtex/perf/alucard-body-observation-v1/audit-sweep-ten-v1.py通過，sweep-ten-v1-checkpoint-verification.json SHA-256：02d7173f4c7fa3e9795e09343b6446faf9f6cccf8094e0655e147e230c7f7b76。三平台既有38來源／159條目、執行檔、設定與17張私人原始截圖保持，未重跑無關全套測試。完整全身、其他動作、Richter、武器、怪物與全遊戲HD仍待完成。
- 本批非root隔離容器均已完成並移除，原版輸入唯讀。提交與遠端回讀入口為同目錄sweep-ten-v1-publication-verification.json；只推送文件與畫廊入口工具，不推送原版或HD素材。

### 2026-10-20：阿魯卡多首存站立／移動子集接入雙擊預覽

- 依核准的真正重繪方向新增26張站立、轉身與停止母稿，只以自製圖作內建圖像生成參考。臉、髮束、肢體、衣褶與輪廓採新畫稿，RGB不依原版亮度或色盤。完整提示留私人素材區alucard-settle26-v1-prompt-set.json；來源次序不是正式動畫ID。
- 新171條目3806720個RGB555／STP字，九來源未使用原索引7，只指派實際索引6；兩來源保留白閃，新輪廓不是原版遮罩逐像素相同。18個常用配對由新正常路線、原始索引與已知色盤字SHA-256核實，舊M4傾印不變。披風材質與動作稱呼保持外觀推論。
- 最終97來源／1002條目26005121 bytes，ZIP25589710 bytes、1088檔。新增小圖最長邊200像素，母稿、比較與GIF不進執行包。正式Go逐字核對62210624字，父831條目保持；另直接核對舊159條目39051648字，場景、兩張頭像、Richter衣料與骷髏局部保持。
- 同一最終包直接核對新增26來源171色盤及舊33來源常用配對，共204配對5100組CPU／實際Intel MTL GPU。完整VRAM、畫面及統計一致，59個正常常用配對全有直接證據。其他舊色盤繼承直接舊測與本輪Go保持證據，不稱1002條目全部直接GPU、完整GPU前端或速度驗收。
- 一次新1002條目包真BIOS開機，正常首存選單讀檔與一般移動至9000。同機器命令同步餵入原人物159條目與新包，316組HD對照中305組有人物差異，限定人物區外及上方220列保持。133個封存原生摘要、像素及卡片點相同，另183點沒有獨立原生對照。沒有程式入口、座標或記憶體注入，正式模擬、GPU及效能程式未改。
- 正常取樣706756列覆蓋59來源完整矩形，原字與快取字相同，新稿未越界。此路線59個人物配對全命中、回退0，非全遊戲清單。清單109／110首次查詢取樣近黑，同場6495僅8個差異像素，只證明轉場與查詢；全59常用配對另有孤立GPU可見性。查詢場不當作顯示姿勢ID。
- 私人素材目錄的首存子集經證據及視覺審查升為READY，使用既有install_hd_preview.py新增--alucard-body-route。已裝入v.0.7.1三平台候選，選castle-alucard-route-v1-compact；只變更素材相對路徑，備份assembly與設定並保留舊雙頭像包。三平台執行檔、原版媒體、其他設定逐項保持，ZIP逐檔及SHA核對通過。
- Linux實際候選用保存的CPU、HD2、PGXP、xBR、寬螢幕及透視設定，無素材或後端參數，正常首存讀檔與移動至9000並呈現最後畫面；獨立重開至至少600正常關窗，設定保持。GUI寬螢幕不作原生狀態相同聲明。首次單檔設定掛載阻擋原子存檔，保留部分紀錄後修正掛載，以相同映像與門檻乾淨重跑；屬驗證環境問題。
- 本機畫廊137個圖像／動畫入口，新增26母稿、結構、59來源總覽、站立及停止對照。182張實際畫面GIF僅供慢速審查，不代表遊戲速度。正式HD檔案級目錄仍0／264；完整角色、其他動作、Richter全身、武器、怪物、速度、人工音訊與通關仍待完成。Windows／macOS僅同步資料，未宣稱真機驗收。
- 研究入口workplace/hdtex/perf/alucard-body-observation-v1/settle26-v1-checkpoint-verification.json、settle26-normal-pixel-verification.json及gpu-settle26-v1-verification.json。安裝、視窗與最終核對在私人素材專案workplace/assets/hd-alucard-body-route-preview-install/delivery-verification.json；唯一現況表仍為該專案docs/hd-assets-evaluation.md §8，角色Issue #6保持開啟。
- 只推送文件與素材工具，不加入原版、美術、包或新截圖，既有17張私人原始截圖保持。所有本批容器含已停止者均已移除，工作根無root-owned或異常.md目錄；沿用既有映像，未清理共用資源。提交與遠端回讀收據為同研究目錄settle26-v1-publication-verification.json。

### 2026-10-20 Richter全身39來源重繪子集

- 承接已核准的真正重繪方向，內建圖像生成只以自製Richter樣張或同批母稿作參考，新畫五官、髮束、肢體、衣褶及輪廓。39張單姿勢母稿中兩張原v1觸邊，以同工具另畫v2補足靴尖及手部，37張v1保持；原稿與未完成編製保留。來源動作名稱為外觀推論，不稱正式動畫ID。完整提示與圖像雜湊只留私人素材目錄。
- 原始M4傾印的常用色盤有41來源，一張完全透明、一張128×128多姿勢圖集未解，其餘39單姿勢本批完成。39常用加37已觀測白閃共76條目，四倍小圖最長邊180像素；1098303精簡至679892 bytes，正式Go核對1282752個RGB555／STP字保持。合併127來源／1069條目26660516 bytes，正式Go63349632字；九個舊衣料條目由新全身稿取代，其餘993條目逐字保持。
- 同一1069條目包直接通過76配對1900組CPU／實際Intel MTL GPU案例，涵蓋五種混色與五種變換，完整VRAM、畫面及取樣統計一致。舊993條目沿用先前直接GPU與本輪Go保持證據，不稱本批全1069條目直接GPU、完整GPU前端或速度驗收。
- 原始M4命令僅重播必要區間至20700，99個封存畫面與統計保持，137172列原字／快取觀測覆蓋39完整來源矩形。另一次真BIOS、工廠空白卡與原M4按鍵正常新遊戲至20700，同機器同步舊1002／新1069包共428組HD對照；99點另對封存原生摘要、像素與卡片一致，329點沒有獨立原生對照。預定活動區外及三個HUD區保持，正常取樣再得137172列完整來源核對，無入口、座標或RAM注入。
- 正常路線91人物配對中72命中新稿，含39常用及33白閃；19特殊效果回退原版。另四白閃只有直接GPU證據，多姿勢圖集未解，完整Richter及其他動作仍未完成。查詢場不是顯示姿勢ID，不猜補機制或動畫時序。研究與視覺證據審查後，私人素材目錄僅將此子集升為READY，才新增既有安裝工具的--richter-body-route。
- 已裝入既有v.0.7.1三平台本機候選，選castle-richter-body39-v2-compact，只改素材相對路徑，備份assembly及設定；舊阿魯卡多人物與雙頭像包保留。三平台執行檔、原版媒體、其他畫質及手把設定保持，1185檔執行ZIP26241200 bytes逐檔與SHA-256核對通過。母稿、提示、比較、GIF與原版資料不進執行ZIP。
- Linux實際候選從非專案目錄啟動，無素材／後端參數，沿保存的CPU、HD2、PGXP、xBR、寬螢幕及透視設定，正常新遊戲至11724並呈現1280×720最後畫面；卡片與封存原版同場一致。獨立重開至603正常關窗，設定保持。GUI寬螢幕不作原生狀態一致聲明，017既知右側圖元仍在；速度最低0.348、中位0.824未通過，沒有同負載新舊因果比較，未開效能優化。
- 本機畫廊182個圖像／動畫入口，含39母稿、完整127來源總覽、走動及出招實際比較。走動GIF由66實際場每來源40ms製作，精簡1078321 bytes，只作慢速審查，不代表遊戲速度。出招的鞭子仍原版，未稱武器重繪完成。正式HD檔案級目錄仍0／264；完整角色、圖集、特殊效果、武器、怪物、全遊戲HD、速度、人工音訊及通關仍待完成。Windows／macOS僅資料同步，未宣稱真機驗收。
- 研究入口workplace/hdtex/perf/richter-body-redraw-v1/下richter-body39-v2-checkpoint-verification.json、richter-normal-v2-pixel-verification.json、richter-body39-v2-go-verification.json及richter-body39-v2-offline-verification.json。私人素材專案workplace/assets/hd-richter-body39-preview-install/delivery-verification.json保存安裝、視窗及交付核對；唯一現況仍該專案docs/hd-assets-evaluation.md §8，角色Issue #6保持開啟。
- 本批正式模擬、GPU與效能程式未改，只推送文件與素材工具，不加入原版、抽檔、美術、包或新截圖；既有17張私人截圖保持。批次容器與擁有權檢查記於本機發布回讀收據richter-body39-v2-publication-verification.json，不清理其他專案資源。

### 2026-10-20 中文字HD原型與保存1080p輸出

- 使用者要求中文字也做HD，並明確要求繪製層突破PS1解析度、至少1920×1080，確認沿用目前美術。路由載入字模索引還原及復古驗收，沿用既有素材替換018／019；未重新改角色美術、翻譯或模擬規則。
- 原M4傾印找到234個3／4字寬、16列、4bpp候選，含英文、圖示與空白，非完整中文字庫。field14500對話24字位的8852個非透明HD2像素與22來源完全相同，索引重建的上傳雜湊相同。Unicode字名依逐字判讀，完整字碼及光碟字庫仍未知。十樣本未在小於4MiB的抽出檔直接找到4bpp序列，不推論壓縮格式。
- 以既有Noto Sans CJK TC／OFL-1.1快照做22字原型，12×16格提升為96×128八倍索引PNG，沿觀測灰階索引1–8，原字位及換行保持。23檔含manifest共22186 bytes，不附完整字型；正式Go載入接受22條目。1920×1080比較是字形研究畫布，非正常遊戲截圖；字形仍DRAFT，未完成正常遊戲、CPU／實際GPU、淡入淡出、STP或全字庫驗收，未裝入候選。
- 檢查現行016已支援1／2／4／8倍光柵化，019亦接受八倍素材。先新增012 §11的READY契約，再實作保存的-resolution 1920x1080；settings.json保存resolution，無參數重開沿用，auto清除。命令列優先、尺寸範圍與錯誤先驗，不改HD倍率、PGXP、後端、寬螢幕或遊戲狀態。新檔cmd/psgolem/resolution.go由012 §11索引，前端只設定起始視窗大小。
- 命令列與前端套件測試通過，新增優先序、保存、清除、非法尺寸及1080p的4:3／16:9安全顯示檢查。程式在既有Docker映像建置，未改機器、CPU、原生GPU、光柵化或呈現排程。
- 實際Intel MTL顯示環境三次正常BIOS開機至706，HD2首次、無resolution參數獨立重開與HD8都呈現1920×1080最後畫面。HD2兩張PNG逐位元相同，設定保持；HD8輸出有不同的較細邊緣。此輪HD光柵化使用CPU，Intel GPU負責OpenGL顯示，不稱HD GPU後端驗收。速度窗口依序0.480、0.783、0.107，非固定負載交錯A/B，未通過速度，不把功能存在當作加速完成。
- 舊三平台本機候選、原版媒體、127來源／1069條目人物包及設定均未更換。1080p正常遊戲、高解析中文字整合、完整字庫、三平台新包、速度與人工音訊仍待完成；效能優化保持延後。中文專項為私人素材repo #10，1080p為本repo #13，皆保持開啟。
- 唯一現況仍私人素材repo docs/hd-assets-evaluation.md §8，細節及研究工具索引在docs/assets-catalog.md中文字HD節。研究收據為該repo workplace/assets/chinese-font-hd-v1/display1080-font-checkpoint-verification.json，逐字來源、字形原型、正式載入及三次視窗日誌均留本機。只提交文字與Go程式，不新增原版、字模、美術或截圖到Git／Issue。Docker清理與擁有權檢查另記同研究目錄發布收據。

### 2026-10-20 中文字22字實際對話與1080p候選

- 使用者明確指示效能調校延後。本輪沿既有22字原型接入018／019，沒有改效能、CPU、原生GPU或HD光柵路徑。Noto Sans CJK TC八倍索引字圖保留原譯文、字位、換行及遊戲CLUT；其他字回退原版，Unicode名稱為逐字判讀，完整字碼仍未知。
- 合併149來源／1091條目，含1069全彩及22索引條目；索引條目不固定CLUT，不把1091全稱來源／色盤配對。正式Go逐字核對原1069條目63349632個RGB555／STP字保持，新增22字270336索引精確。字形23檔含獨立manifest22186 bytes，不附完整字型；合併26682664 bytes，執行ZIP26261475 bytes、1207檔。
- 最終同一合併包22字各25原CLUT混色／變換共550例，加330受控STP、混合STP與全零CLUT，第一字再加HD8與未知來源回退，共882個CPU／實際Intel MTL GPU案例。完整VRAM、畫面與統計一致。HD8僅一字隔離抽樣，不稱全部22字HD8或完整GPU前端、速度驗收。
- 一次真BIOS、工廠空白卡及原M4按鍵正常新遊戲至15000，寬螢幕關閉，同機器同步1069與1091條目影子，221組HD2對照中56個封存原生摘要、像素與卡片點相同，165點沒有獨立原生對照。差異限制在預定文字區，文字外0。27648列來源／快取涵蓋完整22個12×16矩形，原字與快取一致，新字形未越界。無入口、座標或RAM注入。原型視覺與上述證據審查通過，私人素材規格僅此22字子集升READY後才安裝。
- 程式來源8de152e由既有tools/build_local.sh重建全新v.0.7.2-20261020七個乾淨產物，版號、CRC、SHA、素材排除與Noto OFL附件通過；macOS兩架構、minos11.0與系統相依結構檢查通過，未稱真機。沒有新Release／tag，原版與HD包不公開。
- 新三平台含素材候選沿005 §9組裝，私人素材工具install_hd_font_preview.py繼承v.0.7.1的畫質及手把設定，新增1920×1080輸出與新素材路徑，HD仍獨立沿2倍。原版媒體逐項保持，舊v.0.7.1的執行檔、1069條目包及設定保持。首次安裝守門對Windows／macOS省略媒體欄位停止，尚未寫入；依既有唯一候選自動尋找契約修正後成功重跑，不猜補媒體。初次GUI外層命令的環境變數替換錯誤在程式啟動前停止，修正後乾淨執行，非產品缺陷。
- Linux使用新候選執行檔、其他cwd、無素材／解析度／後端參數，保存1920×1080、HD2、CPU、PGXP、xBR、寬螢幕、透視及手把設定。正常新遊戲至14500並呈現最後HD中文字畫面，卡片與封存原版同場一致；獨立重開至608正常關窗，設定保持。GUI寬螢幕不宣稱原生狀態相同，017既知右側計量圖元仍在。速度僅記原始日誌，未驗收或調校；人工音訊、Windows／macOS真機與通關仍待完成。
- 私人畫廊184個圖像／動畫入口，新增實際對話原字／新字比較及22字總覽；既有17張私人截圖保持。正式HD檔案級目錄仍0／264，完整中文字庫、其他人物動作、武器、怪物與全遊戲HD未完成。中文Issue為私人素材repo #10，1080p為本repo #13，皆保留開啟。
- 唯一現況仍私人素材repo docs/hd-assets-evaluation.md §8，研究入口workplace/hdtex/perf/chinese-font22-v1/下font22-ready-verification.json、font22-normal-GPU-verification.json及normal-player-output/verification.json。部署為私人素材repo workplace/assets/hd-font22-1080-preview-install/delivery-verification.json；發布回讀及Docker／擁有權核對另記font22-publication-verification.json。只提交工具與文件，不加入原版、字模、美術或新截圖。


## 2026-10-20：44字子集與血條／蠟燭來源

使用者要求字形、血條、姓名、裝飾與遊戲內蠟燭都HD；效能調校繼續延後。唯一現況表仍是素材repo docs/hd-assets-evaluation.md §8。
新增22個對話字源，逐字對field14000的9528個非透明HD2像素與原始上傳雜湊，累計44字。原文、字位、換行與顯示時序保持，完整字碼格式未知。
最終1113條目包正式Go保持1091舊條目，含63349632全彩字與270336舊字索引，新增270336索引精確。
新增22字在最終包直接通過882組CPU／實際Intel MTL GPU案例；舊22字繼承先前直接GPU及本批逐字保持，非44字都以本批包直接GPU。

正常真BIOS、工廠空白卡及原M4按鍵到14500，301組同機器HD對照、55個封存原生點一致，另外246點無獨立原生對照，文字區外0。
77320列原字／快取相同，其中38016列完整覆蓋44字來源；另39304列核對血條與蠟燭，目的座標為原生GPU VRAM，含雙緩衝偏移。
入口workplace/hdtex/perf/chinese-font44-v2/font44-normal-GPU-verification.json及font44-ready-verification.json。正式模擬、HD光柵與效能程式未改，沿既有8de152e候選執行檔。

同一候選與新包在獨立試用設定根目錄正常新遊戲至14000，1920×1080最後畫面呈現，卡片與封存同場一致。GUI寬螢幕不作原生狀態相同聲明；內部沿HD2及CPU。
44字包裝入現有v.0.7.2三平台本機預覽，171來源／1113條目、26704954 bytes；ZIP26281868 bytes、1229檔。只有素材設定改變，執行檔、原版、舊22字包與其餘設定保持。
真正Linux候選另零素材／後端參數重開至606，WM_DELETE正常離開，設定保持。Windows／macOS僅資料與結構，沒有真機驗收。部署入口為私人素材repo workplace/assets/hd-font44-preview-install/delivery-verification.json。

血條圖集原始8192 bytes與F_GAME2.BIN的檔案byte偏移0x18000相同；共用小字圖與三份拆檔0x30000相同，實際讀取副本未知。
蠟燭128×128圖集索引與上傳鍵精確，兩組原觀測CLUT；六場同場取得2016來源像素及一組當場CLUT。圖集也含雕像，不猜整張身份或完整火焰週期。
蠟燭透明母稿v1只輸入文字，v2只參考自製v1，未上傳原版；兩張母稿未安裝，輪廓、錨點、透明量化與動畫待驗。
物件、介面與字形追蹤私人素材repo #12、#11、#10，本機畫廊191個入口，正式檔案級HD仍0／264。完整字庫、完整HD、速度、人工音訊與通關未完成。

第一趟GPU驗證選到沒有Weston的Go建置映像，在啟動前停止；保留環境失敗log，改回原有psgolem-wine-verify:noble-r2後同一程式與門檻通過，未另建映像。
本輪只提交文字與研究工具；圖片、字圖、原版與執行包留本機。commit／push與Docker清理以本輪最終收據核對。


## 2026-10-20：站立燭台v2正常色盤子集

接續已核准美術，先做序章四個站立燭台，完整蠟燭及破壞／掉落未完成。唯一現況表仍為素材repo docs/hd-assets-evaluation.md §8，工作追蹤其 #12。
從封存正常命令重播觀測13950～14050，55張父包畫面與既有正常44字路線相同；取得898筆原始CB、404筆位置／UV變體及175472列原字。這是離線命令重播，不是新CPU執行或原版硬體對拍。
原圖24..48列會被重用為延長燭桿。首個整體縮放草稿會把枝架帶入此段，未安裝；v2分枝架、可重用桿段、下方裝飾與底座。保留舊稿與收據，未調整遊戲座標、時序或原生狀態。

自製母稿v3只參考自製v2，無原版圖片輸入，335264 bytes。編製RGB直接取新稿，不用舊RGB、亮度或色盤乘數，alpha量化0／255，兩個不可表示的不透明黑補8／8／8，STP檔保持。
只改正常CLUT的一個既有全彩條目，選定2016來源像素；5689個RGB555字變更、763新不透明子像素。正式Go核對修改條目262144字精確、區外229888字保持，其餘1112條目63087488全彩字及44字540672索引保持。
最終包兩組實際CLUT共53個CPU／Intel MTL GPU案例通過，含50HD2混色／變換及HD8／未知CLUT／未知上傳回退。完整VRAM、畫面與取樣統計相同，非GPU前端完整玩家路線或速度證據。

正常真BIOS、工廠空白卡及原M4按鍵至14500，160組同機器HD對照、55個封存原生摘要／像素／卡片點一致，另外105點無獨立原生對照。
898筆原始CB、UV、位置與切換序列相同；226197列原字／快取相同，完整2016來源像素覆蓋，燭台圖元外差異0。
入口workplace/hdtex/perf/candle-redraw-v1/candle-v2-normal-GPU-verification.json及candle-v2-ready-verification.json。原生模擬、HD光柵及效能程式未改，沿既有8de152e候選執行檔。

同一候選與新包在獨立試用根目錄正常至14000，1920×1080最後畫面呈現，卡片與封存同場一致。GUI寬螢幕不作原生狀態相同聲明，內部沿HD2及CPU。
已裝入既有v.0.7.2三平台本機預覽，171來源／1113條目26728515 bytes，比父包增加23561 bytes；ZIP26305526 bytes、1229檔。
只有素材設定改變，舊44字包、執行檔、原版、手把及其他設定保持；真正Linux候選零素材／後端參數重開至603，WM_DELETE正常離開，設定保持。Windows／macOS限資料與結構，沒有真機驗收。
部署收據為私人素材repo workplace/assets/hd-candle-standing-preview-install/delivery-verification.json。

本機畫廊195個入口，原版／前批／新稿總覽及正常同場比較均保存；101場原始PNG保持，畫廊GIF取26張、333334 bytes供慢速審查，不作原版時序、像素或速度證據。
圖片、母稿與執行包只留本機，17張既有私人截圖保持。正式HD檔案級目錄仍0／264，完整人物、字庫、血條姓名、所有蠟燭、武器、怪物、全遊戲HD及速度仍未完成，效能調校延後。
本輪commit／push與容器清理以私人素材repo workplace/assets/candle-redraw-v1/publication-verification.json核對。


## 2026-10-20：Richter序章HUD八倍索引子集

沿使用者核准美術及先HD後效能的順序，完成序章Richter血條外框、數字底框、捲草、RICHTER／REST／×00、十個數字、短橫與PAUSE PUSH START重繪。原文、字位、數值、狀態與原生模擬不改，其他主角、日文對話姓名、圖示及完整介面未完成。唯一現況仍私人素材repo docs/hd-assets-evaluation.md §8，工作為其 #11。

來源128×128、4bpp圖集與F_GAME2.BIN檔案byte偏移0x18000之8192 bytes相同，上傳鍵6326873f58316d3b8076e611389a1e43428491b370b8320d6734c08af4708883。舊正常原始命令103場觀測615筆HUD圖元、603307列原字／快取、7908來源像素及12組CLUT。Pillow幾何路徑新畫邊框與捲草，Noto Sans CJK TC重新製作字形，沒有圖像生成。八倍索引1024×1024小圖10266 bytes，未分類圖示原樣放大，血量填色來源保持。4244來源像素內變更73390子像素索引，透明輪廓有意更新；顏色、透明與STP沿實際CLUT原字，不將索引0固定透明。

正式Go新1048576索引精確，父1113條目63349632全彩字及44中文字540672索引保持。待提交工具由乾淨來源重製1230包內檔案逐位元組相同。同一1114條目包378組CPU／實際Intel MTL GPU通過，含300實際色盤混色／變換、75受控STP0／混合STP／全零色盤，另HD8、新CLUT索引映射與未知上傳回退3例。非完整GPU前端或速度驗收。

一次真BIOS、工廠空白卡及原M4按鍵至24400，234組同機器雙影子畫面，136封存原生摘要／像素／卡片點相同，98點無獨立原生對照；615筆原始CB及來源／目的／色盤／偏移保持，603307列來源字與快取相同。原M4直接顯示數字0、1、3；其他字有來源字形與直接GPU證據，不稱所有數值都經正常路線。

最後24400場把畫面當貼圖旋轉，原直接HUD邊界檢查停止。原GP0解析器保留封包分組及PGXP精度、跳過光柵操作，取得6431個轉場封包，以五個原HUD封包作正對照。24322快照原樣平移至VRAM Y+256，24398同深度四邊形由實際UV起點12列映射至顯示緩衝。按原頂點與兩輸出像素邊界核對，差異限投影後HUD；其餘233場亦限直接HUD，沒有略過轉場或放寬成整個畫面。此解析不是新CPU或硬體oracle，研究入口workplace/hdtex/perf/hud-redraw-v1/transition24400-verification.json。

同一8de152e候選執行檔與新包，在私人素材repo獨立試用根目錄正常新遊戲，額外20000以START暫停，20020最後1920×1080畫面呈現新暫停字形。內部HD2及CPU，Intel GPU負責OpenGL顯示；PGXP、xBR、透視、寬螢幕與手把沿原設定，GUI因寬螢幕與額外輸入不作原生同場聲明。正式程式及效能路徑不變。

子集升READY後已裝入既有v.0.7.2三平台本機預覽，172來源／1114條目，含1069全彩及45索引，44字保持；26739046 bytes，比燭台包增加10531 bytes，ZIP26314662 bytes、1230檔。只有素材路徑改為castle-hud-richter-v2-compact，舊燭台包、執行檔、原版、Noto OFL與其他設定保持。
真正Linux候選無素材／後端參數另重開至606，WM_DELETE正常關窗，設定保持，已呈現451幅；最後在途452未呈現，重開只驗啟動、顯示與保存，上述遊戲20020定場為final_drawn=true。Windows／macOS限資料與結構，未驗真機。

圖庫197個圖像／動畫入口，新增原版／HD圖集及正常20000同場比較；既有17張私人截圖保持，正式HD檔案級目錄仍0／264。另以15000及15500封存對話逐字核實八個新字源，仍未接入現行44字包，完整編碼未知。完整HUD、人物、字庫、武器、怪物、蠟燭破壞、全遊戲HD與速度仍未完成，效能調校延後。

研究掛載缺少父包／字型及覆蓋檔遺漏顯示原點方法，均在產品驗證前修正；同一工具鏈與原門檻乾淨重跑。完整研究命令在workplace/hdtex/perf/hud-redraw-v1/commands.txt，正常、GPU及READY入口為hud-v2-normal-GPU-verification.json與hud-v2-ready-verification.json。部署、獨立重開與完整核對在私人素材repo workplace/assets/hd-hud-richter-preview-install/delivery-verification.json及reopen-closure-verification.json。只提交文字與素材工具；圖片、原版及包留本機。提交、遠端回讀與Docker／擁有權核對另記同素材repo workplace/assets/hud-redraw-v1/publication-verification.json。


## 2026-10-20：對話99字與四角色原文姓名

沿使用者已核准美術、字型及先HD後效能順序，新增55個已核實中文字／標點，原44字一併重製，共99個字源；德古拉、Richter、DEATH、阿魯卡多的原文姓名亦重新製作，沒有改譯、改字位或逐字時序。唯一現況仍私人素材repo docs/hd-assets-evaluation.md §8，工作為其 #10／#11及本repo #13。正式模擬、GPU光柵與效能程式均未改。

姓名共享128×128、4bpp圖集b4214b179ca94941c27bca1b5208089550e2295559e2728424de874ace435809，與F_GAME.BIN／F_GAME2.BIN／F_TITLE1.BIN檔案byte偏移0x30000的8192 bytes相同。24個8×8基底格與兩個8×4濁音標記涵蓋1600來源像素，35個基底與7個標記的42筆圖元位置保持。相同ヘ字模由原始CB來源消歧，完整Unicode字表仍未知。共用格在其他介面的重用亦會顯示新字形，不聲稱只影響這四列。

v3誤用字寬與字型側距，濁音及標點過小，視覺審查拒絕，未安裝；其GPU2243例不能替代視覺品質。正常路線14000後主動停止，退出143，GUI於11616正常關窗，均不列為完整驗收。v4先裁實際非零墨跡再置入原框，99字及姓名統一重新製作；原44字全部實際變更。未核實辺／边、說／説、里／裏保持原版，不猜補。Noto Sans CJK TC／OFL，沒有圖像生成或完整字型打包。

正式Go核對100個本批索引條目2265088索引精確，姓名區外946176索引保持。父1070個全彩／HUD條目63349632字與1048576 HUD索引保持。乾淨唯讀輸入重製1286檔逐位元組相同。最終1170條目包直接CPU／Intel MTL GPU共4003例，含100實際來源／CLUT配對2500混色／變換、1500受控STP1／混合STP／零CLUT及首字HD8、新CLUT映射、未知來源3例；兩種實際CLUT，不稱100個不同色盤。完整VRAM、畫面及統計相同，不稱完整GPU前端或速度完成。

一次真BIOS、工廠空白卡及原M4按鍵正常至首次存檔45784，同機器父1114／新1170包265組HD2對照，232封存原生摘要／像素／卡片點一致，33點無獨立原生對照。正常212928列原字／快取相同，99個12×16完整字源19008像素及姓名1600來源像素覆蓋，42筆姓名圖元位置保持。文字圖元外差異0，寬螢幕關閉，無狀態或座標注入。

同一8de152e候選執行檔與新包，在獨立試用根目錄以原M4按鍵正常新遊戲至40600，1920×1080最後畫面呈現死神姓名與三行對話。內部HD2、CPU，Intel GPU顯示；PGXP、xBR、透視、寬螢幕、手把及其他設定保持，卡片與封存同場一致。GUI寬螢幕不作原生狀態同場聲明。

READY後裝入既有v.0.7.2三平台本機預覽，228來源／1170條目，1069全彩與101索引。26810838 bytes，比HUD父包增加71792 bytes；ZIP26382447 bytes、1286檔。真正Linux候選無素材／後端參數獨立重開至606並WM_DELETE正常關窗，已呈現511幅，素材與其他設定保持。關窗最後在途是否呈現另見收據，不把重開當40600定場驗收。舊HUD包、執行檔、原版與OFL保持；Windows／macOS限資料與結構，未驗真機。

本機畫廊201入口，新增99字／四姓名來源總覽與14500／40600前版／本批同場比較。既有17張私人截圖保持，正式檔案級HD仍0／264，圖片、字圖與包不進Git或Issue。正常驗證首度將濁音8×4製圖子矩形當作完整8×8 GPU圖元而停止；原始CB證實上方四列透明，以來源／目的同樣上移四列修正驗證，素材、正常執行及差異範圍不改，重跑原門檻通過，入口name-mark-packet-correction.json。順帶排除F_GAME.BIN偏移0x18000之HUD假說：8192 bytes全零且跨位深共用，匹配上傳鍵不能辨識介面，沒有依該偏移實作。完整字庫、其他姓名、主角HUD、人物、怪物、武器、地圖與蠟燭仍未完成，效能調校延後。

正常、GPU、原型拒絕與READY證據在workplace/hdtex/perf/dialogue-font-v3/及私人素材repo同名研究區，入口dialogue-v4-normal-GPU-verification.json、dialogue-v4-ready-verification.json、Go-GPU-v4-verification.json、reproducibility-v4-verification.json及gui-v4-play/verification.json。部署及重開在私人素材repo workplace/assets/hd-dialogue99-preview-install/delivery-verification.json，提交與遠端回讀另記研究區publication-verification.json。只提交文字、畫廊入口及編製／安裝工具；原始素材與研究圖留本機。


## 2026-10-20：阿魯卡多HP圓框、飾框與數字子集

沿使用者已核准美術、先HD後效能的順序，完成HP圓框、HP字樣、長條金屬飾框與兩組0～9字形子集。值、位置、輸入及時序不改，完整介面與全遊戲HD仍未完成。唯一現況為私人素材repo docs/hd-assets-evaluation.md §8，工作為其 #11及本repo #13，正式模擬、光柵與效能程式未改。

先以既有M4原命令完整重播至45784，232個畫面與前批1170包相同，取得1724筆圖元及378615列原字／快取。128×128／4bpp圖集697d61bed2c6be9943c3ca51b816b425d225a9e0d4dd9b6eb899a53ef6bcd247的8192 bytes，與F_GAME.BIN及F_GAME2.BIN檔案byte0x38000相同；實際讀取副本未知。該圖集162筆CB及84808來源列成立，0x3d為12字帶貼圖四邊形。填色另用8×50圖條1bf70e5f297d4ed758be1c7d53d7712a654fc3613b6f49693d4a7a360818bb0c，觀測時原生VRAM988,256；一個填色四邊形也會取鄰接原圖集邊界，不能只按第一個來源分類。

Pillow曲線重畫金屬斜面、圓框、鉚釘及葉形捲草，Noto Sans CJK TC／OFL按實際墨跡重製HP與數字，沒有圖像生成或完整字型打包。v1飾框過細、舊內圈仍露出，未安裝；v2以錯誤的小數字前景索引1取代6，實際圖元與CLUT預覽揭露低對比，未安裝。v3用前景6／陰影1修正。取樣預覽的第三組也曾選錯顯示緩衝，改看該組真正繪圖緩衝後只作審查，不冒稱正常顯示frame證據。研究建置的掛載引號、四邊形封包長度及測試檔案變數撞名均在正式驗收前修正，正式程式未改。

八倍1024×1024索引PNG15889 bytes，4640來源像素內變更81508索引；未選區751616、飾框保護通道31616、圓框心形1280索引精確保持。正式Go新1048576索引精確，父1170條目63349632全彩字及3313664索引保持，含99中文字、姓名與Richter HUD。待提交工具增加原PNG及上傳鍵、父包雜湊核對後，乾淨唯讀輸入重建1287檔逐位元組相同。

同一最終1171包728個CPU／Intel MTL GPU案例通過：26實際CLUT共650混色／變換、75 STP1／混合STP／零CLUT控制，另HD8、新CLUT映射與未知來源3例，完整VRAM／畫面／統計相同。不是完整GPU前端或速度。
一次真BIOS、原首存卡及原有正常讀檔／一般移動腳本至9000，共133組同機器父1170／新1171 HD2對照，全部133個封存原生摘要／像素／卡片點一致，無額外未封存點。505600列原字／快取相同，圓框及飾框2720來源像素完整覆蓋；1120筆原始CB保持，圖元加兩個HD輸出像素邊界外0。正常只覆蓋大數字0、7與小數字0、1，其他字有來源與孤立GPU證據，沒有外推所有數值。

同一8de152e候選執行檔與新包，獨立試用設定按同一正常腳本至9000，1920×1080最後畫面清楚呈現70／10及新框；HD2 CPU光柵、Intel GPU顯示，PGXP、xBR、透視、寬螢幕與手把保持，卡片與封存同場一致。GUI因寬螢幕不作原生同場聲明。
READY後已裝入既有v.0.7.2三平台本機預覽，229來源／1171條目，1069全彩與102索引。26826992 bytes，比99字姓名包增加16154 bytes；ZIP26396798 bytes、1287檔。只有素材路徑改為castle-alucard-hud-v3-compact，舊包、執行檔、原版、OFL及其他設定保持。真正Linux候選無素材／後端參數獨立重開604並WM_DELETE正常關窗，已呈現523幅，末尾在途524未呈現，重開只驗啟動／保存；上述9000定場final_drawn=true。Windows／macOS限資料與結構，未驗真機。

本機畫廊203入口，新增原版／HD圖集及正常9000比較；既有17張私人截圖保持，正式檔案級HD仍0／264。完整HUD、字庫、人物、怪物、武器、地圖、蠟燭及全遊戲HD未完成，效能調校延後。正常／GPU／READY與原始指令在workplace/hdtex/perf/alucard-hud-redraw-v1/，來源、原型拒絕、重製及實際GUI在私人素材repo同名研究區。交付為其workplace/assets/hd-alucard-hud-preview-install/delivery-verification.json，提交及遠端回讀另記研究區publication-verification.json。只提交文字與工具，圖片、原版與包留本機。

## 2026-10-20：揮擊四塊來源重繪與壁掛藍火來源定位

沿已授權美術與先HD後效能的順序，完成一組已觀測攻擊圖集的四塊顯示來源。完整武器、角色、怪物、字庫、介面、蠟燭、地圖與HD未完成；唯一現況仍為私人素材repo docs/hd-assets-evaluation.md §8。工作為其 #9／#12與本repo #8／#13，正式模擬、光柵及效能程式未改。

來源02df79f736fe7f80530ea62db5725279eddbfd92b1d5cdcf8f85234733b81fc9為256×128、4bpp。既有原始trace及archive原字證據保持，四塊共3081來源像素，四組CLUT、原始0x3f及ABR1保持。灰白細條屬刀身或特效、正式武器ID及實際CD副本仍未知，不新增握柄。Pillow貝茲曲線重新畫收尖亮線、主光流與衰減弧線，沒有換色或圖像生成。v1主筆刷跨入鄰槽而多出藍線，未安裝；中止舊稿正常路線與GUI，保留143／137退出及研究資料。v2獨立裁切各槽，兩亮線槽只允許0／14／15，正式工具附此保護。

八倍2048×1024索引PNG4961 bytes，改65188索引，選定區外1899968索引保持。正式Go新2097152索引精確，父1171條目63349632全彩字與4362240索引保持，含99中文字、姓名及兩個HUD。加入保護的待提交工具乾淨重建1288檔逐位元組相同。重用載入器測試曾保留HUD舊輸出標籤，原始數值斷言正確，另外保存語意校正收據，未改寫原始測試結果。

同一最終包194個CPU／Intel MTL GPU案例通過：100個四實際CLUT混色／變換、75個受控色盤、HD8／新CLUT／未知上傳3例，以及16場原始0x3f四邊形。獨立命令研究器初次把四CLUT全放X0，部分不可見；依原始operand改放X0／16／32／48後重跑，16場均有可見差異，舊收據保留。最後16場只歸零雙緩衝偏移、採固定背景，不是正常玩家截圖或速度驗收。

一次真BIOS、工廠空白卡及未改M4按鍵至34960，190組同機器父1171／新1172 HD2對照，169個封存原生摘要、畫面及卡片點一致，21點無獨立原生對照。正常取樣為34945至34959，15筆原始命令、12180列來源／快取字相同，四塊來源完整覆蓋，圖元加兩個HD像素邊界外差異0。初次驗證把停止場34960當成已執行的查詢場而停止；按實際機器迴圈及來源紀錄分開後，最後重複圖元34960只列既有trace及孤立GPU證據，沒有算成第16筆新CPU命令，也未改素材、時序或範圍遮罩。

同一8de152e候選及最終包正常新遊戲至34948，1920×1080最後畫面呈現新起手斜亮線；其他三來源由正常同場及孤立GPU畫面審查。內部HD2、CPU光柵、Intel GPU顯示，PGXP、xBR、透視及寬螢幕保持，GUI不作原生狀態同場聲明。READY後裝入三平台v.0.7.2雙擊預覽，230來源／1172條目、1069全彩與103索引。26832219 bytes，比介面父包增加5227 bytes；ZIP26400729 bytes、1288檔，SHA256為7dd44c6e97d8e3716b156a9c85c6ec2c3bb348c5a3ecc2606d389dce1517c7dd。

真正Linux候選無素材／後端參數重開604，WM_DELETE正常關窗，路徑及其餘設定保持；已呈現415幅，最後在途416未呈現，只驗啟動／保存。34948定場final_drawn=true。三平台程式、原版、OFL、手把與舊介面包保持，Windows／macOS限資料及結構。私人17張截圖保持，新比較與素材留本機。畫廊205個草稿／局部比較入口，正式檔案級HD仍0／264，效能調校延後。

另由既有原命令重播定位左側壁掛藍火，232張父1170包畫面相同，61個查詢場的976筆ROI圖元與143433列原字／快取成立，9來源／12組來源色盤配對包含背景及人物。b423eb09b08e13f3dc24a1214fcb3a18d9a79f2ecc3499d24328a3bfa4e86cca共用圖集[0,0,24,24)完整576來源像素為雙燭頭藍火外觀，語意為強推論；完整128×128／4bpp載荷命中F_GAME.BIN及F_GAME2.BIN檔案byte0x36000。17筆0x3f在34984至35000隨捲動逐步進入ROI，不將其局部邊界當作完整物件大小。正式ID、實際讀取副本、可破壞性與完整火焰動畫未知，沒有新的壁掛蠟燭美術或安裝。

引擎研究入口workplace/hdtex/perf/weapon-swing-redraw-v1/weapon-normal-GPU-verification.json、weapon-ready-verification.json、normal-field-boundary-correction.json及commands-v2.json。來源、原型、重製與GUI在私人素材repo workplace/assets/weapon-source-observation-v1/swing-redraw-v2-final/；交付為其hd-weapon-swing-preview-install/delivery-verification.json。壁掛定位入口為兩repo各自的candle-wall-observation-v1/，私人repo保存source-provenance.json及wall-candle-component-verification.json。只提交工具與文字，原版、PNG、字圖及包不進公開引擎或Issue。

## 2026-10-20：壁掛新稿揭露多邊形混色順序缺陷

工作為 #15，完整HD仍由 #8追蹤，效能延後。私人素材repo的目前表維持1172條目舊候選。
新壁掛雙藍火母稿以內建imagegen生成，只參考自製站立燭台；編製後只改b423共用圖集[0,0,24,24)，
新1173包比父包增加21693 bytes。正式Go、1290檔乾淨重製、70組CPU／Intel GPU通過。
一次真BIOS／空白卡／原M4至45784，262組父1172／新1173配對、232封存原生點通過，
139992列原字／快取與576來源像素完整，圖元兩HD像素邊界外0。這些是修正前結果。

同一舊執行檔的1080p實際GUI35000雖呈現兩個新燭台，右邊出現黑暈，因此未安裝。
查出原生與HD的drawPolygon先取f.mode，再載入自身TPage，違反既有Beetle ProcessFIFO順序。
固定e57860d82faa953dc0fc10f201fd583739bc2f7d的gpu.c已重新下載核對，SHA-256
c7aa6dc4b298e149e01d08c3a2f3df5f012d4ead2a94c1edb8c4ee2bf9db094e，GPL-2.0-or-later。
READY契約及來源為007 §10。新回歸先在四倍率全部重現previous0/current1用錯模式，
再將原生／HD兩處取mode移到setTPage之後。原生與HD單元全過，新增2048組獨立公式驗證，
涵蓋前後16種ABR、raw flat／Gouraud三角及四邊形、ABE／STP／透明、倍率1/2/4/8與兩側。

完整套件回歸及修正前後雙機M4正在執行。最初./...誤納ignored研究區的C檔與多個main，
屬測試範圍錯誤，該容器已停止；改以正式套件明列的相同Docker環境乾淨重跑。
因Digest含VRAM，研究雙機使用舊ABR僅作封存232點校驗，另比較不含VRAM的執行摘要與記憶卡。
該舊模式開關只在ignored overlay，沒有進正式GPU。新畫面若不同，另列修正結果，不改舊收據。
新候選預定v.0.7.3-20261020，待新正常路線、實際GPU、三平台與1080p通過後才交付，舊包保留。

首存離房兩個先前探針未成功，原生互動研究確認正常路線可由9000向右160場，
9160按CROSS+RIGHT10場，再向右80場，9250已由房間17,37到18,36；未寫PC／RAM或注入座標。
原先失敗是跳躍位置太早，未判為產品缺陷。後續35筆16×16特效來源仍是強推論、身份未知。
容量另由 #14追蹤，1173包manifest451863 bytes、解碼圖素133682944 bytes，未改019格式或限制。
研究、原失敗圖、正常重播與新回歸日誌入口均為workplace/hdtex/perf/candle-wall-observation-v1/redraw-v1/。

## 2026-10-20：壁掛雙藍火交付與混色修正驗收

70221ea只改原生與HD兩個正式多邊形取mode順序；與舊8de152e比對全部72個正式Go來源，只有兩檔改變，另新增公式回歸。正式套件完整回歸通過，公開GPU語料維持既有台帳，CPU／GTE及前端測試通過。實際Intel MTL共121例：53個原來源／CLUT／變換，加17筆真實四邊形各測前一ABR0至3的68例。舊53例會先設相同ABR，沒有覆蓋這次跨頁問題。007 §10已CONFORMED，整個GPU規格仍保留原有範圍與已知差異。

首趟修正後雙機重播在12500後以SIGTERM143中斷，沒有驗證assert失敗，原因未知，不稱為逾時或產品缺陷；原normal-fixed-output與中斷紀錄保留。相同程式、素材及輸入以明確命名容器乾淨重跑至45784，262組父1172／新1173 HD配對通過。262組排除VRAM的標準執行摘要與記憶卡相同；研究限定舊ABR機器的232個完整摘要符合封存收據。新原生畫面有103個歷史點改變，修正後N1命令重播232個像素雜湊則全部等於新機。沒有改寫舊oracle或把新圖片宣稱舊版逐位元相同。
正常139992列原字／快取相同，576來源像素完整，331筆圖元與17筆既有原始CB保持，圖元兩HD像素邊界外差異0。全彩512×512改4042字、區外252928字與STP保持，正常CLUT以外回退，父1172條目保持。

新v.0.7.3-20261020由70221ea建立，三平台乾淨包與本機ZIP完成版號、CRC、UTF-8、素材排除、雜湊、OFL及macOS universal結構核對。原版只以唯讀來源組裝私人local-full。壁掛新稿READY後安裝231來源／1173條目，1070全彩與103索引，含99中文字、一個姓名、兩個HUD與揮擊。26853912 bytes，比父包增加21693；ZIP26420562 bytes、1290檔，SHA-256 20acc609230af39d9c69479d1630dbecadf294c77980e6e9cddc197cb480c267。
首次安裝因新版本尚無textures父目錄而停止，設定與assembly已回復並核對。安裝器改mkdir parents後從空白備份重跑，失敗收據移至私人研究區install-attempt1，沒有沿用不完整安裝。舊v.0.7.2、其包／設定／原版與17張私人截圖保持。

Linux同一新候選執行檔與最終包，原M4正常至35000，實際1920×1080最後畫面final_drawn=true，25496已呈現。右側黑暈消失，右控制裁切4487像素改變，左控制裁切0；新稿素材檔未變。CPU光柵、Intel GPU顯示，HD2／PGXP／xBR／透視／寬螢幕及手把設定保持。GUI寬螢幕不作原生同場聲明。真正安裝候選從其他cwd無素材／後端參數重開606，WM_DELETE正常關窗，398幅已呈現；最後606的final_drawn=false，只驗啟動與設定，不取代35000定場圖。
Windows新執行檔的Wine啟動，初次Weston compositor崩潰，保留環境失敗日誌；同一映像改既有Xvfb軟體OpenGL，正常至600、final_drawn=true、設定保持。要求1920×1080，Xvfb桌面視窗實際1920×1058，只算啟動，不算Windows1080p或真機。macOS只驗結構。
Linux執行檔SHA-256 e52234c47429a93455bc9100f1eea9b28d218345d746ea4d2d9c40b13d7c2fb8；Windows cc0f16ba60462ead72ad6cf9b8cad75d9c5885180b05307b5af29470be3e6870；macOS a536653272b764203257d23c3085ab36f229d72702c1dddd00cc0bd2554fae64。

本機畫廊208草稿／局部入口、479個圖片連結全部有效，三份原版與三份HD總覽保持；正式檔案級HD仍0／264。生成母稿477872 bytes與兩張比較不進執行包。畫廊更新曾把既有總覽輸出目錄整個唯讀掛載而停止，改只放行三個既有總覽檔寫入，原圖與拆檔仍唯讀，乾淨重跑通過。
前節首存17,37至18,36數值來自015的RAM座標候選，保留假說等級；9250畫面確實離房，但不稱正式房間ID。16×16特效身份、壁掛物件可破壞性及完整動畫仍未知。完整人物、怪物、字庫、介面、武器、蠟燭、地圖與HD未完成，效能延後。

研究入口為workplace/hdtex/perf/candle-wall-observation-v1/redraw-v1/的wall-normal-GPU-fixed-verification.json、wall-ready-verification.json、native-same-state-verification.json、commands-fixed.json；私人來源／GUI在同名研究區，交付與重開為其workplace/assets/hd-candle-wall-preview-install/delivery-verification.json及reopen-closure-verification.json。唯一現況仍為私人docs/hd-assets-evaluation.md §8。只提交程式、工具與文字，不提交原版、PNG、字圖或包。

## 2026-10-20：燭台破壞四段火焰子集

沿正常首存讀檔、離房、跳躍與揮擊按鍵，在階梯區觀測一盞燭台消失及四段小火焰，沒有PC／RAM或座標注入。心數10、金錢25在原生選單前後相同，沒有拾取證據。正式物件ID與掉落身份未知，不把此盞消失外推為先前壁掛雙藍火的可破壞性。舊b423圖集16×16紅色特效候選仍維持身份未知。

來源2b891c794c2affc2f40a4e28cde871554365fce598c3f2a8e0aaddea9fe147b9為128×128、4bpp，原始8192 bytes命中BIN/F_GAME.BIN檔案byte0x10000，未命中F_GAME2。原生VRAM上傳640,256。先前run/在完成畫面後輸出大量來源JSON時被記憶體限制終止137，保留不完整紀錄；run-v2/縮小觀測ROI並依來源分區圖元，正常至9700得到111組原生／HD點、131467來源列與665分區圖元，四張原生圖精確重現互動試玩。

私人素材規格READY後，以內建imagegen只參考自製燭台風格，畫出四個新火焰階段。1254×1254透明母稿501453 bytes，SHA-256 6be0ea4156e68826cfc34312daa8939852e62dde4d709ed0e15f0aa2dbc69a93。四倍編製只改[64,32,80,48)、[80,32,96,48)、[96,32,112,48)、[112,32,128,48)與兩個實際CLUT的既有配對。新RGB直接採母稿，alpha覆蓋率乘入加法RGB，新可見貼素STP1；共享STP保留原標記並新增1058個新輪廓標記。透明輪廓有意改變，不稱所有解碼bit15保持。

正式Go核對524288新色字精確、未選區507904字保持，父1173條目63611776全彩字及6459392索引保持。乾淨1293檔重製逐位元組相同。直接正式CPU／實際Intel MTL GPU共94例：兩組實際CLUT的50個混色／變換、受控未知CLUT25例、HD8／未知CLUT／未知上傳3例、16筆原始12字0x3f與ABR1四邊形。完整VRAM、畫面與統計相同。受控未知色盤不是第三個遊戲CLUT，不將孤立驗證當完整GPU前端或速度。

最終1175包另一次真BIOS、原首存卡與相同正常按鍵至9700，同機器父1173／新1175共111組HD對照；111個修正後原生摘要、像素、卡片點及父包HD像素相同。正常4224列來源／快取原字精確，1024來源像素完整，16個相鄰來源像素保持，16筆連續火焰圖元及原UV／位置／切換保持；圖元兩HD像素邊界外0。此為70221ea修正後場景基準，不稱獨立硬體oracle，寬螢幕關閉。

同一v.0.7.3-20261020 Linux候選與新包，以正常按鍵至9520，實際1920×1080最後畫面呈現，final_frame與shown_frame均6833、final_drawn=true。新細曲火焰可見且無黑暈，原命中星芒及未覆蓋人物動作仍保留。GUI沿HD2、CPU、Intel顯示、PGXP、xBR、透視、寬螢幕及手把；GUI寬螢幕不作原生同場聲明。

READY後只切換三平台本機素材路徑，現行232來源／1175條目、1072全彩及103索引，26894204 bytes，比壁掛父包增加40292。ZIP26457742 bytes、1293檔，SHA-256 03a41ad3b8c388da28fc239f91ff25f890459cc39d4472c26c2df1ae91938fb3。原72個正式Go來源及三平台執行檔未變，沒有重做無關回歸或新增渲染器特例。manifest452639 bytes、解碼134731520 bytes，019格式及容量限制未改，#14保持開啟。
真正安裝Linux候選另從其他cwd無素材／後端參數重開602，WM_DELETE正常關窗，388幅已呈現，最後602的final_drawn=false，只驗啟動與設定保存。首次重開誤將設定目錄唯讀掛載而無法寫回，保留gui-reopen-readonly-attempt/；修正可寫設定與唯讀媒體／素材掛載後，在相同映像與腳本乾淨重跑，無設定錯誤。Windows／macOS本批限新包資料與既有程式結構，沒有新平台試玩或真機驗收。

舊壁掛包、舊v.0.7.2、三平台執行檔／原版／OFL、既有記憶卡、其他設定及17張私人PNG均保持。母稿、提示與比較不進執行包。畫廊新增來源比較、正常同場、母稿與四階段慢速GIF，共212草稿／局部入口、483圖片連結全部有效；GIF非原時序或速度證據。正式檔案級HD仍0／264。命中星芒、掉落／拾取、其他燭台、完整人物、怪物、字庫、介面、武器、地圖與HD未完成，效能延後。

研究入口為workplace/hdtex/perf/candle-wall-observation-v1/redraw-v1/fire4-v1/下normal-GPU-verification.json、ready-verification.json及commands.json；私人來源／作者稿／GUI在素材repo workplace/assets/candle-wall-observation-v1/break-effects-v1/，完整交付為workplace/assets/hd-candle-fire4-preview-install/delivery-verification.json。唯一現況仍為私人docs/hd-assets-evaluation.md §8。只提交工具與文字，原版、PNG、字圖與包不進公開引擎或Issue。

本批Docker一次性容器均已移除，兩工作根掃描無root擁有檔案或誤建.md目錄。提交、push與遠端Issue精確回讀另外保存在私人break-effects-v1/publication-verification.json，相關HD工作Issue保持開啟。

## 2026-10-20：三段命中星芒交付與裝備選單字源觀測

前一批四段火焰已安裝，本批接續同一正常首存攻擊鏈的三段命中星芒。來源2708d5b9f5a9c71ae09b3627458e75dab535acd5a5b8e3f70558d3ad63631568為128×128、4bpp，8192 bytes同時命中BIN/F_GAME.BIN與F_GAME2.BIN檔案byte0x12000，實際讀取副本未知；VRAM上傳672,256。選定[0,96,32,128)、[32,96,64,128)、[64,96,96,128)，原始0x3d、12字圖元使用CLUT5872210010f00b6966cbe36c19d9ee57d1ca5451b7d85b38dce527dbc1d0506a。原非透明色字STP0且ABE關閉，沒有沿用火焰的加法／STP1契約。

私人素材READY規格後，以Pillow可重現幾何曲線新畫金白多束爆光、中央亮面、藍白收束與向外碎光。未用圖像生成、舊RGB或色盤乘數；四倍全彩RGBA使用0／255 alpha，STP全零。只新增一個實際CLUT條目，另一個已觀測CLUT回退。正式Go核對262144新色字精確、未選區212992字保持，父1175條目64136064全彩字與6459392索引保持。改6146色字，842個原透明位置新增可見子像素、3367個舊可見位置轉透明，均為新輪廓有意變更；原圖元位置與切換不變。乾淨1295檔重製逐位元組相同。

同一最終1176包直接通過59組CPU／實際Intel MTL GPU：兩個實際CLUT共50個混色／變換，一組命中、一組保持原版回退；HD8／未知CLUT／未知上傳3例，另六筆原始不透明四邊形。完整VRAM、畫面與統計相同。正式引擎72個Go來源與三平台執行檔未改，沿70221ea，不重跑無關核心回歸，也不把孤立GPU當完整GPU前端或速度。

真BIOS、原首存卡、相同正常按鍵至9700，同機器父1175／新1176共111組HD對照，111個修正後原生摘要／像素／卡片點及父包HD像素相同。6144列原字／快取相同、3072來源像素完整，六筆原始圖元及UV／位置保持。首次驗證誤預期9518可見而停止，原始繪製9517～9522實際呈現9519～9524，依顯示緩衝位置修正為每張精確對應兩場前的單一圖元。兩HD像素邊界外0，沒有改素材或放寬範圍。原報告及腳本保留於normal-field-assumption-correction.json與verify-normal-field-assumption.py。來源墨跡首報誤以P模式色盤轉換建遮罩而得None，也已改索引原字節建遮罩，首報保留，原載荷／CLUT／圖元證據不變。

Linux同一候選與新包正常讀檔至9520，1920×1080最後畫面已呈現，final_frame與shown_frame均6675、final_drawn=true；新細光束與既有新火焰可見、無黑暈。GUI沿HD2、CPU、Intel顯示、PGXP、xBR、透視、寬螢幕及手把；GUI寬螢幕不作原生同場聲明。獨立試玩設定唯讀造成寫回警告，保留日誌，此趟只驗顯示。真正安裝候選以可寫設定、唯讀原版與素材另無素材／後端參數重開604，WM_DELETE正常關窗，391幅已呈現；最後604的final_drawn=false，只驗啟動與保存。重開首次成功，沒有本批重開失敗；交付metadata一度繼承前批唯讀嘗試措辭，已另存原稿並修正，底層日誌未改。

三平台本機v.0.7.3-20261020只切換素材路徑至textures/castle-hit-star3-v1-compact，舊1175火焰包、壁掛包、舊v.0.7.2、執行檔、原版媒體、OFL、既有卡片與其他設定保持。233來源／1176條目、1073全彩與103索引，26914756 bytes，比火焰父包增加20552；ZIP26476809 bytes、1295檔，SHA-256 6c5b019dc85ccee5cc27f9f5daaa2e5518dd7ddb162e17d2101725bb2cce3bd1。manifest453017 bytes、解碼135255808 bytes，019格式與容量上限未改。Windows／macOS本批限資料與既有程式結構，沒有新平台試玩或真機驗收。17張私人PNG保持；畫廊214個草稿／局部入口、485個圖片連結有效，正式檔案級HD仍0／264。

另外以原生sotnlab從同一正常首存路線9700向左探索，9820在阻擋左側看到心形外觀物件。LEFT、SQUARE、UP與跳躍均未拾取；唯讀roomX候選停在150，跳躍時roomY候選有變化，沒有猜正式房間ID或修碰撞。原生10130選單確認HP70、心數10、金錢25，歸屬及拾取未知，不稱來自前盞燭台。完整按鍵、原生圖、原始數值與限制為break-route-lab/pickup-v1/verification.json；沒有PC／RAM／實體注入，實驗器已EOF正常結束。

裝備選單中文字仍未HD，依字模索引路由進DRAFT觀測。新鮮BIOS／首存卡重播全部實際按鍵到10130，111個舊原生摘要／像素／卡片及HD基準相同，9820、9850、9885、10060、10130五張原生圖與互動試玩逐像素相同。選單ROI取得57646列原字／快取相同、324筆來源分區圖元，辨識13個未在1176包的12×16字源，每字192來源像素完整。尚未回復Unicode或字位，沒有編製新字圖。選單切換為364×240、顯示起點128,256，後續須追繪製／複製／呈現鏈再作非透明字像素匹配，不能沿用256寬遊戲座標或把ROI查詢當完成。入口workplace/hdtex/perf/alucard-hud-redraw-v1/menu10130-v1/capture/verification.json、commands.json及私人同名研究區source-groups.json。

星芒研究入口為workplace/hdtex/perf/candle-wall-observation-v1/redraw-v1/star3-v1/的normal-GPU-verification.json、ready-verification.json及commands.json；私人作者稿／GUI為素材repo workplace/assets/candle-wall-observation-v1/break-effects-v1/，完整交付為workplace/assets/hd-hit-star3-preview-install/delivery-verification.json。工具為私人repo tools/build_hd_hit_star_sample.py與tools/install_hd_hit_star_preview.py。唯一目前表仍為私人docs/hd-assets-evaluation.md §8，提交／push與遠端Issue回讀另在star-publication-verification.json。只提交工具與文字，原版、PNG、字圖與包不進公開引擎或Issue。

本輪所有有界星芒、拾取與選單觀測容器均已移除，兩工作根掃描無root擁有檔案或誤建.md目錄。完整人物、怪物、字庫、介面、武器、蠟燭、地圖與HD尚未完成，效能延後；相關Issue保持開啟。

## 2026-10-21：選單字形與獨立頭像子集

沿正常首存讀檔、移動、START開選單，導覽右／左手、頭盔、鎧甲、披風、飾品與設定，再進按鍵配置並返回遊戲11000。沒有PC／RAM／座標注入。原生364×240顯示在0,0及128,256雙緩衝切換，實際繪圖偏移與GP1分離核對，期間無VRAM複製。43個非空12×16來源已以原索引／CLUT及31505個原生前景像素匹配，另24像素由確認的後畫白框覆蓋；保留装／撃／単／効／顔原字種。Unicode為視覺轉錄，原編碼未知。

私人素材READY規格後，以Noto Sans CJK TC Regular／OFL重製43字及共用小字圖107選格，74格有正常來源／像素證據，其餘33格限來源與直接GPU。16個舊HD姓名字格整格保持，未選格保持。v1正式測試雖通過，1080p仍發現○／□細邊被HD2取樣漏掉，因此拒絕安裝並停止其未完按鍵試玩；v2只將○×□△改用原墨跡矩形內8個HD像素寬幾何線條，保留原前景索引。其他1339個檔案與前稿相同。不能用GPU一致抵銷可讀性缺陷。

選單頭像另有8bpp來源0d7204456ecda62374545f1d7f63c3c13d5c6a6b4bd249f14810779516be206e，原64×128上方64×96顯示。重用核准自製頭像母稿a8409b081ddbc1d8f12e5ec9bad3f2609fc1fe84edcb5d1a9d5831963d1e8ba3，加自製深色襯底與金框；非原RGB或亮度乘數。4倍RGBA＋STP，原720透明像素刻意變成有框畫像，STP逐位元及底32列空白保持。原上半0x3d raw及下半0x3c Gouraud淡出不改。

最終1220包正式Go逐字核對新增528384 CJK索引、共用小字1048576索引及131072頭像色字。其他1175條目64398208全彩字與5410816索引保持，頭像131072個STP位元／32768底部空字保持。乾淨1340檔重製相同。直接正式CPU／實際Intel MTL GPU共1821例：45來源／實際CLUT配對1125個混色／變換、660個STP／零CLUT控制、4個HD8／未知來源／全彩未知CLUT及8筆原頭像圖元×4先前ABR。實際CLUT共3種。完整VRAM、畫面與統計相同，非完整GPU前端或速度驗收。

無HD／PGXP的正式原生核心建立150點新鮮BIOS／首存卡基準，147既有點保持。最終包另正常11000有150組父1176／新1220 HD2配對，全部150原生摘要／像素／卡片及147既有父HD點相同。170474列原字／快取、1525筆來源分區圖元保持；43字8256來源像素與頭像6144來源像素完整。所有差異點前兩場另有54場有界觀測、530640列與5194分區圖元，按實際顯示緩衝交集核對，全部27差異點的文字／頭像圖元加兩HD像素外0；返回遊戲10760、10800、11000保持。v1邊界觀測的原生、父像素與原始圖元與v2相同，只借用原命令幾何證據，不冒稱v1新字像素等於v2。

同一v.0.7.3-20261020執行檔與最終包，以獨立可寫設定、唯讀原版／素材正常到主選單10130及按鍵配置10540，兩張1920×1080最後畫面已呈現，四種控制符號完整。HD2、CPU光柵、Intel GPU顯示，PGXP、xBR、透視、寬螢幕與手把沿原設定；GUI寬螢幕不作原生同場聲明。真正安裝候選無素材／後端參數重開605，正常WM_DELETE關窗，403幅已呈現，最後605為final_drawn=false，只驗啟動／設定保存。Windows／macOS本批限資料及既有程式結構。

三平台本機包只切換到textures/castle-menu-ui-v1-compact。現行277來源／1220條目、1074全彩與146索引，142個12×16來源不代表142不同Unicode。27176138 bytes，增加261382；ZIP26735920 bytes、1340檔，SHA-256 93e501a924ea7fa7e251ed2a1fd96c9e9ba3f17534da2a24ea939eff4c5eedbf。manifest464647 bytes、解碼圖素計帳136046336 bytes，非程序或GPU峰值。72個正式Go來源、三平台執行檔、37980個既有候選／包／媒體／卡片檔案與17張私人PNG保持。畫廊218草稿／局部入口、489圖片連結有效，正式檔案HD仍0／264。

命令收集器首趟因重複狀態包達既有上限而停止，改為目標場完整包與其間顯示／複製包，原上限及正式程式不變。離線報告變數覆用、研究執行檔／輸出同名及啟動器空命令也已修正，在同一隔離環境乾淨重跑，失敗收據保留，不列產品缺陷。

研究入口為workplace/hdtex/perf/alucard-hud-redraw-v1/menu10130-v1/的capture-v2/verification.json、native-baseline-v1/、normal-v2-validation.json、normal-GPU-verification.json及ready-verification.json。私人作者稿／GUI為素材repo同名研究區，完整交付與重開為workplace/assets/hd-menu-ui-preview-install/delivery-verification.json。私人工具build_hd_menu_sample.py、install_hd_menu_preview.py與唯一目前表docs/hd-assets-evaluation.md §8同步更新。提交／推送及Issue精確回讀另在該研究區publication-verification.json。圖片、字圖、原版及包不入公開引擎或Issue。

本批容器均已移除，兩工作根無root擁有檔案或誤建.md目錄。完整UI、字庫、人物、怪物、武器、關卡、地圖與HD未完成，效能調校延後，相關Issue保持開啟。

## 2026-10-21：持盾骷髏構件、正常擊倒與骨骼重繪診斷

接續怪物HD #6。已安裝的骨質／盾面仍是局部表面，這輪沒有安裝新稿。正式Go及三平台候選保持70221ea。原172點舊收據保留，與目前修正後無HD核心比對96點完整相同、76點畫面或混合摘要不同，172點CPU時序前綴與卡片保持。混合摘要含VRAM等狀態，不能據此推論RAM變更或保持，也不把全部76點直接歸因於TPage順序修正。

使用目前正式原生sotnlab從正常BIOS／首存卡讀檔、離房與交戰，只送一般按鍵。可見選單EXP70／BEAT5，9810骨骼散開及火焰，9840與10030為EXP92／BEAT6，10100怪物消失。沒有猜敵人HP位址、存取狀態注入、PC／RAM／座標寫入；色盤閃光不獨立當命中或死亡證據。互動器已EOF正常結束。按鍵固定於scenario-kill-v1.json，另一次無HD重播353點中27張互動原生圖逐像素相同。

父1220包再沿同一線性輸入到10100，353個無HD原生摘要／像素／卡片點保持。58477列來源原字與快取字相同，6242筆原0x3d／12字不透明圖元，44種UV界框、12組實際CLUT保存。23構件有原圖元證據，另3個下身外觀選區未在此路線出現。14226／16384來源像素曾取樣；3877個非零像素中3533曾取樣，未見344個，其中297個在三個靜態候選，不稱完整貼素或全動畫覆蓋。第12組色盤最早8945出現，早於擊倒，語意未知，不命名為死亡效果。正式怪物與動畫ID仍未知，來源格的頭骨、肋骨、手臂等屬外觀強推論。

內建imagegen只接受文字及自製母稿，未提供原版或衍生圖。13張本機畫稿涵蓋同一骨骼造型、四張出招、九格步態、兩臂、頭骨、肋骨、細骨片及盾劍。RGB與新內部輪廓直接採新稿，沒有乘原RGB或亮度。六輪隔離原圖元原型修正握劍手位置、水平劍向與圖集定位；步態改按九個獨立連通物件擷取，排除固定格線混入鄰格邊角及裁腳。母稿、提示與前版原型保留，不進執行包。該流程的矩形、切圖與縮放是圖集編排，不以原索引輪廓裁新骨骼。

目前DRAFT診斷只把22個已有原圖元的構件、一組30e981正常CLUT編入父1220包，另一正常CLUT、特殊色盤、第12格扇形效果及三格未觀測步態保持。四倍圖集512×512、164167 bytes，alpha0／255、STP0。另一次真BIOS／首存卡正常到10100，同機器父1220與診斷1220共353組HD2／PGXP／透視對照，全部353原生摘要／像素／卡片與353舊父HD保持。162點有新骨骼差異，191點相同；全部差異限精確前兩場的目標原圖元，依實際顯示起點投影、固定兩HD像素邊界，區外0。正常交戰、出招與散骨畫面已見新稿，仍不稱全色盤／實際GPU／1080p／安裝或速度驗收。

邊界腳本首版誤讀display陣列為寬度，已依觀測器定義修正為X起點、Y起點及模式，使用相同資料與門檻重跑；候選編製腳本的換行轉義及母稿稽核欄位也已修正，均為研究腳本問題。正式模擬、渲染與效能程式不變。三平台現行執行檔、每平台1340個活躍素材檔、保存設定及17張私人PNG保持；沒有重新雜湊所有歷史封包。現行277來源／1220條目、27176138 bytes與畫廊218／489保持，正式檔案HD仍0／264。

引擎研究入口為workplace/hdtex/perf/skeleton-redraw-v2/下native-baseline/、native-kill-lab/、kill-native-baseline/、kill-source-capture/及normal-single-v1/verification.json。私人來源與作者稿入口為素材repo workplace/assets/skeleton-redraw-v2/source-geometry-v2.json、normal-kill-verification.json、prototype-structure-v6/、normal-single-v1/normal-boundary-verification.json、own-art-source-verification.json與protected-existing-verification.json，完整提示由hd-assets/drafts/skeleton-redraw-v2-prompt-set.json索引。唯一目前表docs/hd-assets-evaluation.md §8同步，工具與收據只留本機。提交僅文字文件，不含圖片、來源字、光碟、BIOS、卡片或包。

這輪有界容器均已結束並自動移除，兩工作根無root擁有檔案或誤建.md目錄。接續須完成另一正常色盤、效果與動畫銜接、實際GPU及1080p，再通過READY契約與候選安裝。完整HD仍未完成，效能調校延後，相關Issue保持開啟。

## 2026-10-21：本機HD怪物素材子集更新

已依既有閘門檢查後更新本機怪物HD子集，正式模擬器程式未改。首版的視覺接合問題已修正，修正稿重新驗證後才接入候選。詳細來源、驗證與交付紀錄保存在私人素材專案的既有現況入口，公開紀錄僅保存工作進度。

完整HD仍未完成，CPU與GPU選項保留，效能調校延後。Windows／macOS本次沒有新增真機驗收。本輪有界工具容器均已結束並移除；文件與製作工具提交、推送及Issue回讀另在私人交付紀錄保存。

## 2026-10-21：本機HD掉落小圖更新

沿正常遊戲路線核對掉落與拾取後，新小圖已通過資料、CPU／GPU、正常路線與實際視窗檢查，接入本機候選。正式模擬器程式未改，詳細來源及交付證據留在私人素材專案的既有現況入口。

完整HD仍未完成，CPU與GPU選項保留，效能調校延後。Windows／macOS本次未新增真機驗收。工具容器已結束；提交、推送、Issue回讀及清理結果另存私人交付紀錄。

## 2026-10-21：本機HD角色攻擊與武器子集更新

已依既有規格與驗收閘門，將新一批角色攻擊及武器美術接入本機候選。資料重建、CPU／GPU、正常遊戲路線、實際視窗及安裝重開均已核對。正式模擬器程式未改，詳細來源與交付證據留在私人素材專案的既有現況入口。

完整HD仍未完成，CPU與GPU選項保留，效能調校延後。Windows／macOS本次未新增真機驗收。製作工具與文件提交、推送、Issue回讀及容器清理結果另存私人交付紀錄；圖片、原版與封包未上傳。


## 2026-10-21：核准動作素材與精簡驗證

使用者核准目前美術，後續每批最多一次審查與一輪整合測試；規則已記入AGENTS.md。42個新增阿魯卡多來源／常用色盤接入本機候選，共341來源／1285條目，約30.5 MB。正式執行器與三平台程式未改。一次正常M4至首存，524個封存原生狀態／像素／卡片點一致；同機器524組HD對照有244組改變，完整可定位175點的區外差異0。實際已安裝Linux候選一次正常讀檔與跳躍，1080p最後畫面已呈現，設定保存；Windows／macOS本批限資料。

GPU實際圖元清單44例完成CPU／GPU完整比較，41個新來源走硬體；1個負座標圖元依既有021回退CPU。工具最初在索引15誤要求硬體三角形，保留原停點；前16例比較由原程式控制流及影像承接，只接續未執行28例，沒有重跑完成案例。索引15不外推孤立可見性或硬體執行證據。唯一當前狀態及逐項證據仍在素材專案 docs/hd-assets-evaluation.md §8 與其工作紀錄。來源圖、ROM、母稿、提示與包只留本機。完整HD、全程速度與通關未完成，CPU／GPU選項保留，優化延後。


## 2026-10-21：Richter鞭節HD子集

已沿核准美術與每批一輪整合政策，將銀色鞭節與端頭兩格接入三平台本機候選，342來源／1286條目，執行素材約30.5 MB。正式執行器與平台程式未改。正式資料、既有正常命令同場、兩格實際CPU／GPU及未知來源回退完成；一次實際Linux正常新遊戲揮鞭，1080p最後畫面呈現、保存設定保持。來源與快取綁定已核對；這次沒有重跑原生CPU／RAM，未把封存命令當成新原生狀態重播。

首次GPU容器輸出掛載唯讀，尚未執行案例；保留日誌後修正掛載，只完成未開始四例。兩格以外、原有1285條目與玩家卡片保持。其他色盤及完整武器／角色／怪物HD仍未完成，CPU／GPU保留、優化延後，Windows／macOS本次限資料。詳細證據與清理結果仍由私人素材專案docs/hd-assets-evaluation.md §8及WORKLOG索引；此公開紀錄不含原版、圖片或素材包。


## 2026-10-21：兩張對話頭像HD子集

沿已核准人物美術與每批一次審查／一輪整合政策，德古拉及死神兩張已觀測對話頭像接入三平台本機候選，共344來源／1288條目，約30.8 MB。正式執行器與三平台程式未改。新資料及父包保持、既有正常命令成對畫面、兩張實際CPU／GPU與未知配對回退通過。這次沒有重跑原生CPU／RAM，不把封存命令當成新原生狀態重播。一次實際Linux正常新遊戲至死神對話，1080p最後畫面呈現、設定保存，獨立卡符合封存同場值。

正常M4四張已觀測對話頭像均有HD，其他頭像、人物與怪物本體及完整HD仍未完成。CPU／GPU與原設定保留，優化延後，Windows／macOS本批限資料及既有程式結構。唯一目前表及詳細證據仍在私人素材專案docs/hd-assets-evaluation.md §8與WORKLOG；原版、圖片、母稿、提示及素材包未加入公開儲存庫。提交、Issue回讀與清理結果另存私人交付紀錄。


## 2026-10-21：背景優先與天空重繪子集

使用者指定先背景、再全部怪物，沿定案畫風及提示詞策略，每批集中驗證一次後定案。序章月面與雲層四組正常色盤已改採新全彩母稿並接入本機候選，344來源／1288條目、約31.0 MB，正式執行器不變。資料及父1284條目保持、封存正常命令畫面、CPU／GPU與一次實際Linux1080p正常序章通過；沒有重新執行原生CPU／RAM。研究JSON準備錯誤在0案例時修正，原工具及失敗範圍保留，沒有反覆重繪或重跑完成案例。

其餘背景與全怪物圖鑑／戰鬥動畫繼續製作，完整HD與速度未完成，優化延後，CPU／GPU及原設定保持。Windows／macOS本批限資料及既有程式結構。唯一現況與詳細證據仍在私人素材專案docs/hd-assets-evaluation.md §8及WORKLOG，原版、圖片、母稿、提示與包未上傳公開repo。

GPU首例雲層只承接停點前已完成的完整比較，不宣稱其孤立新稿／硬體可見性；原二進位及日誌保留。只接續未執行1至6，四正常配對實際走硬體、另兩例為未知配對回退，沒有重跑已完成索引0。

使用者另授權後續Ada GPU與sudo Docker。僅完成遠端既有容器的GPU能力查詢與收尾，沒有重跑本批遊戲、部署ps-golem或傳送原版／素材。後續新批次可使用，登入及探測識別只留私人本機索引，不放公開repo。


## 2026-10-22：可選 HD 精靈繪圖序列

依 READY `023-hd-sprite-runs.md`加入可選`sprite-runs.json`。完整精靈序列與原生來源通過守門時，沿既有索引PNG、CLUT與CPU／GPU取樣路徑逐格替換。圖像、翻譯與作品序列由外部素材包提供，019 manifest保持，整包解碼上限保持256MiB；不改機器、原生GPU、鏡頭或遊戲記憶體。

未完成、未知來源、狀態變更與同步邊界原樣回退。來源驗證只在原貼圖快取副本模擬原取樣，不改真實快取引用或統計。精確重送目前有效狀態的E1／零E2可交換，其他命令先沖出；此例外先補READY契約再實作。新增合成資料測試涵蓋完整序列、256列、破損來源、錯序／截斷、同步邊界、重送／改變狀態及嚴格附件載入。

容器內`xvfb-run -a go test ./render/... ./machine/...`與`go test -race ./render/hd`通過。最初未啟動顯示器而使hdgpu初始化失敗，修正驗證環境後用相同選擇乾淨通過，不列產品缺陷。三平台`tools/build_local.sh v.0.7.5-20261022`通過封包CRC、版本、字型授權與原版資料排除；macOS僅通用執行檔結構，未宣稱真機功能。較早v.0.7.4為邊界勘誤前的未交付建置，沒有移動或覆寫既有版本。

實際素材、正常遊戲證據與截圖由素材專案保存。單獨繪圖測試不聲明新原生CPU／RAM對拍、全速或全遊戲HD完成。此處只記錄通用能力與公開合成驗證，不帶入私人素材。


## 2026-10-22：019內部共用圖塊容量

依019 §8先READY再實作，首次線性計帳將超過256MiB時，把已完整驗證的圖像編入不可變32×32圖塊及uint32索引；原可接受包保持線性，PNG／STP、manifest v1、上限、素材鍵與機器狀態不變。索引與色字分庫，雜湊只查找、逐字相同才共用，索引資料也納入同一上限，編製字典不留在renderer。CPU／GPU仍經同一texturePixel讀取。

合成測試覆蓋完整原字／索引／CLUT／STP、32格邊緣、強制雜湊碰撞、資料及索引計帳與拒絕；正式65條目真PNG／JSON包超過原線性界線後成功，晚期壞PNG仍回傳nil而非部分包。render／hdgpu／machine回歸與完整race通過。首趟race在600秒Go預設逾時中斷，當時大包第二次Load仍在image/color.Convert，未報race；保留全部資料與選擇，僅-timeout25m後713.725秒通過，不列產品race缺陷。

純合成8例HD4線性CPU／共用CPU／實體Intel GPU的完整VRAM、畫面及統計相同，涵蓋四混色、翻轉、裁切與未知CLUT／上傳回退。八例均走硬體；裁切先在CPU決定有效矩形後仍走GPU，原工具文案CPU回退已依統計訂正，不重跑案例。實際素材的逐字比較由素材專案保存，不將私人圖像或輸入轉入通用核心。

v.0.7.6-20261022三平台乾淨候選的建置、CRC、版號、授權與原版排除通過；macOS只作結構，沒有本批正常遊戲GUI或玩家資料組裝。此層CONFORMED限載入器與合成取樣，完整素材包的其他界線、全遊戲HD與速度仍未完成。

## 2026-10-23：完整HD候選的共用圖塊容量接續

素材專案209c09c已完成完整候選，正式Load在256MiB停止，當時新色字／GPU案例0。依素材專案唯一§8及no0-completion-v1/continuation-v1.json接續，不重繪或重審美術。路由命中規格實作閘門，載入retro-remake-spec-gated-workflow、000及001後，追查全部textureTile與tiles讀寫端。確認正式讀取集中於texturePixel／textureTileImage.pixel，圖塊維度原由同組常數推導，沒有其他正式路徑寫死32。

019 §8.1先DRAFT再審查READY。起初考慮直接縮至8，檢查發現大張重複圖會增加圖格開銷，故在實作前修訂為保留已接受線性／32表示，只有型別辨識的資源不足才於Load內編8。已驗證的線性或32色字直接重編，不重讀、重新量化或改PNG；所有新條目及023姓名仍完整驗證，8仍不足或內容錯誤就整包拒絕。此為既有不可變共用機制的容量擴充，原256MiB、manifest及單圖界線保持。

正式實作在texture_tiles.go、texture_pack.go及sprite_runs.go。圖塊庫保留自身位移與實際長度的索引／色字切片，雜湊只選候選，完整內容相同才共用。取得pack前才能轉換，途中失敗不回傳部分資料；已交回pack沒有延遲寫入或解壓。私有小預算載入入口沿同一驗證流程，公開入口仍固定原預算；姓名與圖像共用同一計帳。原32單塊與已可載入32包保持。

一次render/hd及render/hdgpu回歸已通過，178個測例、兩套件pass、失敗與skip0。新公開合成真PNG覆蓋首次條目、部分線性／32轉換、32追加轉8、原32保持、碰撞與非整塊、晚期alpha／STP／索引拒絕，以及姓名總額轉8後內容守門和合計超限。既有65個真PNG觸發共用與晚期壞PNG拒絕亦通過。原測例只調整colorID切片介面，沒有放寬預算或刪測例；本輪未重跑完整race或原版CPU套件。

素材專案保存engine-storage-review-v1.json、engine-storage-tests-v1.jsonl及engine-storage-unit-verification-v1.json。原零案例停止、原READY及Go工具保留，新接續按實際切片長度計帳並鎖新來源，使用未改動完整候選；正式資料與實體GPU仍待完成，不宣稱引擎未修改或正常新關卡使用。已安裝程式與素材包保持，速度、完整HD與通關未驗收。

正式接續已完成：完整1930條目在原界線載入，父1757條目維持32、新候選1930條目為8；45350912新色字、186130240父值、2790144邊界及四姓名保持。實際計帳172765640 bytes，與先前精確布局量測相同。原175案例全通過，全部Intel MTL硬體精靈、Triangles0／Fallbacks0，173新像素及兩未知控制保持，整個程序exit0。原0案例容量停止、原契約／工具與唯一美術保持。019 §8.1按載入與資料取樣範圍轉CONFORMED；正常新關卡使用、三平台候選、速度及完整HD仍未驗收。沒有修改原版routine或CPU／RAM。


## 2026-10-23：容量修正的新版三平台候選

以乾淨main f5b9522建置v.0.7.7-20261023，沿既有tools/build_dist.sh輸出dist-all同版patch，沒有覆寫既有版號或建立Release／tag。Linux與Windows兩個執行檔、macOS arm64／x86_64通用執行檔完成；macOS四道結構與最低11.0檢查通過。dist_pack.py的版號、附件、字型授權及原版素材排除守門通過。Linux psgolem -version實際輸出同一完整版號。三包大小分別14277105、14623584、28371549 bytes，SHA256SUMS由正式打包流程產生。

這是新候選建置與封包結構，不是新Windows／macOS真機、GUI或完整HD驗收。原已通過178載入／渲染測例與175實體GPU案例承接，沒有重跑舊美術。原生觀測工具以唯讀首存卡與正常按鍵接續新區域，路線仍停在煉金研究棟，未到待驗目標；Clone只用於研究分支，最終腳本尚待乾淨重播，不當作新的正常路線完成證據。實際安裝版本與素材保持，完整HD與閃爍仍未完成。研究容器已正常EOF退出，原資料沒有公開。


## 2026-10-23：原生前置路線的完整重播與新存讀檔

本輪沒有修改正式Go來源、GPU程式、原版routine、資料格式或三平台候選。相鄰私人素材專案的015一次性唯讀逐場probe核對現用房間圖層與原生運動；先重現前輪端點的全部RAM／PNG，再以正常按鍵接續新存檔室。研究Clone只用於選路，最終完整按鍵從真BIOS與唯讀原首存卡無Clone、無PC／RAM／道具注入重播至15004，14694的全部2097152-byte RAM及原生PNG與研究端點完全相同。

初次只有覆寫確認、卡片未改的停止保留，沒有當作存檔成功。正常確認後寫出的新卡另由真BIOS與檔案選單冷開機至9000，回到新存檔室；讀檔前後卡片完全相同，原卡保持，故由遊戲實際讀取證實卡片可用。這限本引擎原生前置存讀檔路線，不是待驗新HD區域、HD GUI、獨立硬體oracle或速度驗收。所有原版、研究畫面、RAM與卡片只留私人研究區，不附本repo。

既有178個載入／渲染測例與175實體GPU結果承接，新增正式GPU／美術／安裝GUI案例0。v.0.7.7-20261023候選仍未安裝，已交付雙擊版保持。正常目標區域、完整HD與閃爍未完成，三個研究容器皆正常退出並刪除。


## 2026-10-23 完整HD包的無損圖塊內索引

沿容量Issue #14與019 §8.1，完整候選在原8圖塊仍超過256MiB。私人只讀診斷得到原278962504 bytes，新表示132782541 bytes，122966208唯一圖塊色字逐字往返。先建立DRAFT、審查及READY，再接入019 §8.2；manifest v1、PNG／STP、資源界線、API及玩家設定保持。

僅typed資源不足時在尚未交回的Load內轉為8圖塊內的完整uint16盤與0／1／2／4／8位元索引，無收益則保留原64字。模式byte、盤、索引、圖格與姓名均計帳。舊可接受線性／32／8表示保持，碰撞逐字確認，內容錯誤不重試，不寫renderer快取、CPU或RAM。

實作準備器的entries錯誤／memory定位出現兩次而停止，當時產品測試0；部分本次修改與產生器另存素材研究根，核對基準後只還原該檔。改唯一分支定位並在所有替換確認後才寫入，規格與表示未變；不將此準備失敗列為產品缺陷。

唯一render/hd及render/hdgpu回歸184項含子測例通過，失敗／skip0；包含所有編碼、0／0x8000、碰撞、邊緣、原表示、實際PNG容量轉換、晚期非法alpha及姓名合計容量。八個新表示合成Intel MTL實體GPU案例完整VRAM、畫面及統計相同，全部單一硬體精靈、Triangles0／Fallbacks0／Readbacks1；原素材115例及其他已完成矩陣不重跑，沒有新全套race。

完整2999條目正式載入及資料核對一次通過，482090816父像素、29622272新預期字、1822464邊界、四姓名保持，常駐資料132782541 bytes守原268435456，非程序／GPU峰值。來源包與原失敗收據不改，所有原版／母稿／圖板只留相鄰素材專案，公開repo不含遊戲資料。§8.2只標載入及資料取樣CONFORMED；正常新區域、1080p與玩家包未驗收，速度調校延後。具體SHA、命令、184／8及完整資料由素材專案LIB入口storage-study-v1/storage-acceptance-v1.json索引。


容量實作及規格以d543ab1提交推送；從該乾淨提交建置v.0.7.8-20261023三平台純引擎候選至dist-all同版本patch目錄。Linux／Windows各6檔、macOS8檔逐檔與封包SHA相同，版號、設定入口、授權與素材排除通過；Linux實際-version正確，macOS兩程式雙架構、簽章、最低11.0及系統依賴四道通過。Windows／macOS無新增真機，沒有新GUI、AppImage、tag／Release或玩家包切換。候選收據為素材專案LIB儲存研究根candidate-build-verification-v1.json，公開容量#14進展留言6367440165。後續背景已接DAI原始來源DRAFT，完整HD與速度仍未完成。


## 2026-10-24：有界清單容量

019 §8.3依DRAFT、證據審查與READY後實作，只將清單上限改為8192條／4MiB。v1 schema、來源識別、PNG／STP與256MiB圖像資料計帳保持，正式程式僅改兩個常數。

新增自製4×1索引PNG與不同合成來源鍵測試：8192條跨越舊兩道界線仍逐條精確；8193先於PNG拒絕，4MiB精確成功及加一拒絕，最後一條壞PNG／重複鍵仍拒絕整包，輸入檔案保持。

150項HD先通過；hdgpu因漏設DISPLAY在測例前停止，初次環境紀錄保留，只補有界xvfb-run執行未開始的40項，未重跑已通過HD。合計190項回歸及新增容量六項race通過，失敗／skip0。019 §8.3只驗收清單契約，未建立Release或三平台交付聲明。本輪有界測試容器與虛擬顯示器已退出，未重建image或清理其他專案資源。

## 2026-10-24：外部海報切換與安全融合

使用者核准三款海報與柔和融合原型後要求正式接入。依024先保存來源／邊界、READY及索引，再實作通用外部PNG成對清單、完整預載、前端GPU外側九點融合、逐張黑邊保護及工具列／可攜設定保存。未修改machine、GPU原生核心、遊戲地址或HD素材格式；所有遊戲美術留私人素材repo。

前端與命令列40項通過，既有可選手把截圖fixture skip；真實BIOS一場的Digest與卡片前後不變。實際Linux視窗A／B／C／關閉與柔和／放大保存通過，正常原BIOS／首存卡及既有腳本至8330，獨立重開706，兩次最後Draw完成，卡片保持。Xvfb使用軟體OpenGL，不是硬體GPU或速度驗收，沒有整段原版CPU新對拍。

集中截圖發現三個新標籤高度不足，保存原圖後只修一行水平inset。最終程式實際字形像素106／62／70通過，正常遊戲不重跑，其他凍結來源不變。首次缺XTest及既有材質包載入69.118秒超過30秒開窗等待、另有標籤固定畫面數等待停止，均另存環境／腳本紀錄；沿已查明原因修正驗證，不改產品參數掩蓋失敗。

Windows amd64與macOS雙架構編譯及格式核對完成，未做兩平台真機或新版封包，既有正式版不覆寫，沒有tag／Release。024僅在上述Linux呈現與保存範圍CONFORMED。公開repo僅有通用源碼、規格與自製合成測試；來源、二進位及收據由私人素材專案海報索引的production-v1根保存。所有容器有界並自動回收，未新增image或清理共用資源。

## 2026-10-25：有界圖像容量與RNO1接續

完整私人6035條HD候選已在既有256MiB及全部無損共用表示後於entries[5881]停止。獨立512MiB診斷需273052733 bytes，原素材、色字與透明／STP精確；由相鄰素材專案的唯一現況與逆城來源索引進入，原版與畫稿不進公開repo。

先保存a3f5f74原來源及量測，依019 §8.4形成READY，再只將正式textureMemoryLimit從256改512MiB。v1、8192條／4MiB、單圖與內容守門保持，所有表示及姓名沿同一總量計帳，超界仍nil pack。較高預算可改變內部表示選擇，整數像素契約保持。這是完整素材需要的容量修正，沒有延遲載入、刪圖、降解析度或效能調校。

一次render/hd回歸155項與新增／小型容量路徑race17項通過，失敗／skip0。原65條跨256MiB及晚期壞PNG測例改用明示舊預算保留原轉換覆蓋；自製微型PNG新增舊界線以上、正式界線、零及超界的預算守門。未重跑原CPU／GTE、已定案美術或舊GPU矩陣。

正式新入口完整載入6035條，67108864新預期字、5329216選區字、61779648區外字與alpha／STP精確。常駐圖像參照191468021＋圖格81541704＋姓名43008＝273052733 bytes，守512MiB；非程序／GPU峰值。5779父條目及全部父檔、四姓名保持。

原258來源／未知控制及59靜態區塊共317例完整VRAM／畫面／統計與獨立預期通過，Fallbacks0／Readbacks1，僅來源衍生圖元，不作實體GPU或正常新關卡聲明。426.2823秒是研究驗證耗時，不作遊戲速度。原256容量停止與接續驗證器誤讀舊READY的腳本停止皆為0例，原紀錄保留；僅修驗證器一個計畫路徑字面值後接續未開始案例，不改素材／預期、不重播完成案例。美術仍原一次定案。

019 §8.4只在載入與上述繪圖資料範圍CONFORMED。實際新來源已建置Linux研究入口，正常RNO1／新包GUI、聲音、速度、通關及三平台交付仍未驗，既有正式v.0.7.7不切換，沒有新tag／Release。完整HD與其他背景／全部怪物繼續。研究、來源SHA及停止／接續收據沿私人素材repo的rno1-completion-v1索引；公開只提交通用來源、合成測試與文件。

## 2026-10-25：繪圖接管與實際寬視野研究

- 使用者要求避免原生與HD重複繪製，並擴大實際可見場景。唯讀參考MM3 ec4570149的074與RenderControl，確認其已完成呈現交易，直接省略像素迴圈仍為DRAFT，未搬程式碼。
- 新增並索引025-hd-primary-native-demand.md，狀態DRAFT。現有HD輸出已直接選影子，原生GP0仍光柵化；公開VRAM陣列／切片、GPUREAD、複製、貼圖回授與完整克隆均屬接管契約。真寬視野沿既有017，每列17改23格、HD中央寬342，不把單純升解析度算視野擴大。
- 私人素材專案probe_native_render_contract.go沿保存的226封包、167精靈作一次有效負對照，略過原生精靈後GPUSTAT相同，VRAM差25921字，後加受控讀回／複製顯露差異。原始工具缺列01快取清除而停止已保留；沒有正常DRE讀回頻率、FPS、完整延後生成或新正常GUI證據。來源與收據在相鄰素材專案workplace/render-control-v1/。
- 正式引擎程式、既有候選與遊戲資料不改，不升READY、不重跑素材或美術。下一步為正常依賴觀察與同步介面，未完成事項在素材Issue #13／#3列管。

## 2026-10-25：正常原生需求與視野偏好工具列

- 025保持DRAFT，補回正常開機／同卡讀檔／移動至8330的兩機中立觀察。四歷史正常摘要相符，七點完整RAM／原生GPU、CPU GPR／摘要與卡片一致。最後170場CPU像素讀回0，但貼圖快取18283字及混色54023次依賴原生圖元；不能空跳原生。非像素GPUREAD呼叫2430及Status呼叫5384分開記錄。正式原生觀察位置15處，不宣稱外部陣列別名全涵蓋。
- 既有v.0.7.9程式與最新HD包正常寬視野至8330，九處改字、卡片與1368×960擷取核對；首存房間尺寸1／1，沒有大場景正向證據。正常向右探索至9600仍在牆內，保留輸入與失敗，不注入狀態或重試挑圖。原資料留相鄰素材專案workplace/render-control-v1/。
- 017 §8先READY再實作：工具列保存原版／擴大視野供下次啟動，本次cfg／RAM／AfterField保持；主程式保留待套用設定，其他偏好保存不覆寫。兩項TestWidescreen新增測試一次通過，Linux開發執行檔已編譯。
- 實際設定GUI保存／獨立重開／復原已完成。兩次工具停止為省略空值預期及固定更新等待，原來源與未顯示工具列舊圖保留；完成動作不重播，最後只補實際可見文字及正常WM關窗。三個視窗分段收據只接受Linux設定介面，沒有新正常遊戲、Windows／macOS重編、三平台交付或速度聲明。現行v.0.7.9／素材／原媒體與存檔保持，新工具列未安裝進該包。
- 核心GPU／CPU／Machine.Step沒有正式改動，研究覆蓋不入production。新UI源碼、工具、首停與接續收據索引在素材docs/assets-catalog.md#render-control。後續需交付新控制，並完成HD主繪圖及更寬房間路線；完整HD仍未完成。

## 2026-10-26：視野控制新版候選準備

- 新候選採v.0.7.10-20261026，本機與遠端tag及版本目錄均未使用。017 §8既有工具列與兩項測試、分段Linux設定GUI證據承接，不重審美術。
- 封包內遊玩說明補上F1／頂緣視野開關、保存後下次啟動及回原版後恢復海報。現行v.0.7.9與素材、設定及存檔維持，待新封包與本機入口核對後更新唯一現況。

## 2026-10-26：新視野候選交付與完整圖層勘誤

- e58f8d3乾淨來源沿既有Docker工具鏈一次建置七包，Linux實際版本／動態庫與AppImage解出payload相同，macOS四道結構及素材排除通過。相鄰素材專案以8257ad7組裝v.0.7.10-20261026本機三平台候選，沿用已接納素材與各平台設定。新Linux正常BIOS／首存／左移8330、frame／shown2277、final_drawn=true；保存下一次寬視野，一次獨立重開確認啟用後保存回原版，1217正常關窗。重開最後在途frame145未呈現、shown144，只接受已見工具列與啟動／保存／關窗。原卡與測試副本、其他設定保持，最終偏好原版。
- 新交付manifest1741c5fe，正常GUI1、重開1、集中呈現核對1，美術審查0、原矩陣重跑0。軟體Mesa／2CPU，不是實體GPU或速度通過；Windows／macOS真機、正常DRE、更大場景與完整HD未完成。新候選與收據只留本機，公開純程式仍不帶原版資料，沒有tag／Release。
- SCROLL-VIEW-20261026：唯讀既存正常RAM與關卡解析索引，47筆NO3圖層定義的完整宣告陣列與來源相同，其中10筆寬於256。定義可能共用陣列；初次唯一指標假設停止後，按執行時矩形解別名，原停止保留。撤回「界限外沒有圖塊」的過強推論，017 §9與025 §7回填，原圖及歷史失敗不改標。資料載入、捲軸、命令與裁切分開；其他圖頁／色盤、跨房遮擋仍未知。離線互動示意只用既有原版前景，不宣稱正常接管。
- MM3參考更新為366104d的075，限定中文普通／鏡像直接命令已CONFORMED，保留相容資料及CPU／RAM／時間；GPU／SIMD未實作，076未追蹤草案不採為事實。只讀方法、不搬程式碼，月下025仍DRAFT。研究索引沿相鄰素材專案docs/assets-catalog.md#render-control。

- 正常跳躍出口一次成功：原程式於8293進入512×768房間，原簽章、卡片保持，到8382，沒有注入。原始VRAM前景93種來源圖塊與七完整色盤皆相同；五整頁一頁相同，其他整頁結果與來源保存，不從整頁差異推成必要圖塊缺失。接續只讀局部資料，沒有重跑遊戲或整頁。
- 相同固定正常輸入，以e58f8d3現行psexe／已接納包及017作一次HD4擷取，exit0、8382、1368×960、九改字與原卡保持。右側新增142416非黑像素，石柱／牆面接續，左側界外黑。一次集中技術檢視、新美術審查0，527.92秒不是FPS。兩份終點捲軸0／319與人物105／136相同；中途8370捲軸290曾被誤當終點差異，已直接核對兩份最終RAM更正，沒有重跑。完整CPU／RAM同狀態未驗、寬視野沿既有決定性例外。僅此房間右側單點接受，沒有跨房、全場景、實體GPU、GUI或速度聲明。
- 接受入口為素材專案render-control-v1/scroll-view-v1/normal-room-view-acceptance-v1.json，兩張原圖不編輯，只存私人repo並記provenance。本機比較頁normal-room-v1.html與現況索引已連結。新候選根manifest只加限定證據欄位，平台檔案不重寫；原1741c5fe索引與v1收據保存，v2索引SHA 3509d95959e38da3d6014520e0b79e222699816a5fcca85d83f713cf97053fe6，full_HD_complete仍false，原版偏好與全部存檔保持。

## 2026-10-26：一倍原生相容平面原型

- 上輪屬進展：v.0.7.10控制已交付，正常大房間右側延伸與47筆完整圖層、93種實際圖塊駐留已取得限定證據。本輪兩庫乾淨開工，沿復古逆向／規格技能及唯一現況，不重跑舊圖或美術矩陣。
- 新增prepare_native_plane.py／probe_native_plane.go，僅用Go覆蓋保留原生解析器、時序、TPage與折線游標；無美術N1平面供應像素／CLUT／貼圖快取，控制端跳過像素工作。每封包／正常field阻塞fence，原始輸入與正式引擎唯讀。
- 首次CPU前兩例通過，第三例奇數C0讀回得到0而停止。根因是現有HD顯示影子的C0只invalidateTexCache，不啟動已有fbReadStart，不能直接當相容平面。來源／二進位與exit5保留，Xvfb收尾訊息分開。研究覆蓋才補讀回起始／消費，不改一般HD。前兩例只重建必要前綴承接原證據，另19例CPU新檢查與21例GPU新角色全通過，沒有重審美術。
- 完整GPU含像素快取、逐字GPUSTAT／IRQ與讀回相同，克隆隔離通過。GPU三三角形／25精靈／2568像素，兩次CPU回退／11回讀；使用軟體Mesa，當前預設容器glxinfo確為llvmpipe，不宣稱新實體GPU或速度。兩次獨立正常開機／同卡讀檔、移動與跳躍8382，七點完整RAM／GPU含CLUT與貼圖快取／摘要／卡片相同，控制側562672精靈、98887多邊形等像素處理由N1 CPU平面承接，不宣稱省掉相容運算。
- CPU／GPU公開VRAM陣列負控制都證實同步前過期。下一正式切片先收束原生觀測／別名，再同掛HD4主畫面與相容平面；025保持DRAFT。正式候選、7365條素材、海報、設定、媒體及存檔保持，正常GPU平面、實體GPU、HD4同掛、GUI／速度與完整HD未驗。
- 原執行runner及來源雜湊按intake-v2保留，提交來源只做gofmt，另存格式化副本逐字比對。accept_native_plane.py只讀收據與原始輸入，research-verification-v1.json接受限定研究，原v.0.7.10交付manifest3509d959不變。兩庫來源提交、Issue回讀與Docker清理另存本機publication，不用研究通過取代正式交付。

## 2026-10-26：HD4與GPU相容平面同掛、正式顯存觀測封裝

- 沿既有復古逆向／規格路由與文件職責，承接前批N1研究；新native-dual-v1保存獨立覆蓋與5da8162完整來源tar，不改前批來源／二進位。GP0／GP1／場奇偶分送兩角色，精度及素材場只送HD4，C0讀回僅N1，原生觀測自動同步。CPU／GPU各三組契約及直接公開陣列編譯失敗負對照通過，沒有呼叫端手動同步。
- 正常真BIOS／同卡兩機、完整已核准素材、CPU HD4／PGXP／透視，候選另掛GPU N1；一次至8382，七點完整RAM、原生GPU含CLUT／貼圖快取、摘要、卡片及完整HD4 VRAM／畫面／統計相同。1024×960兩張原圖逐RGBA相同，一次集中技術檢視；GPU N1有115766三角形／499453精靈，PGXP命中135906。789.58秒是雙機阻塞研究耗時，不是遊戲FPS。軟體Mesa，不是兩個GPU裝置、實體GPU、新GUI或速度驗收。
- 025 §9先獨立READY，再將原生陣列私有化，新增GPU.VRAMSnapshot並接Machine.VRAM／Digest；像素熱路徑只改欄位名稱，既有同步繪圖保留。所有權、原生GPU、PGXP、仿射中心及M3台帳回歸通過。M3五項既有非零差異保留；TestShadowPS1GPU的17項通過，只比較自身同步後端。第一輪全編譯誤掃workplace舊研究六項失敗，原日誌保留，219正式檔乾淨來源同命令編譯23套件通過；只補原篩選漏跑的HD測試，沒有為舊研究修改正式程式。
- accept_native_dual.py只讀收據、固定來源tar與正式編譯來源，接受雙平面研究及獨立觀測封裝，後者CONFORMED。025主規格仍DRAFT，正式原生像素分工、雙GPU生命週期與玩家接入尚未完成。v.0.7.10雙擊候選程式、manifest3509d959、7365條素材、設定與卡片保持；新美術審查0，已完成素材矩陣不重跑。資料與原圖只留本機，工具／規格可提交，沒有tag或Release。
- 收據及本機對照頁沿素材docs/assets-catalog.md#render-control進入。兩庫提交／遠端Issue回讀與Docker所有權／容器清理結果另存native-dual-v1/publication-v1.json；後續保持整體HD目標，不將此切片稱為全HD或全場景寬視野完成。

## 2026-10-26：正式相容平面與實體Intel正常路線

- 沿復古驗收／規格及回填路由，上輪正式接線、CPU／GPU三組與回歸屬進展。本輪續同一個6301／63072程序，沒有因觀察逾時重啟。原軟體雙機1800秒停止，四點相同、最後記錄6900；GUI確認實體環境後停止，最後報告4800，原結果不改稱通過。
- Ada客戶端先缺UID1000的NSS對應，補該容器passwd／group映射，不建新image、不搬憑證。橋接與主機路由SSH均連線逾時，沒有啟動遠端工作或移轉原版資料。代理端不見GPU節點屬命名空間差異；唯讀確認Docker主機renderD128／GID992及card1／GID44後，沿DRE既有WestonGL／Xwayland／Intel MTL工具鏈。GLX確認Accelerated=yes。
- 025 §11先READY再實作NativePlane與兩入口，保留解析／時序／游標，像素由一倍無美術平面供應；快照／Screen／Clone／Digest／讀回／卸下同步原字與快取。事件4096後有界fence，前端完成通知等兩角色，正常Close在Cancel前。Trace前置觀測及異常返回清理等待後補，只有針對性競爭測試及玩家重建；最後實體GUI直接用包含兩修正的來源。
- 正式CPU／雙GPU各三例通過；競爭檢查原1／2／4倍與新所有權／角色／關閉通過，8倍達時限，只接未完成8倍非競爭檢查及未開始影子，不重跑前綴。M3原18筆台帳、17項機器HD、PGXP及23正式套件編譯通過，五項原非零差異保持。原225檔及最後三檔精確差異、新測試保存。
- 首次實體GPU正常兩機8382七點完整RAM／GPU含CLUT及貼圖快取／摘要／卡片與HD4畫面／統計相同。最後正式GPU前端正常8382、1920×1080、frame／shown=8270、final_drawn=true，原卡及副本保持，正常關閉通過。硬體GUI首次副本漏執行模式，Popen前停止、遊戲未開始；原目錄／收據存GUI-prestart-stop-v1，只修模式後接續同一首輪，未重播遊戲。一次集中技術檢視、新美術審查0，無速度或硬體exact聲明。
- accept_native_production.py接納025 §11，其他圖文交易、跨房視野與完整HD仍待完成。021回填NATIVE-PLANE-ROLE-20261026，普通HD隔離與精確原生角色分開。新1080p原PNG只依授權保存私人repo；公開引擎不帶原圖。v.0.7.10程式、manifest3509d959、7365素材、偏好與卡片保持，來源未安裝。兩庫提交／push、Issue回讀及Docker清理保存native-production-v1/publication-v1.json，下一步交付新來源並續其餘HD，整體目標保持。

- 收尾補記：實體正常容器退出0、1909秒，原外層軟時限124保留。GUI程式退出0且最後幀8270已畫出；外部X11搜尋未定位的原false收據保持，另以已存Layout／Draw畫布、設定／卡片／原PNG接納，不宣稱外部幾何搜尋通過，沒有重啟遊戲。未來啟動器已補轉送TERM與強制kill-after界限，僅做bash語法核對，原執行腳本及SHA保持。

## 2026-10-26：v.0.7.11交付與跨房間候選定位

- 純程式以0e527b4乾淨來源建置七包至dist-all/v.0.7.11-20261026/patch，macOS結構、ZIP／SHA、Linux依賴與AppImage實際payload通過。三平台含媒體包僅組入相鄰素材專案既有local-full例外；舊v.0.7.10與卡片保持，無tag／Release。
- 新Linux雙擊包一次正常真BIOS／同卡讀檔、移動／跳躍8382，CPU HD4／1920×1080，frame／shown=5664、final_drawn=true；一次保存設定重開至605正常關窗。全檔、媒體及卡片保持。新美術審查0、集中技術檢視1，實體GPU證據直接承接，速度與完整HD未完成；025 §11.5只接納此交付範圍，主規格仍DRAFT。
- 按使用者要求核對mm3的075／076，沿用直接命令與必要相容資料的契約，不移植程式碼。既存大房間終點的左43像素在靜態矩形下分別落在兩個鄰房，屬強推論候選；沒有新遊戲執行或正式寬視野改動。017補證據界線，實際場景工作另立Issue #18，#16只處理序章框外直條。
- 兩庫提交／push、遠端Issue回讀、交付manifest與Docker清理收據留在相鄰素材專案native-delivery-v1/publication-v1.json。公開repo不加原圖、媒體、RAM、素材包或卡片。

## 2026-10-26：鄰房前景與既有HD來源

- 沿017 §10的DRAFT研究，僅用既存8382 RAM／VRAM與原正常M4傾印。IDA一次性9.4輸出與原MIPS前景控制124精靈、兩鄰房9／33精靈完成，42個封包的位置／TPage／UV／CLUT相同。受控fixture停於背景入口，不稱正常玩家或CPU／RAM／時間等價；原API及函式建立準備停止保留。
- 26號房靜態F_GAME失配已由原20字×16列正常上傳來源解開，完整字與保存顯存相同；42格全有現行HD條目，9張核准圖像的離線左側前景完成，中央／右側逐RGBA保持。新生成／美術／遊戲／舊矩陣重播0、集中技術檢視1。原版輸入與圖只留私人素材研究根，不加公開repo。
- 背景、特殊層參數、遮擋、實體與正常場景接入仍DRAFT，正式Go程式及v.0.7.11不變。兩庫提交／push、Issue回讀、Docker與擁有權收尾由相鄰素材專案neighbor-bindings-v1/publication-v1.json保存。

## 2026-10-26：背景捲軸與正常鄰房命令研究

- 沿017 §11的DRAFT切片，既有DRA資料庫唯讀，只切三函式8784 bytes。kind1原捲軸三例各25步，整數高半複製、小數及其他RAM保持；16號房選定九ID為零，完整原背景呼叫零精靈／24模式符合資料。0x20分支原a0=3及既存正常世界座標連續另有原bytes／收據。
- 原尾端ED520的研究BREAK5756不在延遲槽，補封包後恢復SP原動作；正式Go未改。一次受控360→402精靈，49143步／263687模型週期及所有GPR／COP0相同，RAM僅宣告繪圖區，關閉恢復原bytes，42個payload對到前批原MIPS。
- 唯一正常BIOS／讀卡／向右與跳躍到8382，原DMA實際送出3342／3384個新封包，末42仍待送；左側新增65939場景像素，中央／右側逐RGBA保持，卡片保持。85次補入、1804次模式略過未逐項定位，不稱全場景；一次集中技術檢視，新美術與舊矩陣重播0。
- 程式退出0、1284.80秒屬整合驗證耗時；最終畫面同步期間接受工具曾因收據未完成前置停止，只讀接續同一趟。正式提供者、遮擋／實體、其他視差與新GUI仍DRAFT，v.0.7.11與全部素材／設定不變。原圖僅存私人素材repo，公開不加圖、RAM、卡片或包。來源／收據／比較頁／發布與衛生檢查由相鄰素材docs/assets-catalog.md#render-control的neighbor-scroll-v1索引。

## 2026-10-26：缺背景模式補入與NO3正式來源

- 沿017既有研究、原位址回填路由及唯一現況，26號房的背景桶為空，原鏈後續首包重設E1／E2；原ECCD8／ED458提供0xC74、12-byte及1024界限。新原型只在可恢復原狀態的鏈補模式。原MIPS受控250→298精靈、24→25模式，28104步／152864模型週期、全部GPR／COP0相同，48個封包及宣告RAM核對通過。
- 唯一新正常BIOS／讀卡／左移至8330，右側新增142416場景像素，中央／左側逐RGBA保持；88416個精靈中88368觀測送出、末48待送，1842次模式補入、模式略過0。48格原生字／色盤及三張既定HD圖精確鍵通過，卡片保持。480.77秒為整合耗時，不作速度判定；新美術0、集中技術檢視1，原圖只留私人素材repo。
- 先READY再接009 §7與017 §12。ReadFile增加階層及讀取上限；Widescreen透過既有cue獨立唯讀原片、驗原檔，再逐格比原生字與色盤bit15。兩個程式入口設定同一媒體路徑，沒有新設定格式或素材。第十字／第四陷阱保留SP原動作與載入延遲，Check／Disable十字一致。
- 原根BOOT及兩來源完整SHA與工具界限通過；首批月下因測試fmt未用而未開始，只接未開始月下，原光碟案例直接承接。兩個保存狀態的正式原封包／ABI／RAM／還原及八防護通過，簽章／陷阱必要回歸通過；舊可選擷取skip，未重播。補CLUT座標512列界限後只驗新負例，舊正例在界內；最後兩入口編譯通過。回填護欄要求完整原位址，補0x800F06F8後三鍵通過。
- 009 §7限唯讀工具CONFORMED；017 §12保持READY，已實作與受控通過，正式正常GUI／新包、動態色盤／過場連續性、遮擋／實體與全HD未驗。現行v.0.7.11及全部素材／設定／卡片保持。公開只加程式、測試與文字，不加圖片、原媒體、RAM或素材包；原型正常證據不能改稱正式GUI。
- 固定來源490b62a、凍結覆蓋、正式日誌／源碼雜湊、原圖與來源、backlink護欄及發布回讀由相鄰素材docs/assets-catalog.md#render-control的neighbor-modes-v1／production-v1索引。舊研究重建用保留tar，不對新介面套舊覆蓋。後續先交付新來源，再續未完成HD；效能另沿Issue #9。

## 2026-10-26：鄰房正式來源交付的唯讀觀測

- 正常新包須區分原型及正式來源，補Widescreen停止後唯讀NeighborFrames與-stats日誌，沒有改場景選格、來源核對、RAM或設定格式。新交付只驗一次新正式寬視野GUI，使用獨立可攜設定明確開啟，原偏好／卡片保持；既有設定重開與美術矩陣不重播。
- 建置沿005既有七包與Docker工具鏈，原版不進可寫建置樹；新版本與交付結果由相鄰素材neighbor-delivery-v1保存，GUI未完成前不改稱已驗新包。

## 2026-10-26：正式鄰房來源新包交付

- 純程式以fdf8eda乾淨來源建置v.0.7.12-20261026七包，原版未掛入可寫來源。macOS結構、ZIP／SHA、Linux依賴與AppImage實際payload通過；含原版只組入相鄰素材既有local-full例外，不發布tag／Release。
- 唯一正式封包正常GUI使用逐位元相同可攜副本、唯讀素材／媒體及獨立明確寬視野設定。真BIOS／讀卡／左移8330，退出0，frame／shown2634、final_drawn=true、1920×1080，NeighborFrames1842。原版安裝偏好及卡片保持，既有設定重開、CPU／GPU矩陣與實體GPU證據承接，沒有重播。
- 一次集中技術檢視接受右側石材／紫色牆／橫邊的實際場景。左黑區、跨房遮擋／實體、動態色盤過場及其他圖層仍未完成；017 §12只限此普通圖塊與路線交付CONFORMED。新美術0，917.42秒屬軟體Mesa整合耗時，不作FPS或速度判定。
- 交付manifest128ce912、未修改原圖99b4d065及完整版本／來源收據留相鄰素材neighbor-delivery-v1，沿其docs/assets-catalog.md#render-control進入。前版v.0.7.11及全部素材／設定保持，Windows／macOS待真機，完整HD仍未完成。公開repo只有程式與文字；Issue與提交／Docker收尾另由publication-v1.json索引。

## 2026-10-26：使用者重申Go直接繪圖

- 使用者明示正式接管要以Go直接取代原遊戲繪圖，原ASM僅供資料與行為證據。已回填AGENTS及025 §12，將既有相容平面與017補鄰房命令明確列為支援／過渡能力，不當接管完成，不再以原繪圖迴圈執行作為HD主實作。
- 後續Go場景／精靈／文字由apps/sotn解讀遊戲資料，直接交HD CPU／GPU後端。必要RAM／顯存／快取／返回／時間仍須證據，不自行放寬決定性。第一切片承接保存NO3資料、原函式與封包，先做純Go直接繪圖，再補正常入口替換與必要相容效果；主規格仍DRAFT。
- 同輪私人素材已完成兩圖集22組德古拉色盤及一次24例CPU／GPU、原命令串流；只更新外部素材，正式引擎Go來源保持fdf8eda。原圖、母稿、素材包與命令不進本公開repo，來源與限定收據沿私人素材索引。

- 純Go前景原型已實際跑通，來源為相鄰素材Go工具及已索引direct-go-v1。直接讀保存NO3資料建立型別化材質／圖塊，直接寫Go CPU像素／GPU RasterJob，沒有Machine、CPU.Step、Shadow或GP0主路徑。原ASM與保存封包只作oracle，沒有重跑原routine。
- 一次新驗證：原版109可見圖塊／寬342的154格，各7材質；原MIPS可見封包身分及143520原生PNG像素精確，2296320 HD4 CPU／GPU色字精確。原圖留私人研究根，一次技術檢視、新美術0，188.58秒含整包載入不作速度判定。正常入口未接，原呼叫返回／RAM／顯存／時間與其他圖層／角色／文字仍DRAFT，025 §12.3明列範圍。
- 025標題與較早按需研究段落已區分Go主路徑及相容層支援，避免再將原生按需效能研究當成Go接管完成條件。既有§9／§11接納保留，正式引擎Go來源與本機程式仍fdf8eda；新德古拉素材7387條外部更新由私人素材獨立收據列管。


## 2026-10-26 Go完整呼叫資料與背景合成研究

025 §12.4–12.5回填四個完整原呼叫oracle、Go高階圖層／相容緩衝與新兩圖層直接繪圖證據。原資料與實作研究器保存在相鄰私人素材專案tools/probe_sotn_draw_call.go、probe_sotn_draw_plan.go及probe_sotn_direct_scene.go的room模式；不把原圖、RAM、BIOS或封包放公開repo。本repo本輪沒有改正式Go來源或可玩包。

Go相容計畫四例各2096896個非堆疊RAM位元組精確，堆疊50／50／33／50差異及CPU／scratch／週邊時間保持待解。原oracle第一次在執行原指令前因空GTE觀測器序列化停止，首次來源／日誌保存；只修空值處理後首次執行四例。沒有重播舊美術或正常玩家案例，模型週期不當硬體或速度證據。

新普通背景／前景合成原版294格、擴大342格，各19材質；143520原生像素與獨立圖層PNG合成精確，2296320 HD4色字CPU／GPU精確。Go繪圖路徑沒有Machine、MIPS或GP0，一次技術檢視、新美術0。GPU為軟體Mesa，356.08秒含完整素材載入，不稱新實體GPU、1080p GUI或速度通過。正常遊戲入口、其他圖層／角色／文字與完整HD仍未完成，025主規格及§12保持DRAFT。

研究根為相鄰素材workplace/render-control-v1/direct-go-v1，由其docs/assets-catalog.md#render-control索引。來源／構建／執行／技術收據及最後Issue、commit、容器清理核對分別保存。前批前景精確來源另存，不以擴充後工具冒稱舊二進位。正式v.0.7.12交付與原版模式不變。


## 2026-10-26 真實caller與Go完整儲存狀態

025 §12.6–12.7新增原0x800E3C24→0x800ECC10→0x800E3C2C的IDA直接xref與正常機器證據，原名／bytes保留，不外推全部間接呼叫。DRA／boot／來源DB SHA由規格及私人固定計畫保存，原資料、資料庫、圖片與CPU收據不進公開repo。本repo仍只改規格與工作記錄，正式Go來源及候選不變。

素材專案tools/probe_sotn_live_draw_call.go從真BIOS及同卡正常讀檔移動到8382，再自然Step捕捉一次原呼叫，未注入狀態。原呼叫42321指令、59271模型週期，該次I/O及中斷0；卡片保持。這不是正常Go接管或速度聲明。完整CPU／機器時鐘、期限、RAM／scratch／VRAM及存取供下一時間切片使用，原四個受控函式不重播。

tools/probe_sotn_draw_state.go已用Go高階資料流程產生完整RAM、scratch及CPU值。五份保存狀態各2MiB RAM、1KiB scratch與12個CPU欄位精確，35格GPR、HI／LO與返回流程均在範圍內；Go狀態流程沒有Machine、原MIPS或GP0。CPU的now、cost、muldivDone、icache與週邊事件仍待接，正常Go入口及完整HD未完成，主規格維持DRAFT。

GO-CALL-STATE-20261026明文回填§12.2–12.4的早期未知，對照及自動護欄在私人direct-go-v1/call-state-resolution-v1.json。舊收據保持原範圍，新美術／技術畫面審查／舊繪圖矩陣重播0。唯一私人§8與網頁已同步，後續來源、Issue與清理收據由publication-live-state-v1.json列管。

補充本輪證據界限：正常呼叫沒有HD影子或PGXP，CPU.precision／pgxp.Tracker不在12欄位比較範圍；後續HD接線須保留並驗證精度追蹤及原生／HD排序效果。此補充沒有改動已驗Go來源、二進位或收據。


## 2026-10-26 Go時鐘與正常原生入口研究

025 §12.8–12.9記錄五態完整16個CPU欄位及原生Go入口新證據，GO-CALL-CLOCK-20261026回填§12.2–12.7。時鐘足跡不讀opcode或計算繪圖，繪圖仍由高階Go資料生成。原真實狀態42321指令／3947 RAM讀取／1162取指成本，共59271模型週期，不使用人工227393固定值。首次helper未用fmt導致建置停止、案例0；來源／日誌保存，只修準備後首次五態比較通過。

相鄰素材的新正常真BIOS／同卡路線以Go接管60次原呼叫，8383至8442的60場完整Machine.Clone正規化值相同，略過0、卡片保持；支援呼叫CPU.Step／MIPS解碼0。研究覆蓋只加返回資料、原生所有權與週邊轉接，正式Go來源未改。無HD／PGXP，跨IRQ／field及守門負例尚未驗，不能稱正式HD接管。新正常路線1條，新美術／畫面檢視及舊GPU矩陣重播0，46.76秒不作速度聲明。工具、來源、覆蓋與收據在私人素材direct-go-v1，由其docs/assets-catalog.md#render-control索引；接受入口tools/accept_sotn_go_entry.py只讀收據。025保持DRAFT，接續HD型別化工作、原生相容排序與PGXP；正式候選不變。


## 2026-10-26 Go圖層直接進HD排序研究

025 §12.10追加DRAFT限定證據，正式Go來源與雙擊候選不改。私人研究覆蓋將Go自有圖塊在原DMA排序位置直接交HD光柵接口，原生相容仍接相同命令。新正常真BIOS／同卡路線一條，六次Go呼叫／六場完整原生、CPU HD4顯存／畫面與素材統計相同，1550型別化工作、4650個HD命令字免解析，末310格未送出。五負例與三個CPU／軟體GPU合成排序案例首次通過，後者完整25165824字精確；新美術與舊矩陣重播0，集中技術檢視1。

完整HD前綴由CPU獨立分叉，事件預覽成功才轉移HD附件。PGXP、NativePlane、正常GPU／GUI、IRQ／field跨越、其他圖層與正式入口待接。1219.15秒含645秒整包載入，不作速度聲明。完整包載入接近原容器3GiB上限，查Docker主機約31GiB後只將同一程序上限調為5GiB，未重啟／改輸入，最後退出0。一次文件定位讀取不存在的素材CONTEXT只屬唯讀工具失敗，改讀既有唯一§8，遊戲未重播。

GO-ORDERED-HD-20261026回填四個較早段落，同地址017原資料契約不受影響。保存來源、原圖、限定接納、Issue同步與本輪容器／UID清理由相鄰direct-go-v1/publication-ordered-HD-v1.json索引；原資料不入公開repo。


## 2026-10-26 Go精度標籤與雙平面同掛研究

025 §12.11–12.12追加DRAFT限定證據，正式Go及候選不改。一次原PGXP呼叫42321指令／59271模型週期、18251回呼；高階Go整理全部標籤，不重播回呼。六份完整保存精度相符，其中後五份連完整CPU／RAM／scratch相符。另一條正常真BIOS／同卡路線以PGXP／無美術HD2／CPU NativePlane同掛，Go接管一個呼叫，後續六場完整原生、精度、HD2與原生像素／快取相同；310圖塊、930個HD字免解析，拒絕0，83.43秒蒐證耗時不作速度聲明。正常兩機分叉前後的診斷累計範圍不同，不要求nativeSkipped總數相等。

七個新精度／雙平面守門案例通過，包含浮點NaN／負零與完整深度位元；另兩個穩定物件提交核心案例分版本接納。提交v1錯把SIO None空埠當nil，正例在守門停止，負例已過；保留首次來源，依實際SIO.New修正後只接未完成正例。初模型也曾因活動倒數的有效零上半字標籤被過嚴守門拒絕，重查Tracker.checked契約後首個全值比較通過。Go來源目錄及Python縮排兩次準備停止皆為遊戲0，原紀錄保留。沒有反覆重播完成矩陣或原函式。

新美術／畫面檢視及舊矩陣重播0。GO-PRECISION-PLANES-20261026回填六個舊段，接受與執行來源索引在相鄰direct-go-v1/precision-planes-DRAFT-acceptance-v1.json、precision-frozen-sources-v1.json及publication-precision-planes-v1.json。正式Step、就地提交的正常玩家路徑、017 BREAK協作、整包HD4正常同掛／GPU、IRQ／field及其他場景／人物／文字仍待接。原始資料及新無美術HD2圖只留本機，公開引擎無新圖片；本輪提交、Issue與容器／UID收尾見publication。


## 2026-10-26 Go繪圖接管速度與海報保持

依使用者要求提前量速度，025 §12.13追加DRAFT限定研究，正式程式與v.0.7.12不改。第一輪固定暖身1／配對7定位完整精度JSON成本；後續只做型別化精度子集／範圍更新與共用計畫的有界原型。六個既存完整保存態及三個新原子拒絕／範圍外保持案例通過，未重播舊原函式或美術矩陣。第二輪固定暖身1／平衡三路6組，全部原生、精度、HD2、相容像素／快取精確，兩輪後停止。

第二輪六組標準中位數：選定圖層呼叫原模擬1.368ms、原Go58.711ms、型別化Go9.572ms；呼叫至兩VBlank像素完成15.521／73.451／25.540ms。型別化Go相對原Go逐組倍率中位數5.81，仍慢於原模擬7倍。主要成本整機／精度複製與套用5.84ms、輸入1.70ms、時鐘與CPU JSON 1.62ms。原runner偶數摘要取上中位數保留，衍生摘要以全部六組算標準中位數。這不是整包HD4／1080p／GPU／海報穩態FPS，速度未通過。

小驗證器首次sha helper建置失敗、後續缺binary啟動127均案例0；保留來源後只改hash helper，首次六態比較通過。接受工具首次少engine唯讀掛載，補掛後只核對保存結果。原計畫guard_scope殘留no-PGXP文字，實際兩輪PGXP=true；規格與台帳勘誤，不修改原輸入。GO-DRAW-SPEED-20261026回填025三段早期蒐證耗時，來源凍結與原始日誌保留。

六張A／B／C海報及A／soft／zoom保存設定SHA保持；前端只在載入解碼及初始化建立圖像。未新增GUI或美術／畫面檢視；後續正常速度接受須保留海報與既定畫質。正式Step、017協作、移除每呼叫整機／Tracker複製、完整HD4／1080p遊玩速度及全HD仍待接。唯一現況表在素材docs/hd-assets-evaluation.md §8，工具與收據入口docs/assets-catalog.md#render-control。本輪commit／push、Issue #9／#17／素材#13與UID／容器收尾由publication-draw-speed-v1.json記錄，不建立tag／Release。


## 2026-10-26 Go就地提交及原版／HD分流

使用者再次強調速度、全Go繪圖與3D HD，並明確指定原版沿原繪圖、HD才由Go全部接手，以同存檔／輸入比較差異。025 §12.14先寫有限DRAFT契約再做裝置復原原型，§12.15及兩repo AGENTS保存模式分流目標，沒有把既有視野選項誤稱繪圖模式。正式入口、全部人物／文字／3D仍未接完。

原型不複製Machine／完整Tracker，僅保存裝置時間推進的可變值並暫時移除呈現附件；所有拒絕條件通過後提交精度子集及Go返回，原物件位址保持。新增六個成功／CPU／精度／晚到IRQ／跨場／觀測器案例首次通過。唯一完成的正常量測固定暖身1、三路6組，完整原生、PGXP、HD2、NativeSnapshot像素／快取全部相同；局部呼叫1.110／8.641／3.752ms，兩VBlank完成15.083／24.534／16.087ms，前版Go與就地Go配置38.70／8.73MB。速度未通過，完整1080p／GPU／海報穩態FPS未驗，海報與畫質保持。

準備曾猜錯frontend檔名及不存在的support檔，依實際索引修正；未執行v1計畫補重用helper的GPU依賴後凍結v2。首次正常量測誤選早期go-entry-support，暖身原路徑已執行、Go接管0、完整比較0；保留首次binary／日誌，改用原計畫已固定的precision-oracle-support後乾淨接續，算法不改、已過六個案例不重跑。兩次開機嘗試、完成正常路線一條，不把環境／觀測工具失敗寫成產品缺陷。

新美術／畫面檢視與舊矩陣重播0，不新增候選、tag或Release。GO-DRAW-COMMIT-20261026回填較早複製瓶頸，來源與結果沿direct-go-v1/draw-commit-summary-v1.json、draw-commit-frozen-sources-v1.json。正式Go源未改；下一步為型別化CPU／輸入、共用時鐘展開、正式Step／017及雙模式玩家入口，再接全部繪圖。原始資料只留本機，提交／push、Issue #9／#17／素材#13與UID／容器收尾見publication-draw-commit-v1.json。


## 2026-10-26 CPU型別傳遞與單次相容時鐘

025 §12.16先固定有限READY契約，再將通用CPU CallState按值快照／原子返回與ReadRAM區段複製實作到正式引擎。CPU／GTE全測與新增14個CPU／一個RAM環繞副本案例通過，此有限資料契約CONFORMED；原READY全文保存於transport-spec-READY-v1.md。通用API不改GTE資料／觀測器／匯流排引用，不是遊戲Step接管或任意savestate匯入。

研究圖層不再做CPU JSON，單次cache／時間足跡同時產生退休成本與load前綴。六份原保存態完整CPU／RAM／scratch／PGXP及每例42321個成本精確，原函式不重播；一條正常真BIOS／同卡路線固定暖身1＋六組三路，原生Machine、PGXP、無美術HD2與NativeSnapshot像素／快取全部相同。局部呼叫1.701／4.370／2.836ms、完成窗口19.369／22.477／22.211ms。局部改善、整體窗口改善有限，速度仍未過；共享主機負載不同不跨批挑數字。時鐘／CPU1.693降至0.656ms，窗口配置3217.5降至943次。165.69秒為開機／蒐證耗時，不是FPS。

準備讀取曾猜不存在的machine/ram.go與cpu_test.go，改讀已定位實際檔；沒有因此重跑遊戲。首次比較器建置重複宣告新舊researchEventScalars，案例／遊戲0；只區分新型別名，保留v1覆蓋／計畫，舊generator依原固定SHA精確還原。接續首次六保存態與首次正常比較通過，CPU／GTE／RAM已過案例不重跑，不加第二輪速度。研究機器覆蓋明確承接正式ReadRAM改善，原三路均用新引擎。

新美術／畫面檢視與舊矩陣重播0。A／B／C海報、soft／zoom、設定、素材與v.0.7.12候選manifest保持，未打新包、tag或Release。正式Step、原版／HD模式、017協作、人物／文字／效果／3D及全HD仍待接；完整HD4／1080p／硬體GPU／海報穩態速度未驗。GO-CPU-TRANSPORT-20261026回填§12.14，來源、原始結果與接受沿direct-go-v1/transport-summary-v1.json及transport-frozen-sources-v1.json；提交／push、Issue #9／#17／素材#13與UID／容器收尾見publication-transport-v1.json。


## 2026-10-26 正式Go普通NO3入口

025 §12.17先READY後移入正式apps/sotn與通用資料層。RunUntilField按目標PC使用Go註冊；原版未註冊保留原迴圈，公開Step／除錯仍逐指令。整個三字DMA封包先核對再送HD Sprite16，來源／環境不符原狀回退；Shadow消費端不符則重建同三字交原解碼器。遊戲位址與1280雙緩衝容量留apps，通用層保留原CPU／RAM／快取／週邊及呈現所有權。HD前端啟動明列部分接管，關閉解除註冊，原版不建立Go renderer；熱切換未加。

CPU／GTE／PGXP先通過，首次HD消費端誤用Shadow.GP0名稱讓GPU建置停止，新案例0；保留首次來源，只改Renderer.WriteGP0後接GPU全測與15個新案例，已過部分不重跑。初態觀測helper另缺舊名稱別名，遊戲0；保留v1來源／計畫後只補觀測別名，正式執行程式不改。cmd設定測試及實際程式建置完成。

唯一正常路線真BIOS／同卡／固定按鍵到8382，再自然到0x800ECC10，未注入PC／RAM／座標。正式Go連續60次、8383至8442六十場完整原生、PGXP、HD2與NativeSnapshot像素／CLUT／快取相同；18,323型別化精靈實送、54,969 HD字免解析，過期／環境拒絕0。觀測覆蓋僅初態複製／完整精度讀取，正式執行／提交／DMA／apps／HD替換0。另兩場寬視野原路徑控制相同，Go接受0／嘗試2，原Check／BREAK與鄰房保留，不稱Go寬視野接管已完成。

單場RunUntilField＋HD畫面／原生快照完成中位數原6.517ms／Go7.423ms，第一個從場中真入口開始，不能當穩態FPS；速度仍未過。83.87秒為蒐證耗時。§12.17只限普通NO3／CPU HD2同掛CONFORMED，GO-FORMAL-ENTRY-20261026回填八段正式入口缺口。新美術／畫面檢視與舊矩陣重跑0；原媒體、卡片、海報／設定及v.0.7.12保持，未打新包／tag／Release。完整素材HD4／GPU／GUI、新候選、原版／HD熱切換、寬視野Go協作及其餘場景／角色／文字／3D仍待接。來源／接納沿entry-frozen-sources-v1.json、entry-summary-v1.json，commit／push／Issue及UID／容器收尾見publication-formal-entry-v1.json。


## 2026-10-27 下一段Go物件動畫


#### 物件幾何集中驗證 GO-ACTOR-GEOMETRY-20261027

一條真BIOS／同卡／固定正常按鍵路線，未注入PC／RAM／程式，自然擷取8382至8393共12次原函式。Go直接讀入口動畫資料產生181個型別化四邊形與144個圖像上傳／清邊工作；12次全部封包bytes、完整512筆排序表與計數相同。角色語意沿原槽位列出，不把抽樣槽位稱為全部主角／怪物動畫。

原函式觀測有1008次CPU I/O讀、1625次CPU I/O寫及0次中斷入口。已保存每次完整前後RAM／scratch、CPU、位址存取與到訪足跡。Go幾何未執行原MIPS或提交GP0，但圖像工作尚未由Go執行，SDK裝置／非同步搬移、完整CPU／RAM／scratch／stack／clock／PGXP及正式註冊仍DRAFT。下一閘門是上傳／清邊事件及相依契約，不能直接把此原型接入正式遊戲。

首次觀測準備漏HD附件，EnablePGXP在CPU執行前停止，正常路線與幾何案例0；保留binary／來源／日誌，只補既有HD2 shadow。首次正常12次擷取完成，第一次Go比較11次通過，第12次因槽63的flags3縮放被守門拒絕。根據已查詢0x800EB544的固定8位小數公式補縮放，第二次只用保存12態離線比較，全數通過；沒有重開遊戲或重驗美術。第一版來源／拒絕／日誌保留，不稱首次全過。旋轉及bank15複製仍拒絕。無新畫面、完整HD4／1080p GUI／硬體GPU或速度聲明。先完成全部Go接管，效能後續；CPU／GPU及海報保持。限定來源／收據由私人direct-go-v1/actor-summary-v1.json與actor-frozen-sources-v1.json索引，本節維持DRAFT。

本repo僅更新規格／索引／決策與紀錄，正式程式不改；原型與原資料留相鄰私人素材專案。提交／push及Issue由該專案direct-go-v1/publication-actor-geometry-v1.json記錄。


## 2026-10-27 月下與Mednafen純繪圖速度比較

使用者要求先量測目前ps-golem與PS模擬器的render差距，本輪只量測，不調校或重啟遊戲。比較使用既有正常月下命令串流，兩段各120個連續TextureField場；兩側同一邏輯CPU、一次暖身及九組交錯樣本。Mednafen為Debian官方1.29.0+dfsg源碼，原gpu.cpp／polygon／line／sprite程式未改，C++ -O2；Go使用目前正式internal/gpu與render/hd來源，GOMAXPROCS=1，只加獨立初態複製／場奇偶與讀回觀測附件。

| 場景 | Mednafen原生 ms／場 | Go原生 ms／場 | Go／Mednafen耗時比 | Go獨立HD4 ms／場 | 終點顯存不同字 |
|---|---:|---:|---:|---:|---:|
| 序章樓梯與背景 | 0.7710 | 1.7630 | 2.29 | 23.2288 | 0 |
| 王座廳戰鬥 | 0.4478 | 1.3508 | 3.02 | 7.2645 | 0 |

硬體：Intel(R) Core(TM) Ultra 7 165U；固定邏輯CPU 0。倍率越大代表Go耗時越久。純繪圖計時涵蓋命令解析至同步顯存工作完成，排除前綴重播、初態複製、垃圾回收準備、檔案／PNG／雜湊、CPU／GTE／SPU／CD模擬及GUI／海報。CPU耗時、各組實際耗時範圍另存render-speed-summary-v1.json，不以共享主機載入總時長推算FPS。

HD4是獨立CPU四倍繪圖器、原版材質，沒有PGXP／透視／HD素材包、相容平面併行、硬體GPU或GUI。它只補充解析度成本，不與Mednafen一倍混稱相同畫質比較；完整HD／1080p遊玩速度未由本批驗收。終點比較各524288個RGB555／STP顯存字，沒有宣稱逐場像素或完整機器同態。

初次抽取誤用稀疏tag9截圖點，計時樣本0；保留原計畫後改用逐場tag6並驗證各120場。Go觀測main曾保留build-ignore，建置0案例；只在生成覆蓋移除該標記。首次GPU前綴遇到呈現串流沒有CPU讀回事件，Mednafen守門在暖身前拒絕，樣本0；兩側均以原讀取介面完成唯讀傳輸，沒有略過繪圖。其後只完成一次固定六暖身／54樣本批次，不反覆重測或挑選結果。來源與各次準備紀錄保持。

結果由render-speed-summary-v1.json、render-speed-execution-plan-v4.json與render-speed-samples-v2.jsonl索引，入口為素材docs/assets-catalog.md#render-control。既有Go角色／背景進度、美術、設定、海報與雙擊候選不變。本機前綴是量測初態，載入耗時不列入結果；原資料不入Git或公開Issue。commit／push、Issue #9與UID／容器收尾另存publication-render-speed-v1.json。

收尾核對六暖身／54樣本、計時窗口GPU讀回0及兩份完整顯存差異0。整理準備工具，使生成的main／唯讀完成附件與實際v4覆蓋逐字相同；實際執行的舊準備來源先保存，光柵化與樣本不變。刪除三份從未量測的錯誤衍生輸入，共623435784 bytes；正確串流、原壓縮檔、準備停止與雜湊均保留。沒有重跑樣本，完整GPU／1080p效能未驗。


## 2026-10-28 Rust原生繪圖與現成核心評估

使用者指定ps-golem改用Rust並量測，之後補充可沿用現成Rust PS1模擬器。已先記026 §2 READY，再建立rust/psgolem-core原生GPU命令renderer。Go可玩版與其原版／HD分流保持。Rust自行執行精靈／線／折線／平面及Gouraud三角與四邊形、texture window／CLUT／cache、混色／遮罩／抖色、VRAM傳輸與重疊複製；繪圖庫禁止unsafe，不呼叫Go或C++繪圖。只有Linux量測外殼使用libc clock記CPU時間，不把它當成renderer FFI。

沿用既有mobox-fw:rust，rustc1.99.1，Docker非root、離線、無額外crate相依。首次六項必要無素材案例全過，包含4096組混色、512組圖元自身TPage及傳輸／遮罩／重疊／CLUT／部分封包。未為補日誌重跑。原版月下串流沿同一兩段各120場，Rust／Go／Mednafen六個暖身端點的完整524288顯存字先確認差異0，才執行唯一固定54個計時樣本。未重啟遊戲或重驗美術。

| 場景 | Mednafen ms／場 | Go ms／場 | Rust ms／場 | Go／Rust耗時比 | Rust／Mednafen耗時比 |
|---|---:|---:|---:|---:|---:|
| 序章樓梯與背景 | 0.5787 | 1.2556 | 0.8558 | 1.47 | 1.48 |
| 王座廳戰鬥 | 0.3002 | 0.7724 | 0.4379 | 1.76 | 1.46 |

同一Intel(R) Core(TM) Ultra 7 165U邏輯CPU0，九組交錯中位數。Go這輪1.2556／0.7724ms，不與昨日不同主機負載的數字直接混算。Rust快約1.47至1.76倍，仍耗Mednafen約1.46至1.48倍時間。這是原生單CPU繪圖器，完整機器、HD素材、PGXP、實體GPU與GUI／遊玩FPS未量，不宣稱全Rust月下已能玩。HD型別命令、素材快取與CPU／GTE／週邊尚未全部遷移，現行雙擊候選未換。

按使用者新指示固定CoffeePSX／StarPSX／Trapezoid來源。CoffeePSX的官方HEAD在Codeberg，初次誤用GitHub格式的commit API返回404；改用Forgejo branches／git/commits介面後查證成功，沒有以歸檔鏡像代稱目前canonical。CoffeePSX優先列可撤回月下試跑候選，公開API需要wgpu device／queue，GPLv3版次及完整決定性待核，尚未搬程式；StarPSX上游月下卡Konami，Trapezoid為MIT但GPU依賴Vulkan，均不把README能力視為本片可玩／速度通過。完整表與來源在026 §4，現成核心整合仍DRAFT。

來源、第一次六案例、完整端點與54樣本由本機direct-go-v1/rust-render-first-build-tool-result-v1.json、rust-render-parity-v1.json、rust-render-execution-v1.json、rust-render-samples-v1.jsonl及rust-render-summary-v1.json索引；候選原始檔由rust-candidate-survey-v1.json與rust-coffee-canonical-v1.json索引。原資料不入Git，工具入口docs/assets-catalog.md#render-control。提交／push、遷移Issue、#9／#17及UID／Docker收尾另存publication-rust-render-v1.json，沒有新tag／Release或封包。


## 2026-10-28 CoffeePSX隔離試跑

依026 §4固定官方Codeberg ed7c266bf43cf9ffd4211be6d8cf15c8424033f2，來源hash與三份先前canonical證據相同；核心release建置完成。獨立公開API外殼不修改上游核心，原光碟／BIOS／卡片唯讀，正常固定按鍵，不注入PC／RAM。沿既有Rust與ps-golem Go容器，SIMD軟體繪圖、Mesa llvmpipe GLES呈現，不是實體GPU或HD試跑。

第一次外殼使用不存在的InstanceDescriptor Default而建置停止，遊戲0；保留來源／日誌，只改鎖版API的new_without_display_handle，離線建置通過。CUE兩次在emulator建構前停止：Big5非UTF-8，以及上游只認TRACK／INDEX固定兩／四格縮排。依001 §6生成一次性UTF-8與正確FILE引用，再依固定解析器規則正規化空白；兩軌模式、INDEX時間及光碟bytes保持。既有雙擊包CUE亦是Big5，未改包或原CUE。

唯一真正正常開機記錄完成500幀後，讀取Track1第16扇區因EDC停止，退出1，9000幀／月下遊玩未完成。獨立逐位元CRC以鎖版CRC_32_CD_ROM_EDC參數核對，原MODE2 Form1扇區所存3165503166、計算379105012，與候選錯誤一致；不關閉檢查、不修原光碟、不反覆重開遊戲。500幀的3.1275秒只作診斷，沒有同狀態Go比較，不宣稱完整遊戲速度。音訊資料含非零樣本，未人工聆聽。

上游memory.rs用未固定種子的rand::random填RAM／scratch，公開builder沒有種子選項。接入前需建立中文版光碟EDC相容契約、固定初始化、觀測API與授權版次證據；目前DRAFT，未採用候選為正式基底。第一批自有Rust原生繪圖與兩份完整端點／54速度樣本保持，不重跑。Rust遷移Issue #19保持OPEN，#17與#9承接；原版／HD、CPU／GPU、海報及現行Go雙擊候選保持。

本機來源、初次工具錯誤、各版CUE／計畫與唯一正常日誌保留在既有direct-go-v1/rust-coffee檔，結果由rust-coffee-trial-summary-v1.json及rust-coffee-disc-checksum-v1.json索引；工具入口為素材docs/assets-catalog.md#render-control。原資料與候選來源不入公開引擎。提交／push及容器收尾沿publication-rust-render-v1.json，不建立tag／Release。

收據工具的頁面更新曾因既有HTML沒有明示body標籤而停止；兩個定位嘗試均未改頁。原執行來源已保存，依實際style結束點只接續頁面插入與來源索引，沒有重跑EDC、遊戲或已驗案例。


## 2026-10-28 StarPSX參考與自有Rust CPU／GTE

使用者指定StarPSX參考，再明示以ps-golem突破並開發自己的可玩Rust版。固定官方main e8551eed，與前次調查同版，沒有把舊相容表直接當本機結論。core與公開API外殼首次release建置通過，未改上游核心；一次真BIOS／同卡／固定正常按鍵試跑，原光碟與卡片唯讀，沿已驗等價UTF-8 CUE，沒有PC／RAM注入。完成9000次run_frame，GPU計數8225；一次集中技術檢視顯示4500至9000的四張完整RGBA相同且停Konami。未到遊玩，33.816秒為診斷耗時，不與Go混算速度，不重跑／修補候選。

StarPSX可參考core／renderer分層、純CPU framebuffer介面、零RAM／scratch初態及const generic圖元模式；固定2 CPI與20指令批次不沿用為ps-golem時鐘，GPLv3來源未搬入正式引擎。現有Go可玩版與HD／CPU／GPU／海報保持，語言目標依使用者改Rust。

026 §5先READY，再將自有Go GTE移入rust/src/gte.rs；首次1150筆實機語料的73600個讀回完全相同，另兩項IR0／旗標及FIFO／延遲邊界通過，本節原生架構CONFORMED。原硬體語料SHA及003推論限制保持。026 §6先READY，再加入自有Rust CPU，保留MIPS I、load／branch delay、例外／COP0／COP2、乘除與GTE等待、快取／隔離及原008週期。沒有呼叫Go／C++或外部模擬器核心來執行Rust CPU。

新Go基準從固定0xc0ffee產生364組各16步，Rust首次5824個完整CPU／GTE讀回／快取／時間直接相同，RAM與完整來源寫值／實際讀取以FNV64摘要比較。這是Go移植差分，不是原版硬體CPU程式或月下可玩驗收。已過GPU及GTE案例不重跑，沒有新美術、GUI／HD／硬體GPU、速度或雙擊包。完整Machine／週邊、004公開CPU程式、正常月下、音訊／存檔／輸入與HD仍OPEN。

來源及結果由既有direct-go-v1/rust-cpu-gte-summary-v1.json、rust-gte-plan-v1.json／rust-gte-test-v1.log、rust-cpu-plan-v1.json／rust-cpu-oracle-v1.bin／rust-cpu-test-v1.log、rust-starpsx-source-v1.json／rust-starpsx-normal-v1.jsonl索引，工具入口素材docs/assets-catalog.md#render-control。兩份Rust契約與各首次結果分列，不把局部進度稱完整版。原資料與候選來源不入公開Git；commit／push、Issue #19／#17／素材#13與UID／Docker收尾沿publication-rust-cpu-gte-v1.json。沒有tag／Release。
