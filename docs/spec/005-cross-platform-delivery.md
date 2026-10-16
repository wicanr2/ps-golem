# 005 跨平台交付

狀態：`READY`

## 1. 決定

Linux、Windows、macOS 三個平台全部在 Linux 的 docker 內建置，不用 GitHub Actions 的 macOS runner。

做法沿用 `~/cht/kol`（同為 Go＋Ebiten）的 `tools/build_dist.sh`、`tools/build_macos.sh`、
`docker/Dockerfile.osxcross-go`，已在該專案實際產出過三平台執行檔。

## 2. 三個平台

| 平台 | 方式 | 映像 | 依據 |
|---|---|---|---|
| linux/amd64 | 容器內原生建，`CGO_ENABLED=1`（Ebiten 在 Linux 要 cgo） | `psgolem-go-ebiten` | kol `tools/build_dist.sh` 註解 |
| windows/amd64 | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`（Ebiten 在 Windows 走 syscall，不用 cgo） | `psgolem-go-ebiten` | 同上；第一次建置時實際編一次確認 |
| darwin arm64＋amd64 | `CGO_ENABLED=1`，`CC` 指 osxcross 的 clang，兩弧各編一次再 `lipo` 成 universal | 另一個映像，Ubuntu 24.04 起底 | kol `tools/build_macos.sh` |

macOS 要另一個映像：`crazymax/osxcross` 的執行檔要 glibc 2.38，`psgolem-go-ebiten`
是 Debian 12（glibc 2.36）跑不動。工具鏈前綴帶 SDK 次版號，從 `osxcross-conf` 讀，不寫死。

## 3. 界線

- macOS SDK 的授權只允許在 Apple 硬體上使用。osxcross 映像只留本機，不推送、不進版控。
- Linux 上執行不了 Mach-O。macOS 版的驗收只有靜態檢查：雙弧、arm64 有 ad-hoc 簽章、
  最低系統版本、沒有連到系統目錄以外的動態庫。這不代表功能正常，真機驗收另外做。
- 不做 notarization。
- 發行包只含 ps-golem 的程式碼、文件與 `LICENSE`，不含 BIOS、光碟映像或從光碟抽出的檔案。

## 4. 腳本與映像

| 檔案 | 作用 |
|---|---|
| `tools/build_dist.sh <vX.Y.Z>` | 建三平台、驗 macOS 結構、組 `.app`、打包到 `dist/<版號>/`，附 `SHA256SUMS`；版本目錄已存在就停 |
| `tools/build_macos.sh <目錄>` | `psgolem`、`psexe` 兩弧各編一次後 `lipo` |
| `tools/verify_macos.sh <檔>...` | 四道結構驗收（沿用 kol `tools/verify_macos.sh`） |
| `tools/build_image.sh osxcross` | `psgolem-osxcross`：Ubuntu 24.04＋`crazymax/osxcross:15.5-debian`＋`psgolem-go-ebiten` 的 Go 1.24.13 |
| `tools/build_image.sh wine` | `psgolem-wine-verify`：Ubuntu 24.04＋Wine＋xvfb，Windows 版冒煙測試 |

版號以 `-ldflags "-X main.version=<版號>"` 寫進執行檔，`-version` 印出。

## 5. 發行包

| 檔名 | 內容 |
|---|---|
| `ps-golem-<版號>-linux-amd64.tar.gz` | `psgolem`、`psexe` |
| `ps-golem-<版號>-windows-amd64.zip` | `psgolem.exe`、`psexe.exe` |
| `ps-golem-<版號>-macos-universal.zip` | `ps-golem.app`（`Contents/MacOS/psgolem`＋`Info.plist`，最低 macOS 11.0）、`psexe` |

每個執行檔都要含該版號字串，視窗執行檔還要含設定畫面的字串，否則停止打包（Mach-O 在 Linux 上只能這樣確認包進去的是這次的程式）。
每包另帶 `LICENSE`、`README.md`、`docs/play.md`。打包前檢查不得有 `.bin`、`.cue`、`.iso`、`.img`、`.chd`、`.mcd`、`.mcr`、`.state`。
`.app` 不做 bundle 簽章（Linux 做不出 `_CodeSignature`）；arm64 執行檔帶 ld64 加的 ad-hoc 簽章。
雙擊開啟沒有參數，BIOS 與光碟位置由設定畫面與設定檔處理（`012` §9）。

## 6. 驗收

| 項目 | 判準 |
|---|---|
| Linux | 解開發行包，在沒有 Go 與標頭檔的 Ubuntu 24.04（`psgolem-wine-verify`）上 `xvfb-run` 執行：`ldd` 沒有缺庫；先以參數開機一次寫入設定檔，再不帶參數開機，第 2000 次 `Update` 的畫面與 `psexe` 相同 |
| Windows | 同上，以 Wine 執行 `psgolem.exe`；另以 `psexe.exe` 跑 2B 週期，摘要與畫面和 Linux 版相同 |
| macOS | `tools/verify_macos.sh` 四道全過；真機遊玩另外做 |

## 7. 驗收結果（2026-10-16，v0.1.1，SCPH5500）

- Linux：`ldd` 無缺庫；設定檔寫入後不帶參數開機，第 2000 次 `Update` 與 `psexe` 0 像素不同。
- Windows（Wine 9，xvfb）：同上 0 像素不同；設定檔寫在 `%AppData%\ps-golem\settings.json`。
  `psexe.exe` 2B 週期摘要 `e6cca47ab31dc42c`、畫面 PNG 與 Linux 版逐位元相同。沒有音效裝置時（容器內無 ALSA 卡）照常執行。
- macOS：執行檔含 `v0.1.1` 與設定畫面字串；`psgolem`、`psexe` 雙弧、arm64 有 `LC_CODE_SIGNATURE`、`minos 11.0`、只連系統動態庫。未在實機執行。
- 長時間重播（2026-10-16）：Wine 下的 `psexe.exe` 重播 M4 腳本（260 億週期，開機到入口區存檔），
  摘要、記憶卡、最後畫面與 Linux 逐位元相同（`015` §7）。
- 視窗版（2026-10-16，v0.1.2）：解開發行包，Linux 的 `psgolem` 與 Wine 下的 `psgolem.exe` 以 `-press` 重播 M4 腳本，
  前端自動存下的記憶卡與 `psexe` 逐位元相同。
