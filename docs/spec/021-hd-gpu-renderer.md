# 021 HD 的 GPU 光柵後端

狀態：READY。2026-10-19 使用者明確要求 HD 繪圖支援 GPU，技術選型由 Codex 查證。
本規格只擴充呈現後端，不改原生 CPU／GPU、時序、RAM、記憶卡或素材格式。

## 1. 依據與範圍

- 已證實：`pinned-parameter-comparison.json` 的正常開機選單取樣，HD drawSpan 佔 CPU
  累計54～57%，開關PGXP都如此。單一參數誤設不是已證實根因，共享主機負載亦影響速度。
- 已證實：現有 `render/hd/poly.go`、`raster.go` 定義覆蓋、固定小數、快取、抖色、混色及遮罩。
  本後端以同版本CPU影子為等價oracle，不新增PS1語意。
- Ebitengine固定v2.9.9，Apache-2.0。依其image.go、shader.go公開API及官方Kage文件：
  https://ebitengine.org/en/documents/shader.html 。只使用既有依賴，不搬移其他模擬器的GPU程式。
- GPU先執行調變、抖色、RGB555/STP混色、遮罩與HD目的寫入。CPU保留命令解碼、
  精確覆蓋與PS1貼圖／CLUT快取準備。這是實際像素繪圖的GPU工作，不是輸出放大濾鏡。
- RGB555混色公式沿用 `render/hd/raster.go`，其上游為 beetle-psx-libretro 的
  `mednafen/psx/gpu_common.h`，commit `e57860d82faa953dc0fc10f201fd583739bc2f7d`，
  GPL-2.0-or-later；來源及相容性入口為 `002-references-and-licensing.md`。
- 原生GPU保持CPU決定性基準。無頭使用不載入Ebitengine；GPU實作分開放 `render/hdgpu/`。

## 2. 整數像素契約

- RGBA8的RG保存RGB555/STP低／高字節，A固定255。輸入A=0是沒有覆蓋。
- Shader不使用一般alpha混色；所有複製用BlendCopy，PS1半透明依既有整數公式算出。
- 色彩插值的32位元固定小數拆成16位元上下半部，再算低半部進位。座標最多8191／4095，
  累加小於2^31，避免浮點尾數或有號乘法溢位。RGB最高8位與CPU模2^32相同。
- 所有65536個原字、STP、四種半透明、mask set/eval、抖色、覆蓋空洞、非零矩形起點、
  倍率1／2／4／8須在真正GPU上下文與CPU獨立公式比較。
- GPU回讀只進HD影子，不能影響Machine、CPU或原生GPU。

## 3. 命令整合與相依性

- CPU影子保存GPU尚未修改的VRAM鏡像；追蹤CPU髒區及GPU髒區。
- GPU工作前先上傳CPU髒區；CPU讀到GPU髒區時同步回讀。
- CPU髒區合併若涵蓋GPU髒區，先同步，避免把過期CPU資料覆蓋GPU結果。
- 三角形／精靈可以把未調變貼素送往GPU批次。自我貼圖、座標環繞、
  側邊畫布及尚未達等價契約的組合走既有CPU路徑，先同步再繪圖。
- 混合CPU／GPU命令的順序、VRAM複製、上傳、快取、CLUT、素材來源與統計須保持。
- 呈現／關閉沿用016 §8.1的一場有界佇列，裝置由影子消費端獨佔。

## 4. 選擇與驗收

- 數值GPU prototype已通過：軟體OpenGL14.652秒；真正Intel MTL OpenGL的完整字值／混色／mask／插值6.41秒、全字恆等及倍率／非零子矩形0.58秒。收據為 workplace/hdtex/perf/gpu-kernel-hardware/gpu-kernel.log，硬體資訊同目錄glxinfo.log。
- 後端提供cpu／gpu／auto選擇並記住設定；舊設定未指定時保留CPU。
  GPU為明確選用的實驗後端。正常讀檔已量到GPU同步成本，速度閘門未通過前不自動取代CPU。
  建立裝置失敗時auto回到CPU並記錄原因，明確要求gpu時報錯，不假稱已啟用GPU。
- 保留CPU後端作無頭、oracle及不支援情形的路徑。既有圖像與操作設定不變。
- 正式驗收包含公開GPU語意及混合命令、完整影子VRAM／快取／來源對照、
  真BIOS正常讀檔的CPU／RAM摘要及畫面、真Intel GPU GUI與速度、三平台建置。
- 同狀態像素不符先定位並修正或保守回退，不以換oracle／放寬誤差通過。
- GPU支援、速度通過、完整HD美術與通關是分開的完成狀態。


### 4.1 已審查的接線細節

- GPU工作以三角形或精靈的有界矩形為單位。CPU準備精確覆蓋及未調變貼素，
  GPU以同樣32位元插值完成HD色彩與目的寫入。暫存影像依容量重用。
- 目前保守回退包含自我貼圖、座標環繞、側邊畫布跨界及mask eval。
  CPU回退先同步GPU；GPU結果不回到原生GPU。
- GPU髒區只影響HD鏡像。CPU合併髒區若會覆蓋GPU尚未回讀區域，先上傳舊CPU髒區、
  回讀GPU，再執行CPU寫入。複製及CPU光柵化先全同步。
- Screen回讀HD VRAM後沿用既有RGB24／RGB555及側畫布呈現。GPU支援不等於零回讀。
- field Fence以有界GPU提交點維持既有一場CPU命令佇列；關閉釋放裝置，不留下背景執行緒。
- fallback計數及GPU工作量只供呈現診斷，不納入原生Digest或存檔。

- 關窗及-frames結束先在圖形主迴圈內非同步要求停止並等GPU工作排空，再結束RunGame，避免主迴圈停止後才等回讀。

- 已查明Ebitengine v2.9.9 internal/ui/ui.go 的readPixels在frame之外會等到下一frame。
  裝置的Upload／Draw／Read／Flush改以容量1的同步請求佇列送進Game.Update；
  一次服務到Flush或25ms預算，CPU後端及暫停不等待空佇列。
  Screen及VRAM快照結尾送Flush，所有回讀在有效圖形frame內完成。
  呼叫端等待回覆後才重用資料切片。這是呈現排程，不改模擬時鐘。

- 圖形主迴圈結束時Cancel釋放尚在等候的GPU呼叫，避免圖形裝置錯誤後halt等不到回覆。只影響正在關閉的HD影子，不把未回讀內容寫回原生狀態。


### 4.2 GPU提交與像素同步分開

- 初版每場Flush回讀一像素，會把完整GPU工作及先前呈現一起同步，非必要。
  CPU取樣與服務佇列計數保存在gpu-profile-v2/、gpu-service-profile/。
- Flush只在Update服務佇列標記本場提交結束，不做GPU回讀。
  這仍限制每次UI服務的CPU命令工作，GPU硬體佇列由Ebitengine管理。
- Read為真正的同步點，必須先完成先前GPU工作才把值交回HD鏡像。
  原生Machine不使用這些值；Screen／VRAMCopy仍由Read保證像素就緒。
- 必須重跑混合CPU／GPU回讀、正常玩家路線與關窗；不能只看佇列計數宣稱速度通過。

- VRAM與input／background／output工作影像明確使用NewImageOptions.Unmanaged，
  避免反覆在來源與目的切換的工作紋理被放入自動圖集。來源為Ebitengine2.9.9 image.go
  NewImageOptions的公開契約；仍須全字值與混合命令GPU驗證。

## 5. 實作與驗證範圍

狀態維持READY。GPU功能已接入並通過下列檢查，速度閘門尚未通過。
總收據為 `workplace/hdtex/perf/gpu-backend-verification.json`，原版衍生證據只留本機。

| 項目 | 結果與證據 |
|---|---|
| 真正GPU像素 | Intel MTL OpenGL通過65536字、半透明、遮罩、抖色、固定小數、倍率與非零子矩形，`gpu-command-v3-hardware/gpu-kernel.log` |
| 混合命令 | 30組倍率／混色／側畫布、4／8／15位元貼圖、PGXP、翻轉、CPU回退、回讀與VRAM複製；比較完整HD VRAM及輸出。最後補測GPU新寫區的來源／目的重疊，`gpu-command-final-race/race.log` |
| 素材與內部狀態 | 索引及全彩RGBA＋STP遮罩均實際命中；透明、實色、半透明黑色與CPU相同。記憶體裝置另比對整個Renderer、快取、來源、參照數及傾印狀態。真正GPU與HD狀態race均通過 |
| 正常玩家路徑 | 真BIOS／光碟／既有M4記憶卡正常開機、讀檔、移動、攻擊至field9000；15組原生摘要及HD像素全部相同，卡片與原生畫面相同。`gpu-normal-verification.json` |
| 真正GPU工作量 | 上列路徑138610個三角形、636791個精靈、2935327782個HD像素；CPU回退132234次、回讀19359次，沒有宣稱所有工作都已交給GPU |
| Linux候選包 | v.0.7.0-20261019從保存的GPU設定自動找素材；幀數180正常完成且final_drawn=true，重開後field600正常WM_DELETE關窗並排空GPU。`gpu-package-verification.json` |
| 三平台 | Linux amd64、Windows amd64及macOS universal建置，封包、版號、SHA-256、空素材目錄、UTF-8與macOS結構通過。Windows／macOS GPU尚未真機驗證 |

9000fields證據使用研究小視窗，每場擷取、取消垂直同步；不是正式遊戲GUI的速度驗收。
功能重播期間另有建置與短驗證，不拿其wall-clock宣稱效能A/B。首次420秒逾時保存為
`gpu-normal-v3-timeout.json`，12組已完成檢查點相同；後續提高有界時限，完整重跑退出0。

通用程式未指定後端時保留CPU。一次指定 `-hd-renderer gpu` 或保存
`"hd_renderer": "gpu"` 後，雙擊沿用GPU。`auto`只在建立裝置失敗時回到CPU；
執行中的驅動錯誤仍回報錯誤，不宣稱能無縫切換。
素材專案的 `tools/assemble_local_full.sh <完整版號> [cpu|gpu|auto]` 可選本機包後端，
省略時CPU。GPU候選不取代現行v.0.6.0-20261018。

尚未完成：CPU／GPU同步成本的速度改善、正式GUI的GPU完整首存、人工音訊、
Windows／macOS GPU真機、完整HD美術及通關。這些不因像素一致或三平台能編譯而完成。


### 5.1 原生相容角色的後續分工

NATIVE-PLANE-ROLE-20261026：以上『GPU回讀只進HD影子』限定一般HD呈現角色。後續[025 §11正式原生相容平面](025-hd-primary-native-demand.md#11-正式原生相容平面接線)另定無美術、無PGXP的一倍NativePlane，由精確原始命令生成RGB555／STP及快取，供原生觀測與讀回。該角色可以使用CPU或獨立GPU裝置，不能從HD新稿反推原生字。此為025 §11的獨立CONFORMED切片，不將021的呈現隔離契約擴寫成所有GPU裝置永遠不得供應原生資料。一般HD主影子仍維持原隔離與既有回退。
