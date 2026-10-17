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
