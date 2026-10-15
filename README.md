# ps-golem

無頭、決定性、可以當 Go 套件 import 的 PlayStation 1 執行器。
方法與分層沿用 [dosgolem](https://github.com/wicanr2/dosgolem)。

第一個案例是《惡魔城X：月下的夜想曲》中文版：在 Linux、Windows、macOS 上可玩、有 HD 輸出，
並讓 AI agent 能在 `go test` 裡直接查詢執行中的狀態。

規格見 [`docs/spec/000-index.md`](docs/spec/000-index.md)。目前可以從光碟開機玩到遊戲內，有聲音（SPU、XA、CD-DA）。

## 執行

建置在 docker 內（`tools/go.sh build -o bin/psgolem ./cmd/psgolem`）。執行：

```
bin/psgolem -bios <BIOS> -cue <光碟 cue>
```

| PS 手把 | 鍵盤 |
|---|---|
| 十字鍵 | 方向鍵 |
| ✕ ○ □ △ | `Z` `X` `A` `S` |
| L1 R1 L2 R2 | `Q` `W` `1` `3` |
| START SELECT | `Enter` `Backspace` |

也支援標準手把。記憶卡預設存在使用者設定目錄的 `ps-golem/<開機檔名>.mcd`，可用 `-card` 指定。
細節見 [`012-frontend-ebiten`](docs/spec/012-frontend-ebiten.md)。

## 原版素材

本 repo 不含任何 BIOS、光碟映像或從光碟抽出的檔案。執行時由玩家指定自己的檔案。

## 授權

GPL-3.0-or-later，見 `LICENSE`。參照來源見
[`docs/spec/002-references-and-licensing.md`](docs/spec/002-references-and-licensing.md)。
