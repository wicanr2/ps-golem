#!/usr/bin/env bash
# Go 走 docker，不裝到系統環境。
#
#   tools/go.sh build ./...
#   tools/go.sh test ./internal/cpu -run TestSingleStep -v
#
# 跑 go 以外的工具用 PSGOLEM_GO_CMD：
#
#   PSGOLEM_GO_CMD=gofmt tools/go.sh -l .
#
# 要跑原版執行檔時，用 PSGOLEM_ORIG 指到玩家自己的素材目錄，
# 它會**唯讀**掛到容器裡的 /orig：
#
#   PSGOLEM_ORIG=~/cht/castlevinia_x tools/go.sh test ./... \
#   （BIOS 在 /orig/bios/，光碟映像由玩家自己解壓到素材目錄）
#
# 本專案不含任何原版檔案，掛載一律唯讀（`ro`），容器也不會把它們帶出去。
#
# 測試讀的環境變數（PSGOLEM_*、SOTN_*、EBITEN_*，見下面
# PASS_PREFIXES）原樣傳進容器。**值要寫容器裡的路徑**：原版放 /orig 底下，
# 其他目錄用 PSGOLEM_EXTRA_MOUNTS 掛進來。
#
# 預設映像是 `psgolem-go-ebiten`（tools/docker/go-ebiten.Dockerfile，用
# tools/build_image.sh 建），有 Ebiten 要的 X11／ALSA 標頭與 xvfb。
#
# module cache 放 workplace/（gitignore），避免每次重抓。容器預設不開網路，
# 新 clone 先抓一次模組：
#
#   PSGOLEM_NETWORK=bridge tools/go.sh mod download
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PSGOLEM_GO_IMAGE:-psgolem-go-ebiten:1.24-r1}"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "tools/go.sh：找不到映像 $IMAGE。先跑 tools/build_image.sh，" \
    "或用 PSGOLEM_GO_IMAGE 指定別的映像。" >&2
  exit 1
fi
# 預設映像比 Dockerfile 舊的話提醒一聲（不擋）：少了新加的套件，錯誤會出現在很後面。
if [[ -z "${PSGOLEM_GO_IMAGE:-}" ]]; then
  want="$(sha256sum "$ROOT/tools/docker/go-ebiten.Dockerfile" | cut -d' ' -f1)"
  have="$(docker image inspect -f '{{index .Config.Labels "psgolem.dockerfile.sha256"}}' "$IMAGE" 2>/dev/null)"
  if [[ "$have" != "$want" ]]; then
    echo "tools/go.sh：映像 $IMAGE 不是用目前的 Dockerfile 建的，跑 tools/build_image.sh 更新。" >&2
  fi
fi
RUN_UID="${PSGOLEM_RUN_UID:-$(id -u)}"
RUN_GID="${PSGOLEM_RUN_GID:-$(id -g)}"

mkdir -p "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"

# docker 預設不繼承 shell 的環境。交叉編譯要 GOOS 這幾個；測試的素材位置
# 依前綴整批傳。go.sh 自己的設定（PSGOLEM_GO_*、掛載、資源上限）不傳。
PASS=()
for v in GOOS GOARCH CGO_ENABLED; do
  [[ -n "${!v:-}" ]] && PASS+=(-e "$v=${!v}")
done
PASS_PREFIXES='^(PSGOLEM_|SOTN_|EBITEN_)'
SELF='^PSGOLEM_(GO_|ORIG$|EXTRA_MOUNT|NETWORK$|TIMEOUT$|MEM$|CPUS$|RUN_)'
while IFS= read -r v; do
  if [[ "$v" =~ $PASS_PREFIXES && ! "$v" =~ $SELF ]]; then
    PASS+=(-e "$v=${!v}")
  fi
done < <(compgen -e)

# 原版素材唯讀掛載。沒設就不掛——容器預設看不到任何原版檔案。
MOUNTS=()
if [[ -n "${PSGOLEM_ORIG:-}" ]]; then
  MOUNTS+=(-v "$(cd "$PSGOLEM_ORIG" && pwd):/orig:ro")
fi
# 可寫的輸出目錄，格式 `主機路徑:容器路徑`。截圖要落在 repo 外面時用它——
# **不要為了省事把整個家目錄掛進來**。
if [[ -n "${PSGOLEM_EXTRA_MOUNT:-}" ]]; then
  MOUNTS+=(-v "$PSGOLEM_EXTRA_MOUNT")
fi
# 多個掛載用分號隔開，格式同 docker -v（例 `~/buck:/buck:ro;~/fonts:/fonts:ro`）。
# 主機路徑不存在就停：dockerd 會以 root 建一個空目錄頂替，測試讀到空目錄不報錯。
if [[ -n "${PSGOLEM_EXTRA_MOUNTS:-}" ]]; then
  IFS=';' read -ra extra <<<"$PSGOLEM_EXTRA_MOUNTS"
  for spec in "${extra[@]}"; do
    [[ -z "$spec" ]] && continue
    host="${spec%%:*}"
    if [[ ! -e "$host" ]]; then
      echo "tools/go.sh：掛載來源 $host 不存在" >&2
      exit 1
    fi
    MOUNTS+=(-v "$spec")
  done
fi

# --init：讓 tini 當 PID 1。xvfb-run 當 PID 1 時等不到 Xvfb 的就緒訊號，會一直卡住。
exec timeout "${PSGOLEM_TIMEOUT:-30m}" docker run --rm --init --network "${PSGOLEM_NETWORK:-none}" \
  --memory "${PSGOLEM_MEM:-4g}" --cpus "${PSGOLEM_CPUS:-4}" --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$RUN_UID:$RUN_GID" \
  -v "$ROOT:/src" \
  -v "$ROOT/workplace/gocache:/gocache" \
  -v "$ROOT/workplace/gomodcache:/gomodcache" \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
  -e HOME=/tmp -e GOFLAGS=-mod=mod \
  "${PASS[@]}" "${MOUNTS[@]}" -w /src "$IMAGE" "${PSGOLEM_GO_CMD:-go}" "$@"
