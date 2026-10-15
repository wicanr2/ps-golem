# 005 跨平台交付

狀態：`DRAFT`

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

## 4. 進入 READY 前要補

- 有 `cmd/` 可建的執行檔之後，讀 kol 的 `tools/verify_macos.sh` 定靜態驗收清單。
- 發行包格式（AppImage、zip、`.app`）與命名。
