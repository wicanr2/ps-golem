# 002 參考來源與授權

狀態：`READY`

## 1. ps-golem 的授權

GPL-3.0-or-later（全文見 `LICENSE`）。

選定理由：

- 要參照的模擬器核心是 GPL-2.0-or-later，可以併入 GPL-3.0 的作品。
- 前端 Ebiten 是 Apache-2.0。Apache-2.0 與 GPL-3.0 相容，與 GPL-2.0-only 不相容，
  所以不能選 GPL-2.0-only。

## 2. 可參照實作的來源

2026-10-15 以 GitHub API 與原始檔檔頭核實。

| 專案 | 授權 | 核實依據 | 用途 |
|---|---|---|---|
| `libretro/beetle-psx-libretro` | GPL-2.0-or-later | `mednafen/psx/cpu.c`、`gte.c`、`gpu.c`、`mdec.c`、`spu.c`、`cdc.c` 檔頭 | CPU、GTE、GPU、CD-ROM、SPU 的主要參照 |
| `grumpycoders/pcsx-redux` | GPL-2.0-or-later | `src/core/r3000a.cc` 檔頭 | 第二參照、除錯器設計參考 |
| `hajimehoshi/ebiten` | Apache-2.0 | GitHub license API | 前端相依 |
| `ebitengine/oto`、`ebitengine/purego` | Apache-2.0 | GitHub license API（2026-10-15） | Ebiten 音訊的間接相依 |
| `JaCzekanski/ps1-tests` | MIT | GitHub license API | 測試 ROM 語料，不進版控 |
| `notofonts/noto-cjk` Sans2.004 | OFL-1.1 | 官方 Sans/LICENSE、本機 TTC 著作權與 SHA-256，見 `020` §6 | 工具列 UI 位圖字圖集；授權附件獨立保存 |

2026-10-19另核實 `Koops0/GPSXE`，commit `b1ecd8cdcd6dab0a588d2e0a6077210e5fa2ba11`，根LICENSE為MIT。
只比較Go直譯迴圈、持久映射頂點緩衝與GPU批次提交，未移植程式碼；其COP2與貼圖仍未完成，
不作《月下》速度基準。來源逐檔雜湊在`workplace/hdtex/perf/gpsxe-reference/provenance.json`，
比較及改善候選在同目錄上一層`gpsxe-comparison.json`。

Beetle PSX 的 PGXP 實作在 `pgxp/`（2026-10-17 於 commit `e57860d` 核實）。`pgxp_gte.c`、`pgxp_gpu.c` 與各 `.h`
檔頭為 GPL-2.0-or-later；`pgxp_cpu.c`、`pgxp_mem.c`、`pgxp_value.c`、`pgxp_main.c` 沒有檔頭，
原始出處 `iCatButler/pcsxr`（commit `62467b8`）同樣沒有檔頭，該 repo `COPYING` 為 GPLv3。
兩種授權都與 GPL-3.0-or-later 相容，可移植（`016` §5.2）。

## 3. 只能讀、不能移植程式碼的來源

| 專案 | 狀況 |
|---|---|
| `stenzek/duckstation` | GitHub 回報 `NOASSERTION`，現行授權不是自由授權，只讀文件與行為描述 |
| `Xeeynamo/sotn-decomp` | AGPL-3.0。可查位址與資料結構當 `apps/sotn/` 的證據，不把程式碼搬進 ps-golem |

硬體文件 psx-spx（nocash）可自由參考，引用時註明章節。

## 4. 參照紀錄格式

參照外部實作寫成的函式，在函式註解寫：

```go
// 參照 beetle-psx-libretro mednafen/psx/gte.c (commit <sha>)，GPL-2.0-or-later。
```

commit SHA 用 `gh api repos/<repo>/commits/<branch> --jq .sha` 取得，不憑記憶寫。

## 5. 原版素材

ISO、BIOS 與從光碟抽出的檔案不進版控、不進發行包。執行時由玩家指定路徑。
