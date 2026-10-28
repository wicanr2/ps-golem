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
