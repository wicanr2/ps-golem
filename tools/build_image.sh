#!/usr/bin/env bash
# 建 tools/go.sh 預設用的映像。只建這一個映像，不動任何共用資源。
#
#   tools/build_image.sh
#
# 要網路（apt 與拉基底映像），所以不走 tools/go.sh 的 --network none。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PSGOLEM_GO_IMAGE:-psgolem-go-ebiten:1.24-r1}"
DOCKERFILE="$ROOT/tools/docker/go-ebiten.Dockerfile"
# 標上 Dockerfile 的雜湊，tools/go.sh 據此提醒映像是不是舊的。
SUM="$(sha256sum "$DOCKERFILE" | cut -d' ' -f1)"
exec timeout 30m docker build -t "$IMAGE" --label "psgolem.dockerfile.sha256=$SUM" \
  -f "$DOCKERFILE" "$ROOT/tools/docker"
