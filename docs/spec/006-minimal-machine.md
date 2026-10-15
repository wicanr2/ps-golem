# 006 最小機器：匯流排、BIOS 開機、PS-EXE 側載、TTY

狀態：`CONFORMED`（2026-10-15，§7 驗收在 SCPH5500、SCPH-100 兩個 BIOS 上通過）

M2 的範圍（`001-scope-and-mvp` §4）。目標是讓 ps1-tests 的 `cpu/*` 在真 BIOS 上跑起來，
攔下 TTY 輸出與實機紀錄比對。GPU 只做到不擋住 BIOS 與測試程式，畫面不在範圍內。

文件出處：psx-spx（`psx-spx/psx-spx.github.io` commit `6d7d1bc106a7e0b616b0330fe58401ab1ba57f0f`）
的 `memorymap.md`、`iomap.md`、`kernelbios.md`、`cdromfileformats.md`、`graphicsprocessingunitgpu.md`。

## 1. 記憶體映射

| 實體位址 | 大小 | 內容 | 本階段 |
|---|---|---|---|
| `00000000` | 2 MB，鏡像到前 8 MB | 主記憶體 | 完整 |
| `1F000000` | 8 MB | DEV0 擴充區 | 讀回 `0xFF`，寫入忽略 |
| `1F800000` | 1 KB | Scratchpad（只在 KUSEG、KSEG0） | 完整 |
| `1F801000` | 4 KB | I/O 埠 | 見 §2 |
| `1F802000` | 8 KB | DEV8 擴充區（含 POST 顯示 `1F802041`） | 讀回 `0xFF`，寫入忽略 |
| `1FC00000` | 512 KB | BIOS ROM | 唯讀 |
| `FFFE0130` | 4 | 快取控制（BIU） | 存值 |

區段轉換：KSEG0（`80000000`）去掉最高位元，KSEG1（`A0000000`）遮成 `1FFFFFFF`，
KUSEG 原樣（照 Beetle `cpu.c` 的 `addr_mask`）。對不到任何區域的讀取回 0、寫入忽略。

## 2. I/O 埠（本階段的最小行為）

| 位址 | 裝置 | 行為 |
|---|---|---|
| `1F801000–1F801023`、`1F801060` | 記憶體控制 | 存值、讀回 |
| `1F801040–1F80105F` | SIO0／SIO1 | 狀態暫存器讀回「傳送就緒」，其餘存值 |
| `1F801070`、`1F801074` | 中斷控制 I_STAT／I_MASK | 完整：寫 I_STAT 是清除（AND），`(I_STAT & I_MASK) != 0` 拉 CPU 的 IRQ 線（CAUSE bit 10） |
| `1F801080–1F8010FF` | DMA | 暫存器存值；啟動傳輸時立即完成，channel 6（OTC）實作清表，其餘不搬資料；DICR 依規則產生中斷 |
| `1F801100–1F80112F` | 計時器 | 存值；計數依 CPU 週期（本階段只做 system clock 來源） |
| `1F801800–1F801803` | CD-ROM | 狀態暫存器讀回「無命令進行中」，其餘忽略 |
| `1F801810`、`1F801814` | GPU | GP0／GP1 寫入忽略；GPUSTAT 讀回「可接收命令、可接收 DMA、可送 VRAM」，奇偶行位元隨 VBlank 翻轉 |
| `1F801820`、`1F801824` | MDEC | 忽略 |
| `1F801C00–1F801FFF` | SPU | 暫存器存值；SPUSTAT 低 6 位映出 SPUCNT 低 6 位 |

GPU、CD-ROM、SPU、MDEC 的完整行為屬 M3、M4，各自另立規格。

## 3. 時間

- 主時鐘是 CPU 週期（`001-scope-and-mvp` §3.1）。
- 影像時鐘 = CPU 時鐘 × 11/7（psx-spx GPU 章「Video Clock」）。用整數累加器換算，不用浮點。
- NTSC 一個 field：263 掃描線 × 3413 影像週期。每到 field 起點對中斷控制送 VBlank（I_STAT bit 0）。
- 本階段的週期模型：每條指令 1 週期，不模擬快取與匯流排等待。`cpu/access-time` 量的就是這些，
  所以它**不在本階段的驗收**，留給匯流排時序規格。

## 4. CPU 與 i-cache

- CPU 依 `004-cpu-r3000a`，GTE 依 `003-gte`。
- 本階段不模擬 i-cache 內容，取指令直接讀記憶體。SR.IsC（bit 16）成立時，資料寫入不進記憶體
  （BIOS 用這個清快取）；`(BIU & 0x81) == 0x80` 時寫入 scratchpad（照 Beetle `cpu.c`）。
  依賴舊快取內容的自我修改程式碼會跑錯，這是已知差異。

### 4.1 指令匯流排錯誤

從下列區域取指令丟例外 6（IBE）。依據是 ps1-tests `cpu/code-in-io` 的實機紀錄，Beetle 沒有實作：

| 區域 | 實機結果 |
|---|---|
| Scratchpad | 匯流排錯誤（另見 psx-spx `memorymap.md`） |
| MDEC `1F801820` | 匯流排錯誤 |
| 中斷控制 `1F801070` | 匯流排錯誤 |
| SPU `1F801C00`、DMA `1F801084`、`1F8010F0` | 正常執行 |

其餘 I/O 區域沒有證據（未知），目前維持可執行。

## 5. PS-EXE 側載

格式（psx-spx `cdromfileformats.md`）：800h 位元組檔頭，`"PS-X EXE"` 開頭；
`010h` PC、`014h` GP、`018h` 載入位址、`01Ch` 大小、`028h`／`02Ch` BSS、`030h`／`034h` SP 基底與位移。

流程：

1. 從 `BFC00000` 開機，跑真 BIOS。
2. PC 第一次到 `80030000`（BIOS 進入 shell 的位址，psx-spx `kernelbios.md` Cheat Devices 一節）時停下。
   此時 A0h／B0h／C0h 函式表已初始化。
3. 把程式本體複製到載入位址，BSS 清零，設 PC、GP；SP 基底非 0 時設 SP 與 FP 為基底加位移。
   R4=1、R5=0（psx-spx 同節）。
4. 繼續執行。

## 6. TTY 攔截

psx-spx `kernelbios.md`：putchar 是 A(3Ch) 與 B(3Dh)，呼叫方式為跳到 `000000A0`／`000000B0`，
函式號放 R9。

- PC 等於 `A0`（任一區段鏡像）且 R9=`3Ch`，或 PC 等於 `B0` 且 R9=`3Dh` 時，把 R4 的低 8 位記入 TTY。
- 不改變執行流程，BIOS 照常處理。
- 程式的 printf 輸出經由這兩個向量（confirmed：`cpu/cop` 的 `pass - ` 行全部攔到）。
  BIOS 開機時的核心訊息（`PS-X Realtime Kernel Ver.2.5` 等）也攔得到。

## 7. 驗收

| 測試 | 判準 |
|---|---|
| ps1-tests `cpu/cop` | TTY 與 `psx.log` 正規化後相同 |
| ps1-tests `cpu/code-in-io` | 同上 |
| ps1-tests `gte/test-all` | 同上（透過 CPU 的 COP2 指令驗 GTE） |
| ps1-tests `cpu/io-access-bitwidth` | 逐格台帳，見 §7.1 |
| ps1-tests `cpu/access-time` | 不在本階段（§3） |

正規化規則（`psx.log` 是擷取工具的輸出，不是程式的原樣）：

1. CRLF 換成 LF；BIOS putchar 會把 LF 展成 CR LF，所以實得也一樣處理。
2. 期望值每行開頭的 `% ` 去掉。`cpu/*/psx.log` 有這個前綴、`gte/test-all/psx.log` 沒有，
   而原始碼 `common/test.h` 只印 `pass - ...`，所以前綴來自擷取工具。
3. 兩邊都去掉空行。
4. 實得輸出從期望值第一行出現的位置開始比。`psx.log` 從程式自己印的標題行開始，
   BIOS 核心訊息與 PSn00bSDK `ResetGraph` 的除錯訊息不在裡面。
5. 程式比實機紀錄新而多出的行，逐條列出處後才允許略過，只比對完全相同的字串。
   目前一條：`gte/test-all` 的 `Total tests: 1150`。`psx.log` 在 ps1-tests commit `8544f93` 錄下，
   這行到 `3c210f4` 才加進 `tests.c`。

每次執行給固定的週期上限，測試程式最後的無窮迴圈靠上限結束。

### 7.1 `io-access-bitwidth` 台帳

108 格中 81 格相符（2026-10-15）。不符的 27 格列在 `machine/bitwidth_gaps.txt`，
`machine/bitwidth_test.go` 每次比對：台帳外出現新的不符、或台帳內的格子變成相符，都算失敗。

8／16 位元寫入的資料寬度：`sb`／`sh` 把整個來源暫存器放上匯流排（psx-spx `unpredictablethings.md`
「I/O Write Datasize」）。表中標 `(w32)` 的區域（MEMCTRL、RAM_SIZE、中斷控制、DMA 的 ADDR／CTRL／DPCR／DICR、
計時器）把整個暫存器左移到位址對應的位元組後寫入整個字；其他區域只寫入該寬度。
《月下》的 CD 中斷處理以 `sb` 寫 DICR 的 bit 16–23，若改成「讀回合併」會把 bit 24–30 的中斷旗標寫 1 而清掉，
MDEC 輸出 DMA 的完成中斷因此遺失（`011`）。

| 類別 | 格子 | 負責 |
|---|---|---|
| 32 位元讀 16 位元暫存器時上半部的殘值（開放匯流排） | I_MASK、T0_TARGET 的 `0x1234`、`0x3c04` | 匯流排規格（待寫） |
| GPU 重置後的 GPUSTAT `0x14802000` | GPUSTAT | M3 GPU 規格 |
| CD-ROM 狀態暫存器 | CDROM_STAT | M4 CD-ROM 規格 |
| SIO0／SIO1 的模式與控制暫存器 | JOY_*、SIO_* | M4 手把規格 |
| SPUCNT 的寫入寬度 | SPUCNT | M4 SPU 規格 |
| DEV1 擴充區 `1FA00000` | EXPANSION3 | 未排定；沒有遊戲依賴的證據 |

## 8. 介面

```go
package machine
func New(bios []byte) *Machine
func (m *Machine) RunUntilPC(pc uint32, maxCycles uint64) error // 跑到 PC（三個區段都算）
func (m *Machine) SideloadEXE(exe []byte) error
func (m *Machine) Run(cycles uint64)
func (m *Machine) TTY() string
```

命令列：`cmd/psexe -bios <BIOS> -exe <EXE> -cycles N`，印出 TTY。

## 9. 已知差異

- 週期模型粗略（§3）。
- 不模擬 i-cache（§4）。
- 未對應區域讀回 0，不產生匯流排錯誤例外。
