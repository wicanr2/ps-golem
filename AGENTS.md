# AGENTS.md

給在本 repo 工作的 coding agent（Codex、Claude Code）共用的規則。Claude 專屬補充見 `CLAUDE.md`。

## 這是什麼

ps-golem 是無頭、決定性、可以當 Go 套件 import 的 PlayStation 1 執行器，仿照
`~/cht/dosgolem`。第一個案例是《惡魔城X：月下的夜想曲》中文版（`SLPM-86023` 為底），
目標是在 Linux、Windows、macOS 上可玩並有 HD 輸出，同時讓 agent 能程式化觀測。

原版素材在 `~/cht/castlevinia_x`（不在本 repo）。

## 動手前

1. 讀 `docs/spec/000-index.md` 與 `docs/spec/001-scope-and-mvp.md`。
2. 只有標 `READY` 的規格可以實作。量測／反組譯 → 規格 → 才寫程式。

## 指令

全部走 docker，不裝到系統環境。

```sh
tools/build_image.sh                         # 建 psgolem-go-ebiten 映像（要網路）
PSGOLEM_NETWORK=bridge tools/go.sh mod download   # 新 clone 先抓模組
tools/go.sh build ./...
PSGOLEM_GO_CMD=xvfb-run tools/go.sh -a go test ./...   # 全部（frontend/ebiten 在 init 開 GLFW，要 xvfb）
tools/go.sh test ./machine/... ./internal/...       # 不含前端時不需要 xvfb
tools/go.sh test ./internal/cpu -run TestXxx -v    # 單一測試
PSGOLEM_EXTRA_MOUNTS="<BIOS 目錄>:/bios:ro" PSGOLEM_BIOS=/bios/SCPH5500.BIN tools/go.sh test ./machine/...   # 要 BIOS 的測試
PSGOLEM_GO_CMD=gofmt tools/go.sh -l .
PSGOLEM_ORIG=~/cht/castlevinia_x tools/go.sh test ./...   # 原版素材唯讀掛到 /orig
tools/build_image.sh osxcross && tools/build_image.sh wine   # 交叉編譯 macOS、Wine 冒煙測試用的映像
tools/build_dist.sh v0.1.0                    # 三平台發行包到 dist/v0.1.0/（005 §4）
```

osxcross 映像含 macOS SDK，只留本機，不推送。

測試讀的環境變數前綴 `PSGOLEM_`、`SOTN_`、`EBITEN_` 會傳進容器，值要寫容器內路徑。

## `[HARD]` 硬規則

- **不得散布原版素材。** ISO、BIOS、光碟抽出的檔案、截圖不進版控、不進發行包。
  需要原版的測試缺檔就 skip，不用自製代用品。
- **建置與測試一律走 docker**（`tools/go.sh`）。只清理自己建立的 container；
  禁止任何 `docker ... prune` 與 `docker rmi`。
- **git 身分一律 `wicanr2@gmail.com`。** commit message 不放 `Claude-Session:` 連結。
- **授權 GPL-3.0-or-later。** 參照外部實作時依 `docs/spec/002-references-and-licensing.md`
  §4 記錄專案、檔案、commit 與授權。DuckStation 與 sotn-decomp 只讀不搬程式碼。
- **CPU 與 GTE 的驗收判準是全部通過。**
- **推論標籤要誠實**：confirmed／強證據／假說／未知。
- **HD 不得改變機器狀態。** `render/hd/` 只讀 GPU 命令流。開關 HD 前後，同一份輸入重播的
  RAM 與 CPU 狀態必須相同。

## 分層

見 `docs/spec/001-scope-and-mvp.md` §3。判準：「換一片光碟之後，這段程式碼還成立嗎？」
成立就往下放，不成立就留在 `apps/sotn/`。最容易放錯的是位址：演算法通用，遊戲內位址屬於 `apps/`。

## HD 分工

HD（內部解析度放大、PGXP 類次像素精度、寬螢幕 16:9、2D 圖素放大濾鏡）由 Codex 實作。
寬螢幕要攔截遊戲的投影與裁切，屬遊戲專屬，放 `apps/sotn/`。驗收附原生解析度與 HD 的對照截圖
（截圖不進版控）。
