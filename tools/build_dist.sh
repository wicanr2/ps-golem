#!/usr/bin/env bash
# 在 Linux 的 docker 內建出三個平台的發行包（docs/spec/005-cross-platform-delivery.md）。
#
#   tools/build_dist.sh <版號>        # 例：tools/build_dist.sh v.0.6.0-20261018
#
# 產出 dist-all/<版號>/patch/：
#   ps-golem-<版號>-linux-amd64.tar.gz     psgolem、psexe（cgo，連系統的 X11／OpenGL／ALSA）
#   ps-golem-<版號>-windows-amd64.zip      psgolem.exe、psexe.exe（不用 cgo）
#   ps-golem-<版號>-macos-universal.zip    ps-golem.app（arm64＋x86_64）、psexe
# 每包都帶 LICENSE、README.md、docs/play.md、UI-FONT-LICENSE.txt。不帶任何 BIOS、光碟映像、記憶卡。
#
# 版本目錄必須是新的，既有版本不覆寫。
set -euo pipefail
VER="${1:?用法：build_dist.sh <完整版號，例如 v.0.6.0-20261018>}"
[[ "$VER" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]] || { echo "完整版號格式 v.X.Y.Z-YYYYMMDD" >&2; exit 2; }
CORE="${VER#v.}"
CORE="${CORE%-*}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/dist-all/$VER/patch"
[ ! -e "$OUT" ] || { echo "$OUT 已存在，不覆寫" >&2; exit 2; }
STAGE="$ROOT/workplace/stage-$VER"
[ ! -e "$STAGE" ] || { echo "$STAGE 已存在，先自行移走" >&2; exit 2; }
mkdir -p "$OUT" "$STAGE"/{linux,windows,macos}
REL="workplace/stage-$VER"
# psgolem 的版號放在 cmd/psgolem/early（要在 Ebiten 初始化前印出）；psexe 沒有這個符號，連結器略過。
LDF="-X main.version=$VER -X github.com/wicanr2/ps-golem/cmd/psgolem/early.Version=$VER"
cd "$ROOT"

echo "== linux/amd64"
for c in psgolem psexe; do
  CGO_ENABLED=1 tools/go.sh build -p 2 -trimpath -ldflags "$LDF" -o "$REL/linux/$c" ./cmd/$c
done
echo "== windows/amd64"
for c in psgolem psexe; do
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 tools/go.sh build -p 2 -trimpath -ldflags "$LDF" -o "$REL/windows/$c.exe" ./cmd/$c
done
echo "== darwin universal"
PSGOLEM_LDFLAGS="$LDF" tools/build_macos.sh "$REL/macos/bin"
tools/verify_macos.sh "$REL/macos/bin/psgolem" "$REL/macos/bin/psexe"

# 平台附件、版號／素材守門、封包與雜湊全部在 Docker 內處理。
PSGOLEM_GO_CMD=python3 tools/go.sh tools/dist_pack.py "$VER"
