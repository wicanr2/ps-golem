# ps-golem

無頭、決定性、可以當 Go 套件 import 的 PlayStation 1 執行器。
方法與分層沿用 [dosgolem](https://github.com/wicanr2/dosgolem)。

第一個案例是《惡魔城X：月下的夜想曲》中文版：在 Linux、Windows、macOS 上可玩、有 HD 輸出，
並讓 AI agent 能在 `go test` 裡直接查詢執行中的狀態。

規格見 [`docs/spec/000-index.md`](docs/spec/000-index.md)。目前可以從光碟開機玩到遊戲內，有聲音（SPU、XA、CD-DA）。

目前已驗證正常開機、首存與重開讀檔。HD 提供內部解析度、PGXP、輸出濾鏡、
素材替換及可選 CPU／GPU 後端。完整遊戲 HD 美術、速度與通關仍待驗收；
3D 貼圖透視校正可選用 `-hd 2 -pgxp -texture-perspective`，範圍見 [022](docs/spec/022-hd-texture-perspective.md) 與 [Issue #12](https://github.com/wicanr2/ps-golem/issues/12)。

Rust改寫位於[`rust/`](rust/)，自有核心已能正常首存與重新讀檔。玩家視窗位於[`rust/player/`](rust/player/)，支援原版顯示與LOGO／OPN的HD影片替換，影片沿原生時間線、原SPU聲音與場邊界切換。正常讀檔、HD影片暫停續播及關窗存卡已驗；實體音訊／手把、海報與完整設定、其餘影片、全遊戲HD及三平台Rust雙擊仍待完成。軟體視窗仍有音訊補零，速度未過關。入口與範圍見[026](docs/spec/026-rust-migration.md#14-rust原版玩家視窗)及[027 §15](docs/spec/027-hd-movie-replacement.md#15-hd影片接入rust玩家視窗)。目前雙擊遊玩版本仍使用Go。

## 執行

發行包（Linux、Windows、macOS）用 `tools/build_dist.sh <版號>` 在 docker 內建出，放在 `dist-all/<完整版號>/patch/`；完整版號採 `v.X.Y.Z-YYYYMMDD`。
雙擊 `psgolem` 後把 BIOS 與光碟的 `.cue` 拖進視窗即可開機，路徑會記住。遊玩說明見 [`docs/play.md`](docs/play.md)。

開發時在 docker 內建置（`tools/go.sh build -o bin/psgolem ./cmd/psgolem`），也可以用參數執行：

```
bin/psgolem -bios <BIOS> -cue <光碟 cue>
```

| PS 手把 | 鍵盤 |
|---|---|
| 十字鍵 | 方向鍵 |
| ✕ ○ □ △ | `Z` `X` `A` `S` |
| L1 R1 L2 R2 | `Q` `W` `1` `3` |
| START SELECT | `Enter` `Backspace` |

也支援已連線 USB 與已配對藍芽手把。滑鼠移到視窗上緣或按 F1 顯示工具列，可設定按鍵、死區及測試輸入。藍芽配對由作業系統處理。

記憶卡預設存在使用者設定目錄的 `ps-golem/<開機檔名>.mcd`，可用 `-card` 指定。
細節見 [`012-frontend-ebiten`](docs/spec/012-frontend-ebiten.md)。

HD 素材包可用 `-hd 2 -textures <目錄>` 選擇，路徑會保存，下次雙擊自動載入。
GPU 後端用 `-hd-renderer gpu` 選擇，預設為 `cpu`；選項與限制見 [遊玩說明](docs/play.md)。

外部海報包可用`-posters <目錄> -poster a -widescreen=false`載入。F1工具列可切換款式、外側融合與安全放大，選項會保存；格式與限制見[024](docs/spec/024-poster-presentation.md)及[遊玩說明](docs/play.md#兩側海報)。公開執行器不附遊戲海報。

## 原版素材

本 repo 不含任何 BIOS、光碟映像或從光碟抽出的檔案。執行時由玩家指定自己的檔案。

## 授權

GPL-3.0-or-later，見 `LICENSE`。參照來源見
[`docs/spec/002-references-and-licensing.md`](docs/spec/002-references-and-licensing.md)。
