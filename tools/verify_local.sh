#!/usr/bin/env bash
# 005 §8.3：從本機 Linux ZIP 解包，在既有無 Go 映像驗收 AppImage。
set -euo pipefail
if [[ "${1:-}" == --runtime ]]; then
  VER="${2:?}"
  E="/src/workplace/local-pack/$VER"
  GAME="$E/smoke/ps-golem"
  IMAGE="$GAME/ps-golem-$VER-x86_64.AppImage"
  if command -v go; then echo '驗收映像不應有 Go' >&2; exit 1; fi
  cd "$E/smoke/extracted"
  "$IMAGE" --appimage-extract
  ldd squashfs-root/usr/bin/psgolem > "$E/linux-ldd.log"
  if grep -F 'not found' "$E/linux-ldd.log"; then exit 1; fi
  export XDG_CONFIG_HOME="$E/smoke/user-config"
  mkdir -p "$XDG_CONFIG_HOME"
  # 由另一個工作目錄啟動，確認設定與素材基準不依賴 cwd。
  cd /tmp
  xvfb-run -a "$IMAGE" --appimage-extract-and-run -version > "$E/linux-version.log"
  grep -Fx "$VER" "$E/linux-version.log"
  xvfb-run -a "$IMAGE" --appimage-extract-and-run -mute -fields 2000 -screenshot "$E/first.png" > "$E/linux-first.log" 2>&1
  grep -F '"bios": "bios/SCPH5500.BIN"' "$GAME/settings.json"
  grep -F '"cue": "disc/sotn.cue"' "$GAME/settings.json"
  for value in '"hd": 2' '"pgxp": true' '"filter": "xbr"' '"widescreen": true'; do
    grep -F "$value" "$GAME/settings.json"
  done
  test ! -e "$XDG_CONFIG_HOME/ps-golem/settings.json"
  cp "$GAME/settings.json" "$E/settings-first.json"
  xvfb-run -a "$IMAGE" --appimage-extract-and-run -mute -fields 2000 -screenshot "$E/second.png" > "$E/linux-second.log" 2>&1
  cmp "$E/first.png" "$E/second.png"
  cmp "$GAME/settings.json" "$E/settings-first.json"
  echo 'AppImage：無 Go、無缺庫、外部可攜設定、自動尋找、相對路徑寫回、field 2000 與重啟畫面相同，通過'
  exit
fi
VER="${1:?用法：verify_local.sh <版號>}"
[[ "$VER" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]] || exit 2
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
ORIG="${PSGOLEM_ORIG:?需指定原版素材目錄}"
for file in bios/SCPH5500.BIN disc/sotn.cue disc/sotn.bin; do
  test -f "$ORIG/$file" || { echo "缺少 $ORIG/$file" >&2; exit 1; }
done
PSGOLEM_GO_CMD=python3 tools/go.sh tools/local_pack.py smoke "$VER"
PSGOLEM_EXTRA_MOUNTS="$ORIG/bios/SCPH5500.BIN:/src/workplace/local-pack/$VER/smoke/ps-golem/bios/SCPH5500.BIN:ro;$ORIG/disc/sotn.cue:/src/workplace/local-pack/$VER/smoke/ps-golem/disc/sotn.cue:ro;$ORIG/disc/sotn.bin:/src/workplace/local-pack/$VER/smoke/ps-golem/disc/sotn.bin:ro" \
  PSGOLEM_GO_IMAGE=psgolem-wine-verify:noble-r2 PSGOLEM_GO_CMD=bash PSGOLEM_TIMEOUT=15m \
  tools/go.sh tools/verify_local.sh --runtime "$VER"
PSGOLEM_GO_CMD=python3 tools/go.sh tools/local_pack.py check-image "$VER"
