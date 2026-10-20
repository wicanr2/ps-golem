# 007 GPU（原生解析度）與 DMA

狀態：`READY`（實作中，見 §6）

M3 的範圍（`001-scope-and-mvp` §4）。GPU 在原生解析度、以軟體光柵化畫進 1 MB VRAM，
這是決定性的基準；HD 只讀 GPU 命令流，另立規格。

## 1. 範圍

- VRAM：1024×512、每像素 16 位元（bit 15 為 mask）。
- GP0：環境命令 E1–E6、多邊形（平面／Gouraud、貼圖、半透明、三角形與四邊形）、
  線段與折線、矩形與 sprite、VRAM 填色、VRAM↔VRAM 複製、CPU→VRAM 上傳、VRAM→CPU 下載。
- GP1：重置、命令緩衝重置、確認中斷、顯示開關、DMA 方向、顯示區域與範圍、顯示模式、GPU 資訊查詢。
- GPUSTAT：依 psx-spx 的位元定義；重置值要能讓 `io-access-bitwidth` 的 GPUSTAT 格子相符
  （實機為 `0x14802000`，見 `006-minimal-machine` §7.1）。
- DMA channel 2：block 模式（VRAM 上傳、下載）與 linked list 模式（OT）。
  其他 channel 維持 `006` 的行為；channel 6 已實作。
- 光柵化移植 `beetle-psx-libretro` 的 `gpu.c`、`gpu_polygon.c`、`gpu_polygon_sub.c`、
  `gpu_line.c`、`gpu_sprite.c`、`gpu_common.h`（commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，
  GPL-2.0-or-later），只搬原生解析度路徑；放大、PGXP、寬螢幕屬 HD，不搬。

不在範圍：畫面輸出到視窗（前端，M4）、GPU 命令時序與忙碌位元的精確模型。

## 2. 驗收語料

ps1-tests（release `build-158`）。

### 2.1 VRAM 比對

| 測試 | 期望 |
|---|---|
| `gpu/rectangles`、`gpu/lines`、`gpu/triangle`、`gpu/quad`、`gpu/transparency`、`gpu/texture-flip`、`gpu/texture-overflow`、`gpu/clut-cache`、`gpu/clipping`、`gpu/uv-interpolation`、`gpu/vram-to-vram-overlap` | `vram.png` |

`vram.png` 的格式（2026-10-15 解碼確認）：1024×512、每分量 8 位元，值都是 5 位元色左移 3（0、8、…、248），
沒有 alpha，bit 15 沒有編進圖。比對方式與作者的 `tools/diffvram/main.go` 相同：逐像素完全相等。
本專案的 VRAM 以同樣方式展開（`r5<<3`、`g5<<3`、`b5<<3`）後比對。

`vram.png` 是否為實機擷取，README 與提交訊息都沒有寫（未知）。在得到實機證據前，它是回歸基準，
不是實機 oracle；相符代表與作者的參考一致。

執行方式：開機進 shell、側載測試程式、跑固定 **200,000,000** 週期後取 VRAM。

### 2.2 TTY 比對

| 測試 | 期望 |
|---|---|
| `gpu/gp0-e1`、`gpu/mask-bit` | `psx.log` |
| `dma/otc-test`、`dma/dpcr`、`dma/chain-looping`、`dma/chopping` | `psx.log` |

比對規則同 `006-minimal-machine` §7。

### 2.3 排除

| 測試 | 理由 |
|---|---|
| `gpu/animated-triangle` | 動畫，沒有期望輸出 |
| `gpu/benchmark` | 量測速度，`screenshot.png` 是畫面截圖不是 VRAM |
| `gpu/bandwidth` | 量測頻寬，依賴週期模型 |
| `gpu/version-detect` | 沒有期望輸出 |

### 2.4 台帳

每個測試一列，記不符像素數（VRAM）或不符行（TTY），同 `006` §7.1 的機制：
台帳外的新不符、台帳內變成相符都算失敗。不因一張圖過不了就放寬判準。

## 3. 實作順序

每一步跑對應測試，不等全部寫完：

1. DMA2 block 與 linked list、GP0 解析、VRAM 上傳／下載／填色／複製 → `vram-to-vram-overlap`、`dma/*`
2. 矩形與 sprite → `rectangles`
3. 線段 → `lines`
4. 平面與 Gouraud 三角形 → `triangle`、`quad`、`clipping`
5. 貼圖、CLUT、texture window → `texture-flip`、`texture-overflow`、`clut-cache`、`uv-interpolation`
6. 半透明、抖色、mask bit → `transparency`、`mask-bit`、`gp0-e1`

《月下》大量用半透明與抖色，第 5、6 步的測試與目標遊戲最相關。

## 4. 介面

```go
func (m *Machine) VRAM() []uint16  // 1024*512，觀測與 HD 後端用
```

`cmd/psexe` 加 `-vram out.png`，以 §2.1 的方式輸出。

## 5. 與參照實作的差異（依實機紀錄或測試參考圖修正）

| 行為 | Beetle | 本實作 | 證據 |
|---|---|---|---|
| CLUT 快取重載 | 貼圖模式改變就重載 | 位址相同且已載入筆數夠用（4 位元 16 筆、8 位元 256 筆）就不重載 | `gpu/clut-cache` 參考圖第 152 列（8 位元切 4 位元） |
| 「貼圖停用」位元 | 不允許時保留舊值 | 每次寫貼圖頁都改寫，不允許時清成 0 | `gpu/gp0-e1` 實機紀錄 |
| DICR bit 6 | （psx-spx 寫 bit 0–6 可寫） | bit 0–5 可寫 | `cpu/io-access-bitwidth` 實機紀錄：寫 0x78 讀回 0x38 |
| 遊戲別的 UV 修正 `Calc_UVOffsets_Adjust_Verts` | 原生解析度也套用 | 不搬 | 遊戲別 hack，不是硬體行為 |

GPUSTAT bit 28（GP0 寫入 FIFO 空）照 Beetle `CalcFIFOReadyBit`：FIFO 空就是 1，即使 CPU→VRAM 傳輸
進行中。寫成「沒有進行中的傳輸」會讓 PSn00bSDK 的 `DrawSync` 在送像素之前卡死（`gpu/clut-cache`、`gpu/mask-bit`）。

## 6. 驗收結果（2026-10-15，含 `008-cycle-model`；SCPH5500 與 SCPH-100 相同）

台帳 `machine/m3_gaps.txt`，`machine/m3_test.go` 每次比對。

| 測試 | 不符 | 原因 |
|---|---|---|
| `gpu/triangle`、`quad`、`clipping`、`texture-flip`、`texture-overflow`、`uv-interpolation`、`vram-to-vram-overlap`、`clut-cache` | 0 | |
| `gpu/transparency` | 0（只比對 320×240） | 參考圖在 commit `da6b472` 錄下，那一版用白色清 VRAM；`2fe1570` 重構後改清成黑色。程式只畫 320×240 |
| `gpu/gp0-e1`、`gpu/mask-bit`、`gpu/rectangles` | 0 | `rectangles` 每個 frame 在同一處重畫半透明矩形、不清背景，混色逐 frame 累積；結果依賴 frame 數，需要 `008-cycle-model` 才正確 |
| `gpu/lines` | 64 像素 | 測試程式 `setRGB2` 呼叫兩次、沒有 `setRGB3`，第四頂點顏色是未初始化的堆疊內容 |
| `dma/otc-test` | 2 行 | 實得多出 `testOtcBigTransfer`、`testOtcControlBitsAfterTransferWithChopping` 的 pass 行；實機紀錄沒有這兩行，但同一 commit 的原始碼已含這兩個測試（原因未知） |
| `dma/chain-looping`、`dma/chopping` | 8、260 行 | 量 DMA 的週期數與自我參照串列，需要 DMA 時序；本專案 DMA 立即完成。`chopping` 的不符全部是「took N CPU cycles」計時行 |
| `dma/dpcr` | 0 | `013` 加入 SPU DMA 後相符 |
| `spu/memory-transfer` | 8 行 | DMA 時序（模式 1 區塊間讓 CPU 執行、傳輸耗時）；另有 `SyncMode0` 兩個測試比 `psx.log` 新，實機紀錄沒有 |

DMA 開始條件照 Beetle `dma.c`：CHCR bit 24 一設就開始，只有 channel 6 另需 bit 28。

BIOS 開機到 SCE 開場畫面（SONY 字樣、GTE 投影的鑽石標誌與 Gouraud 漸層），以 `psexe -vram` 目視確認。
與參考模擬器逐像素比對尚未做。

## 7. 已知差異

- 不模擬 GPU 命令時序；GPUSTAT 的忙碌位元在命令收齊後立即就緒。
- 顯示輸出只做 §8 的靜態截取；掃描時序、隔行逐場合成不在本規格。

## 8. 畫面截取（`GPU.Screen`）

把目前顯示的區域從 VRAM 取出成 RGB 影像，給 `psexe -screen` 與之後的視窗前端用（psx-spx `gpu.md`）：

- 起點：GP1(05) 的 X、Y。
- 寬度：GP1(06) 的水平範圍 (X2−X1) 除以點時脈除數，取 4 的倍數（psx-spx 的 `((X2-X1)/cycles_per_pix+2) AND NOT 3`）；
  範圍為 0 或超過 GP1(08) 的水平解析度時用水平解析度（256／320／368／512／640）。
- 高度：GP1(07) 的垂直範圍 (Y2−Y1)；GP1(08) 為 480 線且隔行時乘 2；範圍為 0 時用 240。
- 15 位元模式：每像素一個 VRAM 字，5 位元色展開為 8 位元（`v<<3 | v>>2`）。
- 24 位元模式（GP1(08) bit 4）：從起點開始每 3 位元組一個像素（R、G、B），一列佔寬度 × 1.5 個 VRAM 字。
- 超出 VRAM 的座標在 1024×512 內環繞。

## 9. 精靈中性色調變的等價省略

狀態：`READY`。依現有 `raster.go` 的 `modTexel(2,3,...)` 與抖色表，
精靈固定使用的偏移為 0；每個 5 位元色值 c 在強度 128 時為
`min(31, ((c * 128) >> 4) >> 3) = c`，bit15 原樣保留。

- 只在精靈的 R、G、B 全部等於 128 時省掉調色，其他強度保持原路徑。
- 保留原始 texel 為 0 的透明判斷、混色、遮罩、快取、寫入順序與所有計時。
- 不把此捷徑套在帶抖色的多邊形。HD 精靈及替換素材的精靈子像素依相同規則。
- 驗收對全部 65536 個 16 位元色值，與保留的原 modTexel 路徑比較；
  另跑 GPU 語料、HD 素材與競態回歸。以正常路線及實際速度驗證，不以恆等式單獨宣稱可玩完成。

## 10. 多邊形自身貼圖頁的混色模式

狀態：`CONFORMED`，限本節順序與Linux驗證。2026-10-20 壁掛燭台的 1080p 寬螢幕抽樣發現黑暈。
`internal/gpu/poly.go` 與 `render/hd/poly.go` 在 `setTPage` 前建立 `f.mode`，
會沿用上一筆命令的 ABR。相鄰同素材可能因此套用不同混色。

證據：`beetle-psx-libretro` 的 `mednafen/psx/gpu.c`，固定 commit
`e57860d82faa953dc0fc10f201fd583739bc2f7d`，GPL-2.0-or-later。
`ProcessFIFO` 先從 `CB[4 + ((cc >> 4) & 1)] >> 16` 呼叫 `SetTPage`，
再以 `command->func[GPU.abr][GPU.TexMode]` 分派。下載來源為
https://raw.githubusercontent.com/libretro/beetle-psx-libretro/e57860d82faa953dc0fc10f201fd583739bc2f7d/mednafen/psx/gpu.c 。
psx-spx 的 TPage attribute 與 GP0(E1h) 亦規定多邊形貼圖頁含 ABR bit5–6：
https://psx-spx.consoledev.net/graphicsprocessingunitgpu/#tpage-attribute-parameter-for-textured-polygon-commands 。

| 項目 | 契約 |
|---|---|
| 輸入與順序 | 貼圖多邊形先載入自身 TPage，再取 ABR；三角形與四邊形、平面與 Gouraud 都適用 |
| 非貼圖多邊形 | 維持目前 draw mode 的 ABR |
| ABE 與 STP | ABE 關閉時不混色；ABE 開啟時只有 STP1 貼素混色；word0 保持透明 |
| 邊界 | 前一 ABR 與本次 ABR 的 16 組轉換都須正確，側邊畫布與中央相同 |
| 輸出 | 原生、HD CPU 與 GPU 使用同一個正確模式；不改時序、命令字或 RAM |
| 驗收 | 獨立整數公式檢查 4 種混色與 ABE/STP/透明；倍率 1/2/4/8、中央與側邊；公開 GPU 語料和現有回歸；正常 M4 與 1080p 重抓 |

研究與原始失敗畫面留在 `workplace/hdtex/perf/candle-wall-observation-v1/redraw-v1/`。
舊有 232 點原生畫面收據保留為修正前基準。新原生畫面若改變，不冒稱與舊錯誤輸出完全相同，
另驗證 RAM、CPU 摘要、記憶卡與輸入序列，並保存新畫面證據。

回歸入口：`internal/gpu/polygon_page_test.go` 的 `TestPolygonOwnTPageBlend`，以正常 GP0 命令同時覆蓋原生與影子側邊。

本節驗收通過：2048組公式、正式套件完整回歸、121組Intel GPU、完整M4雙機262點、232個封存舊機摘要及232個修正後原生像素。1080p實際35000黑暈消失，同一素材檔未改。三平台建置通過；Windows Wine只啟動600，macOS限結構，真機尚未驗。完整收據入口為上述研究區wall-ready-verification.json與私人素材repo的hd-candle-wall-preview-install/delivery-verification.json。
