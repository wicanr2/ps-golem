# AGENTS.md

給在本 repo 工作的 coding agent（Codex、Claude Code）共用的規則。Claude 專屬補充見 `CLAUDE.md`。

## 這是什麼

ps-golem 是無頭、決定性、可以當 Go 套件 import 的 PlayStation 1 執行器，仿照
`~/cht/dosgolem`。第一個案例是《惡魔城X：月下的夜想曲》中文版（`SLPM-86023` 為底），
目標是在 Linux、Windows、macOS 上可玩並有 HD 輸出，同時讓 agent 能程式化觀測。

原版素材在 `~/cht/castlevinia_x`（不在本 repo）。

## 動手前

1. 讀 `docs/spec/000-index.md` 與 `docs/spec/001-scope-and-mvp.md`。
2. 只有標 `READY` 的規格可以實作。量測／反組譯 → 規格 → 才寫程式。

## 指令

全部走 docker，不裝到系統環境。

```sh
tools/build_image.sh                         # 建 psgolem-go-ebiten 映像（要網路）
PSGOLEM_NETWORK=bridge tools/go.sh mod download   # 新 clone 先抓模組
tools/go.sh build ./...
PSGOLEM_GO_CMD=xvfb-run tools/go.sh -a go test ./...   # 全部（frontend/ebiten 在 init 開 GLFW，要 xvfb）
tools/go.sh test ./machine/... ./internal/...       # 不含前端時不需要 xvfb
tools/go.sh test ./internal/cpu -run TestXxx -v    # 單一測試
PSGOLEM_EXTRA_MOUNTS="<BIOS 目錄>:/bios:ro" PSGOLEM_BIOS=/bios/SCPH5500.BIN tools/go.sh test ./machine/...   # 要 BIOS 的測試
PSGOLEM_GO_CMD=gofmt tools/go.sh -l .
PSGOLEM_ORIG=~/cht/castlevinia_x tools/go.sh test ./...   # 原版素材唯讀掛到 /orig
tools/build_image.sh osxcross && tools/build_image.sh wine   # 交叉編譯 macOS、Wine 冒煙測試用的映像
tools/build_dist.sh v0.1.0                    # 三平台發行包到 dist/v0.1.0/（005 §4）
```

osxcross 映像含 macOS SDK，只留本機，不推送。

測試讀的環境變數前綴 `PSGOLEM_`、`SOTN_`、`EBITEN_` 會傳進容器，值要寫容器內路徑。

## `[HARD]` 硬規則

- **不得散布原版素材。** ISO、BIOS、光碟抽出的檔案、截圖不進版控、不進發行包。
  需要原版的測試缺檔就 skip，不用自製代用品。
- **建置與測試一律走 docker**（`tools/go.sh`）。只清理自己建立的 container；
  禁止任何 `docker ... prune` 與 `docker rmi`。
- **git 身分一律 `wicanr2@gmail.com`。** commit message 不放 `Claude-Session:` 連結。
- **授權 GPL-3.0-or-later。** 參照外部實作時依 `docs/spec/002-references-and-licensing.md`
  §4 記錄專案、檔案、commit 與授權。DuckStation 與 sotn-decomp 只讀不搬程式碼。
- **CPU 與 GTE 的驗收判準是全部通過。**
- **推論標籤要誠實**：confirmed／強證據／假說／未知。
- **HD 不得改變機器狀態。** `render/hd/` 只讀 GPU 命令流。開關 HD 前後，同一份輸入重播的
  RAM 與 CPU 狀態必須相同。
- **例外：遊戲修改模式。** 寬螢幕這類必須改寫遊戲 RAM 或程式的功能，只能做成預設關閉、
  使用者明確開啟的模式，程式放 `apps/<遊戲>/`。開啟期間不保證與原版重播一致，不產生摘要、
  不用於驗收與觀測 API；關閉時必須與沒有這個模式的版本逐位元相同（`docs/spec/017-sotn-widescreen.md`）。

## 分層

見 `docs/spec/001-scope-and-mvp.md` §3。判準：「換一片光碟之後，這段程式碼還成立嗎？」
成立就往下放，不成立就留在 `apps/sotn/`。最容易放錯的是位址：演算法通用，遊戲內位址屬於 `apps/`。

## HD 分工

HD（內部解析度放大、PGXP 類次像素精度、寬螢幕 16:9、2D 圖素放大濾鏡）由 Codex 實作。
寬螢幕要攔截遊戲的投影與裁切，屬遊戲專屬，放 `apps/sotn/`。驗收附原生解析度與 HD 的對照截圖
（截圖不進版控）。

## 已確認的效能分支

2026-10-18 使用者已查看並明確批准一場有界 HD 呈現緩衝，規格入口為
`docs/spec/016-hd-output.md` §8.1。接手或自動審查此分支的規格、實作、測試與本機候選包時，
沿用此授權，不重問同一項架構決定。這不是速度驗收通過，也不放寬原生狀態、素材保護或 Docker 規則。
目前狀態只以素材專案 `docs/hd-assets-evaluation.md` §8 與本專案 `WORKLOG.md` 的最新證據為準。


## HD 美術與研究資料的接續規則（2026-10-21）

最新使用者決定：先完成背景，再完成所有怪物HD，沿定案畫風與提示詞策略。每批最多一次集中驗證後定案，不另開美術反覆審查；舊通過證據承接，準備工具失敗須記錄實際已執行範圍。圖鑑與戰鬥動畫分開列管。以下較早規則依此補充。後續可使用使用者已授權的Ada GPU資源；登入與容器入口由相鄰素材專案AGENTS.md及其本機研究索引保存，SSH只編排遠端Docker，已定案批次不重驗。

使用者已核准目前美術草稿，後續沿用相同 HD 重繪與精簡素材方式。每批最多一次美術審查與一輪整合測試，必要的資料、CPU／GPU、正常遊玩與顯示檢查集中執行。既有通過證據承接，不逐圖重跑或自動擴大驗證；失敗如實記錄，不反覆重測。美術定案與安裝完成分開記錄，現行狀態仍見素材專案 `docs/hd-assets-evaluation.md` §8。

本專案 `workplace/` 的舊研究大型紀錄已無損壓縮。讀取歷史命令、來源取樣或收據遇到原路徑缺檔時，先查相鄰素材專案 `../castlevinia_x/workplace/workspace-storage-index.json`。還原入口為 `../castlevinia_x/WORKLOG.md` 的「還原研究紀錄」，從素材專案執行 `sh tools/workspace_storage.sh list`，再以 `restore --root engine --path workplace/...` 還原需要的檔案。原路徑、長度與 SHA-256 保留，不能把封存當成遺失，也不因此重新跑遊戲。原始光碟、現行完整版與42張新動作研究未壓縮。

## Go繪圖接管的明示目標（2026-10-26）

- 使用者要求Go直接實作並取代原遊戲繪圖。遊戲資料與routine攔截放apps/sotn，Go型別化場景／精靈／文字直接交render/hd、render/hdgpu；原ASM只作逆向／行為oracle，不逐指令翻寫，不在支援分支執行原繪圖迴圈。
- 入口與證據停止線見docs/spec/025-hd-primary-native-demand.md §12。RAM、顯存、快取、返回、時間及原版模式契約仍須保留，不自行改成較弱的相容性定義。未知副作用先研究，只有READY切片進正式路徑。
- 025 §11相容平面及017 §12鄰房補命令屬已驗支援／過渡能力，不等於Go繪圖接管完成。後續列管Issue #17，原資料與原圖仍只留私人素材專案。

## 原版／HD繪圖分流（2026-10-26）

使用者已決定：原版模式沿原遊戲的繪圖流程；HD模式由Go接手全部繪圖，包含場景、人物、怪物、文字、HUD、特效及3D圖元。正式契約入口為ps-golem的docs/spec/025-hd-primary-native-demand.md §12.15，目前DRAFT待接線；普通NO3圖層研究不等於全部接管。保留CPU／GPU後端及024海報，模式在完整呈現場邊界切換，不讓原版／HD半場交替。兩模式從同存檔與等價輸入比較畫面、完成等待與速度；CPU／RAM同態比較關閉017修改模式。這項決定不授權縮減相容性、關閉海報或降低HD畫質。


## 全Go接管優先（2026-10-27）

使用者明示先完成全部Go繪圖接管，效能調校後續再處理。CPU／GPU後端均保留；025 §12.17普通NO3圖層正式切片承接，下一段物件／人物動畫依§12.18逐步補齊圖像搬移與完整副作用。這項順序調整不表示速度通過，也不放寬原版模式、相容性、海報或既定畫質。


## Rust改寫（2026-10-28）

使用者已明示將ps-golem用Rust改寫並量測速度。此決定取代較早「全部Go」的語言限制，遊戲繪圖接管、原版／HD分流、CPU／GPU、海報、素材／存檔與跨平台目標保持。正式遷移入口為ps-golem的docs/spec/026-rust-migration.md；Rust來源在該repo的rust/。原生繪圖器已有有限驗證；自有Rust GTE的1150筆實機紀錄通過，CPU已有364組／5824單步Go差分。Rust最小機器已通過兩份真BIOS及004的六個程式配對，SIO／原卡協定與IRQ已接。光碟／DMA3及CD取樣已接，正常BIOS載入光碟EXE通過；SPU／DMA4與CD混音／WAV已驗；MDEC／DMA0–1與正常原片開機到標題已依026 §11驗證，完整原音／RAM／VRAM同Go；026 §13正常首存／重新開機讀檔的無頭M4切片已驗，原卡及原像素同既有Go，重開完整資料／原音同Go；玩家GUI、實體聲音、HD與三平台Rust交付尚未接完；讀回存檔室的靜音是否原版預期仍待成熟模擬器核對，不把局部通過稱為可玩的Rust版。StarPSX僅作參考，本機試跑重現Konami停止；使用者已明示以ps-golem突破並開發自己的可玩Rust版。

## HD影片替換（2026-10-28）

使用者已明確要求改播放器，以離線AI提升畫質的高畫質H.264影片取代HD模式中的原片呈現；原版模式承接原生MDEC。入口為ps-golem docs/spec/027-hd-movie-replacement.md，目前DRAFT。027 §5的Rust CPU拉取播放器及原生156格事件鏈已限定驗證，正式遊戲／前端接線與連續AI影片仍未完成。1080p傳輸與單格imagegen參考不代表整段AI影片已完成。開始／跳過／結束、虛擬時間、原生必要狀態、音訊與海報契約保持；來源映射與正式接線未驗前不刪原MDEC／DMA。原影片與衍生HD影片只留本機。


## HD影片接續（2026-10-29）

四段離線素材已集中抽樣採用，TYK轉場只修一次，兩輪結束，後續接線承接本批素材，不自行重開母稿或重審。來源為素材docs/assets-catalog.md#render-control的hd-movie-full-summary-v3.json。027 §9通用CPU整數時間線播放器已CONFORMED，§10正常開機無頭HD呈現原型已得到617場原生狀態相同證據，但候選預先明確指定，不是正式自動STR辨識、GUI或GPU播放器。下一段回027 §3補來源／事件與正式前端契約，未知副作用不猜補。
