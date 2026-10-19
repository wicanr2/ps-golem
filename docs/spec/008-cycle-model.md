# 008 週期模型

狀態：`CONFORMED`（2026-10-15，見 §5）

## 1. 為什麼需要

`006-minimal-machine` §3 的「每條指令 1 週期」讓 CPU 相對影像時脈快了好幾倍。
已經造成的錯誤（2026-10-15 量到）：BIOS 的 VSync 等待迴圈最多跑 0x8000 次，
每次讀 RAM 裡的 VBlank 計數；每條指令 1 週期時約 33 萬週期就跑完，
早於一個 field（約 57 萬週期），BIOS 一直印 `VSync: timeout`。
M4 的 CD-ROM、XA、SPU 也都綁在 CPU 週期上。

## 2. 模型

時間基準仍是 CPU 週期（`001-scope-and-mvp` §3.1）。每條指令的週期數 =
1 + 取指令成本 + 資料讀取成本 + 乘除法／GTE 等待。寫入不計成本（寫入佇列）。

### 2.1 資料讀取成本

數值取自 ps1-tests `cpu/access-time` 的實機紀錄：表中的數字是「迴圈多一條讀取指令」
多花的週期，包含讀取指令本身的 1 週期，所以讀取成本 = 表值 − 1，四捨五入成整數。

| 區域 | 8 位元 | 16 位元 | 32 位元 | 實機表值（8／16／32） |
|---|---|---|---|---|
| RAM | 4 | 4 | 4 | 5.21／5.03／5.14 |
| BIOS ROM | 6 | 12 | 24 | 7.06／12.94／24.94 |
| Scratchpad | 0 | 0 | 0 | 1.05／1.01／0.94 |
| DEV0 擴充 1 | 6 | 12 | 24 | 6.94／13.07／25.07 |
| DEV8 擴充 2 | 10 | 25 | 55 | 10.99／25.99／55.98 |
| DEV1 擴充 3 | 5 | 5 | 9 | 6.07／6.01／9.95 |
| CD-ROM | 7 | 13 | 25 | 8.00／14.00／25.93 |
| SPU | 17 | 17 | （見下） | 17.99／17.99／38.94 |
| 其餘 I/O（DMA、SIO、記憶體控制、中斷、計時器、GPU、MDEC） | 2 | 2 | 2 | 約 2.92–3.18 |
| 快取控制 `FFFE0130` | 0 | 0 | 0 | 0.95／1.09／1.09 |

SPU 的 32 位元格：測試位址 `1F801DAA` 不是 4 的倍數，編譯器產生 `lwl`／`lwr` 兩條指令
（反組譯 `access-time.exe` 的 `80013fc8`、`80013fd0` 確認），兩條各做一次 16 位元讀取，
所以表值是兩次 16 位元讀取的總和，不是一次 32 位元讀取。對齊的 32 位元 SPU 讀取成本沒有量測，
暫用 38（假說）。

LWL／LWR 照 Beetle 依位址對齊發出 8、16、24（成本同 32）、32 位元的存取：
LWL 低 2 位元為 0、1、2、3 時分別是 8、16、24、32 位元；LWR 為 32、24、16、8 位元。

表值的寫法是 `printf("%2d.%2-d", 週期/100, 週期%100)`，所以 `5.3` 是 5.03、`3.1` 是 3.01。

### 2.2 取指令成本

- 經快取的區域（KUSEG、KSEG0）且 BIU bit 11（i-cache 開啟）成立：模擬 i-cache 的標籤與有效位元
  （256 行、每行 4 字，psx-spx `memorymap.md` i-Cache 一節）。命中 0；沒命中時從該字填到行尾，
  成本 3 + 填入字數（照 Beetle `cpu.c` ReadInstruction）。只模擬時間，指令內容仍從記憶體讀。
- 不經快取（KSEG1，或 i-cache 關閉）：成本 = 該區域 32 位元讀取成本（RAM 4、BIOS 24）。
- SR.IsC 成立時的寫入（BIOS 清快取）讓對應的快取行失效。

### 2.3 乘除法與 GTE

照 Beetle `cpu.c`：
- MULT／MULTU 完成時間 = 發出時間 + `MULT_Tab24[lzcount(...)]`（7、10 或 14）；DIV／DIVU 為 37。
  MFHI／MFLO 在完成前等待。
- COP2 命令完成時間 = 發出時間 + `gte.Execute` 回傳的週期數 − 1；之後的 MFC2、CFC2、MTC2、CTC2、
  LWC2、SWC2、COP2 命令在完成前等待。

### 2.4 不模擬

- 讀取延遲被後續指令吸收（Beetle 的 ReadAbsorb）：表值是實機淨成本，已含常見情況的吸收。
- DMA 搶匯流排的週期。

### 2.5 週邊同步

週邊（GPU 掃描、計時器、CD-ROM、SIO、MDEC）不必每條指令推進。機器累積已執行的週期，
在下列時刻才一次交給週邊：

- 到達期限：各週邊回報「最多再過多少週期狀態就可能改變或拉中斷」，取最小值。
  GPU 取下一條掃描線結束；計時器只看開了中斷的，點時脈來源取保守下界，HBlank 來源由掃描線期限涵蓋；
  CD-ROM 取各計數器；SIO 取傳輸中的下一個位元與 /ACK 延遲；一次最多累積 65536 週期。
- 程式存取 I/O（`1F801000`–`1F802FFF`）的前後：存取前先同步，週邊在存取當下的狀態與逐條指令推進時相同；
  存取後再同步一次，更新中斷線與期限。
- MDEC 在等時脈、或 DMA channel 0／1 啟動中：MDEC 與 DMA0／1 逐條指令推進（MDEC 時脈有 128 週期預存上限，
  一次推進大量週期會少算，`011` §4）；其他週邊照常延後。MDEC 只透過 DMA 寫 RAM 與 DMA 中斷影響其他部分，
  拆開推進不改變結果。

每個週邊推進 a 再推進 b 的結果，必須與一次推進 a+b 相同（計時器跳到下一個目標值或 0xFFFF；
CD-ROM 依計數器分段）。判準是逐位元等價：同一起點以 `ExactTiming`（每條指令同步）與延後同步各跑一次，
RAM、scratchpad、VRAM、CPU 暫存器、TTY、週期與 field 完全相同（`machine/sync_test.go`、`psexe -exact -digest`）。

結果（2026-10-15，docker 2 CPU）：《月下》開機、動畫、按鍵進入序章 7B 週期，兩種方式摘要與記憶卡相同；
執行時間 519 秒降到約 190 秒。分段速度（`PSGOLEM_SPEEDLOG`）：開機約 2–3 倍實機、選單與序章遊戲中
約 1.2–1.6 倍、序章動畫約 1.2–1.5 倍；標題畫面約 0.7–0.8 倍，時間花在 GPU 光柵化。

## 3. 驗收

| 項目 | 判準 |
|---|---|
| BIOS 開機 | 600,000,000 週期內 TTY 沒有 `VSync: timeout` |
| ps1-tests `cpu/access-time` | 每格與實機表值差距在 1 週期以內；超出的格子列入台帳 |
| 既有驗收（`003`、`004`、`006`、`007`） | 不退步；時序相依的格子另列 |

`access-time` 的判準用容差：它量的是平均週期，週期模型的目標是比例正確，不是逐週期相同。

## 5. 驗收結果（2026-10-15）

- BIOS 開機 600,000,000 週期：沒有 `VSync: timeout`（改前每個 field 都逾時）。
- `cpu/access-time`：51 格中 50 格在 1 週期以內。
  `SPUCNT/32` 期望 38.94、實得 36.07：實際是 `lwl`／`lwr` 兩次 16 位元 SPU 讀取，
  實機比兩次單獨的 16 位元讀取多約 3 週期，原因未知。列在 `machine/accesstime_test.go` 的已知差異。

## 4. 參照

`beetle-psx-libretro` 的 `mednafen/psx/cpu.c`（ReadInstruction、MULT_Tab24）、`libretro.c`（PSX_MemRead*），
commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，GPL-2.0-or-later。

## 6. 保留狀態與週期的取指優化

狀態：`CONFORMED`（2026-10-18；僅此等價取指範圍，速度仍獨立驗證）。

範圍只限 FetchMemory 已提供 RAM、PC 在 KSEG0 的 8 MiB RAM 視窗、
BIU bit 11 開啟的指令擷取。此路徑先直接檢查既有 i-cache 行的 tag／valid；
未命中仍呼叫 §2.2 的 icacheCost。內容每次從實際 RAM 讀取，不保存指令副本。

RAM 索引用實際長度鏡像；i-cache tag 用實體位址，不先做 RAM 鏡像。
其他位址、快取關閉與沒有 FetchMemory 的匯流排仍走原 fastFetch。
未對齊例外、擷取後的中斷檢查、load delay、分支與執行順序維持原樣。
不跳過指令、不更改週期、不加入遊戲位址特例。

證據為原 cpu.step 與原型逐步比較完整 CPU 狀態及週期的 4096 個案例，
涵蓋 cache hit／miss、tag、RAM 鏡像、KUSEG／KSEG0／KSEG1、未對齊與 IRQ。
同程序交錯執行真實開機後的 field 7200 至 7800，三組 CPU 時間為
舊 5.080／4.897／5.310 秒、新 4.511／4.453／4.315 秒。
六次終態摘要皆為 800e0aa7bce449bf。這是局部效能證據，完整玩家路徑另驗收。

驗收：保留修改前 step 為測試 oracle；公開 CPU／GTE 語料及 access-time
不得退步；正常 M4 流程與既有終態逐位元一致。原始證據留在本機
workplace/hdtex/perf/cpu-hot-benchmark.json，沒有原版檔案進版控。


§6 驗收完成：公開 CPU／GTE／access-time、17 個 GPU 語料與完整正常 M4 通過。
M4 為 26150980006 週期、field 45784，摘要尾碼 6c35c2701228722f，
卡片與原生畫面符合既有 SHA-256。HD2＋PGXP＋研究素材的終點 PNG 與改前逐位元相同，
SHA-256 a1e60cc76521bfe6217231f47fdf3b48f3014626a7e65a44d761ffff1c16bb9a。
收據 workplace/hdtex/perf/{candidate-oracles.log,candidate-native-m4.log,native-candidate-m4/}。
不把這些狀態等價結果稱為速度通過、全遊戲通關或新的畫面排程已定案。


## 7. 有界純 RAM 倒數與門檻迴圈

狀態：CONFORMED（2026-10-18，限等價行為）。原型及勘誤入口為 workplace/hdtex/perf/poll-loop-proposal.md。
272 組完整狀態、事件／IRQ 與有效 v2、v3 正常 M4 支持正式採用；既有 §1–6 的 CONFORMED 範圍不變。
正式程式移除研究用全域原子計數器，保留全部拒絕閘門與 PGXP 回呼。

### 7.1 證據與輸入

正常玩家指令取樣 guest-pc-probe.json 找到兩段共 14 條指令的 RAM 迴圈；
函式名未知，不依遊戲位址判斷。第一個分支與另一個 GPR 比較，不能誤限 zero。
v1 正常 M4 原生不變但 0 次命中，該結果不作為加速證據。
v2 有效完整 M4 為 6870854 批、14276539524 週期，CPU／RAM 摘要、卡片、
原生與最後 HD 圖保持既有值。source／收據見 poll-v2-short-verification.json、poll-v2-m4.log。

泛用形狀為兩段 8＋6 條：載入 counter、NOP、ADDIU -1、寫回、重載、NOP、
BNE temp/terminal、NOP；再 LUI／LW poll、NOP、有號 SLT temp/bound、BNE 返回、NOP。
沒有固定遊戲 PC、RAM 位址或暫存器編號。temp、base、terminal、bound 的必要互異關係依原型檢查。

### 7.2 必要閘門

- 只由 RunUntilField 在已完成的向後分支邊界嘗試；Step、Run、RunUntilPC 保留逐指令。
- 明示 PollingRAM provider 才可用：同 FetchMemory backing、power-of-two 長度、
  普通 RAM 讀寫無 I/O／觀測副作用，LW 額外固定 4 週期，ReadCost 無觀測副作用。
- 所有指令是已暖 i-cache 的對齊 KSEG0 RAM，保持原 cache 標籤；TTY 掛點區排除。
  每次重新驗證實際指令，不建立跨寫入的推測性解碼快取。
- BIU cache 啟用、SR.IsC 關閉、無待處理 IRQ、branch delay 或 delayed load，
  nextPC=PC+4。未知 bus 或精度 observer 回退。
- counter／poll 都是對齊普通 RAM，不能別名；counter 不能與任一指令字別名。
- 精度為 nil，或明示支援 RepeatRAMCountdown 且不觀測中間 native RAM。
  內建 PGXP 保留每次原 Load／Store 掛點及時間戳。
- ExactTiming、MDEC 等待或 DMA0／1 活躍時回退。不能跨 Machine.nextEvent 的期限、
  呼叫者週期上限或 uint64 時間溢位。

### 7.3 狀態與終止

每次暖快取迭代 26 週期。signed poll < bound 才能保持返回。
terminal 為第一個 BNE 的未修改 GPR；zero 明確視為 0，不能與 temp 相同。
D=uint32(counter-terminal)，只折疊 D-1 次；D=0 時距離完整一圈，最多 0xFFFFFFFF 次。
單批仍限制 1024 次及 deadline/26；不執行改走退出路徑的迭代。

僅合併 counter 的最後普通 RAM 寫入。PC／nextPC 回到迴圈入口，temp=1、
GPR0=0、ldNone 格與 ldValue=最後 poll、cost=1、branched=false；now 增加 26N。
HI／LO、GTE、COP0、cache、乘除／GTE ready 時間保持。
Machine 的 cycles／pending／mdecPending 同增 26N，到期限時照舊 sync。

內建 PGXP 每迭代按原順序：
Load counter at +0 ready +5；Store counter-1 at +7；
Load counter-1 at +8 ready +13；Load poll at +17 ready +22。
保留所有原 metadata，包括部分有效旗標及 pending load，不以停用 PGXP 取得加速。

### 7.4 驗收範圍

272 組 CPU／RAM／完整 PGXP 比對與 race、未知 observer／程式改變／別名等拒絕、
真 BIOS 的事件邊界與排定 VBlank IRQ 通過。正常 field 1454 逐指令與 v2、
HD2／PGXP 三者摘要相同且 v2 有命中。
v2 完整有效 M4 通過；真正 GUI 舊固定 P 核心最低 0.260，解除固定核心的短段約 0.762–0.938，
另關閉 WS 約 0.786–0.996。不同負載不當作因果 A/B，不能宣稱完整速度過關。
v3 加入 §8 的分派後，獨立完整正常 M4 通過（1245.25 秒）。
field 45784、摘要尾碼 6c35c2701228722f、記憶卡及原生／HD PNG 與基準相同；
仍有 6870854 批、14276539524 週期命中。入口 poll-v3-m4.log、poll-v3-m4/。
實際 Intel GUI 仍有低速區，不能宣稱速度通過。

## 8. NOP 與 KSEG1 BIOS 的分派研究

狀態：CONFORMED（2026-10-18，限等價分派）。入口 workplace/hdtex/perf/cpu-dispatch-proposal.md。

- NOP 仍先執行原取指成本與中斷判定。之後原 doLDS、GPR0=0、advance 照舊；
  不能省略 ldNone 備用格，也不能在 IRQ 前提前返回。其他 opcode 的 execute 不變。
- KSEG1 BIOS 只在原 FetchMemory 通道有效、ROM 恰為 512 KiB、對齊 PC 位於
  BFC00000–BFC7FFFF 時直接讀同一 LE32，額外成本仍 24。
  cached BIOS、其他 ROM 大小／區段、未知 bus 都走原路徑，不改 IBE 判斷。
- 合成 ISA-ROM 只驗取指，不能代替實際 BIOS 開機或正常玩家路徑。

480 組與保留原 step 的完整 CPU 狀態、272 組迴圈／PGXP 及 race 通過，192.618 秒。
公開 CPU／GTE／access-time 通過，SPUCNT/32 的既有差異維持，沒有新增差異。
微基準見 dispatch-v3-bench.log，屬局部分派，不宣稱整體倍速。
正常 v3 M4 獨立通過，完整狀態及兩種畫面保持基準值，見 §7.4。
實際 GUI 速度尚未通過，跨平台新包尚未建置；READY 僅授權等價優化，不代表交付或完整速度完成。

正式採用收據：workplace/hdtex/perf/formal-v3-adoption-verification.json。正式 CPU／PGXP 全套及倒數／分派／機器事件與 IRQ race 均通過；source 與日誌雜湊在收據內。M4 原型額外的研究計數器不在正式來源，沒有以計數器改 guest 狀態。完整 GUI 正常首存通過，但速度最低 0.394，仍未通過速度閘門。CONFORMED 只涵蓋本節的等價行為，不宣稱本機完整版已更新、全遊戲已玩完或速度完成。
