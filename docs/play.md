# 遊玩說明

ps-golem 是 PlayStation 1 執行器。發行包不含 BIOS 與遊戲光碟，請用自己的檔案：

- BIOS：512 KB 的映像檔，例如 `SCPH5500.BIN`。
- 光碟：`.cue` 與它指到的 `.bin`，放在同一個目錄。

## 開始

| 平台 | 執行 |
|---|---|
| Windows | 解壓後雙擊 `psgolem.exe` |
| macOS | 解壓後雙擊 `ps-golem.app`。程式沒有 Apple 公證，第一次開啟會被擋下，處理方式見下方 |
| Linux | 解壓後執行 `./psgolem`。需要系統有 X11、OpenGL 與 ALSA 的程式庫（一般桌面環境都有） |

### macOS 第一次開啟

ps-golem 沒有經過 Apple 公證（notarization），從網路下載後第一次開啟會被 Gatekeeper 擋下。擇一處理：

- 在終端機進到解壓的目錄，執行 `xattr -dr com.apple.quarantine ps-golem.app`，之後就能雙擊開啟。
- macOS 15（Sequoia）以後：先雙擊一次讓它被擋，再到「系統設定 → 隱私權與安全性」頁面底部按「強制打開」。
- macOS 14 以前：在 Finder 對它按右鍵選「打開」。

macOS 15 拿掉了右鍵「打開」的繞過方式（[Macworld](https://www.macworld.com/article/2457844/what-to-do-when-you-cant-open-an-app-you-just-installed-in-macos-sequoia.html)）。

## 設定 BIOS 與光碟

第一次開啟會看到設定畫面。把 BIOS 檔與光碟的 `.cue` 檔拖進視窗，兩個都放進去後就會開機。
路徑會記在設定檔，之後雙擊就直接開機。

也可以用命令列指定，命令列優先於設定檔：

```
psgolem -bios <BIOS> -cue <光碟 cue> [-card <記憶卡檔>] [-scale <視窗倍率>]
```

要換 BIOS 或光碟，用命令列指定一次，或刪掉設定檔再開。

## HD 與濾鏡

```
psgolem -hd 2 -filter xbr
```

| 參數 | 效果 |
|---|---|
| `-hd 0` | 原生解析度，預設值 |
| `-hd 2`、`-hd 4`、`-hd 8` | 以指定倍率繪圖，增加記憶體與運算量 |
| `-hd-renderer cpu` | CPU HD 繪圖，預設值 |
| `-hd-renderer gpu` | 實驗性 GPU HD 繪圖，需要有效圖形裝置 |
| `-hd-renderer auto` | 嘗試建立 GPU HD 裝置；建立失敗時回到 CPU |
| `-filter none` | 最近鄰縮放，預設值 |
| `-filter xbr` | xBR-lv2 濾鏡，可與 HD 疊加 |

成功開機後，`hd`、`hd_renderer` 與 `filter` 會存入 `settings.json`，下次雙擊沿用。
例如執行一次 `psgolem -hd 2 -hd-renderer gpu`，之後雙擊即可使用 GPU HD。
也可在既有設定檔填入 `"hd_renderer": "gpu"`。啟動日誌會顯示實際後端。
命令列參數優先；用 `-hd-renderer cpu` 回到 CPU，用 `-hd 0 -filter none` 關閉 HD。
GPU 執行色彩、抖色、半透明及 HD 像素寫入；貼圖快取與精確覆蓋仍由 CPU 準備。
目前 CPU／GPU 同步成本仍高，GPU 模式尚未通過遊戲速度驗收。
HD 只影響輸出，存檔與模擬狀態相同。視窗維持 4:3，較寬的視窗左右補黑。
`psexe -hd N -screen <PNG>` 可擷取放大畫面，`psexe` 不提供 xBR 濾鏡。

## HD 素材包

素材包目錄需包含 `manifest.json` 及對應圖片。啟用一次後會記住路徑，下次雙擊啟動自動載入：

```sh
psgolem -textures <素材包目錄>
```

可攜完整版會將包內素材路徑保存為相對路徑，整個資料夾一起搬移仍可使用。
也可在包根 `settings.json` 的 `textures` 欄位填入素材目錄，之後直接雙擊啟動。
清空該欄位，或啟動一次 `psgolem -textures=`，即可關閉替換。
未製作的素材維持原圖；損壞的素材包會回報錯誤。這項功能不改記憶卡與原生遊戲狀態。

素材製作者可用 `-dump-textures <新目錄>`，正常遊玩後關閉程式，保存實際使用過的預覽與中繼資料。
輸出目錄必須不存在，傾印路徑不會被記住。原版衍生素材只存本機，不加入公開包。
格式與製作契約見 `018-hd-texture-replacement.md`、`019-hd-texture-pack.md`。

## 兩側海報

外部海報包可在4:3遊戲畫面的兩側加入裝飾，形成16:9版面。執行器不附遊戲海報，請使用有權使用的圖片包。

```sh
psgolem -posters <海報目錄> -poster a -widescreen=false
```

滑鼠移到上緣或按F1顯示工具列。「海報」依序切換各款與關閉；「柔和」切換外側場景漸層；「放大」只在當前黑邊足夠時等比放大中央。任何非黑內容可能被裁到時，保留完整畫面。這些操作不暫停遊戲、不改鏡頭或記憶卡。

海報目錄、選擇、柔和與放大選項保存在同一份settings.json，下次雙擊沿用。可攜設定中的海報路徑使用相對路徑。`-poster=`只關閉裝飾，`-posters=`清除目錄；`-poster-soft=false`恢復明確接縫，`-poster-zoom=false`保持完整4:3。

既有真正寬螢幕模式仍保留。`widescreen`開啟時優先顯示完整寬螢幕內容，海報偏好保留但不繪製；海報版面請用`-widescreen=false`。工具列切換會完整交換已預載的海報，不逐張補入。包缺檔或損壞時記錄錯誤並保留原呈現；保存失敗時不套用新選擇。

製作者的PNG成對清單、容量與路徑界線見[024-poster-presentation](spec/024-poster-presentation.md)。海報與遊戲內HD素材分開載入；加入海報不表示背景或人物重繪已完成。

## 《月下》寬螢幕

滑鼠移到上緣或按 F1，點選「視野 原版／擴大」。選擇會保存，畫面提示下次啟動套用；關閉程式再雙擊開啟即可切換。本次遊戲繼續使用啟動時的視野。

切回原版視野後，會沿用已保存的海報選擇。只有一畫面寬的房間，兩側仍可能沒有場景內容。

```
psgolem -hd 2 -widescreen
```

`-widescreen` 開啟 16:9 遊戲修改模式，預設關閉。未指定 HD 時自動使用倍率 1，
也可疊加 `-filter xbr`。成功開機後，設定記在 `settings.json` 的 `widescreen` 欄位。
下次啟動用 `-widescreen=false` 關閉。

此模式會修改《月下》的圖塊繪製範圍，記憶卡照常讀寫。HUD、選單與部分效果仍在中央，
房間邊界外與特殊背景可能留黑。程式只套用已核對簽章的版本；遊玩中簽章失配只警告一次，保留已套用的修改。
`psexe -widescreen -hd 2 -screen <PNG>` 只供截圖，開啟時不印機器摘要。

## 按鍵

| PS 手把 | 鍵盤 |
|---|---|
| 十字鍵 | 方向鍵 |
| ✕ ○ □ △ | `Z` `X` `A` `S` |
| L1 R1 L2 R2 | `Q` `W` `1` `3` |
| START SELECT | `Enter` `Backspace` |

### USB 與藍芽手把設定

1. USB 手把先接上電腦。藍芽手把先在作業系統完成配對與連線。
2. 把滑鼠移到視窗上緣，或按 F1 顯示工具列，再點「手把設定」。
3. 用裝置列左右箭頭選擇手把。預設「自動」會合併所有已連線手把；也可選「僅鍵盤」。
4. 點選要修改的 PS 按鍵，再按下手把按鈕或推動搖桿。等待指派時可按 Delete 或 Backspace 清除舊指派，方便交換按鍵。下方可調整死區，查看即時動作、原始按鈕與軸值。
5. 點「儲存」才套用並記住設定；「取消」或 ESC 放棄本次修改。

工具列平常隱藏，不改遊戲畫面的比例。F1 可以固定顯示工具列，再按一次取消固定。
設定面板開啟時遊戲暫停；關閉後先放開按鍵，再繼續操作。
標準手把沿用 Xbox 類配置，左搖桿也可控制方向。沒有標準配置的手把可逐項指派原始按鈕或軸。
選定手把斷線時保留配置，鍵盤仍可使用；連回後可繼續使用相同模型配置。
同型手把以目前列舉順序區分，USB 與藍芽若回報不同識別會保存為不同配置。

設定記在同一份 settings.json 的 gamepad 欄位。程式不負責藍芽配對，實際連線狀態由系統提供。

## 存檔位置

記憶卡放在使用者設定目錄的 `ps-golem/` 底下；沒有可攜設定檔時，設定檔也放在此處：

| 平台 | 目錄 |
|---|---|
| Windows | `%AppData%\ps-golem\` |
| macOS | `~/Library/Application Support/ps-golem/` |
| Linux | `~/.config/ps-golem/`（或 `$XDG_CONFIG_HOME/ps-golem/`） |

記憶卡檔名是光碟的開機檔名加 `.mcd`，例如 `SLPM_860.23.mcd`。遊戲寫入記憶卡後約 1 秒自動存檔，關閉視窗時也會存。
設定檔是 `settings.json`。

## 已知限制

- macOS 版在 Linux 上交叉編譯，尚未在實機上驗證。
- 速度依電腦而定；模擬比實機慢時遊戲整體變慢，聲音會斷續。

## 本機完整版

解開 `ps-golem-<完整版號>-local-<平台>.zip`，進入頂層的 `ps-golem/`：

1. 把自己的 512 KiB PlayStation BIOS 放進 `bios/`，只放一份。
2. 把自己的 cue 與所有 bin 放進 `disc/`，只放一份 cue。保留 cue 引用的 bin 檔名。
3. 雙擊 Linux 的 `psgolem`、Windows 的 `psgolem.exe`，或 macOS 的 `ps-golem.app`。Linux 也保留 AppImage。

Linux 的執行檔與 AppImage 需要執行權限；若解壓縮工具沒有保留，先用檔案管理員允許執行。
系統需提供 X11、OpenGL 與 ALSA。沒有 FUSE 時可在終端機執行：

```sh
./ps-golem-<完整版號>-x86_64.AppImage --appimage-extract-and-run
```

包內 `settings.json` 預設開啟 HD 2 倍、PGXP、xBR 與寬螢幕。
寬螢幕會修改遊戲程式，限制見本文件的寬螢幕說明；程式本身的預設仍為關閉。
自動尋找成功後，BIOS 與 cue 會以相對路徑寫回此設定檔，整個 `ps-golem/` 可以一起搬移。
同類檔案有多個候選或缺檔時顯示設定畫面，可拖入檔案。明確指定的命令列優先於設定檔，再來才是自動尋找。
記憶卡仍存使用者設定目錄，搬移遊戲資料夾不會搬移記憶卡。包內沒有 BIOS、光碟與記憶卡。
Windows 本輪只驗封包內容，macOS 只驗執行檔結構，尚待各平台實機確認。

### 3D 貼圖透視校正

以 `-hd 2 -pgxp -texture-perspective` 啟用選用透視取樣，支援 CPU／GPU 兩後端。
選項保存為 `texture_perspective`，下次雙擊沿用；用 `-texture-perspective=false` 恢復仿射取樣。
舊設定預設關閉。缺少可靠深度、等深度圖元與精靈保持既有取樣，原生遊戲與記憶卡狀態不變。
範圍見 [022](spec/022-hd-texture-perspective.md)。完整遊戲覆蓋、效能與候選包更新另驗。
