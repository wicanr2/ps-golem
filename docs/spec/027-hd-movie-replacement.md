# 027 HD影片替換與H.264播放

狀態：整體DRAFT。使用者2026-10-28明示「改播放器，用H.264及AI提升畫質，播放時以高畫質影片取代」，決定已確認。離線來源與編碼原型已驗；§5 CPU拉取播放器及完整原生顯示事件鏈限定通過，正式播放攔截、前端與連續AI影片尚未完成。Rust核心移植沿026，MDEC原生相容沿011，不把本原型稱為遊戲影片已替換。

## 1. 已確認的呈現方向

- 原版模式保留原片／MDEC呈現；HD模式在影片開始前選定外部HD版本並替換顯示。未安裝對應影片時沿原版相容來源，不能原版先閃一格再換HD。
- 高畫質素材離線製作，播放時解碼H.264，不即時呼叫AI。至少1080像素高，保留原構圖、角色、鏡頭與片長；目前4:3來源以1440×1080測試，交由既有024海報組合到16:9，不自行裁切／拉寬影片。
- 承接已核准的HD美術風格與精簡包策略。imagegen可製作重繪參考格；整段AI增強必須處理時間一致性，不將逐張無關重繪或單純Lanczos放大稱為完整AI影片。
- 先保留15fps原始時間與音訊，不自動插格或改台詞。播放開始、跳過、暫停、結束及回到遊戲須跟遊戲事件一致。CPU／GPU解碼路徑保持目標，未實測硬體解碼時不得宣稱已支援。

## 2. 已證實的來源與離線原型

唯讀來源為相鄰私人素材的workplace/assets/raw-sectors/*.STR.raw，已由既有extract_assets.py依光碟ISO extent抽取；本批另核對來源SHA。四個根檔與LBA為LOGO15XA.STR 23104、OPN_WS.STR 13864、TYK_WS.STR 8308、END_WS.STR 870。位址基準是光碟LBA，非CPU PC或影片PTS。

既有kol-package:1.26.7-ebiten2.9.9-20261011-r1映像固定SHA256 0b933af1238ab5839ad7a64b6ae05f069f57cb85da5917dfbe59730f6d7ba53f，FFmpeg 5.1.9-0+deb12u1。實測具有psxstr、mdec、adpcm_xa、libx264及H.264解碼，CPU原型已跑；列出cuda／vaapi等編譯介面不等於硬體驗收。

四段探測均為320×240、15fps、37800Hz雙聲道XA。完整解碼計數OPN 922、TYK 555、END 743格；LOGO出現ac-tex damaged，未取得完整格數。這是FFmpeg解碼相容缺口，不能稱原素材壞掉或略過驗證；LOGO已由§5的既有正常玩家原生繪圖封存取得完整156格並編碼；外部STR解碼錯誤保留，未重跑遊戲或舊美術。

本批OPN第2至5秒以Lanczos製作1440×1080、15fps、45格的H.264/yuv420p MP4，無音軌、2368721 bytes，完整解碼無錯。這只驗傳輸格式，尚無AI美化或遊戲接入。另以TYK第45格作一次imagegen重繪，保存4:3哥德城堡參考；它是單格參考，不能當成已產生連續動畫或時間一致性通過。

來源、命令、完整SHA與離線頁面都沿私人素材既有workplace/render-control-v1/direct-go-v1的hd-movie-*保存。工具入口為該素材tools/prepare_hd_movie.py及record_hd_movie.py，由docs/assets-catalog.md#render-control索引。沒有原資料／影片／AI參考圖加入公開Git。

技術來源：[FFmpeg psxstr demuxer](https://github.com/FFmpeg/FFmpeg/blob/n5.1.9/libavformat/psxstr.c)、[FFmpeg編解碼文件](https://ffmpeg.org/ffmpeg-codecs.html#libx264_002c-libx264rgb)、[FFmpeg相依與授權](https://ffmpeg.org/general.html#External-libraries)。本原型只呼叫既有FFmpeg，未移植其程式；其封裝與跨平台解碼相依仍須在正式播放器READY前固定。

## 3. 正式播放器待證實的契約

此節DRAFT，禁止以未知事件或常數接正式玩家路徑。下一個窄研究為正常影片的開始／停止／跳過事件、MDEC整格／切片與顯示提交、來源STR／時間戳對應。遊戲識別與攔截放apps/sotn，作品中立的有界解碼／幀佇列／呈現放通用影片層。

H.264時基跟隨遊戲虛擬時鐘或已驗影片格事件，不能以主機wall-clock讓影片跑完而遊戲尚未完成。音訊先承接SPU／XA，不再疊播MP4音軌；獨立預覽可另做有聲包，但不順帶更換遊戲音源。高畫質只影響呈現，原版必要RAM／VRAM／IRQ與等待副作用保持，不能因播放器存在就刪除MDEC／DMA；若能省略某段原生工作，須另有資料流證據與READY契約。

載入前驗來源與影片SHA、尺寸、格率／PTS、有效範圍及解碼能力。正式模式切換沿完整呈現場／影片邊界，啟動時預備首格，佇列與記憶體有界。影片包及版本／映射格式尚未定案，不修改019既有HD材質manifest或存檔格式。原生CPU／RAM同態、跳過／暫停／回到遊戲與檔案故障結果須用正常路徑驗證，不能由離線播放器取代。

## 4. 驗證與停止線

本批只接納一次離線傳輸檢查與一張重繪參考，不重開已核准美術。後續先做短連續AI片段，集中檢查構圖／角色／文字、前後格形狀與亮度、片長／PTS／聲畫同步、完整可解碼性及容量。通過再批次製作，其後依已定美術策略最多兩輪，不為用滿次數重審。

正式接入另驗原版／HD同存檔與等價輸入、開始／跳過／結束／暫停、海報／工具列、CPU／GPU及實際速度。Linux離線檔可解碼不等於Windows／macOS交付、GPU功能或月下可玩。所有原片衍生影片與AI母稿保持本機，不加入公開引擎或一般發行包；包裝沿005／019既定本機範圍。

## 5. Rust CPU播放器切片

狀態：CONFORMED，限程式化CPU解碼／取幀與已保存顯示事件的離線接線；兩個必要案例首次通過，156個完整RGB幀符合獨立CRC參考。未含正式前端、素材包自動辨識或完整AI動畫。使用者已核准H.264方向，本節只實作其可獨立驗證的播放器介面，不修改既有HD材質schema或原生機器狀態。

來源證據為既有正常玩家commands.u64le.gz，SHA256 d78b7256678a566e349abc97139006ce6bc0dd412733d21dc69ff9e0b93606a2。本批一次GPU重播到field9500，51471448筆原始記錄，17360筆24×240切片上傳；每20片完整後才接受320×240雙緩衝顯示。第一段在field1716進24-bit、1720首格、2333退出，156格；來源影像目視為KONAMI標誌片，以管線編成1440×1080 H.264。此156是本次完整顯示鏈，011較早143格是當時抽樣，不互相覆寫。第二段field6691開始，到9500仍在播放，709格，完整片名／結束映射尚未知。

公開Rust來源在rust/src/movie.rs；使用系統明確指定的FFmpeg／ffprobe工具路徑，以參數陣列啟動、不用shell。先probe H.264、尺寸／格率／格數；只接受有界大小及正整數格數。CPU輸出RGB24，每次拉取一格並只快取一格，前進時丟棄不需的過渡格；同格重取或暫停保持快取，不隨wall-clock前進。原生顯示事件提供格號，禁止用讀碟進度當呈現格號。

播放器begin預讀首格；present接受單調格號，越界／倒退／部分幀／子程序錯誤回報具體錯誤，不把錯誤冒充成功。pause只凍結播放器快取，stop／Drop必須kill並wait解碼子程序且可重複；不影響模擬時鐘／音訊。影片開始前準備首格的正式遊戲接點仍DRAFT，本API的離線正確性不能當作前端已無閃爍。

驗收只做一次集中切片：以同一FFmpeg獨立解碼原生來源H.264，逐格產生CRC32／來源顯示field參考，再由Rust播放器以這156次事件拉取全部RGB幀；同格重取／pause不前進，stop後不再取圖且子程序已回收。另驗原有45格編碼樣片的前進跳讀、檔案錯誤及越界／倒退；CRC32只作測試比對，不作正式素材身份或權利驗證。SHA256與版本來源由私人hd-movie-player-plan-v1.json保存。

本機使用既有FFmpeg5.1.9／kol-package映像，Rust用026 §7.1既有1.99.1映像。這個CPU切片不宣稱GPU解碼、三平台包、月下正常跳過／暫停或自動替換通過；影片包格式、STR／遊戲控制狀態與正式入口沿§3，仍DRAFT。影像只是原生來源的高解析傳輸版，尚無AI美化；已核准城堡參考與後續時間一致性製作保持。

§5界限：單一檔案最多256MiB、寬4096／高2160且總像素不超過3840×2160、格數1至100000、格率正有理數不超過120fps。這是程式化解碼器邊界，HD素材品質與正式包仍依§1／§3；目前只驗1440×1080的15fps來源。工具路徑由呼叫者明確傳入，不要求主機安裝工具或新增相依。


§5結果：rust/src/movie.rs的Player::open明確接受FFmpeg／ffprobe／MP4路徑並預讀首格；present(index)跟外部顯示事件前進，set_paused／current／stop提供程式化控制，Drop回收子程序。PSGOLEM_RUST_MOVIE_FIXTURES指定私人direct-go-v1，另以PSGOLEM_RUST_FFMPEG／PSGOLEM_RUST_FFPROBE指定容器內工具。在026既有Rust容器build --lib --no-run後，以本節固定FFmpeg容器執行測試檔movie::tests；缺來源則明列SKIP，不把跳過算通過。

首次兩案例通過：156個來源顯示事件逐完整RGB幀同獨立解碼CRC32；每格重取／pause不前進，45格樣片前進跳讀到20格及提前stop、缺檔／越界／倒退和重複stop均通過。只快取一格RGB，沒有不受控影片格佇列。原生KONAMI傳輸版1440×1080、15fps、156格、740355 bytes，不是AI美化版。GPU硬體解碼／正式前端／遊戲內跳過／暫停／自動影片識別及素材包仍未驗；第二段播放只觀測到field9500，不能稱自然結束已驗。

IDA9.4在原DB的一次性副本查得936函式、StSetStream的原始指令及資料引用；沒有由名稱宣稱遊戲控制語意，STR映射與讀寫端仍未知。來源與首次結果沿私人hd-movie-event-replay-v1.json、hd-movie-player-execution-v1.json及hd-movie-player-summary-v1.json；Rust CPU／GTE／GPU／Machine原始碼與SPU批次相同，未重跑舊矩陣。正式接線回到§3補證據，本節不代替正常玩家／GUI驗收。


## 6. 原音保存與有聲預覽

狀態：CONFORMED，限原音保存與有聲獨立預覽。使用者2026-10-28回報沒有聲音並要求影片補聲音。已證實現有OPN第2至5秒H.264沒有音軌，頁面另帶muted；Rust §5只解影像，實體聲音輸出及正式遊戲接線未完成。正式遊戲承接§3原SPU／XA單一路徑，禁止同時疊播MP4音軌。離線預覽須帶原音，預設不靜音，由使用者按播放。

原始四段STR及SHA沿§2與私人hd-movie-inventory-v1.json。FFmpeg5.1.9容器沿§2，原始扇區唯讀。先核對2352-byte同步／Mode2及雙份subheader，音訊檔號／通道1／1、coding01，4-bit／37800Hz雙聲道，每扇區18組×28取樣×4雙聲道單位等於2016組取樣。首次唯讀探測LOGO193、OPN1155、TYK684、END879個音訊扇區，解碼組數均等於扇區數×2016，均非靜音。來源尾端FFmpeg回報Input/output error而退出碼0，不能只憑退出碼通過。[FFmpeg5.1 psxstr原始碼](https://www.ffmpeg.org/doxygen/5.1/psxstr_8c_source.html)在不足2352-byte讀取回EIO；只在完整扇區與取樣數核對後將單一完全相符的檔尾訊息分類為demuxer EOF限制，保留原stderr，其餘訊息失敗即停止。

工具為私人素材tools/prepare_hd_movie_audio.py，入口docs/assets-catalog.md#render-control。四音源存本機FLAC，維持37800Hz／雙聲道／16-bit，完整解碼後與原XA PCM逐位元相同，不調音量、不改詞。獨立OPN樣片沿原剪輯2至5秒，複製45格H.264而不重編影像；同段原音轉44100Hz雙聲道AAC192k封入MP4，供網頁播放。3秒有聲預覽的聲畫起點與長度誤差不超過一個44100Hz取樣，影像packet雜湊不變。完整音畫解碼無錯、PCM非靜音；與原STR同段重取樣參考做零偏移相關度至少0.98及SNR至少15dB，僅作有損轉碼驗證，不稱硬體音訊parity。AAC末包padding另列，播放以MP4 duration／priming為準。

KONAMI原生156格與LOGO原檔168格尚無逐格映射證據，不能把10.293秒原音猜貼到10.4秒原生影片；先保存其完整音源，原生影像參考標明無聲／待同步。這項限制不阻塞已有確切同源剪輯的OPN有聲預覽。正式同步仍需正常遊戲開始／跳過／暫停／結束與回遊戲事件；本批不宣稱Rust聲音裝置、完整AI影片或GPU播放器完成。

一次集中驗證包含四段完整PCM數量／非靜音／FLAC無損往返、有聲樣片的格數／聲道／PTS／完整解碼／音訊相關度，之後記錄結果，不重跑已通過美術或舊核心矩陣。原音、FLAC、MP4及收據只留既有私人direct-go-v1，不加入公開Git或一般發行包。


§6結果：四原音完整解碼並保存FLAC，LOGO389088、OPN2328480、TYK1378944、END1772064組雙聲道取樣，均37800Hz／16-bit，全部FLAC往返PCM逐位元相同且非靜音。FFmpeg5.1檔尾EIO訊息原樣保存，完整扇區與取樣數相符；未隱藏錯誤或當作原片損壞。

有聲OPN預覽hd-movie-h264-with-audio-v1.mp4共2441893 bytes，1440×1080／15fps／45格，加44100Hz雙聲道AAC。音畫起點0、片長3秒，影像packet SHA不變；完整解碼無錯，原STR同段零偏移相關度0.9996519843530091、SNR31.5392dB，3秒264600個左右聲道取樣中264571個非零。解碼器暴露820組尾端AAC padding，與MP4標示3秒有效範圍分列，不延長正式播放。頁面移除muted與loop，新增四段原音控制項，無聲原生影像移入明確標示的參考區。

限定結果沿私人hd-movie-audio-plan-v1.json／hd-movie-audio-v1-summary.json／hd-movie-audio-acceptance-v1.json，工具prepare_hd_movie_audio.py及record_hd_movie_audio.py由素材索引連入。正式遊戲聲音裝置與時序、KONAMI逐格映射、整段AI影片未完成；本批無人耳／硬體音訊驗收，無Rust核心變更或舊測試重跑。


## 7. 已核准AI單格的短連續細節傳遞原型

狀態：DRAFT，可丟棄離線研究，禁止接正式播放器。承接§4的短片停止線與已核准TYK第45格AI母稿，僅取原片第30至59格，即2至4秒、30格／15fps。以原生格之間的運動估計，把同一母稿的新增細節傳至鄰格，不重畫已定案美術，不把獨立逐格改稿當時間一致性。

先沿既有Pillow11.3.0工具映像，實測三個現有映像均無cv2。只在相同映像／相同Python ABI的明確私人輸出根安裝鎖版numpy2.2.6及opencv-python-headless4.11.0.86輪檔，不掛主機Python或虛擬環境，不改系統或重建重複Docker映像。下載僅走官方PyPI，逐檔SHA與版本JSON核對；後續處理網路關閉，原資料唯讀、工作有界，來源與執行manifest只留本機。

[OpenCV4.11 DIS光流](https://docs.opencv.org/4.11.0/de/d4f/classcv_1_1DISOpticalFlow.html)及[remap](https://docs.opencv.org/4.11.0/da/d54/group__imgproc__transform.html)只作運動估計與幾何取樣，AI內容來自既有imagegen母稿，不能把光流稱生成模型。以目標原格到第45格的逆向光流配合正反一致性與原格亮度誤差，低信心處回原生放大；保留15fps、1440×1080與同段原音。這種單參考方法未證實可處理遮擋、切鏡或所有電影，不能批次套到整段。

本原型只集中檢查30格、格率／片長／PTS／原音／解碼、細節保留與畫面變形／亮度跳動，數值與一次影像抽樣分開記錄。若不足就如實記錄限制，不自動反覆改圖或擴大審查；原型不等於整段HD影片、完整美術或正式遊戲接線。工具入口私人素材docs/assets-catalog.md#render-control的prepare_hd_movie_flow.py及後續原型腳本；輪檔／AI母稿／逐格資料／MP4及收據沿既有direct-go-v1，不入公開Git或發行包。


§7首次原型結果：已核准的第45格母稿沒有重畫，對原片第30至59格做一次光流細節傳遞，保留30格／15fps／2秒／1440×1080。固定母格與縮至目標尺寸的AI母稿逐像素相同；低信心區回原生。每格AI權重至少半數的像素占比，最低0.4334505208，不能稱整個畫面全AI重繪。逐格運動補償RGB平均差最高3.6501，但一次抽樣30／45／59格仍發現塔尖、屋脊彎折；數值低不能當幾何／藝術通過。

有聲H.264研究版1177341 bytes，同段原片放大對照1065875 bytes；兩者皆30格、起點0／片長2秒，完整音畫解碼無錯，44100Hz雙聲道AAC。AI版原音與同段來源零偏移相關度0.9995662291、SNR30.5737dB。技術封裝通過，建築幾何閘門未通過，§7維持DRAFT，禁止自動批次處理整片或安裝遊戲。下一窄問題是為剛性建築加幾何約束，母稿保持，不再重開美術。這是一次研究檢查，沒有為用滿審查次數重跑。

輪檔SHA／Python ABI與來源沿私人hd-flow-runtime-v1.json，光流／逐格／編碼原始結果hd-flow-plan-v1.json、hd-flow-propagation-v1.json、hd-flow-encoded-v1.json，完整限定紀錄hd-flow-summary-v1.json。工具prepare_hd_movie_flow.py、propagate_hd_movie_flow.py、encode_hd_movie_flow.py、record_hd_movie_flow.py由素材docs/assets-catalog.md#render-control索引。只有私人預覽提供同段對照及原音；原片、母稿、輪檔、逐格資料與MP4均未公開，沒有正式HD影片或播放器接入完成聲明。


## 8. 完整離線候選與硬質物體幾何

狀態：DRAFT，離線製作候選，未接正式播放器。使用者2026-10-29更新優先順序：先完成影片HD，再集中確認合理性，並回報兩秒片段右下角岩石晃動。本條取代§7的先逐次短片審查再批次停止線，允許先產出完整候選；既有失敗保留，沒有把未驗畫質改成通過，也不重畫已核准母稿。

來源仍為§2四段STR及§6完整原音，時間戳基準是FFmpeg解出的原片PTS，非CPU位址。OPN 922格／61.466667秒、TYK555格／37秒、END743格／49.533333秒，音軌分別61.6／36.48／46.88秒，起點均0。保留各軌原長度，不以shortest刪音、伸縮配音或新增背景音樂。LOGO外部解碼失敗與原生156格的來源映射仍未知，保持明列，禁止猜貼音訊當正式完成。

完整三段候選採官方Real-ESRGAN v0.2.5.0發行內realesr-animevideov3-x4神經超解析模型，原320×240先重建1280×960，再輸出1440×1080、15fps H.264及原音AAC。這是神經超解析候選，不能稱每格都經imagegen重繪。已核准城堡母稿另用在同一廣角鏡頭的城堡／岩層；以原片點對應估計單一相似變換，整塊建築與岩石共用同一平滑相機軌跡，不以逐像素光流扭曲岩石。雲、水等動態來源沿神經超解析，切鏡不用上一鏡的母稿。幾何與全片藝術合理性待完整產物一次集中檢查，不能由相似矩陣或數值推定肉眼已通過。

上游工具參考來源為Real-ESRGAN-ncnn-vulkan在發行日期前的固定commit 37026f49824c5cf84062e7c6a5dd71445dcf610f，MIT；Real-ESRGAN模型專案BSD-3-Clause。官方預編譯發行未明列其精確建置commit，不能由參考來源日期推定二者完全相同；固定官方發行及實際二進位／模型SHA。官方預編譯zip SHA256 e5aa6eb131234b87c0c51f82b89390f5e3e642b7b70f2b9bbe95b6a285a40c96，來源／授權全文／模型逐檔SHA保存私人hd-movie-ncnn-source-v2.json與後續製作收據。本專案只呼叫工具，沒有移植其程式；發行包相依另驗。

[官方影片模型與固定發行](https://github.com/xinntao/Real-ESRGAN/releases/tag/v0.2.5.0)、[NCNN工具與MIT](https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/tree/37026f49824c5cf84062e7c6a5dd71445dcf610f)、[OpenCV 4.11相似變換估計](https://docs.opencv.org/4.11.0/d9/d0c/group__calib3d.html)、[仿射取樣](https://docs.opencv.org/4.11.0/da/d54/group__imgproc__transform.html)。光流僅估計點對應，不作硬質物體的逐像素變形。

工具入口私人素材docs/assets-catalog.md#render-control的prepare_hd_movie_full.py及tools/Dockerfile.hd-movie-vulkan。既有bookworm Mesa無法枚舉本機Intel 0x7d45；沿用既有Ubuntu runtime，只補固定版Mesa／OpenMP，保存原映像與套件版本，不修改其他專案映像。Ada連線逾時不當成Ada驗收；本機GPU離線推論也不等於遊戲GPU解碼或FPS。

原始輸入唯讀、有界Docker、私人direct-go-v1可寫。一次集中技術檢查為完整格數／尺寸／15fps／各軌PTS與原長度、完整解碼、全段原音非靜音及轉碼相關度、輸出SHA／容量；合理性依使用者順序留待全片完成，不預先宣稱美術通過。候選只放本機研究預覽，非發行包，不修改019素材manifest、存檔、原版模式或正式玩家路徑。


§8離線製作結果：完整離線HD候選已產出：開場922格、城堡555格、結尾743格，另有正常播放封存的KONAMI候選，其617場畫面摘要全部對回156格來源及初始畫面。皆1440×1080並含原音，合計52.70MiB。前段城堡／岩層共85格改用同一平滑相似變換，取消逐像素扭曲；其餘畫面為官方影片模型AI超解析。這不等於每格imagegen重繪。依使用者順序，完整候選的藝術／運動合理性稍後集中確認，尚未宣稱岩石肉眼驗收通過。KONAMI依已保存VBlank時間線作可變格率候選，保留同段原音，原STR168格映射仍未知。正式遊戲HD自動換片／GPU播放器／Rust GUI仍未接入。

四段完整解碼／尺寸／格數／各軌原長度與起點檢查通過，三段原STR保留15fps；原音完整AAC轉碼相關度均大於0.98、SNR大於15dB。這是本次一次集中技術驗證，不含新藝術審查、實際硬體音訊、遊戲開始／跳過／結束或GPU解碼驗收。逐片雜湊、音訊值與容量沿私人hd-movie-full-summary-v2.json；母稿及全部媒體只留本機，沒有發行包或素材公開。

KONAMI新增來源是§5同一正常玩家命令封存重取的156格無損影像，全部RGBA SHA與已驗事件相同，沒有新遊戲開機。再以保留的初始畫面及逐場FNV摘要，617場全部對回026 §11正常播放來源，依該場cycles／33868800建立可變格率時間線，沿相同VBlank原音區間。H.264每個影像轉換時間以1/90000秒保存，不插光流中間格；最後同圖哨兵確保末格時長，整體影像時長與來源誤差小於一組44100Hz取樣。這是VBlank觀測精度，不稱逐CPU指令或原STR168格映射已證實。§5原CPU播放器仍為索引／固定格率介面，此新VFR候選未接正式播放器，後續事件介面回到§3處理。

首次LOGO封裝的concat片段逐項轉成整數微秒，末端PTS累積偏移63微秒，沒有調寬門檻。改為先取整絕對邊界再相減，MP4 movie timescale使用90000／44100的公倍數4410000；只重封失敗LOGO，不重新推論或重畫，前三段尚未封裝。[固定FFmpeg concat契約](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n5.1.9/doc/demuxers.texi)及[MP4時基](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n5.1.9/doc/muxers.texi)。首次MP4與失敗檢查保留本機，不改稱通過。


## 9. CPU週期驅動的CFR／VFR播放器

狀態：CONFORMED，限通用CPU時戳播放器API，未接正式遊戲／GUI／素材包。承接已確認HD影片方向，補§5的固定格號API不能表達§8原生VBlank時間線的缺口。此切片不識別STR，不以猜測位址、第一段24-bit模式或固定開機場數自動選片。

### 9.1 實際證據與範圍

固定FFmpeg5.1.9、§8已驗KONAMI VFR候選有93個影像包／time_base 1/90000。現有§5解碼參數省略vsync時會產生157格RGB，和probe93格不符；明確-vsync 0後為93格。這是輸出同步插複格，不是原影片缺格。來源SHA、兩側格數與原始flat probe保存私人hd-movie-VFR-decoder-evidence-v3.json／hd-movie-VFR-probe-v3.txt。修正Player解碼命令為逐來源格輸出，原CFR介面保持。

新TimedPlayer在Player之外持有Timeline；沿用明確FFmpeg／ffprobe／MP4路徑、首格預讀與單格RGB快取。Timeline從同一MP4的ffprobe flat輸出讀整數best_effort_timestamp、stream time_base及duration_ts，不能由名目fps推算VFR。時間戳來源是MP4視訊軌，不是CPU PC或原光碟LBA。

### 9.2 輸入／時間契約

影片既有256MiB、4096×2160／總像素與100000格／120fps上限沿§5。額外metadata最多8MiB；以有界stdout讀取，溢位／讀取／子程序失敗均kill並wait。只接受單一所選視訊軌的正u32 time_base分子／分母、正u64 duration_ts；格數必須與ClipInfo相同、首PTS0、嚴格遞增、末PTS小於duration。缺欄位、重複欄位、負值、N/A、索引缺口及不合法範圍回具體錯誤，不默默猜fps。

present_at_cycles(elapsed, clock_hz)接收影片起點後的u64模擬週期與非零u32主時鐘。先比較elapsed×time_base_den與PTS×time_base_num×clock_hz，全部用u128整數交叉相乘，不用浮點。選半開區間[PTS_i,PTS_next)；末格到duration。時鐘第一次選定後不接受中途改變，非暫停時elapsed不能倒退。同格重取只讀快取，前進跳過不保留中間RGB格。

pause保持目前影格且不更新已接受週期；正式前端負責暫停遊戲時鐘。到duration時stop／回收解碼器並回完成，重複完成保持完成。顯式stop可重複，之後取圖回已停止錯誤；倒退必須重新開啟session。沒有wall-clock讀取、音訊裝置、RAM／VRAM／IRQ或遊戲等待副作用；正式遊戲仍沿SPU／XA單一路徑，這個API只呈現影像。

### 9.3 一次集中驗收與停止線

先以人工可算的有理數區間驗精確邊界／大u64值／錯誤metadata，再以已保存真實CFR45格與VFR93格候選取完整RGB，逐格對FFmpeg獨立framecrc，使用其AVAdler32初值0。每個來源PTS邊界驗前一週期與到達週期，另驗pause／同格、改時鐘／倒退拒絕、到期完成／重複完成、stop及子程序回收。既有§5兩個CFR案例承接到本次movie模組集中執行，因解碼參數有變才重跑，舊CPU／GPU／MDEC／M4矩陣不重測。

測試參考由私人素材tools/prepare_hd_movie_timed_player.py產生，入口docs/assets-catalog.md#render-control；內部二進位參考不成為正式影片包格式、不入公開Git。來源沿hd-movie-timed-plan-v3.json，固定工具映像仍為§2與026既有Rust映像。此切片完成不表示正常遊戲已自動換片、海報／GUI／GPU已接或Windows／macOS交付通過，正式來源識別與VFR事件適配留§3。


§9結果：五個movie案例集中通過。新CFR45及VFR93共138格完整RGB同獨立Adler32參考，所有PTS前一週期／到達週期、同格／pause、倒退／改時鐘／零時鐘拒絕、到期完成與stop／回收通過；既有兩CFR案例也通過。舊核心32案例未執行，未重跑M4或新增速度聲明。測試檔及紀錄沿私人hd-movie-timed-accepted-tests-v3與hd-movie-timed-tests-v3.log，來源在rust/src/movie/timeline.rs，Player解碼明確-vsync 0。

## 10. 正常開機的顯示接線觀測

狀態：DRAFT，可丟棄原型，禁止當正式自動選片功能。§9已通過五個movie案例；本節只把同一已測TimedPlayer放到正常真BIOS／原光碟執行的只讀觀察程式，於GPU24-bit影片VBlank輸出HD影像，離開影片後停止／回收。候選檔案在開機前由原型參數明確選定為已知KONAMI，首格預讀；不由第一個24-bit模式猜STR身份，也不改正式CLI、素材包或前端。

原型使用引擎Rust來源的獨立副本，只有複本Cdrom新增讀cur_sector／drive的getter，標成內部下一扇區計數觀測，不能稱實際讀取事件或正式來源識別。記錄已知LOGO LBA extent23104至24783內的觀測；正式來源映射仍沿§3研究，不把命中區間當所有影片選片規則。原引擎CD-ROM／CPU／GPU／GTE／MDEC不改。

同一有界程序兩台Machine從相同真BIOS／光碟重設起跑，一側原生、一側增加HD呈現。正常逐指令到2600場，無側載／PC跳轉／座標注入／新遊戲輸入。每場比完整公開CPU足跡、RAM、VRAM與SPU取樣；結束比完整CPU匯出／RAM／VRAM／TTY／PCM及原畫面，原MDEC與DMA照常保留。HD只寫外部1440×1080影像，原生狀態不可回寫。這是新增呈現分支的必要同態實驗，不重跑舊核心矩陣或完整M4。

來源與SHA沿私人hd-movie-normal-plan-v3.json，工具為素材tools/prepare_hd_movie_normal.py及probe_hd_movie_normal.rs，索引docs/assets-catalog.md#render-control。首段呈現VBlank取得起點，後續以實際Machine週期差供TimedPlayer選格；MP4時間戳量化後與原VBlank可能有一場取樣差，原型不偷加時差常數。GUI、聲音裝置、HD自動來源、海報、GPU播放器與後續影片不在本實驗完成範圍，產圖不等於玩家視窗。


§8本批集中抽樣：OPN／END／LOGO首次抽樣採用；TYK首次發現母稿跨入原片近景，依原生56至71格定位70格交叉淡入，將疊圖限0至69、61至69回既有神經超解析，第二次也是最後一次抽樣採用。母稿及模型未重畫／重跑；只重編TYK影像，原AAC封包保持。來源hd-full-v3/transition-result.json與hd-movie-review-decision-v3.json，最新素材總表hd-movie-full-summary-v3.json。採用限所選格，不稱全片人眼逐格／連續觀看或人工聆聽。

§10首次原型結果：兩台Machine正常重設從真BIOS／原光碟到2600場、1483757071週期、PC80015494；1717至2333共617場選取HD影像，2334退出影片回原生並回收解碼器。每場完整公開CPU足跡、RAM／VRAM及SPU音訊相同；完整端點CPU／RAM／VRAM／TTY／PCM／原圖也相同，RAM／VRAM／TTY／PCM及最後原圖同026 §11已保存正常基準。音訊1931975組44100Hz立體聲，未另播放MP4音軌。內部CD下一扇區計數落已知LOGO範圍625場，只是觀測提示，不當作已證實自動來源識別。原型明確指定候選，正式自動選片／CLI／GUI／海報／GPU與後續影片仍未接，§10保持DRAFT。詳見私人hd-movie-normal-v3/result.json及本批總表。


## 11. 影片來源的實際光碟管線證據

狀態：DRAFT，只讀觀測副本，禁止以未知事件接正式自動換片。§10的cur_sector是下一扇區計數，尚不足以證實實際讀取來源。改在Cdrom實際呼叫Disc::read_sector、內容送入pipe後記錄LBA與格式頭中繼資料；32筆有界佇列，每指令取出，溢位計數必須0。原始8個u32以2352-byte扇區byte24／28／32／36／40／44／48／52為基準，不從數值猜功能名稱。資料負載及原音不進記錄，無原素材公開。

ISO檔案範圍從原Disc既有ISO9660路徑尋址程式的副本查得，保留雙端序及界線檢查，不以固定LBA當普遍選片規則。只追正常M4既有腳本至12000場，沒有PC／RAM注入、不改輸入；來源與準備工具為私人素材prepare_hd_movie_source_trace.py／probe_hd_movie_source_trace.rs，由docs/assets-catalog.md#render-control索引。完整來源SHA、原始頭offset與腳本沿hd-movie-source-plan-v4.json。

第一窄驗收為實際讀取／原始扇區頭／檔案範圍與影片模式開始／結束的先後關係，以及完整端點RAM／VRAM／TTY／原音同既有12000場基準。不重跑舊核心矩陣或完整M4；未知檔案／多義範圍／壞頭或來不及預讀時應沿整段原生，不中途先原版再HD。此節先蒐證，證據足以定義來源及選片契約才推READY，影片素材不重畫／重審。


§11實際讀取結果：一次正常原M4腳本至12000場，28150筆管線讀取、0遺漏；完整CPU端點、RAM／VRAM／TTY／原音／卡片／原圖同既有基準。影片模式兩段的最後實際讀取分別落LOGO15XA.STR與OPN_WS.STR，並非僅靠內部下一扇區計數。逐筆電影格式頭與原片對應LBA的原始扇區一致。精確ISO範圍、首個有效影片扇區到影片開始的週期差、原生模式起訖及完整SHA沿私人hd-movie-source-evidence-v4.json，位址基準是原始光碟LBA／2352-byte扇區offset，不是CPU位址。

## 12. 實際讀取驅動的影片來源選擇

狀態：CONFORMED，限通用來源辨識／整段選擇API及已觀測正常前綴；不是正式GUI、所有影片事件或影片包格式。§11真實讀取及既有ISO尋址足以固定本切片，不以第一段24-bit模式或固定開機場數猜片名。正式來源SHA／影片SHA由呼叫端已驗的素材綁定提供，此API只選來源，不替代§3的素材驗證。

1. Disc::file_extent公開既有ISO尋址的LBA／邏輯bytes；read_file_limit沿同一尋址，雙端序／路徑／上限規則保持。原ISO邏輯2048-byte扇區數向上取整後形成不重疊來源範圍；STR的2352-byte內容不經user-data視圖誤截。
2. CD-ROM的讀取觀測預設關閉，只在原有Disc::read_sector完成、送入pipe的位置記格式中繼資料。啟用後最多32筆、溢位明列，取出不影響DMA、IRQ、時鐘、音訊或guest資料；reset／換片回報觀測中斷。啟用／關閉只動主機觀測狀態。
3. SourceTracker接受最多64個呼叫端已驗來源範圍／opaque key／原生尺寸。拒絕空／重複key、0長度、溢位或重疊範圍與0尺寸。沒有原片bytes、遊戲位址、固定LBA或正式素材包schema寫進通用播放器。
4. 僅影片外更新候選。實際讀取必須有Mode2同步／重複XA頭且位於來源範圍；有效STR magic80010160、DATA或VIDEO型別、chunk序號／總數／frame_size界線與原生尺寸匹配才備妥來源。格式依FFmpeg n5.1.9/libavformat/psxstr.c的公開契約核實，chunk載荷上限2016bytes。相同來源交錯XA音訊不改候選；未知檔案／壞頭／中斷／遺漏清候選，不沿用舊來源。
5. begin在完整影片邊界一次決定：唯一目前來源且呼叫端已預讀Ready才用HD，否則整段原生。進入後不因後續讀碟／預讀完成更換來源，禁止原生先閃再HD。end清除來源與本段選擇；明確解碼故障可單向退原生且本段不重試。只有begin／end／故障改呈現決定，均不回寫原生機器。影片本身的開始／結束事件由呼叫端已驗呈現邊界提供。
6. 正常接線驗收先只綁已有VBlank映射的KONAMI；OPN來源辨識可驗，但raw CFR與原生起點尚未同態映射時沿原生，不猜偏移。TYK／END在本正常前綴未播放，不能外推其開始／跳過／結束已驗。正式GUI與三平台包仍沿§3，原MDEC／DMA照常保留。

一次集中測試涵蓋範圍／格式／音訊交錯、未知／壞頭／重疊／重複來源、缺包／未Ready整段原生、延遲Ready不可中途升HD、影片中來源不改、故障單向回退／結束可重開，以及32筆觀測溢位與reset。再重播§11全部28150筆已保存真實中繼資料，兩次begin分別得LOGO與OPN，不重新跑遊戲。正常原M4前綴另以真正來源選擇接線一次，和§11／既有完整端點比較，影片素材不再審查、舊核心矩陣不重跑。

來源與獨立參考為私人素材analyze_hd_movie_source_trace.py、hd-movie-source-evidence-v4.json／hd-movie-source-reference-v4.bin，入口docs/assets-catalog.md#render-control。原始扇區與圖像負載不加入公開測試語料。方法只讀格式文件及自有程式，未移植外部編解碼實作。


§12集中結果：六個新案例通過，source_篩選同時命中一個既有DMA案例，合計7通過、36未執行。真實參考的28150筆格式頭與原光碟逐筆相同，四個ISO extent相同；兩次開始分別辨識LOGO與OPN。正常原M4腳本到12000場的自動接線一次有效驗證，1717至2333共617場自動LOGO HD，2334回原生並停止／回收；OPN在6692辨識正確但未綁未驗時間線，整段Native到10333。預讀來源早於影片開始約13／16場，使用最新連續有效來源，不沿用先前掃描TYK／END的讀取。

完整CPU端點、RAM／VRAM／TTY／原音PCM／卡片／原圖逐位元組同§11及既有原生基準；12000場的cycles／PC／24-bit模式列完整相同。1920場HD RGB與§10已採用原型相同，沒有新美術審查。音訊僅原生SPU，未疊播AAC。這一新正常路線沒有逐場比完整RAM，§10的2600場成對完整狀態證據直接承接。來源、SHA與接納收據為私人素材tools/prepare_hd_movie_auto.py、probe_hd_movie_auto.rs、record_hd_movie_auto.py及hd-movie-auto-v4/acceptance.json，由docs/assets-catalog.md#render-control索引。

公開格式來源：[FFmpeg n5.1.9 psxstr.c](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n5.1.9/libavformat/psxstr.c)，LGPL-2.1-or-later，只讀公開格式契約，未移植編解碼器。實際參考全文SHA256 e224cd2b4d9e244abf01a2d681f9a63656308be995b7903f657a751573879d0c，保存在私人hd-movie-ffmpeg-psxstr-n5.1.9.c。未驗OPN時間線／TYK與END正常事件、GUI、實體音訊、GPU播放器及正式包保持未完成，沒有新速度聲明。§10原先明確指定的原型歷史保持，本節才是實際讀取自動選片證據。


## 13. OPN原片到正常原生顯示時間線

狀態：CONFORMED，限原生派生OPN素材的VBlank時間線與無頭正常自動呈現；原STR逐格身分與正式GUI仍未知。以下保留最初只讀觀測及失敗對格的範圍。§12已辨識OPN來源，但不能由15fps或6692場直接推算每格。沿同一真BIOS／原光碟、原M4腳本與新卡正常到12000場，於實際來源為OPN及24-bit影片模式的每個VBlank保存完整320×240 RGBA；相鄰相同影像逐位元組合併，保留每場cycles與對應影像序號。CD扇區頭frame_id僅標最後實際預讀的上界線索，不冒稱已顯示格號。正式核心不改、沒有PC或RAM注入。

先核對完整CPU／RAM／VRAM／TTY／原音／卡片／原圖端點及12000場cycles／PC／模式同既有基準，再與已固定原片FFmpeg922格內容對照。沒有唯一身分或等價影像證據的畫面保持未知，不加猜測偏移。既有HD畫稿與原音不重做、不重審；只有映射證據足以固定typed契約後才推READY與接線。此研究不宣稱GUI、GPU或速度驗收。工具為私人素材prepare_hd_movie_opn.py／probe_hd_movie_opn.rs，由docs/assets-catalog.md#render-control索引；來源SHA與畫面沿hd-movie-opn-native-v5/plan.json。


§13原生結果：正常12000場端點與逐場cycles／PC／模式全部同基準。OPN6692至10332共3641場，920段原生影像，第一段為實際全黑畫面；原片15fps直接起播會漏掉此停留且首個完整畫面已對原片第1格。原片像素近似對照有386段超過先前兩個樣本的4色階保守篩選，最大9色階；其中第905段的最近平均差候選是904，次候選才是905，證實平均差不足以證實影格身分。沒有調高門檻或把這些unknown改成通過。

沿已定案LOGO方法，改以這份完整原生920段畫面作同一官方realesr-animevideov3-x4模型的新來源，保存每段原生VBlank週期，末尾加同圖哨兵確保時長。沒有重畫母稿、改風格或新美術審查，舊四段定案素材保留；OPN新來源與原始STR逐格身分繼續分列。SPU原始PCM按同一VBlank週期邊界擷取，樣本序號為floor(cycles/768)，依rust/src/spu.rs初始clock_divider768及每組取樣768CPU週期，並核對完整末端sample_count= floor(total_cycles/768)。影像沿同一33868800Hz原生週期，封裝沿§8絕對邊界取整與90000視訊／4410000容器時基。


### 13.1 原生派生影片的正式窄契約

1. 輸入固定為本節正常原M4路線的920段完整320×240原生RGBA與每段第一個VBlank的週期。完整端點與12000場cycles／PC／模式須同既有基準，逐像素來源SHA不可替換成近似原STR候選。原STR最近影格比對的386段未知保留，不調高4色階研究篩選，也不作runtime規則。
2. HD處理承接已定案的Real-ESRGAN官方v0.2.5.0／realesr-animevideov3-x4／320整格分塊，模型與工具SHA沿hd-movie-ncnn-lock-v2.json；新原生輸入直接採同一流程，不重畫母稿、不增加美術審查。產物1440×1080 H.264，920段加同圖末尾哨兵共921格，只留本機。其他三段已定案影片保持。
3. 影片起點為實際來源OPN且24-bit模式開始的第一個呈現VBlank，不用固定場數選片。每段MP4整數PTS=floor(elapsed_cycles×90000/33868800)，最後哨兵取ceil(duration_cycles×90000/33868800)；以絕對微秒邊界先取整再相減送concat，encoder time_base1/90000，容器4410000。每個PTS不得晚於原生對應VBlank，且必須晚於上一個觀察VBlank，3641場選出的影像序號須全部相同。這可避免最近取整把同一格延後整個VBlank，沒有改變遊戲週期或加光流插格。
4. 原音從同次完整原SPU的PCM按floor(start_cycles/768)至floor(end_cycles/768)擷取，44100Hz雙聲道，確認完整末端取樣數同floor(total_cycles/768)。預覽AAC只轉碼同段原音，起點0、長度誤差不超過一組原取樣，完整解碼與非靜音、相關度>0.98及SNR>15dB；正式遊戲仍只播放原SPU，不疊播AAC。
5. 通用TimedPlayer、首格預讀、單格快取、整數時鐘與stop／回收沿§9，不新增播放器或素材包schema。選片沿§12的實際CD管線來源／ISO範圍／SourceTracker，呼叫端核對素材SHA。缺包或來不及預讀時整段原生，開始後不改來源；本次只增加OPN綁定，不把TYK／END未驗事件或GUI當完成。
6. 集中技術驗證為全部921PTS、3641場原生影像序號、完整RGB解碼與獨立FFmpeg framecrc、同段AAC原音。再用相同原M4輸入正常到12000場自動呈現LOGO及OPN，完整CPU／RAM／VRAM／TTY／PCM／卡片／原圖端點與逐場cycles／PC／模式保持，OPN每個實際呈現格的完整RGB Adler32須同獨立參考。原MDEC／DMA保留；不聲稱逐場完整RAM、GPU影片解碼、速度或正式玩家GUI。

離線集中技術結果已通過：全部921PTS與3641場影像序號精確，完整RGB及原音沿私人hd-movie-opn-hd-v5/verification.json；正式正常呈現尚待本節一次接線驗證。來源與工具由素材docs/assets-catalog.md#render-control的build_hd_movie_opn_native.py、verify_hd_movie_opn_native.py與prepare_hd_movie_opn_player.py索引。原始位址基準為模擬CPU累計週期、VBlank及PCM取樣序號，不是遊戲RAM位址或原片15fps格號。


### 13.2 零時長末尾與嚴格預讀

狀態：CONFORMED，補齊§13.1的末端契約，通用播放器檢查不放寬。首次正常接線在OPN預讀時因duration_ts5526635等於末PTS5526635被既有Timeline拒絕，尚未選OPN HD。完整失敗前綴同原生，保留hd-movie-opn-player-zero-tail-v5。初版離線檢查只驗所有PTS及原生影像序號，漏驗duration必須嚴格大於末PTS，已補此必要條件。不是CPU或遊戲音畫錯誤。

實際B-frame封包最後DTS5514491、首DTS−12143、最大PTS5526635；將最後解碼封包的duration設為2個90000Hz tick，使track duration由5526635變5526636，只延長1tick。使用FFmpeg n5.1.9 setts的N／PTS／DTS／DURATION契約，明確pts=PTS:dts=DTS，不能省略而讓預設TS覆蓋B-frame的PTS。僅stream-copy重封，所有921視訊與全部音訊封包的PTS、DTS、payload SHA、trim side-data及兩軌codec extradata相同。全部RGB／原音證據因此承接，不重新推論、重畫或整片重編。修正後仍須先以真正Rust TimedPlayer開啟、首格、stop／回收，再乾淨跑同一正常路線；首試不改稱通過。

[固定FFmpeg setts原始契約](https://raw.githubusercontent.com/FFmpeg/FFmpeg/n5.1.9/libavcodec/setts_bsf.c)，LGPL-2.1-or-later，只呼叫工具，不移植程式。全文SHA256 dd13f1a67e021c9685fb3c02d58b77dbd741def37f0304612f08caa7a6948e47；本機來源ffmpeg-setts-n5.1.9.c、完整無損證據tail-repair.json。工具repair_hd_movie_opn_tail.py由素材docs/assets-catalog.md#render-control索引。原GPU模型及原生觀測不重跑，素材修正不影響其他三片、模擬核心、海報或現行Go雙擊包。


§13限定結果：OPN已從完整原生顯示序列建立同風格HD，920段影像加末尾哨兵共921格。3641場影像身分全部對齊原生，避免近似原片對格與時間戳取整晚一場。正常12000場自動選取LOGO及OPN，共4258場HD；完整端點狀態／原音與逐場cycles／PC／模式同基準。四段現行素材合計52.50MiB，另外三段SHA保持。沿同一已定案模型，沒有新母稿或美術審查；原MDEC／DMA保留，正式GUI／GPU、TYK／END正常事件與完整包仍待接。全部OPN呈現格的完整RGB Adler32同獨立FFmpeg，原生影像序號／整數PTS全數相同，包含開頭四場原生全黑停留；新正常路線端點CPU／RAM／VRAM／TTY／PCM／卡片／原圖相同，不稱逐場完整RAM比較。來源為私人hd-movie-opn-player-v5/acceptance.json、hd-movie-opn-hd-v5/verification.json，由素材docs/assets-catalog.md#render-control索引。這是無頭呈現，沒有正式CLI／GUI接線、GPU影片解碼或速度驗收。


## 14. 共用影片Session、暫停與原生結束

狀態：CONFORMED，限通用Rust控制層與已觀測LOGO／OPN正常路線，正式GUI／素材包仍DRAFT。證據承接§9整數時間線、§12來源／整段選擇與§13原生OPN；原M4腳本的6800:START:5及實際6800至6805影片模式持續為true，10333才退出，已由同一腳本SHA與保存逐場資料核對。因此按鍵不等於影片已跳過，禁止收到START就強制停止HD或改guest等待／PC。位址基準為模擬週期、VBlank與標準手把按鍵mask，無新遊戲RAM位址。

1. 新movie::Session只組合SourceTracker與既有TimedPlayer，接受來源profiles／非零固定clock_hz。沒有Machine、遊戲位址、按鍵規則、檔案格式或音訊裝置，呼叫端仍核對來源／素材SHA。一次只持有一個預備或活動解碼器，首格由既有Player預讀。
2. observe只更新來源；影片外候選消失／變更時停止舊預備解碼器。needs_prepare僅在影片外且目前唯一來源尚未備妥時回key。prepare只接受目前來源且未開始，錯key或活動中拒絕且回收傳入Player。begin在實際完整影片邊界一次決定HD或整段原生，重複begin不重置epoch；活動中不因預讀或讀碟升HD。
3. present_at_cycles以begin時的原生cycle為epoch，沿同一非零固定時鐘選格。週期倒退或暫停期間週期改變回錯誤，保持既有合法會話；同週期／同格讀快取。pause只凍結呈現器，前端必須同時停止Machine.step及新音訊取樣的提交，不能用主機時間補償影片進度。resume不新增偏移或重設epoch。
4. 影片自然結束或原遊戲接受跳過後，呼叫端在觀察到原生影片模式退出的完整VBlank才呼叫end。end清來源／epoch／呈現並stop、Drop／回收解碼器，可重複。按鍵繼續沿正常手把送原遊戲，本控制層不實作跳片或模擬遊戲狀態。
5. 解碼錯誤、缺影像或素材先到尾端而原生仍播放時，先停止解碼器，再單向退原生，該段不可因晚到素材重新升HD。呼叫端可記錄具體錯誤；原生機器繼續原流程，原MDEC／DMA／SPU保持。準備／停止錯誤不可冒充成功；關閉與Drop仍確保回收。
6. 一次集中新測試涵蓋錯key／活動中準備、整段原生及晚到不升HD、正確epoch與重複begin、pause凍結／暫停時計時違約與resume、原生end／重複end、早到EOF／解碼故障單向退原生與來源重開。使用既有真實CFR45格／VFR素材，未選真實fixture時明列skip，不自製遊戲素材。舊播放器／核心矩陣承接。
7. 新正常接線沿同一原M4腳本至12000場，LOGO／OPN來源不猜。既有START6800維持原生決定，pause採100次停止Machine.step的同cycle呈現要求，CPU／RAM／VRAM／卡片與已取原音不變。全路線端點及逐場cycles／PC／模式與§13基準相同，OPN3641場序號／完整RGB校驗保持，原生結束後沒有解碼器。這是無頭控制接線，不宣稱實體音訊／鍵盤／手把、真正成功跳過OPN或完整GUI。

私人來源入口為素材docs/assets-catalog.md#render-control；準備工具與收據沿既有direct-go-v1/hd-movie-session-v6，不建立第二份現況表。既有四段素材及52.50MiB總表保持，新美術審查0；本段不重編影片、不重畫或增加速度聲明。


§14限定結果：Rust共用Session已接到正常影片路線，整合來源、固定epoch、暫停／續播與原生結束回收。五個新案例通過，正常12000場仍自動呈現4258場HD；OPN3641場影像身分與完整RGB校驗保持。100次暫停呈現要求沒有Machine步進、新原音取樣或狀態變更，續播600場的完整公開CPU／RAM／VRAM／原音／卡片同原生分支。端點及逐場cycles／PC／模式同既有基準，原腳本START6800未使OPN提早結束，控制層沒有強制跳片。四段定案素材與52.50MiB總表保持，新美術審查0；正式玩家GUI、實體聲音／手把、TYK／END正常事件與完整包仍待接。暫停只驗同cycle的100次呈現呼叫，沒有主機wall-clock／實體音訊宣稱；不是成功跳過OPN。五個新Session案例使用保留真實CFR45素材，0skip，其餘43案例未重跑。新路線兩段開始／結束與全部12000場、OPN完整呈現列皆同§13保存資料，沒有重新審查或修改影片。來源為rust/src/movie/session.rs與私人hd-movie-session-v6/tests.log、pause.json、result.json／acceptance.json，由素材docs/assets-catalog.md#render-control的prepare_hd_movie_session.py／probe_hd_movie_session.rs／record_hd_movie_session.py索引。
