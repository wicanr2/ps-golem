# 024 可切換海報與外側融合

狀態：`CONFORMED`（限Linux軟體OpenGL呈現與設定、下述正常路線；Windows／macOS僅編譯，真機待驗）。使用者已核准三案、柔和融合原型，並於2026-10-24要求繼續正式接入。此規格只管呈現，不改機器、鏡頭、HD素材格式或寬螢幕遊戲修改。

## 1. 已確認輸入與證據

- 前端基準`2203f0c`，`frontend/ebiten/game.go`在useFrame鎖內上傳唯讀RGBA，`layout.go`提供4:3與16:9置中，`controller_ui.go`的F1／頂緣工具列及`cmd/psgolem/main.go`／`portable.go`提供原子保存。以上為程式確認。
- 鎖版Ebitengine v2.9.9本機image.go確認DrawRectShader的來源影像須與矩形相同尺寸，SubImage限制目的區。shader使用既有GPU呈現機制，不新增模擬執行緒或每幀PNG解碼。
- 私人素材專案三案原型及34項瀏覽器收據確認1536中央、192兩側、48外側融合；A／B依使用者要求加入瑪利亞，C保持。母稿與圖片只在私人素材repo，通用執行器不含遊戲美術。
- 靜態原PNG1440×1080非黑範圍[0,90,1440,1021)，16/15放大可只裁黑邊。這不是全遊戲黑邊證據；實際每份發布畫面需另守像素邊界，不能外推。

## 2. 資料、設定與失敗

- 外部目錄manifest.json：version=1，posters列1至8項，每項id為1至32個ASCII字母／數字／連字號、name最多64 rune、left及right為同目錄PNG basename。id唯一，PNG不接受符號連結、超出2048×2160或單檔8MiB；合計解碼RGBA不得超過32MiB，manifest不得超過64KiB。
- 載入時完整核對所有項目。繪圖執行緒一次建立全部GPU圖，切換只換完整項目，不能先顯示原版再逐圖補入。
- 新增CLI -posters、-poster、-poster-soft、-poster-zoom。保存可選posters路徑及poster={id,soft,zoom}；id空為關閉。未給欄位的舊設定維持關閉，soft／zoom預設true但不自行啟用。明確CLI優先，路徑沿可攜包相對規則，空-posters可清除。
- 載入失敗記錄原因，遊戲沿原呈現，保留保存的偏好供修復；不載入部分組合。設定保存失敗不套用新偏好，工具列顯示錯誤。

## 3. 呈現與操作

- 非寬螢幕且選中有效海報時，使用置中的16:9裝飾畫布；中央沿4:3顯示比例。寬螢幕修改模式仍走017原路徑，保留海報偏好但不覆蓋實際新增視野。
- zoom開啟時，最多在原4:3置中矩形上等比放大16/15。每張發布RGBA先查上下各ceil(height/32)原像素列的RGB是否全0；任何非黑即以完整4:3顯示，不能裁到HUD或任何非黑像素。此為保守顯示保護，不稱原版畫面邊界識別。
- 海報等比cover裁切，不壓扁人物。soft開啟時海報亮度90%，場景左右窄條鏡射、九點模糊、漸層覆上海報，融合寬度按16:9畫布寬的48/1920，目的SubImage嚴格留在中央外。soft關閉恢復明確接縫及100%海報亮度。中心保持原濾鏡與像素。
- F1／頂緣工具列新增海報循環、柔和、放大按鈕。循環包含關閉與pack項目。操作只影響前端設定，不寫按鍵、不暫停機器；手把設定面板開啟時維持原互動。
- 同一畫面切換或resize亦須重畫；關閉、無pack、寬螢幕時保留既有呈現。海報GPU資源隨Run收尾釋放。

## 4. 垂直鏈與驗收

私人定案母稿→精簡成對PNG及manifest→完整驗證loader→唯讀發布RGBA→前端GPU呈現→工具列→同一settings.json保存／可攜相對路徑→獨立重開。字型沿既有020，不修改遊戲字形。

集中執行loader界線／重複id／壞PNG、明亮HUD阻止zoom、4:3及16:9幾何、關閉／切換／寫檔失敗、保存並保留其他設定、shader編譯與前端既有測試。實際Linux GUI抽樣工具列及正常遊戲，原生RAM／CPU／卡片比較；三平台編譯與真機分開記錄。缺合法原版輸入則skip，不造假BIOS。

停止線：本規格不代表完整背景、人物、怪物、中文或效能完成。失敗保存具體案例，不自行重審美術、重畫或放寬條件。圖片與完整包只在私人素材repo／本機，公開引擎只含通用程式與合成測試。

## 5. 本次接受範圍

前端及命令列40項測試通過，包含載入／路徑／壞PNG、可見像素保護、取消寫檔不切換、設定往返與可攜路徑、shader編譯及真實BIOS一場前後的Digest／卡片不變。既有可選手把截圖fixture未設輸出而skip，改由本批實際視窗與文字像素證據核對新增功能；沒有重跑既有美術或HD GPU矩陣。

Linux正常BIOS／首存卡／既有按鍵腳本讀檔及移動至field8330，HD4、1920×1080、CPU HD、xBR、PGXP與透視選項，海報A／B／C／關閉、柔和及放大切換均保存。獨立重開至field706沿用所有偏好，最後Draw完成，首存卡SHA保持。這不是整段原版CPU逐場對拍、實體GPU或速度驗收。

集中實際截圖發現新按鈕的4px上下inset使20px文字區小於21px字高，原B／C截圖保留。只修一行為水平inset，另外以最終執行檔的真實設定畫面補驗，三個標籤的白色字形像素分別106／62／70。正常路線未重跑，遊戲呈現與其他來源對凍結版本保持；原正常二進位與一行差異另存。

首次GUI準備在建置映像缺XTest而停於玩家啟動前，改沿既有完整顯示映像。原30秒開窗等待在既有材質包載入階段結束；隔離量測同包69.118秒，卡片與空設定未改，確認尚未推進遊戲後才修正檢查期限。標籤補驗原固定180更新等待也未產生截圖，改直接觀察實際字形墨跡；所有停止原文保存，不將環境或驗證腳本停止當成產品通過。

Linux執行檔已補入本機開發測試目錄；Windows amd64及macOS arm64／x86_64 universal編譯與檔案架構核對通過。沒有Windows／macOS真機、新版正式封包、tag或Release，現行完整版保持。私人素材專案的`docs/assets-catalog.md#sotn-side-panels`及其`production-v1/implementation-verification.json`索引原輸入／來源SHA與完整收據，公開執行器不含海報、光碟、BIOS、卡片或截圖。

## 6. Rust海報、工具列與設定遷移

狀態：CONFORMED，限本節Linux軟體SDL功能與下述近似取樣範圍；沿026／027既有Rust玩家視窗的024呈現功能。承接本規格的manifest、32MiB解碼預算、16:9畫布、4:3比例、16/15黑邊保護與九點外側融合，不重畫A／B／C、不新增素材格式。來源是d3e321d的frontend/ebiten/poster.go、layout.go、controller_ui.go及cmd/psgolem/portable.go；Rust SDL2 0.38.0的copy_f／set_scale_mode／set_color_mod已由鎖檔實際原始碼核對。

Rust入口rust/player/src/poster.rs及preferences.rs，main.rs只接前端事件及整張呈現，不改worker／Machine／影片Session。PNG與manifest嚴格沿§2驗證，拒絕未知欄位、錯版本／數量、重複id、非basename、符號連結、錯PNG／尺寸／預算；整包失敗不載部分。所有來源先解碼與建立SDL紋理，切換不逐張補入。PNG來源只取外部目錄，公開引擎沒有遊戲美術。

CLI沿-posters DIR、-poster ID、-poster-soft true|false、-poster-zoom true|false，新增-settings FILE指定同schema的設定檔，測試只用獨立副本。預設定位沿可攜settings.json優先，否則使用者設定目錄ps-golem/settings.json；只讀／更新posters及poster，不改媒體、HD／PGXP、手把、存檔或未知JSON欄位。舊檔無poster保持關閉，soft／zoom預設true。明確CLI優先；可攜／明確-settings的相對路徑以設定檔目錄解析。保存用同目錄唯一暫存檔、完整寫入／sync／原子rename；保存前重讀既有JSON合併，失敗不套用新選項且工具列回報。啟動覆寫若保存失敗回到原有偏好；載包失敗保留原偏好供修復，畫面沿無海報呈現。

F1改為沿020／024顯示或釘住32px工具列，滑鼠只在上緣時自動顯示；這取代026 §14較早原型的F1直接暫停。既有手動暫停改由F2或工具列暫停按鈕觸發，沿相同場邊界機器／Session／音訊凍結，正常PS按鍵不變。海報循環／柔和／放大沿原Go按鈕位置230–324／332–402／410–480，只改呈現及設定，不暫停或送PS按鍵。完整020手把面板另遷移，不放無功能按鈕；中文字沿現有圖集。

渲染沿置中16:9畫布、4:3中央、可證安全才16/15放大，畫布外保持黑色。側圖以浮點目的矩形等比cover及linear取樣，不壓扁；中央維持既有nearest取樣。soft開啟使用最接近90%的SDL 8bit調色230／255，誤差最多一個8bit量化階，不改素材RGB。九點鏡射模糊按Go shader像素中心與Blur=5/sx、band=min(side,max(1,canvasWidth*48/1920))重建，目的嚴格只在中央外。SDL沒有自訂片段shader，本切片由CPU計算兩條RGBA後上傳SDL紋理，CPU／accelerated呈現介面均保留；不稱GPU模糊或GPU影片解碼。新影格／resize／選項變更才更新融合快取，只保留一份，不累積影格。

中央在soft／海報款式切換時逐像素保持；zoom開關造成的等比位置差異分開驗。上下各ceil(height/32)行任一RGB非黑就禁zoom。可選SDL軟體／OpenGL取樣與定點alpha造成的量化差異不能稱Go GPU逐位元相同；Go幾何及來源RGBA可精確比，融合演算法及範圍另列。原版／HD影片均可裝飾，不改鏡頭或遊戲視野；真正寬螢幕修改仍沿017，Rust尚未接此修改。

一次集中驗證：新Rust loader／保存失敗保護與未知欄位往返、Go既有幾何／黑邊／來源RGBA參考、正常原版存檔重開至9000場及短HD LOGO至2600場，完整端點／原音同既有已驗資料。每段只跑一次正常玩家路線，用F2在路線內暫停操作海報A／B／C／關閉、soft／zoom及F1／滑鼠工具列，中央／側邊／保存與原卡分開核對；另一次短獨立重開確認記住路徑及選項。原圖只放私人素材，已定案美術不審查。診斷-report在實際暫停確認時另保存當前來源PNG供同圖比較，不改RAM／PC；正常無report不傾印。Go oracle只讀這份原圖，重建布局／黑邊／已驗資料，不重新啟動原遊戲。

所有工作沿既有有界Docker：Rust GUI工具a5b26f4、執行kol-package 0b933af、Go參考沿既有dosgolem-go-ebiten:1.24-r1的8ed6bdee2bce1f32f804e888f942b25bbf62f2165a43344bb4587f3371de0581。來源及原資料唯讀，唯一輸出在私人direct-go-v1，UID／GID1000；不建立重複映像或操作其他容器。工具及收據由素材docs/assets-catalog.md#render-control索引。完整手把設定、實體設備／GPU、速度、TYK／END與三平台Rust雙擊包不由本段宣稱完成。

§6診斷補充：有-report時只在工具列／選項／暫停／尺寸狀態改變後，於真正present完成後原子更新單份ui.json，供集中驗證觀察；正常無report不寫檔，不進玩家流程文案。Go交叉參考可沿同一kol-package 0b933af內Go1.26.7／Xvfb執行，不另建工具映像。


§6限定結果：Rust視窗已接入定案A／B／C海報、關閉、外側九點柔和融合、安全16/15等比放大及偏好保存。F1／滑鼠上緣顯示工具列，F2或工具列按鈕暫停；切換海報不新增PS按鍵或改機器狀態。正常原版讀檔9000場與HD LOGO至2600場的完整RAM／VRAM／原音及已驗原像素／卡片保持，原版與HD各一次正常路線；同一暫停場切換全部選項、保存失敗不套用及706場獨立重開通過。中央在相同縮放下切換款式／柔和逐像素保持，HD中央完整RGB同來源。其他HD、手把及未知JSON欄位保持。

完整手把設定、實體設備、GPU模糊／影片解碼、其他影片與三平台Rust雙擊包仍待完成，速度未過關。Go與SDL縮放／融合不宣稱逐位元相同；幾何與來源RGBA相同，外側非邊界誤差維持三階，四張原版比較的2283個超出三階位置均落在精確最近點取樣邊界，全部吻合相鄰選樣模型；原始最大差24保留，產品與圖片未改。

八個Rust前端小型案例通過，其中三個新載入／黑邊／保存案例與五個受影響既有案例；Go同圖參考實際渲染13個配置，不執行原遊戲。正常原版至9000場、5139514453週期、PC8001548C，完整端點及原音符合026 §13。HD至2600場、1483757071週期、PC80015494，完整RAM／VRAM／TTY／PCM同026 §11；617場HD、decoder開1次且回收。F2於原版8521與HD 2025場凍結，各425／319次GUI檢查保持field／cycles／音訊消費；全部海報操作同一凍結場。保存失敗不套用、恢復後保存成功，獨立706場重開保留B／soft=false／zoom=true及包路徑。只改posters／poster已知欄位，原HD／手把、額外future_setting及poster.futureEffect保持。

來源與排版已證實：全六張定案PNG為240×1080且不透明，1440×1080中央及安全時1536×1152、1280×960視窗內置中1280×720畫布均符合Go幾何；上下黑邊守門符合Go同源判定。soft／款式切換相對同一Rust無海報基準的中央逐像素保持；Go／SDL本身的nearest縮放邊界差異不等於本功能改變原生RGBA，原始全畫面差異另存，沒有宣稱跨後端中央逐像素相同。

首次接納工具錯將全部外側像素一律要求三階而在native-a-soft最大24停止。查[OpenGL最近／線性取樣](https://raw.githubusercontent.com/KhronosGroup/OpenGL-Refpages/main/gl4/glTexParameter.xml)、[SDL色調公式](https://wiki.libsdl.org/SDL2/SDL_SetTextureColorMod)及固定Ebitengine v2.9.9 internal/graphics/shader.go後，只分析既有圖片：以精確有理數定位九個取樣座標落在整數邊界的點，不用epsilon猜測；四種相鄰貼素組合逐點驗RGB，模型殘差最大2.175，非邊界仍三階。578／582／586／537點分屬原版A／B／C未放大與resize，其他九配置無此超差。已證實是邊界選樣範圍，浮點插值跨界為強推論，不稱特定GPU指令已證實；未改圖、未改產品、遊戲重跑0。sampling-audit.json及acceptance-resolution.json保留首次失敗、模型與工具差異。

Go參考的RunGame主執行緒要求由固定SDK核對後在執行前改為TestMain，初稿／差異在go-oracle-preflight.json保留；實際Go測試首次通過。一般Go測試仍走原m.Run路徑，沒有改Go產品。工具修正前的shell引號問題亦在接納執行前更正並記錄，不將準備錯誤當產品通過或重跑遊戲。

本輪原版／HD軟體功能路線各254.897／59.532秒，含互動暫停與截圖；音訊補零7339560／654304個i16，沒有速度或人耳驗收。本切片CPU計算融合帶再交SDL，保留-present cpu|gpu|auto，不稱GPU模糊或硬體影片解碼。AppImage／.app外層定位與三平台正式封包沿005後續處理。

私人工具verify_rust_posters.py負責prepare／run／accept，audit_rust_poster_sampling.py只分析保存圖片，record_rust_posters.py只產生本機比較頁及授權私人截圖／來源。入口素材docs/assets-catalog.md#render-control，收據direct-go-v1/rust-poster-v1；原版與定案資料唯讀，只有/output可寫。整批正常路線及一次同圖差異定位後定案，美術審查0，沒有重跑核心或舊美術矩陣。Go完整包、原設定與定案素材保持。
