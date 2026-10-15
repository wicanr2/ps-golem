# ps-golem

無頭、決定性、可以當 Go 套件 import 的 PlayStation 1 執行器。
方法與分層沿用 [dosgolem](https://github.com/wicanr2/dosgolem)。

第一個案例是《惡魔城X：月下的夜想曲》中文版：在 Linux、Windows、macOS 上可玩、有 HD 輸出，
並讓 AI agent 能在 `go test` 裡直接查詢執行中的狀態。

目前在規格階段，見 [`docs/spec/000-index.md`](docs/spec/000-index.md)。

## 原版素材

本 repo 不含任何 BIOS、光碟映像或從光碟抽出的檔案。執行時由玩家指定自己的檔案。

## 授權

GPL-3.0-or-later，見 `LICENSE`。參照來源見
[`docs/spec/002-references-and-licensing.md`](docs/spec/002-references-and-licensing.md)。
