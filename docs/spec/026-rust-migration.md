# 026 Rust改寫與同條件速度比較

狀態：整體DRAFT；§2原生軟體繪圖切片READY。2026-10-28使用者明示「psgolem用rust改寫，然後量測速度」。語言選擇已核准，取代較早「全部Go」的語言限制；遊戲專屬／中立分層、原版／HD、CPU／GPU、海報與跨平台目標保持。Rust完整機器、HD與GUI尚未完成，現行Go與雙擊候選繼續可用，不以第一個繪圖crate稱全機已改寫。

## 1. 遷移邊界

Rust來源集中於本repo的rust/，避免把新語言檔混入Go內部套件。現有cmd／internal／render均為Go來源，沒有可直接承載Cargo原始碼的目錄，所以新增單一rust根。先建立無額外相依的psgolem-core crate與原生繪圖模組；其後CPU／GTE、機器與週邊、HD／素材／PGXP、GUI／手把／海報及交付各自補READY契約再移植。舊API／存檔／HD素材格式不由語言改寫順帶更動。CPU與GTE仍須通過原有全部語料，原版遊玩／存檔／聲音／HD與實際GPU驗收不能由renderer benchmark取代。

## 2. 原生軟體繪圖切片

狀態：READY，實作完成且本節六案例／兩份月下端點及固定速度批次已驗；完整M3硬體台帳與機器接線未驗。依007-gpu-and-dma與目前正式Go的gpu.go／gp0.go／raster.go／draw.go／poly.go移植GP0命令、GP1控制、矩形／精靈、線／折線、三角形／四邊形、Gouraud、貼圖／CLUT／cache、texture window、混色／遮罩／抖色及VRAM填色／重疊複製／上傳／讀回。來源基準f3ef194edbf4e95ec255b486ed322fa191d389f4，逐檔SHA由相鄰私人研究輸入表保存。Go本身參照beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d的mednafen/psx/gpu*.c／gpu_common.h，GPL-2.0-or-later；Rust沿用本repo GPL-3.0-or-later，不搬受限來源。

Rust使用自身整數與容器實作光柵化，不呼叫Go／C++繪圖器，不執行PS1 ASM。原生RGB555／STP與快取、11-bit座標、固定小數溢位／截斷、圖元順序及交錯場奇偶保持。輸入為原32-bit GP0／GP1與場事件；輸出為自有1024x512字顯存、讀回、命令狀態。沒有原資料時相關實際串流檢查skip，不以代用品代稱月下。安全Rust實作繪圖核心，禁止以unchecked索引換取測試通過或速度。

本切片不包含CPU／GTE／DMA排程、完整GPU掃描時序／忙碌期限、GUI或HD四倍。只有Rust庫能解碼原GPU命令不算完整PS1執行器完成。影片24-bit、顯示掃描輸出與保存態跨版本相容另接既有規格；本批不以未知行為猜補。

集中驗證先跑必要無素材案例：傳輸／遮罩／重疊、混色各通道／STP、CLUT／texture window、圖元自身TPage、座標與固定小數。再用014 §6已固定的月下兩段各120場及完整原始前綴比Rust／Go／Mednafen顯存。每場資料與終點差異分開記，不以同一語言的自我比較稱原版exact。錯誤先保留來源與輸出，再修具體原因，不重開美術。

## 3. 同條件速度比較

Rust正確性通過後才量測。沿014 §6的原命令與場奇偶、同邏輯CPU、一暖身與九組交錯，重新量Rust／目前Go／Mednafen1.29三個原生CPU繪圖器。三側均預先建立顯存及快取，複製／初始化／檔案／雜湊在計時外；同步顯存工作完成才停表，同記行程CPU時間。呈現串流缺CPU讀回時三側都從原讀取介面完成唯讀傳輸。Rust release固定版本與portable CPU目標，不將debug、FFI橋或不同工作量當Rust差距。

每組需確認實際命令數、顯存差異及穩定輸出。原生比較與HD4／HD素材／PGXP／硬體GPU／GUI完整遊玩速度分列；沒有新平台真機或雙擊包時不得宣稱已交付Rust版。本輪先完成可量測的Rust繪圖器，後續全機移植仍維持OPEN。

## 4. 現成Rust PS1核心評估

狀態：DRAFT，使用者2026-10-28允許沿用現成Rust模擬器。以下是固定來源與介面查證，不是本片中文版月下實測，沒有把任何候選程式移入正式引擎。

| 候選 | 固定版本與授權證據 | 適配判斷與下一閘門 |
|---|---|---|
| CoffeePSX | Codeberg `ed7c266bf43cf9ffd4211be6d8cf15c8424033f2`；GPLv3 LICENSE。GitHub歸檔鏡像為`4eccc84adb65680c0cff91b113bde049211568f7`，不能當成canonical HEAD | 優先做可撤回的原版月下試跑候選。CPU／GPU後端、wgpu硬體繪圖、高解析度與PGXP方向較吻合。`Ps1EmulatorBuilder`公開介面可接音訊／卡片，但即使軟體後端也需要wgpu device／queue；SOTN與無頭決定性仍待實測。檢查檔案未找到明確or-later版次聲明，不自行擴張為本repo GPL-3.0-or-later或先搬程式 |
| StarPSX | `e8551eedac6d15a4ed7b7fb123c70f97217d1ed4`；GPLv3 LICENSE，core／renderer分離 | 上游同版`Compatibility.md`把月下列為卡Konami標誌；目前不作月下首選。可研究核心介面，README的快不能代替速度實測 |
| Trapezoid | `b2411afe405a4c1d33586338b0768b3343f8353f`；root與core manifest均MIT | 保留作MIT核心候選。`trapezoid-core`可由外部host驅動，但manifest明列關閉Vulkan時GPU不正常；純CPUfallback與HD接管仍需補，月下相容性未證實 |

來源：[CoffeePSX官方Codeberg](https://codeberg.org/jsgroth/CoffeePSX)、[GitHub搬移說明](https://github.com/jsgroth/CoffeePSX)、[StarPSX固定相容表](https://github.com/kaezrr/starpsx/blob/e8551eedac6d15a4ed7b7fb123c70f97217d1ed4/Compatibility.md)、[Trapezoid core固定manifest](https://github.com/Amjad50/Trapezoid/blob/b2411afe405a4c1d33586338b0768b3343f8353f/trapezoid-core/Cargo.toml)。詳細檔案與SHA只沿私人來源索引保存。舊Rustation已由上游指向[rustation-ng](https://github.com/simias/rustation-ng#cd-controller-firmware)；後者還需要本案未提供的CD控制器韌體，因此沒有擴大做新原始資料需求或拿它替代目前可玩的Go版。

目前依使用者明示，以已驗ps-golem為基準開發自己的可玩Rust版，CPU／GTE沿§5–6移植；CoffeePSX與StarPSX作參考及隔離試跑，未採用其核心為正式基底。來源相容、可重播CPU／RAM、讀卡／存卡、聲音、CPU／GPU與本片月下正常路徑驗證通過後，才把依賴／fork契約推到READY；不為沿用候選降低ps-golem的觀測、原版／HD、海報與跨平台要求。

### 4.1 首輪可撤回試跑結果

已證實：固定Codeberg版本的core與公開API外殼release建置完成，核心未修改。CUE入口要求UTF-8與固定TRACK／INDEX縮排；使用001 §6的同軌道資料、一次性正規化CUE後正常BIOS開機完成500幀，讀Track1第16扇區因EDC不符退出1。獨立MODE2 Form1 CRC核對所存3165503166、計算379105012，與候選錯誤一致，原光碟未改。未完成9000幀與月下正常遊玩；不繞過檢查、不稱相容性／速度通過。

已證實：memory.rs初始化以未固定種子的rand::random填RAM／scratch，公開builder未提供種子。中文版EDC相容策略、可重播初始化與觀測能力仍DRAFT，未定案採用或修改候選。第一個可玩的完整Rust版、HD與GPU／GUI仍待完成。工作列管[Issue #19](https://github.com/wicanr2/ps-golem/issues/19)，既有繪圖／速度沿#17／#9。

## 5. 自有Rust幾何引擎

狀態：CONFORMED，限本節原生整數架構，1150筆實機紀錄及兩項邊界通過。使用者明示以ps-golem突破並開發自己的可玩Rust版。StarPSX仍作分層／繪圖參考，不因它未通過月下而降低本引擎的原有硬體契約。此切片只移植本repo的internal/gte/gte.go，來源4951d1e；既有003的64暫存器副作用、22命令、FLAG／飽和／FIFO、44-bit溢位、UNR除法與返回週期照原契約。零初態與順序Read／Write／Execute，不依賴BIOS、遊戲或外部Rust核心。

Rust來源為rust/src/gte.rs，由rust/src/lib.rs公開；自行實作整數計算，GPL-3.0-or-later。原Go參照beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d的mednafen/psx/gte.c，GPL-2.0-or-later，沿用來源註記。保持003的推論等級，IR0上限與i44邊界不能因移植而稱為實機confirmed。

驗收使用003已固定MIT實機紀錄gte_valid_0xc0ffee_50.log，SHA256 4577214dec452509eb5e8f3ff4c120d50bb361db13381ce9fee8bad2b3520421；1150筆各64個讀回全部相同，缺檔明列skip，不能稱通過硬體語料。另驗原契約IR0上限及旗標讀寫邊界。集中執行一次，失敗保留具體輸出後修正未通過部分，不重跑舊GPU或美術。

本切片不包含CPU的COP2延遲、完整機器／週邊、PGXP呈現觀測器或遊戲入口。後續CPU／機器仍須承接004／006／008及原公開測試全部通過，正常月下、音訊、存檔、輸入、原版／HD、CPU／GPU與海報接線後才稱可玩Rust版。現行Go候選保持。

## 6. 自有Rust CPU

狀態：READY，已實作並通過364組／5824單步Go差分，限架構與單步契約；完整機器的004公開語料未驗。來源為本repo 4951d1e的internal/cpu/cpu.go與004／008，Rust來源rust/src/cpu.rs。Bus以可變借用傳入step，型別化讀8／16／32、寫入完整來源值、IsolatedWrite、InstructionBusError、ReadCost與BIU保持Go契約；CPU自有GPR35、HI／LO、PC／nextPC、branch／load delay、COP0、乘除／GTE期限及256行時間cache，GTE沿§5。reset／IRQ／普通BREAK例外、所有MIPS I／COP0／COP1／COP3不可用與COP2資料轉移及命令、未對齊、LWL／LWR／SWL／SWR、overflow及隔離寫入都照原Go，不搬StarPSX固定2 CPI。

先以無原版資料的固定種子Go差分語料，逐單步比完整架構狀態、GTE64個讀回、cache、時間、記憶體與寫入來源值。公開語料生成器為tools/oracle_rust_cpu.go，以既有tools/go.sh的Docker入口執行；來源／雜湊保存在相鄰素材的既有direct-go-v1，Rust只讀固定二進位記錄，不呼叫Go CPU。這項比較證明Go移植相符，不等於實機exact；004的公開CPU測試須在後續完整Machine接線後全部重跑，才滿足可玩版底層閘門。

首次先保留逐指令路徑；可選FetchMemory快速擷取、008 §7倒數折疊、025 typed-call及PGXP／BREAK呈現掛點另接，不以它們缺少而更動正常CPU語意，現有Go版不改。此READY不包含週邊匯流排、正式觀測API格式、GUI／HD、遊戲繪圖接管或正常月下可玩驗收。所有完整機器／HD目標仍OPEN。

### 4.2 StarPSX指定參考與本機結果

使用者明確指定https://github.com/kaezrr/starpsx。2026-10-28核對main仍為e8551eedac6d15a4ed7b7fb123c70f97217d1ed4，與前次固定版相同；本機release編譯及一次正常BIOS／同卡／等價CUE／固定按鍵試跑完成9000次run_frame，GPU計數8225。一次集中技術檢視確認4500／7000／8382／9000的完整RGBA相同且為Konami標誌，重現上游相容表，未到月下遊玩。原始碼與原資料未修改，未補錯誤容忍、時間特例或注入遊戲狀態。

參考重點為core／renderer／frontend分層、GPU裝置獨立的CPU framebuffer介面與以const generic分化圖元模式。RAM／scratch零初態可供對照，尚不是本案完整決定性驗收。System::run_frame每20指令固定增加40週期，step_instruction固定2週期；不能沿用作本案008時鐘，也不能用33.816秒的診斷執行時間推導相對Go的速度。renderer是軟體光柵化，不等於本案硬體GPU／HD已接好。

來源為固定commit的core/src/lib.rs、core/src/mem.rs、renderer/src/lib.rs與Compatibility.md；GPLv3 LICENSE及來源SHA由私人rust-starpsx-source-v1.json保存。沒有將上游實作複製進正式引擎；自有CPU與GTE沿本repo4951d1e的相容授權來源。CPU差分只證明Go移植相符，RAM／存取用摘要，完整架構／GTE／快取／時間直接比較。接下來須接匯流排、DMA、IRQ／timer、CD-ROM、SPU、SIO／卡片、MDEC與GPU掃描時序，重跑004／006原語料及正常月下後才稱Rust可玩。
