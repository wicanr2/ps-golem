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

### 4.2 StarPSX指定參考與本機結果

使用者明確指定https://github.com/kaezrr/starpsx。2026-10-28核對main仍為e8551eedac6d15a4ed7b7fb123c70f97217d1ed4，與前次固定版相同；本機release編譯及一次正常BIOS／同卡／等價CUE／固定按鍵試跑完成9000次run_frame，GPU計數8225。一次集中技術檢視確認4500／7000／8382／9000的完整RGBA相同且為Konami標誌，重現上游相容表，未到月下遊玩。原始碼與原資料未修改，未補錯誤容忍、時間特例或注入遊戲狀態。

參考重點為core／renderer／frontend分層、GPU裝置獨立的CPU framebuffer介面與以const generic分化圖元模式。RAM／scratch零初態可供對照，尚不是本案完整決定性驗收。System::run_frame每20指令固定增加40週期，step_instruction固定2週期；不能沿用作本案008時鐘，也不能用33.816秒的診斷執行時間推導相對Go的速度。renderer是軟體光柵化，不等於本案硬體GPU／HD已接好。

來源為固定commit的core/src/lib.rs、core/src/mem.rs、renderer/src/lib.rs與Compatibility.md；GPLv3 LICENSE及來源SHA由私人rust-starpsx-source-v1.json保存。沒有將上游實作複製進正式引擎；自有CPU與GTE沿本repo4951d1e的相容授權來源。CPU差分只證明Go移植相符，RAM／存取用摘要，完整架構／GTE／快取／時間直接比較。接下來須接匯流排、DMA、IRQ／timer、CD-ROM、SPU、SIO／卡片、MDEC與GPU掃描時序，重跑004／006原語料及正常月下後才稱Rust可玩。

## 5. 自有Rust幾何引擎

狀態：CONFORMED，限本節原生整數架構，1150筆實機紀錄及兩項邊界通過。使用者明示以ps-golem突破並開發自己的可玩Rust版。StarPSX仍作分層／繪圖參考，不因它未通過月下而降低本引擎的原有硬體契約。此切片只移植本repo的internal/gte/gte.go，來源4951d1e；既有003的64暫存器副作用、22命令、FLAG／飽和／FIFO、44-bit溢位、UNR除法與返回週期照原契約。零初態與順序Read／Write／Execute，不依賴BIOS、遊戲或外部Rust核心。

Rust來源為rust/src/gte.rs，由rust/src/lib.rs公開；自行實作整數計算，GPL-3.0-or-later。原Go參照beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d的mednafen/psx/gte.c，GPL-2.0-or-later，沿用來源註記。保持003的推論等級，IR0上限與i44邊界不能因移植而稱為實機confirmed。

驗收使用003已固定MIT實機紀錄gte_valid_0xc0ffee_50.log，SHA256 4577214dec452509eb5e8f3ff4c120d50bb361db13381ce9fee8bad2b3520421；1150筆各64個讀回全部相同，缺檔明列skip，不能稱通過硬體語料。另驗原契約IR0上限及旗標讀寫邊界。集中執行一次，失敗保留具體輸出後修正未通過部分，不重跑舊GPU或美術。

本切片不包含CPU的COP2延遲、完整機器／週邊、PGXP呈現觀測器或遊戲入口。後續CPU／機器仍須承接004／006／008及原公開測試全部通過，正常月下、音訊、存檔、輸入、原版／HD、CPU／GPU與海報接線後才稱可玩Rust版。現行Go候選保持。

## 6. 自有Rust CPU

狀態：CONFORMED，限架構／單步契約。364組／5824單步Go差分，以及§7–8的兩份真BIOS／六個004公開CPU程式通過；008完整時序台帳與M4裝置／HD仍另驗。來源為本repo 4951d1e的internal/cpu/cpu.go與004／008，Rust來源rust/src/cpu.rs。Bus以可變借用傳入step，型別化讀8／16／32、寫入完整來源值、IsolatedWrite、InstructionBusError、ReadCost與BIU保持Go契約；CPU自有GPR35、HI／LO、PC／nextPC、branch／load delay、COP0、乘除／GTE期限及256行時間cache，GTE沿§5。reset／IRQ／普通BREAK例外、所有MIPS I／COP0／COP1／COP3不可用與COP2資料轉移及命令、未對齊、LWL／LWR／SWL／SWR、overflow及隔離寫入都照原Go，不搬StarPSX固定2 CPI。

先以無原版資料的固定種子Go差分語料，逐單步比完整架構狀態、GTE64個讀回、cache、時間、記憶體與寫入來源值。公開語料生成器為tools/oracle_rust_cpu.go，以既有tools/go.sh的Docker入口執行；來源／雜湊保存在相鄰素材的既有direct-go-v1，Rust只讀固定二進位記錄，不呼叫Go CPU。這項比較證明Go移植相符，不等於實機exact；004的公開CPU測試須在後續完整Machine接線後全部重跑，才滿足可玩版底層閘門。

首次先保留逐指令路徑；可選FetchMemory快速擷取、008 §7倒數折疊、025 typed-call及PGXP／BREAK呈現掛點另接，不以它們缺少而更動正常CPU語意，現有Go版不改。此READY不包含週邊匯流排、正式觀測API格式、GUI／HD、遊戲繪圖接管或正常月下可玩驗收。所有完整機器／HD目標仍OPEN。


## 7. 自有Rust最小機器

狀態：CONFORMED，限006的M2＋007的GPU／DMA2／OTC及008所移植週期。1024個Go掃描事件、64份完整輸出摘要、四個機器案例、兩份真BIOS及六個公開CPU程式通過；未含完整M4裝置。來源343afa4的machine/machine.go、io.go、exe.go、internal/gpu/gpu.go及screen.go；各輸入／BIOS／公開語料SHA由相鄰素材既有direct-go-v1/rust-machine-plan-v1.json保存。CPU與GTE承接§5–6，不改其指令語意。Rust Machine分持Cpu與SystemBus，直接執行自有Rust，無Go或其他核心FFI。

記憶體、區段／鏡像、scratch例外／隔離、BIU與ReadCost沿006／008；TTY攔A(3C)及B(3D)，BIOS從BFC00000正常到ShellEntry 80030000後才側載公開測試。PS-EXE解析驗證範圍、載入／BSS與GP／SP／argc依006及既有exe.go，不注入遊戲狀態。普通ROM開機與側載硬體測試分列。

GPU新增原Go掃描與Screen契約，11/7整數時鐘、NTSC／PAL與交錯場、VBlank／GPU IRQ及顯示讀出奇偶。softReset不清掃描進度；舊GP0光柵化與命令流benchmark的set_field保持。DMA2 block／linked-list、OTC6、DICR及timer 0–2沿Go實作，CHCR及窄寫入全來源值／合併分類保持。同步先採008 ExactTiming每條指令結束推進，I/O發生在本指令尚未退休時，IRQ在下一指令邊界處理，不採StarPSX固定2 CPI。

本M2初始切片的CD／SPU／SIO／MDEC保留006 §2明示的初始化暫存器契約，SIO現已由§8取代：CD無命令進行狀態，SPU暫存器與SPUSTAT低6位，SIO傳送就緒，MDEC忽略。未移植的DMA通道沿006的完成模型，沒有偽造光碟、聲音、手把／卡片或影片資料。本節初始M2命令只接受BIOS與測試EXE；光碟入口已由§9接入，完整遊玩仍待完成。後續完整裝置須各依009／010／011／013補READY切片，這裡不當可玩或M4相容完成。

集中驗證：必要RAM／scratch／BIU／IRQ／窄寫入及DMA／timer／video與CPU取指整合，先比新GPU掃描輸出與固定Go事件；再真BIOS正常到ShellEntry，執行006列出的cpu/cop、cpu/code-in-io與gte/test-all，沿原100000000週期、原TTY正規化與唯一已知Total tests漂移。缺BIOS／公開語料明列skip；不跑舊GPU／GTE矩陣。通過範圍限Rust M2架構整合，不代替正常月下／HD／GPU前端、速度或三平台交付。

### 7.1 驗證入口與現行能力

正式來源為rust/src/machine.rs、machine/dma.rs、machine/timers.rs與gpu/video.rs；公開期望資料產生器為tools/oracle_rust_video.go及tools/oracle_rust_sio.go，不含原遊戲。Rust命令列為rust/src/bin/machine_probe.rs，Cargo binary名psgolem-machine-probe。現有Docker工具映像dosgolem-rust-sc:1.99.1-r1提供rustc/cargo 1.99.1；在有界、UID/GID對應、網路關閉的容器中於rust/執行cargo build --release --offline --locked --bin psgolem-machine-probe，CARGO_TARGET_DIR放明確可寫輸出根。這是本機既有工具鏈，不隨程式散布，未稱三平台Rust包已完成。

在ps-golem根目錄，先確認既有映像與來源，再用Docker建置。輸出留在既有workplace研究根，不建立交付包：

```sh
set -eu
test -f rust/Cargo.toml
docker image inspect dosgolem-rust-sc:1.99.1-r1
timeout -k 10s 180s docker run --pull never --rm --init \
  --user "$(id -u):$(id -g)" --memory 2g --cpus 2 --pids-limit 128 \
  --network none --log-opt max-size=10m --log-opt max-file=3 \
  -v "$PWD":/engine:rw -w /engine/rust \
  -e CARGO_TARGET_DIR=/engine/workplace/rust-target \
  --entrypoint cargo dosgolem-rust-sc:1.99.1-r1 \
  build --release --offline --locked --bin psgolem-machine-probe
```

執行時同樣保留資源／逾時與非root限制，將引擎唯讀掛至/engine、已存在的BIOS目錄唯讀掛至/bios、已存在的輸出目錄可寫掛至/out，entrypoint改為/engine/workplace/rust-target/release/psgolem-machine-probe。此入口只支援本機Linux原生M2驗證，三平台與玩家GUI待後續接線。

命令 -bios <512KB檔> [-exe <公開測試EXE> | -cue <光碟CUE>] -cycles <週期> -tty <輸出檔> -state <輸出前綴> [-wav <輸出檔>]；-stop-pc <十六進位位址>只在真正到達該PC時停止。程式先真BIOS到80030000，再依006正常側載指定公開測試。未給EXE而用-cycles 0只驗BIOS核心入口。-state是本機除錯輸出RAM、原生VRAM及CPU／GTE／快取JSON，未定義存檔或正式觀測API格式。BIOS與原資料唯讀掛載，原生畫面／RAM／卡片不加入Git。

初始M2案例與影像／公開程式均首次通過。SIO取代實際I/O空殼後另做一次必要兩BIOS／六程式整合，全部通過，沒有重跑GPU／GTE舊矩陣。BIOS入口SCPH5500為68939104週期、SCPH-100為68913704週期，皆field120；每份BIOS的cop／code-in-io／gte/test-all分別19／10／5行完全相同。CLI首次正常啟動與JSON／2MB RAM／1MB VRAM輸出另驗，不以它代替月下。

CD-ROM／DMA3與正常BIOS載入光碟EXE已由§9補齊；SPU／DMA4及WAV已由§10接入；MDEC／DMA0–1與正常開機到標題已由§11接入；實體聲音輸出與正常月下遊玩仍待接。手把與卡片目前只有原硬體協定／機器接線，USB／藍芽前端設定、原卡持久化與正常遊戲存讀檔未驗。HD、海報、GUI與完整速度目標保持，Rust可玩版未交付。詳細首輪與來源沿相鄰素材direct-go-v1/rust-machine-summary-v1.json及原始日誌，Issue #19保持OPEN。

## 8. 自有Rust手把與記憶卡

狀態：CONFORMED，限010的數位手把／SIO0及128KB原卡協定。22580個Go事件、兩張完整131072-byte測試卡、三個新案例及接線後兩BIOS／六公開程式通過；未含實體手把或遊戲存檔驗收。來源343afa4的internal/sio/sio.go與memcard.go，延續Beetle frontio.c e57860d82faa953dc0fc10f201fd583739bc2f7d GPL-2.0-or-later參照，本repo Rust實作維持GPL-3.0-or-later。Rust來源rust/src/sio.rs與sio/card.rs；不搬StarPSX裝置實作。

保留兩埠的pad／card有線AND、DTR、bit時鐘、收發暫存器、mode／control／baud遮罩、ACK延遲與32週期脈衝、IRQ／NextEvent、重置及取消選擇。Gamepad按下位元輸入為1、協定回送active-low，ID 5A41。原卡Format／Load長度、旗標／XOR／位址邊界、成功Writes及內容變更Dirty分開，寫入協定不得直接改原檔。

Machine以真SIO取代§7的SIO0初始化空殼，保留SIO1。I/O寬度及偏移沿原Go：Read/Write(p&0xE)，SIO0在每條CPU退休時Tick，IRQ上升送I_STAT bit7。Machine clone深拷貝pad/card，前端的USB／已配對藍芽設定與卡片檔案持久化尚未接，不將協定通過稱實體手把或遊戲存檔驗收。

集中驗證以固定Go原bit／register事件比較RX、SR、IRQ、時鐘、ACK及卡片摘要，含合法／錯XOR／越界／相同內容重寫及兩埠；Rust單獨執行，不呼叫Go核心。整合後只補新SIO相關的機器驗證與必要BIOS／公開CPU程式回歸，舊GPU／GTE矩陣保持。CD／SPU／MDEC與月下仍未接完，原版／HD及舊Go候選不變。

## 9. 自有Rust光碟控制器

狀態：CONFORMED，限009光碟／控制器／DMA3及013的CD音訊來源。12656個Go事件、18份原始扇區／SubQ、四個新案例、兩次正常BIOS光碟載入及必要六公開程式通過。來源f00311b的internal/disc/disc.go、internal/cdrom/{cdrom,commands,audio}.go與machine/io.go；輸入SHA固定於相鄰素材既有direct-go-v1/rust-cdrom-plan-v1.json。控制器與CD音訊延續Beetle cdc.c e57860d82faa953dc0fc10f201fd583739bc2f7d GPL-2.0-or-later來源註記，Rust沿本repo GPL-3.0-or-later，不搬候選核心。

型別為Disc／Track、Cdrom及Machine。單一BIN的MODE2/2352、MODE1/2352與AUDIO沿現行Go解析；非UTF-8 CUE檔名找不到時，僅有唯一BIN才退回並回報。保留INDEX 00／01、負LBA全零、TOC／SubQ含CRC、SYSTEM.CNF區域辨識與009 §7唯讀ISO工具的路徑／extent／長度界線。檔案以唯讀位置讀取，clone共用不可變光碟，控制器與音訊狀態深拷貝。原檔不更改、不公開。

命令00–1E的已實作集合、參數與結果FIFO溢位／空讀、bank、IRQ／ACK／ready、兩格扇區管線、seek／read／play／pause／reset、2048／2328／2340資料視圖、NextEvent與確定性中點延遲均沿009及目前Go。XA解碼／重取樣、CDDA／音量／靜音保留013 §2原演算法，音訊取樣可由呼叫端讀出；這不代表SPU混音、喇叭輸出或人工聆聽通過。機器仍採ExactTiming每條CPU退休推進，CD IRQ上升沿送I_STAT bit2；SPU整合前Tick不消耗CD音訊，沿Go Tick(clocks,nil)的既有契約，不製造假SPU或假音訊。

DMA3立即搬移的字數、零值、同步模式、RAM鏡像與MADR／BCR／DICR副作用沿現行machine/io.go。驗證CLI新增-cue，與-exe互斥；有光碟時從reset正常執行BIOS，不先跳ShellEntry或側載遊戲。可選-stop-pc只在真正執行到指定PC時停下，僅作載入驗證，不注入PC／記憶體；預設保持有界週期。缺原版資料明列skip。

集中驗證先以合成小光碟／固定Go事件，比暫存器、IRQ／NextEvent、控制器完整純量／緩衝摘要、原始扇區／SubQ、ISO檔案及CD取樣，涵蓋壞參數／命令、FIFO／ACK、讀取大小、尋軌與音訊狀態。再驗CPU匯流排IRQ與DMA3的正常接線／clone，以及實際中文版光碟SYSTEM.CNF／開機EXE雜湊。正常BIOS載入光碟EXE以真實到達其header指定入口為判準，TTY與來源對照分列；成功不稱月下可玩、M4、完整時序或HD完成。只補受此次接線影響的必要整合，不重跑已通過GPU／GTE矩陣，不從診斷耗時推FPS。SPU／MDEC、正常遊戲存讀檔、HD／GUI／海報與完整速度保持OPEN。

§9來源入口為rust/src/disc.rs、cdrom.rs、cdrom/{commands,audio,tests}.rs與machine/cd_tests.rs，公開期望產生器為tools/oracle_rust_cdrom.go。沿§7.1的Docker工具鏈；原資料唯讀、輸出既有direct-go-v1。產生器建立該研究根下的rust-cdrom-fixtures-v1合成光碟子目錄，只驗控制器／工具，不當成月下來源。PSGOLEM_RUST_CD_FIXTURES指定該研究根，PSGOLEM_RUST_REAL_CUE及PSGOLEM_RUST_CD_BIOS另選真實光碟／BIOS；沒有選原版資料就明列SKIP。


§9結果：首次兩個光碟／控制器案例通過，12656個Go事件的結果、NextEvent及完整控制器／音訊狀態FNV64摘要相同；18份完整raw sector／SubQ與3000-byte唯讀ISO檔案相同。新匯流排案例確認CPU窄／寬存取仍只取一byte、IRQ2／ACK、DMA3的RAM鏡像／MADR／BCR／DICR；正常真BIOS載入中文版光碟，在PC 80010DFC、634175911週期、field1112停止，完整561152-byte執行檔body與光碟相同。

CLI亦從reset實際到同一入口，未側載／注入PC；另由獨立raw ISO解析核對CLI RAM完整body與SHA。因CD接入真I/O，必要兩份BIOS／六公開CPU程式補驗仍全通過，BIOS核心入口週期不變。來源與首次日誌沿相鄰素材direct-go-v1/rust-cdrom-summary-v1.json。這些是Go移植差分與載入證據，並非CD實機逐週期、月下正常遊玩、人工音訊、HD或速度完成。

## 10. 自有Rust聲音處理器

狀態：CONFORMED，限本節Go算術／取樣、DMA4與正常BIOS／公開程式接線。2311個SPU事件／7530組取樣、150個CD事件／19143組取樣、8份完整RAM及三個公開程式通過；原硬體DMA時序仍有既有差異。來源a60af2b的internal/spu/{spu,firtable}.go、internal/cdrom/cdrom.go與machine/{io,machine}.go，SHA固定於相鄰素材既有direct-go-v1/rust-spu-plan-v1.json。SPU與FIR表沿Beetle mednafen/psx/spu.c／spu.h e57860d82faa953dc0fc10f201fd583739bc2f7d GPL-2.0-or-later來源，Rust沿本repo GPL-3.0-or-later，不搬其他核心。推論等級為已證實的Go移植契約，不外推實機逐週期或人工聆聽。

Rust來源為rust/src/spu.rs及spu/{voice,reverb,fir,tests}.rs。型別Spu持有24個Voice、ADPCM解碼與FIR、ADSR／音量掃描、FM／noise、混響／工作RAM、IRQ／暫存器、每768週期一組的時鐘及最多4096組交錯立體聲緩衝。整數溢位、飽和、保留shift、key on／off延遲、block end／loop、RAM環繞與IRQ副作用沿目前Go。clone深拷貝所有可變狀態；TakeAudio取走緩衝不改虛擬時間，滿了仍丟棄新的，不無界累積。

CD的無SPU tick保持§9契約，新增tick_with_spu依實際Go Tick的SPUCounter切chunk；每段先處理CD事件、再Advance SPU並由CD提供取樣。013 §3早期「先SPU再CD事件」散文與目前已驗Go來源順序不符，以本節列出的實際來源為移植契約，並保留此勘誤。不能用來源註記替代實機證據。機器仍ExactTiming逐指令退休，SPU IRQ上升送I_STAT bit9；暫存器16位元、8位元偶址整半字寫／奇址忽略、32位元拆兩次，DMA4的字數、零值、方向與MADR／BCR／DICR沿Go。

驗收以固定Go合成聲部／CD資料的暫存器與Advance／DMA事件，比完整聲部／包絡／混響純量狀態摘要、實際全部輸出取樣及RAM檢查點，涵蓋24聲部、包絡／音量掃描、noise／FM、ADPCM filter與保留shift、混響工作區及飽和、IRQ與資料FIFO。另比CD tick整合的取樣邊界／消耗結果與分段推進。新匯流排／DMA4／IRQ及clone／緩衝容量案例獨立驗；需要原資料則唯讀、缺檔SKIP。只補受SPU接線影響的公開程式及正常BIOS光碟入口，不重跑舊GPU／GTE或美術。

CLI的-wav輸出44.1kHz、16-bit、雙聲道RIFF，按虛擬場定期取走音訊，保留原-cue與-exe／-stop-pc；WAV測試驗長度／取樣／非靜音與可解碼，不稱喇叭或人工聆聽通過。正常光碟啟動仍從reset，禁止側載月下或注入遊戲狀態。MDEC／DMA0–1、完整正常月下遊玩／存讀檔、HD／GPU／GUI／海報及三平台包保持後續目標，診斷時間不換算FPS。本批不做全面效能調校。

§10工具入口為tools/oracle_rust_spu.go；-programs <輸出根> <BIOS> <tests.zip>模式沿既有研究根建立rust-spu-public-v1，固定spu/memory-transfer、stereo、test的Go逐指令TTY／PCM／完整SPU RAM。Rust相應整合限此固定週期，原硬體TTY只承接既有DMA時序差異，不稱全部實機精確。PSGOLEM_RUST_SPU_FIXTURES指定研究根，PSGOLEM_RUST_SPU_BIOS選真BIOS；沒有選BIOS即明列SKIP。


§10首次集中結果：三個SPU／CD時鐘與容量／clone案例、兩個機器案例通過。2311事件的完整聲部／包絡／混響純量摘要、7530組實際取樣及8份完整512KiB RAM相同；150個CD時鐘事件的19143組取樣完全相同，1250個非零取樣。三個公開SPU程式各正常BIOS初始化後執行100000000週期，TTY／完整輸出PCM與SPU RAM逐位元組同Go；memory-transfer沿013 §7仍差原硬體紀錄8行，含4項DMA時序失敗，沒有修成假通過。

CLI從reset正常啟動原片至80010DFC，634175912週期、field1112，561152-byte body的SHA沿§9獨立raw ISO證據一致。SPU接入後這個入口比前批多1週期，未稱整機同態或速度差距。WAV為44100Hz／16bit／雙聲道，825749組，長度等於CPU週期整除768，非零取樣1387964、峰值22239，檔案可完整解碼；未做人工聆聽／喇叭驗收。必要兩份BIOS與六公開CPU程式仍全過，未重跑舊GPU／GTE矩陣。完整結果／SHA沿相鄰素材direct-go-v1/rust-spu-summary-v1.json。MDEC、正常遊玩／存讀檔、HD／GPU／GUI／海報接線與三平台包仍未完成。

HD過場影片新增明示方向沿027-hd-movie-replacement.md。Rust原生MDEC／DMA0–1仍沿011接續；外部HD影片只替換呈現，不由離線原型推論原生副作用可以省略。


## 11. 自有Rust MDEC與DMA0／1

狀態：CONFORMED，限011既有Go解碼／時序契約的Rust移植、原生機器接線及正常開機到標題。來源b7f80d4的internal/mdec/mdec.go及machine/{io,machine}.go；輸入SHA固定於私人素材direct-go-v1/rust-mdec-plan-v1.json。解碼來源承接Beetle mednafen/psx/mdec.c e57860d82faa953dc0fc10f201fd583739bc2f7d GPL-2.0-or-later，Rust維持本repo GPL-3.0-or-later。這是已證實的Go移植契約，011 §7的實機算術／狀態／時序差異保留，不稱硬體逐週期完成。

Rust Mdec持有32-word輸入／輸出FIFO、量化／IDCT表、RLE／係數／色塊、格式與DMA重排、待輸出像素及時鐘；clone深拷貝。4／8／24／15位元、有號／mask位元、表轉置、整數溢位／符號擴充／截斷／飽和全部沿原Go；reset只清來源指定欄位並保留表。每區塊512週期、命令1週期、預存上限128，輸入／時鐘／輸出背壓均保留。Go檔頭較早「不模擬時間」與現行Tick程式及011 §4不符，以現行來源與本節時序為準。

Machine替換1F801820／824空殼，CPU讀寫後pump；資料peek不消耗輸出，狀態peek使用狀態。DMA0／1保持CHCR啟動至可搬整個區塊，遵守DPCR、MADR24-bit／RAM鏡像、方向、遞增／遞減、BCR區塊數環繞、完成DICR／IRQ3。Go現行active／dmaMaybeStart只對OTC要求bit28，011 §3較早模式0另需bit28的描述與現行來源不符；此移植沿現行Go，不暗中改原版行為。MDEC WaitingClock或DMA活動時，CPU退休後先Tick／pump再推其他裝置；否則沿ExactTiming的裝置同步點推進，保持SPU／CD／SIO相對順序。

驗證入口tools/oracle_rust_mdec.go以固定事件產生完整純量／陣列狀態摘要與回傳值，涵蓋表載入、四輸出格式、有號／mask、DMA offset、FIFO背壓、clock邊界、reset與非法命令。Machine案例驗CPU窄／寬存取、DPCR等待、DMA兩方向／負step／RAM鏡像、MADR／BCR／DICR／IRQ及clone。公開九個mdec程式沿011來源zip與真BIOS正常初始化、側載公開EXE後固定200000000週期，與ExactTiming Go比較TTY／完整原生VRAM／RAM及MDEC狀態。公開測試側載不當作月下正常玩家路徑；011既有硬體差異台帳不重寫。

工具鏈沿§7.1既有Rust1.99.1、Go1.24.13有界Docker；私人素材tools/prepare_rust_mdec.py固定本節及來源，入口docs/assets-catalog.md#render-control。PSGOLEM_RUST_MDEC_FIXTURES指定direct-go-v1，PSGOLEM_RUST_MDEC_BIOS選真BIOS；缺資料明列SKIP，不用代用品。原資料與完整輸出本機保存，公開Git只放通用程式／合成事件產生器。

正常月下另從reset與原CUE開機，至少觀察24-bit影片實際完整顯示及音訊；命令列可增有界-field停止，不注入PC／RAM。音訊仍取原SPU／XA並輸出WAV，不疊播HD MP4；本批不稱實體聲音裝置、GUI、整段HD／AI影片或三平台完成。後續可玩路徑／原卡持久化／HD接管／海報與速度仍列原目標，不由MDEC局部驗證推論達成。

§11正常光碟驗證工具為私人素材tools/probe_rust_mdec_normal.rs，直接連結自有Rust library；Go參考入口tools/oracle_rust_mdec_normal.go。兩者從reset／唯讀原CUE開始，無側載或輸入注入，最多4000000000週期、指定不超過5000場。逐場記錄GPU狀態與24-bit畫面雜湊，至少11個相異影像並正常退出24-bit才通過；每場取出原SPU取樣保存WAV，最終RAM／VRAM／TTY及音畫事件同Go分開比較。這是原生影片／回到遊戲的觀測，不代表手把可玩、HD、GUI或喇叭驗收。


§11首次結果：70632個MDEC事件／31330次回讀全部狀態與完整純量／陣列摘要同Go；1344個CPU匯流排／DMA／時鐘事件的完整RAM與裝置狀態相同。九公開mdec程式各在真BIOS正常初始化後執行200000000週期，TTY／完整2MiB RAM／1MiB VRAM及MDEC狀態同ExactTiming Go。011 §7的硬體差異台帳SHA未變，不把Go差分稱實機精確。必要兩BIOS／六公開CPU程式仍全通過，沒有重跑舊GPU／GTE矩陣。

正常中文版光碟從reset開始，Go與Rust均到field2600、1483757071週期、PC80015494；24-bit模式由1717觀測到2333，92個相異逐場畫面，已退出影片回到標題PRESS START BUTTON。逐場事件113815 bytes、完整RAM／VRAM／TTY及WAV全部逐位元組相同。WAV為44100Hz／16-bit雙聲道、1931975組、2701701個非零左右聲道取樣、峰值24574；原音取樣數等於CPU週期整除768。一次目視核對field1920的KONAMI與field2600標題。這是正常開機／原生影片／回到標題證據，不是移動／戰鬥／存檔或GUI／喇叭驗收；92是相異逐場畫面，不冒稱完整影片格數，亦不覆寫027較早156個顯示事件。

來源與首次結果沿私人direct-go-v1/rust-mdec-plan-v1.json、rust-mdec-execution-v1.json、rust-mdec-normal-execution-v1.json及rust-mdec-summary-v1.json。record_rust_mdec.py只核對既有結果並按已觀測場邊界取原生影片期間WAV，未猜配到舊H.264。編譯後只更正lib.rs首行過期能力註解，執行版本已保存rust-mdec-executed-lib-v1.rs並以source-resolution記錄；CPU／GTE／GPU源碼未改。第一次Go工具編譯的Write8／16參數型別修正後才生成首次參考，沒有重新跑失敗媒體或美術。
