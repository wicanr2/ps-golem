#!/usr/bin/env bash
# 建 ps-golem 用的映像。只建指定的映像，不動任何共用資源。
#
#   tools/build_image.sh            # psgolem-go-ebiten：tools/go.sh 預設的建置與測試映像
#   tools/build_image.sh osxcross   # psgolem-osxcross：macOS 交叉編譯（只留本機）
#   tools/build_image.sh wine       # psgolem-wine-verify：Windows 版冒煙測試
#
# 要網路（apt 與拉基底映像），所以不走 tools/go.sh 的 --network none。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
case "${1:-go}" in
  go)       IMAGE="${PSGOLEM_GO_IMAGE:-psgolem-go-ebiten:1.24-r1}"; DF=go-ebiten.Dockerfile ;;
  osxcross) IMAGE="${PSGOLEM_MAC_IMAGE:-psgolem-osxcross:1.24-15.5-r1}"; DF=osxcross-go.Dockerfile ;;
  wine)     IMAGE="${PSGOLEM_WINE_IMAGE:-psgolem-wine-verify:noble-r1}"; DF=wine-verify.Dockerfile ;;
  *) echo "用法：build_image.sh [go|osxcross|wine]" >&2; exit 2 ;;
esac
DOCKERFILE="$ROOT/tools/docker/$DF"
# 標上 Dockerfile 的雜湊，tools/go.sh 據此提醒映像是不是舊的。
SUM="$(sha256sum "$DOCKERFILE" | cut -d' ' -f1)"
exec timeout 30m docker build -t "$IMAGE" --label "psgolem.dockerfile.sha256=$SUM" \
  -f "$DOCKERFILE" "$ROOT/tools/docker"
