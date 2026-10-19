#!/usr/bin/env bash
# 005 §8：沿用公開包的建置與驗收，再組成本機可攜包。原版素材不掛入建置容器。
set -euo pipefail
VER="${1:?用法：build_local.sh <完整版號，例如 v.0.6.0-20261018>}"
[[ "$VER" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]] || { echo '完整版號格式 v.X.Y.Z-YYYYMMDD' >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
[[ ! -e "dist-all/$VER" && ! -e "workplace/stage-$VER-local" ]] || { echo '本機版本目錄已存在，不覆寫' >&2; exit 2; }
# 既有 build_dist 也拒絕覆寫它的版本與 staging 目錄。
[[ ! -e "dist-all/$VER" && ! -e "workplace/stage-$VER" ]] || { echo '基礎版本目錄已存在，不覆寫' >&2; exit 2; }
unset PSGOLEM_ORIG PSGOLEM_EXTRA_MOUNT PSGOLEM_EXTRA_MOUNTS
export PSGOLEM_NETWORK=none
PSGOLEM_GO_CMD=bash tools/go.sh -c 'set -euo pipefail; cd workplace/appimage; sha256sum -c /src/tools/appimage.sha256'
# osxcross 既有映像含 appimagetool 要求的 file；Go 映像沒有此工具。
APPIMAGE_IMAGE="${PSGOLEM_MAC_IMAGE:-psgolem-osxcross:1.24-15.5-r1}"
PSGOLEM_GO_IMAGE="$APPIMAGE_IMAGE" PSGOLEM_GO_CMD=bash tools/go.sh -c 'set -euo pipefail; command -v file; workplace/appimage/appimagetool-x86_64.AppImage --appimage-extract-and-run --version'
tools/build_dist.sh "$VER"
PSGOLEM_GO_CMD=python3 tools/go.sh tools/local_pack.py prepare "$VER"
PSGOLEM_GO_IMAGE="$APPIMAGE_IMAGE" PSGOLEM_GO_CMD=bash tools/go.sh -c '
  set -euo pipefail
  export ARCH=x86_64
  workplace/appimage/appimagetool-x86_64.AppImage --appimage-extract-and-run \
    --no-appstream --runtime-file workplace/appimage/runtime-x86_64 \
    "workplace/stage-$1-local/AppDir" "dist-all/$1/patch/ps-golem-$1-x86_64.AppImage"
' _ "$VER"
PSGOLEM_GO_CMD=python3 tools/go.sh tools/local_pack.py finish "$VER"
