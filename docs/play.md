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
| `-hd 2`、`-hd 4`、`-hd 8` | 影子 GPU 以指定倍率繪圖，增加記憶體與 CPU 用量 |
| `-filter none` | 最近鄰縮放，預設值 |
| `-filter xbr` | xBR-lv2 濾鏡，可與 HD 疊加 |

成功開機後，`hd` 與 `filter` 會存入 `settings.json`。命令列參數優先，可用 `-hd 0 -filter none` 關閉。
HD 只影響輸出，存檔與模擬狀態相同。視窗維持 4:3，較寬的視窗左右補黑。
`psexe -hd N -screen <PNG>` 可擷取放大畫面，`psexe` 不提供 xBR 濾鏡。

## 《月下》寬螢幕

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

也支援標準手把（Xbox 類配置）。

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

解開 `ps-golem-v0.5.0-local-<平台>.zip`，進入頂層的 `ps-golem/`：

1. 把自己的 512 KiB PlayStation BIOS 放進 `bios/`，只放一份。
2. 把自己的 cue 與所有 bin 放進 `disc/`，只放一份 cue。保留 cue 引用的 bin 檔名。
3. 雙擊開啟 Linux 的 AppImage、Windows 的 `psgolem.exe`，或 macOS 的 `ps-golem.app`。

Linux 的 AppImage 需要執行權限；若解壓縮工具沒有保留，先用檔案管理員允許執行。
系統需提供 X11、OpenGL 與 ALSA。沒有 FUSE 時可在終端機執行：

```sh
./ps-golem-v0.5.0-x86_64.AppImage --appimage-extract-and-run
```

包內 `settings.json` 預設開啟 HD 2 倍、PGXP、xBR 與寬螢幕。
寬螢幕會修改遊戲程式，限制見本文件的寬螢幕說明；程式本身的預設仍為關閉。
自動尋找成功後，BIOS 與 cue 會以相對路徑寫回此設定檔，整個 `ps-golem/` 可以一起搬移。
同類檔案有多個候選或缺檔時顯示設定畫面，可拖入檔案。明確指定的命令列優先於設定檔，再來才是自動尋找。
記憶卡仍存使用者設定目錄，搬移遊戲資料夾不會搬移記憶卡。包內沒有 BIOS、光碟與記憶卡。
Windows 本輪只驗封包內容，macOS 只驗執行檔結構，尚待各平台實機確認。
