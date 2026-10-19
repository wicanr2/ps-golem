# 022 HD 3D 貼圖透視校正

狀態：`CONFORMED`，限已測Linux功能、CPU／GPU命令與正常序章抽樣。使用者已確認 HD 採透視校正，保留原版模式；可靠深度不足時回退。

## 範圍與依據

只改影子 GPU 的貼圖取樣。原生 CPU、GTE 暫存器、RAM、GPU、計時與記憶卡均不改。
未開啟時沿用既有仿射畫面；矩形、精靈、無貼圖與等深度三角形也維持原取樣。
本項是選用的畫面增強，不宣稱符合 PS1 原始貼圖畫面。

本規格補充 `016-hd-output.md` §5 原先不含透視貼圖的範圍，沿用 `018`／`019` 素材格式
與 `021` CPU／GPU 後端，不新增美術檔案。

原始依據為目前 GTE／PGXP 搬運程式及可重播實驗。GTE 原生契約見
[psx-spx GTE](https://psx-spx.consoledev.net/geometrytransformationenginegte/)。
既有 PGXP 投影參照 Beetle commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`
的 `mednafen/psx/gte.c`，GPL-2.0-or-later；本次有理插值按下列公式獨立實作。
沒有移植 DuckStation 或遊戲反編譯程式碼。

| 結論 | 等級與證據 |
|---|---|
| PS1 GPU 命令不帶 Z，需呈現用平行資料 | 已證實，既有 `internal/gte/pgxp.go`、`internal/pgxp/tracker.go` 與 GPU 命令格式 |
| 只保存 XY 或清掉半字重組深度不足以套用此功能 | 已證實，第一版正常開機研究無有效深度；不能當遊戲內修正完成 |
| 同一投影來源的兩半可重組深度 | 已證實，自製搬運測試及正常序章的第二版研究；混合來源必須回退 |
| 有理插值可消除自製斜面的仿射變形 | 已證實，獨立反投影公式比對；不是原版像素 oracle |
| 遊戲全部 3D 圖元均可追蹤 | 未知，不作完成條件；未追到即回退 |

工具為 Go 1.24.13、既有 Docker Go／Intel GPU 驗證映像；基準程式為 `6271e76` 加當前工作樹。
研究入口為 `workplace/hdtex/perf/perspective-depthv2-overlay.json`，正常路徑為
`perspective-depthv2-prologue/verification.json`，對照基準為 `perspective-prologue-pair/verification.json`。
本機 BIOS、cue、按鍵與資料雜湊由這些收據及既有素材驗證保存，不加入公開素材。
位址空間是 GTE 輸入、呈現用 RAM 影子及放大後畫布座標；沒有修改遊戲位址。

## 深度與搬運契約

- GTE RTPS／RTPT 在原生計算完成後額外送出未量化投影深度 `Z = signed44(TRZ*4096 + R3·V)/4096`。
  與既有 PGXP XY 使用相同投影模型。`Z <= H/2`、非正、非有限值及 XY 發生裁切時不提供深度。
  不用受限的原生 SZ3 整數代替 PGXP 的平行浮點深度，也不回寫任何 GTE 值。
- 每個投影頂點取得不同的 64 位元來源編號。完整字搬運保留來源；低、高半字分別保存來源與深度。
  LH／LHU 只把讀到的半字來源放到低半，符號延伸不產生另一個座標來源。
- SH 只更新目的半字。兩半的實際原值、XY 旗標、非零來源編號及正深度都相符，才重建有效 Z。
  不同頂點即使整數座標與深度相同也不能混合。
- 原值不符、位元組覆寫或既有精度失效時，深度同時失效。延遲載入與 COP2 FIFO 的既有順序保持。
  不新增猜測性的 CPU 算術追蹤或頂點快取。
- 影子 GP0 在命令前按值複製 Z，包含跨批次情況；消費端不得再讀機器或可改寫的追蹤器。

## 取樣契約

三角形保留既有覆蓋範圍、剔除、顏色插值、STP、E2、遮罩與混色。
三個頂點都有正的有限深度且不全相同時，以放大後頂點座標建立線性平面：

```text
q = interpolate(1/Z)
u = interpolate(U/Z) / q
v = interpolate(V/Z) / q
```

在像素中心 `x+0.5, y+0.5` 求值，使用 float64，向下取整轉回既有 8.24 UV。
分母不正、結果不有限或 UV 絕對值超過65536時，該貼素沿用原仿射值。
CPU 原色／調變及 GPU 指令準備共用此取樣；原生貼圖與 HD 素材子貼素使用同一對 UV。
GPU 仍依 `021` 執行像素繪圖，不聲稱全部運算已交給 GPU，也不以此宣稱加速。

## 選項、保存與失敗行為

- `psgolem`、`psexe` 使用 `-texture-perspective`，預設 false。
- `psgolem` 保存布林設定 `texture_perspective`，省略命令列時沿用；明確 `-texture-perspective=false` 關閉。
  已選擇後，下次雙擊自動沿用。舊設定缺欄位即為 false。
- 需要 HD2／4／8 及 PGXP；不符時以明確錯誤結束，不偷偷改畫質設定。
- API 為 `Shadow.SetTexturePerspective(bool)`，在序列化命令邊界切換。
  診斷統計只回報貼圖三角形、有效深度、不等深度及實際校正數，不參與模擬狀態。
- 存檔資料格式不變；唯有前端呈現選項保存到既有 settings.json。

## 驗收與停止線

1. 自製解析反投影公式、兩個三角形接縫、等深度／缺深度／NaN／Inf／負深度回退。
2. 同來源與混合來源半字重組、整字、原值覆寫、COP2、DMA、延遲載入及跨批次測試。
3. GTE 公開實機語料及既有 CPU／PGXP／HD 回歸通過；race 抽測真正並行搬運。
4. 真實 Intel GPU 對照 CPU：精確載入 HD 素材、原生貼圖、五種混合模式、完整 VRAM 與畫面。
5. 正常開機按鍵路線到序章 3D 場景，連續截圖核對透視差異，RAM／CPU 摘要、原生像素與卡片保持。
   不用任意 PC、RAM、座標或 direct-entry 替代正常玩家路徑。
6. 前端設定往返、明確關閉、舊設定與缺少相依選項的錯誤測試。

研究圖、遊戲素材與遊戲截圖留本機；公開只放程式、自製測試及規格。
完整遊戲美術、全程 3D 追蹤覆蓋、FPS 與 Windows／macOS GPU 真機另驗。
只有通過上述適用範圍才記錄實作完成，不把原生狀態相同當成整款遊戲 HD 完成。

## 驗收結果

正式全套與race通過，帶深度觀測的GTE實機1150筆各64暫存器相符。
正常開機到序章11724fields的20個原生摘要／像素／卡片檢查點相同，鐘樓連續三張畫面可見透視差異。
真正Intel GPU與CPU的五素材、五模式完整VRAM與畫面相同。
啟用保存、重開自動沿用及明確關閉均通過。
本機證據總入口為 `workplace/hdtex/perf/perspective-production-verification.json`。
尚未更新本機候選執行檔；本節不宣稱完整遊戲、速度與非Linux真機。
