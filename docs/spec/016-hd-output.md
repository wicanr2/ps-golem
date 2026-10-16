# 016 HD 輸出

狀態：`READY`（第 1、2 階段）；第 3、4 階段為 `DRAFT`

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

## 5. 第 3、4 階段（DRAFT，不得實作）

- 次像素精度（PGXP 類）：Beetle `mednafen/pgxp/`。PGXP 要在 CPU 的載入與儲存旁追蹤高精度值，
  會碰到 CPU 熱路徑；高精度值只能餵給影子 GPU。檔頭授權未核實（`002`）。
- 寬螢幕 16:9：Beetle 的寬螢幕做法改 GTE 投影結果，GTE 結果會寫回 RAM，違反 §1 的決定性要求。
  《月下》以 2D 為主，畫面外的物件遊戲本來就不畫。要做必須在 `apps/sotn` 找出不經 RAM 的攔截點，
  目前沒有證據。現階段 16:9 視窗只做左右補黑。

## 6. 驗收

| 項目 | 判準 |
|---|---|
| 轉送自我檢查 | `N = 1`：ps1-tests `gpu/*` 與《月下》開機到序章，每個 field 結束時影子 VRAM 與原生 VRAM 逐位元相同 |
| 決定性 | `TestM4FirstSave` 以 `-hd 2` 重播：摘要 `6c35c2701228722f`、記憶卡、原生畫面與不開 HD 相同 |
| 畫面 | 標題、序章王座廳、城之入口各一張原生與 `-hd 2`、`-hd 4` 對照截圖；以 `-hd 2` 縮回原生尺寸（2×2 取左上）後與原生比對，差異只在多邊形邊緣 |
| 濾鏡 | `psgolem -filter xbr` 在 xvfb 下截圖，與 `none` 對照 |
| 效能 | §3 效能列，附量測方式與主機負載 |
| 交叉編譯 | `tools/build_dist.sh` 三平台照常通過（Kage shader 不需 cgo） |
