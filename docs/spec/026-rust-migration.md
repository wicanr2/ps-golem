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


## 12. Rust按鍵腳本、原卡檔案與命令列玩家路徑

狀態：CONFORMED，限010既有輸入格式／原卡與012的有界命令列重播；本節不選定或更換GUI工具鏈。來源417fbcb的internal/padscript/padscript.go、internal/sio/sio.go、010／012與已驗apps/sotn/testdata/m4-first-save.press；逐檔SHA沿私人direct-go-v1/rust-player-plan-v1.json。Go移植沿本repo GPL-3.0-or-later；原卡硬體沿§8，必要狀態與原音沿§11。推論等級為已證實的現行Go契約，不外推實機時序。

Rust input::Press保持FIELD:BTN[+BTN][:DUR]、預設5場、穩定依起場排序及重疊按鍵聯集。BTN名稱、大小寫、空／減號、空白與錯誤解析沿原Go；舊來源忽略第三個冒號以後內容、At加Dur以u64環繞，均明列保留，不藉移植改格式。Parse／format／at以固定Go案例及完整M4腳本差分，含零長度、邊界、非法數字與未知位元。來源input不改SIO協定。

命令列psgolem-machine-probe新增-press <文字>／-press-file <既有格式檔>、-fields <絕對VBlank數>、-card <128KiB原卡檔>，兩種press輸入互斥。腳本在每個field開始前更新1P Buttons，第一次含field0；CPU／MDEC／CD／SPU仍逐指令推進，不跳時間。-fields與-stop-pc互斥，原-cycles可明確作預算；只給-fields時預算為fields×1000000週期，checked overflow回錯，未抵達終點明列失敗。未指定fields保持原預設週期；原狀態JSON／RAM／VRAM與WAV格式保持，輸入由解析資料決定，不注入遊戲RAM／PC。

cardfile層與原卡硬體分離，僅當指定-card才讀寫磁碟。存在時長度必須131072，讀失敗／錯長回報且不覆寫；缺檔沿原格式空白卡。不dirty不寫；正常停止且dirty時，以同目錄create_new暫存檔、完整寫入／sync_all、rename更新原檔，成功後才清dirty。臨時檔名稱含程序號及單調序號，失敗只清自己建立的暫存檔、原檔與dirty保持。格式、玩家既有檔與現行Go設定不改。本CLI只在正常結束保存；GUI的60場安靜後自動存卡／視窗關閉等012能力仍待前端接線，不藉本CLI稱GUI完成。

WAV以場邊界取原SPU取樣，保持44100Hz／16-bit雙聲道；沒有HD音軌疊播或實體聲音裝置驗收。驗證先固定Go parser期望，涵蓋M4全部腳本及場邊界；再驗卡檔長度／無dirty不寫／合法覆寫／失敗保留與暫存回收。只用本批新建測試卡或只讀載入既有資料，不碰玩家卡片。

正常玩家切片使用同一M4腳本，由真BIOS／原CUE／空白新卡從reset開始至field12000，與Go同場套鍵及完整RAM／VRAM／TTY／原卡／原音比對；一次目視確認序章實際畫面。按鍵只來自腳本，禁止PC／RAM或物品注入。卡片frame63初始化寫入只能稱原卡檔案鏈通過，第一存檔室／真正進度保存及重新開機讀取仍另驗，不能把檔案存在當遊戲存檔完成。

工具鏈沿§7.1既有Rust／Go有界Docker；輸出沿私人direct-go-v1，公開原始碼input.rs、cardfile.rs、bin/machine_probe.rs與tools/oracle_rust_input.go。PSGOLEM_RUST_INPUT_ORACLE指定固定案例，PSGOLEM_RUST_CARD_TEST_DIR指定本批卡片輸出根，沒有選資料就明列SKIP。圖像／音訊／記憶卡／完整原資料不入公開Git。本節不改CPU／GTE／GPU／MDEC演算法，不重跑其已驗矩陣，不由重播耗時推測FPS。


§12首次結果：51個Go parser案例、1680個場邊界及1024個format mask相同，包含M4全部腳本；原卡新建／無dirty不寫／合法覆寫／錯長度／失敗保留／暫存回收通過，五個CLI互斥／格式／預算錯誤界線通過。沒有更動CPU／GTE／GPU／MDEC或其時序、沒有重跑舊矩陣。

真BIOS／原CUE／新卡從reset重播既有M4腳本至field12000，Go與Rust均6853150729週期、PC8001548C。完整2MiB RAM、1MiB VRAM、TTY、原卡131072 bytes、原音WAV與實際RGBA逐位元組相同。WAV為44100Hz／16-bit雙聲道8923373組；卡片有1次原協定寫入，Rust正常結束原子保存並可由原格式讀回。一次目視第12000場，確認里希特已走進序章城內。這是原卡檔案鏈，不稱第一存檔室或真正進度存讀檔完成。

命令列入口例：依§7.1在有界Docker中建置psgolem-machine-probe，從既有輸入唯讀掛載執行-bios /bios/SCPH5500.BIN -cue /disc/sotn.cue -press-file /engine/apps/sotn/testdata/m4-first-save.press -fields 12000 -card /output/new-card.mcd -wav /output/replay.wav -state /output/replay。新卡／輸出根須可寫且不能指向玩家原檔作測試。-press／-fields是一般輸入與真時鐘，沒有RAM／PC注入；-state仍是診斷輸出，不是正式存檔格式。

固定來源與首次結果沿私人direct-go-v1/rust-player-plan-v1.json、rust-player-execution-v1.json、rust-player-normal-execution-v1.json及rust-player-summary-v1.json。素材工具record_rust_player.py只核對已保存結果與取最後3秒原音，入口docs/assets-catalog.md#render-control。正式玩家GUI／實體聲音與USB／已配對藍芽設定、第一存檔室存讀、完整HD接管／GPU／海報與三平台Rust雙擊包仍待接。整段AI影片與HD自動替換沿027仍未完成，不能把原生序章或1080p放大樣片稱HD完成。


## 13. Rust正常首存與重新開機讀檔

狀態：CONFORMED，限015 §7既有M4路線的Rust正常首存及010／012原卡重新開機。承接§12已驗的CLI／輸入／原卡，這輪不改核心、畫質或格式。來源936f286的Rust binary與m4-first-save.press；私人素材tools/prepare_rust_save.py固定SHA及已驗Go基準，入口docs/assets-catalog.md#render-control。

已證實的Go基準為workplace/final/linux.mcd，SHA256 5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36，131072 bytes；linux.log記錄正常field45784、26150980006週期及摘要6c35c2701228722f，原M4測試通過。linux_end.png SHA256 d0dc2e0be51124a3e595589180d39089883243ae1bacb82ac36d0797d8b26627。015 §7記錄同卡在Mednafen1.29正常讀到存檔室HP70；只在Rust卡逐位元組相同時承接這項舊交叉證據，不宣稱本批新跑Mednafen或整機同態。

Rust從真BIOS／原CUE／本批新空卡reset開始，只用原m4-first-save.press至field45784，經序章戰鬥、阿魯卡德入城與第一存檔室，不側載、不注入RAM／PC／座標／物品。一次完整Rust重播即可；已驗Go首存不重跑。終點需原卡目錄0x80項為51、檔名BISLPM-86023DRAX00，卡片完整bytes同已驗Go，並一次目視實際存檔室；保存成功由完整卡片基準一致及重新開機讀回共同證明，不假稱固定終點截圖仍有保存完畢文字。原生終點RGBA同Go PNG則列像素相同；若只卡片相同而畫面不同，保留差異／推論等級，不由卡片推論CPU／RAM同態。

之後以新產生的卡分別複製成本批Rust可寫測試副本及Go唯讀輸入，各從真BIOS／原CUE重新reset。沿既有stone-reload.go已驗讀檔按鍵4500:START:5,4700:START:5,4900:START:5,5650:CIRCLE:5,5900:CIRCLE:5,6200:CIRCLE:5,6500:CIRCLE:5,8200:LEFT:120至field9000，確認存檔室與HP70。Go採ExactTiming，與Rust比完整RAM／VRAM／TTY／原卡、RGBA及原音WAV，沒有用舊RAM重建機器或補PC。遊戲位址若需觀測，僅在私人素材探針依015現有證據解讀，不放通用核心。

Rust CLI仍用§12，Go讀檔參考tools/oracle_rust_saved_game.go只做正常輸入／觀測。外層Docker首存與讀檔分段有界，UID／GID1000、原版與基準唯讀、明確輸出根；本批新卡與副本不覆寫玩家卡。首存WAV只驗取樣／長度與非靜音，不稱喇叭或人工聆聽。卡片與原生素材只留私人direct-go-v1。

通過只表示正常無頭首存／重新開機讀檔切片完成。玩家GUI、實體聲音與USB／藍芽、HD接管／GPU／海報、整段AI影片與三平台Rust雙擊包仍待接。CPU／GTE／GPU／MDEC舊矩陣及已核准美術直接承接，不因這個較長玩家切片重跑或調低畫質，也不把執行耗時換算FPS。


§13首次結果：唯一一次Rust原M4腳本從reset至field45784，26150980006週期、PC800154CC，與既有Go首存紀錄相同。正常原協定成功寫入66個frame，原卡131072 bytes與已驗Go逐位元組相同，SHA256 5b86c3b3402064ca2e7fd551ce111968c3b54b7ae258278d35250969af5b0a36；有效第一目錄51、檔名BISLPM-86023DRAX00。原生RGBA同既有Go PNG解碼像素，SHA256 02b9bc2db161fa5675d867aeec057ccebd5cb9f6912db0fe46ed159d78c8ddd7。一次目視阿魯卡德在存檔台。固定終點文字已消失，未宣稱圖中看見保存完畢，也未為等文字另跑遊戲。首存WAV為44100Hz／16-bit雙聲道34050755組，長度等於CPU週期整除768。

新卡分別給Go及Rust各一次新BIOS／光碟開機、正常選單讀檔／左移至field9000，均5139514453週期、PC8001548C。完整2MiB RAM／1MiB VRAM、TTY、26768348-byte原音WAV、131072-byte原卡及RGBA逐位元組相同。原卡保持，重開原協定寫入1次但內容相同，不另存。HP70沿015已證實RAM位址0x97BA0核對；一次目視同一存檔室的阿魯卡德已向左移。重開WAV6692076組。卡片同已驗基準，因此承接015 §7舊Mednafen讀取相容證據，這輪沒有新跑Mednafen或完整Go首存。

聲音限制：完整首存WAV60274059個非零左右聲道取樣，重開WAV4927984個非零取樣，但重開最後3秒Go／Rust均全零。接納腳本原先額外假設尾段應非靜音，首次停止後只分析既有PCM，未重跑遊戲；移除未有原版證據的尾段假設，同時把存檔室BGM列為未知／待成熟模擬器核對，不能把Go差分當音樂正常。預覽取首存路線最後3秒並明確標示，沒有拿別段聲音補入重開或遊戲。rust-save-audio-audit-v1.json保留全程每秒統計。

這輪沒有Rust核心變更或重跑舊CPU／GTE／GPU／MDEC矩陣；只增加通用Go正常讀檔觀測工具、驗收與文件。來源、命令及首次結果沿私人direct-go-v1/rust-save-plan-v1.json、rust-save-execution-v1.json、rust-save-comparison-v1.json、rust-save-summary-v1.json。prepare_rust_save.py／run_rust_save.py／record_rust_save.py由素材docs/assets-catalog.md#render-control進入。兩張已驗原生截圖只依既有授權加入私人素材docs/screenshots與README，公開引擎沒有圖片或原資料。

完成聲明限正常無頭首存／重開讀檔M4切片。玩家GUI、實體音訊／USB／藍芽、全部HD／GPU／海報、整段AI影片與三平台Rust雙擊仍未交付；全遊戲通關及速度未驗。本輪短HD影片原型另見027 §7，其建築變形未通過，不能混稱HD已完成。

## 14. Rust原版玩家視窗

狀態：CONFORMED，限Linux原版視窗功能／Xvfb與dummy音訊驗證。這是026既有Rust核心的原版視窗切片，沿012的顯示、輸入、時鐘與原卡契約；不宣稱Rust完整HD或取代現行Go雙擊包。來源是本專案6e66bd5的machine、input、cardfile與frontend/ebiten/input.go。平台繫結只用rust-sdl2 0.38.0的SDL2介面，不搬其他模擬器核心。

入口rust/player/Cargo.toml，獨立crate依賴../的psgolem-core，使原核心保持std-only及forbid(unsafe_code)。新增相依固定版本並保存Cargo.lock：sdl2 0.38.0（MIT）、serde_json 1.0.145（MIT／Apache-2.0）、png 0.17.16（MIT／Apache-2.0）、sha2 0.10.9（MIT／Apache-2.0）；SDL2 2.26.5為zlib。中文字使用frontend/ebiten/ui-font.png／json與同目錄OFL，不新增字型設計。來源：https://docs.rs/sdl2/0.38.0/sdl2/ 、https://wiki.libsdl.org/SDL2/SDL_QueueAudio 。只用原生視窗／輸入／音訊API，依賴授權與鎖檔另列於本節結果。

命令列psgolem-player -native -bios FILE -cue FILE [-card FILE] [-resolution 1920x1080] [-mute] [-present auto|cpu|gpu]。原版輸出固定4:3置中，不以PS1儲存寬高當螢幕比例。present只選SDL呈現裝置，原生PS1仍由已驗Rust軟體GPU計算，不能稱HD GPU或硬體影片解碼。這個分段入口必須明確-native，避免把尚未遷移的HD偏好靜默降為原版；不寫現有Go設定，也不覆蓋local-full。缺原版資料即停止，不自造輸入。

按鍵與012相同：方向鍵、Z/X/A/S、Q/W/1/3、Enter／Backspace。USB與已配對藍芽標準SDL手把從已連線裝置選第一支；熱插拔後重新列舉，十字／左類比與標準實體位置映射PS按鍵，觸發器對L2／R2。左類比預設死區0.2，鍵盤與手把OR。失焦釋放按鍵，F1暫停／續播，Esc正常關窗。這輪只驗標準映射，完整020裝置選擇／重設／保存面板仍待遷移，不把SDL虛擬手把當實體藍芽驗收。

主執行緒持有SDL。先呈現第一張黑畫面並開啟44100Hz、雙聲道i16音訊，再啟動唯一機器worker。機器只在worker改動，每場開始取一次輸入，每次執行到真VBlank。單場上限兩百萬CPU週期防止GPU停場無界。worker發布至多一張最新完整畫面；GUI可漏掉呈現，機器不能跳場。最終場一定真正繪製後才能自動結束。

CPU主時鐘33868800Hz，步調依週期除頻，不改模擬週期；落後100ms重設實時基準，暫停後重設基準。音訊只取原SPU，不另播影片AAC。callback以有界FIFO取樣，不足補零並計數。FIFO最多22050組立體聲，滿時worker等待並可被stop打斷，不能無界累積或丟原取樣。暫停在場邊界確認，停止步進與新取樣提交，再暫停輸出；續播保留已排取樣。靜音不等音訊裝置。裝置失敗回報後靜音沿012，不能將dummy後端稱人耳驗收。

原卡沿cardfile原子保存；定期在120場無新增原協定寫入後保存，及任何正常／錯誤關窗時收尾保存。stop可打斷等待並join worker。錯誤須顯示於stderr並非零退出；保存失敗不能回報成功。按鍵／GUI操作均不改遊戲RAM或PC。測試使用獨立卡片，不能覆寫玩家卡。

診斷-fields N、-press／-press-file、-report PREFIX及-screenshot FILE只服務正常重播。腳本依既有input解析、與即時按鍵互斥；每個field仍正常執行。report保存完整最終RAM／VRAM摘要、原PCM串流摘要、卡片摘要、週期／PC／field與呈現／音訊計數，原圖僅留私人素材工作區。-unpaced僅供靜音診斷且需明確-fields，不能拿它推即時FPS。

集中驗證：平台無關鍵位、類比邊界與4:3排版；一次Xvfb／dummy音訊正常重開讀檔至9000場，使用既有新卡副本及已驗重開腳本，對照026 §13的完整RAM／VRAM／原音／卡／原像素。視窗最後場必須呈現，音訊須實際交callback而非只生WAV。另以同一場內受控F1暫停／續播與正常WM關窗驗證worker／原卡生命週期。沒有更動核心，不重跑CPU／GTE／GPU舊矩陣；不稱實體音訊、實體手把、全遊戲速度、HD／海報或三平台Rust包完成。


§14首次結果：三個新平台無關案例通過，鍵位14項、chord、類比邊界、4:3排版與既有中文字圖集可讀。唯一一次正常視窗讀檔至9000場、5139514453週期、PC8001548C，完整RAM／VRAM／卡片／原像素與§13已驗Go／Rust完全相同；13384152個i16原取樣的SHA256亦同舊WAV資料區。GUI最後實際呈現9000場，1920×1080畫面一次目視確認阿魯卡德存檔室及4:3比例。

實際X11鍵盤事件在7214場、4119329659週期完成暫停；39次GUI呈現檢查中場數、CPU週期與音訊callback已消費數均不變，續播後仍達相同完整端點。dummy音訊裝置確實取走13384152個原i16取樣，其中4927984個非零；FIFO峰值10266個i16，上限44100。獨立短重開由WM_DELETE_WINDOW正常關閉，worker已join且原卡保存成功，兩張測試卡與來源卡保持相同。原版資料、PC及RAM均未注入，沒有重跑舊核心矩陣。

速度限制：本次1920×1080軟體SDL／Xvfb單次功能重播耗時234.390秒，包含開機與暫停，callback補零7032360個i16。聲音資料與傳送鏈通過，連續播放品質／全速未通過；這不是實體硬體、穩定遊玩FPS或Go／Rust同環境效能比較。效能成本沿既有Issue #9列管，不因資料摘要相同而稱聲音或速度完整驗收。

現有可重現工具為本機Docker image dosgolem-rust-san5-gui-tools:1.99.1-r3，固定image ID a5b26f4bd035234bbcaab0e12c3081bb70c4833f1b1c4983d06fbcea98eb7ad8；Rust／Cargo1.99.1、SDL2 2.26.5、Python3、Xvfb／xdotool。沿§7.1的有界Docker契約，工作樹掛/src、明確Cargo快取／target掛可寫/output，其餘原版資料唯讀；先在明確network bridge容器執行cargo fetch --manifest-path /src/rust/player/Cargo.toml --locked，之後network none執行cargo test及cargo build --release，兩者皆帶相同--manifest-path、--locked --offline。這個獨立crate不改核心Cargo.toml或Cargo.lock；正式三平台工具鏈及Rust發行包仍另做。

Cargo.lock固定37個package含本機核心／前端；取得的相依授權已逐一核對，皆MIT／Apache-2.0、zlib、0BSD、BSL-1.0、Unicode-3.0或其相容選項。來源／版本／checksums由rust/player/Cargo.lock定位；SDL2來源由sdl2-sys 0.38.0的vendor與MIT／zlib條款定位；UI字型OFL全文仍在frontend/ebiten/ui-font-OFL.txt，後續打包須附上，不因include_bytes嵌入而省略。現行Go包不變。

私人驗證入口tools/verify_rust_window.py，從素材docs/assets-catalog.md#render-control進入，依序prepare、run、accept；使用/engine、/materials、/baseline、/build唯讀及唯一/output可寫。完整命令、固定來源與首次結果存rust-window-v1/{plan,execution,acceptance}.json及reload／close報告，accept只讀既有結果不重新開遊戲。HD影片Session、A／B／C海報、原版／HD切換、完整020設定與實體手把、GPU影片及三平台Rust雙擊包仍未接到本入口。


§14接續解析：027 §15已限定CONFORMED，原入口另支援-sotn-hd-movies DIR並與-native互斥；LOGO／OPN的Session已接到正式Rust視窗。§14既有原版功能與限制保持，海報／完整手把設定、實體聲音與三平台Rust交付仍待完成，不能沿較早的「HD影片Session未接入」重新開啟本段已完成工作。


§14海報接續：024 §6已限定CONFORMED，原版／HD影片共用外部海報及同schema設定。F1已回到使用者確認的工具列顯示，舊原型直接暫停改為F2／工具列按鈕；凍結機制保持，兩條新正常路線及獨立重開已驗。較早「海報未接入」由024 §6解析，完整手把／平台交付仍待接。


## 15. Rust HD 素材逐值相容性

狀態：`CONFORMED`，只涵蓋離線診斷。正式遊戲的素材載入、繪圖替換與全部 HD 仍未完成。

既有契約為 `019-hd-texture-pack.md` §2–4、§8.3–8.4，來源基準
4778ac7d88e3873ccc5ead50ebcd8cf53a621854 的 `render/hd/texture_pack.go`、
`texture_tiles.go`、`texture_tracking.go`。上述格式及 Go 解碼結果已證實；
Rust 的逐值結果以下列本段收據為準，不由舊 Go 測試推定通過。位址空間為包內路徑與解碼貼素座標，
不涉及客體 RAM 或遊戲位址。原版／HD 素材及輸出只留私人素材專案。

- 在既有 Rust 玩家程式新增明確的 `-check-textures DIR` 診斷入口，不啟動 SDL、Machine、音訊或 FFmpeg，不讀寫玩家設定與記憶卡。
- 沿用 019 v1 欄位、大小寫、重複 JSON／null／未知欄位拒絕、重複素材鍵、同上傳鍵尺寸一致與包內路徑檢查。只讀一般檔案，符號連結解析後必須留在包內。manifest 4 MiB／8192 條，PNG 64 MiB／16 Mi 貼素及原尺寸／倍率界線保持。
- 每次只解碼一個條目的圖像與 STP；不建立整包常駐 TexturePack，不把串流診斷當作 019 的 512 MiB 常駐容量驗收。先核對 PNG 尺寸，再分配解碼緩衝。
- indexed 保留原索引，包含 1／2／4／8 位 PNG，不由預覽顏色反查；color 轉未預乘 RGBA，依 019 的 alpha／黑白 STP／不透明零字守門產生原值 LE16。每條輸出完整解碼資料 SHA-256、尺寸與貼素數。
- sprite-runs.json 若存在，明確回報「未由本診斷驗證」，不忽略後宣稱整包可渲染。玩家模式不接這個部分載入結果，中文字／姓名與既有素材保持。
- 全部圖像成功才輸出 JSON。晚期壞檔不輸出部分成功報告，錯誤含條目與路徑。此診斷不生成或修改圖片，也不宣稱逐圖藝術審查。

驗收：公開合成 PNG 驗證索引同色異值、低位深索引、RGB／RGBA／16 位通道、透明與 STP、錯誤 alpha／黑字／遮罩、路徑、JSON、尺寸及晚期失敗。
私人現行包完整條目只執行一次 Rust 解碼，與既有 Go 正式載入器的完整逐值雜湊比較；
保留來源／manifest／執行檔雜湊與工具版本，不重跑遊戲或已核准美術。失敗如實記錄，先查原因，不改圖或放寬三種透明語意。
這一段通過只證明圖像資料相容；全部渲染、GPU、PGXP、正常玩家路線、速度與三平台 Rust 包仍須另驗。


§15 結果：七項合成案例首次通過；索引 PNG 的合成調色盤修為符合各自位深後，只補測該一項並通過。另依固定 Go 1.26.7 的 image/color/color.go 補齊 16 位 NRGBA 的整數往返，alpha=0xFF00 的 0x0800／0xF800 通道依 Go 為 7／247，再作 RGB555 量化；獨立 Go 四值及單項 Rust 合成檢查相同。原本直接 STRIP_16 的來源保留，不稱原16位實作全部相容。現行10719張不同引用圖無16位來源，新增守門後只補一次最終 Rust 完整讀取，Go 完整參考不重跑。

Rust 明確 CLI 完整讀取現行 7387 條／1136 上傳鍵，1651519296 貼素、3295497856 個累計解碼 bytes 的逐條雜湊與 Go 正式 LoadTexturePack 全部相同；該累計值不是常駐量或素材磁碟體積。缺路徑與多餘參數兩個 CLI 守門通過，無 SDL 環境下可用，來源與素材不改，遊戲／美術審查皆 0。

首次 Go 參考程序於 176.876 秒回報 signal: killed，原始原因未知，沒有保留 cgroup 收據，不宣稱已證實 OOM。保留首份日誌後，僅接續未完成參考：同一 3 GiB 容器配置加入 GOMEMLIMIT=1400MiB 與 cgroup 觀測後完成。沒有重跑成功的 Rust 完整解碼、已核准美術或遊戲。Rust 不驗 sprite-runs、不建立常駐包，不由本段推定繪圖／GPU／正常路線、速度與全 HD 完成。來源與收據由私人素材專案 docs/assets-catalog.md 的 rust-textures-v1 入口追溯。


## 16. Rust 整段繪圖流程接管

狀態：方向已由使用者明確定案；實作 `DRAFT`。025 的既有 Go 圖層與幾何研究是來源證據，不是本節已完成的 Rust 主流程。§15 的圖像逐值核對不接遊戲繪圖，不當作本節完成。

2026-10-29 使用者再次明示整段攔截，以 Rust 直接實作並寫入 VRAM，並將未來 3D 繪圖納入。此決定更新舊 Go 語言限制，保留原版／HD、CPU／GPU、海報、素材與存檔。跳躍高度等是後續可選遊戲修改，不在本節默認改動玩法。 後續工作已登記於[素材專案#26](https://github.com/wicanr2/castlevinia_x/issues/26)。

### 16.1 主路徑與完成條件

- HD 從場景、圖層、動畫、人物、怪物、武器、物件、文字、HUD、特效及 3D 資料開始，由 Rust 建立型別化整場工作，完成剔除、排序、材質取樣、混色與目標寫入。支援路徑跳過整段原繪圖迴圈與 SDK 繪圖呼叫，不執行 MIPS 繪圖指令，也不以解碼原 GP0 封包作為 HD 主流程。
- Rust 直接擁有 HD 顯存／呈現目標，依 CPU／GPU 後端繪製並在完整場邊界發布。不得先將原版一整場光柵化，再放大、覆蓋或讀回後換圖；始終先畫完整一倍相容平面的雙畫法只屬過渡，不算本節完成。
- 原生 PS1 VRAM 與 HD 呈現目標分清。遊戲實際需要的來源上傳、清除、搬移、遮罩、快取、顯存讀回、CPU／RAM／scratch／返回與虛擬時間效果，由有證據的 Rust 資料／狀態流程保留。不能從 HD 美術縮圖反推原字，也不能為接管默默刪除效果。按需相容契約尚未證實前維持 DRAFT，不能以執行完整舊光柵化再稱整段接管完成。
- 原版模式維持原遊戲繪圖；HD 只在完整場邊界切換。未知或未支援分支如實列為未完成，研究不能以半場原版／半場 HD 冒充完成。既有 H.264 影片與必要原 MDEC／DMA 相容仍沿027，未證明替代效果前不刪除；這項過渡能力也不當作本節全流程完成。
- 3D 由 Rust 直接建幾何、投影、深度與材質資料，再交 CPU／GPU；承接已定案 HD 透視校正、原版模式及可靠深度不足時的既有取樣契約。不得只處理已量化的舊畫面。
- 同存檔、等價輸入與相同畫質／素材／海報核對玩家路線；關閉017及其他遊戲修改後核對原生狀態。計數至少包含原繪圖入口／指令、HD 主路徑 GP0 解碼、完整原版光柵場數、Rust 圖元、顯存讀回與完成等待；完成聲明要求前三種舊主流程計數為零，保留必要原版模式控制。
- 速度以相同輸出與工作量分項量測場景生成、相容效果、光柵化、同步及呈現，另列完整遊玩音訊與場時間。省去原 MIPS／SDK 模擬是可省成本的機制；尚無整段同條件數據，不預先宣稱倍率或穩定滿速。

### 16.2 本輪入口研究與停止線

原始來源為既有 DRA-map.i64、DRA.BIN 與 SLPM_860.23，檔案／腳本／工具 SHA-256 及 IDA9.4 位址空間記在私人 rust-flow-v1/plan.json。候選 PS1 KSEG0 sub_800E3880 已有呼叫0x800ECC10／0x800EB998的已證實交叉參照；它是否恰為純繪圖整段邊界仍是 unknown，不能只憑 caller 名稱全部跳過。

只在原 DB 一次性副本匯出完整候選、直接與間接呼叫、呼叫者及原 bytes。直接呼叫另由 DRA 原檔固定32位字解碼交叉核對，不移植原組合語言。以已保存場景、動畫及實際 I/O 證據標註已知支線，未知保持分級。需要補的副作用／分支未封閉前不接正式 Rust 入口，不重跑已核准美術與已通過遊戲路線。


§16 本輪靜態結果：IDA9.4一次性副本匯出 sub_800E3880，PS1 KSEG0 0x800E3880至0x800E3FA7，共1832 bytes，SHA `08f7ac668f1e5d491d2ef9a176c17d11760ce602a269a6dfe4f8acd659690da0`。原DRA逐個固定32位字交叉核對62個JAL、56個不同直接目標，全部同IDA xref。0x800E3FA0直接回跳0x800E3B80、範圍內JR RA為0，已證實控制流；整個外層屬持續主迴圈為strong inference，不能當成單一純繪圖函式直接返回。SDK名稱僅導覽，本輪未由名稱升格完整語意。

0x800E3C24／0x800E3C2C的兩個已驗圖層／物件呼叫保留，但它們只覆蓋外層的子集。沒有因此把完整場景交易邊界標READY。後續應沿整場、分模式資料與呼叫者可見效果接Rust，包含必要顯存搬移及回讀；不能只攔最後GP0，也不能跳過遊戲更新、計數或亂數。已保存原始bytes、operands、xref與固定字核對；原DB哈希保持。原版／HD正式程式未改，遊戲／美術／GPU／速度新驗收0。


### 16.3 整場緩衝與排序資源初始化

狀態：DRAFT研究。以既有外層資料流為依據，先釐清每次0x800E3B80至0x800E3BE4的緩衝選取、計數與排序表初始化；這是整場Rust交易的一個資料階段，不是完整繪圖接管。0x80012C90的SDK語意、DMA及時間效果須由IDA原始字與實際I/O證實，未確認前不依名稱猜補。

研究只開一條正常真BIOS／同卡／固定按鍵路線至8382，再自然觀測八次此階段，禁止PC／RAM／程式注入。保存完整前後RAM／scratch／CPU、實際CPU I/O及獨立clone的DMA／GPU暫存器。原機器不因診斷新增MMIO讀取；顯存只取獨立快照雜湊。原繪圖只在oracle執行，研究工具不裝入正式玩家路徑。

資料模式若有足夠證據，另將純Rust型別化輸入／寫入計畫設為READY，再實作並對保存八態核對完整RAM。CPU／時間／SDK／週邊／PGXP與完整場景必須另驗，不能以RAM相同當成主接管完成；不重驗舊美術、素材與GUI。輸入／工具雜湊、IDA9.4位址空間與結果留私人rust-flow-v1/frameprep-v1，由素材索引的prepare_sotn_frameprep.py／probe_sotn_frameprep.go追溯。


#### 16.3.1 純RAM資料計畫

狀態：CONFORMED，八個計數寫入邊界勘誤已驗；僅限型別化資料讀取、計畫與原子RAM套用；未接Machine或正式玩家入口。CPU端點／快取資料由16.3.2限定補齊；整段接管、裝置時間與週邊提交仍DRAFT。

已證實：正常路線8382至8389八次階段，每次5161原指令／7351原週期、I/O與中斷入口皆0；完整RAM可由下列資料規則重建，scratch、VRAM及擷取的IRQ／DMA／GPU暫存器保持。0x80012C90..0x80012D3B共172 bytes、SHA `86577716397f0014ba20a60992c84534f9b48ad5138b9421d6581f6620a4982e`，是CPU前向鏈ClearOTag，非反向OT或DMA6。不得因排序表名稱猜成DMA。SDK debug>=2的間接輸出分支未納入，須拒絕。

資料契約：讀取恰2MiB RAM與入口PC／SP／S0／S1。PC須0x800E3B80、S1須0x8009792C以符合原計數區基底，原外層及ClearOTag原始碼SHA須符合、debug位於0或1。原緩衝p來自0x8006C37C，下一緩衝q來自p首字，排序起點為q+0x474，共512個32位字。所有來源／輸出地址須為對齊且有界的KSEG0 RAM，不允許輸出區域互疊或碰到程式、debug及p來源欄位。守門不相符回錯，不輸出部分RAM。

計畫直接產生：0x80138364=p、0x8006C37C=q、0x8003C998以u32繞回加1、0x8013556C=q+0x474；512個前向連結為下一字地址的低24位、末字0x00FFFFFF；0x8009792C起偏移0／32／4／8／12／16／20／24的八個計數清0，偏移28保持；原可見的三個堆疊字在SP-16／SP-12／SP-8分別保存入口S0／S1／固定原呼叫返回0x800E3BC4。這些是資料／ABI效果，Rust不執行MIPS、SDK或GP0。

套用前重新核對程式與全部輸入讀集合、入口暫存器，拒絕過期計畫；驗證完成才寫入527個字，失敗不改任何RAM。圖像／顯存／CPU／時鐘／裝置不在此API內，不用資料模式冒充完整呼叫等價或省去正式原流程。

實作位置為Rust玩家apps/sotn/frame.rs，先作已驗資料元件，不在正式玩家宣告此模組。由獨立frameprep整合測試載入來源。公開測試只測型別資料的界線、別名拒絕、32位繞回與原子拒絕；原版語料測試預設ignored，缺原始輸入不自製替代物。私人八態由已存capture一次集中核對完整2MiB RAM及過期／程式／入口拒絕，輸入不改、不重播遊戲。完整Renderer／CPU／時間／正常HD仍另驗。


§16.3.1結果：Rust直接產生528個字，八態完整16777216 bytes RAM全部同原版；每態四個過期／程式／入口拒絕共32個，失敗未改任何RAM。五個公開界線／別名／繞回／原子守門通過；原版語料預設ignored，另明確載入保存八態首次通過。後續審查確認原計數清零使用S1相對地址，先由八態S1=0x8009792C回填READY，再加入嚴格入口值檢查；最終來源補測改動守門與八態、共用輸入改變的五個公開案例皆過。保留初版來源與收據，沒有重開遊戲、重繪、CPU矩陣或第三版重測。

前述16.3較早的SDK／DMA未知已由本段回填：所選非診斷分支為CPU前向排序鏈，八例I/O0且DMA／GPU等所取暫存器保持。本段的CPU端點／指令快取未知已由16.3.2限定解開；未來中斷與完整裝置提交仍未閉合，計畫只能產生無中斷參考分支的資料，不得直接當成Machine呼叫替換。整場正式接管仍DRAFT。原場景主繪圖、素材、海報、輸入及玩家程式保持，尚無新畫面或速度驗收。


#### 16.3.2 CPU端點與快取計畫

狀態：CONFORMED，僅限無中斷的資源初始化CPU端點、指令快取與週期資料計畫；裝置時鐘與Machine提交由16.3.3限定補齊；整段HD與正式玩家仍DRAFT。承接16.3.1的RAM契約，不取代遊戲更新、整場繪製或原版模式。來源是frameprep-v1保存的八份正常CPU／RAM前後態及其完整雜湊，IDA9.4的外層1832 bytes／ClearOTag172 bytes與16.2–16.3.1的原始地址／SHA保持。分析器tools/study_sotn_framecpu.py只用原32位字核對讀取與指令計數，非正式解碼器；結果framecpu-v1/evidence.json索引於私人素材assets-catalog的render-control。未引入外部程式碼，CPU成本契約沿自有rust/src/cpu.rs與008-cycle-model.md。

已證實：八態完整CPU欄位可由RAM資源、入口CPU和快取公式重建，包含35個GPR槽、HI／LO、PC／nextPC、分支及延遲載入、COP0、now／cost、乘除／GTE完成時間及256項快取。所選路徑基礎5161步，518個RAM讀取各4週期；原指令快取補入合計118週期，得到7351。這只證明該CPU端點；裝置內部、音訊、IRQ到達時間不在保存CPU相同的結論內。

型別輸入為恰2MiB RAM、cpu::State與BIU。先通過16.3.1所有來源／別名守門；入口PC=0x800E3B80、nextPC=PC+4、bdbt=0、branched=false、ld_which=34。拒絕快取關閉、COP0 SR隔離寫入、已可受理的IRQ、now加法溢位；快取valid只能低四位。這是明確的支援子集，不能用拒絕掩蓋正式HD缺支援。API保持未接正式玩家模組。

快取以四個來源區間的首次取用順序計算：PS1 KSEG0 [0x800E3B80,0x800E3BC4)、[0x80012C90,0x80012CB8)、[0x80012CD4,0x80012D3C)、[0x800E3BC4,0x800E3BE4)。只有快取行標籤與有效字的資源效果，不讀取或解碼指令；已載入的迴圈範圍不反覆計算。每個16-byte行依第一個尚未有效的需求字w補入後綴，費用7-w；其餘需求字命中為0。標籤、有效位與成本遵循008及既有Rust CPU。總成本為5161+518×4+快取補入，最後cost為1加0x800E3BE0的取用費用，不固定寫死7351。

CPU輸出由型別資源推導：tail=ordering_start+511×4；GPR1=0x80130000、2與4=tail、3=tail低24位、5=0x00FFFFFF、6=0xFF000000、31=0x800E3BC4；0=0、34及ld_value=入口S0。PC=0x800E3BE4、nextPC=0x800E3BE8，bdbt=0、branched=false、ld_which=34。now加總成本；未列GPR、HI／LO、COP0、muldiv_done、gte_done保持。GTE本體不在API內，來源中沒有GTE操作。這些是ABI及相容資源效果，不逐指令翻譯原遊戲。

計畫不操作Machine或裝置。套用前核對完整入口State、BIU與16.3.1的RAM讀集合；任何拒絕均不改RAM與CPU。檢查完成後才原子提交RAM及CPU端點。公開測試涵蓋不支援入口、隔離／IRQ／BIU與快取資料；時間溢位由私人八態驗證；原版語料測試預設ignored，缺原資料不得造替身。私人驗證只讀原保存八態，完整CPU／RAM同Go；再用Rust CPU oracle對冷／熱／稀疏快取的局部診斷抽測，同時核對完整CPU與RAM。局部診斷不是正常玩家HD接管或速度證據。驗證一次集中執行，失敗記錄原因再限定修正，不重開美術或既有正常路線。

停止線：Machine逐步推進裝置的事件順序、IRQ途中到達、DMA／MDEC／音訊／顯存副作用及正常Rust主流程仍未閉合。不得直接把總週期塞入一次bus.advance就稱原版等價，也不得因端點相同把此切片標為完整可玩HD。下一閘門是裝置事件與整場工作提交，不再重驗本段已過CPU資料。


16.3.1邊界勘誤：已證實0x800E3BC4至0x800E3BE0只清八個計數，偏移28的第九項由0x800E3BE8的下一個呼叫延遲槽才寫入，位於本階段結束0x800E3BE4之外。上輪九項／528字的計畫提前多寫此欄；八份正常快照該欄原本為0，故完整RAM相等未揭露此錯。舊來源／收據保留在framecpu-v1，不重寫歷史。修正資料契約為八項／527字，偏移28必須保持任何入口值；此修正先標READY，待含非零診斷案例的CPU／RAM原版局部oracle通過再回CONFORMED。其他RAM地址與CPU公式不變。這是未掛正式玩家路徑的資料邊界錯誤，不重跑正常路線或美術。


16.3.2集中結果：七個公開資料／守門案例通過，兩個原版語料案例預設ignored。另明確載入私人八態，原CPU所有欄位及完整RAM同Go；每態加入冷／稀疏／全熱快取及偏移28非零的局部Rust CPU oracle，共40例CPU與RAM精確、200次過期入口／RAM／BIU原子拒絕及八次時間溢位拒絕通過。普通例7351、全熱例7233週期，確認成本來自輸入快取，非固定7351。這些局部變體含診斷輸入修改，不能稱正常玩家HD。原GTE本體不由計畫接觸。

初版CPU計畫及32個快取案例已過後，資料足跡審查找到16.3.1提前清偏移28的錯誤；先保存舊來源／報告、訂正READY，再修為八計數／527字並加入非零案例，最終40例與受影響公開案例一次通過，無第三版重測。原保存八態與原版路線未重開，素材／美術／GUI／GPU／核心舊矩陣重驗0。由完整CPU／RAM對照不能推論裝置狀態或音訊相等；正式模組宣告／Machine接線、整場直接VRAM與速度仍未完成。16.3.1及16.3較早CPU／快取未知由本節限定回填，裝置時鐘／IRQ時序仍沿16.3.2停止線。


#### 16.3.3 無I/O資源階段的裝置時鐘提交

狀態：CONFORMED，限定無I/O、無活動DMA／MDEC等待、未跨場與IRQ的RAM／CPU資料階段。整段HD繪圖、其餘場景與玩家註冊仍DRAFT。來源沿16.3.1–16.3.2正常八態、IDA9.4原位址與雜湊；私人framecommit-v1/clock-evidence.json保存各成本足跡與來源SHA。通用Rust機器的step／SystemBus::advance及DMA活動條件是現有裝置契約，未引入外部程式碼或硬體exact聲明。

已證實的資料相依：本階段只有RAM讀寫與指令快取，不直接讀寫MMIO；無DMA時，時鐘推進的GPU掃描／timer／CD與SPU混音／MDEC／SIO不消費本階段RAM寫入。故可先按原時間足跡推進裝置，再一次套用已驗RAM與CPU端點。DMA活動或MDEC等待不在此契約，不能猜補相容效果。

遊戲層直接建立ClockPlan，使用固定資料工作量與快取補入，不讀取／解碼MIPS。時間工作分為17格緩衝準備、10格SDK入口、6格鏈準備、511組10格連結、10格尾端及8格計數；RAM讀取位置分別為準備格1／3／6、SDK入口格1、每組連結格3與尾端格4／5／6，每次讀取加4。快取按既有來源足跡推導；只有首組連結需求填入快取，其後510組為命中。總5161個1至32週期的量子，總和／末量子／快取須與16.3.2端點一致。這是相容時間資源，不逐指令翻譯遊戲，也不重播GP0。私人離線oracle須逐項核對完整成本向量，不能只比總和。

通用入口放rust/src/machine/phase.rs，不含月下位址。輸入為CPU端點State、對齊有界RAM字patch、成本量子及呼叫者週期預算。先檢查量子非空且最多100000筆、每筆1至32、總和不溢位及不超預算；CPU now與Machine cycles須一致，端點now=目前+總和、cost=末量子、PC／nextPC為有界RAM且對齊／順序正確，沒有待處理load或branch。COP0、HI／LO、GTE與乘除完成期限保持，GPR0為0、快取valid僅低4位。字patch不得超界、未對齊或重複。初始未處理IRQ、DMA活動及MDEC等待拒絕；這是受限資料提交API，不是任意savestate匯入。

先clone候選Machine，按每個成本量子依現有advance推進裝置，保留GPU掃描、timer、CD／SPU、MDEC、SIO與IRQ鎖存的順序。每次更新cycles，若field改變、IRQ可受理、DMA活動或MDEC進入等待即整筆拒絕。原Machine未寫入，包含RAM／scratch、完整音訊緩衝與所有裝置保持。成功才在候選寫入RAM／CPU端點並移交給原Machine；CPU的GTE本體保持。副本成本暫不優化，不稱已達速度目標。

遊戲ClockPlan提交前再次核對完整原入口CPU／BIU及RAM讀集合，沿16.3.1來源身分與別名守門；只有通過才交通用API。元件先由獨立測試與研究程式載入，不在正式玩家開啟尚未完整的HD模式。原版、CPU／GPU、海報與已定案素材不變。

驗證：公開合成案例只證明通用API的成功、預算／量子／patch／端點拒絕與IRQ／跨場拒絕。私人保存八態核對完整成本向量；正常真BIOS／原片／同卡／等價按鍵只跑一條Rust路線，自然抵達8382，再讓原指令控制組與Rust資料提交組連續比較八次階段及下一場。完整機器以一次性觀測覆蓋逐欄比較CPU／GTE、RAM／scratch、GPU含快取／掃描、timer／DMA、CD／SPU／MDEC／SIO、原卡、音訊緩衝與時鐘；只對不可變光碟採同一Arc身分，不比較共享檔案游標。覆蓋僅增加比較能力，不改正式執行邏輯。來源／輸入SHA、工具版本及初次失敗保留。不重驗已過美術、GUI、GPU或舊核心矩陣。

停止線：通過只代表初始化整段的CPU／RAM／裝置提交，未完成場景／動畫／文字／3D與直接VRAM，也未證明正式玩家HD或速度。跨IRQ／field、活動DMA與其餘例外保持未支援；未補來源與相容效果前不得接完整HD主路徑。


16.3.3集中結果：五個通用合成提交／拒絕案例與七個既有資料案例通過，兩個原版語料預設ignored。新增ClockPlan由普通獨立測試編譯，另以研究程式執行正常路線；原已過案例沒有因時限修正而重跑。保存八態的成本向量及新正常原指令各5161量子逐項同Rust計畫；八次資料提交的完整Machine逐欄相同，包含GPU掃描／VRAM／快取、全部CD／SPU／MDEC／SIO、CPU／GTE、RAM／scratch、timer／DMA、原卡及音訊緩衝。續跑至8390場、4791075078週期仍完整相同，取出的原音序列SHA相同。各階段完整RAM亦同既有Go八態。

初始化階段本身不執行原MIPS／SDK或GP0，八次提交對應原參考共41288指令。這只證明初始化，其他場景／人物／文字／3D及原繪圖仍未全接管；不能用這個局部零計數稱整個HD主流程完成。正常GUI、硬體GPU、三平台與速度未驗，現行玩家未註冊FrameClockPlan。

另在獨立診斷副本驗IRQ途中到達、跨field及過期CPU三種拒絕，完整Machine皆保持。副本試行採相同不可變Disc Arc，完整比較排除共享檔案游標；每次原讀碟自行seek且原媒體SHA前後保持。比較覆蓋僅加入衍生逐欄相等，不改正式runtime邏輯；完整輸入及覆蓋原碼SHA留私人收據。clone的配置／複製成本尚未調校，不承諾速度。

首次正常路線180秒逾時，最後4800場、接管比較0；保留normal-initial-timeout.log及execution-initial-timeout.json。只將尚未完成的正常項目放寬至600秒，沿同一binary／來源完成一次正常路線；實際294.918秒是蒐證耗時，不是FPS。公開測試／建置已過後不重跑，沒有第三次正常啟動。前段兩次文件錨點未匹配是編輯準備問題，讀現行索引後修正，沒有因此開遊戲或修改產品行為。

回填16.3.2停止線：其無I/O初始化的裝置時間／Machine提交未知已由本節解開。跨IRQ／field與活動DMA／MDEC、完整場景與顯存工作仍未支援，不能用本API省略必要I/O或強接整段HD。下一段沿16.1直接場景／材質／顯存生成，承接已驗初始化，不重驗舊美術、CPU或正常資料。


### 16.4 Rust直接場景與HD材質畫布

狀態：CONFORMED，僅限保存NO3房15普通前景及kind1背景的資料／CPU繪圖元件；正式玩家接線、其他場景、角色／文字／3D及GPU後端仍DRAFT。承接025 §12.3–12.4及§12.17的原始資料／IDA9.4地址與雜湊，不重開舊美術或遊戲。來源為原RAM／VRAM、F_NO3.BIN、獨立原圖層PNG及已驗Go直接合成，完整SHA由私人scene-v1/scene-evidence.json保存，入口為素材assets-catalog的render-control。所有程式為自有GPL-3.0-or-later來源的Rust高階資料重寫，Go來源8fbc874；原ASM只作證據。

遊戲層從2MiB RAM／1024×512個原VRAM字及SHA=a0bb78a1260eee5e3bda4bae6cfbbd0a61ddd5604905f9fd6ef833fd03319c66的F_NO3讀取原資料。NO3 header SHA=0a19676621ba9dfaad1a39a2ab6ead6fd0438fad85a57c91be011b4f9fe0cd82；只接0x800730D8背景及0x80073084前景，flags=3、kind=1、排序32／96、相機偏移與倒數0。資料／定義／三個表須在對齊有界場景RAM，ID<4096、page<=7。所有未知配置回錯，不部分繪製或偷偷沿原MIPS。

以16格整除向下取整選可見格，原順序背景後前景；不改鏡頭或角色大小。圖塊頁／UV／色盤直接由資料表取值，核對原顯存中的16×16來源字與16色完整RGB555／STP，再建立型別材質與Sprite。來源鍵為32×128字上傳尺寸及8192原bytes的SHA，CLUT為32個色盤bytes的SHA。每材質128×128貼素，圖塊16×16；同來源／色盤去重。正常256寬185＋109格、擴大342寬188＋154格，皆19材質；擴大畫布僅保存態研究，不聲稱017正常寬視野已接。

通用核心scene資料與繪圖不認遊戲位址、Machine、CPU或GP0。先驗全部畫布／材質／圖塊與圖像形狀，再分配目標，失敗不回部分畫面。直接按順序在RGB555／STP畫布繪製，word=0透明、非0整字覆蓋，包含0x8000不透明黑；此切片原圖塊為不混色raw sprite，不自行加入混色。支援原生1倍與HD4，保留原比例及所有來源位元；不得先畫原版畫面再覆蓋HD。未提供HD圖像才明確畫原生，HD要求的任一材質缺少或形狀不符整場拒絕。

沿既有manifest v1及019解碼，Rust新增場景選定圖像helper，共用原嚴格JSON、PNG、色彩與STP邏輯。全manifest結構、metadata／重複鍵／尺寸須有效；只解碼本場需要的4bpp color精確CLUT或indexed後備，color優先。最多512組請求，實際選定圖像資料計帳<=512MiB；保留單圖／單檔／manifest界線。此helper不是正式完整TexturePack載入或全包PNG驗證，不改Go完整Load、格式、快取策略或玩家設定。圖片先全準備，再交畫布，不在畫到一半載入。原圖不改，記錄選定圖片SHA。

一次集中驗證：通用合成案例驗透明／不透明黑、順序／裁邊／來源與圖像拒絕；既有七項圖像單元測試只因共用解碼重構執行一次，不重跑7387條全包核對。私人僅讀保存資料，Rust全部型別圖塊與獨立分析相同，原生RGBA同獨立原PNG及既有Go合成，HD4同既有Go完整PNG；另以現行Go官方LoadTexturePack對19組同格式研究子集及既有直接繪圖產生完整RGB555／STP oracle，核對所有色字。Go只讀同一保存場景，不啟動Machine、原routine或GPU，不重驗舊美術／GPU矩陣。研究子集僅測試，不加入正式包或發行。來源／圖片／原資料唯讀、輸出獨立；不把靜態畫布稱正常遊戲接管、1080p GUI或速度通過。


16.4集中結果：三個通用畫布案例與八個圖像案例通過，原版語料案例預設ignored。保存態兩個畫布的全部型別圖塊、UV、材質／色盤鍵及位置同獨立分析；正常寬度294格、擴大畫布342格，各19材質。原生兩圖完整143520像素同獨立前景／背景PNG合成及既有Go合成。HD4共2296320個RGB555／STP色字全部同本輪Go官方載入／直接繪圖，也與既有Go完整RGBA PNG相同，輸出1024×960及1368×960。六個原始來源／旗標／指標／維度／顯存拒絕案例通過，原資料及圖片SHA保持。

實作為rust/src/scene.rs通用CPU畫布、rust/player/src/apps/sotn/scene.rs場景資料、texture_audit.rs共用選定解碼與tests/scene.rs。場景及HD主繪圖本身沒有Machine、MIPS、原繪圖routine或GP0；測試中的原生畫布是獨立oracle比較，不是HD前置繪圖。原版模式、海報、存檔、現行素材包及玩家參數保持。圖像預算只計傳回的圖像資料，不代表程序／解碼暫存峰值；此helper不建立正式完整TexturePack，也不驗證未選中PNG內容或sprite-runs。

首個Go準備因可寫輸出使用了只讀/materials別名而在建置前停止，修/output後只執行一次Go參考。保存Rust測試先停在把baseline原碼SHA套到新實作，改驗保留副本；第二次停在從/output推導父目錄，兩次皆像素比較0。清理暫停後由已驗證來源記錄取得原圖目錄，完成唯一一次兩圖像素／完整色字比較。正式繪圖實作僅一版，測試器三版／三次啟動皆保留，不把準備停止稱為通過；已過11個公開案例不重跑。新遊戲、原routine、GPU、GUI、art review與7387條整包重驗皆0。

本節未接正式玩家。呼叫者可見的完整NO3 CPU／RAM／scratch／時鐘效果、原生顯存相依、完整排序與其他人物／文字／3D仍須另接；不能只把這張靜態場景接上就跳過整段原routine。GPU後端、1080p GUI、原版／HD熱切換、正常遊玩與速度未驗，不以HD4 PNG稱完整月下HD。初始化沿16.3.3承接，後續沿16.1補場景的正常提交與其餘圖元，不重開本節已通過美術／資料。私人scene-v1的rust-verification.json、tested-source.json、acceptance.json及publication.json由素材索引追溯。


### 16.5 Rust普通NO3呼叫資料與正常提交

狀態：CONFORMED，限定已證實NO3普通兩層、空圖元緩衝與六份正常保存態的高階RAM／scratch／CPU／時間計畫；只進受限正常研究入口，完整玩家HD／人物文字／GPU仍DRAFT。來源承接025 §12.17的2332-byte原函式、兩helper、IDA9.4原位址與SHA，以及現行自有Go apps/sotn/hd_call_{plan,effects,clock}.go；GPL-3.0-or-later，不移植外部非自由原碼。原程式只作oracle，Rust不解碼或逐指令實作MIPS。

私人no3-call-v1/evidence.json由素材assets-catalog索引，記錄precision-oracle及precision-next-8383至8387六態完整前後RAM／scratch／CPU與來源SHA；入口RAM皆同既有entry-gates保存值，不新跑舊遊戲。原PS1 KSEG0函式0x800ECC10..0x800ED52B SHA40dfd40f6a9922dd5ed1b69f1fd32ada3f7a8139b793984f49d373d3f31b797c，0x80011F18長60 SHA1c2e33c9e0ddfcd56385916112089fab3d7d343336e2514633a70260ef7d4064，0x800135F0長1464 SHAbbae27e62769d757261709c0c55190c9d7ea6098ee3efd5cbb8f83c2f1636615；NO3 header與44-byte固定配置SHA沿025 §12.17。

支援入口PC0x800ECC10、RA0x800E3C2C、SP0x801FFFC0、無待處理load／branch／IRQ、無隔離、BIU125320，原碼／helper／header／配置身份須相符。只接兩個已證實緩衝0x8003CB08／0x800542FC、圖元／模式計數0、前景及背景flags3、倒數0、背景kind1、其餘背景關閉、相機偏移0及SDK編碼欄0。兩層非負捲軸的17×16相容掃描窗須在有界資料內，場景data／definition／表仍須有效；未知配置回錯，不改原版、不半場回退或強開不完整HD。

資料計畫直接掃描圖層／格子，建立型別圖塊、頁bucket與必要原相容RAM字，保留原掃描順序、640圖元／1024模式界線、完整排序鏈及計數。這些RAM封包是後續原生相容副作用，不作HD來源或GP0主重播。高階掃描終值及最後非零格推導全部scratch、ABI堆疊字、CPU GPR／HI／LO與返回；不是執行原組語。CPU快取、RAM讀取成本及列乘積完成時點承接Go已驗的資源足跡，總量子、每量子及完整CPU須同原oracle，不寫死59271。

計畫擁有前態完整RAM／scratch摘要與CPU／BIU，提交前再核對全部；過期計畫拒絕，不發布HD。通用Machine新增明確的完成乘積／scratch資料提交，保留16.3.3舊commit_ram_phase的HI／LO及期限守門；新入口只容許已完成muldiv_done<=結束時間、恰1024-byte scratch，COP0與GTE完成期限仍保持，GTE本體不寫。原預算、量子／RAM patch、DMA／MDEC／IRQ／跨場與候選副本原子拒絕皆沿16.3.3，成功才一次套用RAM／scratch／CPU／裝置。

先對六保存態核對完整2MiB RAM、1024-byte scratch及CPU全部欄位，再由Rust CPU oracle局部執行同原bytes逐量子核對成本；原指令只出現在測試。不得用保存輸出生成計畫。正常比較只開一條真BIOS／同卡固定按鍵至8382，自然到入口後兩路連續八次：原路執行原routine，另一側直接Rust計畫／提交，完整Machine及續跑場相同。用可重生的逐欄比較覆蓋，正常路線不注入PC／RAM／原碼；獨立診斷驗scratch長度、未完成乘積、過期入口及IRQ拒絕。所有失敗／版本與來源保存，不重跑已過美術、矩陣或離線場景像素。

在正常入口以16.4元件直接生成CPU-HD4圖層畫布，圖片先準備再提交，未知來源或提交失敗不發布；只記已接受的圖層數與畫布摘要。不是完整前端或完整整場，其他圖元仍未接；原生相容後續DMA／GPU沿原流程保留，不能因此稱整個HD的舊光柵計數為0。原版模式、CPU／GPU、海報、影片、存檔、素材及跨平台目標保持。只限本節完成後標CONFORMED，不宣稱可玩的完整月下HD或速度通過。


16.5限定驗收：六份保存態完整2MiB RAM／1024-byte scratch／CPU與42321個成本量子逐筆相同，每份59271週期，72個來源／過期拒絕通過；七個通用資料提交案例通過，舊RAM-only的HI／LO守門保持。真BIOS／原片／同卡／固定按鍵只新開機一次，自然抵達入口後連續八次Rust提交，完整Machine逐欄與原路相同，續跑8390場及所取音訊相同。原控制組合計338568個指令，Rust提交guest step為0。每次提交前由場景／定案素材直接準備1024×960 HD4圖層畫布，成功才接受摘要；不輸出成假遊戲截圖。沒有先畫原生圖層再放大HD，後續原生相容DMA／GPU仍保留，因此完整舊光柵化尚未移除。

第二緩衝位址的準備轉錄0x800543FC由六份原態訂正為0x800542FC，READY原始快照保留；另兩次編譯準備停止分別為缺少Bus匯入及測試器JSON表達式，均未啟動正常遊戲。來源／停止／完整欄位比較與收據由私人素材assets-catalog的no3-call-v1入口追溯；美術／GUI／硬體GPU／舊矩陣／全素材重驗0。本節不代表人物、文字、特效、3D、正式GUI或完整HD已完成，沒有新速度聲明。


### 16.6 Rust物件幾何及直接圖像搬移

補註 `RUST-ROTATION-GEOMETRY-20261031`：以下為§16.6當時驗收。DRA sub_800EB544的旋轉原表數學／型別幾何已沿§16.7.16限域封閉，完整CPU／GTE／時鐘效果仍拒絕；ordinary bank15沿§16.7.14。舊未知與拒絕收據保留歷史，不單用本節重開已解幾何。

狀態：CONFORMED，限256筆物件資料解析、普通／packed幀、無旋轉固定8位縮放、型別四邊形及必要相容封包，另含同步GPU圖像上傳／精確清邊。整段物件的CPU／scratch／stack／裝置時間與正式玩家接管仍DRAFT，不因局部GPU完成而跳過未知效果。

來源沿025 §12.18：IDA Pro 9.4的DRA-map.i64及原bytes／xref，PS1 KSEG0 sub_800EB998在0x800EB998..0x800EC9AF，4120 bytes SHA5345790c96491fc6f21610d3ff975bc7311c46515f660dea3e0d1a96c66c39d7；sub_800EB544提供固定8位縮放，旋轉及bank15複製仍未知。來源原DRA SHA c37e2e6d80a91952cd29e6581d3346f05acf4e53038cb8345deb03ef92b2c308。型別模型承接素材tools/sotn_actor_plan_research.go及既有12次正常原態，已證實181四邊形及144圖像工作。完整SHA、原位址、工具與原始存取由私人actor-v1/evidence.json指回既有收據；GPL-3.0-or-later自有Go來源，不移植非自由外碼。

遊戲層讀2MiB RAM及1024-byte scratch，核對原routine身份與已證實緩衝。物件為0x800733D8起256筆188-byte，普通幀22-byte部件；packed幀由來源header及40-byte cache記錄給寬高、偏移、翻轉、頁及色彩。先驗全部來源、幀數、768 quad及512排序bucket；未知旋轉／bank15或越界回錯，不交出半個計畫。以高階格子／子圖計算頂點、UV、裁邊、固定8位縮放、CLUT、混色、Gouraud／raw旗標與排序，再序列化必要52-byte相容資料；主輸入不是原GP0封包，不執行MIPS。回傳型別圖元及圖像工作，未註冊正式玩家，不修改存檔或原資料。

通用GPU新增有界型別TextureJob::Upload及Clear。Upload擁有來源word及Rect，直接寫顯存，保留遮罩、完整framebuffer cursor／尺寸與SDK快取失效、FIFO殘值及DMA方向。Clear直接清精確矩形，沿原line-skip，保留原繪圖區／偏移；SDK原流程清遮罩，保留資訊讀回及FIFO殘值。不是02h fill向16字外擴，不呼叫GP0／GP1 parser或任何原光柵重播。入口須完整命令邊界，Rect非空且不跨顯存，最多768工作／524288來源字；含Clear時只接已觀測資訊高位為0的上下文。所有工作先驗，再在候選GPU上一次成功提交；任何錯誤原GPU全部欄位保持。這是同步GPU資料契約，不自行更新Machine DMA／IRQ或虛擬時間。

驗證一次集中使用12保存態：完整型別資料、相容封包／512排序表／計數及144工作同Go與保留原版；Rust測試端從原CPU存取依序還原RAM寫入、GP0／GP1及DMA2 block／linked-list，與型別直接作業比較全部GPU欄位及完整VRAM。參考端才用GP0解析，Rust作業路徑不使用它；使用同一保留原VRAM種子，明示這是局部事件oracle，非正常新開機或整段Machine等價。公開合成案例驗遮罩、精確清邊、快取／游標、未完成FIFO、長度／預算及原子拒絕。注入邊界案例另標diagnostic，不當正常玩家通過。已過場景、美術、GUI／GPU硬體與全素材不重跑；完整人物HD呈現及其餘CPU／時間效果仍待接，不稱完整HD或速度通過。


### 16.7 物件呼叫完整CPU與裝置時間接續

狀態：DRAFT。承接16.6已備的幾何及直接GPU資料作業，先由保存12態的原CPU／存取序列定位剩餘RAM、scratch、ABI與逐量子時間。原CPU僅在私人研究oracle執行，不是正式接管實作，也不由原輸出或存取序列回播生成正式結果。只有資料來源、全部必要效果與途中SDK事件／DMA／IRQ／時間都封閉後才升READY；目前不註冊正式玩家、不跳過整段物件routine。記憶體／切場可作使用者已授權的診斷，另標注入來源，不把局部注入當正常可玩完成。


16.6限定驗收：三個通用上傳／精確清邊／原子拒絕案例通過，人物模組編譯通過。唯一12態資料／GPU批次的181個完整型別四邊形、封包／512排序表／計數及144個工作同保留Go／原版；原CPU存取還原的GPU事件參考與直接型別工作之完整GPU逐欄及VRAM全同。參考種子為既有原VRAM，加原存取已觀測的繪圖區／偏移；不稱正常新開機或Machine整段已替換。直接路徑不呼叫GP0／GP1 parser，參考端才用原命令／DMA；正式玩家仍未註冊，未新畫／重審美術或重跑已過矩陣。

16.7接續證據：一次12保存態局部Rust原CPU oracle，完整RAM／scratch／CPU與所有原存取逐筆相同，保存成本量子、原PC資源足跡及I/O發生步數。這些原指令只用於研究，不能拿輸出或事件回播當正式Rust實作。幾何計畫與完整原RAM仍差46–52個word、121–127個byte，scratch差23–23個word；具體差異依每例收據，不能把GPU相同稱整段呼叫相同。動畫cache兩個邊框矩形、最後48-byte圖元scratch、掃描終值及排序／下一封包指標等來源方程已由12態驗證，SDK工作區／ABI、其餘scratch、完整CPU與時間／DMA／IRQ仍待實作，16.7維持DRAFT。原始位址與未解欄位保持，不推測性改名。工具／來源由私人素材索引actor-v1進入verification、effect-oracle、effect-gap-ledger、tested-source及acceptance。


#### 16.7.1 物件RAM／scratch與架構資料

狀態：CONFORMED，僅資料計畫，不含快取／時鐘／乘除完成期限、Machine提交或正式接管。來源為16.6十二態及新增IDA9.4原SDK bytes／邊界／xref，原函式名與PS1 KSEG0位址保持：sub_80012978、sub_80013D80、sub_80013FCC、sub_80014670、sub_80014F40、sub_80015308、sub_800EB544。每段範圍及SHA由私人actor-state-v1/SDK-functions.json保存，低位址bytes已對原boot檔offset EA-0x80010000+2048相同；DRA及正式DB原SHA沿16.6。程式為自有GPL-3.0-or-later資料實作，不執行或逐指令翻寫原MIPS。

型別輸入包含入口RAM／scratch／CPU及明確GPU繪圖區／偏移、IRQ mask。GPU提供唯讀型別context，不經GP0查詢。普通入口PC0x800EB998、RA0x800E3C34、SP0x801FFFC0、完整load／branch邊界、初始quad0。沿16.6來源／緩衝／圖元守門；限無hidden、至少一個packed上傳、最後活躍物件為完整無縮放／翻轉的普通幀、未滿768圖元，SDK未開debug／extended、queue idle、callback關閉，所有本批清邊走非64字整齊的精確矩形。其餘分支回錯，不回播保存結果或猜填。

資料直接從ActorPlan及原幀表推導：packed cache+8／+16兩個清邊矩形、最後圖元48-byte scratch、掃描末bank／frame、寬高／物件原點／裁後offset／page低位／部件計數／flags／flip及OT／下一封包。未寫的scratch高位保留。RAM亦建立SDK最後精確清邊linked-list、VBlank timeout=原global+240及零計數，保留原SDK／外層／最後transform的ABI框架。transform前callee-saved資源由前一子圖UV、當前cache／partflags及幀表位置推導，包含原call return位址；不是把oracle stack抄回。

架構結果用獨立型別表示GPR、HI／LO及return/load/branch，不冒充完整CPU State。volatile由最後圖元、CLUT索引、幀表及掃描尾端推導，callee-saved沿原入口；HI／LO取最後實際乘積，packed上傳尺寸後可能由後續X／Y固定小數縮放覆寫。快取、now／cost、muldiv_done與gte_done保持待接，不用前態時間標成CPU完整等價。資料apply先驗完整RAM／scratch摘要、CPU及context，過期整筆拒絕；沒有Machine或裝置推進。正式玩家不得直接使用此半完成資料apply跳過原routine。

驗證只載十二份保存態，完整2MiB RAM、1024-byte scratch與全部已描述架構欄位同原版；原CPU／SDK不重跑。另改前態保留區、callee-saved及scratch未寫半字做診斷，若需原CPU僅在測試作oracle，不能擴稱正常路線。集中驗來源／context與過期輸入的原子拒絕。16.6已過幾何／GPU及美術不重驗，完整Machine與時間仍沿16.7 DRAFT。

16.6清邊契約追加訂正：新增SDK證據sub_80013D80表明x與width同時64字對齊時走02h快速清除，不查／改read_buffer，FIFO末command為02h；其他矩形維持精確60h路徑、恢復環境與資訊讀回。兩者像素範圍皆同型別Rect，mask清0、line-skip保持。原144工作全非此對齊分支，舊限定收據保持；新增對齊邊界只做一個針對案例，不重開既有美術／矩陣。


16.7.1首次資料比較停止：case0完整RAM及scratch相同，架構GPR1／GPR10不同，尚無完整案例通過。原碼、日誌與最初READY快照保留於actor-state-v1。回到原始bytes確認0x800EC480的CLUT假指令包含LUI 0x8004、ADDU及LHU帶符號offset0xC104，因此GPR1保留0x80040000+palette*2，並非有效位址0x8003C104+palette*2；0x800EC954每次部件推進T2 22 bytes，結束值為最後part address+22，transform保存值則為當前part address。依此明確訂正架構資料契約後回READY，其餘RAM／scratch公式保持；不重跑舊GPU／美術／正常路線。

16.7.1訂正後READY：保留初版及第一次停止。入口另核對SDK靜態函式／MMIO綁定，表在actor-state-v1/SDK-bindings.json；末槽須無活躍圖元，使最後掃描skip的架構終值有證據。這是已描述普通分支的守門，不採用保存輸出生成資料。

16.6短上傳邊界訂正READY：新增SDK原bytes 0x800140D0..0x80014108及0x800141E4..0x80014230表明ceil(width*height/2)不足16個32-bit字時，全部經CPU寫入，沒有GP1 DMA方向2；方向保持起始寫入的0。達一個16字區塊才設方向2並啟DMA。原144工作上傳皆達此界線，舊GPU收據保持。更新錯把短上傳也設2的泛用端點與原合成測試參考，僅補30／31／32個16-bit字的邊界及受影響遮罩案例，不重跑全部素材或舊資料批次。


16.7.1限定集中結果：12保存態共25165824 bytes完整RAM、12288 bytes scratch及全部Architecture欄位同原版，COP0保留且同原版。72個過期RAM／scratch／CPU／時間／context／IRQ mask原子拒絕與4個隔離／IRQ／context範圍拒絕通過；case0與11各一個局部原CPU注入對照亦同，包含RAM保留欄0x80097948非零、scratch未寫半字0x1F800046非零及S0／S7改值。兩個注入只用於診斷，不代表新正常玩家路線。原CPU只在這兩個測試執行，12普通態承接既有原結果，不再開機或重跑原routine。

首次停止在case0架構GPR1／GPR10，當時RAM及scratch已同、完整案例0；保留初版原碼／日誌／READY後回RE，由原CLUT假指令三個原字及0x800EC954推進端訂正。第二次只完成未過資料批次，12普通態／兩注入及拒絕皆過，沒有第三次。不可將兩次啟動改稱首次全過。16.6新增對齊清邊、短上傳30／31／32個16-bit字邊界及受影響遮罩案例通過；既有144工作皆非新邊界，未重跑其GPU／美術矩陣。

已封閉的是資料與架構結果，尚未封閉快取、now／cost、乘除期限及Machine裝置時間／DMA／IRQ原子提交。Architecture是獨立型別；apply_data保持原時鐘／快取，不得在正式Machine使用它跳過整段人物routine。16.7整體仍DRAFT。原版模式、原卡／原片、定案素材、海報、影片及Go完整包保持；新正常GUI／硬體GPU／速度／跨平台驗收0。來源與原始位址／SHA、原DB保持、首停及最終收據沿私人素材索引actor-state-v1。


#### 16.7.2 普通物件快取與時鐘資源

狀態：CONFORMED，限定16.7.1資料分支及已證實普通packed／未旋轉幀的資源模型。DRA／boot／原DB沿16.7.1；新增IDA9.4確認sub_800145D0、sub_8001461C、SetIntrMask原名稱／邊界／bytes／xref，原始範圍及SHA由私人actor-clock-v1/SDK-functions.json保存。原DB唯讀，低位址bytes同boot offset EA-0x80010000+2048。216個已觀測非零讀取成本位置沿原存取及008／現行cpu.rs硬體模型，無成本類別衝突；十二原成本向量及足跡已凍結於evidence.json，僅作測試oracle，不由其生成正式執行序列。

模型由高階工作量生成資源區段：256個物件掃描、幀表選擇、圖像上傳與兩清邊、固定8位縮放、普通部件裁邊／UV／混色、52-byte相容資料拷貝與排序連結、返回ABI。正式繪圖資料仍由ActorDataPlan產生；Clock不讀／解碼／執行MIPS，不由保存輸出或PC向量回播。區段原PC僅定位指令快取及讀取成本，並沿既有16.3／16.5資源方法逐量子更新快取、now／cost及乘積期限。RAM讀+4、I/O讀+2、scratch+0；cached fill=7-word。signed乘法期限依現行CPU的leading-sign-bits表，MFLO等待取max(base,deadline-now)，不是固定總週期或固定26476。

BIU125320、無隔離／待處理IRQ／load／branch、快取valid低4位；16.7.1來源／靜態綁定／queue守門保持。此Clock先限packed bank1、原帧帶上傳、無facing、上傳至少一個16個32-bit字DMA block，清邊非64字整齊分支；普通幀page低位0、不走bank15／未知rotation、垂直UV翻轉／order+4／未觀測color分支。未知工作回錯，不半場沿原MIPS補完。最多100000量子、每步1至32、總和checked，超量／溢位拒絕。Clock endpoint合併已驗Architecture與推導的cache／now／cost／muldiv_done，COP0／GTE完成時點及GTE本體保持。

一次集中核對十二保存態全CPU State、完整成本量子與既有向量，RAM／scratch沿16.7.1不重驗；另case0冷cache與case11完整valid做兩個局部原CPU oracle診斷，供應保留原MMIO回覆，不能稱正常Machine或硬體wall-clock。原CPU只在測試使用。Cache／時間守門及溢位拒絕需通過。舊美術／幾何／GPU矩陣不重跑，通過後只限本節CONFORMED。

#### 16.7.3 同步圖像與資料的Machine提交

狀態：CONFORMED，限定16.7.1–16.7.2支援分支、同步DMA2且不跨場／不發生可受理IRQ。現行SystemBus::advance不讀RAM或VRAM，GPU end_line只更新video欄位，scanout到screen才取pixels；無活動其他DMA／MDEC時可逐原成本量子推進，再套用本段完整資料／圖像終值。GPU line-skip的field_readout在VBlank才更新，跨場拒絕維持其穩定。這是現行執行器已證實相依，不稱PS1實機逐週期時間。

通用新提交入口不放月下位址。輸入為CPU端點、RAM word patches、scratch、成本量子及已完成的TextureJob批次；限定最後job為Clear，完成DMA2 linked-list端點為MADR=0xFFFFFF、BCR=0、CHCR=0x401。啟用DPCR GPU通道、初始GPU完整命令邊界且ready、全部DMA quiescent、MDEC無等待、IRQ非pending；DICR force或啟用GPU通道IRQ時拒絕，不猜中途IRQ。RAM／scratch／CPU／量子／預算守門沿16.5，新入口只放寬已證實的同步GPU／DMA終值，不改舊no-I/O API。

先在候選Machine依每量子保留原advance裝置順序，任何跨field／IRQ／活動DMA或MDEC等待整筆拒絕，原音訊及機器全保持。候選GPU直接套TextureJob，不用GP0／GP1 parser或原光柵回播；設定已描述DMA2終值並沿現行dma_finish更新master狀態，若因此IRQ亦拒絕。初始及結尾i_mask相同，途中SDK設0且沒有可受理IRQ的已驗普通分支不改端點；讀GPU狀態／info及timer counter無其他破壞性副作用，GPU info／FIFO終值沿16.6。所有時鐘與圖像成功才套RAM／scratch及全CPU並移交候選。

遊戲ActorClockPlan提交前核對完整前態RAM／scratch／CPU、BIU、GPU context、IRQ mask，並核對ready／靜態來源，過期回錯。先做通用公開傳輸／原子拒絕測試，再只新跑一條真BIOS／原片／同卡／固定按鍵正常路線至8382，自然到入口連續八次與原MIPS控制組比較全Machine及原音，續跑8390。只在測試副本增加逐欄Eq，無注入PC／RAM／原碼；獨立IRQ／跨場／DMA條件診斷另標。主路徑仍未註冊不完整整場HD，原版、海報、影片、素材／存檔及CPU／GPU目標保持，沒有速度或全遊戲HD聲明。

16.7.2來源守門補充：既有next-draw-helper-IDA-v1已含sub_80012A90及sub_80012B24兩個wrapper，邊界／bytes同原boot。Clock除原16.7.1與三個新增helper身份外，亦核對這兩個wrapper完整SHA，覆蓋每個已觀測時間資源位置。這只收緊接受範圍，不改資料／時間公式；原READY及首次已過成本結果保留，新增守門以十二保存態及破壞wrapper來源／時間溢位拒絕核對。正常對照使用完全相同原來源，不因此重開遊戲。


16.7.2–16.7.3限定結果：12保存態的全部CPU State及逐量子成本同原版，另case0冷cache／case11完整valid兩個局部原CPU對照相同。BIU／cache無效位、兩個wrapper身份及時間溢位共五種拒絕通過。新增來源守衛後只補集中快取核對，初次已過結果保留，沒有重開正常遊戲。通用texture／data交易兩個新案例與七個受共享傳輸影響的既有案例共九項通過；舊no-I/O限制保持。

一次真BIOS／原片／同卡／固定按鍵自然開機，8382起八次人物呼叫不執行原MIPS或GP0／GP1，直接提交typed texture jobs、RAM／scratch、CPU及逐量子裝置時間。八次完整Machine逐欄同原控制組，包含VRAM／快取／DMA／timers／CD／SPU／SIO／MDEC及排隊原音，續跑8390仍全同，所取原音SHA相同、原片與卡片保持。IRQ途中、跨場、過期CPU、DICR force、DMA2 IRQ及DPCR關閉六種獨立診斷全機器原子保持；注入副本不回正常路線。輸入／版本、每次指令數及工作量沿私人actor-clock-v1的normal-verification.json、tested-source.json及acceptance.json，原始來源與DB沿既有IDA索引。

這封閉已列普通人物呼叫的同狀態提交，16.7整體與16整段仍DRAFT，尚需其他人物分支、其餘圖元／文字／特效／3D及完整場呈現交易。正式GUI未註冊本切片，後續原生相容DMA／光柵仍存在，不稱完整舊光柵移除或遊戲HD完成。CPU／GPU、海報、影片、已定案素材與原版模式保持；新美術、GUI、硬體GPU、三平台包驗收0，沒有新FPS或加速倍率聲明。


#### 16.7.4 未旋轉人物朝向與UV翻轉

狀態：CONFORMED，限定沿16.7.1–16.7.3上下文與最後普通未翻轉部件端點，只擴充中途packed／ordinary的左右朝向及ordinary部件UV bit0／bit1。輸入、原DRA／boot／DB與來源SHA沿前節；新證據直接讀既有IDA9.4的sub_800EB998與sub_800EB544完整函式匯出，bytes及邊界同原保存RAM。原始KSEG0位址／運算元未改，DRA file offset為EA-0x800A0000。actor-mirror-v1/evidence.json及new-resource-sites.json保存原始來源雜湊、分支與83個新增資源位置，不重開原DB或美術。

已證實朝向欄在entity+0x14，packed以其左移一位xor cache+0x23；此切片只接受entity朝向0／1及最終flip0／2。packed座標基準由EBD98 bit2分支選x±dx；翻轉後兩側為base-width+1及base+1，U兩端為u1-1／u0-1，固定8位縮放的X支點亦隨flip改成x-anchor。ordinary以原entity朝向選x±dx、以flip決定頂點排列；U翻轉條件為(part_flags&2) xor flip，V翻轉條件為part_flags&1，V兩端為v1-1／v0-1。混色、裁邊、縮放、圖像上傳／清邊及遊戲時序保持，不新增鏡頭或玩法修改。

沿既有ActorDataPlan即可生成完整資料，17組局部原CPU注入已核對完整2MiB RAM、scratch及Architecture相同，涵蓋兩保存態的第一／最後packed、普通及多部件、V／UV合翻、朝向與UV組合及scaled-XY。這是診斷證據，不稱正常Machine。正式繪圖仍由高階ActorDataPlan建立，不解碼／逐指令執行原碼或回播保存輸出。

時鐘新增高階朝向／UV工作分支，沿16.7.2快取資源模型生成逐量子成本。新增load中只有0x800EB57C由a2指向entity RAM、成本+4；其餘13個由s0指向scratch、成本0。原型資料與IDA寄存器來源互證；不以opcode或保存PC向量驅動正式執行。來源完整SHA、未知分支／BIU／cache／溢位／stale與原子Machine守門保留。仍拒絕旋轉、page低位、order+4、未知color／SDK及未封閉最後物件端點。

集中驗收：17個新注入加兩個原朝向控制，核對全CPU State及每個成本量子；原CPU只作獨立測試oracle。再只新跑一條真BIOS／原片／同卡正常路線，8383至8388改為正常LEFT、8389起RIGHT，在自然人物入口連續16次比較全Machine與原音，正常觀測須同時含朝向0及1，續跑8400。正常路線不注入RAM／PC／code；UV特殊組合只由前述診斷證明，不冒稱全在正常路線出現。舊GPU／美術矩陣與完整包不重跑；驗收只限本切片，不宣稱正式GUI、全人物、整場HD或速度完成。


16.7.4限定結果：一批17個新朝向／UV注入與兩個原朝向控制，共249354個時間量子、完整CPU／RAM／scratch同局部原CPU。17個新資料分支在實作前先有完整資料證據，READY後只補高階朝向與UV的時間資源分支，沒有改ActorDataPlan、圖像工作、原ISA或GP0路徑。83個新資源PC皆由原IDA9.4匯出覆蓋；14個新增load只有EB57C讀entity RAM成本4，另外13個讀scratch成本0。正式路徑不讀opcode或原PC足跡，注入只在測試oracle。

唯一正常真BIOS／原片／同卡路線在8383起正常按LEFT六場、8389起RIGHT二十場，自然進入16次人物呼叫，觀測朝向0及1，其中6次為翻轉朝向。全Machine逐欄及全部成本同原控制組，略去原參考208439指令，建立246個型別四邊形與192個圖像工作，續跑8400及所取原音SHA全同。原片／卡片保持，正常路線無PC／RAM／code注入。UV特殊組合仍只由局部診斷證明，沒有聲稱全部出現在這條正常路線。來源、有效按鍵與限定結果由私人actor-mirror-v1的tested-source、effective-input、normal-verification及acceptance追溯。

本節只放寬已證實的未旋轉朝向／UV分支，最後物件端點仍沿16.7.1限制，其他bank／跳過上傳、旋轉、色彩／SDK、page／order及整場呈現仍待完成。正式GUI未註冊，原生後續相容光柵仍在，不把這段稱全人物、完整HD或速度完成。原版、CPU／GPU、海報、影片與定案美術保持；新美術／GUI／硬體GPU／包驗收0，舊矩陣不重跑。


#### 16.7.5 最後普通部件與第255槽的終值

狀態：CONFORMED，沿16.7.1–16.7.4支援上下文，放寬最後ordinary部件為朝向0／1、flip0／2、無旋轉的flags0至3，以及普通物件位於末槽255；packed末物件與其他未知色彩／SDK／bank仍拒絕。原DRA／boot／DB與KSEG0位址空間沿前節，DRA file offset為EA-0x800A0000。新研究只讀既有IDA9.4原bytes與邊界，不重開DB；來源SHA、候選及初停由私人actor-tail-v1/evidence.json、candidate-initial-evidence.json及initial-stop.json保存。

末尾ordinary的座標／UV、scratch、圖元／排序／RAM與最後縮放HI／LO已由ActorDataPlan既有公式直接產生，不執行MIPS或回播事件。兩保存態×兩末槽98／255×朝向0／1×四縮放旗標×四UV共128組，加四個負X／Y縮放組，共132組局部原CPU診斷；完整RAM與scratch全部相同。66個槽98案例的Architecture也全同，槽255的66個案例只差GPR3。原候選尚未修改正式程式，不把初停改寫成全過。

GPR3來源已證實：0x800EC940從scratch+0x1C載入最後幀的部件數，0x800EC94C比較部件迴圈終值；0x800EC958至0x800EC978與epilogue未改GPR3。若後面還有空槽，0x800EBA4C／0x800EBA5C／0x800EBA68依末槽bank/frame為零覆成1。因此普通最後活躍槽為255時GPR3=last.n，否則沿原限制的空末槽為1。保留原位址與原始operand，此公式不寫死保存值5。其他volatile、ABI、保留值與乘法終值仍由已有型別資料推導，時鐘沿現行高階掃描自然處理末槽，無新opcode／成本表或GPU作業改動。

正式只調整ActorDataPlan末尾守門與GPR3來源；Clock、直接texture工作、原ISA與原版模式保持。參數接受範圍及來源身份沿前節，過期CPU／RAM／scratch／context與Device原子拒絕保持，不因末槽放寬IRQ／跨場。存檔及正式玩法不改，注入只在診斷副本。

集中驗收：正式來源核對132組全CPU State／完整RAM／scratch與全部成本量子，再補末槽255的部件數1與2兩個界線，共134組；每次由局部原CPU獨立執行作oracle，正式資料不使用它。再只新跑一次正常真BIOS／原片／同卡前綴到8382，先對自然人物呼叫驗一個未注入基準，再在同一正常入口的獨立副本做8組末尾／末槽／縮放／UV診斷。每組完整Machine與全部成本／排隊原音同原參考，不改原卡／原媒體；正常基準續跑8383。末槽與縮放的新覆蓋只稱注入診斷，不宣稱它們由正常玩家自然出現。舊朝向／GPU／美術矩陣不重跑，通過只限本節，不稱正式GUI、全人物、整場HD或速度完成。


16.7.5限定結果：一批134局部原CPU對照共1801679個量子，完整CPU State／2MiB RAM／scratch及每個成本相同；包含128個正縮放組合、四個負縮放及末槽部件數1／2兩界線。初候選132組全部RAM／scratch同、66個普通末槽Architecture同；另66個第255槽只差GPR3。保存候選／初停後由EC940與EBA4C／5C／68證實終值來源，READY後以last.n修正，不寫死5。正式批次只執行一次，沒有把初候選停止改成首次全過。

唯一正常原片前綴到8382的自然人物基準同原版並續跑8383，完整Machine與所取原音同。另在同一自然入口的獨立副本做八組RAM注入，末尾朝向、X／Y／XY、負縮放／UV及第255槽完整Machine、全部成本與排隊音訊同原CPU控制組，注入控制組共105929指令；這八組明確不算正常玩家覆蓋。原片／卡片保持，未知玩法不猜補，副本不回正常路線。原始來源、有效診斷參數、受測原碼與限制由私人actor-tail-v1的initial-stop、READY、verification、machine-verification、tested-source及acceptance追溯。

只放寬末尾ordinary的已列終值與末槽，packed末物件、其他bank／跳過上傳、色彩／SDK、旋轉、page／order與正式整場仍待。Clock／GPU工作／原ISA未改，正式GUI未註冊，後續原生相容光柵仍在，不稱全人物HD、完整接管或速度完成。原版、CPU／GPU、海報、影片、定案美術及Go包保持；新美術／GUI／硬體GPU／打包驗收0，舊矩陣不重跑。


#### 16.7.6 帶上傳的packed末物件

狀態：CONFORMED，沿16.7.1–16.7.5上下文，只擴充packed bank1、frame含上傳、強制CLUT索引、entity朝向0／1、flip0／2及flags0至3的最後物件，允許末槽255。須有此前已完整處理的ordinary部件以封閉保留scratch／S7來源；全packed、跳過上傳、其他bank、未知color／SDK／rotation保持拒絕。原DRA／boot／DB及PS1 KSEG0 RAM空間沿前節，DRA file offset為EA-0x800A0000。只讀既有IDA9.4 sub_800EB998／sub_800EB544原bytes與邊界，不新開DB或改原名稱。來源SHA、隔離候選與72局部原CPU資料對照保存於私人actor-packed-tail-v1/evidence.json、actorstate-candidate.rs及candidate-evidence.json。

已證實資料公式：packed的scratch+0x34與+0x4C分別保留前一ordinary部件的page低位及已完成部件數；EBB90／EBBC4將+0x50更新為packed header低15位圖像ID，不是ordinary部件數。其餘最後寬高、原點、offset、flip、flags及圖元由最後型別幀推導。ordinary在EC938將S7部件來源指標前進22，所以其後packed transform的保存S7為前部件address+42；ordinary本身transform仍保存前進前address+20。型別迴圈只在ordinary完成後更新這個資源，不改資料格式或模仿原ISA執行。

packed末尾的GPR4由EC194為原global計數位址0x80097930；GPR5由EC110最後一次拷貝為最後圖元word10的XY3原字；GPR10保留EBB84後的frame_data+2。末槽255的GPR3由EC1A0／EC1A8為總圖元數，後面若有空槽則沿EBA4C／5C／68覆成1。普通末尾沿16.7.5保持。GPR1、其餘返回／ABI、最後上傳／精確清邊工作區及HI／LO沿已有型別資料推導，不讀保存輸出、不回播PC／MMIO或原GP0。

證據涵蓋兩保存態、packed來源0／3、末槽254／255、左右朝向、四縮放旗標與正負XY共72組，含有／無cache頂點色來源，完整RAM／scratch／Architecture首次全同。移動原packed實體到末槽並關閉舊槽，保持上傳數；只在測試副本修改，局部oracle的MMIO回覆沿原上下文，不稱完整Machine或正常玩家覆蓋。正式實作只採已封閉型別終值與守門，Clock／GPU／ISA不改。

集中驗收：72組正式完整CPU State、全部成本量子及完整RAM／scratch；新增全packed無ordinary前態、frame跳過上傳、非強制CLUT三種未支援拒絕。接著只新跑一次真BIOS／原片／同卡正常前綴到8382，一個自然未注入基準續跑8383；同入口獨立副本做八組packed末尾注入，核對全Machine、所有成本及排隊原音。新packed末尾覆蓋明列注入，不當作自然玩法或全角色完成。原媒體／原卡保持，舊美術／GPU／ordinary矩陣不重跑；正式GUI與整場HD仍未註冊，速度另沿Issue #9。


16.7.6限定結果：72局部原CPU案例共945332量子、完整CPU State／RAM／scratch及每個成本相同；候選資料72例首次全同，READY後正式只接型別終值及來源守門。全packed無ordinary前態、frame跳過上傳與非強制CLUT三種拒絕通過，空ordinary等未知分支沿Clock既有拒絕。正式測試器首次因讀檔Result漏傳播在編譯停止、執行0例；保留原測試器／日誌後僅修測試器，唯一完成批次全過，不改繪圖公式。Clock／GPU工作／原ISA不改，未重跑舊矩陣。

唯一正常原片前綴取得8382自然入口，一個未注入人物基準與原版同並續跑8383，所取原音同。另在同入口獨立副本移動packed來源0／3至254／255、關閉舊槽，八組朝向／縮放與負XY的全Machine、所有成本及排隊音訊同原CPU控制組，共103360控制指令。原片／卡片保持，八組注入不當自然玩家覆蓋，副本不回正式路線。來源與測試版本、READY、準備停止、局部及完整機器收據由私人actor-packed-tail-v1的tested-source、verification、machine-verification及acceptance追溯。

這只封閉本節packed末尾分支，全packed前態、其他bank／跳過上傳、未知色彩／SDK／旋轉、page／order、文字／特效／3D及正式整場仍待。正式GUI未註冊，後續原生相容光柵仍在，不稱全人物、整場HD或速度完成。原版、CPU／GPU、海報、影片、定案美術及Go包保持；新美術／GUI／硬體GPU／打包0，沒有新速度倍率。


#### 16.7.7 部分／全部packed圖像重用

狀態：CONFORMED，沿16.7.1–16.7.6支援資料分支，允許packed entity frame半字bit15表示本次不再上傳；與packed header的bit15型別標記不同。仍限bank1、強制CLUT、朝向／縮放既有範圍、至少一個packed圖元及已封閉的最後ordinary或有ordinary前態的packed末尾。全packed前態、無packed的純ordinary、其他bank／未知色彩／SDK／rotation等保持拒絕。原DRA／boot／DB及KSEG0 RAM空間沿前節，DRA offset為EA-0x800A0000；只讀既有IDA9.4原bytes／邊界，不重開DB。來源SHA、分組證據與候選由私人actor-reuse-v1/evidence.json追溯。

已證實原分支0x800EBCC4／EBCC8檢查frame bit15，delay slot EBCCC仍設定V1資源，跳至EBD90，略過EBCD0–EBD8C的cache尺寸／兩矩形更新、三次SDK／DMA及暫存框架。型別圖元、來源及幀尺寸仍按原資料產生；跳過的圖像不生成TextureJob，也不改cache+4至+23的上傳效果。混合呼叫只由最後實際上傳推導SDK工作區／timeout與leaf frames；全部重用時這些RAM全部沿前態保持，由後續已證實的outer／transform ABI覆寫其實際位置。HI／LO在無上傳且無後續縮放時沿前態，有縮放仍依最後乘積，時鐘保留或推導相應完成期限。

正式高階Clock只依frame bit決定是否加入既有上傳／清邊工作，新增共用段為EBC48–EBCCC及EBD90起；不解碼原指令或回播PC。ActorClockPlan有TextureJob時沿16.7.3同步顯存提交；零工作時沿16.5已驗commit_cpu_data_phase，只更新資料／scratch／CPU與逐量子裝置時間，不改GPU、DMA2、readback或顯存。核心既有API與原版路徑不改；source／stale／IRQ／跨場／BIU／cache等守門保持。

候選先以兩保存態×16個跳過遮罩×ordinary／packed末尾，加16個正負縮放packed末尾，共80局部原CPU完整RAM／scratch／Architecture對照首次全同。測試oracle只從原始輸入bank／frame／cache獨立選擇上下文回覆：原每個cache的21筆SDK讀取由原EBCDC寫入邊界分組；全部重用時0筆，混合只取仍上傳的group。這些回覆只供局部oracle，不當完整裝置或正常玩家證據，也不在正式程式使用。

集中驗收：80組正式全CPU State、完整RAM／scratch及逐成本量子，一個無packed前態拒絕；再只新跑一條真BIOS／原片／同卡正常前綴到8382，一個自然基準續跑8383，同入口八個獨立部分／全部重用與末尾縮放RAM診斷比較全Machine／所有成本／排隊原音。全部重用的診斷另給DMA2非預設idle MADR／BCR，必須保留，不能偽造完成端點。原卡／原片保持，八組注入不當自然玩家覆蓋。舊ordinary／GPU／美術矩陣不重跑，正式整場／GUI／速度仍未完成。


16.7.7限定結果：80局部原CPU案例共886162量子，完整CPU State／RAM／scratch及逐成本相同，其中八組為0個TextureJob。原SDK group只在測試按原輸入選擇回覆，全部重用原CPU的MMIO讀取0，正式程式不讀此group／事件。無packed的純ordinary前態拒絕，原有全packed／未知bank／color／SDK等限制保持。

唯一正常原片前綴取得8382入口，一個自然基準續跑8383同原版及所取原音；另八個獨立部分／全部重用診斷的全Machine／成本／排隊音訊同原CPU，共81951控制指令。四個全重用診斷確認0個顯存工作，DMA2 idle MADR=0x135790、BCR=0x2468、CHCR=0x401保持，GPU／VRAM等完整欄位也相同，沒有偽造上傳完成。原片／原卡保持，八個注入不當自然玩家覆蓋。SDK／cache寫入只取實際Upload工作，0工作用既有commit_cpu_data_phase，核心API未改。

只封閉本節frame bit15混合／全重用分支，無packed純ordinary、全packed前態、其他bank／未知color／SDK／rotation、page／order與正式整場GUI仍待，後續原生相容光柵仍在。原版、CPU／GPU、海報、影片、定案美術與Go包保持；新美術／GUI／硬體GPU／打包0，未量穩態速度，不把少傳送工作換算成倍率。來源、READY、受測原碼及verification／machine-verification／acceptance由私人actor-reuse-v1索引追溯。


#### 16.7.8 全packed與純ordinary前態

狀態：CONFORMED，沿16.7.1–16.7.7來源、上下文、朝向／UV／未旋轉縮放及重用限制，擴充全packed沒有ordinary前態、純ordinary沒有packed及完全沒有transform呼叫的非空物件批次。其他bank／未知color／SDK／rotation、page／order、空批次及正式整場仍未封閉。原DRA／boot／DB與PS1 KSEG0 RAM空間沿前節，DRA offset為EA-0x800A0000；來源與保留原位址的IDA9.4匯出由私人actor-families-v1/evidence.json索引，不重開DB。

已證實prologue在EB998–EBA44初始化scratch+3、+7、+48、+5C、+60等欄位，沒有清scratch+34或+4C。ordinary分支EC1C8的delay slot才清+4C，並在EC1CC設定S7=part.address+20，完成後EC938推進22。全packed末尾因此由輸入保留scratch+34與+4C，transform保存的S7沿輸入，若有ordinary才沿16.7.6最後ordinary終值。不得將未寫欄位假設為零。

ordinary flags低兩位皆0時不呼叫EB544 transform；若整批都沒有transform，caller frame+10／14／20／24／28及其下helper frame沿原前態。外層callee-save ABI仍照常寫入。純ordinary沒有圖像上傳，SDK工作區、DMA2及顯存沿既有零工作提交保持；無乘法時HI／LO與完成期限沿原前態，有縮放才由已證實乘積更新。最後active槽98／3或255、scratch、架構與CPU時鐘均由型別資料推導，不讀保存輸出、解碼原ISA、回播GP0或原PC向量。

隔離候選先以兩保存態作144組完整RAM／scratch／Architecture局部原CPU對照，首次全同。108組全packed含部分／全部上傳重用、36組純ordinary，涵蓋末槽、左右朝向、flags0至3及正負縮放。每組注入非零scratch+34／4C、SDK工作區、caller／helper stack、S6／S7及HI／LO，避免零值掩蓋保留錯誤。MMIO回覆僅在測試oracle按原始輸入選擇，不當完整裝置或正常玩家證據。

正式實作只移除兩種已封閉前態拒絕，transform保存框架改為有呼叫才寫，全packed scratch保留值改從輸入取得。Clock、GPU工作、原ISA與核心交易API不改，來源／stale／IRQ／跨場／BIU／cache守門保持。集中驗收只補144組正式全CPU State／完整RAM／scratch與全部成本量子；再新跑一次正常原片前綴至8382，一個自然未注入基準續跑8383，同入口八個獨立全packed／純ordinary非零前態診斷核對完整Machine、成本及排隊音訊。零工作另以非預設idle DMA2核對保持。原卡／原片保持，注入副本不回正常路線且不稱自然覆蓋。舊GPU／美術矩陣不重跑，正式GUI／全人物／整場HD及速度仍未完成。


16.7.8限定結果：144局部原CPU案例共1187986量子，完整CPU State／2MiB RAM／scratch及每個成本相同。108全packed、36純ordinary，每組皆非零保留值，72個零顯存工作；八個純ordinary flags0組合完全沒有transform，保留caller／helper框架及HI／LO同原版。候選資料首次144全同，READY後唯一正式批次全過。正式只改型別資料的前態與可選transform終值，Clock／核心API／原ISA不改。

唯一正常原片前綴取得8382自然入口，一個未注入基準與原版同並續跑8383、所取原音同。另八個獨立全packed／純ordinary、朝向／末槽／重用／正負縮放與非零前態診斷，全Machine、全部成本及排隊原音同原CPU，共62850控制指令。六個無顯存工作診斷保留非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401；原卡／原片保持。注入副本不回正常路線，不當自然玩家覆蓋。

本節只封閉非空全packed／純ordinary前態；其他bank／未知color／SDK／rotation、page／order、空批次、文字／特效／3D及正式整場GUI仍待，後續原生相容光柵仍在。026 §16／16.7整體仍DRAFT，原版、CPU／GPU、海報、影片、定案素材與Go包保持。新美術／GUI／硬體GPU／打包0，速度未量，不以原控制組指令數換算倍率。來源與READY、受測原碼、verification／machine-verification／acceptance由私人actor-families-v1索引追溯。


#### 16.7.9 普通物件材質頁、排序偏移與逐色通道

狀態：CONFORMED，沿16.7.1–16.7.8來源與交易契約，只擴充ordinary材質page低兩位、part flags bit8排序加4、entity flags bit3色彩及bit4／5／6三通道取128的分支，允許ordinary最後物件與末槽255。最後packed仍限flags0至3；其他bank／SDK／rotation、空幀／裁掉物件、bank15特殊複製及正式整場保持未完成。原DRA／boot／DB、PS1 KSEG0 RAM及DRA offset EA-0x800A0000沿前節，原bytes／邊界及雜湊由私人actor-material-v1/evidence.json追溯，不重開DB或美術。

已證實ordinary page由part+12加entity+5A形成，低兩位寫scratch+34，高位右移2寫材質頁。EC4AC／B0分支低bit0時，EC4B4／B8將兩個U資源減128；EC4BC／C0分支bit1時，EC4C4／C8將兩個V資源減128，保留完整32-bit工作暫存器與最後UV byte截斷。既有ActorPlan／Part已建此資料，不以原ISA／GP0或保存輸出回播。

EC608–618檢查色彩bit3；有色彩時EC61C–658把entity+6C強度填四頂點RGB，再由bit4／5／6分別以128覆蓋R／G／B。EC6B4–6BC清raw-texture bit，沿EC6CC共用資料複製；無色彩沿EC6C0–6C8保持舊raw-texture路徑。EC620讀entity RAM成本4，EC61C／66C／690讀scratch成本0；正式只加入高階條件資源區段。EC718–720檢查part bit8，有時EC724排序加4；最終排序511接受，512由既有typed parser拒絕，不向未定義排序表寫入。透明／半透明、UV／裁邊、縮放及ABI沿既有公式，時序、原卡及玩法保持。

兩保存態192候選完整RAM／scratch／CPU State與逐成本量子首次相同，包含128個全部page／RGB遮罩配對、16個無色彩page組、16個中途物件及32個純ordinary組。朝向、UV、正縮放、半透明模式、非零caller／SDK／scratch與HI／LO同批涵蓋。候選接受範圍審查後於READY前收緊未測packed末尾色彩，保留初版來源；既有ordinary候選結果不變，不因此重跑。正式只修改ordinary時鐘條件、EC620讀取成本與最後ordinary色彩接受範圍，ActorPlan、核心交易API與原版路徑不改。

集中驗收：192組正式完整CPU／RAM／scratch與全部成本，另兩保存態各一次排序512拒絕及packed末尾色彩拒絕。再只開一條正常原片前綴到8382，一個自然未注入基準續跑8383，同入口八個獨立材質／排序／色彩診斷比較完整Machine、成本與排隊音訊，兩個純ordinary案例保留非預設idle DMA2。注入不當自然玩家覆蓋，不回正式路線。原片／原卡保持，舊GPU／美術矩陣不重跑；正式整場GUI／全人物HD／3D與速度仍未完成。


16.7.9限定結果：192組局部原CPU對照、2444952個量子及完整CPU State／RAM／scratch同原版，32組零顯存工作；全部有非零保留值。128個全部page／RGB遮罩配對、16個無色彩page組、16個中途物件及32個純ordinary組同批通過，四個排序512／packed末尾色彩拒絕通過。候選資料／時鐘首次全同；READY前收緊未驗packed末尾接受範圍，保留初稿；唯一正式批次全過。ActorPlan與核心API／原版路徑不改，正式只增高階ordinary資源條件及EC620讀取成本，不解碼原ISA或回播PC／GP0。

唯一正常原片前綴取得8382自然入口，一個未注入基準及續跑8383的完整Machine／原音同。另八個獨立材質頁／排序／逐色通道診斷的全Machine、成本及排隊音訊同原CPU，共95840原控制組指令；兩個純ordinary零工作案例保留DMA2 idle MADR=0x135790、BCR=0x2468、CHCR=0x401。原片／原卡保持，注入副本不回正常路線，不稱自然玩家覆蓋。

本節只封閉ordinary所列分支；其他bank／SDK／旋轉、空幀／隱藏／裁掉物件、bank15特殊複製、文字／HUD／特效／3D與正式整場GUI仍待。026 §16與16.7整體仍DRAFT，後續原生相容光柵仍保留。原版、CPU／GPU、海報、影片、定案美術與Go包保持，新美術／GUI／硬體GPU／打包0，速度未量，不以量子或原指令數推論加速倍率。來源、READY、初稿與正式受測版本、verification／machine-verification／acceptance由私人actor-material-v1索引追溯。


#### 16.7.10 Packed色彩與相對色盤索引

狀態：CONFORMED，沿16.7.1–16.7.9來源／型別資料與原子交易，擴充bank1 packed的entity色彩bit3／RGB遮罩bit4至6及未設CLUT強制bit15的相對索引；允許最後packed及255末槽。cache四頂點色彩仍優先，普通物件分支保持。其他bank／未知SDK／rotation、空幀／隱藏／裁掉物件、bank15特殊複製及正式整場仍未封閉。原DRA／boot／DB、PS1 KSEG0 RAM及DRA offset EA-0x800A0000沿前節；原IDA9.4 bytes／邊界與新分支雜湊由私人actor-packed-material-v1/evidence.json追溯，不重開DB或美術。

已證實EBF0C／10檢查entity+16的強制bit15；清除時EBF18從packed frame+6讀有號半字，EBF20與entity色盤基值相加，再於EBF24／28取CLUT。強制時沿既有低15位索引，資料型別原已完整建立。相對索引限定0至65535，負結果拒絕，不以wrap猜補未知索引；本批覆蓋正偏移及128加-128到0邊界。新增EBF18為RAM讀成本4。

EBF84–C02C的既有cache色彩優先序保持：cache+1B非零直接取四頂點色彩，忽略entity強度／通道。cache色彩關閉才由EC030–040檢查entity bit3；開啟時EC044–080把entity+6C強度填四頂點RGB，再由bit4／5／6選擇四個R／G／B字節是否改128，EC0DC–E4清raw-texture bit。無色彩沿EC0E8–F0，EC0F4共用存回。EC048為entity RAM讀成本4，EC044／94／B8為scratch讀成本0。正式只加入高階條件資源，不解碼原ISA、回播GP0／PC足跡或使用保存輸出產生資料。

兩保存態280候選完整RAM／scratch／CPU State與逐成本首次全同：256組cache來源0／3、中途／255末槽、八RGB遮罩、強制／相對CLUT、上傳／重用；另16組signed CLUT及負XY縮放，八組全部重用。朝向、半透明、非零scratch／SDK／堆疊／S6／S7／HI／LO同批涵蓋。正式只放寬已驗packed最後色彩旗標與相對CLUT守門，補Clock條件及兩RAM讀成本；ActorPlan／核心API／原版路徑與IRQ／跨場／stale守門保持，玩法及原卡不改。

集中驗收：280組正式完整CPU／RAM／scratch及每個成本，兩保存態各一個CLUT負索引與rotation拒絕，共四個。再只開一次正常原片前綴到8382，一個自然未注入基準續跑8383，同入口八個獨立packed材質診斷核對完整Machine、全部成本及排隊音訊；兩組全重用保留非預設idle DMA2。原片／原卡保持，注入不回正常路線、不當自然玩家覆蓋，舊GPU／美術矩陣不重跑。正式GUI／完整HD／其餘圖元與速度仍未完成。


16.7.10限定結果：280組局部原CPU對照、3519084個量子及完整CPU State／RAM／scratch同原版；八組全部圖像重用為零顯存工作。候選資料／時鐘首次全同，READY後唯一正式批次全過，四個CLUT負索引／rotation拒絕通過。cache色彩優先序、entity強度／八通道遮罩、強制／相對CLUT、signed偏移至索引0、朝向／正負縮放、圖像重用及中途／255末槽同批涵蓋。正式只補高階條件及EBF18／EC048兩個RAM成本，ActorPlan／核心API／原版模式不改。

唯一正常原片前綴到8382自然入口，一個未注入基準、續跑8383與所取原音同原版。另八個獨立packed材質診斷的全Machine、全部成本及排隊音訊同原CPU，共93735控制指令；兩個全部重用案例保留非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401。原卡／原片保持，注入不回正式路線，不當自然玩家覆蓋。

本節只封閉bank1的packed色彩與相對CLUT；其餘bank／SDK／旋轉、空幀／隱藏／裁掉物件、bank15特殊複製、文字／HUD／特效／3D及正式整場GUI仍待。026 §16／16.7整體仍DRAFT，後續原生相容光柵仍在。原版、CPU／GPU、海報、影片、定案美術與Go包保持；新美術／GUI／硬體GPU／打包0，未量速度。來源、READY、受測版本及局部／整機收據由私人actor-packed-material-v1索引追溯。


#### 16.7.11 普通末物件之前的隱藏、裁掉與空幀

狀態：CONFORMED，沿16.7.1–16.7.10來源／資料與交易，只擴充最後活躍物件仍為已完整產生圖元的ordinary時，中途entity bit7奇偶隱藏、XY超出原範圍、frame低15位零及ordinary header部件數零。最後活躍物件被略過、全部無圖元、或有無圖元物件而最後為packed仍拒絕；其他bank／SDK／rotation／bank15與正式整場不外推。原DRA／boot／DB、PS1 KSEG0 RAM及DRA offset EA-0x800A0000沿前節，原IDA9.4 bytes／邊界及SHA由私人actor-skips-v1/evidence.json追溯，不重開DB或美術。

已證實EBA74–88保存flags，bit7開啟才走EBA8C–AA8，以slot xor RAM8003C998低bit決定是否跳至EC958；隱藏時不讀座標、幀表或SDK。IDA EBA90的8-byte pseudo-lw包含EBA94實際RAM讀，成本4；EBA8C讀scratch成本0。未隱藏才於EBAAC–AF4求X與Y並寫scratch，X+512以16-bit截斷後大於1024即跳過，否則EBAF8–BB08檢查Y。原範圍包含正負512，正負513裁掉；X剔除的delay slot仍寫Y。EBB0C–1C檢查frame低15位為零時，略過幀表及圖像工作，與frame半字全零在入口就略過的情況不同。

EC1B8–1C8對ordinary部件數零跳EC958，delay slot仍清scratch+4C，不設定S7或呼叫transform／ordinary圖元。既有ActorPlan已略過這些物件；後續完整ordinary會覆寫已封閉的scratch與終值，原架構／SDK資料公式保持。為封閉保留值來源，若typed active數大於產生圖元的不同entity數且末圖元為packed，資料計畫明確拒絕；最後active必須仍等於最後圖元來源，整批無圖元沿舊守門拒絕。空幀Clock只在可安全讀取既有Part範圍的保存來源驗證，不宣稱任意RAM邊界header皆可接受。

高階Clock按原輸入先後執行奇偶、X、Y、frame低位與空部件判定，再生成現有資源區段及一次共同掃描尾段；沒有原ISA／GP0／PC足跡回播。首次162候選完整RAM／scratch／CPU State及全部成本相同，含156個packed來源0／3或ordinary67的奇偶／XY界線／frame低位零與上傳／重用組、四個空ordinary、兩個多重剔除組，非零SDK／scratch／stack／S6／S7／HI／LO保持。測試oracle按原始輸入獨立選可見packed的SDK context group，只供測試，不由ActorPlan輸出選取。

集中驗收：162組正式完整CPU／RAM／scratch與逐成本；兩保存態各驗最後裁掉、最後空ordinary、有略過物件的packed末尾及全空批次拒絕，共八個。只開一次正常原片前綴至8382，一個自然未注入基準續跑8383，同入口八個獨立剔除／空幀診斷核對完整Machine、全部成本及排隊音訊；兩個無上傳案例保留非預設idle DMA2。原卡／原片保持，注入不回正常路線、不當自然玩家覆蓋，舊GPU／美術矩陣不重跑。原版、CPU／GPU、海報及定案美術保持，正式GUI／完整HD／其餘圖元與速度仍未完成。


16.7.11限定結果：162組局部原CPU對照、1738720個量子及完整CPU State／RAM／scratch相同，82組零顯存工作，98組含hidden物件。48組XY正負512界線仍產生圖元，正負513與奇偶隱藏、frame低位零正確略過；四個空ordinary及兩個多重剔除同批通過。候選首次完整全同，READY後唯一正式批次全過；最後裁掉／最後空幀、有略過物件的packed末尾及全空批次共八個拒絕通過。正式只增加高階Clock分支與EBA94 RAM成本4，保留typed資料公式與核心交易API。

唯一正常原片前綴到8382自然入口，一個未注入基準、續跑8383與所取原音同原版。另八個獨立奇偶／X界線／Y裁掉／frame低位零／空ordinary／多重剔除診斷，完整Machine、全部成本與排隊音訊同原CPU，共87096控制指令。兩個無上傳案例保留非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401。原卡／原片保持，注入不回正常路線，不當自然玩家覆蓋。

本節只封閉已完整ordinary末物件之前的略過分支；最後active被略過、全空、略過後packed末尾、其餘bank／SDK／rotation／bank15、文字／HUD／特效／3D與正式整場GUI仍待。026 §16／16.7整體仍DRAFT，後續原生相容光柵保持。CPU／GPU、海報、影片、定案美術及Go包保持，新美術／GUI／硬體GPU／打包0，速度未量。來源、READY、正式版本及局部／整機收據由私人actor-skips-v1索引追溯。


#### 16.7.12 末尾掃描結果與完全無圖元批次

狀態：CONFORMED，沿16.7.1–16.7.11來源與原子交易，擴充最後圖元之後的隱藏／裁掉／frame低位零／空ordinary，以及完全無圖元批次；接入末物件bit7的可見幀。bank／旋轉／bank15／SDK及容量限制沿舊範圍，正式整場仍DRAFT。原DRA／boot／DB、PS1 KSEG0 RAM與DRA offset EA-0x800A0000沿前節，原IDA9.4 prologue、幀表及末尾分支／SHA由私人actor-endpoints-v1/evidence.json追溯，不重開DB或美術。

已證實無圖元仍有prologue及256槽掃描。EB9D8、EBA04、BA0C、BA3C／44的pseudo寫入使AT=1F800000，設定scratch+3=12、+7=3C、+48最後256、+5C及+60緩衝指標，未寫的packet／尺寸／offset沿輸入。callee-save及frame+18最後entity指標仍寫，A0／A1／A2／A3／T0／T2與HI／LO只在其原分支實際寫入時改；無SDK、無transform時其框架保持，不偽造GPU或DMA完成。

末圖元後按原輸入產生typed掃描結果：inactive槽僅改bank／frame半字與V1=1；奇偶隱藏改flags及V1=完整8003C998，不改座標／幀指標。通過奇偶後更新flip／XY，V1為global Y的lhu值，A0保留未截斷的Y兩半字加總；XY剔除或frame低位零按原先後停止，frame檢查另把A0設低15位。可見空ordinary在EBB84把T2設fd+2、scratch+50／4C設0、V1=0；正bank的EBB38 pseudo-lw使AT=800A0000+bank*4，負bank保留先前AT。prefix中的每個可見ordinary header也推導最後counter，讓packed末尾在先前空ordinary後保留0，頁低位仍沿最後實際ordinary圖元或輸入。

資料由typed metadata及既有ActorPlan生成，無圖元只提交必要ABI／scratch／架構資料與裝置時間；不執行MIPS／SDK／GP0或回播PC足跡。Clock及核心API不改。末可繪製物件的bit7只控制入口奇偶，座標／縮放／色彩分支沿既有低位旗標；保留rotation bit2拒絕。第一批128完整資料／CPU／成本首次全同，僅補八個普通末物件可見奇偶案例也首次全同，沒有重跑原128。非零scratch全域與volatile1至12、S6／S7／HI／LO作診斷前態，不假設未寫欄位為零。

集中驗收：正式140組完整CPU／RAM／scratch及全部成本，包括前述136組及四個packed cache來源0／3的末尾可見奇偶邊界。兩保存態各驗rotation、bank15、非零起始圖元計數及全空仍核對SDK來源，共八個拒絕。只開一次正常原片前綴至8382，一個自然未注入基準續跑8383，同入口十個獨立末尾／全空診斷核對完整Machine、成本及排隊音訊；四個無圖元案例保留非預設idle DMA2。原卡／原片保持，注入不回正常路線、不當自然玩家覆蓋，舊GPU／美術矩陣不重跑。CPU／GPU、海報及定案素材保持，正式GUI／整場HD／速度仍未完成。


16.7.12限定結果：正式140組局部原CPU對照、1292325個量子及完整CPU State／RAM／scratch同原版；82組零圖像工作、28組完全無圖元，八個rotation／bank15／起始計數／全空仍核對SDK來源的拒絕通過。先128組候選首次全同，只補八個新普通末物件可見奇偶幀也全同，原128未重跑；READY後唯一正式批次再納入四個packed來源0／3可見奇偶幀。非零整張scratch、volatile1至12、S6／S7與HI／LO驗證保留值；相對Y加總未截斷、正bank AT與空ordinary後packed counter0同原版。Clock／核心API未改，資料由typed metadata直接推導，不回播原ISA／GP0或PC足跡。

唯一正常原片前綴到8382自然入口，一個未注入基準、續跑8383及所取原音同。另十個獨立末尾隱藏／可見奇偶／X裁掉／正bank空幀／空幀後packed／完全無圖元診斷，完整Machine、全部成本及排隊音訊同原CPU，共93421控制指令。四個完全無圖元案例保留非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401；原片／原卡保持，注入不回正常路線、不當自然玩家覆蓋。

本節封閉所列末尾／空批次與奇偶端點；其他bank／SDK／rotation／bank15、文字／HUD／特效／3D及正式整場GUI仍待。026 §16／16.7整體仍DRAFT，後續原生相容光柵保持。CPU／GPU、海報、影片、定案美術與Go包不改，新美術／GUI／硬體GPU／打包0，速度未量。來源、READY、候選補項、正式版本與局部／整機收據由私人actor-endpoints-v1索引追溯。


#### 16.7.13 Packed圖像池的bank選擇

狀態：CONFORMED，沿16.7.1–16.7.12支援的幀／色彩／縮放／重用與交易範圍，擴充packed圖像池的完整原選擇樹；bank15特殊複製、rotation、其他SDK及正式整場仍未封閉。原DRA／boot／DB、PS1 KSEG0 RAM及DRA offset EA-0x800A0000沿前節，原IDA9.4 EBB94–EBC48原bytes／邊界與SHA由私人actor-banks-v1/evidence.json追溯。資料表、來源與payload仍須通過既有RAM範圍／來源／容量守門，不因bank放寬IRQ／stale／跨場限制。

已證實正bank13由EBBE8／F4取8013C004所指圖像池，其餘正bank取固定8013C020。負bank低15位1由EBC08–14取8003C7A0；低15位16在EBC18比較相等後落入EBC20固定8013C020；其他負bank跳EBC34，取8003C7A4後於EBC44載入圖像。新RAM成本位置為EBBEC／EBC0C／EBC38／EBC44，各4；其餘新資源為ALU／分支。

因此先前Rust型別初稿對8010與其餘負bank的來源選擇要訂正：8010走固定池，其他負bank除8001走3C7A4。這些非bank1分支當時被Clock／末物件守門拒絕，未當作正式已驗能力，不以舊Go模型當新oracle。ActorPlan與Part採同一原分支規則，Clock由高階bank分類生成資源區段，不解碼原ISA或回播PC／GP0，SDK／核心交易API不改。

兩保存態240候選完整RAM／scratch／CPU State及全部成本相同：bank2／13／8001／8010／8002分別代表五路，搭配cache來源0／3、中途4／末槽255、上傳／重用及flags0／3／78。診斷只在副本把真實幀表／圖像複製到1F0000／1F0400／1F1000，selected pool在1E0000；未選池為無效指標，clone的origin xor1作路由辨識，像素payload逐位元組保持。每組都用原MIPS獨立執行；這是來源路由診斷，不代表相應角色／關卡自然啟用已驗。測試器首停把header誤當payload，尚未執行CPU對照；以原EBCE4的A1+4訂正斷言後，唯一完成候選全過，候選邏輯未改，首版／日誌保留。

集中驗收：240正式完整CPU／RAM／scratch與每個成本；兩保存態各驗selected pool失效、rotation、bank15及原主routine身份破壞，共八拒絕。只開一次正常原片前綴到8382，一個未注入bank1自然基準續跑8383，同入口十個五路×上傳／重用診斷核對完整Machine、成本及排隊音訊，五個無上傳保留非預設idle DMA2。原卡／原片保持，診斷不回正常路線、不當自然bank覆蓋；舊GPU／美術矩陣不重跑，README最終畫面仍待完整HD接入後更新。


16.7.13限定結果：240正式局部原CPU案例、2086800個量子及完整CPU State／RAM／scratch同原版；120組無上傳，八個selected pool失效／rotation／bank15／原routine身份拒絕通過。正bank13、正bank預設、負bank1、負bank16預設及其他負bank五路皆同原分支，8010／其他負bank的舊型別選擇已訂正。測試器首停把payload位址誤寫成header，CPU比較0；原EBCE4確認+4後只修斷言，保留首版／日誌，唯一完成候選及正式批次全過，未改候選邏輯。

唯一正常原片前綴到8382，一個未注入bank1自然基準、續跑8383及所取原音同。另十個五路×上傳／重用的獨立表映射診斷，完整Machine、全部成本及排隊音訊同原CPU，共84849控制指令；五個重用案例保持非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401。幀表／source clone與origin sentinel只在診斷副本，像素payload保持；原卡／原片保持，副本不回正常路線，不稱自然非bank1或角色／關卡覆蓋。

本節只封閉原bank來源選擇與已驗資料／時間路徑；自然來源載入生命週期、其餘幀／SDK／rotation／bank15、文字／HUD／特效／3D及正式整場仍待。026 §16／16.7整體仍DRAFT，後續原生相容光柵保持。CPU／GPU、海報、影片、定案美術及Go包不改；新美術／GUI／硬體GPU／打包0，速度未量。來源、READY、準備停止、正式版本與局部／整機收據由私人actor-banks-v1索引追溯。


#### 16.7.14 Ordinary bank15的額外圖元

狀態：CONFORMED。沿16.7.1–16.7.13的架構／SDK／同步交易範圍，接普通bank15每個部件的第二份圖元；packed bank15、rotation、容量耗盡、自然來源生命週期與正式整場不在本切片。來源DRA／boot／DB SHA與工具沿16.7.13；PS1 KSEG0 RAM、DRA offset EA-0x800A0000，原IDA9.4 EC788–EC954的原bytes與輸入SHA由私人actor-bank15-v1/evidence.json追溯。

已證實EC7A4在普通bank15第一份圖元後進額外分支，容量未滿時將同一型別部件另作一份：X加20或44減entity0的X，分別依當前entity朝向非零／零；Y加24減entity0的Y，CLUT直接取8003C30C。第二份排序為當前entity排序加416減entity0排序，part_flags的100仍加4。頂點、UV、色彩、混色與材質頁沿第一份，只有列出的座標／CLUT／排序變化；scratch保留第二份資料，主routine正常推進部件與entity。

候選將直接建立兩份Quad，標示duplicate以避免重算兩次transform堆疊／乘積；後續CPU／GPU取樣仍沿既有型別資料，不重播GP0或原MIPS。資料／資源Clock及完整Machine須獨立同原CPU。診斷只映射兩份原五部件幀資料，涵蓋朝向／縮放／色彩、part排序加4、上傳前綴、中途／末槽及後續普通部件；不當正常bank15角色／關卡證據。未知來源／第二排序越界／packed bank15／rotation／容量拒絕，不改玩法或存檔。

READY證據：192個隔離候選首次完整通過，包含兩保存態、96／98兩份原五部件幀、中途／末槽、兩朝向、flags0／3／78、part排序加4及有無packed上傳前綴。完整2MiB RAM、scratch、CPU State及每個成本量子與原CPU相同，共1904992量子。尚未將隔離映射當作自然bank15生命週期。

集中驗收：正式來源重做同192案例，加兩保存態各五拒絕，分別packed bank15、rotation、第二排序超界、容量端點與原routine身份破壞。只開一次正常原片前綴、一個未注入基準續跑8383；八個獨立bank15診斷核對完整Machine／逐成本／排隊音訊，有無上傳各四組；無上傳保持非預設idle DMA2。原片／原卡保持，既有美術／GPU／GUI／包不重跑，不作速度聲明。


16.7.14限定結果：192正式局部案例、1904992個量子，完整CPU State／2MiB RAM／scratch及逐成本同原CPU；96組無顯存上傳、十個packed bank15／rotation／第二排序超界／容量／原routine身份拒絕通過。兩份五部件原幀各直接生成五份額外Quad，保留UV／色彩／混色與材質頁，資料由Rust建立，未執行原ISA／SDK或GP0回播。

唯一正常原片前綴的一個未注入基準續跑8383及原音同；八個獨立表映射診斷全Machine／成本／排隊音訊同原CPU，共79366控制指令。四個無上傳保持非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401，原片／原卡保持。映射不回正常路線，不當自然bank15、角色／關卡生命週期覆蓋。

候選192首次全同。READY文件已寫、收據因唯讀掛載停止，正式來源未安裝、正式案例0；改收據輸出為/output後沿同一候選續接，沒有重跑候選。整機啟動前索引校準成有無上傳各四組，前後hash保存，不涉及production修改。唯一正式局部與整機批次全過。026 §16／16.7整體仍DRAFT；packed bank15、rotation、容量、其他SDK、自然來源、文字／HUD／特效／3D及正式整場仍待，後續原生相容光柵保持。CPU／GPU、海報、影片、定案美術與Go包保持，無新美術／GUI／硬體GPU／打包或速度通過聲明。來源、READY、環境停止、受測來源及集中收據由私人actor-bank15-v1索引追溯。


#### 16.7.15 接續圖元與普通容量端點

狀態：CONFORMED。沿16.7.1–16.7.14已驗資料、朝向、縮放、SDK與同步交易，補非零圖元前綴及普通部件到768上限的停止；packed在滿容量時的超界寫入不在本切片，保持拒絕。來源DRA／boot／原DB SHA與工具沿16.7.13，原IDA9.4主routine SHA與EC1D0–1E4、EC7AC–7B0、EC938–978原bytes／輸入SHA由私人actor-capacity-v1/evidence.json追溯。位址為PS1 KSEG0 RAM，DRA offset EA-0x800A0000。

已證實EC1D0–1E4在每個普通部件開始前以全域圖元數小於768決定是否畫該部件；滿時仍完成entity掃描、必要frame metadata與volatile效果。ordinary bank15第一份使數量到768時，EC7AC直接跳掃描尾端，省略第二份與部件counter／資料指標推進；第二份到768則先完成counter，再由下次普通部件開始時停止。packed原分支沒有同樣容量守門，未證實的超界不能自行當作skip。

Rust型別計畫只生成新圖元，前綴RAM逐位元組保留，排序頭仍接現有桶。前綴由上游型別場景負責，這不授權用GP0重播前綴當主呈現。架構資料依實際新增部件數與停止點推導scratch／volatile／返回，資源Clock以同樣高階容量分類生成成本，不執行原ISA。零新增圖元仍保留原counter、掃描與必要狀態，不能當空操作。

候選以兩保存態、普通／bank15／單packed、前數2／761／763／766／767／768、兩朝向、flags0／3、中途／末槽及有無上傳抽樣。packed只在有剩餘容量時接受，其他越界、rotation、未知SDK／來源、IRQ／過期／跨場守門保持；只診斷RAM副本，不改正式玩法／存檔，不當自然滿容量或關卡證據。GUI／GPU／定案美術不重驗，完整整場仍DRAFT。

READY證據：448隔離候選首次全同，32組無新增圖元、272組無上傳，既有前綴逐位元組保持；完整2MiB RAM／scratch／CPU State及逐成本相同，共3204304量子。原ordinary檢查及bank15第一份／第二份到上限的不同停止點均由獨立原CPU比較，未推廣成packed滿容量或自然場景覆蓋。

集中驗收：正式來源同448案例及兩保存態各五拒絕，共十個超界counter／滿容量packed／rotation／來源失效／原routine身份破壞。只開一次正常原片前綴，一個未注入基準續跑8383與原音；十個獨立容量診斷比較完整Machine／逐成本／排隊音訊及前綴，六組無上傳保留非預設idle DMA2。原片／原卡保持，CPU／GPU、海報、影片與定案美術保持，不重跑GUI／GPU／包／舊美術，不作速度聲明。


16.7.15限定結果：448正式局部案例、3204304個量子，完整CPU State／2MiB RAM／scratch及逐成本同原CPU；272組無上傳、32組無新增圖元，所有前綴逐位元組保持。非零counter接普通／bank15／單packed；普通滿容量仍掃描其後entity並保留metadata；bank15第一份到上限保留未推進的counter／資料指標，第二份到上限則正常推進後停止。十個超界counter／滿容量packed／rotation／失效來源／原routine身份拒絕通過。沒有執行原ISA／SDK／GP0或保存PC足跡回播建立型別資料。

唯一正常原片前綴的一個未注入自然基準續跑8383及原音同；十個獨立RAM診斷全Machine／成本／排隊音訊與前綴同原CPU，共67402控制指令。六個無上傳含兩個無新增圖元，保持非預設idle DMA2 MADR=0x135790、BCR=0x2468、CHCR=0x401。原片／原卡保持，注入不回正常路線、不當自然滿容量或關卡覆蓋；case0未套用診斷variant，該列variant欄位不作自然狀態證據。

候選、正式局部及整機批次各一次通過。此節封閉既有前綴接續與普通容量停止，不包含packed滿容量、rotation、其他SDK／自然來源生命週期。026 §16／16.7整體仍DRAFT，文字／HUD、城堡其餘素材、特效／3D與正式整場仍待，後續原生相容光柵保持。CPU／GPU、海報、影片、定案美術與Go包保持，無新美術／GUI／硬體GPU／打包或速度聲明。來源、READY、受測版本及集中收據由私人actor-capacity-v1索引追溯。


#### 16.7.16 旋轉整數數學與型別幾何

狀態：CONFORMED，限數學／型別幾何。原800EB544的flags4分支由四頂點位移呼叫8001721C、800190AC、80016D68及rsin，位址／原bytes沿既有IDA9.4 next-draw-helper-IDA-v1。這一段須以原固定點查表、捨入及副作用為證據，不能用浮點sin／cos或新近似替代後宣稱同態。輸入BOOT／DRA／原DB、原caller及現行Rust SHA在私人actor-rotation-v1/IDA-plan.json與evidence.json；位址為PS1 KSEG0 RAM，BOOT offset EA-0x80010000+2048、DRA offset EA-0x800A0000。原始資料唯讀，只對一次性DB副本查詢。

本切片先封閉數學資料與型別幾何，整段CPU／GTE／RAM／scratch／快取時間及同步裝置效果另需證明。未達READY前不改正式ActorPlan；即使幾何已驗，ActorDataPlan／Clock仍須明確拒絕尚未封閉的旋轉，不以末物件非旋轉掩蓋中間物件的旋轉。原版模式、CPU／GPU、海報、影片、定案美術、存檔與正式整場契約保持，診斷不當正常玩家或關卡覆蓋。


已證實原BOOT五個函式：1721C平方根148 bytes、190AC求角384、16D68餘弦160、16C9C rsin60與其直接callee16CD8 sin_1 144。原名／bytes／xref與函式SHA保存在SDK-IDA.json及sin-IDA.json，兩次查詢只補直接依賴，原DB保持。查表位於2D3CC共1025個i16正弦四分之一週期、2DBE0共192個i16平方根、32210共1025個i16求角；內容由合法原RAM讀取，長度及SHA在math-evidence.json，不內嵌或公開原表。

READY輸入域：X／Y差值為signed16，求角按絕對值較小／較大之比乘1024取表，再還原象限，包含零向量及-32768。正弦／餘弦保留正負角與4096週期；四角原角先依朝向取負。平方根只接受非負u32，使用leading-zero的偶數部分正規化、192-entry表及原移位，輸出近似sqrt(input)*64；負值的原查表行為未封閉，拒絕。平方和以u32左移12，乘積／加0x80000採32位wrapping，右移20取整，再以16位座標回寫，不能用浮點代替。

平方根的原mtc2修改GTE LZCS30及衍生LZCR31。純數學型別回傳所需LZCS輸入，不直接修改Machine；這不代表GTE原子提交已完成。ActorPlan直接建立旋轉後四頂點，仍生成必要相容資料；ActorDataPlan以rotations標記拒絕整個未封閉呼叫，包含中間物件旋轉、末物件未旋轉，Clock／核心API保持。既有§16.6的旋轉幾何拒絕由本節限域取代，完整狀態拒絕仍保持，舊四拒絕收據只代表當時版本。

READY證據：32786三角、5632平方根、8333求角共46751原CPU數學比較同，平方根30／31及其餘GTE暫存器保持已核對；576個兩保存態×普通／bank15／packed×中途／末槽×朝向×flags4／7／7C×八角度的型別資料／排序／新圖元全部同原CPU，既有前綴保持。兩個未旋轉保存態的完整RAM／scratch／CPU State與逐成本同原版。只少複製既有Clock檔案曾導致建置停止，CPU案例0，補測試器後候選邏輯未改，首份日誌／probe保留。

集中驗收：正式來源同46751數學、576幾何及兩個未旋轉完整資料／時鐘控制；576個完整旋轉拒絕必須保持。四個程式／查表身份破壞及兩個負半徑拒絕共六例。這是原表固定點／型別幾何切片，不是完整旋轉Machine、GUI／GPU、新美術或正常玩家驗收，不新開機、不重播既有美術／整包。完整helper堆疊、GTE效果、HI／LO、volatile、快取／時鐘與同步交易留待後段，整體DRAFT。


原始PS1 KSEG0定位：DRA sub_800EB544的旋轉caller；BOOT sub_8001721C、sub_800190AC、sub_80016D68、rsin 0x80016C9C及sin_1 0x80016CD8。表EA為0x8002D3CC、0x8002DBE0及0x80032210；工具位址與檔案offset的換算沿本節首段，不混用基準。推論等級為已證實的原查表／固定點控制流及本節所列域內輸出，不包含完整呼叫副作用。

16.7.16限定結果：46751正式數學輸出及576旋轉幾何一次通過，全部新圖元／512排序桶／計數同原CPU，前綴保持。兩個未旋轉保存態完整2MiB RAM／scratch／CPU State及逐成本仍同。576個完整旋轉資料計畫拒絕保持，四個來源／表身份破壞及兩負半徑拒絕通過。此切片不推導完整旋轉的helper堆疊、HI／LO、volatile、GTE快取／時間或原子提交；整段旋轉與026 §16仍DRAFT，未註冊正式玩家。

候選建置首停漏複製原Clock，CPU比較0；原日誌／probe保留，補原檔後候選邏輯未改，唯一完成候選及正式批次全過。舊ActorPlan的旋轉幾何拒絕斷言由本節576幾何與完整狀態拒絕接替，舊正案例不重跑。原DB／BOOT／DRA SHA保持；兩次IDA只匯出四個原目標及新發現的直接sin_1依賴，沒有改名或回寫原DB。新正常開機／GUI／硬體GPU／美術／打包0，不作速度或完整HD聲明。來源、READY、準備停止、舊斷言維護、回鏈及集中收據由私人actor-rotation-v1索引追溯。
