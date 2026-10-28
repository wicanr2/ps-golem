# 027 HD影片替換與H.264播放

狀態：整體DRAFT。使用者2026-10-28明示「改播放器，用H.264及AI提升畫質，播放時以高畫質影片取代」，決定已確認。離線來源與編碼原型已驗；正式播放攔截、同步及連續AI影片尚未完成。Rust核心移植沿026，MDEC原生相容沿011，不把本原型稱為遊戲影片已替換。

## 1. 已確認的呈現方向

- 原版模式保留原片／MDEC呈現；HD模式在影片開始前選定外部HD版本並替換顯示。未安裝對應影片時沿原版相容來源，不能原版先閃一格再換HD。
- 高畫質素材離線製作，播放時解碼H.264，不即時呼叫AI。至少1080像素高，保留原構圖、角色、鏡頭與片長；目前4:3來源以1440×1080測試，交由既有024海報組合到16:9，不自行裁切／拉寬影片。
- 承接已核准的HD美術風格與精簡包策略。imagegen可製作重繪參考格；整段AI增強必須處理時間一致性，不將逐張無關重繪或單純Lanczos放大稱為完整AI影片。
- 先保留15fps原始時間與音訊，不自動插格或改台詞。播放開始、跳過、暫停、結束及回到遊戲須跟遊戲事件一致。CPU／GPU解碼路徑保持目標，未實測硬體解碼時不得宣稱已支援。

## 2. 已證實的來源與離線原型

唯讀來源為相鄰私人素材的workplace/assets/raw-sectors/*.STR.raw，已由既有extract_assets.py依光碟ISO extent抽取；本批另核對來源SHA。四個根檔與LBA為LOGO15XA.STR 23104、OPN_WS.STR 13864、TYK_WS.STR 8308、END_WS.STR 870。位址基準是光碟LBA，非CPU PC或影片PTS。

既有kol-package:1.26.7-ebiten2.9.9-20261011-r1映像固定SHA256 0b933af1238ab5839ad7a64b6ae05f069f57cb85da5917dfbe59730f6d7ba53f，FFmpeg 5.1.9-0+deb12u1。實測具有psxstr、mdec、adpcm_xa、libx264及H.264解碼，CPU原型已跑；列出cuda／vaapi等編譯介面不等於硬體驗收。

四段探測均為320×240、15fps、37800Hz雙聲道XA。完整解碼計數OPN 922、TYK 555、END 743格；LOGO出現ac-tex damaged，未取得完整格數。這是FFmpeg解碼相容缺口，不能稱原素材壞掉或略過驗證；LOGO後續沿自有MDEC原生輸出取來源，不為轉檔重跑全部遊戲／舊美術。

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
