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
