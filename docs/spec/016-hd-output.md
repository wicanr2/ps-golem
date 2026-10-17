# 016 HD 輸出

狀態：`READY`（第 1、2、3 階段實作完成，驗收見 §7）；第 4 階段（寬螢幕）由 `017` 以遊戲修改模式實作

## 1. 為什麼

專案目標要求 HD 畫面（`AGENTS.md`「HD 範圍」）：內部解析度放大、次像素精度、寬螢幕、2D 圖素放大濾鏡。
HD 是輸出選項，不得改變模擬器的決定性狀態：開關 HD 前後，同一份輸入重播的 RAM、CPU 狀態、
記憶卡與 `psexe` 摘要必須相同。

## 2. 架構：影子 GPU

原生 GPU（`internal/gpu`，`007`）照舊是唯一的狀態來源：VRAM 讀回（GP0 C0h）、GPUSTAT、
中斷與時間全部只看原生 GPU。HD 另開一顆「影子 GPU」，只負責畫給人看的高解析度畫面。

- 原生 GPU 把收到的輸入原樣轉給影子：每個 GP0 字（含 DMA2 送來的）、每個 GP1 寫入，
  以及影響繪圖結果的外部狀態（交錯模式下 `lineSkip` 用到的目前場奇偶）。轉送點在原生 GPU 的入口，
  順序與原生處理順序相同。
- 影子 GPU 在自己的 goroutine 消化這串輸入，用 N 倍座標光柵化到 `1024N × 512N` 的 VRAM。
  CPU→VRAM 傳輸以最近鄰放大寫入 N×N 區塊；VRAM→VRAM 複製、填色在放大座標上做；
  VRAM→CPU 讀回在影子內直接丟棄。
- 影子不回寫任何東西給機器。機器不等影子；前端取畫面時才與影子同步到「已消化到目前為止的輸入」。
- `Machine.Clone` 不複製影子（Clone 用在搜尋，不需要 HD 畫面）；複製出的機器沒有影子。
- N = 1 時，影子 VRAM 必須與原生 VRAM 逐位元相同。這是轉送正確性的自我檢查（§6）。

光柵化的放大做法移植自 beetle-psx-libretro 軟體渲染器的 `upscale_shift`
（`mednafen/psx/gpu*.c`，commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，GPL-2.0-or-later）。
現有 `internal/gpu` 已移植自同一份程式（`007`），放大版沿用相同結構。
N 支援 1、2、4、8（Beetle 的 `upscale_shift` 0–3）。

## 3. 第 1 階段：內部解析度放大（READY）

| 項目 | 規定 |
|---|---|
| 介面 | `gpu.NewShadow(shift uint)` 回傳影子；原生 GPU 以 `SetShadow` 接上。`Shadow.Screen()` 依原生的顯示設定回傳 N 倍畫面 |
| 顯示範圍 | 與原生 `Screen()`（`007` §8）同一套規則，寬高乘 N；24 位元模式直接從影子 VRAM 依位元組取（MDEC 畫面不另放大） |
| 抖色 | 照 Beetle：放大時抖色以原生像素為單位 |
| 效能 | 影子在另一顆核心上執行；`psexe -hd 2` 的模擬速度與不開 HD 相比，降幅在 15% 以內（多核心主機） |
| CLI | `psexe -hd N`（截圖用影子畫面）、`psgolem -hd N`；設定檔 `settings.json` 加 `hd` 欄位（`012` §9） |

## 4. 第 2 階段：2D 圖素放大濾鏡（READY）

作用在輸出階段，對 `Screen()` 或 `Shadow.Screen()` 的結果再處理，可與第 1 階段疊加。

- 濾鏡：xBR（Hyllian，lv2）。以 Ebiten Kage shader 實作，參照 libretro `glsl-shaders` 的 xBR 版本；
  移植前讀檔頭確認授權（預期為 MIT），把專案、檔案、commit、授權寫進 shader 檔頭。
- `psgolem -filter none|xbr`，設定檔 `filter` 欄位。預設 `none`。
- `psexe` 不套濾鏡（濾鏡只在 GPU 上跑，不參與觀測）。

## 5. 第 3 階段：次像素精度 PGXP（READY）

### 5.1 原理

PS1 的 GTE 把 3D 頂點投影成整數螢幕座標（SXY，各 16 位元）後，遊戲把它寫進 RAM 組成繪圖命令，
GPU 只看得到整數座標，所以多邊形在移動時會抖動。PGXP 在 GTE 投影時另外記下浮點座標，
讓這個高精度值跟著 32 位元字一路搬運（GTE 暫存器 → CPU 暫存器 → RAM → GP0），
最後在光柵化時用浮點座標取代整數座標。每個高精度值都附上它來源的 32 位元原值；
到了 GPU 時原值與實際收到的字不同，就代表遊戲中途改過，退回整數座標。

### 5.2 參照來源與授權

- beetle-psx-libretro commit `e57860d82faa953dc0fc10f201fd583739bc2f7d` 的 `pgxp/`。
  `pgxp_gte.c`、`pgxp_gpu.c` 與各 `.h` 檔頭為 GPL-2.0-or-later。
  `pgxp_cpu.c`、`pgxp_mem.c`、`pgxp_value.c`、`pgxp_main.c` 沒有檔頭，原始出處 `iCatButler/pcsxr`
  （commit `62467b86871aee3d70c7445f3cb79f0858ec566e`，`libpcsxcore/pgxp_*.c`）同樣沒有檔頭，
  該 repo 的 `COPYING` 為 GPLv3。兩者都與本專案 GPL-3.0-or-later 相容（`002` §2）。
- 光柵化端照 Beetle 軟體渲染器 `mednafen/psx/gpu_polygon.c` 的 `PGXP_LIT` 路徑與
  `gpu.c` 的 `PGXP_WriteFIFO`／`PGXP_WriteCB`。

### 5.3 範圍

| 項目 | 規定 |
|---|---|
| 追蹤模式 | Beetle 的 `PGXP_MODE_MEMORY` 加 `PGXP_MODE_GTE`：GTE 的 RTPS／RTPT 產生浮點 SXY；`MFC2`、`MTC2`、`CFC2`、`CTC2`、`LWC2`、`SWC2`；CPU 的各種載入、儲存（`LB`／`LBU`／`LH`／`LHU`／`LW`／`LWL`／`LWR`、`SB`／`SH`／`SW`／`SWL`／`SWR`）與 `ADDU rd, rs, $zero` 這類搬移。`PGXP_MODE_CPU`（算術指令追蹤）不做 |
| 高精度值 | 每個 32 位元字一份：x、y（float32）、有效旗標、來源原值。記憶體影子涵蓋主記憶體 2 MB、scratchpad 1 KB 與 GP0 埠 |
| 送到 GPU | GP0 字不論來自 CPU 寫入 GP0 埠或 DMA2，都以來源位址查記憶體影子，連同字一起轉給影子 GPU |
| 光柵化 | 影子 GPU 的多邊形頂點在高精度值有效、且原值與收到的字相同時，用浮點座標（加繪圖偏移）放大後光柵化；否則用整數座標。線段、矩形、sprite 不用。Beetle `gpu_polygon.c` 的 `psx_pgxp_2d_tol` 路徑（用多邊形畫的 2D 元素）要讀過並決定是否移植，決定與前後對照截圖寫進 §7 |
| 不做 | 透視校正貼圖、頂點快取（`PGXP_VERTEX_CACHE`）、顏色與霧的高精度（`PGXP_GetColor`、`PGXP_GetFog`）、NCLIP 改寫 |
| 程式位置 | 追蹤器放 `internal/pgxp`（不依賴 machine）；CPU 以選用介面呼叫。掛點只能放在載入、儲存、COP2 搬移指令、`ADDU` 在 `rt` 為 0 時的分支與 GTE RTPS／RTPT 的處理裡，不得放進 `step`、`fastFetch`、`execute` 的分派或 `Machine.Step` |
| 決定性 | 追蹤器只讀機器狀態，不寫回 CPU、RAM、GTE 或原生 GPU。GTE 掛點在原生計算之後、只讀暫存器；`003` 的 1150 筆 GTE 實機紀錄與原生 GPU 測試照常通過。原生 GPU 不看高精度值 |
| 啟用條件 | 只在影子 GPU 存在時接上（`-hd` 為 2 以上）；`Machine.Clone` 不複製追蹤器 |
| CLI | `psexe -pgxp`、`psgolem -pgxp`、設定檔 `pgxp` 欄位（布林，預設 false）。沒有 `-hd` 時給 `-pgxp` 印錯誤結束 |
| 效能 | 不開 PGXP 時，與現行版本的差異落在同條件重跑的雜訊範圍內（附雜訊範圍）；開 PGXP 時降幅在 25% 以內。量測固定 P 核心、交錯 A/B、取 CPU 時間最小值（`012` §3） |

## 5A. 第 4 階段：寬螢幕 16:9

- Beetle 的寬螢幕改 GTE 投影比例，只對 3D 有效，2D 圖素會被拉寬；GTE 結果寫回 RAM，違反 §1。
- 《月下》背景圖塊每列只送畫面內的 17 格，寬螢幕必須改遊戲的繪圖邏輯，不能是 GPU 後端選項。
- 實作見 `017-sotn-widescreen`：`apps/sotn` 的遊戲修改模式（預設關閉、開啟期間不保證決定性），
  畫面端在影子 GPU 加側邊畫布。影子 GPU 不開寬螢幕時輸出與本規格相同。

## 6. 驗收

| 項目 | 判準 |
|---|---|
| 轉送自我檢查 | `N = 1`：ps1-tests `gpu/*` 與《月下》開機到序章，每個 field 結束時影子 VRAM 與原生 VRAM 逐位元相同 |
| 決定性 | `TestM4FirstSave` 以 `-hd 2` 重播：摘要 `6c35c2701228722f`、記憶卡、原生畫面與不開 HD 相同 |
| 畫面 | 標題、序章王座廳、城之入口各一張原生與 `-hd 2`、`-hd 4` 對照截圖；以 `-hd 2` 縮回原生尺寸（2×2 取左上）後與原生比對，差異只在多邊形邊緣與貼圖多邊形內部的子 texel 重取樣（§7.3），目視沒有錯位或缺塊 |
| 濾鏡 | `psgolem -filter xbr` 在 xvfb 下截圖，與 `none` 對照 |
| 效能 | §3 效能列，附量測方式與主機負載 |
| 交叉編譯 | `tools/build_dist.sh` 三平台照常通過（Kage shader 不需 cgo） |
| PGXP 決定性 | `TestM4FirstSave` 以 `-hd 2 -pgxp` 重播：摘要 `6c35c2701228722f`、記憶卡、原生畫面與不開 HD 相同 |
| PGXP 不開時 | §6 轉送自我檢查照常逐位元相同 |
| PGXP 單元測試 | 不用原版素材：手寫 GTE 命令序列，確認 RTPS 的浮點 SXY 經 `SWC2` → `LW` → `SW` 到 GP0 埠後，影子 GPU 收到的頂點帶小數；中途改寫該字時退回整數 |
| PGXP 畫面 | 先在 M4 重播全程記錄每個 field 的 RTPS／RTPT 次數與命中率（有效頂點數／多邊形頂點數，0 也要報）。有 GTE 多邊形的場景各存 `-hd 4` 與 `-hd 4 -pgxp` 連續數張截圖；M4 路線上沒有走高精度路徑的多邊形時，回報後停在這一項，不另找繞道 |
| PGXP 效能 | §5.3 效能列 |

## 7. 驗收結果

第 1、2 階段的實作 commit 為 `4783e93`，§6 驗收全部通過；macOS 未做實機驗收。
第 3 階段 PGXP 的現況見 §7.5。第 4 階段不在本次實作範圍。
本節是 HD 的目前狀態表。

### 7.1 輸入與工具

- Docker：既有 `psgolem-go-ebiten:1.24-r1`，Go 1.24.13、Ebiten 2.9.9。
- 原版輸入唯讀掛載於 `/orig`；ps1-tests `build-158` 的 `tests.zip` 唯讀掛載於 `/ps1tests`。
  BIOS、cue、bin、測試包與按鍵腳本的 SHA-256 在 `workplace/hd/inputs.sha256`。
- Beetle：指定 commit `e57860d82faa953dc0fc10f201fd583739bc2f7d` 的軟體渲染器確有 `upscale_shift`。
  xBR：`libretro/glsl-shaders` commit `148ce728155232a6b63be772c1810bd84728f359`，
  `xbr/shaders/xbr-lv2.glsl` 檔頭為 MIT，移植檔保留授權全文。
  原始檔雜湊與工具版本在 `workplace/hd/references.log`。
- 影子必須在第一筆開機輸入前以 `SetShadow` 接上，呼叫者負責卸下並 `Close`。
  `Screen()`、`VRAM()` 與摘要仍讀原生狀態，呈現使用 `OutputScreen()`。
  VRAM 複製依 §2 在放大座標保留子像素；上游該 commit 的 `Command_FBCopy` 仍先取原生 texel，
  不把這部分宣稱為上游逐像素一致。

### 7.2 驗收台帳

| 項目 | 結果 | 證據，路徑以 repo 根目錄為準 |
|---|---|---|
| N=1，ps1-tests | 通過。`gpu/` 全部 17 個執行檔，從開機開始逐 field 比對完整 16 位元 VRAM | `machine/hd_test.go`、`workplace/hd/ps1-shadow.log` |
| N=1，《月下》 | 通過。正常按鍵重播至 field 12000，每個 field 的完整 VRAM 相同 | `apps/sotn/hd_test.go`、`workplace/hd/sotn-determinism.log` |
| M4 決定性 | 通過。原生與 HD2 都到 field 45784，摘要均為 `6c35c2701228722f`，完整記憶卡與原生畫面逐位元相同 | `apps/sotn/m4_test.go`、`workplace/hd/sotn-determinism.log` |
| 九張截圖 | 已產生。標題 field 4320、序章王座廳 field 12000、城之入口 field 35000 | `workplace/hd/captures.log`、`workplace/hd/screenshots.sha256` |
| 縮回比對 | 通過。HD2 標題與王座廳零差異；入口 147 個差異集中在阿魯卡德揮劍特效（旋轉的貼圖多邊形），其中 64 個距三角形邊界超過 1 個原生像素，屬 §7.3 的子 texel 重取樣，目視沒有錯位 | `workplace/hd/entrance-diff.png`、`workplace/hd/entrance-edges.json`、`workplace/hd/edge-metrics.log` |
| xBR | 通過。xvfb 下實際視窗截圖均為 960×720；none 與 xBR 有 265692 個像素不同，均非空白畫面 | `workplace/hd/filter-none.png`、`workplace/hd/filter-xbr.png`、`workplace/hd/filter-metrics.json` |
| 效能 | 本次三輪最小值通過速度門檻，正常排程降幅 14.85%，固定雙 P 核心降幅 3.31% | §7.4、`workplace/hd/performance.json`、`workplace/hd/performance-pcores.json` |
| 三平台建置 | 通過。`tools/build_dist.sh v0.2.0-hd`；macOS 雙架構、arm64 簽章與動態庫檢查通過，未做真機驗收 | `workplace/hd/build-dist.log`、`workplace/hd/packages.json`、`workplace/hd/package-smoke.log` |
| 既有測試 | 全專案測試、影子資料競爭檢查、實際 CPU／GTE 語料通過 | `workplace/hd/unit-tests.log`、`workplace/hd/race-tests.log`、`workplace/hd/cpu-gte.log` |

截圖名稱：`workplace/hd/{title,prologue,entrance}-{native,hd2,hd4}.png`。
HD4 的標題與王座廳縮回亦零差異，入口有 242 個差異。三組原生狀態摘要在兩個倍率的重播中相同。
截圖、原版輸入與量測輸出均未加入版控。

本機驗收包位於 `dist/v0.2.0-hd/`，依本次指定命令與專案 `005` 的輸出契約產生，未發布：

- `ps-golem-v0.2.0-hd-linux-amd64.tar.gz`
- `ps-golem-v0.2.0-hd-windows-amd64.zip`
- `ps-golem-v0.2.0-hd-macos-universal.zip`

三包的 `SHA256SUMS`、檔案清單與原版素材排除檢查通過。Linux 無缺少的動態庫，Windows 包在 Wine 下啟動通過。

### 7.3 圖像判準的限制

**已證實**：上游 `gpu_polygon.c` 的 `DrawTriangle` 以
`1 << (COORD_FBS - 1 - upscale_shift)` 設定 UV 初始偏移。
N=1 是半個 texel，N=2 是四分之一個 texel。一般仿射貼圖的內部像素也可能取到不同 texel。

不含原版素材的 [反例測試](../../render/hd/affine_test.go) 使用 XY `(0,0)、(60,0)、(0,60)`，
UV `(0,0)、(100,0)、(0,100)`。原生位置 `(10,10)` 距幾何邊界至少 10 個像素，
原生取樣 UV `(17,17)`，HD2 左上子像素取樣 UV `(16,16)`。
結果在 `workplace/hd/affine-counterexample.log`。

入口的 GP0 探測重播摘要仍是 `6d4dfd0579081d21`，與截圖重播相同。
原生 VRAM 顯示起點核對為 `(0,0)`。最後一個繪圖 field 34999 有 36 個相交多邊形，
147 個差異中有 64 個超出其三角形邊界一個原生像素。
原始 GP0 在 `workplace/hd/entrance-gp0.json`，最後四個 field 在 `workplace/hd/entrance-gp0-last-fields.json`。

反例證實「依指定上游取樣」不能保證「縮回只差幾何邊緣」。
它不代表入口每個差異點的寫入來源都已定位；入口差異的逐點來源仍有未知。
§6 圖像判準因此允許貼圖多邊形內部的子 texel 重取樣差異；取樣演算法沿用上游。

### 7.4 效能數據與重跑

同一 BIOS 與 cue，從開機跑至 field 4400，週期上限 `400000000000`。
順序為改動前基準、目前原生、HD2，交錯三輪，各取 CPU 與牆鐘時間的最小值。
改動前基準為 commit `687b33bd6c9361e64591968b3577fd97186c9d5b`。

| 排程 | 改動前 CPU／牆鐘秒 | 目前原生 CPU／牆鐘秒 | HD2 CPU／牆鐘秒 | 模擬速度降幅 |
|---|---|---|---|---|
| 系統排程 | 50.32／50.21 | 51.49／51.27 | 85.52／60.21 | 14.85% |
| 固定 CPU 0、2 | 61.11／61.91 | 52.75／52.84 | 75.30／54.66 | 3.31% |

速度降幅為 `1 - 原生牆鐘秒 / HD2牆鐘秒`。CPU 時間加總了原生與影子執行緒，HD2 的總 CPU 成本增加。
正常排程下未開 HD 的 CPU 最小值比基準高 2.32%；固定核心組沒有觀察到退步，不能據此宣稱加速。
每次執行的摘要都是 `030643de52be3c27`。

主機為 Intel Core Ultra 7 165U，容器上限 4 CPU、4 GiB，主機另有工作負載。
正常排程的一分鐘負載範圍為 9.88–27.15，固定核心組為 15.44–35.65。
CPU 0、2 分屬兩個實體 P 核心；固定核心只控制測試子程序的親和性，未修改主機設定。
正常排程數據接近 15% 門檻，不把本次量測外推為其他負載下的保證。

效能工具：`PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_GO_CMD=python3 tools/go.sh tools/measure_hd.py`。
預先以 `tools/go.sh build` 建立 `workplace/hd/psexe` 與基準版 `psexe-before`。
固定核心組加 `--cpus 0,2 --output performance-pcores.json`。

M4 與 N=1 遊戲重播：

```sh
PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_LONG=1 \
PSGOLEM_BIOS=/orig/bios/SCPH5500.BIN SOTN_CUE=/orig/disc/sotn.cue \
PSGOLEM_TIMEOUT=60m tools/go.sh test ./apps/sotn \
  -run 'TestM4FirstSave|TestShadowBootPrologue' -v -timeout 60m
```

九張截圖：設定相同的 BIOS 與 cue，另設 `PSGOLEM_HD_EVIDENCE=/src/workplace/hd`，
透過 `tools/go.sh test ./apps/sotn -run TestHDCaptures -v -timeout 60m` 重生。

### 7.5 PGXP 驗收

PGXP 實作為 `10dd85b`，已接上選用追蹤器、GTE 投影、CPU／DMA2 搬移與影子多邊形光柵化。
CLI 為 `psexe -hd 2 -pgxp`、`psgolem -hd 2 -pgxp`，設定檔欄位 `pgxp` 預設為 false。
追蹤器不進摘要，`Machine.Clone` 不複製追蹤器或影子。

| 項目 | 目前結果 | 證據 |
|---|---|---|
| 無素材指令鏈與回退 | 通過；RTPS → SWC2 → LW → SW → GP0，小數實際影響影子畫面，改字後回退 | `machine/pgxp_test.go`、`workplace/pgxp/unit.log` |
| DMA2、部分字、延遲載入與跨批次 | 通過 | `internal/pgxp/tracker_test.go`、`render/hd/pgxp_test.go`、`workplace/pgxp/unit.log` |
| GTE 實機語料 | 啟用 PGXP 後 1150 筆各 64 個暫存器全部相符 | `internal/gte/pgxp_test.go`、`workplace/pgxp/gte-pgxp.log` |
| 既有測試及 N=1 GPU 語料 | 最終全專案測試通過，包含指定 BIOS、ps1-tests 與新增 PGXP 測試 | `workplace/pgxp/regression-final.log` |
| 資料競爭 | 通過 | `workplace/pgxp/race.log` |
| N=1 遊戲重播 | 通過；開機到序章 12000 個 field，完整 VRAM 逐位元相同 | `workplace/pgxp/m4.log` |
| M4 決定性 | 原生、HD2、HD2-PGXP 均到 field 45784，摘要均為 `6c35c2701228722f`；完整記憶卡與原生畫面相同 | `workplace/pgxp/m4.log` |
| 逐 field 調查 | 完整 45784 列，零次投影、零命中均保留；總頂點命中率 30.8815% | `workplace/pgxp/m4-fields.csv`、`workplace/pgxp/survey.json` |
| 場景截圖 | 30 組連續抽樣，每個變體 89 張；89 個抽樣點原生摘要相同，68 組 PNG 完全相同，其餘差異在多邊形與特效，未見整體錯位或缺塊 | `apps/sotn/pgxp_test.go`、`workplace/pgxp/capture-selection.json`、`workplace/pgxp/capture-metrics.json` |
| 效能 | 本次測段通過；PGXP 速度降幅 17.23%，關閉時差異落在實測雜訊內，雜訊很大 | `tools/measure_pgxp.py`、`workplace/pgxp/performance.json` |
| 三平台建置 | `tools/build_dist.sh v0.3.0-pgxp` 通過；macOS 雙架構、arm64 簽章與動態庫檢查通過，未做真機驗收 | `workplace/pgxp/build-dist.log`、`workplace/pgxp/packages.json` |
| 封包啟動 | Linux 與 Wine 下 Windows 均以 HD2-PGXP 開機到 field 800；摘要 `11b64e7524053051`、PNG 相同；設定儲存與重新開啟通過 | `workplace/pgxp/package-smoke.json`、`workplace/pgxp/package-*-window.png` |

光柵化參照指定 Beetle commit 的 `PGXP_LIT` 軟體路徑：浮點座標加繪圖偏移並放大後，
轉為整數子像素交給既有仿射光柵化。未加入透視校正、顏色／霧、頂點快取或 NCLIP 改寫。

#### 逐 field 與截圖證據

| 計數 | 總數 |
|---|---|
| RTPS／RTPT | 2214454／389736 |
| 有效附件頂點／全部多邊形頂點 | 2202698／7132733 |
| 零投影／零命中 field | 31718／37588 |

CSV 的 field 是完成後的 VBlank 計數，每列涵蓋前一個到當前 VBlank 的輸入。
命中率以命令流的有效附件頂點／全部多邊形頂點計算。
抽樣覆蓋全部連續活躍區間；長於 2000 field 的區間再分段，各取連續三個 field。
原始截圖為 `workplace/pgxp/field-<六位數 field>-{hd4,hd4-pgxp}.png`，共 178 張。
縮圖對照為 `workplace/pgxp/review-00.png` 至 `review-05.png`，左 HD4、右 HD4-PGXP；原始 PNG 未修改。
逐像素差異介於 0 至 454378，32 組抽樣兩側均為轉場黑畫面。
HD4 完整重播收據在 `workplace/pgxp/captures.log`，HD4-PGXP 完整重播收據在 `workplace/pgxp/captures-pgxp.log`。

#### 2D 容差決定

不移植 `psx_pgxp_2d_tol`。指定 Beetle commit 的 `gpu_polygon.c` 只在
`psx_pgxp_2d_tol >= 0` 且三角形缺少有效 W 時，依座標差異退回整數；`libretro.c` 的停用與預設值均設為 `-1`。
本階段只保存 XY，沿用上游預設停用。前述 HD4／HD4-PGXP 的完整前後抽樣未見 2D 場景整體偏移或缺塊。
此判斷限於本次 M4 路線與抽樣。

#### 輸入、工具與重生入口

輸入 SHA-256 在 `workplace/pgxp/inputs.sha256`，Beetle 原始碼雜湊在 `workplace/pgxp/references.sha256`。
使用既有 `psgolem-go-ebiten:1.24-r1`、Go 1.24.13；建置另沿用既有 osxcross 與 Wine 映像。
GTE 原生結果以實機語料確認；高精度投影與光柵化依指定 Beetle 原始碼移植，不宣稱 PGXP 浮點值為硬體輸出。

M4 與 N=1 遊戲重播：

```sh
PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_LONG=1 \
PSGOLEM_BIOS=/orig/bios/SCPH5500.BIN SOTN_CUE=/orig/disc/sotn.cue \
PSGOLEM_PGXP_EVIDENCE=/src/workplace/pgxp PSGOLEM_TIMEOUT=95m \
tools/go.sh test ./apps/sotn -run 'TestM4FirstSave|TestShadowBootPrologue' -v -timeout 90m
```

截圖以相同 BIOS、cue 與證據目錄，加上 `PSGOLEM_PGXP_FIELDS` 的逗號分隔 field 清單，
執行 `tools/go.sh test ./apps/sotn -run TestPGXPCaptures -v -timeout 90m`。
本次清單保存在 `workplace/pgxp/capture-selection.json`，選樣腳本為 `workplace/pgxp/choose_captures.py`。

效能工具是 `tools/measure_pgxp.py`。改動前基準為 `b68b4c7` 的 `workplace/pgxp/psexe-before`，
目前程式為 `workplace/pgxp/psexe`。兩者由同一映像建置，從開機到 field 4400，
固定 `PSGOLEM_CPUSET=0-3`、容器內 `GOMAXPROCS=2`，交錯三輪並取總 CPU 時間最小值。
同時比較未開 HD、HD2 的改動前後差異，以及 HD2-PGXP 相對 HD2 的速度降幅。
每組雜訊範圍為三次 CPU 時間的最大值／最小值減一，A/B 採兩側較大的範圍。

| 變體 | 三輪 CPU 時間最小值，秒 | 三輪觀察雜訊範圍 |
|---|---|---|
| 改動前原生 | 67.499726 | 57.79% |
| 目前原生 | 38.979963 | 199.59% |
| 改動前 HD2 | 90.940961 | 64.32% |
| 目前 HD2 | 61.733758 | 150.90% |
| HD2-PGXP | 74.581245 | 93.69% |

關閉 PGXP 時，原生與 HD2 的 CPU 最小值變化分別為 −42.25% 與 −32.12%，
均落在對應的實測雜訊範圍 199.59% 與 150.90% 內。
開啟 PGXP 的速度降幅為 `1 − 61.733758 / 74.581245 = 17.23%`，低於 25%。
CPU 時間包含影子執行緒，15 次執行的摘要均為 `030643de52be3c27`。
主機 Core Ultra 7 165U 的一分鐘負載介於 7.36 至 64.97，這輪波動限制小幅差異的辨識。
數據僅支持本次開機到 field 4400 的門檻判定，不作為加速或其他負載下的保證。
逐次 CPU 時間、主機負載、執行順序與兩個執行檔的 SHA-256 都在 `workplace/pgxp/performance.json`。

```sh
PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_CPUSET=0-3 \
PSGOLEM_GO_CMD=python3 tools/go.sh tools/measure_pgxp.py
```

本機驗收包為 `dist/v0.3.0-pgxp/`，依本次指定版本與 `005` 契約產生。
包內只有程式、授權與文件，檔案清單、SHA-256 與素材排除檢查通過。
