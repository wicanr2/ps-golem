#!/usr/bin/env bash
# 在 Linux 的 docker 內建出三個平台的發行包（docs/spec/005-cross-platform-delivery.md）。
#
#   tools/build_dist.sh <版號>        # 例：tools/build_dist.sh v0.1.0
#
# 產出 dist/<版號>/：
#   ps-golem-<版號>-linux-amd64.tar.gz     psgolem、psexe（cgo，連系統的 X11／OpenGL／ALSA）
#   ps-golem-<版號>-windows-amd64.zip      psgolem.exe、psexe.exe（不用 cgo）
#   ps-golem-<版號>-macos-universal.zip    ps-golem.app（arm64＋x86_64）、psexe
# 每包都帶 LICENSE、README.md、docs/play.md。不帶任何 BIOS、光碟映像、記憶卡。
#
# 版本目錄必須是新的，既有版本不覆寫。
set -euo pipefail
VER="${1:?用法：build_dist.sh <版號，例如 v0.1.0>}"
[[ "$VER" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版號格式 vX.Y.Z" >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/dist/$VER"
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
  CGO_ENABLED=1 tools/go.sh build -trimpath -ldflags "$LDF" -o "$REL/linux/$c" ./cmd/$c
done
echo "== windows/amd64"
for c in psgolem psexe; do
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 tools/go.sh build -trimpath -ldflags "$LDF" -o "$REL/windows/$c.exe" ./cmd/$c
done
echo "== darwin universal"
PSGOLEM_LDFLAGS="$LDF" tools/build_macos.sh "$REL/macos/bin"
tools/verify_macos.sh "$REL/macos/bin/psgolem" "$REL/macos/bin/psexe"

# macOS 的 .app：只放執行檔與 Info.plist，不簽 bundle（Linux 上做不出 bundle 簽章，壞簽比未簽更糟）。
APP="$STAGE/macos/ps-golem.app/Contents"
mkdir -p "$APP/MacOS"
mv "$STAGE/macos/bin/psgolem" "$APP/MacOS/psgolem"
mv "$STAGE/macos/bin/psexe" "$STAGE/macos/psexe"
rmdir "$STAGE/macos/bin"
cat > "$APP/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>psgolem</string>
  <key>CFBundleIdentifier</key><string>io.github.wicanr2.psgolem</string>
  <key>CFBundleName</key><string>ps-golem</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>${VER#v}</string>
  <key>CFBundleVersion</key><string>${VER#v}</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

for p in linux windows macos; do
  cp LICENSE README.md "$STAGE/$p/"
  mkdir -p "$STAGE/$p/docs"
  cp docs/play.md "$STAGE/$p/docs/"
done

# 守門：包裡不得有原版素材的副檔名。
bad=$(find "$STAGE" -type f \( -iname '*.bin' -o -iname '*.cue' -o -iname '*.iso' -o -iname '*.img' \
  -o -iname '*.mcd' -o -iname '*.mcr' -o -iname '*.chd' -o -iname '*.state' \) -print)
[ -z "$bad" ] || { echo "包裡有不該出現的檔案：" >&2; echo "$bad" >&2; exit 1; }

# 守門：每個執行檔都含這次的版號與設定畫面的字串，證明包進去的是這次編的程式（Mach-O 在 Linux 上只能這樣驗）。
for b in "$STAGE"/linux/* "$STAGE"/windows/*.exe "$STAGE"/macos/psexe "$APP"/MacOS/psgolem; do
  [ -f "$b" ] && [ -x "$b" ] || continue
  grep -qaF "$VER" "$b" || { echo "$b 沒有版號 $VER" >&2; exit 1; }
done
for b in "$STAGE"/linux/psgolem "$STAGE"/windows/psgolem.exe "$APP"/MacOS/psgolem; do
  grep -qaF "Drag and drop onto this window" "$b" || { echo "$b 沒有設定畫面" >&2; exit 1; }
done

echo "== 打包"
PSGOLEM_GO_CMD=bash tools/go.sh -c "
  set -euo pipefail
  cd /src/$REL
  tar -C linux -czf /src/dist/$VER/ps-golem-$VER-linux-amd64.tar.gz --owner=0 --group=0 .
  (cd windows && python3 -I -m zipfile -c /src/dist/$VER/ps-golem-$VER-windows-amd64.zip *)
  (cd macos && python3 -I -m zipfile -c /src/dist/$VER/ps-golem-$VER-macos-universal.zip *)
"
(cd "$OUT" && sha256sum ./*.tar.gz ./*.zip > SHA256SUMS)
ls -la "$OUT"
