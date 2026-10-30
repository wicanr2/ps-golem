# 規格索引

SDD：量測／反組譯 → 規格 → 才寫程式。只有標 `READY` 的規格可以動手。
引用規格連號碼帶檔名，例如 `002-references-and-licensing`。

| 規格 | 狀態 | 內容 |
|---|---|---|
| `001-scope-and-mvp.md` | DRAFT | 目標、分層、里程碑、已核實的 BIOS 與光碟事實 |
| `002-references-and-licensing.md` | READY | GPL-3.0-or-later、可參照與只能讀的來源 |
| `003-gte.md` | CONFORMED | GTE 暫存器與命令，1150 筆實機紀錄 |
| `004-cpu-r3000a.md` | CONFORMED | CPU；與參照實作的差異依實機紀錄修正 |
| `005-cross-platform-delivery.md` | READY（實作完成，macOS 待實機） | 三平台在 Linux docker 內建置與打包 |
| `006-minimal-machine.md` | CONFORMED | 匯流排、BIOS 開機、PS-EXE 側載、TTY；週邊台帳 |
| `007-gpu-and-dma.md` | READY（實作中） | GPU 原生光柵化、DMA2；M3 台帳；§10 多邊形自身 TPage 混色順序已驗 |
| `008-cycle-model.md` | CONFORMED（含 §7–8 等價優化；速度未過） | 取指令、讀取、乘除法、GTE 的週期成本 |
| `009-cdrom.md` | CONFORMED | cue／bin、CD-ROM 控制器、DMA3 |
| `010-pad-and-memcard.md` | READY（手把完成） | SIO0、數位手把、記憶卡、按鍵腳本 |
| `011-mdec.md` | READY（實作完成，台帳見 §7） | MDEC 動畫解碼、DMA0／1 |
| `012-frontend-ebiten.md` | READY（實作完成） | 視窗、鍵盤與手把、記憶卡自動存檔、聲音輸出、設定畫面與拖放；§11保存輸出解析度，1080p開機抽樣通過 |
| `013-spu-and-cd-audio.md` | READY（實作完成，待人工聆聽） | SPU、XA、CD-DA |
| `014-cross-oracle.md` | READY | mednafen 交叉 oracle：時間表送鍵、平移比對截圖 |
| `015-sotn-autoplay.md` | DRAFT（M4 驗收完成，見 §7） | 《月下》RAM 位址台帳、Clone 搜尋自動遊玩、M4 驗收 |
| `016-hd-output.md` | READY（第 1、2、3 階段實作完成，驗收見 §7） | 影子 GPU 內部解析度放大、xBR 濾鏡、PGXP；寬螢幕見 017 |
| `017-sotn-widescreen.md` | READY（實作與指定驗收完成，平台限制見 §7） | 《月下》寬螢幕：既有遊戲修改模式；§12正式NO3鄰房來源／命令已實作與受控通過，GUI／交付待驗 |
| `018-hd-texture-replacement.md` | CONFORMED（既有素材鏈）；§4.1 快取 READY | 上傳辨識、傾印、索引／全彩素材包、子貼素與來源／快取追蹤 |
| `019-hd-texture-pack.md` | CONFORMED（僅載入器層）；§8 CONFORMED；§8.1 CONFORMED（載入／資料取樣）；§8.2 CONFORMED（載入／資料取樣）；§8.3 CONFORMED（清單界線）；§8.4 CONFORMED（圖像容量／接續繪圖資料） | HD manifest v1、索引 PNG、全彩 PNG 與 STP 遮罩的載入與驗證 |
| `020-gamepad-toolbar.md` | READY（實作與隔離驗收完成，實體手把待真機） | 自動隱藏工具列、USB／已配對藍芽手把設定、指派、死區與即時測試 |
| `021-hd-gpu-renderer.md` | READY（GPU功能已驗證，速度未過） | 使用者要求HD繪圖交給GPU；整數像素後端、CPU／GPU相依與等價驗收 |
| `022-hd-texture-perspective.md` | CONFORMED（限Linux功能及序章抽樣） | HD選用深度追蹤、透視取樣、保守回退與設定保存 |
| `023-hd-sprite-runs.md` | READY | 可選精靈繪圖序列、來源驗證、逐字格HD替換與保守回退 |
| `024-poster-presentation.md` | CONFORMED（限Linux；Windows／macOS已編譯，待真機） | 外部成對海報、工具列切換、外側GPU融合、安全等比放大及設定保存；§6 Rust海報／工具列／偏好限定CONFORMED，取樣邊界差異明列 |
| `025-hd-primary-native-demand.md` | DRAFT（主繪圖接管）；§9 CONFORMED（原生顯存觀測封裝）；§11 CONFORMED（正式相容平面接線） | Go直接場景／精靈／文字繪圖；相容平面沿§11，純Go圖層／時鐘、HD4排序與PGXP／雙平面研究沿§12；就地提交與原版／HD分流沿§12.14–12.15；§12.16通用CPU資料及§12.17普通NO3正式CPU-HD2入口CONFORMED，§12.18物件幾何12次相同但SDK／完整副作用未接，仍DRAFT；先全Go後調速，熱切換／其餘圖元未完成，真寬視野沿017 |
| `026-rust-migration.md` | DRAFT整體；§2原生軟體繪圖READY，已實作／有限驗證 | Rust原生六案例與月下兩端點相同，快Go1.47至1.76倍；§5–8原生GTE／CPU、M2機器及SIO協定已驗；兩BIOS／六公開程式通過；§9光碟／DMA3／CD取樣及正常BIOS載入遊戲EXE已驗，§10 SPU／DMA4／CD時鐘及WAV已驗，§11 MDEC／DMA0–1與原片正常開機到標題已驗，原音／RAM／VRAM同Go；§12腳本輸入／原卡檔案／正常序章12000場同Go，§13正常首存／重新開機讀檔M4切片已驗，原卡／原圖同既有Go，讀回完整資料同Go；§14原版SDL玩家視窗限定CONFORMED，正常9000場同既有基準；024 §6海報、027 §15兩段HD影片GUI已驗；§15圖像逐值診斷限定CONFORMED；§16整段繪圖／3D接管DRAFT；§16.3.1每場RAM初始化資料及§16.3.2 CPU端點／快取限定CONFORMED，§16.3.3無I/O初始化裝置提交限定CONFORMED，§16.4保存NO3場景與19材質CPU-HD4限定CONFORMED，正式玩家入口未接；實體聲音／完整手把設定與遊戲HD待接 |
| `027-hd-movie-replacement.md` | DRAFT整體；§5／§6／§9／§12／§13／§14／§15限定CONFORMED | 實際CD來源與原生時間線、共用Session暫停／續播／原生結束已驗；正常12000場4258場HD、100次凍結呈現、續播600場同原生分支，OPN影像與原音保持；§15 LOGO／OPN正式Rust視窗接線限定CONFORMED；GPU、TYK／END事件及完整包仍待接 |

狀態定義：

- `DRAFT`：還有會改變行為的未知，只允許研究與可丟棄的 probe。
- `READY`：輸入、狀態轉移、邊界、輸出與驗收測試都有證據支持。
- `CONFORMED`：實作通過內部測試與所有適用的 oracle。
- `SUPERSEDED`：被新證據推翻，保留舊證據與訂正理由。
