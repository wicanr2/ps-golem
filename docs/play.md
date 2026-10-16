# 遊玩說明

ps-golem 是 PlayStation 1 執行器。發行包不含 BIOS 與遊戲光碟，請用自己的檔案：

- BIOS：512 KB 的映像檔，例如 `SCPH5500.BIN`。
- 光碟：`.cue` 與它指到的 `.bin`，放在同一個目錄。

## 開始

| 平台 | 執行 |
|---|---|
| Windows | 解壓後雙擊 `psgolem.exe` |
| macOS | 解壓後雙擊 `ps-golem.app`。第一次開啟若被擋下，在 Finder 對它按右鍵選「打開」；仍打不開時在終端機執行 `xattr -dr com.apple.quarantine ps-golem.app` |
| Linux | 解壓後執行 `./psgolem`。需要系統有 X11、OpenGL 與 ALSA 的程式庫（一般桌面環境都有） |

第一次開啟會看到設定畫面。把 BIOS 檔與光碟的 `.cue` 檔拖進視窗，兩個都放進去後就會開機。
路徑會記在設定檔，之後雙擊就直接開機。

也可以用命令列指定，命令列優先於設定檔：

```
psgolem -bios <BIOS> -cue <光碟 cue> [-card <記憶卡檔>] [-scale <視窗倍率>]
```

要換 BIOS 或光碟，用命令列指定一次，或刪掉設定檔再開。

## 按鍵

| PS 手把 | 鍵盤 |
|---|---|
| 十字鍵 | 方向鍵 |
| ✕ ○ □ △ | `Z` `X` `A` `S` |
| L1 R1 L2 R2 | `Q` `W` `1` `3` |
| START SELECT | `Enter` `Backspace` |

也支援標準手把（Xbox 類配置）。

## 存檔位置

記憶卡與設定檔放在使用者設定目錄的 `ps-golem/` 底下：

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
