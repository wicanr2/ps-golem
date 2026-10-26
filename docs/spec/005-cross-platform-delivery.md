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
| `tools/build_image.sh wine` | `psgolem-wine-verify`：Ubuntu 24.04＋Wine＋Xvfb＋Weston／Xwayland＋PulseAudio，Windows 冒煙與 Linux 硬體 GPU／節拍音訊驗證；noble-r2 替代 noble-r1，來源為 tools/docker/wine-verify.Dockerfile |

版號以 `-ldflags "-X main.version=<版號>"` 寫進執行檔，`-version` 印出。psgolem 的版號在
`cmd/psgolem/early.Version`：該套件在 Ebiten 初始化前處理 `-version`，Linux 沒有顯示器時也能印出。

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

## 8. 本機完整版

給自己在本機或自己的其他電腦上玩的包：功能全開、雙擊就能玩。與 §5 發行包相同，**不含 BIOS、光碟映像或從光碟抽出的檔案**；
玩家把自己的檔案放進包內的資料夾，或沿用設定檔記住的路徑。

### 8.1 可攜設定

| 項目 | 規定 |
|---|---|
| 可攜目錄 | Linux AppImage：環境變數 `APPIMAGE` 所指檔案的目錄；macOS：`.app` 所在的目錄；其他：執行檔所在的目錄 |
| 可攜設定檔 | 可攜目錄有 `settings.json` 時，讀寫都用它，不用 `<使用者設定目錄>/ps-golem/settings.json`；沒有時照 `012` §9 |
| 自動尋找 | 命令列與設定檔都沒給 BIOS 或 cue 時，在可攜目錄、其下 `bios/`、`disc/` 三處（不遞迴）找：大小正好 512 KiB 的檔案當 BIOS，副檔名 `.cue` 的檔案當光碟。各剛好一個才採用，多個或沒有就照舊顯示設定畫面 |
| 優先順序 | 命令列 > 設定檔 > 自動尋找 |
| 相對路徑 | 可攜設定檔裡的 `bios`、`cue` 可以是相對路徑，以可攜目錄為基準解析；自動尋找寫回時存相對路徑，整包搬到別台電腦仍可用 |

### 8.2 包的內容

`tools/build_local.sh <版號>` 產生 `dist/<版號>-local/`，版本目錄已存在就停：

| 檔名 | 內容 |
|---|---|
| `ps-golem-<版號>-x86_64.AppImage` | AppDir：`AppRun` 啟動 `usr/bin/psgolem`，另含 `usr/bin/psexe`、`.desktop`、圖示 PNG。只包 ps-golem 自己的執行檔；X11、OpenGL、ALSA 用系統的（同 §5 Linux 包） |
| `ps-golem-<版號>-local-linux.zip` | AppImage 加上 §8.3 的資料夾與預設設定，解開即可雙擊 |
| `ps-golem-<版號>-local-windows.zip` | `psgolem.exe`、`psexe.exe` 加上 §8.3 |
| `ps-golem-<版號>-local-macos.zip` | `ps-golem.app`、`psexe` 加上 §8.3 |

每個 zip 解開後的頂層目錄 `ps-golem/` 含：

- `settings.json`：`{"hd": 2, "pgxp": true, "filter": "xbr", "widescreen": true}`，不含 `bios`、`cue`。
- `bios/`、`disc/`：空資料夾，各放一個 `放這裡.txt` 說明要放的檔案。
- `LICENSE`、`README.md`、`docs/play.md`。

寬螢幕是遊戲修改模式（`017` §2）；本機完整版的預設設定檔開啟它，屬玩家自己的設定，程式本身的預設仍是關閉。

AppImage 用 `AppImage/appimagetool` 的 `appimagetool-x86_64.AppImage` 與 `AppImage/type2-runtime` 的 `runtime-x86_64`。
上游只發 `continuous`，所以第一次建置時下載一份到 `workplace/appimage/`（不入版控），把下載日期與 SHA-256 記在本節；
之後的建置一律用這份並先核對雜湊，不重新下載。容器內沒有 FUSE，以 `--appimage-extract-and-run` 執行 appimagetool，`--runtime-file` 指定 runtime。
下載用 `PSGOLEM_NETWORK=bridge`，只在這一步開網路。

固定輸入於 2026-10-18（臺北時間）下載，UTC 收據為 `2026-10-17T16:24:17Z`。
建置時以 `tools/appimage.sha256` 核對，`tools/local_pack.py` 負責組包與內容檢查。
`tools/build_local.sh` 先呼叫未修改的 `tools/build_dist.sh`，其 `dist/<版號>/` 為基礎包，
本機完整版交付在 `dist/<版號>-local/`。不帶原版掛載，建置全程停用網路。
AppImage 封裝使用既有 `psgolem-osxcross:1.24-15.5-r1`，其中有工具要求的 `file`；
Go 映像負責 Go 建置與 ZIP 組包，不增加映像。腳本在編譯前先檢查封裝工具可執行。

| 工具 | 固定版本、授權 | SHA-256 |
|---|---|---|
| [AppImage/appimagetool](https://github.com/AppImage/appimagetool) | continuous，`854e19e951900cf141245b384d45735942521abe`，build 313，MIT | `95cbe7cce9717fce90c484e34052ee7c7f1d7635b33c12525b4776826a7d29b6` |
| [AppImage/type2-runtime](https://github.com/AppImage/type2-runtime) | continuous，`8f39b89e2ac31e1640b3d3f7e9a5108e6ce805fa`，MIT；靜態相依授權見原始 LICENSE | `156f4bdbde9c52d01814600013e0a273f0118dc2de98975f3c8c63427ec79074` |

下載檔、release JSON、原始 LICENSE 與時間收據留在 `workplace/appimage/`，不入版控。
runtime 的原始 LICENSE 隨 Linux AppImage 與 ZIP 保存為 `APPIMAGE-LICENSE.txt`。
appimagetool 僅用於建置，不進封包。PNG 圖示由專案程式產生，不含外部美術。
上述檔案缺少時，唯一需開網路的步驟如下。已有檔案不覆寫；下載後仍須核對固定雜湊：

```sh
PSGOLEM_NETWORK=bridge PSGOLEM_GO_CMD=bash tools/go.sh -c '
  set -euo pipefail
  mkdir -p workplace/appimage
  cd workplace/appimage
  for entry in appimagetool:appimagetool-x86_64.AppImage type2-runtime:runtime-x86_64; do
    project=${entry%%:*}; asset=${entry#*:}
    test ! -e "$asset"
    curl -fL --retry 2 "https://github.com/AppImage/$project/releases/download/continuous/$asset" -o "$asset"
    curl -fL "https://raw.githubusercontent.com/AppImage/$project/main/LICENSE" -o "$project-LICENSE"
  done
  sha256sum -c /src/tools/appimage.sha256
  chmod +x appimagetool-x86_64.AppImage runtime-x86_64
'
tools/build_local.sh v0.5.0
```

`continuous` 若已更換內容，雜湊不符即停止，保留原下載快照，不自行更新固定值。

### 8.3 驗收

| 項目 | 判準 |
|---|---|
| 可攜設定、自動尋找 | 單元測試（不用原版素材）：可攜目錄判定、相對路徑、三處尋找、多個候選不採用、優先順序 |
| AppImage | 在 `psgolem-wine-verify`（Ubuntu 24.04，無 Go）以 `--appimage-extract-and-run` 搭配 `xvfb-run` 啟動：BIOS 與 cue 放在 AppImage 旁的 `bios/`、`disc/`（唯讀掛載原版），不帶參數開機到第 2000 次 `Update`，可攜 `settings.json` 寫入相對路徑 |
| Windows、macOS zip | 內容檢查與 §5 相同的素材排除；Windows 啟動檢查依使用者決定延後 |
| 素材排除 | 所有包不得含 `.bin`、`.cue`、`.iso`、`.img`、`.chd`、`.mcd`、`.mcr`、`.state` 與 512 KiB 的檔案 |

### 8.4 驗收結果

目前狀態表（2026-10-18）：

| 項目 | 結果 | 證據 |
|---|---|---|
| 可攜設定、自動尋找 | 通過；不使用原版素材，涵蓋三處尋找、歧義、優先順序、相對路徑與搬移 | `workplace/local-pack/unit.log`，`cmd/psgolem/portable_test.go` |
| 小修：簽章失配 | 通過；不回傳致命錯誤、只警告一次、RAM 不變、三處處理器仍掛著 | `TestWidescreenMismatchWarning`，`workplace/local-pack/unit.log` |
| 小修：陷阱暫存器寫回 | 通過；取消同目標延遲載入，來源使用舊值，其他目標載入照常提交 | `TestWidescreenRowLoadDelay`，`workplace/local-pack/unit.log` |
| 全專案回歸 | 通過，容器退出碼 0；CPU、1150 筆 GTE、17 個 N=1 GPU 語料全部通過 | `workplace/local-pack/regression.log`、`regression-container-exit.log` |
| CPU 熱路徑與既有建置腳本 | 相對本輪起始 `a16dfc8` 無差異 | `workplace/local-pack/unchanged-core-and-build.diff` |
| 完整 M4 | 通過，容器退出碼 0；原生、HD2、HD2-PGXP 均完成 field 45784，摘要 `6c35c2701228722f`，記憶卡與原生畫面逐位元相同 | `workplace/local-pack/m4.log`、`m4-container-exit.log` |
| 三平台包與 macOS 結構 | 完整腳本通過；雙弧、arm64 簽章、minos 11.0 與系統動態庫檢查通過 | `workplace/local-pack/build.log` |
| local ZIP 與素材排除 | 三包通過 CRC、頂層目錄、設定、素材目錄、UTF-8、禁止副檔名與 512 KiB 排除；四包 SHA-256 通過 | `workplace/local-pack/packages.json`、`package-checksums.log`，`dist/v0.5.0-local/manifest.json` |
| AppImage 啟動 | 通過，容器退出碼 0；無 Go、無缺庫，無 BIOS/cue 參數，兩次 2000 Update 畫面逐位元相同；設定保留四個畫面選項，路徑寫回相對值 | `workplace/local-pack/linux-smoke.log`、`linux-container-exit.log`、`settings-first.json`、`first.png`、`second.png` |
| AppImage 內部內容 | 通過，實際解出執行檔逐位元相同，無素材與 512 KiB 檔案 | `workplace/local-pack/linux-smoke.log`，`tools/local_pack.py check-image` |
| Windows 啟動 | 依使用者決定延後，只驗 ZIP 內容 | 本輪範圍 |
| macOS 實機、效能 | 未驗實機；不量測效能 | 本輪範圍 |
| 收尾 | 無 root-owned 檔案、異常 .md 目錄或本工作樹殘留容器；未推送或發布 | `workplace/local-pack/root-files.log`、`ownership-final.log`、`containers-final.log` |

本輪證據入口為 `workplace/local-pack/`，過程見 `WORKLOG.md` 的本機完整版紀錄。
總驗收索引為 `workplace/local-pack/verification.json`，彙整程式為同目錄的 `write-receipt.py`。
完整交付清單為 `dist-files.txt`，含本機包及沿用 build_dist 產生的三個基礎包。
輸入、來源、映像與截圖收據為 `inputs.sha256`、`source.sha256`、`images.json`、`screenshots.json`。
M4 外層連線曾回傳 143，原容器持續執行並通過；`m4.log` 由同一容器的完整日誌保存，
原工具輸出另存 `m4-tool.log`，狀態核對見 `m4-after-tool143.json`。沒有取消或重跑。
Linux 啟動與 AppImage payload 重跑入口為 `PSGOLEM_ORIG=~/cht/castlevinia_x tools/verify_local.sh v0.5.0`。
此腳本由 local ZIP 解包，唯讀掛載指定三份素材，使用既有 `psgolem-wine-verify:noble-r1`；
不執行 Windows 程式。每次驗收需新的 `workplace/local-pack/smoke/`，已有目錄即停止。

### 8.5 含素材的本機例外

素材 repo 的 AGENTS 已定案：含原版只在 castlevinia_x/local-full 組裝自用，不外傳。新版入口及三平台雙擊流程見 §9；乾淨封包仍不含原版。

## 9. 新版交付與雙擊啟動（2026-10-18，READY）

使用者要求含素材完整版雙擊啟動。新版採 `v.X.Y.Z-YYYYMMDD`，其中開頭字面值為 `v.`；
例如 `v.0.6.0-20261018`。既有 v0.5.0 與其收據保留，不覆寫或移動。
乾淨封包的現行根目錄改為 `dist-all/<完整版號>/patch/`，守門、雜湊及 manifest 仍適用 §8。
舊版 §8.4 是歷史驗收；本輪結果見 WORKLOG 及 castlevinia_x 的目前狀態表。

本專案素材目錄 AGENTS 的明確例外仍優先：含原版只組裝到
`castlevinia_x/local-full/<完整版號>/{linux,windows,macos}/ps-golem/`，不散布或加入 Git。
組裝入口為該 repo 的 `tools/assemble_local_full.sh`；輸入改讀上述 patch/ 的本機 ZIP。
Linux 本機 ZIP 額外附原生 `psgolem`／`psexe`，雙擊 psgolem 不需要 AppImage 的 FUSE 掛載。
AppImage 仍附帶；Windows 雙擊 psgolem.exe，macOS 雙擊 ps-golem.app。
從其他 cwd 啟動須讀包根 settings.json，自動找到唯讀 BIOS／cue，重啟後保持手把與素材路徑。
正式美術未定時不預設研究替換。驗收使用封包內執行檔，不能用開發 bin 代替。

依 [Apple CFBundleShortVersionString](https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundleshortversionstring)
及 [CFBundleVersion](https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundleversion)，
兩個平台數字欄位使用三段核心版號；完整發行識別寫入 PSGolemReleaseVersion、程式版本、封包與 manifest。
不對原有已發布版本套用回溯修改。

打包及驗收的研究收據用 `workplace/local-pack/<完整版號>/`，避免覆寫舊收據。
只有新版本目錄可建立。原版唯讀，輸出使用 UID/GID；三平台仍在 Linux Docker 內編譯。
Windows 執行、macOS 真機與人工聲音限制維持先前決定。

新版容器組包入口為 `tools/dist_pack.py`，由 `tools/build_dist.sh` 呼叫，負責 .app metadata、授權附件、素材守門、三平台封包與 SHA256SUMS。主機包裝器只做 Docker 編排及必要路徑檢查。

### 9.1 新版驗收結果

`v.0.6.0-20261018` 的七個乾淨產物已完成，manifest 與 SHA256SUMS 在該版 patch/。
三平台本機 ZIP 經 CRC、UTF-8、設定、檔案清單、權限與素材排除檢查；macOS 四道結構通過。
AppImage 版本、動態相依與實際 payload 對照通過。`tools/assemble_local_full.sh` 已組裝三平台本機完整版，逐份素材雜湊核對成功。

Linux 以封包啟動檔、其他 cwd、零參數、自動尋找 BIOS／cue 實際開機，F1 面板與正常關窗通過。
驗證副本另以已保存研究包及手把設定重啟，完整設定逐位元保留；正式完整版仍是自動手把，不預設研究圖。
真正完整版再以預設 HD2／PGXP／xBR／寬螢幕從 GUI 開機到 field 4320 標題，已目視確認。
收據在 `workplace/hdtex/texture-integration-verification.json`，來源包內容表在
`workplace/local-pack/v.0.6.0-20261018/packages.json`。Windows／macOS 真機與實體手把維持未驗證。

### 9.2 透視候選（READY）

使用者已確認HD透視校正與雙擊啟動。新本機ZIP的畫面預設加上
`texture_perspective: true`，與既有HD2及PGXP一同使用；通用程式的預設仍為關閉。
只使用新完整版號與新輸出目錄，舊版本不覆寫。原版模式可用022既有明確false恢復，
不改輸入重播、素材格式或存檔格式。005既有三平台守門、版號及素材排除繼續適用。
已驗的精簡HD包只由素材repo安裝器裝到含素材本機候選，不放公開引擎包。
需確認封包內透視選項、跨cwd啟動、保存及重開；macOS仍限結構檢查，未驗真機明示。

新候選採021既定CPU預設供雙擊預覽，GPU作明確選用的實驗路徑。
GPU測試保存獨立的設定快照；切回CPU只用既有-hd-renderer選項並保留其他畫質及手把設定。
既有GPU候選與使用者存檔不改；此選擇不是新的效能實作或速度驗收通過。

### 9.3 鄰房視野候選交付範圍

v.0.7.12-20261026沿tools/build_local.sh重建七個純程式包至dist-all/v.0.7.12-20261026/patch。版號／SHA／ZIP、macOS四道結構、Linux依賴及實際AppImage payload通過。三平台含媒體組裝仍只在相鄰素材專案local-full例外，不加Git、tag或Release；Windows／macOS待真機。

一次新Linux封包正常BIOS／讀卡／左移至8330，CPU HD4／1920×1080，frame／shown2634且最後已呈現；1842幀由017 §12正式來源接受。驗證程式是封包的逐位元相同可攜副本，僅獨立設定明確開啟擴大視野。已安裝原版偏好、素材、海報、媒體與卡片保持。既有設定重開及GPU證據承接，不重播。完整HD、正常DRE、速度及通關未完成。

本輪工具與收據入口為相鄰素材專案docs/assets-catalog.md#render-control的neighbor-delivery-v1；使用其既有prepare_hd_view_delivery.py --neighbor-scene、install_hd_view_delivery.py及verify_native_delivery_gui.py，不另造交付流程。實際範圍及未完成邊界見017 §12.5。
